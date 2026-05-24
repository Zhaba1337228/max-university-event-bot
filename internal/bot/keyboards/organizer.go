package keyboards

import (
	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
	"github.com/Zhaba1337228/max-university-event-bot/internal/domain"
)

// OrganizerEventList — список своих мероприятий организатора.
// Каждое событие — отдельный ряд с переходом в «карточку организатора».
func OrganizerEventList(events []*domain.Event) *maxbot.Keyboard {
	kb := newKB()
	for _, e := range events {
		kb.AddRow().AddCallback(e.Title, schemes.DEFAULT, callbacks.OrgStats(e.ID))
	}
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// OrganizerEventActions — клавиатура «карточки организатора».
// На одной странице: статистика, участники, рассылка, закрыть/открыть, AI-сводка.
func OrganizerEventActions(eventID int64, status domain.EventStatus) *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().
		AddCallback("Участники", schemes.DEFAULT, callbacks.OrgListParticipants(eventID, 0)).
		AddCallback("Найти по коду", schemes.DEFAULT, callbacks.OrgListSearchCode(eventID))
	kb.AddRow().
		AddCallback("Экспорт CSV", schemes.DEFAULT, callbacks.OrgListExport(eventID)).
		AddCallback("Рассылка", schemes.DEFAULT, callbacks.OrgNotifStart(eventID))
	kb.AddRow().AddCallback("AI-сводка", schemes.DEFAULT, callbacks.OrgAISummary(eventID))
	if status == domain.EventStatusOpen {
		kb.AddRow().AddCallback("Закрыть регистрацию", schemes.NEGATIVE, callbacks.OrgCloseAsk(eventID))
	} else if status == domain.EventStatusClosed {
		kb.AddRow().AddCallback("Открыть регистрацию", schemes.POSITIVE, callbacks.OrgOpenAsk(eventID))
	}
	kb.AddRow().AddCallback("К списку мероприятий", schemes.DEFAULT, callbacks.OrgEntry())
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// OrganizerParticipants — навигация по постраничному списку участников.
func OrganizerParticipants(eventID int64, offset, total, perPage int) *maxbot.Keyboard {
	kb := newKB()
	navRow := kb.AddRow()
	if offset >= perPage {
		navRow.AddCallback("Назад", schemes.DEFAULT, callbacks.OrgListParticipants(eventID, offset-perPage))
	}
	if offset+perPage < total {
		navRow.AddCallback("Дальше", schemes.DEFAULT, callbacks.OrgListParticipants(eventID, offset+perPage))
	}
	kb.AddRow().AddCallback("Экспорт CSV", schemes.DEFAULT, callbacks.OrgListExport(eventID))
	kb.AddRow().AddCallback("К карточке мероприятия", schemes.DEFAULT, callbacks.OrgStats(eventID))
	return kb
}

// OrganizerNotifConfirm — подтверждение рассылки.
// Дополнительная кнопка «Улучшить через ИИ» — на день 16; всегда видна, но
// при выключенном AI_NOTIFICATION_REWRITER_ENABLED сервис вернёт ErrAIUnavailable.
func OrganizerNotifConfirm() *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().AddCallback("Улучшить через ИИ", schemes.DEFAULT, callbacks.OrgNotifAIRewrite())
	kb.AddRow().
		AddCallback("Отправить", schemes.POSITIVE, callbacks.OrgNotifSend()).
		AddCallback("Отмена", schemes.NEGATIVE, callbacks.OrgNotifCancel())
	return kb
}

// OrganizerCloseConfirm — подтверждение закрытия регистрации.
func OrganizerCloseConfirm(eventID int64) *maxbot.Keyboard {
	return YesNo(
		"Да, закрыть", callbacks.OrgCloseYes(eventID),
		"Отмена", callbacks.OrgEntry(),
	)
}

// OrganizerOpenConfirm — подтверждение открытия регистрации.
func OrganizerOpenConfirm(eventID int64) *maxbot.Keyboard {
	return YesNo(
		"Да, открыть", callbacks.OrgOpenYes(eventID),
		"Отмена", callbacks.OrgEntry(),
	)
}

// OrganizerParticipantsBack — кнопка «назад к карточке мероприятия» из поиска.
func OrganizerParticipantsBack(eventID int64) *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().AddCallback("К карточке мероприятия", schemes.DEFAULT, callbacks.OrgStats(eventID))
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}
