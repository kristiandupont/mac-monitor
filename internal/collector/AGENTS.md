# `internal/collector` AGENTS.md

**Purpose**: Collects a point-in-time snapshot of system metrics (CPU, memory, network, disk, GPU).

**Notes**:
- GPU stats (`gpu.go`) use `ioreg` via `exec.Command` to read IOKit accelerator entries as a plist — Apple Silicon only; gracefully returns nil on other hardware.
- Disk filtering (`disk.go`) excludes APFS synthetic mounts (`/System/Volumes/*`) to avoid double-counting space shared with `/`.
- `Snapshot` is the shared data model consumed by both `storage` and `server`.

**Key Files**:
- `collector.go`: `Snapshot` type definition and `Collect()` entry point.
- `disk.go`: Disk usage and I/O stats; `isUserFacingMount` filtering logic.
- `gpu.go`: GPU utilization via `ioreg` plist parsing.

**Relationships**: No dependencies on other internal packages. `Snapshot` type is imported by `internal/storage` and `internal/server`.
