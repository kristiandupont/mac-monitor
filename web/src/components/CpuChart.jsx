import {LineChart} from "./LineChart.jsx";
import {toPoints} from "../utils.js";

// view: {xMin, xMax, onView, onReset} passed through to LineChart
export function CpuChart({history, maxGap, view}) {
  const data = toPoints(history.map(s => s.ts), history.map(s => s.cpu_percent), maxGap);
  return (
    <LineChart
      datasets={[{data, color: "#58a6ff", label: "CPU %"}]}
      yMax={100}
      formatY={v => `${v.toFixed(0)}%`}
      {...view}
    />
  );
}
