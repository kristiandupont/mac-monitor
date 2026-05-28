import {renderer} from "@b9g/crank/dom";
import {MetricGauge} from "./components/MetricGauge.jsx";
import {LoadAvg} from "./components/LoadAvg.jsx";
import {CpuChart} from "./components/CpuChart.jsx";

const MAX_HISTORY = 720; // 1 hour at 5s intervals

function* App() {
  let snap = null;
  let history = [];
  let connected = false;
  let error = null;

  const wsProto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const wsUrl = `${wsProto}//${window.location.host}/api/live`;
  const ws = new WebSocket(wsUrl);

  ws.onopen = () => {
    connected = true;
    this.refresh();

    // seed chart with recent history
    fetch("/api/history")
      .then(r => r.json())
      .then(data => {
        history = data || [];
        this.refresh();
      })
      .catch(() => {});
  };

  ws.onmessage = e => {
    snap = JSON.parse(e.data);
    history = [...history, snap].slice(-MAX_HISTORY);
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
      yield (
        <div style="max-width: 960px; margin: 0 auto; padding: 32px 24px;">
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
              <section style="display: grid; grid-template-columns: 1fr 1fr; gap: 24px; margin-bottom: 32px;">
                <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
                  <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px;">CPU</h2>
                  <MetricGauge label="Total" value={snap.cpu_percent} color="#58a6ff" />
                  <LoadAvg load1={snap.load_1} load5={snap.load_5} load15={snap.load_15} />
                  <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(32px, 1fr)); gap: 4px; margin-top: 8px;">
                    {snap.cpu_per_core.map((pct, i) => (
                      <div key={i} title={`Core ${i}: ${pct.toFixed(1)}%`}
                        style={`height: 32px; background: rgba(88,166,255,${(pct / 100).toFixed(2)}); border: 1px solid #30363d; border-radius: 3px;`}
                      />
                    ))}
                  </div>
                </div>

                <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
                  <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px;">Memory</h2>
                  <MetricGauge label="RAM" value={snap.mem_used} max={snap.mem_total} color="#3fb950" />
                  <MetricGauge label="Swap" value={snap.swap_used} max={snap.swap_total} color="#d29922" />
                </div>
              </section>

              <section style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px; margin-bottom: 24px;">
                <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px;">
                  CPU History (last hour)
                </h2>
                <CpuChart history={history} />
              </section>
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
