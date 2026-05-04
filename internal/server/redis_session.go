package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisSessionStore implements SessionStore using Redis as the backend,
// enabling session sharing across multiple server instances.
type RedisSessionStore struct {
	client *redis.Client
}

func NewRedisSessionStore(addr, password string, db int) (*RedisSessionStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &RedisSessionStore{client: client}, nil
}

func (s *RedisSessionStore) Create(username string, ttl time.Duration) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	err = s.client.Set(ctx, token, username, ttl).Err()
	if err != nil {
		return "", fmt.Errorf("redis set failed: %w", err)
	}
	return token, nil
}

func (s *RedisSessionStore) GetUsername(token string) (string, bool) {
	ctx := context.Background()
	val, err := s.client.Get(ctx, token).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

func (s *RedisSessionStore) UpdateUsername(token, username string) {
	ctx := context.Background()
	// Get current TTL, then set with same TTL
	ttl := s.client.TTL(ctx, token).Val()
	if ttl > 0 {
		s.client.Set(ctx, token, username, ttl)
	} else if ttl == -1 {
		// Key exists but no expiry — set without expiry
		s.client.Set(ctx, token, username, 0)
	}
}

func (s *RedisSessionStore) Delete(token string) {
	ctx := context.Background()
	s.client.Del(ctx, token)
}

// Cleanup is a no-op for Redis — expired keys are automatically removed by Redis.
func (s *RedisSessionStore) Cleanup(expiredBefore time.Time) {}

func (s *RedisSessionStore) Close() error {
	return s.client.Close()
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
