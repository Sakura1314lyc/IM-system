package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"log/slog"
	"strconv"
	"strings"
	"time"

)

type ctxKey string

const ctxUser ctxKey = "user"

func (s *Server) authQueryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			if auth := r.Header.Get("Authorization"); len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
				token = strings.TrimSpace(auth[7:])
			}
		}
		if token == "" {
			http.Error(w, "缺少登录 token", http.StatusBadRequest)
			return
		}
		username, ok := s.GetUsernameByToken(token)
		if !ok {
			http.Error(w, "认证失败", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, username)
		next(w, r.WithContext(ctx))
	}
}

func getUserFromContext(r *http.Request) string {
	if v := r.Context().Value(ctxUser); v != nil {
		return v.(string)
	}
	return ""
}

// getTokenFromRequest extracts token from Authorization: Bearer header, then falls back to query param
func getTokenFromRequest(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		if t := strings.TrimSpace(auth[7:]); t != "" {
			return t
		}
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

func decodeJSONBody(r *http.Request, dst interface{}) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1MB
	return json.NewDecoder(r.Body).Decode(dst)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func (s *Server) StartWeb() {
	addr := s.webAddr
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./web")))
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(s.uploadDir))))
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/ws", s.handleWebSocket)
	mux.HandleFunc("/api/online", s.handleOnline)
	mux.HandleFunc("/api/send", s.handleSend)
	mux.HandleFunc("/api/rename", s.handleRename)
	mux.HandleFunc("/api/register", s.handleRegister)
	mux.HandleFunc("/api/avatar", s.handleAvatar)
	mux.HandleFunc("/api/group", s.handleGroup)
	mux.HandleFunc("/api/groups", s.authQueryMiddleware(s.handleGroups))
	mux.HandleFunc("/api/profile", s.handleProfile)
	mux.HandleFunc("/api/history", s.authQueryMiddleware(s.handleHistory))
	mux.HandleFunc("/api/friend", s.handleFriend)
	mux.HandleFunc("/api/friends", s.authQueryMiddleware(s.handleFriends))
	mux.HandleFunc("/api/check-friend", s.authQueryMiddleware(s.handleCheckFriend))

	listener, finalAddr, err := listenWithFallback(addr, 20)
	if err != nil {
		slog.Error("start web failed", "error", err)
		return
	}
	s.webListener = listener
	s.webAddr = finalAddr // update with actual bound address
	defer func() { s.webListener = nil }()

	slog.Info("web ui started", "addr", finalAddr)
	handler := corsMiddleware(mux)
	if err := http.Serve(listener, handler); err != nil {
		slog.Error("start web serve err", "error", err)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	if !s.loginLimiter.Allow(s.getClientIP(r)) {
		http.Error(w, "请求过于频繁，请稍后再试", http.StatusTooManyRequests)
		return
	}

	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSONBody(r, &data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Username) == "" || strings.TrimSpace(data.Password) == "" {
		http.Error(w, "用户名和密码不能为空", http.StatusBadRequest)
		return
	}

	userInfo, err := s.DB.AuthenticateUser(data.Username, data.Password)
	if err != nil {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	token, err := s.CreateSession(data.Username)
	if err != nil {
		http.Error(w, "生成会话失败", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":     token,
		"avatar":    userInfo.Avatar,
		"gender":    userInfo.Gender,
		"signature": userInfo.Signature,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token string `json:"token"`
	}
	if err := decodeJSONBody(r, &data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	token := data.Token
	if token == "" {
		token = getTokenFromRequest(r)
	}
	if token == "" {
		http.Error(w, "token 不能为空", http.StatusBadRequest)
		return
	}

	s.DeleteSession(token)
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) handleOnline(w http.ResponseWriter, r *http.Request) {
	users := s.GetOnlineUsers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"online": users})
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	if !s.sendLimiter.Allow(s.getClientIP(r)) {
		http.Error(w, "消息发送过于频繁，请稍后再试", http.StatusTooManyRequests)
		return
	}

	var data struct {
		Token   string `json:"token"`
		Name    string `json:"name"`
		To      string `json:"to"`
		Message string `json:"message"`
		Mode    string `json:"mode"`
	}

	if err := decodeJSONBody(r, &data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" || strings.TrimSpace(data.Message) == "" {
		http.Error(w, "token 和 message 不能为空", http.StatusBadRequest)
		return
	}

	name, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	if data.Mode == "private" && strings.TrimSpace(data.To) == "" {
		http.Error(w, "私聊必须指定 to", http.StatusBadRequest)
		return
	}

	if data.Mode == "group" && strings.TrimSpace(data.To) == "" {
		http.Error(w, "群聊必须指定 to (群名)", http.StatusBadRequest)
		return
	}

	avatar := ""
	s.wsLock.RLock()
	if client, ok := s.WSConns[name]; ok {
		avatar = client.Avatar
	}
	s.wsLock.RUnlock()

	if data.Mode == "private" {
		if err := s.SendPrivate(name, data.To, data.Message, avatar); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	w.WriteHeader(http.StatusNoContent)
		return
	}

	if data.Mode == "group" {
		if err := s.BroadCastToGroup(data.To, name, data.Message, avatar); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	w.WriteHeader(http.StatusNoContent)
		return
	}

	s.BroadCastFromWeb(name, data.Message, avatar)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "只支持 GET 或 POST", http.StatusMethodNotAllowed)
		return
	}

	switch r.Method {
	case http.MethodGet:
		username := getUserFromContext(r)
		if username == "" {
			token := getTokenFromRequest(r)
			if token != "" {
				var ok bool
				username, ok = s.GetUsernameByToken(token)
				if !ok {
					http.Error(w, "认证失败", http.StatusUnauthorized)
					return
				}
			}
		}

		targetUsername := strings.TrimSpace(r.URL.Query().Get("user"))
		if targetUsername == "" {
			targetUsername = username
		}

		if targetUsername == "" {
			http.Error(w, "缺少登录 token", http.StatusBadRequest)
			return
		}

		userInfo, err := s.DB.GetUserByUsername(targetUsername)
		if err != nil {
			http.Error(w, "用户信息读取失败", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"username":  userInfo.Username,
			"avatar":    userInfo.Avatar,
			"gender":    userInfo.Gender,
			"signature": userInfo.Signature,
			"createdAt": userInfo.CreatedAt,
			"isSelf":    userInfo.Username == username,
		})
		return

	case http.MethodPost:
		var data struct {
			Token     string `json:"token"`
			Gender    string `json:"gender"`
			Signature string `json:"signature"`
		}
		if err := decodeJSONBody(r, &data); err != nil {
			http.Error(w, "请求体解析失败", http.StatusBadRequest)
			return
		}

		if data.Token == "" {
			data.Token = getTokenFromRequest(r)
		}
		if strings.TrimSpace(data.Token) == "" {
			http.Error(w, "token 不能为空", http.StatusBadRequest)
			return
		}

		username, ok := s.GetUsernameByToken(data.Token)
		if !ok {
			http.Error(w, "认证失败", http.StatusUnauthorized)
			return
		}

		if err := s.DB.UpdateUserProfile(username, data.Gender, data.Signature); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token  string `json:"token"`
		Avatar string `json:"avatar"`
	}
	if err := decodeJSONBody(r, &data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" || strings.TrimSpace(data.Avatar) == "" {
		http.Error(w, "token 和 avatar 不能为空", http.StatusBadRequest)
		return
	}

	username, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	avatarURL, err := s.UpdateAvatar(username, data.Avatar)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"avatar": avatarURL})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	username := getUserFromContext(r)

	typeResp := r.URL.Query().Get("type")
	groupName := r.URL.Query().Get("group")
	limit := 300
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		if parsedLimit, parseErr := strconv.Atoi(rawLimit); parseErr == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 500 {
				limit = 500
			}
		}
	}

	var history interface{}
	var err error

	switch typeResp {
	case "group":
		if strings.TrimSpace(groupName) == "" {
			http.Error(w, "缺少 group 参数", http.StatusBadRequest)
			return
		}
		history, err = s.DB.GetGroupMessages(groupName, limit)
	case "private":
		peer := r.URL.Query().Get("peer")
		if strings.TrimSpace(peer) == "" {
			http.Error(w, "缺少 peer 参数", http.StatusBadRequest)
			return
		}
		history, err = s.DB.GetPrivateMessages(username, peer, limit)
	default:
		history, err = s.DB.GetPublicMessages(limit)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"history": history})
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token string `json:"token"`
		New   string `json:"new"`
	}
	if err := decodeJSONBody(r, &data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" || strings.TrimSpace(data.New) == "" {
		http.Error(w, "token/new 不能为空", http.StatusBadRequest)
		return
	}

	username, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	if s.IsNameTaken(data.New) {
		http.Error(w, "用户名已被占用", http.StatusBadRequest)
		return
	}

	if err := s.RenameWSClient(username, data.New); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.UpdateSessionUsername(data.Token, data.New)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	if !s.registerLimiter.Allow(s.getClientIP(r)) {
		http.Error(w, "请求过于频繁，请稍后再试", http.StatusTooManyRequests)
		return
	}

	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Avatar   string `json:"avatar"`
	}
	if err := decodeJSONBody(r, &data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Username) == "" || strings.TrimSpace(data.Password) == "" {
		http.Error(w, "用户名和密码不能为空", http.StatusBadRequest)
		return
	}

	if err := s.Register(data.Username, data.Password, data.Avatar); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token      string `json:"token"`
		Action     string `json:"action"`
		GroupName  string `json:"groupName"`
		MemberName string `json:"memberName"`
	}
	if err := decodeJSONBody(r, &data); err != nil {
		http.Error(w, "请求体解析失败", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" || strings.TrimSpace(data.Action) == "" || strings.TrimSpace(data.GroupName) == "" {
		http.Error(w, "token、action 和 groupName 不能为空", http.StatusBadRequest)
		return
	}

	username, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	user, err := s.DB.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "用户信息读取失败", http.StatusInternalServerError)
		return
	}

	switch data.Action {
	case "create":
		if _, err := s.DB.CreateGroup(data.GroupName, user.ID, ""); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "join":
		if err := s.DB.JoinGroup(data.GroupName, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "leave":
		if err := s.DB.LeaveGroup(data.GroupName, user.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "invite":
		if strings.TrimSpace(data.MemberName) == "" {
			http.Error(w, "memberName 不能为空", http.StatusBadRequest)
			return
		}
		targetUser, err := s.DB.GetUserByUsername(data.MemberName)
		if err != nil {
			http.Error(w, "目标用户不存在", http.StatusNotFound)
			return
		}
		if err := s.DB.JoinGroup(data.GroupName, targetUser.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "kick":
		if strings.TrimSpace(data.MemberName) == "" {
			http.Error(w, "memberName 不能为空", http.StatusBadRequest)
			return
		}
		group, err := s.DB.GetGroupByName(data.GroupName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if group.CreatorID != user.ID {
			http.Error(w, "只有群主可以踢人", http.StatusForbidden)
			return
		}
		targetUser, err := s.DB.GetUserByUsername(data.MemberName)
		if err != nil {
			http.Error(w, "目标用户不存在", http.StatusNotFound)
			return
		}
		if err := s.DB.LeaveGroup(data.GroupName, targetUser.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "未知 action", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	username := getUserFromContext(r)

	groups, err := s.DB.GetUserGroups(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"groups": groups})
}

func (s *Server) handleFriend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "只支持 POST", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Token      string `json:"token"`
		Action     string `json:"action"`
		FriendName string `json:"friend"`
	}

	if err := decodeJSONBody(r, &data); err != nil {
		http.Error(w, "无效的请求格式", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(data.Token) == "" {
		http.Error(w, "缺少登录 token", http.StatusBadRequest)
		return
	}

	username, ok := s.GetUsernameByToken(data.Token)
	if !ok {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	user, err := s.DB.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusUnauthorized)
		return
	}

	if strings.TrimSpace(data.FriendName) == "" {
		http.Error(w, "friend 不能为空", http.StatusBadRequest)
		return
	}

	friendUser, err := s.DB.GetUserByUsername(data.FriendName)
	if err != nil {
		http.Error(w, "好友用户不存在", http.StatusNotFound)
		return
	}

	switch data.Action {
	case "add":
		if err := s.DB.AddFriend(user.ID, friendUser.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case "remove":
		if err := s.DB.RemoveFriend(user.ID, friendUser.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "未知 action", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func (s *Server) handleFriends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	username := getUserFromContext(r)

	user, err := s.DB.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusUnauthorized)
		return
	}

	friends, err := s.DB.GetFriends(user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var friendList []map[string]interface{}
	for _, f := range friends {
		friendList = append(friendList, map[string]interface{}{
			"name":   f.Username,
			"avatar": f.Avatar,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"friends": friendList})
}

func (s *Server) handleCheckFriend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "只支持 GET", http.StatusMethodNotAllowed)
		return
	}

	friendName := r.URL.Query().Get("friend")
	if strings.TrimSpace(friendName) == "" {
		http.Error(w, "缺少必要参数", http.StatusBadRequest)
		return
	}

	username := getUserFromContext(r)

	user, err := s.DB.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusUnauthorized)
		return
	}

	friendUser, err := s.DB.GetUserByUsername(friendName)
	if err != nil {
		http.Error(w, "好友用户不存在", http.StatusNotFound)
		return
	}

	isFriend, err := s.DB.IsFriend(user.ID, friendUser.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"isFriend": isFriend})
}

func listenWithFallback(addr string, maxAttempts int) (net.Listener, string, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, "", err
	}

	var port int
	fmt.Sscanf(portStr, "%d", &port)
	if port <= 0 {
		ln, err := net.Listen("tcp", addr)
		return ln, addr, err
	}

	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		tryAddr := net.JoinHostPort(host, fmt.Sprintf("%d", port+i))
		ln, listenErr := net.Listen("tcp", tryAddr)
		if listenErr == nil {
			if i > 0 {
				slog.Warn("port occupied, auto-switched", "from", addr, "to", tryAddr)
			}
			return ln, tryAddr, nil
		}
		lastErr = listenErr
		if !isAddrInUseErr(listenErr) {
			return nil, "", listenErr
		}
	}

	return nil, "", fmt.Errorf("listen failed after %d attempts: %w", maxAttempts, lastErr)
}

func isAddrInUseErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "only one usage")
}

func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"IM-system Dev"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "127.0.0.1"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}, nil
}
