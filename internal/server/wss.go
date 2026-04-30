package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"log/slog"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// WSClient represents a connected WebSocket client.
type WSClient struct {
	Name   string
	Conn   *websocket.Conn
	Avatar string
	C      chan string
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
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
	token := getTokenFromRequest(r)
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	username, ok := s.GetUsernameByToken(token)
	if !ok {
		http.Error(w, "auth failed", http.StatusUnauthorized)
		return
	}

	// Remove any existing connection for this user
	s.RemoveWSClient(username)

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		slog.Error("websocket accept error", "error", err)
		return
	}
		conn.SetReadLimit(65536) // 64KB max message

	userInfo, err := s.DB.GetUserByUsername(username)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "user not found")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	client := &WSClient{
		Name:   username,
		Conn:   conn,
		Avatar: userInfo.Avatar,
		C:      make(chan string, 100),
		ctx:    ctx,
		cancel: cancel,
	}

	s.AddWSClient(client)
	defer s.RemoveWSClient(username)

	// Write goroutine: reads from client.C and writes to WebSocket connection
	go func() {
		defer slog.Info("ws write goroutine exited", "name", client.Name)
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
	slog.Info("ws client connected", "name", client.Name)
}

func (s *Server) RemoveWSClient(name string) {
	s.wsLock.Lock()
	defer s.wsLock.Unlock()
	if c, ok := s.WSConns[name]; ok {
		c.cancel()
		delete(s.WSConns, name)
		slog.Info("ws client disconnected", "name", name)
	}
}

func (s *Server) sendWSMessage(msg string) {
	s.wsLock.RLock()
	defer s.wsLock.RUnlock()
	for _, client := range s.WSConns {
		select {
		case client.C <- msg:
		case <-client.ctx.Done():
		default:
		}
	}
}

func (s *Server) SendPrivateWS(from, to, content, avatar string) error {
	fromUser, err := s.DB.GetUserByUsername(from)
	if err != nil {
		return fmt.Errorf("sender not found: %v", err)
	}

	toUser, err := s.DB.GetUserByUsername(to)
	if err != nil {
		return fmt.Errorf("recipient not found: %v", err)
	}

	_, err = s.DB.SaveMessage(fromUser.ID, &toUser.ID, nil, content, "private")
	if err != nil {
		return fmt.Errorf("failed to save message: %v", err)
	}

	// Try WebSocket delivery
	s.wsLock.RLock()
	if client, ok := s.WSConns[to]; ok {
		s.writeWS(client, wsOutgoing{
			Type:    "message",
			Mode:    "private",
			From:    from,
			To:      to,
			Avatar:  avatar,
			Content: content,
			Time:    nowLabelWS(),
		})
		s.wsLock.RUnlock()
		return nil
	}
	s.wsLock.RUnlock()

	// Try TCP delivery
	s.mapLock.RLock()
	if user, ok := s.OnlineMap[to]; ok {
		user.SendMsg("[priv] " + from + ": " + content + "\n")
		s.mapLock.RUnlock()
		return nil
	}
	s.mapLock.RUnlock()

	slog.Info("private message saved for offline user", "from", from, "to", to)
		return nil
}

func (s *Server) BroadCastToGroupWS(groupName, from, msg, avatar string) error {
	fromUser, err := s.DB.GetUserByUsername(from)
	if err != nil {
		return fmt.Errorf("sender not found: %v", err)
	}

	group, err := s.DB.GetGroupByName(groupName)
	if err != nil {
		return fmt.Errorf("group not found: %v", err)
	}

	members, err := s.DB.GetGroupMembers(groupName)
	if err != nil {
		return fmt.Errorf("failed to get members: %v", err)
	}

	_, err = s.DB.SaveMessage(fromUser.ID, nil, &group.ID, msg, "group")
	if err != nil {
		return fmt.Errorf("failed to save message: %v", err)
	}

	outgoing := wsOutgoing{
		Type:    "message",
		Mode:    "group",
		Group:   groupName,
		From:    from,
		Avatar:  avatar,
		Content: msg,
		Time:    nowLabelWS(),
	}

	textMsg := fmt.Sprintf("[group %s] %s: %s\n", groupName, from, msg)

	for _, member := range members {
		// WebSocket delivery
		s.wsLock.RLock()
		if client, ok := s.WSConns[member.Username]; ok {
			s.writeWS(client, outgoing)
		}
		s.wsLock.RUnlock()

		// TCP delivery
		s.mapLock.RLock()
		if user, ok := s.OnlineMap[member.Username]; ok {
			user.SendMsg(textMsg)
		}
		s.mapLock.RUnlock()
	}

	return nil
}

func (s *Server) BroadCastFromWS(name string, msg string, avatar string) {
	sendMsg := name + ": " + msg

	fromUser, err := s.DB.GetUserByUsername(name)
	if err == nil {
		_, err = s.DB.SaveMessage(fromUser.ID, nil, nil, msg, "public")
		if err != nil {
			slog.Warn("failed to save public message", "error", err)
		}
	}

	// Send to TCP clients via Message channel
	tcpMsg := "[WEB] " + sendMsg
	s.Message <- tcpMsg

	// Send to all WS clients
	outgoing := wsOutgoing{
		Type:    "message",
		Mode:    "public",
		From:    name,
		Avatar:  avatar,
		Content: msg,
		Time:    nowLabelWS(),
	}
	s.wsLock.RLock()
	defer s.wsLock.RUnlock()
	for _, client := range s.WSConns {
		s.writeWS(client, outgoing)
	}
}
