# `internal/storage` AGENTS.md

**Purpose**: SQLite persistence for metric snapshots — insert, query by time range, latest, and TTL pruning.

**Notes**:
- `db.SetMaxOpenConns(1)` is required; SQLite only supports a single concurrent writer.
- Schema evolution uses `addColumnIfMissing` rather than versioned migrations — safe for additive changes only.
- Slice fields (`cpu_per_core`, `net_stats`, etc.) are stored as JSON strings.

**Key Files**:
- `storage.go`: Only file — `DB` type, `Open`, `Insert`, `Query`, `Latest`, `Prune`.

**Relationships**: Depends on `internal/collector.Snapshot` as the data model. Used by `internal/server` and `cmd/mac-monitor`.
