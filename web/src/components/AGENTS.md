# `web/src/components` AGENTS.md

**Purpose**: Presentational Crank.js components for rendering individual metric panels and charts.

**Notes**:
- All components are stateless — they receive data as props and yield JSX. No side effects or internal state.
- `LineChart.jsx` wraps Chart.js; it manages the Chart.js instance lifecycle across Crank re-renders using a `finally` block to destroy the chart on unmount.

**Key Files**:
- `LineChart.jsx`: Reusable multi-dataset line chart via Chart.js.
- `MetricGauge.jsx`: Single-value progress bar with label and formatted value.
- `CpuChart.jsx`, `GpuCard.jsx`, `DiskCard.jsx`, `LoadAvg.jsx`: Domain-specific panels.

**Relationships**: All data flows down from `../App.jsx`. No inter-component dependencies.
