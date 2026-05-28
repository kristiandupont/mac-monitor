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
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			ts           INTEGER NOT NULL,
			cpu_percent  REAL    NOT NULL,
			cpu_per_core TEXT    NOT NULL,
			mem_total    INTEGER NOT NULL,
			mem_used     INTEGER NOT NULL,
			mem_percent  REAL    NOT NULL,
			swap_total   INTEGER NOT NULL,
			swap_used    INTEGER NOT NULL,
			swap_percent REAL    NOT NULL,
			load_1       REAL    NOT NULL,
			load_5       REAL    NOT NULL,
			load_15      REAL    NOT NULL,
			net_stats    TEXT    NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_snapshots_ts ON snapshots(ts);
	`)
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
	_, err = d.db.Exec(`
		INSERT INTO snapshots
			(ts, cpu_percent, cpu_per_core, mem_total, mem_used, mem_percent,
			 swap_total, swap_used, swap_percent, load_1, load_5, load_15, net_stats)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Timestamp, s.CPUPercent, string(cores),
		s.MemTotal, s.MemUsed, s.MemPercent,
		s.SwapTotal, s.SwapUsed, s.SwapPercent,
		s.Load1, s.Load5, s.Load15,
		string(nets),
	)
	return err
}

func (d *DB) Query(from, to int64) ([]*collector.Snapshot, error) {
	rows, err := d.db.Query(`
		SELECT ts, cpu_percent, cpu_per_core, mem_total, mem_used, mem_percent,
		       swap_total, swap_used, swap_percent, load_1, load_5, load_15, net_stats
		FROM snapshots
		WHERE ts >= ? AND ts <= ?
		ORDER BY ts`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snaps []*collector.Snapshot
	for rows.Next() {
		var s collector.Snapshot
		var coresJSON, netsJSON string
		err := rows.Scan(
			&s.Timestamp, &s.CPUPercent, &coresJSON,
			&s.MemTotal, &s.MemUsed, &s.MemPercent,
			&s.SwapTotal, &s.SwapUsed, &s.SwapPercent,
			&s.Load1, &s.Load5, &s.Load15, &netsJSON,
		)
		if err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(coresJSON), &s.CPUPerCore)
		json.Unmarshal([]byte(netsJSON), &s.NetStats)
		snaps = append(snaps, &s)
	}
	return snaps, rows.Err()
}

func (d *DB) Latest() (*collector.Snapshot, error) {
	row := d.db.QueryRow(`
		SELECT ts, cpu_percent, cpu_per_core, mem_total, mem_used, mem_percent,
		       swap_total, swap_used, swap_percent, load_1, load_5, load_15, net_stats
		FROM snapshots ORDER BY ts DESC LIMIT 1`)
	var s collector.Snapshot
	var coresJSON, netsJSON string
	err := row.Scan(
		&s.Timestamp, &s.CPUPercent, &coresJSON,
		&s.MemTotal, &s.MemUsed, &s.MemPercent,
		&s.SwapTotal, &s.SwapUsed, &s.SwapPercent,
		&s.Load1, &s.Load5, &s.Load15, &netsJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest: %w", err)
	}
	json.Unmarshal([]byte(coresJSON), &s.CPUPerCore)
	json.Unmarshal([]byte(netsJSON), &s.NetStats)
	return &s, nil
}

func (d *DB) Prune(age time.Duration) error {
	cutoff := time.Now().Add(-age).Unix()
	_, err := d.db.Exec("DELETE FROM snapshots WHERE ts < ?", cutoff)
	return err
}
