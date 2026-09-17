package config

import (
	"os"
	"path/filepath"
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
	if string(data) != "{\n  \"retention\": \"1d\"\n}\n" {
		t.Errorf("unexpected default file: %q", data)
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
	for _, in := range []string{`{"retention": "soon"}`, `{"retention": 5}`, `{"retention": "-1h"}`, `not json`} {
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
