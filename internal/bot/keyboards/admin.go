package keyboards

import (
	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

// AdminLoginLink — клавиатура из одной кнопки-ссылки на magic-link.
// URL вида ${ADMIN_WEB_BASE_URL}/auth?t=<JWT>; JWT короткоживущий (5 мин,
// HS256, purpose=magic).
//
// Используем AddLink, а не AddCallback: пользователь должен сразу попасть
// в браузер на frontend, минуя backend.
func AdminLoginLink(url string) *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().AddLink("Открыть веб-админку", schemes.POSITIVE, url)
	return kb
}
