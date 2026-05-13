// Package webhook реализует HTTP-сервер для приёма обновлений MAX через webhook.
//
// Альтернатива long-polling (`internal/transport/longpoll`). Включается через
// MAX_BOT_MODE=webhook + MAX_BOT_WEBHOOK_URL + MAX_BOT_WEBHOOK_SECRET.
package webhook

import (
	"encoding/json"
	"fmt"

	"github.com/max-messenger/max-bot-api-client-go/schemes"
)

// ParseUpdate разбирает входящий webhook JSON в типизированный
// schemes.UpdateInterface.
//
// Дублирует приватный SDK-метод `bytesToProperUpdate` (см. deviations §X):
// сначала парсим базовый Update чтобы узнать update_type, затем повторно
// в нужный конкретный тип.
//
// Список типов соответствует тем, что бот реально обрабатывает в
// dispatcher.go. Остальные апдейты возвращаются как nil — диспетчер их
// проигнорирует через default-ветку switch.
func ParseUpdate(data []byte) (schemes.UpdateInterface, error) {
	base := &schemes.Update{}
	if err := json.Unmarshal(data, base); err != nil {
		return nil, fmt.Errorf("parse base update: %w", err)
	}

	switch base.GetUpdateType() {
	case schemes.TypeMessageCreated:
		u := &schemes.MessageCreatedUpdate{}
		if err := json.Unmarshal(data, u); err != nil {
			return nil, fmt.Errorf("unmarshal message_created: %w", err)
		}
		return u, nil
	case schemes.TypeMessageCallback:
		u := &schemes.MessageCallbackUpdate{}
		if err := json.Unmarshal(data, u); err != nil {
			return nil, fmt.Errorf("unmarshal message_callback: %w", err)
		}
		return u, nil
	case schemes.TypeBotStarted:
		u := &schemes.BotStartedUpdate{}
		if err := json.Unmarshal(data, u); err != nil {
			return nil, fmt.Errorf("unmarshal bot_started: %w", err)
		}
		return u, nil
	case schemes.TypeMessageEdited:
		u := &schemes.MessageEditedUpdate{}
		if err := json.Unmarshal(data, u); err != nil {
			return nil, fmt.Errorf("unmarshal message_edited: %w", err)
		}
		return u, nil
	case schemes.TypeMessageRemoved:
		u := &schemes.MessageRemovedUpdate{}
		if err := json.Unmarshal(data, u); err != nil {
			return nil, fmt.Errorf("unmarshal message_removed: %w", err)
		}
		return u, nil
	}
	// Прочие типы (chat_title_changed, bot_added, user_added и т.д.)
	// нам сейчас не нужны — возвращаем nil без ошибки, диспетчер пропустит.
	return nil, nil
}
