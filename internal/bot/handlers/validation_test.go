package handlers

// Тесты на приватные валидаторы validFullName / validContact.
// Поскольку они приватные, тест в том же пакете (без _test suffix).

import (
	"strings"
	"testing"
)

func TestValidFullName(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"":                     false,
		"x":                    false,
		"Иван":                 false, // одно слово, без пробела
		"a b":                  false, // короче 5 байт (3 байта)
		"a bc":                 false, // 4 байта
		"a bcd":                true,  // ровно 5 байт, есть пробел
		"Иванов Иван":          true,
		"Иванов Иван Иванович": true,
		"  Иванов  Иван  Иванович  ": true,
		"John Smith":                      true,
		strings.Repeat("ABCDEFGHIJ ", 25): false, // 275 байт > 200
	}
	for in, want := range cases {
		got := validFullName(in)
		if got != want {
			t.Errorf("validFullName(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestValidContact(t *testing.T) {
	t.Parallel()

	cases := map[string]bool{
		"":                         false,
		"x":                        false,
		"name@example.com":         true,
		"a@b.c":                    true,
		"+7 999 123-45-67":         true,
		"+79991234567":             true,
		"79991234567":              true,
		"123456":                   false, // < 7 цифр
		"abc-def":                  false, // ни email, ни телефон
		"name@example":             false, // нет точки
		"phone: 8 (999) 123-45-67": true,
	}
	for in, want := range cases {
		got := validContact(in)
		if got != want {
			t.Errorf("validContact(%q) = %v, want %v", in, got, want)
		}
	}
}
