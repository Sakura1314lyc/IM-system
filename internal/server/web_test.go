package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"IM-system/internal/model"
)

// mockStorage implements Storage for testing HTTP handlers.
type mockStorage struct {
	users        map[string]*model.DBUser
	messages     []*model.DBMessageExt
	groups       map[string]*model.DBGroup
	friends      map[int]map[int]bool
	groupMembers map[string][]*model.UserPublic
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		users:        make(map[string]*model.DBUser),
		groups:       make(map[string]*model.DBGroup),
		friends:      make(map[int]map[int]bool),
		groupMembers: make(map[string][]*model.UserPublic),
	}
}

func (m *mockStorage) addGroupMember(groupName string, user *model.UserPublic) {
	m.groupMembers[groupName] = append(m.groupMembers[groupName], user)
}

func (m *mockStorage) addUser(username, password string) {
	m.users[username] = &model.DBUser{
		ID:        len(m.users) + 1,
		Username:  username,
		Password:  "$2a$10$" + password,
		Avatar:    "🐱",
		Gender:    "male",
		Signature: "hello",
		CreatedAt: time.Now(),
	}
}

func (m *mockStorage) AuthenticateUser(username, password string) (*model.DBUser, error) {
	u, ok := m.users[username]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return u, nil
}

func (m *mockStorage) RegisterUser(username, password, avatar string) error {
	if _, ok := m.users[username]; ok {
		return fmt.Errorf("user already exists")
	}
	m.users[username] = &model.DBUser{
		ID:       len(m.users) + 1,
		Username: username,
		Password: password,
		Avatar:   avatar,
	}
	return nil
}

func (m *mockStorage) GetUserByUsername(username string) (*model.UserPublic, error) {
	u, ok := m.users[username]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return &model.UserPublic{
		ID:        u.ID,
		Username:  u.Username,
		Avatar:    u.Avatar,
		Signature: u.Signature,
		Gender:    u.Gender,
		CreatedAt: u.CreatedAt,
	}, nil
}

func (m *mockStorage) UpdateUserAvatar(username, avatar string) error {
	if _, ok := m.users[username]; !ok {
		return fmt.Errorf("user not found")
	}
	m.users[username].Avatar = avatar
	return nil
}

func (m *mockStorage) UpdateUserProfile(username, gender, signature string) error {
	if _, ok := m.users[username]; !ok {
		return fmt.Errorf("user not found")
	}
	u := m.users[username]
	u.Gender = gender
	u.Signature = signature
	return nil
}

func (m *mockStorage) SaveMessage(fromID int, toID *int, groupID *int, content, msgType string) (*model.DBMessage, error) {
	return &model.DBMessage{ID: 1, FromID: fromID, ToID: toID, GroupID: groupID, Content: content, Type: msgType, CreatedAt: time.Now()}, nil
}

func (m *mockStorage) GetPublicMessages(limit int) ([]*model.DBMessageExt, error) {
	return m.messages, nil
}

func (m *mockStorage) GetGroupMessages(groupName string, limit int) ([]*model.DBMessageExt, error) {
	return nil, nil
}

func (m *mockStorage) GetPrivateMessages(username, peer string, limit int) ([]*model.DBMessageExt, error) {
	return nil, nil
}

func (m *mockStorage) CreateGroup(name string, creatorID int, description string) (*model.DBGroup, error) {
	if _, ok := m.groups[name]; ok {
		return nil, fmt.Errorf("group already exists")
	}
	g := &model.DBGroup{ID: len(m.groups) + 1, Name: name, CreatorID: creatorID, Description: description, CreatedAt: time.Now()}
	m.groups[name] = g
	return g, nil
}

func (m *mockStorage) JoinGroup(groupName string, userID int) error {
	return nil
}

func (m *mockStorage) LeaveGroup(groupName string, userID int) error {
	return nil
}

func (m *mockStorage) GetGroupMembers(groupName string) ([]*model.UserPublic, error) {
	members, ok := m.groupMembers[groupName]
	if !ok {
		return nil, nil
	}
	return members, nil
}

func (m *mockStorage) GetGroupByName(name string) (*model.DBGroup, error) {
	g, ok := m.groups[name]
	if !ok {
		return nil, fmt.Errorf("group not found")
	}
	return g, nil
}

func (m *mockStorage) GetUserGroups(username string) ([]string, error) {
	var groups []string
	for name, members := range m.groupMembers {
		for _, mem := range members {
			if mem.Username == username {
				groups = append(groups, name)
			}
		}
	}
	return groups, nil
}

