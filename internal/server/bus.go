package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"IM-system/internal/model"
)

// SessionStore manages user sessions for multi-server support.
// The in-memory implementation is local-only; for horizontal scaling,
// implement this with a shared backend (e.g. Redis).
type SessionStore interface {
	Create(username string, ttl time.Duration) (string, error)
	GetUsername(token string) (string, bool)
	UpdateUsername(token, username string)
	Delete(token string)
	Cleanup(expiredBefore time.Time)
	Close() error
}

// MemorySessionStore is the local in-memory session store.
type MemorySessionStore struct {
	mu     sync.RWMutex
	tokens map[string]*model.Session
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		tokens: make(map[string]*model.Session),
	}
}

func (s *MemorySessionStore) generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *MemorySessionStore) Create(username string, ttl time.Duration) (string, error) {
	token, err := s.generateToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = &model.Session{
		Username:  username,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	return token, nil
}

func (s *MemorySessionStore) GetUsername(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.tokens[token]
	if !ok {
		return "", false
	}
	if sess.IsExpired() {
		delete(s.tokens, token)
		return "", false
	}
	return sess.Username, true
}

func (s *MemorySessionStore) UpdateUsername(token, username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.tokens[token]; ok {
		sess.Username = username
	}
}

func (s *MemorySessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, token)
}

func (s *MemorySessionStore) Cleanup(expiredBefore time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, sess := range s.tokens {
		if expiredBefore.After(sess.ExpiresAt) {
			delete(s.tokens, token)
		}
	}
}

func (s *MemorySessionStore) Close() error { return nil }

// BusMessage is a cross-server message distributed via the MessageBus.
type BusMessage struct {
	OriginID string `json:"origin_id"`
	Type     string `json:"type"`    // "public", "private", "group", "system"
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`  // for private: recipient; for group: group name
	Avatar   string `json:"avatar,omitempty"`
	Content  string `json:"content"`
	Time     string `json:"time,omitempty"`
}

// MessageBus handles cross-server message distribution.
// The in-memory implementation delivers only to the local server;
// for multi-server deployment, implement with Redis pub/sub or similar.
type MessageBus interface {
	Publish(msg BusMessage) error
	Receive(ctx context.Context) (BusMessage, error)
	Close() error
}
