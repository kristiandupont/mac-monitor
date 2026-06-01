// ── AnimatedIcon ──────────────────────────────────────────────────────────────
// Rotates the Mac Monitor icon, varying speed and color with simulated CPU load.
// phase=0 → slow+blue (idle), phase=1 → fast+red (busy).
function* AnimatedIcon({ size = 48, cold = [88, 166, 255], hot = [248, 81, 73] }) {
  let angle = 0;
  let phase = 0;
  let phaseDir = 1;
  let last = null;
  let raf;

  const tick = (now) => {
    const dt = last === null ? 0 : (now - last) / 1000;
    last = now;
    phase = Math.max(0, Math.min(1, phase + (phaseDir * dt) / 5));
    if (phase >= 1) phaseDir = -1;
    if (phase <= 0) phaseDir = 1;
    angle = (angle + (30 + phase * 330) * dt) % 360;
    this.refresh();
    raf = requestAnimationFrame(tick);
  };
  raf = requestAnimationFrame(tick);

  try {
    while (true) {
      const r = Math.round(cold[0] + (hot[0] - cold[0]) * phase);
      const g = Math.round(cold[1] + (hot[1] - cold[1]) * phase);
      const b = Math.round(cold[2] + (hot[2] - cold[2]) * phase);
      const color = `rgb(${r},${g},${b})`;
      const blur = Math.round(4 + 12 * phase);
      const ga = (0.35 + 0.5 * phase).toFixed(2);

      yield (
        <svg
          width={size}
          height={size}
          viewBox="0 0 24 24"
          fill="none"
          stroke={color}
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
          style={`display:block;transform:rotate(${(-angle).toFixed(1)}deg);filter:drop-shadow(0 0 ${blur}px rgba(${r},${g},${b},${ga}))`}
        >
          <path d="M10.827 16.379a6.082 6.082 0 0 1-8.618-7.002l5.412 1.45a6.082 6.082 0 0 1 7.002-8.618l-1.45 5.412a6.082 6.082 0 0 1 8.618 7.002l-5.412-1.45a6.082 6.082 0 0 1-7.002 8.618l1.45-5.412Z" />
          <path d="M12 12v.01" />
        </svg>
      );
    }
  } finally {
    cancelAnimationFrame(raf);
  }
}

// ── MenuBarDemo ───────────────────────────────────────────────────────────────
// A fragment of a macOS menu bar showing the icon in its natural habitat.
function MenuBarDemo() {
  return (
    <div style="display:flex;flex-direction:column;align-items:center;gap:20px;">
      <div
        style="
        display:inline-flex;align-items:center;gap:14px;
        background:rgba(36,36,38,0.95);
        border:1px solid rgba(255,255,255,0.09);
        border-radius:10px;
        padding:7px 16px;
        font-size:13px;
        color:rgba(230,237,243,0.8);
        box-shadow:0 4px 32px rgba(0,0,0,0.5),0 1px 0 rgba(255,255,255,0.04) inset;
        letter-spacing:0.01em;
      "
      >
        {/* Wi-Fi */}
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="rgba(230,237,243,0.55)"
          stroke-width="1.75"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M5 12.55a11 11 0 0 1 14.08 0" />
          <path d="M1.42 9a16 16 0 0 1 21.16 0" />
          <path d="M8.53 16.11a6 6 0 0 1 6.95 0" />
          <circle
            cx="12"
            cy="20"
            r="0.5"
            fill="rgba(230,237,243,0.55)"
            stroke="none"
          />
        </svg>
        {/* Battery */}
        <svg
          width="22"
          height="12"
          viewBox="0 0 26 14"
          fill="none"
          stroke="rgba(230,237,243,0.55)"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <rect x="1" y="1" width="20" height="12" rx="3" />
          <path
            d="M21 5v4"
            stroke-width="2.5"
            stroke="rgba(230,237,243,0.55)"
          />
          <rect
            x="3"
            y="3"
            width="12"
            height="8"
            rx="1.5"
            fill="rgba(230,237,243,0.55)"
            stroke="none"
          />
        </svg>
        {/* The icon */}
        <AnimatedIcon size={18} cold={[255, 255, 255]} />
        {/* Clock */}
        <span style="opacity:0.7;font-variant-numeric:tabular-nums;min-width:34px;text-align:right;">
          10:42
        </span>
      </div>
      <p style="font-size:12px;color:#484f58;letter-spacing:0.04em;">
        macOS menu bar
      </p>
    </div>
  );
}