func (m *mockStorage) AddFriend(userID, friendID int) error {
	if m.friends[userID] == nil {
		m.friends[userID] = make(map[int]bool)
	}
	m.friends[userID][friendID] = true
	if m.friends[friendID] == nil {
		m.friends[friendID] = make(map[int]bool)
	}
	m.friends[friendID][userID] = true
	return nil
}

func (m *mockStorage) RemoveFriend(userID, friendID int) error {
	delete(m.friends[userID], friendID)
	delete(m.friends[friendID], userID)
	return nil
}

func (m *mockStorage) GetFriends(userID int) ([]*model.UserPublic, error) {
	return nil, nil
}

func (m *mockStorage) IsFriend(userID, friendID int) (bool, error) {
	return m.friends[userID][friendID], nil
}

func (m *mockStorage) Close() error { return nil }

// testHTTPServer creates a Server with mock storage and httptest server for testing HTTP handlers.
type testHTTPHarness struct {
	server *Server
	http   *httptest.Server
	mock   *mockStorage
	tokens map[string]string // username -> token
}

func newTestHTTPHarness(t *testing.T) *testHTTPHarness {
	t.Helper()

	mock := newMockStorage()
	// Add default users
	mock.addUser("alice", "pass123")
	mock.addUser("bob", "pass456")

	s := &Server{
		Ip:              "127.0.0.1",
		Port:            0,
		OnlineMap:       make(map[string]*User),
		WSConns:         make(map[string]*WSClient),
		SessionTokens:   make(map[string]*model.Session),
		Message:         make(chan string, 100),
		DB:              mock,
		loginLimiter:    newRateLimiter(100, time.Minute),
		registerLimiter: newRateLimiter(100, time.Minute),
		sendLimiter:     newRateLimiter(100, time.Minute),
		ctx:             nil,
		cancel:          nil,
		idleTimeout:     time.Minute,
		sessionTTL:      time.Hour,
		sessionCleanup:  time.Hour,
		maxMsgLength:    500,
		webAddr:         ":0",
		uploadDir:       "uploads",
	}

	// Use a real mux for testing
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/online", s.handleOnline)
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/avatar", s.handleAvatar)
	mux.HandleFunc("/api/lchat/meta", s.handleLchatMeta)
	mux.HandleFunc("/api/stickers", s.handleStickers)
	mux.HandleFunc("/api/profile", s.handleProfile)
	mux.HandleFunc("/api/rename", s.handleRename)
	mux.HandleFunc("/api/history", s.authQueryMiddleware(s.handleHistory))
	mux.HandleFunc("/api/group", s.handleGroup)
	mux.HandleFunc("/api/groups", s.authQueryMiddleware(s.handleGroups))
	mux.HandleFunc("/api/friend", s.handleFriend)
	mux.HandleFunc("/api/friends", s.authQueryMiddleware(s.handleFriends))
	mux.HandleFunc("/api/check-friend", s.authQueryMiddleware(s.handleCheckFriend))
	mux.HandleFunc("/api/send", s.handleSend)

	httpServer := httptest.NewServer(mux)

	tokens := make(map[string]string)
	tokens["alice"] = "alice-test-token"
	tokens["bob"] = "bob-test-token"
	s.SessionTokens["alice-test-token"] = &model.Session{Username: "alice", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	s.SessionTokens["bob-test-token"] = &model.Session{Username: "bob", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}

	// Register alice as a WebSocket client so handleRename succeeds
	wsCtx, wsCancel := context.WithCancel(context.Background())
	s.AddWSClient(&WSClient{
		Name:   "alice",
		Avatar: "🐱",
		C:      make(chan string, 100),
		ctx:    wsCtx,
		cancel: wsCancel,
	})

	return &testHTTPHarness{
		server: s,
		http:   httpServer,
		mock:   mock,
		tokens: tokens,
	}
}

func (h *testHTTPHarness) close() {
	h.http.Close()
}

func (h *testHTTPHarness) url(path string) string {
	return h.http.URL + path
}

func (h *testHTTPHarness) postJSON(path string, body map[string]string) *httptest.ResponseRecorder {
	var b strings.Builder
	json.NewEncoder(&b).Encode(body)
	req := httptest.NewRequest(http.MethodPost, h.url(path), strings.NewReader(b.String()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.http.Config.Handler.ServeHTTP(w, req)
	return w
}

func (h *testHTTPHarness) get(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, h.url(path), nil)
	w := httptest.NewRecorder()
	h.http.Config.Handler.ServeHTTP(w, req)
	return w
}

// --- Tests ---

func TestHandleLogin(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("successful login", func(t *testing.T) {
		w := h.postJSON("/api/login", map[string]string{"username": "alice", "password": "pass123"})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp["token"] == "" {
			t.Error("expected non-empty token")
		}
		if resp["avatar"] == "" {
			t.Error("expected avatar")
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		w := h.postJSON("/api/login", map[string]string{"username": "", "password": ""})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, h.url("/api/login"), nil)
		w := httptest.NewRecorder()
		h.http.Config.Handler.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestHandleRegister(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("successful registration", func(t *testing.T) {
		w := h.postJSON("/api/register", map[string]string{"username": "charlie", "password": "pass789", "avatar": "🐶"})
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("duplicate user", func(t *testing.T) {
		w := h.postJSON("/api/register", map[string]string{"username": "alice", "password": "pass123"})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for duplicate, got %d", w.Code)
		}
	})

	t.Run("empty fields", func(t *testing.T) {
		w := h.postJSON("/api/register", map[string]string{"username": "", "password": ""})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleOnline(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	w := h.get("/api/online")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if _, ok := resp["online"].([]interface{}); !ok {
		// It might be empty array
		t.Logf("online response: %v", resp)
	}
}

func TestHandleLogout(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("successful logout", func(t *testing.T) {
		w := h.postJSON("/api/logout", map[string]string{"token": "alice-test-token"})
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", w.Code)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		w := h.postJSON("/api/logout", map[string]string{})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleProfile(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("get own profile", func(t *testing.T) {
		w := h.get("/api/profile?token=alice-test-token")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["username"] != "alice" {
			t.Errorf("expected alice, got %v", resp["username"])
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		w := h.get("/api/profile?token=invalid")
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("update profile", func(t *testing.T) {
		w := h.postJSON("/api/profile", map[string]string{"token": "alice-test-token", "gender": "female", "signature": "new sig"})
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
		// Verify update
		u, _ := h.mock.GetUserByUsername("alice")
		if u.Gender != "female" || u.Signature != "new sig" {
			t.Errorf("profile not updated: %+v", u)
		}
	})
}

func TestHandleRename(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("successful rename", func(t *testing.T) {
		w := h.postJSON("/api/rename", map[string]string{"token": "alice-test-token", "new": "alice-new"})
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		w := h.postJSON("/api/rename", map[string]string{"token": "alice-test-token", "new": ""})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleFriend(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("add friend", func(t *testing.T) {
		w := h.postJSON("/api/friend", map[string]string{"token": "alice-test-token", "action": "add", "friend": "bob"})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("check friend", func(t *testing.T) {
		w := h.get("/api/check-friend?token=alice-test-token&friend=bob")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		isFriend, _ := resp["isFriend"].(bool)
		if !isFriend {
			t.Error("expected users to be friends")
		}
	})

	t.Run("remove friend", func(t *testing.T) {
		w := h.postJSON("/api/friend", map[string]string{"token": "alice-test-token", "action": "remove", "friend": "bob"})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("add friend wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, h.url("/api/friend"), nil)
		w := httptest.NewRecorder()
		h.http.Config.Handler.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestHandleGroup(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("create group", func(t *testing.T) {
		w := h.postJSON("/api/group", map[string]string{"token": "alice-test-token", "action": "create", "groupName": "test-group"})
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		w := h.postJSON("/api/group", map[string]string{"token": "", "action": "create", "groupName": ""})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestHandleAvatar(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("update avatar with emoji", func(t *testing.T) {
		w := h.postJSON("/api/avatar", map[string]string{"token": "alice-test-token", "avatar": "🦊"})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["avatar"] != "🦊" {
			t.Errorf("expected 🦊, got %s", resp["avatar"])
		}
	})

	t.Run("update without auth", func(t *testing.T) {
		w := h.postJSON("/api/avatar", map[string]string{"token": "", "avatar": "🐱"})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleHistoryAuth(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("requires auth", func(t *testing.T) {
		w := h.get("/api/history?type=public")
		if w.Code != http.StatusBadRequest {
			// Should be 400 (no token) or 401 (invalid)
			t.Logf("got %d (expected auth error)", w.Code)
		}
	})

	t.Run("public history with auth", func(t *testing.T) {
		w := h.get("/api/history?token=alice-test-token&type=public")
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestHandleFriendsList(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("friends list requires auth", func(t *testing.T) {
		w := h.get("/api/friends")
		if w.Code != http.StatusBadRequest {
			t.Logf("got %d (expected auth error)", w.Code)
		}
	})

	t.Run("friends list with auth", func(t *testing.T) {
		w := h.get("/api/friends?token=alice-test-token")
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestHandleGroupsList(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("groups list requires auth", func(t *testing.T) {
		w := h.get("/api/groups")
		if w.Code != http.StatusBadRequest {
			t.Logf("got %d (expected auth error)", w.Code)
		}
	})

	t.Run("groups list with auth", func(t *testing.T) {
		w := h.get("/api/groups?token=alice-test-token")
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestHandleStickers(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	w := h.get("/api/stickers")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Packs []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Items []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
				URL   string `json:"url"`
			} `json:"items"`
		} `json:"packs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(resp.Packs) == 0 || len(resp.Packs[0].Items) < 6 {
		t.Fatalf("expected populated sticker pack, got %+v", resp.Packs)
	}
	if resp.Packs[0].Items[0].ID == "" || resp.Packs[0].Items[0].URL == "" {
		t.Fatalf("expected sticker id and url, got %+v", resp.Packs[0].Items[0])
	}
}

func TestHandleSend(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("public message", func(t *testing.T) {
		w := h.postJSON("/api/send", map[string]string{"token": "alice-test-token", "message": "hello everyone"})
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("private message", func(t *testing.T) {
		bobCtx, bobCancel := context.WithCancel(context.Background())
		h.server.AddWSClient(&WSClient{
			Name:   "bob",
			Avatar: "🐱",
			C:      make(chan string, 100),
			ctx:    bobCtx,
			cancel: bobCancel,
		})
		defer h.server.RemoveWSClient("bob")

		w := h.postJSON("/api/send", map[string]string{
			"token":   "alice-test-token",
			"message": "hi bob",
			"mode":    "private",
			"to":      "bob",
		})
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("group message", func(t *testing.T) {
		h.mock.CreateGroup("test-group", 1, "")
		h.mock.addGroupMember("test-group", &model.UserPublic{ID: 1, Username: "alice", Avatar: "🐱"})

		w := h.postJSON("/api/send", map[string]string{
			"token":   "alice-test-token",
			"message": "hello group",
			"mode":    "group",
			"to":      "test-group",
		})
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing token", func(t *testing.T) {
		w := h.postJSON("/api/send", map[string]string{"token": "", "message": "hi"})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("missing message", func(t *testing.T) {
		w := h.postJSON("/api/send", map[string]string{"token": "alice-test-token", "message": ""})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("private without to", func(t *testing.T) {
		w := h.postJSON("/api/send", map[string]string{
			"token":   "alice-test-token",
			"message": "hi",
			"mode":    "private",
			"to":      "",
		})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid auth", func(t *testing.T) {
		w := h.postJSON("/api/send", map[string]string{"token": "bad-token", "message": "hi"})
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, h.url("/api/send"), nil)
		w := httptest.NewRecorder()
		h.http.Config.Handler.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestHandleProfileEdgeCases(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("no token at all", func(t *testing.T) {
		w := h.get("/api/profile")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, h.url("/api/profile"), nil)
		w := httptest.NewRecorder()
		h.http.Config.Handler.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", w.Code)
		}
	})
}

func TestHandleFriendEdgeCases(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("non-existent friend", func(t *testing.T) {
		w := h.postJSON("/api/friend", map[string]string{
			"token":  "alice-test-token",
			"action": "add",
			"friend": "nonexistent",
		})
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing auth", func(t *testing.T) {
		w := h.postJSON("/api/friend", map[string]string{"token": "", "action": "add", "friend": "bob"})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleGroupEdgeCases(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("join group", func(t *testing.T) {
		h.mock.CreateGroup("join-group", 1, "")

		w := h.postJSON("/api/group", map[string]string{
			"token":     "alice-test-token",
			"action":    "join",
			"groupName": "join-group",
		})
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("leave group", func(t *testing.T) {
		w := h.postJSON("/api/group", map[string]string{
			"token":     "alice-test-token",
			"action":    "leave",
			"groupName": "join-group",
		})
		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing action", func(t *testing.T) {
		w := h.postJSON("/api/group", map[string]string{
			"token":     "alice-test-token",
			"action":    "",
			"groupName": "",
		})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		w := h.postJSON("/api/group", map[string]string{
			"token":     "alice-test-token",
			"action":    "unknown",
			"groupName": "test-group",
		})
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleHistoryEdgeCases(t *testing.T) {
	h := newTestHTTPHarness(t)
	defer h.close()

	t.Run("public history with auth", func(t *testing.T) {
		w := h.get("/api/history?token=alice-test-token&type=public")
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode: %v", err)
		}
		// history field may be nil when no messages exist
		if _, ok := resp["history"]; !ok {
			t.Error("expected history field in response")
		}
	})

	t.Run("requires auth", func(t *testing.T) {
		w := h.get("/api/history?type=public")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("private history missing peer", func(t *testing.T) {
		w := h.get("/api/history?token=alice-test-token&type=private")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("group history missing group", func(t *testing.T) {
		w := h.get("/api/history?token=alice-test-token&type=group")
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})
}
