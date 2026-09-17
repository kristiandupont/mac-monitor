# `internal/alerts` AGENTS.md

**Purpose**: Decides when something is worth telling the user about, remembers what they've been told, and delivers notifications reliably.

**Notes**:
- `Engine.set` is the single state transition. Notify on a new episode, on escalation (level above anything notified this episode), or after `alerts.repeat`. Text-only changes stay in memory, so a disk alert doesn't write to the DB every 5 s.
- Disk (`disk.go`) runs on every collected snapshot, so it must stay cheap. CPU (`cpu.go`) samples per-process CPU time every 30 s. It only tracks processes above the threshold and looks names up only for those.
- CPU hysteresis: fires after `duration` above `percent`, clears below `percent/2`. After a restart or a missed-sample gap (sleep), a process of an already-active alert is picked up as still firing instead of resolving and re-notifying.
- Delivery (`delivery.go`) is an outbox: one row per channel, retried with `retryBackoff`, then `failed`. The worker sleeps until the next due attempt or a kick; with nothing pending it never wakes.
- The command channel passes data via env/stdin only, never interpolated into the command.
- Tests (`engine_test.go`) use a real SQLite DB with a fake clock and fake process sampler.

**Key Files**:
- `engine.go`: `Engine`, alert state machine, ignore list, `Status` for the API.
- `disk.go`, `cpu.go`: the rules.
- `delivery.go`: delivery worker, retry policy, shell command channel.

**Relationships**: Uses `internal/storage` (state + queue), `internal/config` (`Store`, for the live ignore list), and `internal/collector` (process sampling). The native channel is injected by `cmd/mac-monitor` (from `internal/notify`). Exposed over HTTP by `internal/server`.
