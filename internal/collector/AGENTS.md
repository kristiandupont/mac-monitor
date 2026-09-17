# `internal/collector` AGENTS.md

**Purpose**: Collects a point-in-time snapshot of system metrics (CPU, memory, network, disk, GPU).

**Notes**:
- GPU stats (`gpu.go`) read `IOAccelerator` → `PerformanceStatistics` from the IORegistry via cgo (what `ioreg -rc IOAccelerator` prints, ~500× cheaper than spawning it). The utilization figures appear to cover the time since the previous read by *any* client, so back-to-back reads return 0.
- Collection runs every 5 s for the life of the app: never spawn processes or do per-process work here. Background cost is a core constraint (see root README).
- Network interfaces with zero bytes in both directions are dropped (`netStatsFrom`).
- Disk filtering (`disk.go`) excludes APFS synthetic mounts (`/System/Volumes/*`) to avoid double-counting space shared with `/`.
- `Snapshot` is the shared data model consumed by both `storage` and `server`.

**Key Files**:
- `collector.go`: `Snapshot` type definition and `Collect()` entry point.
- `disk.go`: Disk usage and I/O stats; `isUserFacingMount` filtering logic.
- `gpu.go`: GPU utilization via IOKit (cgo).
- `proccpu.go`: `ProcessCPUTimes()` (one sysctl + `proc_pidinfo` per process, ~1 ms) and `ProcessName()` for the alert engine's background sampling. Only the current user's processes are readable.
- `processes.go`: `ProcessStat` type and `CollectProcesses()` — on-demand per-process CPU/memory snapshot. Maintains a package-level CPU-times cache so delta-based CPU% is accurate across repeated calls.

**Relationships**: No dependencies on other internal packages. `Snapshot` type is imported by `internal/storage` and `internal/server`.
