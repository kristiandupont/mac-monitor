package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"mac-monitor/internal/alerts"
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

// AlertService is the part of *alerts.Engine the API exposes.
type AlertService interface {
	Status() (alerts.Status, error)
	Ignore(process string) error
	Unignore(process string) error
}

type Server struct {
	db        *storage.DB
	hub       *Hub
	mux       *http.ServeMux
	retention time.Duration
	alerts    AlertService
}

func New(db *storage.DB, hub *Hub, static fs.FS, retention time.Duration, alerts AlertService) *Server {
	s := &Server{db: db, hub: hub, mux: http.NewServeMux(), retention: retention, alerts: alerts}
	s.mux.HandleFunc("/api/config", s.handleConfig)
	s.mux.HandleFunc("GET /api/alerts", s.handleAlerts)
	s.mux.HandleFunc("POST /api/alerts/ignore", s.handleIgnore)
	s.mux.HandleFunc("DELETE /api/alerts/ignore", s.handleUnignore)
	s.mux.HandleFunc("/api/live", s.handleLive)
	s.mux.HandleFunc("/api/history", s.handleHistory)
	s.mux.HandleFunc("/api/latest", s.handleLatest)
	s.mux.HandleFunc("/api/processes", s.handleProcesses)
	s.mux.Handle("/", http.FileServer(http.FS(static)))
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

	// step > 1 downsamples to one averaged snapshot per step seconds.
	step, _ := strconv.ParseInt(q.Get("step"), 10, 64)

	snaps, err := s.db.QueryDownsampled(from, to, step)
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

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		RetentionSeconds int64 `json:"retention_seconds"`
	}{RetentionSeconds: int64(s.retention / time.Second)})
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	st, err := s.alerts.Status()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(st)
}

// handleIgnore takes {"process": "name"}. Requiring a JSON content type means
// browsers preflight cross-origin requests (which we don't allow), so other
// web pages can't silently change the ignore list.
func (s *Server) handleIgnore(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
		return
	}
	var body struct {
		Process string `json:"process"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Process) == "" {
		http.Error(w, "expected {\"process\": \"name\"}", http.StatusBadRequest)
		return
	}
	if err := s.alerts.Ignore(body.Process); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnignore(w http.ResponseWriter, r *http.Request) {
	process := r.URL.Query().Get("process")
	if process == "" {
		http.Error(w, "missing process", http.StatusBadRequest)
		return
	}
	if err := s.alerts.Unignore(process); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request) {
	procs, cpuReady, err := collector.CollectProcesses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		CPUReady  bool                   `json:"cpu_ready"`
		Processes []collector.ProcessStat `json:"processes"`
	}{CPUReady: cpuReady, Processes: procs})
}
