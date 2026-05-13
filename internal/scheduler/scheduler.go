// Package scheduler — фоновые периодические задачи: reminders, dispatch,
// purge устаревших FSM-состояний.
//
// Используется gocron/v2 — потокобезопасный, отменяемый, с поддержкой
// duration- и cron-job'ов.
package scheduler

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// Config — настройки.
type Config struct {
	ReminderHoursCSV string // "24,1" → reminder за 24ч и за 1ч до старта
	DispatchInterval time.Duration
	ScheduleInterval time.Duration
	PurgeInterval    time.Duration
	StateTTL         time.Duration
}

// Scheduler — обёртка над gocron.Scheduler.
type Scheduler struct {
	cfg    Config
	log    *slog.Logger
	s      gocron.Scheduler
	notifs repo.NotificationRepo
	events repo.EventRepo
	regs   repo.RegistrationRepo
	users  repo.UserRepo
	states repo.UserStateRepo
	db     repo.Querier
	api    *maxclient.Client
}

// New создаёт scheduler, но не запускает.
func New(cfg Config, log *slog.Logger,
	notifs repo.NotificationRepo, events repo.EventRepo,
	regs repo.RegistrationRepo, users repo.UserRepo,
	states repo.UserStateRepo, db repo.Querier, api *maxclient.Client,
) (*Scheduler, error) {
	if cfg.DispatchInterval <= 0 {
		cfg.DispatchInterval = 60 * time.Second
	}
	if cfg.ScheduleInterval <= 0 {
		cfg.ScheduleInterval = 5 * time.Minute
	}
	if cfg.PurgeInterval <= 0 {
		cfg.PurgeInterval = 24 * time.Hour
	}
	if cfg.StateTTL <= 0 {
		cfg.StateTTL = 7 * 24 * time.Hour
	}
	if cfg.ReminderHoursCSV == "" {
		cfg.ReminderHoursCSV = "24,1"
	}

	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}
	return &Scheduler{
		cfg: cfg, log: log.With("component", "scheduler"),
		s: s, notifs: notifs, events: events, regs: regs, users: users,
		states: states, db: db, api: api,
	}, nil
}

// Start регистрирует все cron-задачи и запускает scheduler.
func (s *Scheduler) Start() error {
	// 1. Каждую минуту — отправлять due-уведомления.
	if _, err := s.s.NewJob(
		gocron.DurationJob(s.cfg.DispatchInterval),
		gocron.NewTask(s.dispatchDue),
	); err != nil {
		return err
	}

	// 2. Каждые 5 минут — планировать reminder'ы для свежих регистраций.
	if _, err := s.s.NewJob(
		gocron.DurationJob(s.cfg.ScheduleInterval),
		gocron.NewTask(s.scheduleReminders),
	); err != nil {
		return err
	}

	// 3. Раз в сутки — чистка устаревших FSM-state'ов.
	if _, err := s.s.NewJob(
		gocron.DurationJob(s.cfg.PurgeInterval),
		gocron.NewTask(s.purgeStaleStates),
	); err != nil {
		return err
	}

	s.s.Start()
	s.log.Info("scheduler started",
		"dispatch", s.cfg.DispatchInterval,
		"schedule", s.cfg.ScheduleInterval,
		"purge", s.cfg.PurgeInterval,
	)
	return nil
}

// Stop корректно останавливает scheduler.
func (s *Scheduler) Stop() {
	if err := s.s.Shutdown(); err != nil {
		s.log.Warn("scheduler shutdown", "err", err)
	}
}

// dispatchDue выгребает pending notifications со scheduled_at <= now
// и отправляет их через MAX SDK.
func (s *Scheduler) dispatchDue() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Now()
	batch, err := s.notifs.PickDue(ctx, s.db, now, 50)
	if err != nil {
		s.log.Error("dispatch: pick due failed", "err", err)
		return
	}
	if len(batch) == 0 {
		return
	}

	sent := 0
	for _, n := range batch {
		select {
		case <-ctx.Done():
			return
		default:
		}

		u, err := s.users.GetByID(ctx, s.db, n.UserID)
		if err != nil || u == nil {
			_ = s.notifs.MarkFailed(ctx, s.db, n.ID, "user not found")
			continue
		}
		if err := s.api.SendTextToUser(ctx, u.MaxUserID, n.Text); err != nil {
			_ = s.notifs.MarkFailed(ctx, s.db, n.ID, err.Error())
			continue
		}
		_ = s.notifs.MarkSent(ctx, s.db, n.ID, time.Now())
		sent++

		// rate limit ~20 rps — между отправками 50 ms.
		time.Sleep(50 * time.Millisecond)
	}
	if sent > 0 {
		s.log.Info("dispatch: sent", "count", sent)
	}
}

