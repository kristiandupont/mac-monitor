import {renderer} from "@b9g/crank/dom";
import {MetricGauge} from "./components/MetricGauge.jsx";
import {LoadAvg} from "./components/LoadAvg.jsx";
import {LineChart} from "./components/LineChart.jsx";
import {CpuChart} from "./components/CpuChart.jsx";
import {GpuCard} from "./components/GpuCard.jsx";
import {DiskCard} from "./components/DiskCard.jsx";

const MAX_HISTORY = 720; // 1 hour at 5s intervals

// ── helpers ──────────────────────────────────────────────────────────────────

function fmtTime(ts) {
  const d = new Date(ts * 1000);
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
function pad(n) { return String(n).padStart(2, "0"); }

function fmtBytes(b) {
  if (b >= 1e9) return (b / 1e9).toFixed(1) + " GB/s";
  if (b >= 1e6) return (b / 1e6).toFixed(1) + " MB/s";
  if (b >= 1e3) return (b / 1e3).toFixed(1) + " KB/s";
  return b.toFixed(0) + " B/s";
}

// Returns [{in, out}] rates aligned to history[1..] (one fewer entry than history)
function netRates(history, ifaceName) {
  const rates = [];
  for (let i = 1; i < history.length; i++) {
    const prev = history[i - 1];
    const curr = history[i];
    const dt = curr.ts - prev.ts;
    if (dt <= 0) { rates.push({in: 0, out: 0}); continue; }
    const pi = prev.net_stats?.find(n => n.name === ifaceName);
    const ci = curr.net_stats?.find(n => n.name === ifaceName);
    if (!pi || !ci) { rates.push({in: 0, out: 0}); continue; }
    rates.push({
      in:  Math.max(0, ci.bytes_recv - pi.bytes_recv) / dt,
      out: Math.max(0, ci.bytes_sent - pi.bytes_sent) / dt,
    });
  }
  return rates;
}

// Returns [{read, write}] rates aligned to history[1..]
function diskIORates(history, diskName) {
  const rates = [];
  for (let i = 1; i < history.length; i++) {
    const prev = history[i - 1];
    const curr = history[i];
    const dt = curr.ts - prev.ts;
    if (dt <= 0) { rates.push({read: 0, write: 0}); continue; }
    const pd = prev.disk_io_stats?.find(d => d.name === diskName);
    const cd = curr.disk_io_stats?.find(d => d.name === diskName);
    if (!pd || !cd) { rates.push({read: 0, write: 0}); continue; }
    rates.push({
      read:  Math.max(0, cd.read_bytes  - pd.read_bytes)  / dt,
      write: Math.max(0, cd.write_bytes - pd.write_bytes) / dt,
    });
  }
  return rates;
}

// Pick the primary network interface: prefer en0, otherwise busiest non-loopback
function primaryIface(snap) {
  if (!snap?.net_stats?.length) return null;
  const ifaces = snap.net_stats.filter(n => n.name !== "lo0" && !n.name.startsWith("utun"));
  if (!ifaces.length) return null;
  const en0 = ifaces.find(n => n.name === "en0");
  if (en0) return en0.name;
  return ifaces.reduce((a, b) => (a.bytes_recv + a.bytes_sent > b.bytes_recv + b.bytes_sent ? a : b)).name;
}

// Pick the primary physical disk: lowest-numbered disk device
function primaryDisk(snap) {
  if (!snap?.disk_io_stats?.length) return null;
  return snap.disk_io_stats.map(d => d.name).sort()[0];
}

// ── sections ─────────────────────────────────────────────────────────────────

function ChartSection({title, children}) {
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
  let history = [];
  let connected = false;
  let error = null;

  const wsProto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(`${wsProto}//${window.location.host}/api/live`);

  ws.onopen = () => {
    connected = true;
    this.refresh();
    fetch("/api/history")
      .then(r => r.json())
      .then(data => { history = data ?? []; this.refresh(); })
      .catch(() => {});
  };
  ws.onmessage = e => {
    snap = JSON.parse(e.data);
    history = [...history, snap].slice(-MAX_HISTORY);
    this.refresh();
  };
  ws.onerror = () => { error = "WebSocket error — is the server running?"; this.refresh(); };
  ws.onclose  = () => { connected = false; this.refresh(); };

  try {
    while (true) {
      const iface = primaryIface(snap);
      const disk  = primaryDisk(snap);
      const rateLabels = history.slice(1).map(s => fmtTime(s.ts));

      let netDatasets = null;
      if (iface && history.length > 1) {
        const rates = netRates(history, iface);
        netDatasets = [
          {labels: rateLabels, data: rates.map(r => r.in),  color: "#58a6ff", label: "IN",  fill: false},
          {labels: rateLabels, data: rates.map(r => r.out), color: "#3fb950", label: "OUT", fill: false},
        ];
      }

      let diskIODatasets = null;
      if (disk && history.length > 1) {
        const rates = diskIORates(history, disk);
        diskIODatasets = [
          {labels: rateLabels, data: rates.map(r => r.read),  color: "#d29922", label: "Read",  fill: false},
          {labels: rateLabels, data: rates.map(r => r.write), color: "#f85149", label: "Write", fill: false},
        ];
      }

      let gpuDatasets = null;
      if (snap?.gpu_stats?.length) {
        const labels = history.map(s => fmtTime(s.ts));
        gpuDatasets = [
          {labels, data: history.map(s => s.gpu_stats?.[0]?.device_utilization ?? 0),   color: "#bc8cff", label: "Device",   fill: true},
          {labels, data: history.map(s => s.gpu_stats?.[0]?.renderer_utilization ?? 0), color: "#d2a8ff", label: "Renderer", fill: false},
          {labels, data: history.map(s => s.gpu_stats?.[0]?.tiler_utilization ?? 0),    color: "#a5d6ff", label: "Tiler",    fill: false},
        ];
      }

      yield (
        <div style="max-width: 1100px; margin: 0 auto; padding: 32px 24px;">
          <header style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 32px; border-bottom: 1px solid #21262d; padding-bottom: 16px;">
            <h1 style="font-size: 18px; font-weight: bold; color: #e6edf3; letter-spacing: 0.02em;">
              Mac Monitor
            </h1>
            <span style={`font-size: 11px; ${connected ? "color: #3fb950;" : "color: #f85149;"}`}>
              {connected ? "● live" : error ? `● ${error}` : "● connecting…"}
            </span>
          </header>

          {snap ? (
            <div>
              {/* ── metric cards ── */}
              <section style="display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 20px; margin-bottom: 28px;">
                <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
                  <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px;">CPU</h2>
                  <MetricGauge label="Total" value={snap.cpu_percent} color="#58a6ff" />
                  <LoadAvg load1={snap.load_1} load5={snap.load_5} load15={snap.load_15} />
                  <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(28px, 1fr)); gap: 3px; margin-top: 10px;">
                    {snap.cpu_per_core.map((pct, i) => (
                      <div key={i} title={`Core ${i}: ${pct.toFixed(1)}%`}
                        style={`height: 28px; background: rgba(88,166,255,${(pct / 100).toFixed(2)}); border: 1px solid #30363d; border-radius: 3px;`}
                      />
                    ))}
                  </div>
                </div>

                <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
                  <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px;">Memory</h2>
                  <MetricGauge label="RAM"  value={snap.mem_used}  max={snap.mem_total}  color="#3fb950" />
                  <MetricGauge label="Swap" value={snap.swap_used} max={snap.swap_total} color="#d29922" />
                </div>

                <GpuCard gpuStats={snap.gpu_stats} />
                <DiskCard diskStats={snap.disk_stats} />
              </section>

              {/* ── charts ── */}
              <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 20px; margin-bottom: 28px;">
                <ChartSection title="CPU History (last hour)">
                  <CpuChart history={history} />
                </ChartSection>

                {gpuDatasets && (
                  <ChartSection title="GPU History (last hour)">
                    <LineChart datasets={gpuDatasets} yMax={100} formatY={v => `${v.toFixed(0)}%`} />
                  </ChartSection>
                )}
              </div>

              {netDatasets && (
                <ChartSection title={`Network — ${iface}`}>
                  <LineChart datasets={netDatasets} formatY={fmtBytes} />
                </ChartSection>
              )}

              {diskIODatasets && (
                <ChartSection title={`Disk I/O — ${disk}`}>
                  <LineChart datasets={diskIODatasets} formatY={fmtBytes} />
                </ChartSection>
              )}
            </div>
          ) : (
            <p style="color: #8b949e; text-align: center; padding: 60px 0;">
              {error || "Waiting for first data point…"}
            </p>
          )}
        </div>
      );
    }
  } finally {
    ws.close();
  }
}

export default App;
