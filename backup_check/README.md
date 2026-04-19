# Comic Downloader

Windows comic downloader in Go.

This repository now uses the Go refactor stack as the active application entry.
The browser/task/runtime layers live under `browser/`, `tasks/`, `sites/`, `runtime/`, `siteflow/`, `ui/`, and `internal/app/`.

The active stack is Go + Playwright-first browser sessions + browser middleware stealth injection.

## Current contract

- The frontend submits `url` and `downloadRoot`.
- The filter layer resolves the route and decides `headless`, `verify`, `browser type`, and `HTTPOnly`.
- `zeri` uses the Firefox browser path and a browser-backed HTTP download flow.
- `myreadingmanga` stays verification-aware on Chromium.
- `nyahentai` and `hentai2` stay browser-driven on Chromium.
- Task state, reports, and logs are written to `runtime/`.
- The UI can read `runtime/tasks/task-<id>/report.json` and `runtime/logs/task-<id>.log`.

## Entry points

- Active app: `go run .`
- Alternate CLI entry: `go run ./cmd/comic-downloader`

## Quick start

```powershell
go test ./...
go run ./cmd/comic-downloader --workspace-root .
```

## Runtime layout

- `runtime/browser-profiles/` stores task-scoped browser profile copies.
- `runtime/browser-profiles/<worker>/task-<id>/original-userdata` stores the task-scoped mother profile copy.
- `runtime/output/` stores task output.
- `runtime/thumbnails/` stores task thumbnails.
- `runtime/tasks/task-<id>/state.json` stores task state.
- `runtime/tasks/task-<id>/report.json` stores the normalized task report.
- `runtime/logs/task-<id>.log` stores the human-readable task log.

## Notes

- The browser layer uses a Playwright-first session adapter with a filesystem fallback so the task/session boundary stays concrete even when a browser runtime is unavailable.
- Chromium tasks default to `runtime/chromium/chrome.exe` plus `runtime/chrome_stealth.js`, and the userdata resolver prefers the current Windows default browser association before falling back to Chromium-family profile paths.
- Firefox tasks auto-resolve the local Firefox install plus `runtime/firefox_stealth.js`, and the profile resolver prefers `profiles.ini` default entries when available.
- The browser middleware automatically injects the correct stealth script before any page is opened.
- The browser middleware also owns adblock rule loading and page blocking.
- The UI can read the new report/log files and merge them into the task list and detail views.
- Browser profile isolation and the Chromium/Firefox mother-profile flow are documented in [`docs/browser_profile_flow.md`](docs/browser_profile_flow.md).
- The current `zeri` summary/reader rules are documented in [`docs/zeri_flow_rules.md`](docs/zeri_flow_rules.md).
