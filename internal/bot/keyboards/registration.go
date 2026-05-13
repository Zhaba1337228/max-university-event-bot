package keyboards

import (
	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
)

// RegConsent — экран согласия на обработку ПДн (152-ФЗ).
// Перед первой записью у пользователя нет consent_at — здесь он выбирает.
func RegConsent() *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().
		AddCallback("Согласен", schemes.POSITIVE, callbacks.RegConsentYes()).
		AddCallback("Отмена", schemes.NEGATIVE, callbacks.RegConsentNo())
	return kb
}

// RegConfirm — финальный экран подтверждения данных перед записью.
func RegConfirm() *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().
		AddCallback("Подтвердить", schemes.POSITIVE, callbacks.RegConfirm()).
		AddCallback("Изменить", schemes.DEFAULT, callbacks.RegEdit())
	kb.AddRow().
		AddCallback("Отменить запись", schemes.NEGATIVE, callbacks.RegCancelDraft())
	return kb
}

// AfterRegistration — клавиатура после успешной записи.
// «Моя запись» позволяет сразу проверить статус, «В главное меню» — выйти.
func AfterRegistration() *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().AddCallback("Моя запись", schemes.DEFAULT, callbacks.MyShow())
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// AfterWaitlist — клавиатура после добавления в лист ожидания.
func AfterWaitlist() *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().AddCallback("Моя запись", schemes.DEFAULT, callbacks.MyShow())
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// MyRegistration — кнопки на экране «Моя запись».
// Если есть активная регистрация (regID != 0) — показываем «Показать QR» и «Отменить».
// «История» доступна всегда.
func MyRegistration(regID int64) *maxbot.Keyboard {
	kb := newKB()
	if regID != 0 {
		kb.AddRow().AddCallback("Показать мой QR", schemes.DEFAULT, callbacks.MyShowQR(regID))
		kb.AddRow().AddCallback("Отменить запись", schemes.NEGATIVE, callbacks.CancelAsk(regID))
	}
	kb.AddRow().AddCallback("История действий", schemes.DEFAULT, callbacks.MyHistory())
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// CancelConfirm — двухшаговое подтверждение отмены конкретной записи.
func CancelConfirm(regID int64) *maxbot.Keyboard {
	return YesNo(
		"Да, отменить", callbacks.CancelYes(regID),
		"Нет, оставить", callbacks.CancelNo(regID),
	)
}

// WaitlistPromote — клавиатура «освободилось место, подтвердить?».
func WaitlistPromote(regID int64) *maxbot.Keyboard {
	return YesNo(
		"Подтвердить", callbacks.WaitlistPromoteYes(regID),
		"Отказаться", callbacks.WaitlistPromoteNo(regID),
	)
}

// ForgetMeConfirm — двухшаговое подтверждение /forget_me.
func ForgetMeConfirm() *maxbot.Keyboard {
	return YesNo(
		"Да, удалить всё", callbacks.ForgetMeYes(),
		"Отмена", callbacks.ForgetMeNo(),
	)
}
