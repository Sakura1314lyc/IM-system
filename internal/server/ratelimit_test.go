package server

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	t.Run("first request is allowed", func(t *testing.T) {
		rl := newRateLimiter(5, time.Minute)
		if !rl.Allow("test-key") {
			t.Error("expected first request to be allowed")
		}
	})

	t.Run("requests within limit are allowed", func(t *testing.T) {
		rl := newRateLimiter(3, time.Minute)
		for i := 0; i < 3; i++ {
			if !rl.Allow("test-key") {
				t.Errorf("expected request %d to be allowed", i+1)
			}
		}
	})

	t.Run("requests exceeding limit are blocked", func(t *testing.T) {
		rl := newRateLimiter(2, time.Minute)
		rl.Allow("test-key")
		rl.Allow("test-key")
		if rl.Allow("test-key") {
			t.Error("expected third request to be blocked")
		}
	})

	t.Run("counter resets after window expires", func(t *testing.T) {
		rl := newRateLimiter(1, 50*time.Millisecond)
		if !rl.Allow("reset-key") {
			t.Fatal("expected first request to be allowed")
		}
		if rl.Allow("reset-key") {
			t.Fatal("expected second request to be blocked within window")
		}
		time.Sleep(60 * time.Millisecond)
		if !rl.Allow("reset-key") {
			t.Error("expected request to be allowed after window reset")
		}
	})

	t.Run("different keys are independent", func(t *testing.T) {
		rl := newRateLimiter(1, time.Minute)
		if !rl.Allow("key-a") {
			t.Error("expected key-a first request to be allowed")
		}
		if rl.Allow("key-a") {
			t.Error("expected key-a second request to be blocked")
		}
		if !rl.Allow("key-b") {
			t.Error("expected key-b first request to be allowed even though key-a is blocked")
		}
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		rl := newRateLimiter(100, time.Minute)
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					rl.Allow("concurrent-key")
				}
			}()
		}
		wg.Wait()
		// Total allowed should be 100, the 101st should be blocked
		if rl.Allow("concurrent-key") {
			t.Error("expected request beyond limit to be blocked after concurrent access")
		}
	})

	t.Run("exhausted then reset then exhaust again", func(t *testing.T) {
		rl := newRateLimiter(1, 30*time.Millisecond)
		rl.Allow("cycle-key")
		rl.Allow("cycle-key") // blocked

		time.Sleep(40 * time.Millisecond)
		if !rl.Allow("cycle-key") {
			t.Error("expected request to be allowed after first reset")
		}
		// second window should also block after limit
		if rl.Allow("cycle-key") {
			t.Error("expected request to be blocked in second window after exhausting")
		}
	})

	t.Run("zero limit allows first then blocks", func(t *testing.T) {
		rl := newRateLimiter(0, time.Minute)
		// First request always creates entry and returns true
		if !rl.Allow("zero-key") {
			t.Error("expected first request to be allowed (creates entry)")
		}
		// Second request: count becomes 1, limit is 0 → blocked
		if rl.Allow("zero-key") {
			t.Error("expected second request to be blocked with zero limit")
		}
	})
}

func TestRateLimiterEdgeCases(t *testing.T) {
	t.Run("empty key is valid", func(t *testing.T) {
		rl := newRateLimiter(5, time.Minute)
		if !rl.Allow("") {
			t.Error("expected empty key to be allowed")
		}
	})

	t.Run("large limit does not overflow", func(t *testing.T) {
		rl := newRateLimiter(10000, time.Minute)
		for i := 0; i < 10000; i++ {
			rl.Allow("large-key")
		}
		if rl.Allow("large-key") {
			t.Error("expected request beyond large limit to be blocked")
		}
	})
}
