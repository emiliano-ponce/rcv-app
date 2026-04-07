package security

import (
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	count   int
	resetAt time.Time
}

// RateLimiter provides a fixed-window, in-memory limiter keyed by caller identity.
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]bucket
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]bucket),
	}
}

func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.entries[key]
	if !ok || now.After(b.resetAt) {
		l.entries[key] = bucket{count: 1, resetAt: now.Add(l.window)}
		return true
	}

	if b.count >= l.limit {
		return false
	}

	b.count++
	l.entries[key] = b
	return true
}

func WrapWithRateLimit(limiter *RateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := ClientIP(r)
		if !limiter.Allow(key) {
			http.Error(w, "too many requests, please try again later", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
