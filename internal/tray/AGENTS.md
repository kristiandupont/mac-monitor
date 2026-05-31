# `internal/tray` AGENTS.md

**Purpose**: macOS menu bar icon — an animated spinning fan that heat-maps to red with CPU load — plus the status-bar menu (Open Dashboard, Animate Icon, Launch at Login, Quit).

**Notes**:
- Requires CGO and Cocoa/QuartzCore/ServiceManagement frameworks. Not unit-testable.
- The fan SVG is embedded at build time and rendered to a PNG; Cocoa's `CALayer` tints it at runtime so only one base image is needed for both light and dark mode.
- `animate()` uses exponential smoothing (tau constants) for velocity and color so the animation feels physical rather than snappy.
- `exports.go` exports a C-callable Go function (`onMenuItemClicked`) that bridges the Objective-C menu callback into a Go channel.
- "Animate Icon" preference is persisted in `NSUserDefaults`; "Launch at Login" state is owned by `SMAppService` and re-read after each toggle to reflect whether registration succeeded.
- `SMAppService` (Launch at Login) only works when the process is running inside a signed `.app` bundle — it silently no-ops with the bare binary during development.

**Key Files**:
- `tray.go`: Animation loop, color interpolation, CGO bridge to Cocoa.
- `exports.go`: `//export onMenuItemClicked` — the Obj-C → Go callback bridge.
- `statusbar.h` / `statusbar.m`: Objective-C NSStatusItem, NSMenu, CALayer, NSUserDefaults, and SMAppService setup.

**Relationships**: Depends on nothing from `internal/`. Receives a `cancel` func from `cmd/mac-monitor/main.go` to trigger graceful shutdown.
