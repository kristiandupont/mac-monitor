import {MetricGauge} from "./MetricGauge.jsx";

export function GpuCard({gpuStats}) {
  if (!gpuStats || gpuStats.length === 0) {
    return (
      <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
        <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em; margin-bottom: 12px;">GPU</h2>
        <p style="color: #8b949e; font-size: 12px;">No GPU data available</p>
      </div>
    );
  }

  return (
    <div style="background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px;">
      {gpuStats.map((gpu, i) => (
        <div key={i}>
          <div style="display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 16px;">
            <h2 style="font-size: 12px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.08em;">GPU</h2>
            <span style="font-size: 11px; color: #8b949e;">{gpu.name}{gpu.core_count ? ` · ${gpu.core_count} cores` : ""}</span>
          </div>

          <MetricGauge label="Device" value={gpu.device_utilization} color="#bc8cff" />
          <MetricGauge label="Renderer" value={gpu.renderer_utilization} color="#d2a8ff" />
          <MetricGauge label="Tiler" value={gpu.tiler_utilization} color="#a5d6ff" />

          {gpu.mem_allocated > 0 && (
            <MetricGauge
              label="VRAM In Use"
              value={gpu.mem_in_use}
              max={gpu.mem_allocated}
              color="#bc8cff"
            />
          )}
        </div>
      ))}
    </div>
  );
}
