# `internal/tray` AGENTS.md

**Purpose**: macOS menu bar icon — an animated spinning fan that heat-maps to red with CPU load.

**Notes**:
- Requires CGO and Cocoa/QuartzCore frameworks. Not unit-testable.
- The fan SVG is embedded at build time and rendered to a PNG; Cocoa's `CALayer` tints it at runtime so only one base image is needed for both light and dark mode.
- `animate()` uses exponential smoothing (tau constants) for velocity and color so the animation feels physical rather than snappy.
- `exports.go` exports a C-callable Go function (`menuItemClicked`) that bridges the Objective-C menu callback into a Go channel.

**Key Files**:
- `tray.go`: Animation loop, color interpolation, CGO bridge to Cocoa.
- `exports.go`: `//export menuItemClicked` — the Obj-C → Go callback bridge.
- `statusbar.h` / `statusbar.m`: Objective-C NSStatusItem, NSMenu, and CALayer setup.

**Relationships**: Depends on nothing from `internal/`. Receives a `cancel` func from `cmd/mac-monitor/main.go` to trigger graceful shutdown.
