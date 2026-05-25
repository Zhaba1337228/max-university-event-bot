package domain

import (
	"encoding/json"
	"time"
)

// ActionType — тип бизнес-события для audit log.
type ActionType string

// Базовые типы действий. Список расширяется по мере добавления сценариев.
const (
	ActionRegistrationCreated       ActionType = "registration_created"
	ActionRegistrationCancelledUser ActionType = "registration_cancelled_by_user"
	ActionRegistrationCancelledOrg  ActionType = "registration_cancelled_by_organizer"
	ActionWaitlistAdded             ActionType = "waitlist_added"
	ActionWaitlistPromoted          ActionType = "waitlist_promoted"
	ActionNotificationSent          ActionType = "notification_sent"
	ActionEventClosed               ActionType = "event_closed"
	ActionEventOpened               ActionType = "event_opened"
	ActionEventCreated              ActionType = "event_created"
	ActionEventUpdated              ActionType = "event_updated"

	ActionAIRecommendation        ActionType = "ai_recommendation_shown"
	ActionAINotificationRewritten ActionType = "ai_notification_rewritten"
	ActionAISummaryGenerated      ActionType = "ai_summary_generated"

	ActionAdminLogin           ActionType = "admin_login"
	ActionAdminLogout          ActionType = "admin_logout"
	ActionMarkedAttendedManual ActionType = "marked_attended_manually"
	ActionMarkedNoShowManual   ActionType = "marked_no_show_manually"
	ActionUserRoleChanged      ActionType = "user_role_changed"
	ActionParticipantsExported ActionType = "participants_exported_csv"
	ActionPIIUnmasked          ActionType = "pii_unmasked"
	ActionCheckinScanned       ActionType = "checkin_scanned"

	ActionConsentGranted ActionType = "consent_granted"
	ActionForgetMe       ActionType = "forget_me"
)

// ActionLog — иммутабельная запись о бизнес-событии.
//
// В Payload кладём только id-шники и нечувствительные поля.
// Запрещено логировать ФИО, email и телефон.
type ActionLog struct {
	ID             int64
	ActorUserID    *int64
	TargetUserID   *int64
	EventID        *int64
	RegistrationID *int64
	Action         ActionType
	Payload        json.RawMessage
	CreatedAt      time.Time
}
