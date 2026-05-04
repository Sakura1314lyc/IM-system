package server

import (
	"testing"
	"time"
)

func redisAvailable(t *testing.T) bool {
	t.Helper()
	store, err := NewRedisSessionStore("localhost:6379", "", 0)
	if err != nil {
		t.Log("redis not available, skipping test:", err)
		return false
	}
	store.Close()
	return true
}

func TestRedisSessionStoreCreateAndGet(t *testing.T) {
	if !redisAvailable(t) {
		t.Skip("redis not available")
	}

	store, err := NewRedisSessionStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	token, err := store.Create("alice", 5*time.Minute)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	username, ok := store.GetUsername(token)
	if !ok {
		t.Fatal("expected token to be found")
	}
	if username != "alice" {
		t.Errorf("expected 'alice', got %q", username)
	}

	// Non-existent token
	_, ok = store.GetUsername("nonexistent-token")
	if ok {
		t.Error("expected nonexistent token to return false")
	}
}

func TestRedisSessionStoreUpdateAndDelete(t *testing.T) {
	if !redisAvailable(t) {
		t.Skip("redis not available")
	}

	store, err := NewRedisSessionStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	token, err := store.Create("bob", 5*time.Minute)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update username
	store.UpdateUsername(token, "bob_renamed")
	username, ok := store.GetUsername(token)
	if !ok {
		t.Fatal("expected token to be found after rename")
	}
	if username != "bob_renamed" {
		t.Errorf("expected 'bob_renamed', got %q", username)
	}

	// Delete
	store.Delete(token)
	_, ok = store.GetUsername(token)
	if ok {
		t.Error("expected token to be gone after delete")
	}

	// Double delete should not panic
	store.Delete(token)
}

func TestRedisSessionStoreTTL(t *testing.T) {
	if !redisAvailable(t) {
		t.Skip("redis not available")
	}

	store, err := NewRedisSessionStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Create with 1-second TTL
	token, err := store.Create("charlie", 1*time.Second)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Should exist immediately
	_, ok := store.GetUsername(token)
	if !ok {
		t.Fatal("expected token to exist immediately")
	}

	// Wait for TTL to expire
	time.Sleep(1500 * time.Millisecond)

	// Should be gone
	_, ok = store.GetUsername(token)
	if ok {
		t.Error("expected token to expire after TTL")
	}
}

func TestRedisSessionStoreImplementsInterface(t *testing.T) {
	if !redisAvailable(t) {
		t.Skip("redis not available")
	}

	store, err := NewRedisSessionStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Compile-time interface check
	var _ SessionStore = store
}

func TestRedisSessionStoreCleanupNoop(t *testing.T) {
	if !redisAvailable(t) {
		t.Skip("redis not available")
	}

	store, err := NewRedisSessionStore("localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Cleanup should not panic (no-op for Redis)
	store.Cleanup(time.Now())
}
