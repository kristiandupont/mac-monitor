package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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
	return New(db, NewHub(), os.DirFS(".")), db
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
