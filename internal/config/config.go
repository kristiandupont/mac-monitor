package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultRetention = 24 * time.Hour

type Config struct {
	// How long snapshots are kept, e.g. "24h" or "7d".
	Retention Duration `json:"retention"`
	Alerts    Alerts   `json:"alerts"`
}

type Alerts struct {
	// Show macOS notifications.
	Native bool `json:"native"`
	// Shell command run for each notification (via /bin/sh -c). Details are
	// passed in MM_ALERT_* environment variables and as JSON on stdin. Exit
	// status 0 means delivered; anything else is retried.
	Command string `json:"command"`
	// How often to remind about an alert that is still active.
	Repeat Duration  `json:"repeat"`
	CPU    CPUAlert  `json:"cpu"`
	Disk   DiskAlert `json:"disk"`
}

type CPUAlert struct {
	// Percent of one core (as in Activity Monitor); 0 disables the rule.
	Percent float64 `json:"percent"`
	// How long a process must stay above Percent before alerting.
	Duration Duration `json:"duration"`
	// Process names never alerted on (case-insensitive).
	Ignore []string `json:"ignore"`
}

type DiskAlert struct {
	// Used-space percentages; 0 disables a level.
	WarnPercent     float64 `json:"warn_percent"`
	CriticalPercent float64 `json:"critical_percent"`
}

func Default() Config {
	return Config{
		Retention: Duration(DefaultRetention),
		Alerts: Alerts{
			Native: true,
			Repeat: Duration(24 * time.Hour),
			CPU: CPUAlert{
				Percent:  80,
				Duration: Duration(5 * time.Minute),
				Ignore:   []string{},
			},
			Disk: DiskAlert{WarnPercent: 90, CriticalPercent: 95},
		},
	}
}

// Load reads the config at path. A missing file is created with defaults so
// the available settings are discoverable; missing keys keep their defaults.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, write(path, cfg)
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("%s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return Default(), fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

func (c Config) validate() error {
	switch {
	case c.Retention <= 0:
		return errors.New("retention must be positive")
	case c.Alerts.Repeat <= 0:
		return errors.New("alerts.repeat must be positive")
	case c.Alerts.CPU.Percent < 0:
		return errors.New("alerts.cpu.percent must not be negative")
	case c.Alerts.CPU.Duration <= 0:
		return errors.New("alerts.cpu.duration must be positive")
	case c.Alerts.Disk.WarnPercent < 0 || c.Alerts.Disk.CriticalPercent < 0:
		return errors.New("alerts.disk percentages must not be negative")
	}
	return nil
}

func write(path string, cfg Config) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false) // keep shell commands readable (">", "&")
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	// Write-then-rename so a crash never leaves a truncated config behind.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Store holds the live config for settings that can change at runtime (the
// ignore list) and persists changes back to the file.
type Store struct {
	mu   sync.RWMutex
	path string
	cfg  Config
	// Set when the file on disk couldn't be loaded. Saving would replace
	// the user's (broken) edits with defaults, so updates are refused.
	loadErr error
}

func NewStore(path string, cfg Config) *Store {
	return &Store{path: path, cfg: cfg}
}

// Open loads path into a Store. On error the store holds defaults and
// refuses updates until the file is fixed and the app restarted.
func Open(path string) (*Store, error) {
	cfg, err := Load(path)
	return &Store{path: path, cfg: cfg, loadErr: err}, err
}

// Get returns a copy of the current config.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.cfg
	cfg.Alerts.CPU.Ignore = slices.Clone(cfg.Alerts.CPU.Ignore)
	return cfg
}

// Update applies fn to the config and saves it.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return fmt.Errorf("not saving settings because %s could not be loaded: %w", s.path, s.loadErr)
	}
	next := s.cfg
	next.Alerts.CPU.Ignore = slices.Clone(next.Alerts.CPU.Ignore)
	fn(&next)
	if err := write(s.path, next); err != nil {
		return err
	}
	s.cfg = next
	return nil
}

// Duration is a time.Duration that (un)marshals as a string and, in addition
// to Go's units, accepts a "d" suffix for days.
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	v := time.Duration(d)
	if v%(24*time.Hour) == 0 {
		return json.Marshal(fmt.Sprintf("%dd", v/(24*time.Hour)))
	}
	// time.Duration prints "5m0s"; drop the zero units for readability.
	str := v.String()
	if strings.HasSuffix(str, "m0s") {
		str = strings.TrimSuffix(str, "0s")
	}
	if strings.HasSuffix(str, "h0m") {
		str = strings.TrimSuffix(str, "0m")
	}
	return json.Marshal(str)
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("duration must be a string like \"24h\" or \"7d\"")
	}
	v, err := ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

func ParseDuration(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.ParseFloat(days, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(n * float64(24*time.Hour)), nil
	}
	return time.ParseDuration(s)
}
