# Mac Monitor

A lightweight system monitor for macOS servers. Collects CPU, memory, GPU, disk, and network metrics at 5-second intervals, stores them for up to 30 days, and exposes a web UI for both live and historical inspection.

## Running

```bash
# Development (Go server on :8080, Vite dev server on :5173)
make dev

# Production
make build        # builds web/dist then compiles the binary
./mac-monitor     # serves everything on http://localhost:8080
```

The binary serves the compiled frontend from `web/dist/` and the database is written to `mac-monitor.db` in the working directory.

## Architecture

```
┌─────────────────────────────────┐
│  Collector goroutines (5s tick) │
│  CPU · Memory · GPU · Disk · Net│
└──────────────┬──────────────────┘
               │
               ▼
        ┌─────────────┐
        │  SQLite DB  │  mac-monitor.db
        └──────┬──────┘  30-day rolling retention
               │
               ▼
     ┌──────────────────┐
     │   HTTP server    │  :8080
     │  REST + WebSocket│
     └────────┬─────────┘
              │
              ▼
    ┌──────────────────┐
    │  Vite + CrankJS  │  SPA
    │  Live + history  │
    └──────────────────┘
```

### Go packages

| Path                  | Responsibility                                                                                                                                                                                         |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `cmd/mac-monitor/`    | Entrypoint. Wires together the collector, storage, and HTTP server.                                                                                                                                    |
| `internal/collector/` | Metric collection via [gopsutil](https://github.com/shirou/gopsutil). One file per domain: `collector.go` (Snapshot struct + orchestration), `gpu.go` (IOKit via `ioreg`), `disk.go`.                  |
| `internal/storage/`   | SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGo). Schema migration using `addColumnIfMissing` so new columns can be added without breaking existing databases. |
| `internal/server/`    | `net/http` server with a WebSocket hub for live metric streaming. Historical data served via `/api/history?from=&to=`.                                                                                 |

### Frontend

Built with [Vite](https://vitejs.dev/) and [CrankJS](https://crank.js.org/). CrankJS uses generator functions as components — the `for (props of this)` loop pattern is how components receive prop updates, and `this.flush()` must be called **inside** that loop (not before it) for effects like Chart.js updates to run on every render.

| File                                 | Responsibility                                                                         |
| ------------------------------------ | -------------------------------------------------------------------------------------- |
| `web/src/App.jsx`                    | Root component. Owns the WebSocket connection, history buffer, and top-level layout.   |
| `web/src/components/LineChart.jsx`   | Generic reusable Chart.js line chart. Accepts `datasets`, `yMax`, and `formatY` props. |
| `web/src/components/MetricGauge.jsx` | Horizontal progress bar for a single metric (works with both percentages and bytes).   |
| `web/src/components/GpuCard.jsx`     | GPU utilization and memory (parsed from `ioreg -rc IOAccelerator`).                    |
| `web/src/components/DiskCard.jsx`    | Disk space per user-facing volume (filters out internal APFS system volumes).          |

### Data flow

1. A ticker goroutine calls `collector.Collect()` every 5 seconds, which gathers all metrics into a `Snapshot` struct.
2. The snapshot is written to SQLite and broadcast to all connected WebSocket clients.
3. The frontend seeds the history chart from `GET /api/history` on connect, then appends live snapshots from the WebSocket feed. Network and disk I/O rates are computed client-side as deltas between consecutive snapshots.

### Adding a new metric

1. Add fields to `collector.Snapshot` in `internal/collector/collector.go`.
2. Collect the data (new file in `internal/collector/` if it warrants one).
3. Add a column to the `CREATE TABLE` statement in `internal/storage/storage.go` **and** add the same column to the `addColumnIfMissing` loop so existing databases are migrated automatically.
4. Update `Insert`, `Query`, and `Latest` to include the new column.
5. Add a component or extend an existing one in `web/src/components/`.

### GPU monitoring note

GPU metrics are read by shelling out to `ioreg -rc IOAccelerator -a` and parsing the XML plist. This does not require `sudo` and works on both Intel and Apple Silicon Macs.

Temperature monitoring was explored but removed: on macOS 15+, Apple removed CPU/GPU die temperatures from the `powermetrics` sampler output. Direct SMC access via IOKit (CGo) would be required — left as a future improvement.
