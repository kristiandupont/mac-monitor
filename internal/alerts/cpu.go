package alerts

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// cpuSampleInterval is how often per-process CPU time is read. Each sample
// costs about a millisecond of CPU; alerts need minutes of sustained load,
// so there is nothing to gain from sampling faster.
const cpuSampleInterval = 30 * time.Second

type cpuState struct {
	prev   map[int32]time.Duration
	prevAt time.Time
	// Only processes currently above the threshold (or firing) are tracked.
	procs map[int32]*procTrack
}

type procTrack struct {
	name       string
	aboveSince time.Time
	firing     bool
	pct        float64
}

func (e *Engine) runCPUSampler(ctx context.Context) {
	ticker := time.NewTicker(cpuSampleInterval)
	defer ticker.Stop()
	for {
		e.sampleCPUOnce()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (e *Engine) sampleCPUOnce() {
	cfg := e.cfg.Get().Alerts.CPU
	st := &e.cpu
	if cfg.Percent <= 0 {
		st.prev, st.procs = nil, nil
		e.resolveCPU(nil)
		return
	}

	times, err := e.sampleCPU()
	if err != nil {
		log.Printf("alerts: sample processes: %v", err)
		return
	}
	now := e.now()
	prev, prevAt := st.prev, st.prevAt
	st.prev, st.prevAt = times, now
	elapsed := now.Sub(prevAt)
	if prev == nil || elapsed <= 0 {
		return
	}
	if elapsed > 3*cpuSampleInterval {
		// We missed samples (e.g. the Mac slept): start over rather than
		// count the gap as sustained load.
		st.procs = nil
		return
	}
	if st.procs == nil {
		st.procs = map[int32]*procTrack{}
	}

	activeNames := e.activeCPUSubjects()
	duration := time.Duration(cfg.Duration)
	firing := map[string]*procTrack{}

	for pid, t := range times {
		last, ok := prev[pid]
		if !ok {
			continue
		}
		pct := float64(t-last) / float64(elapsed) * 100
		tr := st.procs[pid]
		if tr == nil {
			if pct < cfg.Percent/2 {
				continue
			}
			tr = &procTrack{aboveSince: prevAt}
			if len(activeNames) > 0 {
				// After a restart (or a sleep), pick up alerts that are still
				// ongoing instead of resolving them and alerting again later.
				tr.name = e.nameOf(pid)
				tr.firing = activeNames[strings.ToLower(tr.name)]
			}
			if !tr.firing && pct < cfg.Percent {
				continue
			}
			st.procs[pid] = tr
		}
		tr.pct = pct

		if tr.firing {
			if pct < cfg.Percent/2 {
				delete(st.procs, pid)
				continue
			}
		} else {
			if pct < cfg.Percent {
				delete(st.procs, pid)
				continue
			}
			tr.firing = now.Sub(tr.aboveSince) >= duration
		}
		if !tr.firing {
			continue
		}
		if tr.name == "" {
			tr.name = e.nameOf(pid)
		}
		if isIgnored(cfg.Ignore, tr.name) {
			continue
		}
		if worst := firing[tr.name]; worst == nil || pct > worst.pct {
			firing[tr.name] = tr
		}
	}
	for pid := range st.procs {
		if _, ok := times[pid]; !ok {
			delete(st.procs, pid)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for name, tr := range firing {
		title := "High CPU: " + name
		body := fmt.Sprintf("%s has used over %.0f%% CPU for %s (now %.0f%%)",
			name, cfg.Percent, fmtDuration(now.Sub(tr.aboveSince)), tr.pct)
		e.set(KindCPU+":"+name, KindCPU, name, LevelWarning, title, body)
	}
	e.resolveCPULocked(firing)
}

func (e *Engine) nameOf(pid int32) string {
	if name := e.procName(pid); name != "" {
		return name
	}
	return fmt.Sprintf("pid %d", pid)
}

func (e *Engine) activeCPUSubjects() map[string]bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	names := map[string]bool{}
	for _, a := range e.alerts {
		if a.Kind == KindCPU && a.Level > LevelResolved {
			names[strings.ToLower(a.Subject)] = true
		}
	}
	return names
}

func (e *Engine) resolveCPU(keep map[string]*procTrack) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.resolveCPULocked(keep)
}

func (e *Engine) resolveCPULocked(keep map[string]*procTrack) {
	for key, a := range e.alerts {
		if a.Kind == KindCPU && a.Level > LevelResolved && keep[a.Subject] == nil {
			e.set(key, a.Kind, a.Subject, LevelResolved, a.Title, a.Body)
		}
	}
}

func fmtDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%d min", int(d.Round(time.Minute)/time.Minute))
	}
	return fmt.Sprintf("%.1f h", d.Hours())
}
