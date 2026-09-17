# `internal/notify` AGENTS.md

**Purpose**: Posts macOS notifications for the alert engine.

**Notes**:
- Inside an app bundle it uses `UNUserNotificationCenter` (`notify.m`). Outside one (`go run`, `make dev`) that API throws, so `osascript display notification` is used instead — no action buttons there.
- `Send` blocks until macOS accepts or rejects the request (or ctx ends). Authorization is requested lazily on the first send; a denial comes back as an error so the delivery queue records and retries it.
- CPU notifications (category `cpu`) carry a "Don't Alert for This App" action; the process name travels in `userInfo["subject"]` and comes back via `Init`'s callback.
- The alert key is used as the request identifier, so a newer notification about the same thing replaces the old one in Notification Center.

**Key Files**:
- `notify.go`: Go API (`Init`, `Send`) and the osascript fallback.
- `notify.m` / `notify.h`: Objective-C delegate, category registration, posting.
- `exports.go`: `//export`ed callbacks (kept separate per cgo rules).

**Relationships**: Wired into `internal/alerts` as the native channel by `cmd/mac-monitor`. No internal dependencies.
