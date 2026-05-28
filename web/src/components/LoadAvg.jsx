export function LoadAvg({load1, load5, load15}) {
  return (
    <div style="margin-bottom: 20px;">
      <div style="color: #8b949e; text-transform: uppercase; font-size: 11px; letter-spacing: 0.08em; margin-bottom: 10px;">
        Load Average
      </div>
      <div style="display: flex; gap: 24px;">
        {[["1m", load1], ["5m", load5], ["15m", load15]].map(([label, val]) => (
          <div key={label} style="text-align: center;">
            <div style="font-size: 20px; font-weight: bold; color: #e6edf3;">
              {val.toFixed(2)}
            </div>
            <div style="font-size: 11px; color: #8b949e; margin-top: 2px;">{label}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
