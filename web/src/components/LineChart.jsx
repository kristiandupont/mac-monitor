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

// datasets: [{labels, data, color, label, fill?}]
// yMax: number | undefined (auto)
// formatY: function(value) => string
export function* LineChart({datasets, yMax, formatY, height = 180}) {
  let canvas;
  let chart;
  let props = {datasets, yMax, formatY, height};

  this.flush(() => {
    if (!canvas || !props.datasets?.length) return;

    const labels = props.datasets[0].labels ?? [];
    const chartDatasets = props.datasets.map(ds => ({
      label: ds.label ?? "",
      data: ds.data ?? [],
      borderColor: ds.color,
      backgroundColor: hexAlpha(ds.color, ds.fill !== false ? 0.12 : 0),
      borderWidth: 1.5,
      pointRadius: 0,
      fill: ds.fill !== false,
      tension: 0.3,
    }));

    if (!chart) {
      chart = new Chart(canvas, {
        type: "line",
        data: {labels, datasets: chartDatasets},
        options: {
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
              ...(props.yMax != null ? {max: props.yMax} : {}),
              ticks: {
                ...TICK_STYLE,
                callback: props.formatY ?? (v => v),
              },
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
                label: ctx => {
                  const fmt = props.formatY ?? (v => v);
                  return ` ${ctx.dataset.label}: ${fmt(ctx.parsed.y)}`;
                },
              },
            },
          },
        },
      });
    } else {
      chart.data.labels = labels;
      props.datasets.forEach((ds, i) => {
        if (chart.data.datasets[i]) {
          chart.data.datasets[i].data = ds.data;
        }
      });
      if (props.yMax != null) {
        chart.options.scales.y.max = props.yMax;
      } else {
        delete chart.options.scales.y.max;
      }
      chart.options.scales.y.ticks.callback = props.formatY ?? (v => v);
      chart.update("none");
    }
  });

  try {
    for (props of this) {
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