// ── DashboardMockup ───────────────────────────────────────────────────────────

function GaugeBar({ label, pct, color }) {
  return (
    <div style="margin-bottom:10px;">
      <div style="display:flex;justify-content:space-between;margin-bottom:4px;">
        <span style="font-size:10px;text-transform:uppercase;letter-spacing:0.08em;color:#8b949e;">
          {label}
        </span>
        <span style={`font-size:10px;color:${color};`}>{pct}%</span>
      </div>
      <div style="height:3px;background:#21262d;border-radius:2px;overflow:hidden;">
        <div
          style={`height:100%;width:${pct}%;background:${color};border-radius:2px;`}
        />
      </div>
    </div>
  );
}

function MiniChart({ data, color }) {
  const max = Math.max(...data, 1);
  return (
    <div style="display:flex;align-items:flex-end;gap:2px;height:36px;">
      {data.map((v, i) => (
        <div
          key={i}
          style={`flex:1;background:${color};opacity:${(0.2 + 0.8 * (v / max)).toFixed(2)};border-radius:2px 2px 0 0;height:${Math.max(4, Math.round((v / max) * 100))}%;`}
        />
      ))}
    </div>
  );
}

function DashboardMockup() {
  const cpuHistory = [
    8, 12, 10, 18, 42, 67, 82, 78, 64, 55, 48, 38, 42, 36, 30,
  ];
  return (
    <div
      style="
      border:1px solid #30363d;
      border-radius:12px;
      overflow:hidden;
      background:#0d1117;
      box-shadow:0 8px 48px rgba(0,0,0,0.6),0 1px 0 rgba(255,255,255,0.04) inset;
      max-width:360px;
      width:100%;
    "
    >
      {/* Browser chrome */}
      <div
        style="
        background:#161b22;
        padding:10px 14px;
        display:flex;
        align-items:center;
        gap:8px;
        border-bottom:1px solid #21262d;
      "
      >
        <div style="display:flex;gap:5px;">
          <div style="width:10px;height:10px;border-radius:50%;background:#f85149;opacity:0.65;" />
          <div style="width:10px;height:10px;border-radius:50%;background:#d29922;opacity:0.65;" />
          <div style="width:10px;height:10px;border-radius:50%;background:#3fb950;opacity:0.65;" />
        </div>
        <div style="flex:1;background:#0d1117;border:1px solid #21262d;border-radius:4px;padding:3px 8px;font-size:10px;color:#8b949e;text-align:center;letter-spacing:0.02em;">
          192.168.1.42:8080
        </div>
      </div>
      {/* Dashboard */}
      <div style="padding:16px;">
        <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:16px;padding-bottom:12px;border-bottom:1px solid #21262d;">
          <h3 style="font-size:15px;font-weight:100;letter-spacing:0.02em;color:#e6edf3;text-shadow:0 0 2px #088,0 0 6px #088;">
            Mac Monitor
          </h3>
          <span style="font-size:10px;color:#3fb950;">● live</span>
        </div>
        <GaugeBar label="CPU" pct={64} color="#58a6ff" />
        <GaugeBar label="Memory" pct={72} color="#3fb950" />
        <GaugeBar label="GPU" pct={24} color="#bc8cff" />
        <div style="margin-top:14px;">
          <div style="font-size:10px;text-transform:uppercase;letter-spacing:0.08em;color:#8b949e;margin-bottom:6px;">
            CPU History (1h)
          </div>
          <MiniChart data={cpuHistory} color="#58a6ff" />
        </div>
      </div>
    </div>
  );
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function GithubIcon() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="currentColor"
      style="flex-shrink:0;"
    >
      <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0 1 12 6.844a9.59 9.59 0 0 1 2.504.337c1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0 0 22 12.017C22 6.484 17.522 2 12 2z" />
    </svg>
  );
}

// ── App ───────────────────────────────────────────────────────────────────────

