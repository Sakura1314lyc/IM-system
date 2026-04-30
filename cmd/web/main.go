package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	crypto_rand "crypto/rand"
)

var (
	listenAddr = flag.String("listen", ":8080", "web server listen address")
	tcpAddr    = flag.String("tcp", "127.0.0.1:8888", "TCP chat server address")
)

type session struct {
	id       string
	tcpConn  net.Conn
	messages chan string
	closed   chan struct{}
	once     sync.Once
}

func (s *session) close() {
	s.once.Do(func() {
		close(s.closed)
		_ = s.tcpConn.Close()
	})
}

var (
	sessions   = map[string]*session{}
	sessionsMu sync.RWMutex
)

func main() {
	flag.Parse()

	mux := http.NewServeMux()
	mux.Handle("/", staticHandler())
	mux.HandleFunc("/api/connect", handleConnect)
	mux.HandleFunc("/api/send", handleSend)
	mux.HandleFunc("/api/poll", handlePoll)
	mux.HandleFunc("/api/close", handleClose)

	log.Printf("web ui ready at http://127.0.0.1%s (tcp=%s)", *listenAddr, *tcpAddr)
	if err := http.ListenAndServe(*listenAddr, mux); err != nil {
		log.Fatal(err)
	}
}

func staticHandler() http.Handler {
	root := "web"
	if _, err := os.Stat(root); err != nil {
		root = filepath.Join("..", "..", "web")
	}
	return noCache(http.FileServer(http.Dir(root)))
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

func handleConnect(w http.ResponseWriter, _ *http.Request) {
	tcpConn, err := net.Dial("tcp", *tcpAddr)
	if err != nil {
		http.Error(w, "connect tcp failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	b := make([]byte, 8)
	crypto_rand.Read(b)
	s := &session{
		id:       fmt.Sprintf("%x", b),
		tcpConn:  tcpConn,
		messages: make(chan string, 256),
		closed:   make(chan struct{}),
	}

	sessionsMu.Lock()
	sessions[s.id] = s
	sessionsMu.Unlock()

	go readTCPToChannel(s)
	writeJSON(w, map[string]string{"sessionId": s.id})
}

func readTCPToChannel(s *session) {
	reader := bufio.NewReader(s.tcpConn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			s.close()
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		select {
		case s.messages <- line:
		case <-s.closed:
			return
		}
	}
}

func getSession(id string) (*session, bool) {
	sessionsMu.RLock()
	s, ok := sessions[id]
	sessionsMu.RUnlock()
	return s, ok
}

func handleSend(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("sessionId")
	s, ok := getSession(id)
	if !ok {
		http.Error(w, "invalid session", http.StatusUnauthorized)
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(req.Message)
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")
	if msg == "" {
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	_, err := fmt.Fprint(s.tcpConn, msg+"\n")
	if err != nil {
		http.Error(w, "send failed", http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func handlePoll(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("sessionId")
	s, ok := getSession(id)
	if !ok {
		http.Error(w, "invalid session", http.StatusUnauthorized)
		return
	}

	msgs := make([]string, 0, 8)
	timer := time.NewTimer(12 * time.Second)
	defer timer.Stop()

collect:
	for {
		select {
		case <-s.closed:
			break collect
		case m := <-s.messages:
			msgs = append(msgs, m)
			if len(msgs) >= 20 {
				break collect
			}
		default:
			if len(msgs) > 0 {
				break collect
			}
			select {
			case <-s.closed:
				break collect
			case m := <-s.messages:
				msgs = append(msgs, m)
			case <-timer.C:
				break collect
			}
		}
	}

	writeJSON(w, map[string]any{"messages": msgs})
}

func handleClose(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("sessionId")
	sessionsMu.Lock()
	s, ok := sessions[id]
	if ok {
		delete(sessions, id)
	}
	sessionsMu.Unlock()
	if ok {
		s.close()
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(data)
}
