package fsm

import "encoding/json"

// UserFSMContext — структурированное состояние диалога одного пользователя.
//
// Сериализуется в JSONB в user_states.context. Все поля опциональны и
// помечены omitempty, чтобы в БД хранилось только заполненное.
type UserFSMContext struct {
	// Shared.
	CurrentEventID int64 `json:"current_event_id,omitempty"`

	// Registration draft (FSM шаги reg_*).
	DraftFullName string `json:"draft_full_name,omitempty"`
	DraftInterest string `json:"draft_interest,omitempty"`

	// Cancellation.
	CancelRegID int64 `json:"cancel_reg_id,omitempty"`

	// Organizer.
	OrganizerEventID  int64  `json:"organizer_event_id,omitempty"`
	NotificationDraft string `json:"notification_draft,omitempty"`
	NotificationFinal string `json:"notification_final,omitempty"`

	// Pagination.
	Offset int `json:"offset,omitempty"`

	// Event list filter (format: "offline", "online", "hybrid", "" = all).
	EventFilter string `json:"event_filter,omitempty"`

	// AI recommendations pagination.
	AIRecommIDs []int64 `json:"ai_recomm_ids,omitempty"`
	AIOffset    int     `json:"ai_offset,omitempty"`
}

// Marshal сериализует контекст в JSON для хранения в БД.
// При ошибке возвращает "{}" — пустой объект, безопасный для PostgreSQL JSONB.
func (c UserFSMContext) Marshal() []byte {
	b, err := json.Marshal(c)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// Unmarshal восстанавливает контекст из JSON-байтов.
// Пустой или мусорный вход даст zero-value UserFSMContext без ошибки —
// диалог корректно сбросится в дефолтное состояние.
func Unmarshal(b []byte) UserFSMContext {
	var c UserFSMContext
	if len(b) == 0 {
		return c
	}
	_ = json.Unmarshal(b, &c)
	return c
}

// Reset обнуляет контекст до zero value. Используется при выходе из сценария.
func (c *UserFSMContext) Reset() { *c = UserFSMContext{} }
