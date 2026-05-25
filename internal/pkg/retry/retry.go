// Package retry realizes exponential backoff with jitter.
//
// It is used by the MAX SDK wrapper for 429/5xx responses and transient network errors.
package retry

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"math/big"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"
)

// Config controls backoff behavior.
type Config struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      bool
}

// DefaultConfig is tuned for MAX Bot API.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 5,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    8 * time.Second,
		Jitter:      true,
	}
}

// Do retries fn up to cfg.MaxAttempts times while errors stay retryable.
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

// IsRetryable reports whether a request should be retried.
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

	var timeoutErr *maxbot.TimeoutError
	return errors.As(err, &timeoutErr)
}

// computeDelay returns base * 2^attempt with optional +-25% jitter, capped by MaxDelay.
func computeDelay(cfg Config, attempt int) time.Duration {
	d := cfg.BaseDelay << attempt
	if d <= 0 || d > cfg.MaxDelay {
		d = cfg.MaxDelay
	}
	if cfg.Jitter {
		jitter := time.Duration(cryptoRandN(int64(d) / 2))
		d = d - d/4 + jitter
	}
	return d
}

func cryptoRandN(limit int64) int64 {
	if limit <= 1 {
		return 0
	}

	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(limit))
	if err != nil {
		return 0
	}
	return n.Int64()
}
