package handlers

// Тесты на приватный валидатор validFullName.
// Поскольку он приватный, тест в том же пакете (без _test suffix).

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
