# `internal/server` AGENTS.md

**Purpose**: HTTP + WebSocket server exposing live and historical metrics to the web UI.

**Notes**:
- WebSocket upgrader allows all origins — acceptable for a local-only tool, not safe for public deployment.
- `Hub` uses a buffered channel (`cap=16`) per client with a drop-on-full policy so slow or stalled clients never block `Broadcast`.
- Static files are served directly from the `web/dist` directory passed at construction time.

**Key Files**:
- `server.go`: Only file — `Hub`, `Server`, WebSocket handler, history/latest HTTP handlers.

**Relationships**: Depends on `internal/storage` for history queries and `internal/collector.Snapshot` as the broadcast payload.
