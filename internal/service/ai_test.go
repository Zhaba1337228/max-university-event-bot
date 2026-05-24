package service

import "testing"

func TestExtractJSONObject_StripsCodeFences(t *testing.T) {
	t.Parallel()

	raw := "```json\n{\"recommendations\":[{\"event_id\":4,\"title\":\"ДОД\",\"reason\":\"Подходит по интересу к ИТ.\"}]}\n```"
	got := extractJSONObject(raw)
	want := "{\"recommendations\":[{\"event_id\":4,\"title\":\"ДОД\",\"reason\":\"Подходит по интересу к ИТ.\"}]}"
	if got != want {
		t.Fatalf("extractJSONObject() = %q, want %q", got, want)
	}
}

func TestExtractJSONObject_IgnoresPreamble(t *testing.T) {
	t.Parallel()

	raw := "Вот подходящий вариант:\n{\"text\":\"Короткое уведомление\"}\nСпасибо!"
	got := extractJSONObject(raw)
	want := "{\"text\":\"Короткое уведомление\"}"
	if got != want {
		t.Fatalf("extractJSONObject() = %q, want %q", got, want)
	}
}

func TestExtractJSONObject_KeepsBracesInsideString(t *testing.T) {
	t.Parallel()

	raw := "{\"summary\":\"Текст с {фигурными скобками} внутри строки\"}\nЛишний хвост"
	got := extractJSONObject(raw)
	want := "{\"summary\":\"Текст с {фигурными скобками} внутри строки\"}"
	if got != want {
		t.Fatalf("extractJSONObject() = %q, want %q", got, want)
	}
}
