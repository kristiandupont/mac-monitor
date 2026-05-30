import {fmtSize} from "../utils.js";

const COL = {
  name:    {label: "Process",  flex: "1 1 auto",   align: "left"},
  cpu:     {label: "CPU %",    flex: "0 0 80px",   align: "right"},
  mem:     {label: "Memory",   flex: "0 0 90px",   align: "right"},
  pid:     {label: "PID",      flex: "0 0 60px",   align: "right"},
};

const cellStyle = (col) =>
  `flex: ${col.flex}; text-align: ${col.align}; padding: 5px 8px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;`;

const headerStyle = (col) =>
  `${cellStyle(col)} font-size: 11px; color: #8b949e; text-transform: uppercase; letter-spacing: 0.06em;`;

export function ProcessTable({processes, cpuReady}) {
  if (!processes) {
    return (
      <p style="color: #8b949e; text-align: center; padding: 40px 0;">
        Loading…
      </p>
    );
  }

  const sorted = [...processes].sort((a, b) => b.cpu_percent - a.cpu_percent);

  return (
    <div style="font-size: 13px; color: #e6edf3;">
      {/* header */}
      <div style="display: flex; border-bottom: 1px solid #30363d; margin-bottom: 2px;">
        <span style={headerStyle(COL.name)}>{COL.name.label}</span>
        <span style={headerStyle(COL.cpu)}>{COL.cpu.label}</span>
        <span style={headerStyle(COL.mem)}>{COL.mem.label}</span>
        <span style={headerStyle(COL.pid)}>{COL.pid.label}</span>
      </div>

      {/* rows */}
      <div style="max-height: 70vh; overflow-y: auto; scrollbar-gutter: stable;">
        {sorted.map(p => (
          <div key={p.pid}
            style="display: flex; border-bottom: 1px solid #21262d; align-items: center;"
          >
            <span style={cellStyle(COL.name)} title={p.name}>{p.name}</span>
            <span style={`${cellStyle(COL.cpu)}; color: ${cpuColor(p.cpu_percent)};`}>
              {cpuReady ? p.cpu_percent.toFixed(1) : ""}
            </span>
            <span style={cellStyle(COL.mem)}>{fmtSize(p.mem_rss)}</span>
            <span style={`${cellStyle(COL.pid)}; color: #8b949e;`}>{p.pid}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function cpuColor(pct) {
  if (pct >= 50) return "#f85149";
  if (pct >= 20) return "#d29922";
  if (pct >= 5)  return "#3fb950";
  return "#e6edf3";
}
