# `web/src` AGENTS.md

**Purpose**: Root of the Crank.js front-end. Connects to the Go server via WebSocket and renders live system metrics.

**Notes**:
- Uses [Crank.js](https://crank.js.org/) (not React). Generator functions (`function*`) are stateful components; `yield` replaces `return`.
- JSX is transpiled with a custom factory (`createElement`/`Fragment` from `@b9g/crank`) — configured in `vite.config.js`.
- `App.jsx` owns the WebSocket lifecycle and history buffer. Pure data helpers (rate calculations, formatting, interface selection) live in `utils.js`.

**Key Files**:
- `App.jsx`: Root component — WS setup, history ring buffer, layout.
- `utils.js`: Pure data-transform and formatting helpers; all unit-tested.
- `main.jsx`: Entry point — mounts `App` into the DOM.

**Notes**:
- Tab state (`overview` / `processes`) lives in `App.jsx`. The process poller (`setInterval` on `/api/processes`) starts only when the processes tab is active and is cleared on tab switch or unmount.

**Relationships**: Pulls data from Go server at `/api/live` (WS), `/api/history` (HTTP), and `/api/processes` (HTTP, polled every 5 s only while processes tab is active). Components in `./components/`.
