import {
  Chart, LineElement, PointElement, LineController,
  CategoryScale, LinearScale, Filler, Tooltip, Legend,
} from "chart.js";

Chart.register(LineElement, PointElement, LineController, CategoryScale, LinearScale, Filler, Tooltip, Legend);

const GRID_COLOR = "#21262d";
const TICK_STYLE = {color: "#8b949e", font: {family: "SF Mono, Fira Code, monospace", size: 10}};

function hexAlpha(hex, alpha) {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `rgba(${r},${g},${b},${alpha})`;
}

function buildOptions(yMax, formatY) {
  return {
    animation: false,
    responsive: true,
    maintainAspectRatio: false,
    interaction: {mode: "index", intersect: false},
    scales: {
      x: {
        ticks: {...TICK_STYLE, maxTicksLimit: 8},
        grid: {color: GRID_COLOR},
      },
      y: {
        min: 0,
        ...(yMax != null ? {max: yMax} : {}),
        ticks: {
          ...TICK_STYLE,
          callback: formatY ?? (v => v),
        },
        grid: {color: GRID_COLOR},
      },
    },
    plugins: {
      legend: {
        display: false, // toggled per-render based on dataset count
        labels: {color: "#8b949e", font: {family: "SF Mono, Fira Code, monospace", size: 10}, boxWidth: 12},
      },
      tooltip: {
        callbacks: {
          label: (ctx, fmt) => ` ${ctx.dataset.label}: ${(fmt ?? (v => v))(ctx.parsed.y)}`,
        },
      },
    },
  };
}

// datasets: [{labels, data, color, label, fill?}]
// yMax: number | undefined (auto-scale)
// formatY: value => string
export function* LineChart({datasets, yMax, formatY, height = 180}) {
  let canvas;
  let chart;

  try {
    for (const props of this) {
      // Re-register flush every render so the chart stays in sync with live data.
      this.flush(() => {
        if (!canvas || !props.datasets?.length) return;

        const labels = props.datasets[0].labels ?? [];
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

        if (!chart) {
          const options = buildOptions(props.yMax, props.formatY);
          options.plugins.legend.display = props.datasets.length > 1;
          options.plugins.tooltip.callbacks.label = ctx =>
            ` ${ctx.dataset.label}: ${(props.formatY ?? (v => v))(ctx.parsed.y)}`;
          chart = new Chart(canvas, {type: "line", data: {labels, datasets: chartDatasets}, options});
        } else {
          chart.data.labels = labels;
          props.datasets.forEach((ds, i) => {
            if (chart.data.datasets[i]) chart.data.datasets[i].data = ds.data;
          });
          if (props.yMax != null) chart.options.scales.y.max = props.yMax;
          else delete chart.options.scales.y.max;
          chart.options.scales.y.ticks.callback = props.formatY ?? (v => v);
          chart.options.plugins.tooltip.callbacks.label = ctx =>
            ` ${ctx.dataset.label}: ${(props.formatY ?? (v => v))(ctx.parsed.y)}`;
          chart.update("none");
        }
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
