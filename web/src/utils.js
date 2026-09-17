export function fmtDateTime(ms) {
  const d = new Date(ms);
  return `${d.toLocaleDateString(undefined, {month: "short", day: "numeric"})} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export function pad(n) { return String(n).padStart(2, "0"); }

export function fmtBytes(b) {
  if (b >= 1e9) return (b / 1e9).toFixed(1) + " GB/s";
  if (b >= 1e6) return (b / 1e6).toFixed(1) + " MB/s";
  if (b >= 1e3) return (b / 1e3).toFixed(1) + " KB/s";
  return b.toFixed(0) + " B/s";
}

export function fmtSize(b) {
  if (b >= 1073741824) return (b / 1073741824).toFixed(1) + " GB";
  if (b >= 1048576)    return (b / 1048576).toFixed(1) + " MB";
  if (b >= 1024)       return (b / 1024).toFixed(1) + " KB";
  return b + " B";
}

// Returns [{in, out}] byte/s rates aligned to history[1..] (one fewer entry than history).
// Rates spanning more than maxGap seconds are null, since the counters say
// nothing useful about what happened while collection was stopped.
export function netRates(history, ifaceName, maxGap = Infinity) {
  const rates = [];
  for (let i = 1; i < history.length; i++) {
    const prev = history[i - 1];
    const curr = history[i];
    const dt = curr.ts - prev.ts;
    if (dt > maxGap) { rates.push({in: null, out: null}); continue; }
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

// Returns [{read, write}] byte/s rates aligned to history[1..]. See netRates for maxGap.
export function diskIORates(history, diskName, maxGap = Infinity) {
  const rates = [];
  for (let i = 1; i < history.length; i++) {
    const prev = history[i - 1];
    const curr = history[i];
    const dt = curr.ts - prev.ts;
    if (dt > maxGap) { rates.push({read: null, write: null}); continue; }
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

// Prefers en0; falls back to busiest non-loopback, non-tunnel interface.
export function primaryIface(snap) {
  if (!snap?.net_stats?.length) return null;
  const ifaces = snap.net_stats.filter(n => n.name !== "lo0" && !n.name.startsWith("utun"));
  if (!ifaces.length) return null;
  const en0 = ifaces.find(n => n.name === "en0");
  if (en0) return en0.name;
  return ifaces.reduce((a, b) => (a.bytes_recv + a.bytes_sent > b.bytes_recv + b.bytes_sent ? a : b)).name;
}

// Returns the lexicographically first disk device name (lowest-numbered).
export function primaryDisk(snap) {
  if (!snap?.disk_io_stats?.length) return null;
  return snap.disk_io_stats.map(d => d.name).sort()[0];
}

// Converts parallel timestamp (seconds) and value arrays into Chart.js {x, y}
// points with x in ms. A null point is inserted wherever consecutive samples
// are more than maxGap seconds apart, so the line breaks instead of bridging.
export function toPoints(ts, values, maxGap) {
  const points = [];
  for (let i = 0; i < ts.length; i++) {
    if (i > 0 && ts[i] - ts[i - 1] > maxGap) {
      points.push({x: (ts[i - 1] + ts[i]) * 500, y: null});
    }
    points.push({x: ts[i] * 1000, y: values[i]});
  }
  return points;
}

// Sample spacing (seconds) beyond which two samples are considered a gap.
export function gapThreshold(step) {
  return 3 * Math.max(step, 5);
}

const TICK_STEPS_S = [
  1, 5, 10, 15, 30, 60, 120, 300, 600, 900, 1800,
  3600, 7200, 10800, 21600, 43200, 86400, 172800, 345600, 604800,
];

// Returns tick positions (ms) at round local-time intervals within [min, max].
export function timeTicks(min, max, maxTicks = 8) {
  const span = max - min;
  const step = (TICK_STEPS_S.find(s => span / (s * 1000) <= maxTicks) ?? TICK_STEPS_S.at(-1)) * 1000;
  const ticks = [];
  for (let t = alignLocal(min, step, Math.ceil); t <= max; t = alignLocal(t + step, step, Math.round)) {
    ticks.push(t);
  }
  return ticks;
}

// Rounds ms to a multiple of step in local time (so ticks land on local
// midnight rather than UTC midnight).
function alignLocal(ms, step, round) {
  const offset = new Date(ms).getTimezoneOffset() * 60000;
  return round((ms - offset) / step) * step + offset;
}

// Formats a tick label; stepMs is the spacing between ticks.
export function fmtTick(ms, stepMs) {
  const d = new Date(ms);
  const midnight = d.getHours() === 0 && d.getMinutes() === 0 && d.getSeconds() === 0;
  if (stepMs >= 86400000 || (midnight && stepMs >= 3600000)) {
    return d.toLocaleDateString(undefined, {month: "short", day: "numeric"});
  }
  if (stepMs < 60000) return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
