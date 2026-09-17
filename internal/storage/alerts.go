package storage

import (
	"database/sql"
	"time"
)

// Alert is the persisted state of one alert (e.g. "disk:/" or "cpu:ffmpeg").
// Level 0 means resolved.
type Alert struct {
	Key           string `json:"key"`
	Kind          string `json:"kind"`
	Subject       string `json:"subject"`
	Level         int    `json:"level"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	StartedAt     int64  `json:"started_at"`
	UpdatedAt     int64  `json:"updated_at"`
	NotifiedLevel int    `json:"notified_level"`
	NotifiedAt    int64  `json:"notified_at"`
}

// Notification is one delivery attempt queue entry for one channel.
type Notification struct {
	ID            int64  `json:"id"`
	AlertKey      string `json:"alert_key"`
	Kind          string `json:"kind"`
	Subject       string `json:"subject"`
	Level         int    `json:"level"`
	Channel       string `json:"channel"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	Status        string `json:"status"` // pending | sent | failed
	Attempts      int    `json:"attempts"`
	NextAttemptAt int64  `json:"next_attempt_at"`
	LastError     string `json:"last_error"`
	CreatedAt     int64  `json:"created_at"`
	SentAt        int64  `json:"sent_at"`
}

const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)

func migrateAlerts(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS alerts (
			key            TEXT PRIMARY KEY,
			kind           TEXT    NOT NULL,
			subject        TEXT    NOT NULL,
			level          INTEGER NOT NULL,
			title          TEXT    NOT NULL,
			body           TEXT    NOT NULL,
			started_at     INTEGER NOT NULL,
			updated_at     INTEGER NOT NULL,
			notified_level INTEGER NOT NULL,
			notified_at    INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS notifications (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			alert_key       TEXT    NOT NULL,
			kind            TEXT    NOT NULL,
			subject         TEXT    NOT NULL,
			level           INTEGER NOT NULL,
			channel         TEXT    NOT NULL,
			title           TEXT    NOT NULL,
			body            TEXT    NOT NULL,
			status          TEXT    NOT NULL,
			attempts        INTEGER NOT NULL DEFAULT 0,
			next_attempt_at INTEGER NOT NULL,
			last_error      TEXT    NOT NULL DEFAULT '',
			created_at      INTEGER NOT NULL,
			sent_at         INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_notifications_due ON notifications(status, next_attempt_at);
	`)
	return err
}

const alertColumns = `key, kind, subject, level, title, body, started_at, updated_at, notified_level, notified_at`

func (d *DB) Alerts() ([]Alert, error) {
	rows, err := d.db.Query(`SELECT ` + alertColumns + ` FROM alerts ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	alerts := []Alert{}
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.Key, &a.Kind, &a.Subject, &a.Level, &a.Title, &a.Body,
			&a.StartedAt, &a.UpdatedAt, &a.NotifiedLevel, &a.NotifiedAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// SaveAlert upserts a and, in the same transaction, queues notifications so a
// crash can never record "notified" without the notification being queued.
func (d *DB) SaveAlert(a Alert, notify []Notification) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT OR REPLACE INTO alerts (`+alertColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Key, a.Kind, a.Subject, a.Level, a.Title, a.Body, a.StartedAt, a.UpdatedAt, a.NotifiedLevel, a.NotifiedAt)
	if err != nil {
		return err
	}
	for _, n := range notify {
		_, err := tx.Exec(`
			INSERT INTO notifications
				(alert_key, kind, subject, level, channel, title, body, status, next_attempt_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			n.AlertKey, n.Kind, n.Subject, n.Level, n.Channel, n.Title, n.Body, StatusPending, n.NextAttemptAt, n.CreatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

const notificationColumns = `id, alert_key, kind, subject, level, channel, title, body, status,
	attempts, next_attempt_at, last_error, created_at, sent_at`

func (d *DB) queryNotifications(query string, args ...any) ([]Notification, error) {
	rows, err := d.db.Query(`SELECT `+notificationColumns+` FROM notifications `+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ns := []Notification{}
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.AlertKey, &n.Kind, &n.Subject, &n.Level, &n.Channel, &n.Title, &n.Body,
			&n.Status, &n.Attempts, &n.NextAttemptAt, &n.LastError, &n.CreatedAt, &n.SentAt); err != nil {
			return nil, err
		}
		ns = append(ns, n)
	}
	return ns, rows.Err()
}

// DueNotifications returns pending notifications whose next attempt is due.
func (d *DB) DueNotifications(now int64) ([]Notification, error) {
	return d.queryNotifications(`WHERE status = ? AND next_attempt_at <= ? ORDER BY id`, StatusPending, now)
}

// NextAttemptAt returns when the earliest pending notification is due.
func (d *DB) NextAttemptAt() (int64, bool, error) {
	var next sql.NullInt64
	err := d.db.QueryRow(`SELECT MIN(next_attempt_at) FROM notifications WHERE status = ?`, StatusPending).Scan(&next)
	return next.Int64, next.Valid, err
}

func (d *DB) RecentNotifications(limit int) ([]Notification, error) {
	return d.queryNotifications(`ORDER BY id DESC LIMIT ?`, limit)
}

// UpdateNotification records the outcome of a delivery attempt.
func (d *DB) UpdateNotification(n Notification) error {
	_, err := d.db.Exec(`
		UPDATE notifications
		SET status = ?, attempts = ?, next_attempt_at = ?, last_error = ?, sent_at = ?
		WHERE id = ?`,
		n.Status, n.Attempts, n.NextAttemptAt, n.LastError, n.SentAt, n.ID)
	return err
}

// pruneAlerts drops resolved alerts and finished notifications older than cutoff.
func pruneAlerts(db *sql.DB, cutoff time.Time) error {
	c := cutoff.Unix()
	if _, err := db.Exec(`DELETE FROM alerts WHERE level = 0 AND updated_at < ?`, c); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM notifications WHERE status != ? AND created_at < ?`, StatusPending, c)
	return err
}
