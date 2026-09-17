package alerts

import (
	"fmt"

	"mac-monitor/internal/collector"
	"mac-monitor/internal/config"
)

// A level stays active until usage drops this many points below its
// threshold, so hovering around a threshold doesn't flap.
const diskHysteresis = 2.0

// ObserveSnapshot evaluates the disk rule. It is called for every collected
// snapshot, so it must stay cheap.
func (e *Engine) ObserveSnapshot(s *collector.Snapshot) {
	if len(s.DiskStats) == 0 {
		return // collection failed; don't treat that as "all disks fine"
	}
	cfg := e.cfg.Get().Alerts.Disk
	e.mu.Lock()
	defer e.mu.Unlock()

	seen := map[string]bool{}
	for _, d := range s.DiskStats {
		key := KindDisk + ":" + d.MountPoint
		seen[key] = true
		level := diskLevel(d.UsedPercent, e.alerts[key].Level, cfg)
		title := "Disk space running low"
		if level == LevelCritical {
			title = "Disk almost full"
		}
		body := fmt.Sprintf("%s is %.0f%% full (%s free)", d.MountPoint, d.UsedPercent, fmtSize(d.Free))
		e.set(key, KindDisk, d.MountPoint, level, title, body)
	}
	// Unmounted volumes can't be full.
	for key, a := range e.alerts {
		if a.Kind == KindDisk && a.Level > LevelResolved && !seen[key] {
			e.set(key, a.Kind, a.Subject, LevelResolved, a.Title, a.Body)
		}
	}
}

func diskLevel(used float64, prev int, cfg config.DiskAlert) int {
	reached := func(threshold float64, wasActive bool) bool {
		if threshold <= 0 {
			return false
		}
		if wasActive {
			return used >= threshold-diskHysteresis
		}
		return used >= threshold
	}
	switch {
	case reached(cfg.CriticalPercent, prev >= LevelCritical):
		return LevelCritical
	case reached(cfg.WarnPercent, prev >= LevelWarning):
		return LevelWarning
	}
	return LevelResolved
}

func fmtSize(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	}
	return fmt.Sprintf("%d KB", b>>10)
}
