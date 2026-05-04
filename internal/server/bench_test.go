package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"IM-system/internal/config"
)

func BenchmarkBroadcastFanout(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.Server.Port = 0
	cfg.Web.Addr = ":0"
	cfg.DB.Path = ":memory:"
	s, err := New(cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer s.Shutdown()

	// Add N TCP users
	const numUsers = 100
	for i := range numUsers {
		name := fmt.Sprintf("user-%d", i)
		ch := make(chan string, 100)
		s.OnlineMap[name] = &User{
			Name: name,
			Addr: fmt.Sprintf("127.0.0.1:%d", 10000+i),
			C:    ch,
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.fanoutTCP("benchmark test message", "bench")
		}
	})
}

func BenchmarkCreateSession(b *testing.B) {
	store := NewMemorySessionStore()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			token, err := store.Create("user", time.Hour)
			if err != nil {
				b.Fatal(err)
			}
			store.Delete(token)
		}
	})
}

func BenchmarkSanitizeInput(b *testing.B) {
	inputs := []string{
		"hello world",
		"<script>alert('xss')</script>",
		"<b>bold</b> and <i>italic</i>",
		strings.Repeat("a", 1000),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sanitizeInputWithLimit(inputs[i%len(inputs)], 500)
	}
}

func BenchmarkRateLimiter(b *testing.B) {
	rl := newRateLimiter(100, time.Minute)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.Allow("bench-key")
		}
	})
}
