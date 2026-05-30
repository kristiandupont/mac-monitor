export function fmtTime(ts) {
  const d = new Date(ts * 1000);
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export function pad(n) { return String(n).padStart(2, "0"); }

export function fmtBytes(b) {
  if (b >= 1e9) return (b / 1e9).toFixed(1) + " GB/s";
  if (b >= 1e6) return (b / 1e6).toFixed(1) + " MB/s";
  if (b >= 1e3) return (b / 1e3).toFixed(1) + " KB/s";
  return b.toFixed(0) + " B/s";
}

// Returns [{in, out}] byte/s rates aligned to history[1..] (one fewer entry than history).
export function netRates(history, ifaceName) {
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

// Returns [{read, write}] byte/s rates aligned to history[1..].
export function diskIORates(history, diskName) {
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
