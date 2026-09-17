package alerts

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mac-monitor/internal/collector"
	"mac-monitor/internal/config"
	"mac-monitor/internal/storage"
)

type harness struct {
	t      *testing.T
	db     *storage.DB
	cfg    *config.Store
	clock  time.Time
	native []storage.Notification
	// nativeErr, when set, makes native delivery fail.
	nativeErr error
	cpu       map[int32]time.Duration
	names     map[int32]string
}

func newHarness(t *testing.T, mutate func(*config.Config)) *harness {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Default()
	if mutate != nil {
		mutate(&cfg)
	}
	return &harness{
		t:     t,
		db:    db,
		cfg:   config.NewStore(path, cfg),
		clock: time.Unix(1_000_000, 0),
		cpu:   map[int32]time.Duration{},
		names: map[int32]string{},
	}
}

// engine creates an engine over the harness's database; calling it again
// simulates an app restart.
func (h *harness) engine() *Engine {
	h.t.Helper()
	e, err := New(h.db, h.cfg, func(ctx context.Context, n storage.Notification) error {
		if h.nativeErr != nil {
			return h.nativeErr
		}
		h.native = append(h.native, n)
		return nil
	})
	if err != nil {
		h.t.Fatal(err)
	}
	e.now = func() time.Time { return h.clock }
	e.sampleCPU = func() (map[int32]time.Duration, error) {
		cp := map[int32]time.Duration{}
		for k, v := range h.cpu {
			cp[k] = v
		}
		return cp, nil
	}
	e.procName = func(pid int32) string { return h.names[pid] }
	return e
}

func (h *harness) advance(d time.Duration) { h.clock = h.clock.Add(d) }

// deliver runs one delivery pass and returns the titles delivered natively.
func (h *harness) deliver(e *Engine) []string {
	h.native = nil
	e.deliverDue(context.Background())
	var titles []string
	for _, n := range h.native {
		titles = append(titles, n.Title)
	}
	return titles
}

func disk(used float64) *collector.Snapshot {
	return &collector.Snapshot{DiskStats: []collector.DiskStat{{MountPoint: "/", UsedPercent: used, Free: 5 << 30}}}
}

func expectTitles(t *testing.T, step string, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("%s: delivered %q, want %q", step, got, want)
	}
}

func TestDiskAlertLifecycle(t *testing.T) {
	h := newHarness(t, nil)
	e := h.engine()

	steps := []struct {
		used float64
		want []string
	}{
		{80, nil},
		{91, []string{"Disk space running low"}},
		{91.5, nil}, // already told
		{96, []string{"Disk almost full"}},
		{94, nil},                                // hysteresis keeps it critical; no new notification
		{92, nil},                                // back to warning; not worth a notification
		{96, nil},                                // critical again within the same episode: already told
		{87, nil},                                // resolved
		{91, []string{"Disk space running low"}}, // new episode
	}
	for _, s := range steps {
		h.advance(5 * time.Second)
		e.ObserveSnapshot(disk(s.used))
		expectTitles(t, fmt.Sprintf("used %v%%", s.used), h.deliver(e), s.want...)
	}
}

func fmtFloat(f float64) string {
	return strings.TrimSpace(strings.Replace(time.Duration(f*1e9).String(), "s", "", 1))
}

func TestDiskAlertRepeatsAndSurvivesRestart(t *testing.T) {
	h := newHarness(t, nil)
	e := h.engine()
	e.ObserveSnapshot(disk(91))
	expectTitles(t, "first", h.deliver(e), "Disk space running low")

	// Restart: same state, must not notify again.
	e = h.engine()
	h.advance(time.Hour)
	e.ObserveSnapshot(disk(91))
	expectTitles(t, "after restart", h.deliver(e))

	h.advance(24 * time.Hour)
	e.ObserveSnapshot(disk(91))
	expectTitles(t, "after repeat interval", h.deliver(e), "Disk space running low")
}

