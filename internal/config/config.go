package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultRetention = 24 * time.Hour

type Config struct {
	// How long snapshots are kept, e.g. "24h" or "7d".
	Retention Duration `json:"retention"`
}

func Default() Config {
	return Config{Retention: Duration(DefaultRetention)}
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
	if cfg.Retention <= 0 {
		return Default(), fmt.Errorf("%s: retention must be positive", path)
	}
	return cfg, nil
}

func write(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

// Duration is a time.Duration that (un)marshals as a string and, in addition
// to Go's units, accepts a "d" suffix for days.
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	v := time.Duration(d)
	if v%(24*time.Hour) == 0 {
		return json.Marshal(fmt.Sprintf("%dd", v/(24*time.Hour)))
	}
	return json.Marshal(v.String())
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
