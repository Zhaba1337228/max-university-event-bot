// Package fsm — конечный автомат состояний пользователя в боте.
//
// Состояние и контекст хранятся в PostgreSQL (таблица user_states),
// чтобы пережить рестарт бота и можно было раскатывать без потери
// диалогов. См. execution_plan.md, разделы 14 и 8.7.
package fsm

// State — строковый идентификатор состояния в диалоге.
//
// Значение хранится в user_states.state и проверяется при каждом
// входящем апдейте. Если состояние ожидает текстовый ввод, мы
// маршрутизируем сообщение в нужный handler; если callback —
// FSM используется как охранник от устаревших кнопок.
type State = string

// Состояния абитуриентского сценария.
const (
	StateMainMenu = "main_menu"

	StateEventList    = "event_list"
	StateEventDetails = "event_details"

	StateRegConsent      = "reg_consent" // 152-ФЗ — согласие на ПДн
	StateRegFullName     = "reg_full_name"
	StateRegContact      = "reg_contact"
	StateRegInterest     = "reg_interest"
	StateRegConfirmation = "reg_confirmation"

	StateMyRegistration       = "my_registration"
	StateCancelConfirmation   = "cancel_confirmation"
	StateWaitlistConfirmation = "waitlist_confirmation"
	StateForgetMeConfirm      = "forget_me_confirm"

	StateAIPickIntent = "ai_pick_intent"
)

// Состояния организаторского сценария.
const (
	StateOrganizerMenu         = "organizer_menu"
	StateOrganizerEventList    = "organizer_event_list"
	StateOrganizerParticipants = "organizer_participants"
	StateOrganizerNotifText    = "organizer_notif_text"
	StateOrganizerNotifConfirm = "organizer_notif_confirm"
	StateOrganizerCloseConfirm = "organizer_close_confirm"
)

// IsTextInput сообщает, ожидает ли состояние свободного текстового ввода
// от пользователя (например, ФИО на шаге reg_full_name). Используется
// в RouteMessage для решения «отдать в handler» vs «показать fallback».
func IsTextInput(s State) bool {
	switch s {
	case StateRegFullName,
		StateRegContact,
		StateRegInterest,
		StateAIPickIntent,
		StateOrganizerNotifText:
		return true
	}
	return false
}
