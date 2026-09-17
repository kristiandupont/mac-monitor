# `internal/config` AGENTS.md

**Purpose**: Loads user settings from `config.json` in the app's data directory (next to the database).

**Notes**:
- JSON (stdlib only). A missing file is written out with defaults so users can discover the settings; missing keys keep their defaults.
- On a malformed file `Load` returns defaults *and* an error — the caller logs it and keeps running rather than refusing to start.
- `Duration` accepts Go duration strings plus a `d` (days) suffix.

**Key Files**:
- `config.go`: `Config`, `Load`, `Duration`.

**Relationships**: Used by `cmd/mac-monitor`, which passes individual values on to `storage` and `server`.
