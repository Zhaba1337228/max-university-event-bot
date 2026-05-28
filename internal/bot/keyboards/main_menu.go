package keyboards

import (
	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
)

// MainMenu — главное меню абитуриента.
func MainMenu() *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().AddCallback("Записаться на мероприятие", schemes.POSITIVE, callbacks.EventListPage(0))
	kb.AddRow().AddCallback("Моя запись", schemes.DEFAULT, callbacks.MyShow())
	kb.AddRow().AddCallback("Подобрать через ИИ", schemes.DEFAULT, callbacks.AIPickStart())
	kb.AddRow().AddCallback("Задать вопрос ИИ", schemes.DEFAULT, callbacks.AIFAQStart())
	kb.AddRow().AddCallback("Помощь", schemes.DEFAULT, callbacks.MainMenu())
	return kb
}
