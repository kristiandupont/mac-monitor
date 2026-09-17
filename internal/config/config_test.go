package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadCreatesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if time.Duration(cfg.Retention) != DefaultRetention {
		t.Errorf("retention: got %v, want %v", time.Duration(cfg.Retention), DefaultRetention)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("default config not written: %v", err)
	}
	var onDisk Config
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatalf("default file: %v", err)
	}
	if !reflect.DeepEqual(onDisk, Default()) {
		t.Errorf("default file does not round-trip: %s", data)
	}
}

func TestLoadRetention(t *testing.T) {
	for in, want := range map[string]time.Duration{
		`{"retention": "7d"}`:   7 * 24 * time.Hour,
		`{"retention": "12h"}`:  12 * time.Hour,
		`{"retention": "1.5d"}`: 36 * time.Hour,
		`{}`:                    DefaultRetention,
	} {
		path := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(path, []byte(in), 0600)
		cfg, err := Load(path)
		if err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if time.Duration(cfg.Retention) != want {
			t.Errorf("%s: got %v, want %v", in, time.Duration(cfg.Retention), want)
		}
	}
}

func TestLoadInvalid(t *testing.T) {
	for _, in := range []string{`{"retention": "soon"}`, `{"retention": 5}`, `{"retention": "-1h"}`, `not json`, `{"alerts": {"cpu": {"duration": "0s"}}}`} {
		path := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(path, []byte(in), 0600)
		cfg, err := Load(path)
		if err == nil {
			t.Errorf("%s: expected error", in)
		}
		if time.Duration(cfg.Retention) != DefaultRetention {
			t.Errorf("%s: expected defaults on error", in)
		}
	}
}

func TestPartialAlertsKeepDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{"alerts": {"cpu": {"percent": 50}}}`), 0600)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Alerts.CPU.Percent != 50 || time.Duration(cfg.Alerts.CPU.Duration) != 5*time.Minute || !cfg.Alerts.Native {
		t.Errorf("got %+v", cfg.Alerts)
	}
}

func TestStoreUpdatePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, _ := Load(path)
	store := NewStore(path, cfg)
	if err := store.Update(func(c *Config) { c.Alerts.CPU.Ignore = append(c.Alerts.CPU.Ignore, "ffmpeg") }); err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	got.Alerts.CPU.Ignore[0] = "mutated" // must not leak into the store
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"ffmpeg"}; !reflect.DeepEqual(reloaded.Alerts.CPU.Ignore, want) || !reflect.DeepEqual(store.Get().Alerts.CPU.Ignore, want) {
		t.Errorf("ignore list: file %v, store %v", reloaded.Alerts.CPU.Ignore, store.Get().Alerts.CPU.Ignore)
	}
}

func TestOpenBrokenFileIsReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{"retention": "soon"}`), 0600)
	store, err := Open(path)
	if err == nil {
		t.Fatal("expected load error")
	}
	if err := store.Update(func(c *Config) { c.Alerts.CPU.Ignore = []string{"x"} }); err == nil {
		t.Error("update should be refused")
	}
	if data, _ := os.ReadFile(path); string(data) != `{"retention": "soon"}` {
		t.Errorf("broken file was overwritten: %s", data)
	}
}

func TestWriteIsReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	cfg.Alerts.Command = `echo "$MM_ALERT_BODY" >> /tmp/log && true`
	cfg.Alerts.CPU.Duration = Duration(90 * time.Minute)
	cfg.Alerts.Repeat = Duration(2 * time.Hour)
	if err := write(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{`>> /tmp/log && true`, `"duration": "1h30m"`, `"repeat": "2h"`, `"retention": "1d"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %s in:\n%s", want, data)
		}
	}
}