function App() {
  return (
    <div>
      <main style="max-width:1100px;margin:0 auto;padding:0 24px;">
        {/* ── Hero ── */}
        <section
          style="
          min-height:90vh;
          display:flex;
          flex-direction:column;
          align-items:center;
          justify-content:center;
          text-align:center;
          padding:80px 0 64px;
        "
        >
          <AnimatedIcon size={96} />
          <h1
            style="
            margin-top:36px;
            font-size:clamp(38px,6vw,68px);
            font-weight:100;
            letter-spacing:0.02em;
            color:#e6edf3;
            line-height:1.1;
            text-shadow:0 0 2px #088,0 0 10px #0aa,0 0 20px #0668;
          "
          >
            Mac Monitor
          </h1>
          <p
            style="
            margin-top:20px;
            font-size:clamp(16px,2.5vw,20px);
            color:#8b949e;
            max-width:460px;
            line-height:1.75;
          "
          >
            Lightweight macOS system monitor. Silent in the menu bar. UI in the
            browser.
          </p>
          <div style="margin-top:44px;display:flex;gap:14px;flex-wrap:wrap;justify-content:center;">
            <a
              class="btn btn-primary"
              href="https://github.com/kristiandupont/mac-monitor"
              target="_blank"
              rel="noopener noreferrer"
            >
              <GithubIcon />
              View on GitHub
            </a>
            <span class="btn btn-ghost" title="Coming soon to the App Store">
              App Store — coming soon
            </span>
          </div>
        </section>

        {/* ── Divider ── */}
        <div style="border-top:1px solid #21262d;margin-bottom:96px;" />

        {/* ── Feature 1: Menu Bar Icon ── */}
        <section class="feature-grid">
          <div
            class="demo-col"
            style="display:flex;align-items:center;justify-content:center;padding:32px;"
          >
            <MenuBarDemo />
          </div>
          <div>
            <div class="section-label">Menu Bar</div>
            <h2 style="font-size:clamp(22px,3.5vw,34px);font-weight:200;color:#e6edf3;margin-bottom:18px;line-height:1.2;">
              A Subtle Signal
            </h2>
            <p style="color:#8b949e;line-height:1.85;max-width:420px;font-size:15px;">
              Fanless Macs like the Air can run at high CPU load silently — you
              won't notice until your lap is burning. Mac Monitor's menu bar
              icon rotates slowly when things are calm, and spins faster as the
              load builds. It's subtle enough to ignore, until you don't.
            </p>
          </div>
        </section>

        {/* ── Feature 2: Web UI ── */}
        <section class="feature-grid">
          <div>
            <div class="section-label">Web Dashboard</div>
            <h2 style="font-size:clamp(22px,3.5vw,34px);font-weight:200;color:#e6edf3;margin-bottom:18px;line-height:1.2;">
              Monitor From Anywhere
            </h2>
            <p style="color:#8b949e;line-height:1.85;max-width:420px;font-size:15px;">
              Got a Mac Mini running your personal AI agent in the utility room?
              Mac Monitor serves a full web dashboard over your local network —
              CPU, memory, GPU, disk, and network, updated live every five
              seconds. Check in from any browser on any device.
            </p>
          </div>
          <div
            class="demo-col"
            style="display:flex;align-items:center;justify-content:center;padding:32px;"
          >
            {/* <DashboardMockup /> */}
            <img
              src={import.meta.env.BASE_URL + "screenshot.png"}
              alt="Screenshot of the Mac Monitor web dashboard, showing CPU, memory, and GPU usage."
              style="border-radius:8px;box-shadow:0 8px 48px rgba(0,0,0,0.6),0 1px 0 rgba(255,255,255,0.04) inset;max-width:100%;height:auto;"
            />
          </div>
        </section>
      </main>

      {/* ── Footer ── */}
      <footer
        style="
        border-top:1px solid #21262d;
        padding:36px 24px;
        text-align:center;
        color:#8b949e;
        font-size:13px;
      "
      >
        <div
          style="
          max-width:1100px;
          margin:0 auto;
          display:flex;
          justify-content:center;
          align-items:center;
          gap:32px;
          flex-wrap:wrap;
        "
        >
          <a
            href="https://github.com/kristiandupont/mac-monitor"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-link"
          >
            GitHub — build it yourself, free and open source
          </a>
          <span style="opacity:0.35;">App Store — coming soon</span>
        </div>
      </footer>
    </div>
  );
}

export default App;
