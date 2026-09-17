import {
  Chart, LineElement, PointElement, LineController,
  LinearScale, Filler, Tooltip, Legend,
} from "chart.js";
import {timeTicks, fmtTick, fmtDateTime} from "../utils.js";

Chart.register(LineElement, PointElement, LineController, LinearScale, Filler, Tooltip, Legend);

const GRID_COLOR = "#21262d";
const TICK_STYLE = {color: "#8b949e", font: {family: "SF Mono, Fira Code, monospace", size: 10}};

function hexAlpha(hex, alpha) {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `rgba(${r},${g},${b},${alpha})`;
}

function buildOptions(props) {
  return {
    animation: false,
    responsive: true,
    maintainAspectRatio: false,
    parsing: false,
    normalized: true,
    interaction: {mode: "nearest", axis: "x", intersect: false},
    scales: {
      x: {
        type: "linear",
        min: props.xMin,
        max: props.xMax,
        // Place ticks on round local times rather than Chart.js's numeric steps.
        afterBuildTicks: scale => {
          scale.ticks = timeTicks(scale.min, scale.max).map(value => ({value}));
        },
        ticks: {
          ...TICK_STYLE,
          maxRotation: 0,
          callback: (v, i, ticks) => fmtTick(v, ticks.length > 1 ? ticks[1].value - ticks[0].value : 60000),
        },
        grid: {color: GRID_COLOR},
      },
      y: {
        min: 0,
        ticks: {...TICK_STYLE},
        grid: {color: GRID_COLOR},
      },
    },
    plugins: {
      legend: {
        display: props.datasets.length > 1,
        labels: {color: "#8b949e", font: {family: "SF Mono, Fira Code, monospace", size: 10}, boxWidth: 12},
      },
      tooltip: {
        callbacks: {
          title: items => items.length ? fmtDateTime(items[0].parsed.x) : "",
        },
      },
    },
  };
}

// Options that follow props on every render.
function applyProps(chart, props) {
  const formatY = props.formatY ?? (v => v);
  const {x, y} = chart.options.scales;
  x.min = props.xMin;
  x.max = props.xMax;
  if (props.yMax != null) y.max = props.yMax;
  else delete y.max;
  y.ticks.callback = formatY;
  chart.options.plugins.tooltip.callbacks.label = ctx => ` ${ctx.dataset.label}: ${formatY(ctx.parsed.y)}`;
}

// Wheel/pinch zooms around the cursor, horizontal scroll and drag pan,
// double-click resets. Plain vertical scrolling is left to the page.
function attachInteractions(canvas, getChart, getProps) {
  let drag = null;

  const emit = (min, max) => getProps().onView?.(min, max);

  canvas.addEventListener("wheel", e => {
    const chart = getChart();
    if (!chart) return;
    const {min, max} = chart.scales.x;
    const span = max - min;
    if (e.ctrlKey || e.metaKey || e.altKey) {
      e.preventDefault();
      const t = chart.scales.x.getValueForPixel(e.offsetX);
      const factor = Math.exp(e.deltaY * (e.ctrlKey ? 0.01 : 0.002));
      const newSpan = span * factor;
      const newMin = t - (t - min) * factor;
      emit(newMin, newMin + newSpan);
    } else if (Math.abs(e.deltaX) > Math.abs(e.deltaY)) {
      e.preventDefault();
      const {left, right} = chart.chartArea;
      const dt = e.deltaX * span / (right - left);
      emit(min + dt, max + dt);
    }
  }, {passive: false});

  canvas.addEventListener("pointerdown", e => {
    const chart = getChart();
    if (!chart || e.button !== 0) return;
    const {min, max} = chart.scales.x;
    const {left, right} = chart.chartArea;
    drag = {x: e.clientX, min, max, msPerPx: (max - min) / (right - left)};
    canvas.setPointerCapture(e.pointerId);
    canvas.style.cursor = "grabbing";
  });

  canvas.addEventListener("pointermove", e => {
    if (!drag) return;
    const dt = (e.clientX - drag.x) * drag.msPerPx;
    emit(drag.min - dt, drag.max - dt);
  });

  const endDrag = e => {
    if (!drag) return;
    drag = null;
    canvas.releasePointerCapture(e.pointerId);
    canvas.style.cursor = "grab";
  };
  canvas.addEventListener("pointerup", endDrag);
  canvas.addEventListener("pointercancel", endDrag);

  canvas.addEventListener("dblclick", () => getProps().onReset?.());
  canvas.style.cursor = "grab";
  canvas.style.touchAction = "none";
}

// datasets: [{data: [{x: ms, y}], color, label, fill?}]
// xMin, xMax: visible time range in ms
// yMax: number | undefined (auto-scale)
// formatY: value => string
// onView(minMs, maxMs): called when the user zooms or pans
// onReset(): called on double-click
export function* LineChart({datasets, yMax, formatY, height = 180}) {
  let canvas;
  let chart;
  let latest;

  try {
    for (const props of this) {
      latest = props;
      // Re-register flush every render so the chart stays in sync with live data.
      this.flush(() => {
        if (!canvas || !props.datasets?.length) return;

        if (!chart) {
          const chartDatasets = props.datasets.map(ds => ({
            label:           ds.label ?? "",
            data:            ds.data ?? [],
            borderColor:     ds.color,
            backgroundColor: hexAlpha(ds.color, ds.fill !== false ? 0.12 : 0),
            borderWidth:     1.5,
            pointRadius:     0,
            fill:            ds.fill !== false,
            tension:         0.3,
          }));
          chart = new Chart(canvas, {type: "line", data: {datasets: chartDatasets}, options: buildOptions(props)});
          attachInteractions(canvas, () => chart, () => latest);
        } else {
          props.datasets.forEach((ds, i) => {
            if (chart.data.datasets[i]) chart.data.datasets[i].data = ds.data;
          });
        }
        applyProps(chart, props);
        chart.update("none");
      });

      yield (
        <div style={`position: relative; height: ${props.height || 180}px;`}>
          <canvas ref={el => (canvas = el)} />
        </div>
      );
    }
  } finally {
    chart?.destroy();
  }
}