func TestDiskUnmountResolves(t *testing.T) {
	h := newHarness(t, nil)
	e := h.engine()
	e.ObserveSnapshot(&collector.Snapshot{DiskStats: []collector.DiskStat{
		{MountPoint: "/", UsedPercent: 50},
		{MountPoint: "/Volumes/USB", UsedPercent: 99},
	}})
	if n := activeCount(e); n != 1 {
		t.Fatalf("active alerts: %d, want 1", n)
	}
	e.ObserveSnapshot(disk(50))
	if n := activeCount(e); n != 0 {
		t.Errorf("active alerts after unmount: %d, want 0", n)
	}
	// A failed collection (no disks) must not resolve anything.
	e.ObserveSnapshot(&collector.Snapshot{DiskStats: []collector.DiskStat{{MountPoint: "/", UsedPercent: 99}}})
	e.ObserveSnapshot(&collector.Snapshot{})
	if n := activeCount(e); n != 1 {
		t.Errorf("active alerts after empty snapshot: %d, want 1", n)
	}
}

func activeCount(e *Engine) int {
	st, _ := e.Status()
	n := 0
	for _, a := range st.Alerts {
		if a.Level > 0 {
			n++
		}
	}
	return n
}

// cpuStep advances the clock by one sample interval, charging each pid the
// given percentage of a core, and samples.
func (h *harness) cpuStep(e *Engine, pcts map[int32]float64) {
	h.advance(cpuSampleInterval)
	for pid, pct := range pcts {
		h.cpu[pid] += time.Duration(float64(cpuSampleInterval) * pct / 100)
	}
	e.sampleCPUOnce()
}

func TestCPUAlertNeedsSustainedLoad(t *testing.T) {
	h := newHarness(t, nil) // 80% for 5 minutes
	h.names = map[int32]string{1: "render", 2: "spiky"}
	h.cpu = map[int32]time.Duration{1: 0, 2: 0}
	e := h.engine()
	e.sampleCPUOnce() // baseline

	for i := 0; i < 9; i++ { // 4.5 minutes
		h.cpuStep(e, map[int32]float64{1: 150, 2: 100})
	}
	h.cpuStep(e, map[int32]float64{1: 150, 2: 10}) // spiky dips
	expectTitles(t, "at 5 min", h.deliver(e), "High CPU: render")

	for i := 0; i < 20; i++ {
		h.cpuStep(e, map[int32]float64{1: 60, 2: 100}) // 60% is above half: stays firing
	}
	// render is not repeated; spiky has now been busy for 10 minutes.
	expectTitles(t, "still busy", h.deliver(e), "High CPU: spiky")

	h.cpuStep(e, map[int32]float64{1: 10, 2: 100})
	st, _ := e.Status()
	if len(st.Alerts) != 2 || st.Alerts[0].Subject != "spiky" || st.Alerts[1].Level != LevelResolved {
		t.Errorf("after render calmed down: %+v", st.Alerts)
	}
}

func TestCPUIgnore(t *testing.T) {
	h := newHarness(t, func(c *config.Config) { c.Alerts.CPU.Ignore = []string{"FFmpeg"} })
	h.names = map[int32]string{1: "ffmpeg", 2: "Xcode"}
	h.cpu = map[int32]time.Duration{1: 0, 2: 0}
	e := h.engine()
	e.sampleCPUOnce()
	for i := 0; i < 11; i++ {
		h.cpuStep(e, map[int32]float64{1: 400, 2: 100})
	}
	expectTitles(t, "before ignore", h.deliver(e), "High CPU: Xcode")

	if err := e.Ignore("xcode"); err != nil {
		t.Fatal(err)
	}
	if n := activeCount(e); n != 0 {
		t.Errorf("active after ignore = %d, want 0", n)
	}
	for i := 0; i < 20; i++ {
		h.cpuStep(e, map[int32]float64{1: 400, 2: 100})
	}
	expectTitles(t, "after ignore", h.deliver(e))

	if err := e.Unignore("XCODE"); err != nil {
		t.Fatal(err)
	}
	h.cpuStep(e, map[int32]float64{2: 100})
	expectTitles(t, "after unignore", h.deliver(e), "High CPU: Xcode")
}

