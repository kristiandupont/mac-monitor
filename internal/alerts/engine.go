package alerts

import (
	"context"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"mac-monitor/internal/collector"
	"mac-monitor/internal/config"
	"mac-monitor/internal/storage"
)

const (
	KindCPU  = "cpu"
	KindDisk = "disk"

	LevelResolved = 0
	LevelWarning  = 1
	LevelCritical = 2

	ChannelNative  = "native"
	ChannelCommand = "command"
)

// Store is the persistence the engine needs; *storage.DB implements it.
type Store interface {
	Alerts() ([]storage.Alert, error)
	SaveAlert(storage.Alert, []storage.Notification) error
	DueNotifications(now int64) ([]storage.Notification, error)
	NextAttemptAt() (int64, bool, error)
	UpdateNotification(storage.Notification) error
	RecentNotifications(limit int) ([]storage.Notification, error)
}

// Channel delivers one notification. A nil error means delivered.
type Channel func(ctx context.Context, n storage.Notification) error

// Engine evaluates alert rules, remembers what the user has been told, and
// delivers notifications with retries.
type Engine struct {
	store    Store
	cfg      *config.Store
	channels map[string]Channel

	// Replaceable in tests.
	now       func() time.Time
	sampleCPU func() (map[int32]time.Duration, error)
	procName  func(int32) string

	mu     sync.Mutex
	alerts map[string]storage.Alert
	cpu    cpuState

	kick chan struct{}
}

// New loads persisted alert state. native may be nil when native
// notifications are unavailable.
func New(store Store, cfg *config.Store, native Channel) (*Engine, error) {
	e := &Engine{
		store:     store,
		cfg:       cfg,
		channels:  map[string]Channel{},
		now:       time.Now,
		sampleCPU: collector.ProcessCPUTimes,
		procName:  collector.ProcessName,
		alerts:    map[string]storage.Alert{},
		kick:      make(chan struct{}, 1),
	}
	e.channels[ChannelCommand] = e.runCommand
	if native != nil {
		e.channels[ChannelNative] = native
	}
	saved, err := store.Alerts()
	if err != nil {
		return nil, err
	}
	for _, a := range saved {
		e.alerts[a.Key] = a
	}
	return e, nil
}

// Run samples processes and delivers notifications until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	go e.runDelivery(ctx)
	e.runCPUSampler(ctx)
}

// set moves the alert identified by key to level, queueing notifications
// when the user should hear about it. Must be called with e.mu held.
func (e *Engine) set(key, kind, subject string, level int, title, body string) {
	now := e.now().Unix()
	prev, exists := e.alerts[key]
	active := exists && prev.Level > LevelResolved

	if level == LevelResolved {
		if !active {
			return
		}
		prev.Level = LevelResolved
		prev.UpdatedAt = now
		e.save(prev, nil)
		return
	}

	a := prev
	if !active {
		// New episode: forget what was notified about the previous one.
		a = storage.Alert{Key: key, Kind: kind, Subject: subject, StartedAt: now}
	}
	levelChanged := a.Level != level
	a.Level, a.Title, a.Body, a.UpdatedAt = level, title, body, now

	cfg := e.cfg.Get().Alerts
	repeat := int64(time.Duration(cfg.Repeat) / time.Second)
	if level <= a.NotifiedLevel && now-a.NotifiedAt < repeat {
		if levelChanged {
			e.save(a, nil)
		} else {
			// Only the text changed (e.g. free bytes); keep it in memory
			// rather than writing to disk on every sample.
			e.alerts[key] = a
		}
		return
	}

	a.NotifiedLevel, a.NotifiedAt = level, now
	var notify []storage.Notification
	for _, ch := range enabledChannels(cfg) {
		if _, ok := e.channels[ch]; !ok {
			continue
		}
		notify = append(notify, storage.Notification{
			AlertKey: key, Kind: kind, Subject: subject, Level: level,
			Channel: ch, Title: title, Body: body,
			NextAttemptAt: now, CreatedAt: now,
		})
	}
	e.save(a, notify)
}

func (e *Engine) save(a storage.Alert, notify []storage.Notification) {
	if err := e.store.SaveAlert(a, notify); err != nil {
		log.Printf("alerts: save %s: %v", a.Key, err)
		return
	}
	e.alerts[a.Key] = a
	if len(notify) > 0 {
		select {
		case e.kick <- struct{}{}:
		default:
		}
	}
}

func enabledChannels(cfg config.Alerts) []string {
	var chs []string
	if cfg.Native {
		chs = append(chs, ChannelNative)
	}
	if strings.TrimSpace(cfg.Command) != "" {
		chs = append(chs, ChannelCommand)
	}
	return chs
}

// Ignore stops CPU alerts for a process name and resolves any active one.
func (e *Engine) Ignore(name string) error {
	name = strings.TrimSpace(name)
	err := e.cfg.Update(func(c *config.Config) {
		if !isIgnored(c.Alerts.CPU.Ignore, name) {
			c.Alerts.CPU.Ignore = append(c.Alerts.CPU.Ignore, name)
		}
	})
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for key, a := range e.alerts {
		if a.Kind == KindCPU && strings.EqualFold(a.Subject, name) {
			e.set(key, a.Kind, a.Subject, LevelResolved, a.Title, a.Body)
		}
	}
	return nil
}

func (e *Engine) Unignore(name string) error {
	return e.cfg.Update(func(c *config.Config) {
		c.Alerts.CPU.Ignore = slices.DeleteFunc(c.Alerts.CPU.Ignore, func(s string) bool {
			return strings.EqualFold(s, name)
		})
	})
}

func isIgnored(list []string, name string) bool {
	return slices.ContainsFunc(list, func(s string) bool { return strings.EqualFold(s, name) })
}

type Status struct {
	Alerts        []storage.Alert        `json:"alerts"`
	Notifications []storage.Notification `json:"notifications"`
	Rules         Rules                  `json:"rules"`
}

// Rules summarises the alert config for the UI. The command itself is left
// out: the server listens on all interfaces and it may contain secrets.
type Rules struct {
	Native          bool             `json:"native"`
	HasCommand      bool             `json:"has_command"`
	CPUPercent      float64          `json:"cpu_percent"`
	CPUDurationSecs int64            `json:"cpu_duration_seconds"`
	CPUIgnore       []string         `json:"cpu_ignore"`
	Disk            config.DiskAlert `json:"disk"`
}

// Status returns active alerts, recently resolved ones and recent deliveries.
func (e *Engine) Status() (Status, error) {
	cfg := e.cfg.Get().Alerts
	st := Status{
		Alerts: []storage.Alert{},
		Rules: Rules{
			Native:          cfg.Native,
			HasCommand:      strings.TrimSpace(cfg.Command) != "",
			CPUPercent:      cfg.CPU.Percent,
			CPUDurationSecs: int64(time.Duration(cfg.CPU.Duration) / time.Second),
			CPUIgnore:       cfg.CPU.Ignore,
			Disk:            cfg.Disk,
		},
	}
	cutoff := e.now().Add(-24 * time.Hour).Unix()
	e.mu.Lock()
	for _, a := range e.alerts {
		if a.Level > LevelResolved || a.UpdatedAt >= cutoff {
			st.Alerts = append(st.Alerts, a)
		}
	}
	e.mu.Unlock()
	slices.SortFunc(st.Alerts, func(a, b storage.Alert) int {
		if a.Level != b.Level {
			return b.Level - a.Level
		}
		return int(b.UpdatedAt - a.UpdatedAt)
	})
	var err error
	st.Notifications, err = e.store.RecentNotifications(50)
	return st, err
}
