// Package secret предоставляет утилиты для безопасной работы с
// чувствительными значениями: маскировка в логах, безопасное
// сравнение, тип-обёртка с custom String().
//
// Цель — закрыть требование 19.1 плана: «НИКОГДА не пиши токены в код,
// тесты, commit message, лог-строки, error message».
package secret

import (
	"crypto/subtle"
	"strings"
)

// Mask возвращает безопасное для логов представление секрета.
//
// Формат: первые 4 символа + "***" + последние 4 символа.
// Если секрет короче 12 символов — возвращает "***" (показывать
// «короткие» секреты в любом виде небезопасно).
//
// Пустой вход возвращает пустую строку — это удобно для conditional
// логирования «секрет настроен / не настроен».
func Mask(s string) string {
	if s == "" {
		return ""
	}
	const minVisible = 12 // 4 head + 4 tail + 3 stars (запас)
	if len(s) < minVisible {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

// MaskHeader маскирует значение HTTP заголовка вида "Bearer abc...".
// Префикс (Bearer/Basic/Token) сохраняется, тело маскируется через Mask.
func MaskHeader(value string) string {
	if value == "" {
		return ""
	}
	for _, prefix := range []string{"Bearer ", "Basic ", "Token "} {
		if strings.HasPrefix(value, prefix) {
			return prefix + Mask(strings.TrimPrefix(value, prefix))
		}
	}
	return Mask(value)
}

// ConstantTimeEqual выполняет сравнение двух строк за константное время.
// Используется для проверки webhook secret (X-Max-Bot-Api-Secret) —
// см. план 19.3: «crypto/subtle.ConstantTimeCompare. Никогда `==`.».
func ConstantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
