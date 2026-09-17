import { renderer } from "@b9g/crank/dom";
import { MetricGauge } from "./components/MetricGauge.jsx";
import { LoadAvg } from "./components/LoadAvg.jsx";
import { LineChart } from "./components/LineChart.jsx";
import { CpuChart } from "./components/CpuChart.jsx";
import { GpuCard } from "./components/GpuCard.jsx";
import { DiskCard } from "./components/DiskCard.jsx";
import { ProcessTable } from "./components/ProcessTable.jsx";
import {
  fmtBytes,
  fmtDateTime,
  netRates,
  diskIORates,
  primaryIface,
  primaryDisk,
  toPoints,
  gapThreshold,
} from "./utils.js";

const PROCESS_POLL_MS = 5000;
const COLLECT_INTERVAL = 5; // seconds, matches the Go collector
const TARGET_POINTS = 800; // per chart; wider spans are downsampled server-side
const MIN_SPAN = 5 * 60;
const FETCH_DEBOUNCE_MS = 200;
const PRESETS = [
  ["15m", 15 * 60],
  ["1h", 3600],
  ["6h", 6 * 3600],
  ["24h", 86400],
  ["7d", 7 * 86400],
  ["30d", 30 * 86400],
];

// Bucket size in seconds for a visible span; 0 means raw samples.
function stepFor(span) {
  const step = Math.ceil(span / TARGET_POINTS);
  return step <= COLLECT_INTERVAL ? 0 : step;
}

function pillStyle(active) {
  return `padding: 4px 12px; border-radius: 20px; border: 1px solid ${active ? "#58a6ff" : "#30363d"}; background: ${active ? "#1c2d3f" : "transparent"}; color: ${active ? "#58a6ff" : "#8b949e"}; font-size: 12px; cursor: pointer;`;
}

// ── sections ─────────────────────────────────────────────────────────────────

function ChartSection({ title, children }) {
  return (
    <section style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px; margin-bottom: 24px;">
      <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px;">
        {title}
      </h2>
      {children}
    </section>
  );
}

// ── root app ─────────────────────────────────────────────────────────────────

