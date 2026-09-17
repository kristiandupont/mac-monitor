# `web/src` AGENTS.md

**Purpose**: Root of the Crank.js front-end. Connects to the Go server via WebSocket and renders live system metrics.

**Notes**:
- Uses [Crank.js](https://crank.js.org/) (not React). Generator functions (`function*`) are stateful components; `yield` replaces `return`.
- JSX is transpiled with a custom factory (`createElement`/`Fragment` from `@b9g/crank`) — configured in `vite.config.js`.
- `App.jsx` owns the WebSocket lifecycle and history buffer. Pure data helpers (rate calculations, formatting, interface selection) live in `utils.js`.

**Key Files**:
- `App.jsx`: Root component — WS setup, visible time range (`span` + `end`, `end === null` means live), debounced history fetching, layout.
- `utils.js`: Pure data-transform and formatting helpers; all unit-tested.
- `main.jsx`: Entry point — mounts `App` into the DOM.

**Notes**:
- Tab state (`overview` / `processes` / `alerts`) lives in `App.jsx`. Only the visible tab is polled (`POLLERS`: `/api/processes` every 5 s, `/api/alerts` every 10 s); the timer is cleared on tab switch or unmount.

**Relationships**: Pulls data from Go server at `/api/live` (WS), `/api/history` (HTTP), and `/api/processes` and `/api/alerts` (HTTP, polled only while their tab is active). Components in `./components/`.
