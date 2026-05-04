package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const busChannel = "lchat:messages"

// RedisMessageBus implements MessageBus using Redis pub/sub,
// enabling cross-server message distribution.
type RedisMessageBus struct {
	client   *redis.Client
	pubsub   *redis.PubSub
	serverID string
}

func NewRedisMessageBus(serverID, addr, password string, db int) (*RedisMessageBus, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis bus ping failed: %w", err)
	}
	pubsub := client.Subscribe(ctx, busChannel)
	// Wait for subscription to be active
	if _, err := pubsub.Receive(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis bus subscribe failed: %w", err)
	}
	return &RedisMessageBus{client: client, pubsub: pubsub, serverID: serverID}, nil
}

func (b *RedisMessageBus) Publish(msg BusMessage) error {
	msg.OriginID = b.serverID
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("bus marshal error: %w", err)
	}
	ctx := context.Background()
	return b.client.Publish(ctx, busChannel, data).Err()
}

func (b *RedisMessageBus) Receive(ctx context.Context) (BusMessage, error) {
	msg, err := b.pubsub.ReceiveMessage(ctx)
	if err != nil {
		return BusMessage{}, err
	}
	var busMsg BusMessage
	if err := json.Unmarshal([]byte(msg.Payload), &busMsg); err != nil {
		return BusMessage{}, fmt.Errorf("bus unmarshal error: %w", err)
	}
	return busMsg, nil
}

func (b *RedisMessageBus) Close() error {
	if err := b.pubsub.Close(); err != nil {
		b.client.Close()
		return err
	}
	return b.client.Close()
}
