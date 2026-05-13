// Package ratelimit реализует простой token-bucket лимитер per-ключ.
//
// Используется для:
//   - per-user лимита входящих сообщений и callback'ов от MAX SDK
//     (защита от flood и от случайного зацикливания клиента);
//   - per-IP лимита на /api/auth/exchange в admin REST API.
//
// Внутренняя map[key]*bucket с TTL-эвикцией старых ключей.
// Без внешних зависимостей; для production-нагрузки 1k+ qps лучше
// использовать golang.org/x/time/rate с явным GC.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter — потокобезопасный per-key token bucket.
type Limiter struct {
	rate  float64 // tokens per second
	burst float64 // max tokens
	ttl   time.Duration
	now   func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens   float64
	lastFill time.Time
	lastUse  time.Time
}

// New создаёт лимитер.
//
//	rate — сколько токенов восстанавливается в секунду;
//	burst — пиковая ёмкость (≥ rate);
//	ttl — за сколько секунд неиспользования ключ удаляется из map.
func New(rate, burst float64, ttl time.Duration) *Limiter {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Limiter{
		rate:    rate,
		burst:   burst,
		ttl:     ttl,
		now:     time.Now,
		buckets: make(map[string]*bucket),
	}
}

// Allow проверяет, можно ли пропустить запрос для key.
// Возвращает true если есть свободный токен (и потребляет его), иначе false.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, lastFill: now}
		l.buckets[key] = b
	} else {
		// Пополнение токенов с момента lastFill.
		elapsed := now.Sub(b.lastFill).Seconds()
		b.tokens += elapsed * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.lastFill = now
	}
	b.lastUse = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--

	// Ленивая чистка: при каждом вызове с шансом ~1/256 удаляем устаревшие
	// ключи. Это амортизирует cost эвикции без отдельной горутины.
	if len(l.buckets) > 64 && (now.UnixNano()&0xff) == 0 {
		cutoff := now.Add(-l.ttl)
		for k, v := range l.buckets {
			if v.lastUse.Before(cutoff) {
				delete(l.buckets, k)
			}
		}
	}
	return true
}

// Reset очищает все бакеты. Используется в тестах.
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buckets = make(map[string]*bucket)
}

// Size возвращает число активных ключей (для метрик).
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
