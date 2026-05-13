package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
	"github.com/Zhaba1337228/max-university-event-bot/internal/external/gigachat"
)

// AIConfig — feature flags + лимиты.
type AIConfig struct {
	RecommenderEnabled bool
	RewriterEnabled    bool
	SummaryEnabled     bool
	RequestTimeout     time.Duration
}

// Recommendation — одно предложение AI.
type Recommendation struct {
	EventID int64
	Title   string
	Reason  string
}

// AI — фасад над GigaChat. На любую ошибку клиента возвращает ErrAIUnavailable;
// вызывающий сервис/handler должен иметь fallback (обычный список / исходный текст).
type AI interface {
	RecommendEvents(ctx context.Context, interest string, events []*domain.Event) ([]Recommendation, error)
	RewriteNotification(ctx context.Context, draft string, ev *domain.Event) (string, error)
	OrganizerSummary(ctx context.Context, ev *domain.Event, stats *domain.EventStats) (string, error)
}

type aiService struct {
	client *gigachat.Client
	cfg    AIConfig
	log    *slog.Logger
}

// NewAI создаёт фасад. Если client == nil или AuthKey пустой — все методы
// сразу возвращают ErrAIUnavailable (handler уйдёт в fallback).
func NewAI(client *gigachat.Client, cfg AIConfig, log *slog.Logger) AI {
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 15 * time.Second
	}
	return &aiService{
		client: client,
		cfg:    cfg,
		log:    log.With("service", "ai"),
	}
}

// RecommendEvents — подбор мероприятий по интересу пользователя.
func (a *aiService) RecommendEvents(ctx context.Context, interest string, events []*domain.Event) ([]Recommendation, error) {
	if !a.cfg.RecommenderEnabled || a.client == nil {
		return nil, ErrAIUnavailable
	}
	if strings.TrimSpace(interest) == "" || len(events) == 0 {
		return nil, ErrAIUnavailable
	}

	// Минимизируем JSON для AI: только id, title, tags, format, дата.
	minimal := make([]map[string]any, 0, len(events))
	for _, e := range events {
		minimal = append(minimal, map[string]any{
			"event_id":  e.ID,
			"title":     e.Title,
			"tags":      e.Tags,
			"starts_at": e.StartsAt.Format(time.RFC3339),
			"format":    string(e.Format),
		})
	}
	listJSON, _ := json.Marshal(map[string]any{"events": minimal})

	userMsg := fmt.Sprintf(gigachat.UserRecommenderTemplate, interest, string(listJSON))

	resp, err := a.callWithTimeout(ctx, []gigachat.ChatMessage{
		{Role: "system", Content: gigachat.SystemRecommender},
		{Role: "user", Content: userMsg},
	}, 0.2)
	if err != nil {
		return nil, ErrAIUnavailable
	}

	raw := stripCodeFences(resp.Choices[0].Message.Content)
	var parsed struct {
		Recommendations []struct {
			EventID int64  `json:"event_id"`
			Title   string `json:"title"`
			Reason  string `json:"reason"`
		} `json:"recommendations"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		a.log.Warn("ai: bad json from recommender", "err", err)
		return nil, ErrAIUnavailable
	}

	out := make([]Recommendation, 0, len(parsed.Recommendations))
	for _, r := range parsed.Recommendations {
		// Защита: AI может выдумать event_id — фильтруем по реальному списку.
		var found *domain.Event
		for _, e := range events {
			if e.ID == r.EventID {
				found = e
				break
			}
		}
		if found == nil {
			continue
		}
		out = append(out, Recommendation{
			EventID: r.EventID,
			Title:   found.Title,
			Reason:  r.Reason,
		})
	}
	if len(out) == 0 {
		return nil, ErrAIUnavailable
	}
	return out, nil
}

// RewriteNotification — улучшение текста рассылки.
func (a *aiService) RewriteNotification(ctx context.Context, draft string, ev *domain.Event) (string, error) {
	if !a.cfg.RewriterEnabled || a.client == nil {
		return "", ErrAIUnavailable
	}
	draft = strings.TrimSpace(draft)
	if draft == "" || ev == nil {
		return "", ErrAIUnavailable
	}

	when := ev.StartsAt.Format("02.01.2006 15:04")
	user := fmt.Sprintf(gigachat.UserRewriterTemplate, draft, ev.Title, when, ev.Location)

	resp, err := a.callWithTimeout(ctx, []gigachat.ChatMessage{
		{Role: "system", Content: gigachat.SystemRewriter},
		{Role: "user", Content: user},
	}, 0.3)
	if err != nil {
		return "", ErrAIUnavailable
	}

	raw := stripCodeFences(resp.Choices[0].Message.Content)
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		a.log.Warn("ai: bad json from rewriter", "err", err)
		return "", ErrAIUnavailable
	}
	text := strings.TrimSpace(parsed.Text)
	if text == "" || len(text) > 4000 {
		return "", ErrAIUnavailable
	}
	return text, nil
}

// OrganizerSummary — короткая управленческая сводка.
func (a *aiService) OrganizerSummary(ctx context.Context, ev *domain.Event, stats *domain.EventStats) (string, error) {
	if !a.cfg.SummaryEnabled || a.client == nil {
		return "", ErrAIUnavailable
	}
	if ev == nil || stats == nil {
		return "", ErrAIUnavailable
	}

	// Топ-интересы — собираем в одну строку для prompt'а.
	top := topInterestsString(stats.TopInterests)

	user := fmt.Sprintf(gigachat.UserSummaryTemplate,
		ev.Title, stats.Capacity, stats.Registered,
		stats.FreeSeats, stats.Waitlist, stats.Cancelled, top)

	resp, err := a.callWithTimeout(ctx, []gigachat.ChatMessage{
		{Role: "system", Content: gigachat.SystemSummary},
		{Role: "user", Content: user},
	}, 0.3)
	if err != nil {
		return "", ErrAIUnavailable
	}

	raw := stripCodeFences(resp.Choices[0].Message.Content)
	var parsed struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		a.log.Warn("ai: bad json from summary", "err", err)
		return "", ErrAIUnavailable
	}
	text := strings.TrimSpace(parsed.Summary)
	if text == "" {
		return "", ErrAIUnavailable
	}
	return text, nil
}

// callWithTimeout оборачивает Chat() в свой контекст-таймаут.
func (a *aiService) callWithTimeout(ctx context.Context, msgs []gigachat.ChatMessage, temp float64) (*gigachat.ChatResponse, error) {
	if a.client == nil {
		return nil, errors.New("ai client is nil")
	}
	ctx2, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()
	resp, err := a.client.Chat(ctx2, msgs, temp)
	if err != nil {
		a.log.Warn("ai chat failed", "err", err)
		return nil, err
	}
	return resp, nil
}

// stripCodeFences убирает ```json и ``` обёртки — AI иногда их добавляет
// несмотря на инструкцию.
func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// topInterestsString собирает map[interest]count в строку для prompt'а.
func topInterestsString(m map[string]int) string {
	if len(m) == 0 {
		return "—"
	}
	type kv struct {
		k string
		v int
	}
	arr := make([]kv, 0, len(m))
	for k, v := range m {
		arr = append(arr, kv{k, v})
	}
	sort.SliceStable(arr, func(i, j int) bool {
		if arr[i].v != arr[j].v {
			return arr[i].v > arr[j].v
		}
		return arr[i].k < arr[j].k
	})
	parts := make([]string, 0, len(arr))
	for _, p := range arr {
		parts = append(parts, fmt.Sprintf("%s (%d)", p.k, p.v))
	}
	return strings.Join(parts, ", ")
}
