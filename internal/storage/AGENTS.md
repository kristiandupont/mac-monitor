# `internal/storage` AGENTS.md

**Purpose**: SQLite persistence for metric snapshots — insert, query by time range, latest, and TTL pruning.

**Notes**:
- `db.SetMaxOpenConns(1)` is required; SQLite only supports a single concurrent writer.
- Schema evolution uses `addColumnIfMissing` rather than versioned migrations — safe for additive changes only.
- Slice fields (`cpu_per_core`, `net_stats`, etc.) are stored as JSON strings.

- `QueryDownsampled` groups rows into `ts / step` buckets: counters (net/disk JSON) come from the first row in each bucket, gauges (CPU, memory, load, first GPU) are bucket averages. Empty buckets are omitted so gaps survive.

- `Prune` also returns free pages to the filesystem (`auto_vacuum = INCREMENTAL`). Databases created before this get a one-time full `VACUUM` on their first prune.

**Key Files**:
- `storage.go`: Only file — `DB` type, `Open`, `Insert`, `Query`, `QueryDownsampled`, `Latest`, `Prune`.

**Relationships**: Depends on `internal/collector.Snapshot` as the data model. Used by `internal/server` and `cmd/mac-monitor`.
