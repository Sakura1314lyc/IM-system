package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// WSClient represents a connected WebSocket client.
type WSClient struct {
	Name      string
	Conn      *websocket.Conn
	Avatar    string
	RemoteIP  string
	C         chan string
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
}

type wsIncoming struct {
	Type    string `json:"type"`
	Mode    string `json:"mode"`
	To      string `json:"to,omitempty"`
	Message string `json:"message,omitempty"`
}

type wsOutgoing struct {
	Type    string `json:"type"`
	Mode    string `json:"mode,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Group   string `json:"group,omitempty"`
	Avatar  string `json:"avatar,omitempty"`
	Content string `json:"content"`
	Time    string `json:"time,omitempty"`
}

func nowLabelWS() string {
	return time.Now().Format("15:04")
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Error("websocket accept error", "error", err)
		return
	}
	conn.SetReadLimit(65536) // 64KB max message

	// First message must be auth
	_, msgBytes, err := conn.Read(context.Background())
	if err != nil {
		conn.Close(websocket.StatusPolicyViolation, "auth required")
		return
	}

	var authMsg struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(msgBytes, &authMsg); err != nil || authMsg.Type != "auth" || authMsg.Token == "" {
		conn.Close(websocket.StatusPolicyViolation, "invalid auth")
		return
	}

	username, ok := s.GetUsernameByToken(authMsg.Token)
	if !ok {
		conn.Close(websocket.StatusPolicyViolation, "auth failed")
		return
	}

	// Remove any existing connection for this user
	s.RemoveWSClient(username)

	userInfo, err := s.DB.GetUserByUsername(username)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "user not found")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	client := &WSClient{
		Name:      username,
		Conn:      conn,
		Avatar:    userInfo.Avatar,
			RemoteIP:  s.getClientIP(r),
		C:      make(chan string, 100),
		ctx:    ctx,
		cancel: cancel,
	}

	// Confirm auth to client
	authResp, _ := json.Marshal(wsOutgoing{Type: "auth_ok"})
	conn.Write(ctx, websocket.MessageText, authResp)

	s.AddWSClient(client)
	defer s.RemoveWSClient(username)

	// Write goroutine: reads from client.C and writes to WebSocket connection
	go func() {
		defer slog.Debug("ws write goroutine exited", "name", client.Name)
		for {
			select {
			case msg := <-client.C:
				err := client.Conn.Write(ctx, websocket.MessageText, []byte(msg))
				if err != nil {
					slog.Error("ws write error", "error", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Read loop
	for {
		_, msgBytes, err := conn.Read(ctx)
		if err != nil {
			cancel()
			return
		}

		var data wsIncoming
		if err := json.Unmarshal(msgBytes, &data); err != nil {
			s.writeWS(client, wsOutgoing{Type: "error", Content: "invalid message format"})
			continue
		}

		switch data.Type {
		case "send":
			s.handleWSMessage(client, &data)
		case "ping":
			s.writeWS(client, wsOutgoing{Type: "pong"})
		default:
			s.writeWS(client, wsOutgoing{Type: "error", Content: "unknown message type"})
		}
	}
}

func (s *Server) handleWSMessage(client *WSClient, data *wsIncoming) {
	if !s.sendLimiter.Allow(client.RemoteIP) {
		s.writeWS(client, wsOutgoing{Type: "error", Content: "消息发送过于频繁，请稍后再试"})
		return
	}
	content := strings.TrimSpace(data.Message)
	if content == "" {
		return
	}
	content = sanitizeInputWithLimit(content, s.maxMsgLength)

	switch data.Mode {
	case "private":
		to := strings.TrimSpace(data.To)
		if to == "" {
			s.writeWS(client, wsOutgoing{Type: "error", Content: "missing recipient"})
			return
		}
		if err := s.SendPrivateWS(client.Name, to, content, client.Avatar); err != nil {
			s.writeWS(client, wsOutgoing{Type: "error", Content: err.Error()})
			return
		}
		s.writeWS(client, wsOutgoing{
			Type:    "sent",
			Mode:    "private",
			To:      to,
			Content: content,
			Time:    nowLabelWS(),
		})

	case "group":
		groupName := strings.TrimSpace(data.To)
		if groupName == "" {
			s.writeWS(client, wsOutgoing{Type: "error", Content: "missing group name"})
			return
		}
		if err := s.BroadCastToGroupWS(groupName, client.Name, content, client.Avatar); err != nil {
			s.writeWS(client, wsOutgoing{Type: "error", Content: err.Error()})
			return
		}

	default:
		// Public message
		s.BroadCastFromWS(client.Name, content, client.Avatar)
	}
}

func (s *Server) writeWS(client *WSClient, msg wsOutgoing) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case client.C <- string(data):
	case <-client.ctx.Done():
	default:
		slog.Warn("ws client buffer full, dropping message", "name", client.Name)
	}
}

func (s *Server) AddWSClient(client *WSClient) {
	s.wsLock.Lock()
	defer s.wsLock.Unlock()
	s.WSConns[client.Name] = client
	slog.Debug("ws client connected", "name", client.Name)
}

func (s *Server) RemoveWSClient(name string) {
	s.wsLock.Lock()
	defer s.wsLock.Unlock()
	if c, ok := s.WSConns[name]; ok {
		c.cancel()
		delete(s.WSConns, name)
		slog.Debug("ws client disconnected", "name", name)
	}
}

func (s *Server) SendPrivateWS(from, to, content, avatar string) error {
	return s.sendPrivate(from, to, content, avatar)
}

func (s *Server) BroadCastToGroupWS(groupName, from, msg, avatar string) error {
	return s.broadcastToGroup(groupName, from, msg, avatar)
}

func (s *Server) BroadCastFromWS(name string, msg string, avatar string) {
	s.broadcastPublic(name, msg, avatar, false)
}
