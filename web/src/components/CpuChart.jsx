import {Chart, LineElement, PointElement, LineController, CategoryScale, LinearScale, Filler, Tooltip} from "chart.js";

Chart.register(LineElement, PointElement, LineController, CategoryScale, LinearScale, Filler, Tooltip);

export function* CpuChart({history}) {
  let canvas;
  let chart;

  this.flush(() => {
    if (!canvas) return;

    const labels = history.map(s => {
      const d = new Date(s.ts * 1000);
      return `${d.getHours().toString().padStart(2, "0")}:${d.getMinutes().toString().padStart(2, "0")}:${d.getSeconds().toString().padStart(2, "0")}`;
    });
    const data = history.map(s => s.cpu_percent);

    if (!chart) {
      chart = new Chart(canvas, {
        type: "line",
        data: {
          labels,
          datasets: [{
            data,
            borderColor: "#58a6ff",
            backgroundColor: "rgba(88,166,255,0.1)",
            borderWidth: 1.5,
            pointRadius: 0,
            fill: true,
            tension: 0.3,
          }],
        },
        options: {
          animation: false,
          responsive: true,
          maintainAspectRatio: false,
          scales: {
            x: {
              ticks: {color: "#8b949e", maxTicksLimit: 8, font: {family: "SF Mono, Fira Code, monospace", size: 10}},
              grid: {color: "#21262d"},
            },
            y: {
              min: 0,
              max: 100,
              ticks: {color: "#8b949e", font: {family: "SF Mono, Fira Code, monospace", size: 10}},
              grid: {color: "#21262d"},
            },
          },
          plugins: {legend: {display: false}},
        },
      });
    } else {
      chart.data.labels = labels;
      chart.data.datasets[0].data = data;
      chart.update("none");
    }
  });

  try {
    for ({history} of this) {
      yield (
        <div style="position: relative; height: 180px;">
          <canvas ref={el => canvas = el} />
        </div>
      );
    }
  } finally {
    chart?.destroy();
  }
}
