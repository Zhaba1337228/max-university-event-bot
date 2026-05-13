// Package keyboards собирает inline-клавиатуры MAX-бота.
//
// Каждый конструктор возвращает *maxbot.Keyboard. По плану раздел 16.2
// возвращаемый тип назван KeyboardBuilder, но реальный тип SDK v1.6.17
// — Keyboard (см. docs/deviations.md §3).
//
// Конструкторы принимают только данные, нужные для логики (id событий,
// текущая страница и т.п.). Параметр *maxbot.Api НЕ требуется: можно
// создать клавиатуру вне контекста http-клиента (важно для тестов).
package keyboards

import (
	maxbot "github.com/max-messenger/max-bot-api-client-go"
	"github.com/max-messenger/max-bot-api-client-go/schemes"

	"github.com/Zhaba1337228/max-university-event-bot/internal/bot/callbacks"
)

// newKB — фабрика пустой клавиатуры. Вынесена, чтобы при необходимости
// поменять SDK-конструктор (например, добавить middleware) — в одном месте.
func newKB() *maxbot.Keyboard {
	return &maxbot.Keyboard{}
}

// BackToMain — одна кнопка «В главное меню».
func BackToMain() *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().AddCallback("В главное меню", schemes.NEGATIVE, callbacks.MainMenu())
	return kb
}

// YesNo — двухкнопочная клавиатура с произвольными payload'ами.
// Используется для двухшаговых подтверждений (отмена, /forget_me, закрытие).
func YesNo(yesText, yesPayload, noText, noPayload string) *maxbot.Keyboard {
	kb := newKB()
	kb.AddRow().
		AddCallback(yesText, schemes.POSITIVE, yesPayload).
		AddCallback(noText, schemes.NEGATIVE, noPayload)
	return kb
}
