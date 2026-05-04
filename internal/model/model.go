package model

import "time"

// DBUser is the internal user representation (contains password hash, never expose directly).
type DBUser struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"`
	Avatar    string    `json:"avatar"`
	Gender    string    `json:"gender"`
	Signature string    `json:"signature"`
	CreatedAt time.Time `json:"created_at"`
}

// UserPublic is the safe user representation returned to callers (no password).
type UserPublic struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Avatar    string    `json:"avatar"`
	Gender    string    `json:"gender"`
	Signature string    `json:"signature"`
	CreatedAt time.Time `json:"created_at"`
}

type DBGroup struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	CreatorID   int       `json:"creator_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type DBGroupMember struct {
	ID      int `json:"id"`
	GroupID int `json:"group_id"`
	UserID  int `json:"user_id"`
}

type DBMessage struct {
	ID        int       `json:"id"`
	FromID    int       `json:"from_id"`
	ToID      *int      `json:"to_id,omitempty"`
	GroupID   *int      `json:"group_id,omitempty"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type DBMessageExt struct {
	ID        int       `json:"id"`
	From      string    `json:"from"`
	Avatar    string    `json:"avatar"`
	Signature string    `json:"signature"`
	To        string    `json:"to,omitempty"`
	Group     string    `json:"group,omitempty"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type WebClient struct {
	Name   string
	C      chan string
	Avatar string
}

type Session struct {
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}