func TestCPUAlertSurvivesRestart(t *testing.T) {
	h := newHarness(t, nil)
	h.names = map[int32]string{1: "render"}
	h.cpu = map[int32]time.Duration{1: 0}
	e := h.engine()
	e.sampleCPUOnce()
	for i := 0; i < 11; i++ {
		h.cpuStep(e, map[int32]float64{1: 100})
	}
	expectTitles(t, "first", h.deliver(e), "High CPU: render")

	e = h.engine()
	e.sampleCPUOnce()
	for i := 0; i < 20; i++ {
		h.cpuStep(e, map[int32]float64{1: 100})
	}
	expectTitles(t, "after restart", h.deliver(e))
	if n := activeCount(e); n != 1 {
		t.Errorf("active after restart = %d, want 1", n)
	}
}

func TestCPUSleepGapResets(t *testing.T) {
	h := newHarness(t, nil)
	h.names = map[int32]string{1: "render"}
	h.cpu = map[int32]time.Duration{1: 0}
	e := h.engine()
	e.sampleCPUOnce()
	for i := 0; i < 8; i++ {
		h.cpuStep(e, map[int32]float64{1: 100})
	}
	h.advance(time.Hour) // slept
	h.cpu[1] += time.Hour
	e.sampleCPUOnce()
	for i := 0; i < 3; i++ {
		h.cpuStep(e, map[int32]float64{1: 100})
	}
	expectTitles(t, "after sleep", h.deliver(e))
	for i := 0; i < 7; i++ {
		h.cpuStep(e, map[int32]float64{1: 100})
	}
	expectTitles(t, "5 min after sleep", h.deliver(e), "High CPU: render")

	// Already firing: a sleep neither resolves nor re-notifies it.
	h.advance(time.Hour)
	h.cpu[1] += time.Hour
	e.sampleCPUOnce()
	for i := 0; i < 3; i++ {
		h.cpuStep(e, map[int32]float64{1: 100})
	}
	expectTitles(t, "second sleep", h.deliver(e))
	if n := activeCount(e); n != 1 {
		t.Errorf("active after second sleep = %d, want 1", n)
	}
}

func TestDeliveryRetriesThenFails(t *testing.T) {
	h := newHarness(t, nil)
	h.nativeErr = errors.New("not authorized")
	e := h.engine()
	e.ObserveSnapshot(disk(91))

	for i, wait := range append([]time.Duration{0}, retryBackoff...) {
		h.advance(wait - time.Second)
		h.deliver(e)
		if n, _, _ := e.store.NextAttemptAt(); i > 0 && n != h.clock.Unix()+1 {
			t.Fatalf("attempt %d ran early", i+1)
		}
		h.advance(time.Second)
		h.deliver(e)
	}
	ns, _ := h.db.RecentNotifications(10)
	if len(ns) != 1 || ns[0].Status != storage.StatusFailed || ns[0].Attempts != len(retryBackoff)+1 || ns[0].LastError != "not authorized" {
		t.Errorf("got %+v", ns)
	}
	if _, ok, _ := h.db.NextAttemptAt(); ok {
		t.Error("failed notification still pending")
	}
}

func TestCommandChannel(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	h := newHarness(t, func(c *config.Config) {
		c.Alerts.Native = false
		// Succeeds only on the second attempt, and checks env + stdin.
		c.Alerts.Command = `test "$MM_ALERT_ATTEMPT" = 2 || { echo "boom $MM_ALERT_SUBJECT"; exit 3; }
			printf '%s|%s|' "$MM_ALERT_LEVEL" "$MM_ALERT_BODY" > ` + out + ` && cat >> ` + out
	})
	e := h.engine()
	e.ObserveSnapshot(disk(96))

	h.deliver(e)
	ns, _ := h.db.RecentNotifications(10)
	if len(ns) != 1 || ns[0].Channel != ChannelCommand || ns[0].Status != storage.StatusPending ||
		ns[0].LastError != "exit status 3: boom /" {
		t.Fatalf("after first attempt: %+v", ns)
	}

	h.advance(retryBackoff[0])
	h.deliver(e)
	ns, _ = h.db.RecentNotifications(10)
	if ns[0].Status != storage.StatusSent {
		t.Fatalf("after retry: %+v", ns[0])
	}
	got := readFile(t, out)
	if !strings.HasPrefix(got, "critical|/ is 96% full (5.0 GB free)|{") || !strings.Contains(got, `"subject":"/"`) {
		t.Errorf("command saw %q", got)
	}
}
