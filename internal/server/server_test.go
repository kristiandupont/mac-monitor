package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"mac-monitor/internal/alerts"
	"mac-monitor/internal/collector"
	"mac-monitor/internal/storage"
)

func newTestServer(t *testing.T) (*Server, *storage.DB) {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db, NewHub(), os.DirFS("."), 24*time.Hour, &fakeAlerts{}), db
}

func emptySnap(ts int64) *collector.Snapshot {
	return &collector.Snapshot{
		Timestamp:   ts,
		CPUPerCore:  []float64{},
		NetStats:    []collector.NetStat{},
		GPUStats:    []collector.GPUStat{},
		DiskStats:   []collector.DiskStat{},
		DiskIOStats: []collector.DiskIOStat{},
	}
}

func TestHandleHistory_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	w := httptest.NewRecorder()
	srv.handleHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	var snaps []*collector.Snapshot
	if err := json.NewDecoder(w.Body).Decode(&snaps); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("expected empty slice, got %d items", len(snaps))
	}
}

func TestHandleHistory_WithData(t *testing.T) {
	srv, db := newTestServer(t)

	now := time.Now().Unix()
	snap := emptySnap(now)
	snap.CPUPercent = 55.0
	if err := db.Insert(snap); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	url := fmt.Sprintf("/api/history?from=%d&to=%d", now-1, now+1)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	srv.handleHistory(w, req)

	var snaps []*collector.Snapshot
	if err := json.NewDecoder(w.Body).Decode(&snaps); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].CPUPercent != 55.0 {
		t.Errorf("CPUPercent: got %v, want 55", snaps[0].CPUPercent)
	}
}

func TestHandleLatest_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/latest", nil)
	w := httptest.NewRecorder()
	srv.handleLatest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
}

func TestHandleLatest_WithData(t *testing.T) {
	srv, db := newTestServer(t)

	snap := emptySnap(time.Now().Unix())
	snap.MemPercent = 77.0
	if err := db.Insert(snap); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/latest", nil)
	w := httptest.NewRecorder()
	srv.handleLatest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	var got collector.Snapshot
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.MemPercent != 77.0 {
		t.Errorf("MemPercent: got %v, want 77", got.MemPercent)
	}
}

func TestHandleConfig(t *testing.T) {
	srv, _ := newTestServer(t)
	w := httptest.NewRecorder()
	srv.handleConfig(w, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	var cfg struct {
		RetentionSeconds int64 `json:"retention_seconds"`
	}
	if err := json.NewDecoder(w.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cfg.RetentionSeconds != 86400 {
		t.Errorf("retention_seconds: got %d, want 86400", cfg.RetentionSeconds)
	}
}

type fakeAlerts struct {
	ignored []string
}

func (f *fakeAlerts) Status() (alerts.Status, error) {
	return alerts.Status{Rules: alerts.Rules{CPUIgnore: f.ignored}}, nil
}
func (f *fakeAlerts) Ignore(p string) error { f.ignored = append(f.ignored, p); return nil }
func (f *fakeAlerts) Unignore(p string) error {
	f.ignored = slices.DeleteFunc(f.ignored, func(s string) bool { return s == p })
	return nil
}

func TestIgnoreEndpoints(t *testing.T) {
	srv, _ := newTestServer(t)
	fake := srv.alerts.(*fakeAlerts)
	do := func(method, url, contentType, body string) int {
		req := httptest.NewRequest(method, url, strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)
		return w.Code
	}

	if code := do("POST", "/api/alerts/ignore", "text/plain", `{"process":"x"}`); code != http.StatusUnsupportedMediaType {
		t.Errorf("non-JSON POST: got %d", code)
	}
	if code := do("POST", "/api/alerts/ignore", "application/json", `{"process":" "}`); code != http.StatusBadRequest {
		t.Errorf("blank process: got %d", code)
	}
	if code := do("POST", "/api/alerts/ignore", "application/json", `{"process":"ffmpeg"}`); code != http.StatusNoContent {
		t.Errorf("ignore: got %d", code)
	}
	if code := do("GET", "/api/alerts", "", ""); code != http.StatusOK || !slices.Equal(fake.ignored, []string{"ffmpeg"}) {
		t.Errorf("status: got %d, ignored %v", code, fake.ignored)
	}
	if code := do("DELETE", "/api/alerts/ignore?process=ffmpeg", "", ""); code != http.StatusNoContent || len(fake.ignored) != 0 {
		t.Errorf("unignore: got %d, ignored %v", code, fake.ignored)
	}
}
