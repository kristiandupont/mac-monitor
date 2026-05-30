package collector

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type ProcessStat struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemRSS     uint64  `json:"mem_rss"`
}

type procTimes struct {
	user   float64
	system float64
	at     time.Time
}

// staleThreshold is how long without a call before the cached CPU times are
// considered too old to produce meaningful deltas.
const staleThreshold = 15 * time.Second

var procCache struct {
	mu       sync.Mutex
	times    map[int32]procTimes
	lastCall time.Time
}

func init() {
	procCache.times = make(map[int32]procTimes)
}

func CollectProcesses() ([]ProcessStat, bool, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, false, err
	}

	now := time.Now()
	newTimes := make(map[int32]procTimes, len(procs))
	stats := make([]ProcessStat, 0, len(procs))

	procCache.mu.Lock()
	prev := procCache.times
	ready := !procCache.lastCall.IsZero() && now.Sub(procCache.lastCall) <= staleThreshold
	procCache.mu.Unlock()

	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			name = "?"
		}

		t, err := p.Times()
		if err != nil {
			continue
		}

		user := t.User
		sys := t.System
		newTimes[p.Pid] = procTimes{user: user, system: sys, at: now}

		var cpuPct float64
		if last, ok := prev[p.Pid]; ok {
			elapsed := now.Sub(last.at).Seconds()
			if elapsed > 0 {
				cpuPct = ((user - last.user) + (sys - last.system)) / elapsed * 100
				if cpuPct < 0 {
					cpuPct = 0
				}
			}
		}

		mem, err := p.MemoryInfo()
		var rss uint64
		if err == nil && mem != nil {
			rss = mem.RSS
		}

		stats = append(stats, ProcessStat{
			PID:        p.Pid,
			Name:       name,
			CPUPercent: cpuPct,
			MemRSS:     rss,
		})
	}

	procCache.mu.Lock()
	procCache.times = newTimes
	procCache.lastCall = now
	procCache.mu.Unlock()

	return stats, ready, nil
}
