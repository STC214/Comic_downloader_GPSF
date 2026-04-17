# Interface Flow

This document describes the current task and UI contract used by the refactor skeleton.

## Input

- `url`
- `downloadRoot`

## Task lifecycle

The task manager tracks these states:

- `queued`
- `routing`
- `preparing`
- `prepared`
- `running`
- `waiting_verification`
- `verification_cleared`
- `completed`
- `failed`

## Runtime files

Each task writes files under `runtime/`:

- `runtime/tasks/task-<id>/state.json`
- `runtime/tasks/task-<id>/report.json`
- `runtime/logs/task-<id>.log`
- `runtime/output/task-<id>/`
- `runtime/thumbnails/task-<id>/`

## UI data flow

- The task list is built from live task state plus report snapshots.
- Task details read `report.json` and append recent `task.log` content.
- The selected task log panel appends `task.log` to the in-memory log buffer.

## Browser data flow

- The browser layer resolves a launch spec before launch.
- The launch spec includes browser path, selected mother profile, temporary Playwright profile path, headless, keep-open, locale, timezone, viewport, Firefox user prefs, and the stealth script path.
- The browser session boundary uses Playwright persistent context with a copied temporary profile directory.
- Browser middleware owns stealth injection, Firefox user prefs, and launch defaults.
- Firefox defaults to `C:\Program Files\Mozilla Firefox\firefox.exe`.
- Firefox profile selection defaults to `C:\Users\stc52\AppData\Roaming\Mozilla\Firefox\Profiles\jo2klram.default-release`.
- The fixed Firefox User-Agent is `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:149.0) Gecko/20100101 Firefox/149.0`.
- The `task-probe` CLI is the current browser smoke test entry point.
- The `zeri` summary/reader contract is documented in [`docs/zeri_flow_rules.md`](zeri_flow_rules.md).

## Browser mother profiles

- Firefox task runs copy the selected mother profile into `runtime/browser-profiles/tasks/firefox/<worker>/task-<id>/original-userdata`.
- Task preparation then clones that directory into `content` and `verify`.
- Direct browser probes without a task still copy the selected mother profile into a temporary Playwright profile under `runtime/browser-profiles/tasks/firefox-playwright-*`.
- Task cleanup removes the whole task profile root, including the task-scoped mother copy.

## Notes

- The current browser adapter is Playwright-first and centered on Firefox.
- Firefox tasks and probes use `runtime/firefox_stealth.js`.
- The browser layer also handles `keep-open` so the run can wait until the browser window is manually closed.
- Task output and logs still live under `runtime/` as before.
