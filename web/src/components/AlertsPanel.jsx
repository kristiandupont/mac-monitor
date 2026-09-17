import {fmtDateTime} from "../utils.js";

const LEVEL = {
  0: {label: "Resolved", color: "#8b949e"},
  1: {label: "Warning",  color: "#d29922"},
  2: {label: "Critical", color: "#f85149"},
};

const STATUS_COLOR = {sent: "#3fb950", pending: "#d29922", failed: "#f85149"};

const h3 = "font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin: 24px 0 10px;";
const row = "display: flex; gap: 12px; align-items: baseline; padding: 8px 0; border-bottom: 1px solid #21262d;";
const muted = "color: #8b949e; font-size: 12px;";
const button = "padding: 2px 10px; border-radius: 12px; border: 1px solid #30363d; background: transparent; color: #8b949e; font-size: 12px; cursor: pointer;";

const ms = (secs) => secs * 1000;

function fmtMinutes(secs) {
  return secs % 60 === 0 ? `${secs / 60} min` : `${secs} s`;
}

// status: GET /api/alerts response (or null while loading)
// onIgnore(process), onUnignore(process): update the CPU ignore list
export function AlertsPanel({status, error, onIgnore, onUnignore}) {
  if (!status) {
    return <p style="color: #8b949e; text-align: center; padding: 40px 0;">{error || "Loading…"}</p>;
  }
  const {alerts, notifications, rules} = status;
  const channels = [rules.native && "macOS notifications", rules.has_command && "shell command"].filter(Boolean);

  const addIgnore = (ev) => {
    ev.preventDefault();
    const input = ev.target.elements.process;
    const name = input.value.trim();
    if (name) onIgnore(name);
    input.value = "";
  };

  return (
    <div style="font-size: 13px; color: #e6edf3;">
      {error && <p style="color: #f85149; margin-bottom: 12px;">{error}</p>}
      <p style={muted}>
        {rules.cpu_percent > 0
          ? `Alerts when a process uses more than ${rules.cpu_percent}% CPU for ${fmtMinutes(rules.cpu_duration_seconds)}`
          : "CPU alerts are off"}
        {" · "}
        {rules.disk.warn_percent > 0 ? `disk warning at ${rules.disk.warn_percent}%` : "disk warnings off"}
        {rules.disk.critical_percent > 0 ? `, critical at ${rules.disk.critical_percent}%` : ""}
        {" · "}
        {channels.length ? `delivered via ${channels.join(" and ")}` : "no delivery channel configured"}
        {". Change these in config.json."}
      </p>

      <h3 style={h3}>Alerts (last 24 hours)</h3>
      {alerts.length === 0 && <p style={muted}>Nothing to report.</p>}
      {alerts.map((a) => (
        <div key={a.key} style={row}>
          <span style={`flex: 0 0 70px; color: ${LEVEL[a.level].color}; font-size: 12px;`}>{LEVEL[a.level].label}</span>
          <span style="flex: 1 1 auto;">
            <div>{a.title}</div>
            <div style={muted}>
              {a.body} · since {fmtDateTime(ms(a.started_at))}
              {a.level === 0 && ` · resolved ${fmtDateTime(ms(a.updated_at))}`}
            </div>
          </span>
          {a.kind === "cpu" && a.level > 0 && (
            <button style={button} onclick={() => onIgnore(a.subject)} title="Never alert about this process">
              Ignore {a.subject}
            </button>
          )}
        </div>
      ))}

      <h3 style={h3}>Notifications</h3>
      {notifications.length === 0 && <p style={muted}>None sent yet.</p>}
      {notifications.map((n) => (
        <div key={n.id} style={row}>
          <span style={`flex: 0 0 70px; color: ${STATUS_COLOR[n.status]}; font-size: 12px; text-transform: capitalize;`}>
            {n.status}
          </span>
          <span style="flex: 1 1 auto;">
            <div>{n.title}</div>
            <div style={muted}>
              {n.channel === "native" ? "macOS" : "command"} · {fmtDateTime(ms(n.created_at))}
              {n.attempts > 1 && ` · ${n.attempts} attempts`}
              {n.status === "pending" && n.attempts > 0 && ` · next try ${fmtDateTime(ms(n.next_attempt_at))}`}
            </div>
            {n.last_error && <div style="color: #f85149; font-size: 12px;">{n.last_error}</div>}
          </span>
        </div>
      ))}

      <h3 style={h3}>Ignored processes</h3>
      <div style="display: flex; flex-wrap: wrap; gap: 6px; align-items: center;">
        {rules.cpu_ignore.map((name) => (
          <span key={name} style="padding: 2px 4px 2px 10px; border-radius: 12px; border: 1px solid #30363d;">
            {name}
            <button
              style="border: none; background: transparent; color: #8b949e; cursor: pointer; padding: 0 6px;"
              title={`Alert about ${name} again`}
              onclick={() => onUnignore(name)}
            >
              ×
            </button>
          </span>
        ))}
        <form onsubmit={addIgnore} style="display: inline-flex; gap: 6px;">
          <input
            name="process"
            placeholder="Process name"
            style="background: #0d1117; border: 1px solid #30363d; border-radius: 12px; color: #e6edf3; padding: 2px 10px; font-size: 12px;"
          />
          <button type="submit" style={button}>Add</button>
        </form>
      </div>
    </div>
  );
}
