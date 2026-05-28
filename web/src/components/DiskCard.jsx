import {MetricGauge} from "./MetricGauge.jsx";

export function DiskCard({diskStats}) {
  if (!diskStats || diskStats.length === 0) {
    return (
      <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
        <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 12px;">Disk</h2>
        <p style="color: #8b949e; font-size: 12px;">No disk data available</p>
      </div>
    );
  }

  return (
    <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
      <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 16px;">Disk</h2>
      {diskStats.map((d, i) => (
        <MetricGauge
          key={i}
          label={d.mount_point}
          value={d.used}
          max={d.total}
          color={d.used_percent > 90 ? "#f85149" : d.used_percent > 75 ? "#d29922" : "#3fb950"}
        />
      ))}
    </div>
  );
}
