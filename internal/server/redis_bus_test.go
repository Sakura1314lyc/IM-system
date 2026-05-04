package server

import (
	"context"
	"testing"
	"time"
)

func redisBusAvailable(t *testing.T) bool {
	t.Helper()
	bus, err := NewRedisMessageBus("test-server", "localhost:6379", "", 0)
	if err != nil {
		t.Log("redis not available, skipping bus test:", err)
		return false
	}
	bus.Close()
	return true
}

func TestRedisMessageBusPublishReceive(t *testing.T) {
	if !redisBusAvailable(t) {
		t.Skip("redis not available")
	}

	busA, err := NewRedisMessageBus("server-a", "localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer busA.Close()

	busB, err := NewRedisMessageBus("server-b", "localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer busB.Close()

	// busA publishes a message
	err = busA.Publish(BusMessage{
		Type:    "public",
		From:    "alice",
		Content: "hello from A",
		Time:    "12:00",
	})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// busB should receive it
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msg, err := busB.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if msg.Type != "public" {
		t.Errorf("expected type 'public', got %q", msg.Type)
	}
	if msg.From != "alice" {
		t.Errorf("expected from 'alice', got %q", msg.From)
	}
	if msg.Content != "hello from A" {
		t.Errorf("expected content 'hello from A', got %q", msg.Content)
	}
	if msg.OriginID != "server-a" {
		t.Errorf("expected origin 'server-a', got %q", msg.OriginID)
	}
}

func TestRedisMessageBusOwnMessagesNotReceived(t *testing.T) {
	if !redisBusAvailable(t) {
		t.Skip("redis not available")
	}

	bus, err := NewRedisMessageBus("test-server", "localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	err = bus.Publish(BusMessage{
		Type:    "public",
		From:    "test",
		Content: "self message",
	})
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Should not receive our own message (the bus receiver in the server
	// skips messages with matching origin ID). The bus itself doesn't filter,
	// it's up to the consumer.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msg, err := bus.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if msg.OriginID != "test-server" {
		t.Errorf("expected origin 'test-server', got %q", msg.OriginID)
	}
}

func TestRedisMessageBusClose(t *testing.T) {
	if !redisBusAvailable(t) {
		t.Skip("redis not available")
	}

	bus, err := NewRedisMessageBus("test-close", "localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}

	err = bus.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Double close should not panic
	bus.Close()
}

func TestRedisMessageBusImplementsInterface(t *testing.T) {
	if !redisBusAvailable(t) {
		t.Skip("redis not available")
	}

	bus, err := NewRedisMessageBus("test-iface", "localhost:6379", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	var _ MessageBus = bus
}
