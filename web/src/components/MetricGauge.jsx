export function MetricGauge({label, value, max, unit = "%", color = "#58a6ff"}) {
  const pct = max ? Math.min((value / max) * 100, 100) : Math.min(value, 100);
  const displayValue = max
    ? `${fmtBytes(value)} / ${fmtBytes(max)}`
    : `${pct.toFixed(1)}%`;

  return (
    <div style="margin-bottom: 20px;">
      <div style="display: flex; justify-content: space-between; margin-bottom: 6px;">
        <span style="color: #8b949e; text-transform: uppercase; font-size: 11px; letter-spacing: 0.08em;">
          {label}
        </span>
        <span style={`color: ${color}; font-weight: bold;`}>{displayValue}</span>
      </div>
      <div style="height: 6px; background: #21262d; border-radius: 3px; overflow: hidden;">
        <div style={`height: 100%; width: ${pct}%; background: ${color}; border-radius: 3px; transition: width 0.4s ease;`} />
      </div>
    </div>
  );
}

function fmtBytes(bytes) {
  if (bytes >= 1e9) return (bytes / 1e9).toFixed(1) + " GB";
  if (bytes >= 1e6) return (bytes / 1e6).toFixed(1) + " MB";
  if (bytes >= 1e3) return (bytes / 1e3).toFixed(1) + " KB";
  return bytes + " B";
}
