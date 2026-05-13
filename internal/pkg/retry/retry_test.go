package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	maxbot "github.com/max-messenger/max-bot-api-client-go"

	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/retry"
)

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain", errors.New("oops"), false},
		{"context.Canceled", context.Canceled, false},
		{"context.DeadlineExceeded", context.DeadlineExceeded, false},
		{"api 400", &maxbot.APIError{Code: 400}, false},
		{"api 401", &maxbot.APIError{Code: 401}, false},
		{"api 404", &maxbot.APIError{Code: 404}, false},
		{"api 429", &maxbot.APIError{Code: 429}, true},
		{"api 500", &maxbot.APIError{Code: 500}, true},
		{"api 502", &maxbot.APIError{Code: 502}, true},
		{"network", &maxbot.NetworkError{Op: "get", Err: errors.New("eof")}, true},
		{"timeout", &maxbot.TimeoutError{Op: "send"}, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := retry.IsRetryable(tc.err); got != tc.want {
				t.Errorf("IsRetryable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestDoSuccessFirstTry(t *testing.T) {
	t.Parallel()
	cfg := retry.Config{MaxAttempts: 3, BaseDelay: time.Millisecond}
	calls := 0
	err := retry.Do(context.Background(), cfg, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDoSuccessAfterRetry(t *testing.T) {
	t.Parallel()
	cfg := retry.Config{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	calls := 0
	err := retry.Do(context.Background(), cfg, func() error {
		calls++
		if calls < 3 {
			return &maxbot.APIError{Code: 502}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestDoStopsOnNonRetryable(t *testing.T) {
	t.Parallel()
	cfg := retry.Config{MaxAttempts: 5, BaseDelay: time.Millisecond}
	calls := 0
	want := &maxbot.APIError{Code: 401}
	err := retry.Do(context.Background(), cfg, func() error {
		calls++
		return want
	})
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 401)", calls)
	}
	if !errors.Is(err, want) {
		t.Errorf("err: want %v, got %v", want, err)
	}
}

func TestDoExhaustsAttempts(t *testing.T) {
	t.Parallel()
	cfg := retry.Config{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
	calls := 0
	err := retry.Do(context.Background(), cfg, func() error {
		calls++
		return &maxbot.APIError{Code: 503}
	})
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
	if err == nil {
		t.Error("want error after exhaustion")
	}
}

func TestDoRespectsContext(t *testing.T) {
	t.Parallel()
	cfg := retry.Config{MaxAttempts: 10, BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // отменяем сразу

	err := retry.Do(ctx, cfg, func() error {
		return &maxbot.APIError{Code: 502}
	})
	if !errors.Is(err, context.Canceled) && err == nil {
		t.Errorf("want canceled error, got %v", err)
	}
}
