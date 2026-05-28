import {LineChart} from "./LineChart.jsx";

export function CpuChart({history}) {
  const labels = history.map(s => fmtTime(s.ts));
  return (
    <LineChart
      datasets={[{labels, data: history.map(s => s.cpu_percent), color: "#58a6ff", label: "CPU %"}]}
      yMax={100}
      formatY={v => `${v.toFixed(0)}%`}
    />
  );
}

function fmtTime(ts) {
  const d = new Date(ts * 1000);
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
function pad(n) { return String(n).padStart(2, "0"); }
