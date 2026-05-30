# `internal/webui` AGENTS.md

**Purpose**: Embeds the compiled web frontend into the binary so the app is self-contained.

**Notes**:
- `dist/` is populated by `make build` (copies `web/dist/`). It is gitignored except for the `.gitkeep` placeholder — `go build` will fail if `dist/` is empty.
- Uses `//go:embed all:dist` so the `.gitkeep` placeholder is included and the directive never errors on a freshly-checked-out repo.

**Key Files**:
- `webui.go`: Exposes `FS() fs.FS` — a sub-FS rooted at `dist/`, ready to pass to `http.FileServer`.
