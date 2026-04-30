package server

import (
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	counters map[string]*rateEntry
	limit    int
	window   time.Duration
}

type rateEntry struct {
	count int
	reset time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		counters: make(map[string]*rateEntry),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.limit == 0 {
		return false
	}

	now := time.Now()
	entry, ok := rl.counters[key]
	if !ok || now.After(entry.reset) {
		// Cleanup expired entries periodically
		if len(rl.counters) > 1000 && len(rl.counters)%100 == 0 {
			for k, e := range rl.counters {
				if now.After(e.reset) {
					delete(rl.counters, k)
				}
			}
		}
		rl.counters[key] = &rateEntry{count: 1, reset: now.Add(rl.window)}
		return true
	}

	entry.count++
	return entry.count <= rl.limit
}