function* App() {
  let snap = null;
  let history = []; // snapshots covering `loaded`
  // Visible window: `span` seconds ending at `end`, or at now when end is null (live).
  let span = 3600;
  let end = null;
  // What `history` currently holds.
  let loaded = { from: 0, to: 0, step: 0, fetchedAt: 0 };
  let fetchSeq = 0;
  let fetchTimer = null;
  let maxSpan = 86400; // replaced by the server's retention from /api/config
  let connected = false;
  let error = null;
  let tab = "overview"; // "overview" | "processes"
  let processes = null;
  let cpuReady = false;
  let procTimer = null;

  const wsProto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(`${wsProto}//${window.location.host}/api/live`);

  const fetchProcesses = () => {
    fetch("/api/processes")
      .then((r) => r.json())
      .then((data) => {
        processes = data?.processes ?? [];
        cpuReady = data?.cpu_ready ?? false;
        this.refresh();
      })
      .catch(() => {});
  };

  const startProcPolling = () => {
    if (procTimer !== null) return;
    fetchProcesses();
    procTimer = setInterval(fetchProcesses, PROCESS_POLL_MS);
  };

  const stopProcPolling = () => {
    if (procTimer === null) return;
    clearInterval(procTimer);
    procTimer = null;
  };

  const switchTab = (t) => {
    tab = t;
    if (t === "processes") {
      startProcPolling();
    } else {
      stopProcPolling();
    }
    this.refresh();
  };

  const nowSec = () => Date.now() / 1000;
  const viewRange = () => {
    const to = end ?? nowSec();
    return [to - span, to];
  };

  // Fetches the visible range plus half a span on each side, so small pans
  // don't show empty space while the next request is in flight.
  const fetchHistory = () => {
    clearTimeout(fetchTimer);
    const [from, to] = viewRange();
    const step = stepFor(span);
    const pad = span / 2;
    const req = {
      from: Math.floor(from - pad),
      to: Math.ceil(Math.min(to + pad, nowSec())),
      step,
    };
    const seq = ++fetchSeq;
    fetch(`/api/history?from=${req.from}&to=${req.to}&step=${step}`)
      .then((r) => r.json())
      .then((data) => {
        if (seq !== fetchSeq) return;
        history = data ?? [];
        loaded = { ...req, fetchedAt: nowSec() };
        this.refresh();
      })
      .catch(() => {});
  };

  const scheduleFetch = () => {
    clearTimeout(fetchTimer);
    fetchTimer = setTimeout(fetchHistory, FETCH_DEBOUNCE_MS);
  };

  const setView = (newSpan, newEnd) => {
    span = Math.min(maxSpan, Math.max(MIN_SPAN, newSpan));
    const now = nowSec();
    end = newEnd == null || newEnd >= now ? null : Math.max(newEnd, now - maxSpan + span);
    const [from, to] = viewRange();
    const covered = from >= loaded.from && (end === null || to <= loaded.to);
    if (stepFor(span) !== loaded.step || !covered) scheduleFetch();
    this.refresh();
  };

  const view = () => {
    const [from, to] = viewRange();
    return {
      xMin: from * 1000,
      xMax: to * 1000,
      onView: (min, max) => setView((max - min) / 1000, max / 1000),
      onReset: () => setView(span, null),
    };
  };

  ws.onopen = () => {
    connected = true;
    this.refresh();
    fetch("/api/config")
      .then((r) => r.json())
      .then((cfg) => {
        maxSpan = Math.max(MIN_SPAN, cfg.retention_seconds);
        setView(span, end);
      })
      .catch(() => {});
    fetchHistory();
  };
  ws.onmessage = (e) => {
    snap = JSON.parse(e.data);
    if (end === null) {
      if (loaded.step === 0) {
        // Raw resolution: append live samples and drop ones that scrolled out.
        const keepFrom = snap.ts - span * 1.5;
        if (!history.length || snap.ts > history[history.length - 1].ts) {
          history = [...history, snap].filter((s) => s.ts >= keepFrom);
        }
        loaded = { ...loaded, from: Math.max(loaded.from, keepFrom), to: snap.ts };
      } else if (snap.ts - loaded.fetchedAt >= loaded.step) {
        // Downsampled: refetch once a new bucket's worth of data exists.
        fetchHistory();
      }
    }
    this.refresh();
  };
  ws.onerror = () => {
    error = "WebSocket error — is the server running?";
    this.refresh();
  };
  ws.onclose = () => {
    connected = false;
    this.refresh();
  };

  try {
    while (true) {
      const iface = primaryIface(snap);
      const disk = primaryDisk(snap);
      const maxGap = gapThreshold(loaded.step);
      const ts = history.map((s) => s.ts);
      const rateTs = ts.slice(1);
      const chartView = view();
      const [viewFrom, viewTo] = viewRange();

      let netDatasets = null;
      if (iface && history.length > 1) {
        const rates = netRates(history, iface, maxGap);
        netDatasets = [
          {
            data: toPoints(rateTs, rates.map((r) => r.in), maxGap),
            color: "#58a6ff",
            label: "IN",
            fill: false,
          },
          {
            data: toPoints(rateTs, rates.map((r) => r.out), maxGap),
            color: "#3fb950",
            label: "OUT",
            fill: false,
          },
        ];
      }

      let diskIODatasets = null;
      if (disk && history.length > 1) {
        const rates = diskIORates(history, disk, maxGap);
        diskIODatasets = [
          {
            data: toPoints(rateTs, rates.map((r) => r.read), maxGap),
            color: "#d29922",
            label: "Read",
            fill: false,
          },
          {
            data: toPoints(rateTs, rates.map((r) => r.write), maxGap),
            color: "#f85149",
            label: "Write",
            fill: false,
          },
        ];
      }

      let gpuDatasets = null;
      if (snap?.gpu_stats?.length) {
        const gpu = (key) =>
          toPoints(ts, history.map((s) => s.gpu_stats?.[0]?.[key] ?? 0), maxGap);
        gpuDatasets = [
          {
            data: gpu("device_utilization"),
            color: "#bc8cff",
            label: "Device",
            fill: true,
          },
          {
            data: gpu("renderer_utilization"),
            color: "#d2a8ff",
            label: "Renderer",
            fill: false,
          },
          {
            data: gpu("tiler_utilization"),
            color: "#a5d6ff",
            label: "Tiler",
            fill: false,
          },
        ];
      }

      yield (
        <div style="max-width: 1100px; margin: 0 auto; padding: 32px 24px;">
          <header style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 24px; border-bottom: 1px solid #21262d; padding-bottom: 16px;">
            <h1 style="font-size: 28px; font-weight: 100; color: #e6edf3; letter-spacing: 0.02em; text-shadow: 0 0 2px #088, 0 0 6px #088, 0 0 10px #088;">
              Mac Monitor
            </h1>
            <span
              style={`font-size: 11px; ${connected ? "color: #3fb950;" : "color: #f85149;"}`}
            >
              {connected ? "● live" : error ? `● ${error}` : "● connecting…"}
            </span>
          </header>

          {/* ── tabs ── */}
          <nav style="display: flex; gap: 4px; margin-bottom: 28px;">
            {["overview", "processes"].map((t) => (
              <button
                key={t}
                onclick={() => switchTab(t)}
                style={`padding: 6px 16px; border-radius: 20px; border: 1px solid ${tab === t ? "#58a6ff" : "#30363d"}; background: ${tab === t ? "#1c2d3f" : "transparent"}; color: ${tab === t ? "#58a6ff" : "#8b949e"}; font-size: 13px; cursor: pointer; text-transform: capitalize;`}
              >
                {t}
              </button>
            ))}
          </nav>

          {snap ? (
            tab === "overview" ? (
              <div>
                {/* ── metric cards ── */}
                <section style="display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 20px; margin-bottom: 28px;">
                  <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
                    <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px;">
                      CPU
                    </h2>
                    <MetricGauge
                      label="Total"
                      value={snap.cpu_percent}
                      color="#58a6ff"
                    />
                    <LoadAvg
                      load1={snap.load_1}
                      load5={snap.load_5}
                      load15={snap.load_15}
                    />
                    <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(28px, 1fr)); gap: 3px; margin-top: 10px;">
                      {snap.cpu_per_core.map((pct, i) => (
                        <div
                          key={i}
                          title={`Core ${i}: ${pct.toFixed(1)}%`}
                          style={`height: 28px; background: rgba(88,166,255,${(pct / 100).toFixed(2)}); border: 1px solid #30363d; border-radius: 3px;`}
                        />
                      ))}
                    </div>
                  </div>

                  <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
                    <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px;">
                      Memory
                    </h2>
                    <MetricGauge
                      label="RAM"
                      value={snap.mem_used}
                      max={snap.mem_total}
                      color="#3fb950"
                    />
                    <MetricGauge
                      label="Swap"
                      value={snap.swap_used}
                      max={snap.swap_total}
                      color="#d29922"
                    />
                  </div>

                  <GpuCard gpuStats={snap.gpu_stats} />
                  <DiskCard diskStats={snap.disk_stats} />
                </section>

                {/* ── time range ── */}
                <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin-bottom: 16px;">
                  {PRESETS.filter(([, secs]) => secs <= maxSpan).map(([label, secs]) => (
                    <button
                      key={label}
                      onclick={() => setView(secs, end)}
                      style={pillStyle(span === secs)}
                    >
                      {label}
                    </button>
                  ))}
                  <button
                    onclick={() => setView(span, null)}
                    style={pillStyle(end === null)}
                  >
                    ● Live
                  </button>
                  <span style="font-size: 11px; color: #8b949e; margin-left: 8px;">
                    {fmtDateTime(viewFrom * 1000)} – {fmtDateTime(viewTo * 1000)}
                  </span>
                  <span style="font-size: 11px; color: #6e7681; margin-left: auto;">
                    pinch or ⌘-scroll to zoom · drag to pan · double-click for live
                  </span>
                </div>

                {/* ── charts ── */}
                <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-bottom: 28px;">
                  <ChartSection title="CPU History">
                    <CpuChart history={history} maxGap={maxGap} view={chartView} />
                  </ChartSection>

                  {gpuDatasets && (
                    <ChartSection title="GPU History">
                      <LineChart
                        {...chartView}
                        datasets={gpuDatasets}
                        yMax={100}
                        formatY={(v) => `${v.toFixed(0)}%`}
                      />
                    </ChartSection>
                  )}
                </div>

                {netDatasets && (
                  <ChartSection title={`Network — ${iface}`}>
                    <LineChart {...chartView} datasets={netDatasets} formatY={fmtBytes} />
                  </ChartSection>
                )}

                {diskIODatasets && (
                  <ChartSection title={`Disk I/O — ${disk}`}>
                    <LineChart {...chartView} datasets={diskIODatasets} formatY={fmtBytes} />
                  </ChartSection>
                )}
              </div>
            ) : (
              <section style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
                <ProcessTable processes={processes} cpuReady={cpuReady} />
              </section>
            )
          ) : (
            <p style="color: #8b949e; text-align: center; padding: 60px 0;">
              {error || "Waiting for first data point…"}
            </p>
          )}
        </div>
      );
    }
  } finally {
    clearTimeout(fetchTimer);
    stopProcPolling();
    ws.close();
  }
}

export default App;
