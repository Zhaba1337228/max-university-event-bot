package ratelimit_test

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/ratelimit"
)

func TestAllowConsumesTokens(t *testing.T) {
	t.Parallel()

	// rate=1/s, burst=3 — три быстрых запроса проходят, четвёртый нет.
	l := ratelimit.New(1, 3, time.Minute)
	for i := 0; i < 3; i++ {
		if !l.Allow("user1") {
			t.Fatalf("request %d expected to pass", i)
		}
	}
	if l.Allow("user1") {
		t.Error("4th request should be rejected")
	}
}

func TestAllowDifferentKeys(t *testing.T) {
	t.Parallel()

	// rate=1/s, burst=1 — у каждого ключа свой счётчик.
	l := ratelimit.New(1, 1, time.Minute)
	if !l.Allow("a") {
		t.Fatal("a: first should pass")
	}
	if !l.Allow("b") {
		t.Fatal("b: first should pass")
	}
	if l.Allow("a") {
		t.Error("a: second should be rejected")
	}
	if l.Allow("b") {
		t.Error("b: second should be rejected")
	}
}

func TestConcurrentSafe(t *testing.T) {
	t.Parallel()

	l := ratelimit.New(1000, 100, time.Minute)
	var wg sync.WaitGroup
	wg.Add(50)
	for i := 0; i < 50; i++ {
		go func(idx int) {
			defer wg.Done()
			key := "user" + strconv.Itoa(idx%5)
			for j := 0; j < 20; j++ {
				l.Allow(key)
			}
		}(i)
	}
	wg.Wait()
	// Главное — не race, не паника, не stuck.
}

func TestSize(t *testing.T) {
	t.Parallel()

	l := ratelimit.New(1, 1, time.Minute)
	l.Allow("a")
	l.Allow("b")
	l.Allow("c")
	if got := l.Size(); got != 3 {
		t.Errorf("Size: want 3, got %d", got)
	}
	l.Reset()
	if got := l.Size(); got != 0 {
		t.Errorf("Size after reset: want 0, got %d", got)
	}
}
