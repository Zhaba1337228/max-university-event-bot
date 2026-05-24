package domain

import "time"

// EventStatus описывает жизненный цикл мероприятия.
type EventStatus string

// Возможные статусы мероприятия.
const (
	EventStatusOpen      EventStatus = "open"
	EventStatusClosed    EventStatus = "closed"
	EventStatusCancelled EventStatus = "cancelled"
	EventStatusFinished  EventStatus = "finished"
)

// EventFormat — формат проведения.
type EventFormat string

// Возможные форматы мероприятия.
const (
	EventFormatOffline EventFormat = "offline"
	EventFormatOnline  EventFormat = "online"
	EventFormatHybrid  EventFormat = "hybrid"
)

// Event — мероприятие университета.
//
// Capacity — целевое число мест; остаток считается через CountByEvent(registered).
// Tags используется для AI-подбора по интересу абитуриента.
type Event struct {
	ID                int64
	Title             string
	Description       string
	ShortSummary      *string
	StartsAt          time.Time
	EndsAt            *time.Time
	Location          string
	Format            EventFormat
	Capacity          int
	Status            EventStatus
	CreatedBy         *int64
	Tags              []string
	LateCancelAllowed bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// IsOpenForRegistration сообщает, можно ли сейчас принимать новые записи.
func (e *Event) IsOpenForRegistration() bool {
	return e != nil && e.Status == EventStatusOpen
}

// EventStats — управленческая сводка по мероприятию.
type EventStats struct {
	Capacity     int
	Registered   int
	Cancelled    int
	Waitlist     int
	Attended     int
	NoShow       int
	FreeSeats    int
	TopInterests map[string]int
}
