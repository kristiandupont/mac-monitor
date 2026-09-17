# `web/src/components` AGENTS.md

**Purpose**: Presentational Crank.js components for rendering individual metric panels and charts.

**Notes**:
- All components are stateless — they receive data as props and yield JSX. No side effects or internal state.
- `LineChart.jsx` wraps Chart.js; it manages the Chart.js instance lifecycle across Crank re-renders using a `finally` block to destroy the chart on unmount. The x-axis is linear in ms; data is `{x, y}` points with `y: null` marking gaps. Zoom/pan gestures are reported via `onView` — the range itself is owned by `App.jsx` so all charts stay in sync.

**Key Files**:
- `LineChart.jsx`: Reusable multi-dataset line chart via Chart.js.
- `MetricGauge.jsx`: Single-value progress bar with label and formatted value.
- `CpuChart.jsx`, `GpuCard.jsx`, `DiskCard.jsx`, `LoadAvg.jsx`: Domain-specific panels.
- `AlertsPanel.jsx`: Alerts tab; stateless, ignore-list changes go through `onIgnore`/`onUnignore` callbacks.
- `ProcessTable.jsx`: Sortable process list (by CPU desc); stateless — receives `processes` array from `App.jsx`.

**Relationships**: All data flows down from `../App.jsx`. No inter-component dependencies.
