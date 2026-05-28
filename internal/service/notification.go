package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/maxclient"
	"github.com/Zhaba1337228/max-university-event-bot/internal/repo"
)

// Notification — сервис уведомлений.
type Notification interface {
	// SendBroadcast рассылает text всем активным (registered) участникам события.
	SendBroadcast(ctx context.Context, eventID int64, text string) (int, error)

	// ScheduleUpcomingReminders создаёт записи notifications.reminder_*
	ScheduleUpcomingReminders(ctx context.Context, hoursBefore []int) (int, error)

	// DispatchDue выгребает pending уведомления со scheduled_at <= now и шлёт их.
	DispatchDue(ctx context.Context, now time.Time, limit int) (int, int, error)

	// SendPersonalFeed отправляет пользователю персональные рекомендации после check-in.
	// maxUserID — MAX user_id получателя. text — готовый текст сообщения.
	SendPersonalFeed(ctx context.Context, maxUserID int64, text string) error
}

type notificationService struct {
	pool      repo.Querier
	notifs    repo.NotificationRepo
	regs      repo.RegistrationRepo
	events    repo.EventRepo
	users     repo.UserRepo
	logs      repo.ActionLogRepo
	api       *maxclient.Client
	rateRPS   int
	batchSize int
	log       *slog.Logger
}

// NewNotification создаёт сервис.
func NewNotification(
	pool repo.Querier,
	notifs repo.NotificationRepo,
	regs repo.RegistrationRepo,
	events repo.EventRepo,
	users repo.UserRepo,
	logs repo.ActionLogRepo,
	api *maxclient.Client,
	rateRPS, batchSize int,
	log *slog.Logger,
) Notification {
	if rateRPS <= 0 {
		rateRPS = 20
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &notificationService{
		pool:      pool,
		notifs:    notifs,
		regs:      regs,
		events:    events,
		users:     users,
		logs:      logs,
		api:       api,
		rateRPS:   rateRPS,
		batchSize: batchSize,
		log:       log.With("service", "notification"),
	}
}

// SendBroadcast — массовая рассылка по registered участникам.
//
// Алгоритм:
//  1. Проверка события (открыто/закрыто значения не имеет — рассылка возможна).
//  2. ListByEvent(registered) → пользователи.
//  3. Для каждого: Schedule(now) → DispatchDue (sync, в текущем вызове).
//
// На MVP отправляем синхронно с rate-limit между сообщениями (1/rps).
// В production это переедет в scheduler-job с per-user back-off (День 16/17).
func (s *notificationService) SendBroadcast(ctx context.Context, eventID int64, text string) (int, error) {
	if text == "" {
		return 0, fmt.Errorf("broadcast: empty text")
	}
	ev, err := s.events.Get(ctx, s.pool, eventID)
	if err != nil {
		return 0, fmt.Errorf("get event: %w", err)
	}
	if ev == nil {
		return 0, ErrEventNotFound
	}

	regs, err := s.regs.ListByEvent(ctx, s.pool, eventID, domain.RegStatusRegistered, 1000, 0)
	if err != nil {
		return 0, fmt.Errorf("list regs: %w", err)
	}
	if len(regs) == 0 {
		return 0, nil
	}

	now := time.Now()
	sent := 0
	delay := time.Second / time.Duration(s.rateRPS)
	if delay <= 0 {
		delay = 50 * time.Millisecond
	}

	for _, r := range regs {
		select {
		case <-ctx.Done():
			return sent, ctx.Err()
		default:
		}

		// TZ §6: пользователь мог отключить уведомления по этому мероприятию.
		if r.NotificationsDisabled {
			continue
		}

		// Создаём запись в notifications (с дедупом на (user, event, type, minute)).
		notif := &domain.Notification{
			EventID:     ev.ID,
			UserID:      r.UserID,
			Type:        domain.NotifOrganizerBroadcast,
			Text:        text,
			Status:      domain.NotifStatusPending,
			ScheduledAt: now,
		}
		id, err := s.notifs.Schedule(ctx, s.pool, notif)
		if err != nil {
			s.log.Warn("schedule notif failed", "err", err, "user_id", r.UserID)
			continue
		}
		if id == 0 {
			// Дубль (минута уже отправляли) — пропускаем без ошибки.
			continue
		}

		// Грузим max_user_id для отправки.
		u, err := s.users.GetByID(ctx, s.pool, r.UserID)
		if err != nil || u == nil {
			s.log.Warn("user not found for notif", "err", err, "user_id", r.UserID)
			_ = s.notifs.MarkFailed(ctx, s.pool, id, "user not found")
			continue
		}

		if err := s.api.SendTextToUser(ctx, u.MaxUserID, text); err != nil {
			s.log.Warn("send notification failed", "err", err, "user_id", r.UserID)
			_ = s.notifs.MarkFailed(ctx, s.pool, id, err.Error())
			continue
		}
		_ = s.notifs.MarkSent(ctx, s.pool, id, time.Now())

		// ActionLog один раз на рассылку (не на каждого пользователя — слишком шумно),
		// но мы пишем по факту отправки чтобы видеть кому ушло.
		actorID := r.UserID
		evID := ev.ID
		regID := r.ID
		_ = s.logs.Append(ctx, s.pool, &domain.ActionLog{
			TargetUserID:   &actorID,
			EventID:        &evID,
			RegistrationID: &regID,
			Action:         domain.ActionNotificationSent,
		})

		sent++
		// rate limit
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return sent, ctx.Err()
		}
	}

	return sent, nil
}

// ScheduleUpcomingReminders — заглушка под scheduler (День 16).
func (s *notificationService) ScheduleUpcomingReminders(ctx context.Context, _ []int) (int, error) {
	_ = ctx
	// День 16: реализация через ListUpcomingForReminders + Schedule (reminder_24h/1h).
	return 0, nil
}

// DispatchDue — заглушка под scheduler (День 16).
func (s *notificationService) DispatchDue(ctx context.Context, _ time.Time, _ int) (int, int, error) {
	_ = ctx
	return 0, 0, nil
}

// SendPersonalFeed отправляет персональные рекомендации пользователю.
func (s *notificationService) SendPersonalFeed(ctx context.Context, maxUserID int64, text string) error {
	return s.api.SendText(ctx, maxUserID, text)
}
