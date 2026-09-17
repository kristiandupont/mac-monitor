# `internal/server` AGENTS.md

**Purpose**: HTTP + WebSocket server exposing live and historical metrics to the web UI.

**Notes**:
- WebSocket upgrader allows all origins — acceptable for a local-only tool, not safe for public deployment.
- `Hub` uses a buffered channel (`cap=16`) per client with a drop-on-full policy so slow or stalled clients never block `Broadcast`.
- Static files are served from an `fs.FS` passed at construction time (an embedded FS in production; `os.DirFS` in tests).

**Key Files**:
- `server.go`: Only file — `Hub`, `Server`, WebSocket handler, history/latest/processes/config HTTP handlers. `/api/config` exposes the retention so the UI can bound zooming.

**Notes**:
- `/api/processes` calls `collector.CollectProcesses()` on each request — no background goroutine; CPU cost is zero when the tab is not open.

- `/api/alerts` (GET status, POST/DELETE `/api/alerts/ignore`) goes through the `AlertService` interface. The POST requires `Content-Type: application/json` so cross-origin pages can't change it without a (refused) preflight. Alert status never includes the shell command, since it may hold secrets and the server listens on all interfaces.

**Relationships**: Depends on `internal/storage` for history queries and `internal/collector.Snapshot` as the broadcast payload. `/api/processes` calls `collector.CollectProcesses()` directly.