// scheduleReminders — для всех событий, стартующих в ближайшие 25 часов,
// планирует reminder'ы (24h, 1h до старта) для всех registered участников.
//
// Идемпотентно: uniq_notif_dedup (по minute-bucket) защищает от дублей.
func (s *Scheduler) scheduleReminders() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	hours, err := parseHoursCSV(s.cfg.ReminderHoursCSV)
	if err != nil {
		s.log.Warn("schedule: bad hours csv", "err", err, "csv", s.cfg.ReminderHoursCSV)
		return
	}
	if len(hours) == 0 {
		return
	}

	// Берём события на ближайшие 25 часов (с запасом для reminder_24h).
	upcoming, err := s.events.ListUpcomingForReminders(ctx, s.db, 25*time.Hour)
	if err != nil {
		s.log.Error("schedule: list upcoming failed", "err", err)
		return
	}
	if len(upcoming) == 0 {
		return
	}

	scheduled := 0
	for _, ev := range upcoming {
		regs, err := s.regs.ListByEvent(ctx, s.db, ev.ID, domain.RegStatusRegistered, 500, 0)
		if err != nil {
			s.log.Warn("schedule: list regs failed", "err", err, "event_id", ev.ID)
			continue
		}

		for _, h := range hours {
			scheduledAt := ev.StartsAt.Add(-time.Duration(h) * time.Hour)
			if scheduledAt.Before(time.Now()) {
				continue // время уже прошло
			}
			notifType := domain.NotifReminder24h
			if h == 1 {
				notifType = domain.NotifReminder1h
			}
			text := buildReminderText(ev, h)

			for _, r := range regs {
				n := &domain.Notification{
					EventID:     ev.ID,
					UserID:      r.UserID,
					Type:        notifType,
					Text:        text,
					ScheduledAt: scheduledAt,
				}
				id, err := s.notifs.Schedule(ctx, s.db, n)
				if err != nil {
					s.log.Warn("schedule notif failed", "err", err)
					continue
				}
				if id > 0 {
					scheduled++
				}
			}
		}
	}
	if scheduled > 0 {
		s.log.Info("schedule: reminders scheduled", "count", scheduled)
	}
}

// purgeStaleStates удаляет user_states старше StateTTL.
func (s *Scheduler) purgeStaleStates() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cutoff := time.Now().Add(-s.cfg.StateTTL)
	n, err := s.states.PurgeStaleBefore(ctx, s.db, cutoff)
	if err != nil {
		s.log.Error("purge fsm failed", "err", err)
		return
	}
	if n > 0 {
		s.log.Info("purged stale FSM states", "count", n)
	}
}

// parseHoursCSV "24,1" → [24, 1].
func parseHoursCSV(csv string) ([]int, error) {
	parts := strings.Split(csv, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		h, err := strconv.Atoi(p)
		if err != nil || h <= 0 || h > 720 {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

// buildReminderText — короткий текст напоминания (формат — план §16.1).
func buildReminderText(ev *domain.Event, hoursBefore int) string {
	header := "Напоминание о мероприятии"
	switch hoursBefore {
	case 1:
		header = "Через час начнётся мероприятие"
	case 24:
		header = "Завтра мероприятие, на которое вы записаны"
	}
	when := ev.StartsAt.Format("02.01.2006 15:04")
	return header + "\n\n" +
		"Мероприятие: " + ev.Title + "\n" +
		"Когда: " + when + "\n" +
		"Где: " + ev.Location + "\n\n" +
		"Не забудьте показать QR-код на входе."
}
