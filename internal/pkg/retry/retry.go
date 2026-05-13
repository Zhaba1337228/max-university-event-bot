// Package retry реализует экспоненциальный backoff с джиттером.
//
// Используется обёрткой над MAX SDK для 429/5xx ответов и сетевых ошибок,
// см. план §11.10.
package retry

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
)

// Config — настройки backoff.
type Config struct {
	MaxAttempts int           // обычно 4-6
	BaseDelay   time.Duration // обычно 250ms-1s
	MaxDelay    time.Duration // верхний предел для экспоненты
	Jitter      bool          // добавить случайность ±25%
}

// DefaultConfig подходит для MAX Bot API: 5 попыток, от 500ms до 8s, с джиттером.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 5,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    8 * time.Second,
		Jitter:      true,
	}
}

// Do выполняет fn до cfg.MaxAttempts раз; повторяет только при retryable-ошибках
// (см. IsRetryable). При context.Canceled / context.DeadlineExceeded — мгновенно
// возвращает контекстную ошибку.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err
		if !IsRetryable(err) {
			return err
		}
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		delay := computeDelay(cfg, attempt)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}

// IsRetryable определяет, можно ли повторить запрос.
//
// Истина для:
//   - *maxbot.APIError с кодом 429 (rate limit) или 5xx;
//   - *maxbot.NetworkError, *maxbot.TimeoutError;
//   - context.DeadlineExceeded НЕ retryable (отдельная семантика);
//   - всё прочее — false.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	var apiErr *maxbot.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == 429 || apiErr.Code >= 500
	}

	var netErr *maxbot.NetworkError
	if errors.As(err, &netErr) {
		return true
	}

	var toErr *maxbot.TimeoutError
	return errors.As(err, &toErr)
}

// computeDelay возвращает задержку между попытками: base * 2^attempt ± jitter,
// но не больше MaxDelay.
func computeDelay(cfg Config, attempt int) time.Duration {
	d := cfg.BaseDelay << attempt
	if d <= 0 || d > cfg.MaxDelay {
		d = cfg.MaxDelay
	}
	if cfg.Jitter {
		// ±25% случайности.
		// math/rand/v2 здесь безопасен: jitter не используется для криптографии
		// или предсказуемости токенов — это просто разброс времени retry.
		jitter := time.Duration(rand.Int64N(int64(d) / 2)) //nolint:gosec // G404: jitter, не crypto
		d = d - d/4 + jitter
	}
	return d
}
