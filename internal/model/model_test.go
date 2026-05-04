package model

import (
	"testing"
	"time"
)

func TestSessionIsExpired(t *testing.T) {
	t.Run("not expired when expires_at is in the future", func(t *testing.T) {
		s := &Session{
			Username:  "alice",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		if s.IsExpired() {
			t.Error("expected session to NOT be expired, but it was")
		}
	})

	t.Run("expired when expires_at is in the past", func(t *testing.T) {
		s := &Session{
			Username:  "alice",
			CreatedAt: time.Now().Add(-48 * time.Hour),
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		if !s.IsExpired() {
			t.Error("expected session to be expired, but it was not")
		}
	})

	t.Run("expired when expires_at is in the past by 1 nanosecond", func(t *testing.T) {
		s := &Session{
			Username:  "alice",
			CreatedAt: time.Now().Add(-2 * time.Hour),
			ExpiresAt: time.Now().Add(-1 * time.Nanosecond),
		}
		if !s.IsExpired() {
			t.Error("expected session with expires_at in past to be expired")
		}
	})

	t.Run("not expired at boundary edge", func(t *testing.T) {
		s := &Session{
			Username:  "bob",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(1 * time.Millisecond),
		}
		time.Sleep(2 * time.Millisecond)
		if !s.IsExpired() {
			t.Error("expected session with 1ms TTL to be expired after 2ms sleep")
		}
	})

	t.Run("zero value session is expired", func(t *testing.T) {
		var s Session
		if !s.IsExpired() {
			t.Error("expected zero-value session to be expired")
		}
	})
}
