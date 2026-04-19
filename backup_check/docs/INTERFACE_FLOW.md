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
- The launch spec includes browser type, Chromium path, Firefox path, userdata path, baseline path, and task-scoped content / verification dirs.
- The browser session boundary imports and exports storage state and records init scripts.
- Browser middleware owns stealth injection and adblock rule application.
- Chromium defaults to `runtime/chromium/chrome.exe`.
- Firefox defaults to the system-installed Firefox executable and the local Firefox profile as mother profile.
- The `zeri` summary/reader contract is documented in [`docs/zeri_flow_rules.md`](zeri_flow_rules.md).

## Browser mother profiles

- Chromium tasks copy the machine-local Chromium/Chrome-style userdata into `runtime/browser-profiles/<worker>/task-<id>/original-userdata`.
- Firefox tasks copy the local Firefox profile into the same task-scoped `original-userdata` directory.
- Both paths then clone that mother profile into `baseline-userdata`, `content`, and `verify`.
- Task cleanup removes the whole task profile root, including the task-scoped mother copy.

## Notes

- The current browser adapter is Playwright-first with a filesystem fallback so the task/session contract stays runnable when a live browser runtime is unavailable.
- Chromium tasks use `runtime/chrome_stealth.js`.
- Firefox tasks use `runtime/firefox_stealth.js`.
- Adblock rules are cached under `runtime/adblock/` when the source list is available.
