// Package logger инициализирует структурный логгер на базе log/slog.
//
// Поддерживает два формата (json — для прода, text — для dev) и три уровня.
// Никаких эмодзи, никаких PII (см. план §19.5) — это ответственность
// вызывающих сторон, логгер сам не маскирует.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config — параметры инициализации.
type Config struct {
	Level  string // debug | info | warn | error
	Format string // json | text
}

// New создаёт *slog.Logger с заданным форматом и уровнем.
// При невалидном Level использует info, при невалидном Format — json.
//
// Дополнительно добавляет атрибут "service=bot" по умолчанию, чтобы
// в общем агрегаторе логов было легко фильтровать.
func New(cfg Config) *slog.Logger {
	return NewWithWriter(cfg, os.Stdout)
}

// NewWithWriter — то же, что New, но позволяет указать произвольный io.Writer.
// Полезно для тестов.
func NewWithWriter(cfg Config, w io.Writer) *slog.Logger {
	level := parseLevel(cfg.Level)
	handlerOpts := &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	}

	var h slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "text":
		h = slog.NewTextHandler(w, handlerOpts)
	default:
		h = slog.NewJSONHandler(w, handlerOpts)
	}
	return slog.New(h).With("service", "bot")
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// FromContext возвращает логгер из контекста, либо стандартный slog.Default.
// Зарезервировано для будущих request-scoped логгеров с trace_id.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	v := ctx.Value(loggerKey{})
	if l, ok := v.(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// WithContext возвращает новый контекст с присвоенным логгером.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey{}, l)
}

type loggerKey struct{}
