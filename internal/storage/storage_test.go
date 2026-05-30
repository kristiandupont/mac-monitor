package storage

import (
	"testing"
	"time"

	"mac-monitor/internal/collector"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
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

func TestInsertAndQuery(t *testing.T) {
	db := newTestDB(t)

	now := time.Now().Unix()
	snap := &collector.Snapshot{
		Timestamp:   now,
		CPUPercent:  42.5,
		CPUPerCore:  []float64{40.0, 45.0},
		MemTotal:    8 << 30,
		MemUsed:     4 << 30,
		MemPercent:  50.0,
		NetStats:    []collector.NetStat{{Name: "en0", BytesSent: 100, BytesRecv: 200}},
		GPUStats:    []collector.GPUStat{},
		DiskStats:   []collector.DiskStat{},
		DiskIOStats: []collector.DiskIOStat{},
	}

	if err := db.Insert(snap); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	snaps, err := db.Query(now-1, now+1)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("got %d snapshots, want 1", len(snaps))
	}
	got := snaps[0]
	if got.CPUPercent != snap.CPUPercent {
		t.Errorf("CPUPercent: got %v, want %v", got.CPUPercent, snap.CPUPercent)
	}
	if len(got.CPUPerCore) != 2 {
		t.Errorf("CPUPerCore len: got %d, want 2", len(got.CPUPerCore))
	}
	if len(got.NetStats) != 1 || got.NetStats[0].Name != "en0" {
		t.Errorf("NetStats: got %+v", got.NetStats)
	}
}

func TestQueryRangeExclusion(t *testing.T) {
	db := newTestDB(t)

	if err := db.Insert(emptySnap(100)); err != nil {
		t.Fatal(err)
	}
	if err := db.Insert(emptySnap(200)); err != nil {
		t.Fatal(err)
	}
	if err := db.Insert(emptySnap(300)); err != nil {
		t.Fatal(err)
	}

	snaps, err := db.Query(150, 250)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].Timestamp != 200 {
		t.Errorf("expected only ts=200, got %v", snaps)
	}
}

func TestLatest(t *testing.T) {
	db := newTestDB(t)

	if err := db.Insert(emptySnap(1000)); err != nil {
		t.Fatal(err)
	}
	snap2 := emptySnap(2000)
	snap2.CPUPercent = 99
	if err := db.Insert(snap2); err != nil {
		t.Fatal(err)
	}

	latest, err := db.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if latest == nil {
		t.Fatal("Latest returned nil")
	}
	if latest.CPUPercent != 99 {
		t.Errorf("Latest CPUPercent: got %v, want 99", latest.CPUPercent)
	}
}

func TestLatestOnEmptyDB(t *testing.T) {
	db := newTestDB(t)
	snap, err := db.Latest()
	if err != nil {
		t.Fatalf("Latest on empty db: %v", err)
	}
	if snap != nil {
		t.Errorf("expected nil, got %+v", snap)
	}
}

func TestPrune(t *testing.T) {
	db := newTestDB(t)

	// Timestamp far in the past — will be pruned.
	if err := db.Insert(emptySnap(1000)); err != nil {
		t.Fatal(err)
	}
	// Recent snapshot — survives.
	recentTS := time.Now().Unix()
	if err := db.Insert(emptySnap(recentTS)); err != nil {
		t.Fatal(err)
	}

	if err := db.Prune(time.Hour); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	snaps, err := db.Query(0, time.Now().Unix()+1)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 {
		t.Errorf("after Prune: got %d snapshots, want 1", len(snaps))
	}
	if snaps[0].Timestamp != recentTS {
		t.Errorf("wrong snapshot survived prune: ts=%d", snaps[0].Timestamp)
	}
}
