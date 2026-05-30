import {renderer} from "@b9g/crank/dom";
import {MetricGauge} from "./components/MetricGauge.jsx";
import {LoadAvg} from "./components/LoadAvg.jsx";
import {LineChart} from "./components/LineChart.jsx";
import {CpuChart} from "./components/CpuChart.jsx";
import {GpuCard} from "./components/GpuCard.jsx";
import {DiskCard} from "./components/DiskCard.jsx";
import {ProcessTable} from "./components/ProcessTable.jsx";
import {fmtTime, fmtBytes, netRates, diskIORates, primaryIface, primaryDisk} from "./utils.js";

const MAX_HISTORY = 720; // 1 hour at 5s intervals
const PROCESS_POLL_MS = 5000;

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
  let tab = "overview"; // "overview" | "processes"
  let processes = null;
  let cpuReady = false;
  let procTimer = null;

  const wsProto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(`${wsProto}//${window.location.host}/api/live`);

  const fetchProcesses = () => {
    fetch("/api/processes")
      .then(r => r.json())
      .then(data => { processes = data?.processes ?? []; cpuReady = data?.cpu_ready ?? false; this.refresh(); })
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
          <header style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 24px; border-bottom: 1px solid #21262d; padding-bottom: 16px;">
            <h1 style="font-size: 18px; font-weight: bold; color: #e6edf3; letter-spacing: 0.02em;">
              Mac Monitor
            </h1>
            <span style={`font-size: 11px; ${connected ? "color: #3fb950;" : "color: #f85149;"}`}>
              {connected ? "● live" : error ? `● ${error}` : "● connecting…"}
            </span>
          </header>

          {/* ── tabs ── */}
          <nav style="display: flex; gap: 4px; margin-bottom: 28px;">
            {["overview", "processes"].map(t => (
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
    stopProcPolling();
    ws.close();
  }
}

export default App;
