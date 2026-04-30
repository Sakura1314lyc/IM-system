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

	now := time.Now()
	entry, ok := rl.counters[key]
	if !ok || now.After(entry.reset) {
		rl.counters[key] = &rateEntry{count: 1, reset: now.Add(rl.window)}
		return true
	}

	entry.count++
	return entry.count <= rl.limit
}
