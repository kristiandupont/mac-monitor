package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"mac-monitor/internal/collector"
	"mac-monitor/internal/storage"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsClient struct {
	send chan *collector.Snapshot
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*wsClient]struct{})}
}

func (h *Hub) add(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *wsClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *Hub) Broadcast(s *collector.Snapshot) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- s:
		default:
		}
	}
}

type Server struct {
	db  *storage.DB
	hub *Hub
	mux *http.ServeMux
}

func New(db *storage.DB, hub *Hub, staticDir string) *Server {
	s := &Server{db: db, hub: hub, mux: http.NewServeMux()}
	s.mux.HandleFunc("/api/live", s.handleLive)
	s.mux.HandleFunc("/api/history", s.handleHistory)
	s.mux.HandleFunc("/api/latest", s.handleLatest)
	s.mux.Handle("/", http.FileServer(http.Dir(staticDir)))
	return s
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	defer conn.Close()

	client := &wsClient{send: make(chan *collector.Snapshot, 16)}
	s.hub.add(client)
	defer s.hub.remove(client)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case snap := <-client.send:
			if err := conn.WriteJSON(snap); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	to, _ := strconv.ParseInt(q.Get("to"), 10, 64)
	from, _ := strconv.ParseInt(q.Get("from"), 10, 64)
	if to == 0 {
		to = time.Now().Unix()
	}
	if from == 0 {
		from = to - 3600
	}

	snaps, err := s.db.Query(from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if snaps == nil {
		snaps = []*collector.Snapshot{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snaps)
}

func (s *Server) handleLatest(w http.ResponseWriter, r *http.Request) {
	snap, err := s.db.Latest()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snap)
}
