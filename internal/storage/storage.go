package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"mac-monitor/internal/collector"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: single writer
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS snapshots (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			ts             INTEGER NOT NULL,
			cpu_percent    REAL    NOT NULL,
			cpu_per_core   TEXT    NOT NULL,
			mem_total      INTEGER NOT NULL,
			mem_used       INTEGER NOT NULL,
			mem_percent    REAL    NOT NULL,
			swap_total     INTEGER NOT NULL,
			swap_used      INTEGER NOT NULL,
			swap_percent   REAL    NOT NULL,
			load_1         REAL    NOT NULL,
			load_5         REAL    NOT NULL,
			load_15        REAL    NOT NULL,
			net_stats      TEXT    NOT NULL,
			gpu_stats      TEXT    NOT NULL DEFAULT '[]',
			disk_stats     TEXT    NOT NULL DEFAULT '[]',
			disk_io_stats  TEXT    NOT NULL DEFAULT '[]'
		);
		CREATE INDEX IF NOT EXISTS idx_snapshots_ts ON snapshots(ts);
	`)
	if err != nil {
		return err
	}
	for col, def := range map[string]string{
		"gpu_stats":    "TEXT NOT NULL DEFAULT '[]'",
		"disk_stats":   "TEXT NOT NULL DEFAULT '[]'",
		"disk_io_stats": "TEXT NOT NULL DEFAULT '[]'",
	} {
		if err := addColumnIfMissing(db, "snapshots", col, def); err != nil {
			return err
		}
	}
	return nil
}

func addColumnIfMissing(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}

func (d *DB) Insert(s *collector.Snapshot) error {
	cores, err := json.Marshal(s.CPUPerCore)
	if err != nil {
		return err
	}
	nets, err := json.Marshal(s.NetStats)
	if err != nil {
		return err
	}
	gpus, err := json.Marshal(s.GPUStats)
	if err != nil {
		return err
	}
	disks, err := json.Marshal(s.DiskStats)
	if err != nil {
		return err
	}
	diskIOs, err := json.Marshal(s.DiskIOStats)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`
		INSERT INTO snapshots
			(ts, cpu_percent, cpu_per_core, mem_total, mem_used, mem_percent,
			 swap_total, swap_used, swap_percent, load_1, load_5, load_15,
			 net_stats, gpu_stats, disk_stats, disk_io_stats)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Timestamp, s.CPUPercent, string(cores),
		s.MemTotal, s.MemUsed, s.MemPercent,
		s.SwapTotal, s.SwapUsed, s.SwapPercent,
		s.Load1, s.Load5, s.Load15,
		string(nets), string(gpus), string(disks), string(diskIOs),
	)
	return err
}

const snapshotColumns = `ts, cpu_percent, cpu_per_core, mem_total, mem_used, mem_percent,
	swap_total, swap_used, swap_percent, load_1, load_5, load_15,
	net_stats, gpu_stats, disk_stats, disk_io_stats`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(row rowScanner, extra ...any) (*collector.Snapshot, error) {
	var s collector.Snapshot
	var coresJSON, netsJSON, gpusJSON, disksJSON, diskIOsJSON string
	dest := append([]any{
		&s.Timestamp, &s.CPUPercent, &coresJSON,
		&s.MemTotal, &s.MemUsed, &s.MemPercent,
		&s.SwapTotal, &s.SwapUsed, &s.SwapPercent,
		&s.Load1, &s.Load5, &s.Load15,
		&netsJSON, &gpusJSON, &disksJSON, &diskIOsJSON,
	}, extra...)
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(coresJSON), &s.CPUPerCore)
	json.Unmarshal([]byte(netsJSON), &s.NetStats)
	json.Unmarshal([]byte(gpusJSON), &s.GPUStats)
	json.Unmarshal([]byte(disksJSON), &s.DiskStats)
	json.Unmarshal([]byte(diskIOsJSON), &s.DiskIOStats)
	return &s, nil
}

func (d *DB) Query(from, to int64) ([]*collector.Snapshot, error) {
	rows, err := d.db.Query(`
		SELECT `+snapshotColumns+`
		FROM snapshots
		WHERE ts >= ? AND ts <= ?
		ORDER BY ts`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []*collector.Snapshot
	for rows.Next() {
		s, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, s)
	}
	return snaps, rows.Err()
}

// QueryDownsampled returns at most one snapshot per step-second bucket. Each
// is the first real row in its bucket (so cumulative counters stay consistent
// for rate calculations), with gauge values replaced by the bucket average.
// Buckets without rows are omitted, so gaps in collection are preserved.
func (d *DB) QueryDownsampled(from, to, step int64) ([]*collector.Snapshot, error) {
	if step <= 1 {
		return d.Query(from, to)
	}
	rows, err := d.db.Query(`
		WITH b AS (
			SELECT MIN(id) AS id,
			       AVG(cpu_percent) AS cpu_percent,
			       AVG(mem_used)    AS mem_used,
			       AVG(mem_percent) AS mem_percent,
			       AVG(swap_used)   AS swap_used,
			       AVG(swap_percent) AS swap_percent,
			       AVG(load_1) AS load_1, AVG(load_5) AS load_5, AVG(load_15) AS load_15,
			       AVG(json_extract(gpu_stats, '$[0].device_utilization'))   AS gpu_device,
			       AVG(json_extract(gpu_stats, '$[0].renderer_utilization')) AS gpu_renderer,
			       AVG(json_extract(gpu_stats, '$[0].tiler_utilization'))    AS gpu_tiler
			FROM snapshots
			WHERE ts >= ? AND ts <= ?
			GROUP BY ts / ?
		)
		SELECT s.ts, b.cpu_percent, s.cpu_per_core, s.mem_total, CAST(b.mem_used AS INTEGER), b.mem_percent,
		       s.swap_total, CAST(b.swap_used AS INTEGER), b.swap_percent, b.load_1, b.load_5, b.load_15,
		       s.net_stats, s.gpu_stats, s.disk_stats, s.disk_io_stats,
		       b.gpu_device, b.gpu_renderer, b.gpu_tiler
		FROM b JOIN snapshots s ON s.id = b.id
		ORDER BY s.ts`, from, to, step)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []*collector.Snapshot
	for rows.Next() {
		var gpuDevice, gpuRenderer, gpuTiler sql.NullFloat64
		s, err := scanSnapshot(rows, &gpuDevice, &gpuRenderer, &gpuTiler)
		if err != nil {
			return nil, err
		}
		if len(s.GPUStats) > 0 && gpuDevice.Valid {
			s.GPUStats[0].DeviceUtilization = gpuDevice.Float64
			s.GPUStats[0].RendererUtilization = gpuRenderer.Float64
			s.GPUStats[0].TilerUtilization = gpuTiler.Float64
		}
		snaps = append(snaps, s)
	}
	return snaps, rows.Err()
}

func (d *DB) Latest() (*collector.Snapshot, error) {
	row := d.db.QueryRow(`SELECT ` + snapshotColumns + ` FROM snapshots ORDER BY ts DESC LIMIT 1`)
	s, err := scanSnapshot(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest: %w", err)
	}
	return s, nil
}

func (d *DB) Prune(age time.Duration) error {
	cutoff := time.Now().Add(-age).Unix()
	_, err := d.db.Exec("DELETE FROM snapshots WHERE ts < ?", cutoff)
	return err
}
