# Dashboard Theming & Dev Mode

## Dark/Light Toggle

The dashboard supports dark and light themes with a toggle button in the header. All colors use CSS custom properties (`:root` for dark, `html.light` for light). The choice is saved in `localStorage` under `docket-theme`.

## Dev Mode (live reload without restart)

The dashboard handler checks for `dashboard/index.html` on disk (relative to the working directory) on every request. If found, it serves that file instead of the embedded copy. This means during development you can edit the HTML and just refresh the browser — no rebuild or restart needed.

In production (installed binary running from a different directory), the file won't exist on disk, so the embedded version is used automatically.

## Key files

- `dashboard/index.html` — CSS variables, `.light` class overrides, `initTheme()`/`toggleTheme()` functions
- `internal/dashboard/dashboard.go` — dev-mode file override in the `GET /` handler
