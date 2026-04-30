package server

import (
	"testing"
	"time"

	"IM-system/internal/config"
	"IM-system/internal/model"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Server.Port = 0
	cfg.Web.Addr = ":0"
	cfg.DB.Path = ":memory:"
	cfg.Server.TLS = false
	s := New(cfg)
	return s
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "strip HTML tags",
			input: "hello <script>alert('xss')</script> world",
			want:  "hello alert('xss') world",
		},
		{
			name:  "strip nested tags",
			input: "<div><span>nested</span></div>",
			want:  "nested",
		},
		{
			name:  "strip tags with attributes",
			input: "<a href=\"http://evil.com\">click me</a>",
			want:  "click me",
		},
		{
			name:  "truncate long input",
			input: stringOfLen('a', 600),
			want:  stringOfLen('a', 500) + "...",
		},
		{
			name:  "truncate with tags stripped first",
			input: "<b>" + stringOfLen('x', 600) + "</b>",
			want:  stringOfLen('x', 500) + "...",
		},
		{
			name:  "trim whitespace",
			input: "  hello world  ",
			want:  "hello world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace",
			input: "   ",
			want:  "",
		},
		{
			name:  "only HTML tags",
			input: "<br><hr><input>",
			want:  "",
		},
		{
			name:  "malformed tag",
			input: "hello <world",
			want:  "hello <world",
		},
		{
			name:  "self-closing tag",
			input: "line1<br/>line2",
			want:  "line1line2",
		},
		{
			name:  "text within limit is not truncated",
			input: stringOfLen('b', 500),
			want:  stringOfLen('b', 500),
		},
		{
			name:  "exactly at limit is not truncated",
			input: stringOfLen('c', 500),
			want:  stringOfLen('c', 500),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeInput(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func stringOfLen(ch byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}

func TestCreateSession(t *testing.T) {
	s := testServer(t)

	t.Run("create and retrieve session", func(t *testing.T) {
		token, err := s.CreateSession("alice")
		if err != nil {
			t.Fatalf("expected CreateSession to succeed, got: %v", err)
		}
		if token == "" {
			t.Fatal("expected non-empty token")
		}

		username, ok := s.GetUsernameByToken(token)
		if !ok {
			t.Fatal("expected token to be valid")
		}
		if username != "alice" {
			t.Errorf("expected username 'alice', got %q", username)
		}
	})

	t.Run("token is unique each time", func(t *testing.T) {
		t1, _ := s.CreateSession("alice")
		t2, _ := s.CreateSession("alice")
		if t1 == t2 {
			t.Error("expected different tokens for sequential CreateSession calls")
		}
	})

	t.Run("token is hex-encoded and 32 chars", func(t *testing.T) {
		token, _ := s.CreateSession("bob")
		// 16 bytes = 32 hex chars
		if len(token) != 32 {
			t.Errorf("expected token length 32, got %d", len(token))
		}
		for _, c := range token {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("unexpected character %c in hex token", c)
				break
			}
		}
	})
}

func TestGetUsernameByToken(t *testing.T) {
	s := testServer(t)

	t.Run("non-existent token", func(t *testing.T) {
		_, ok := s.GetUsernameByToken("non-existent-token")
		if ok {
			t.Error("expected non-existent token to return false")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, ok := s.GetUsernameByToken("")
		if ok {
			t.Error("expected empty token to return false")
		}
	})

	t.Run("expired token returns false", func(t *testing.T) {
		s.sessionLock.Lock()
		s.SessionTokens["expired-token"] = &model.Session{
			Username:  "alice",
			CreatedAt: time.Now().Add(-48 * time.Hour),
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}
		s.sessionLock.Unlock()

		_, ok := s.GetUsernameByToken("expired-token")
		if ok {
			t.Error("expected expired token to return false")
		}
	})
}

func TestDeleteSession(t *testing.T) {
	s := testServer(t)

	token, _ := s.CreateSession("alice")

	// Verify token exists
	_, ok := s.GetUsernameByToken(token)
	if !ok {
		t.Fatal("expected token to exist before deletion")
	}

	s.DeleteSession(token)

	// Verify token is gone
	_, ok = s.GetUsernameByToken(token)
	if ok {
		t.Error("expected token to be gone after deletion")
	}

	// Deleting again should not panic
	s.DeleteSession(token)
}

func TestUpdateSessionUsername(t *testing.T) {
	s := testServer(t)

	t.Run("update existing session", func(t *testing.T) {
		token, _ := s.CreateSession("alice")
		s.UpdateSessionUsername(token, "alice_renamed")

		username, ok := s.GetUsernameByToken(token)
		if !ok {
			t.Fatal("expected token to still be valid")
		}
		if username != "alice_renamed" {
			t.Errorf("expected 'alice_renamed', got %q", username)
		}
	})

	t.Run("update non-existent session does not panic", func(t *testing.T) {
		s.UpdateSessionUsername("non-existent", "someone")
	})
}

func TestIsNameTaken(t *testing.T) {
	s := testServer(t)

	t.Run("name not taken initially", func(t *testing.T) {
		if s.IsNameTaken("someone-unique") {
			t.Error("expected unique name to not be taken")
		}
	})

	t.Run("name taken by TCP user", func(t *testing.T) {
		s.mapLock.Lock()
		s.OnlineMap["tcp-user"] = &User{Name: "tcp-user"}
		s.mapLock.Unlock()

		if !s.IsNameTaken("tcp-user") {
			t.Error("expected tcp-user name to be taken")
		}
	})

	t.Run("name taken by web client", func(t *testing.T) {
		s.wsLock.Lock()
		s.WSConns["web-user"] = &WSClient{Name: "web-user", C: make(chan string, 1)}
		s.wsLock.Unlock()

		if !s.IsNameTaken("web-user") {
			t.Error("expected web-user name to be taken")
		}
	})
}

func TestGetOnlineUsers(t *testing.T) {
	s := testServer(t)

	t.Run("no online users initially", func(t *testing.T) {
		users := s.GetOnlineUsers()
		if len(users) != 0 {
			t.Errorf("expected 0 online users, got %d", len(users))
		}
	})

	t.Run("returns TCP and web users", func(t *testing.T) {
		s.mapLock.Lock()
		s.OnlineMap["tcp-user"] = &User{Name: "tcp-user", Avatar: "🖥️"}
		s.mapLock.Unlock()

		s.wsLock.Lock()
		s.WSConns["web-user"] = &WSClient{Name: "web-user", Avatar: "🌐", C: make(chan string, 1)}
		s.wsLock.Unlock()

		users := s.GetOnlineUsers()
		if len(users) != 2 {
			t.Fatalf("expected 2 online users, got %d: %v", len(users), users)
		}

		// Verify both users are present
		names := make(map[string]string)
		for _, u := range users {
			names[u["name"]] = u["avatar"]
		}
		if names["tcp-user"] != "🖥️" {
			t.Errorf("expected tcp-user with 🖥️ avatar")
		}
		if names["web-user"] != "🌐" {
			t.Errorf("expected web-user with 🌐 avatar")
		}
	})
}

func TestRenameWSClient(t *testing.T) {
	s := testServer(t)

	s.wsLock.Lock()
	s.WSConns["old-name"] = &WSClient{Name: "old-name", C: make(chan string, 1)}
	s.wsLock.Unlock()

	t.Run("rename web client", func(t *testing.T) {
		err := s.RenameWSClient("old-name", "new-name")
		if err != nil {
			t.Fatalf("expected rename to succeed, got: %v", err)
		}

		if s.IsNameTaken("old-name") {
			t.Error("expected old name to be available after rename")
		}
		if !s.IsNameTaken("new-name") {
			t.Error("expected new name to be taken after rename")
		}
	})

	t.Run("rename non-existent client fails", func(t *testing.T) {
		err := s.RenameWSClient("non-existent", "whatever")
		if err == nil {
			t.Error("expected error when renaming non-existent client")
		}
	})
}

func TestBroadcastToGroupNoMembers(t *testing.T) {
	s := testServer(t)

	err := s.BroadCastToGroup("non-existent-group", "alice", "hello", "👤")
	if err == nil {
		t.Error("expected error broadcasting to non-existent group")
	}
}

func TestSendPrivateOffline(t *testing.T) {
	s := testServer(t)

	// Alice is in the DB but not online
	err := s.SendPrivate("alice", "bob", "hello", "👤")
	if err == nil {
		t.Error("expected error when target is offline")
	}
}

func TestShutdown(t *testing.T) {
	s := testServer(t)
	// Should not panic
	s.Shutdown()
	// Double shutdown should not panic
	s.Shutdown()
}

func TestSessionExpiryCausesDeletion(t *testing.T) {
	s := testServer(t)

	// Manually insert an expired session
	s.sessionLock.Lock()
	s.SessionTokens["already-expired"] = &model.Session{
		Username:  "alice",
		CreatedAt: time.Now().Add(-48 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	s.sessionLock.Unlock()

	// GetUsernameByToken should delete expired tokens
	_, ok := s.GetUsernameByToken("already-expired")
	if ok {
		t.Error("expected expired token to return false")
	}

	s.sessionLock.RLock()
	_, stillExists := s.SessionTokens["already-expired"]
	s.sessionLock.RUnlock()
	if stillExists {
		t.Error("expected expired session to be deleted from map")
	}
}
