package domain

import "time"

// NotificationType — тип уведомления.
type NotificationType string

// Возможные типы уведомлений.
const (
	NotifReminder24h        NotificationType = "reminder_24h"
	NotifReminder1h         NotificationType = "reminder_1h"
	NotifOrganizerBroadcast NotificationType = "organizer_broadcast"
	NotifWaitlistPromoted   NotificationType = "waitlist_promoted"
	NotifEventCancelled     NotificationType = "event_cancelled"
)

// NotificationStatus — статус доставки уведомления.
type NotificationStatus string

// Возможные статусы.
const (
	NotifStatusPending NotificationStatus = "pending"
	NotifStatusSent    NotificationStatus = "sent"
	NotifStatusFailed  NotificationStatus = "failed"
	NotifStatusSkipped NotificationStatus = "skipped"
)

// Notification — отложенное сообщение пользователю (рассылка / напоминание / waitlist).
//
// Уникальный индекс idx_notif_dedup (см. миграция 8) защищает от дубля при
// многократном вызове DispatchDue (округление scheduled_at до минуты + ключ
// user_id/event_id/type).
type Notification struct {
	ID          int64
	EventID     int64
	UserID      int64
	Type        NotificationType
	Text        string
	Status      NotificationStatus
	ScheduledAt time.Time
	SentAt      *time.Time
	Error       *string
	CreatedAt   time.Time
}
