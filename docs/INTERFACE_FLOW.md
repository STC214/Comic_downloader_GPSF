# Interface Flow

This document describes the current UI and task contract used by the refactor.

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

Current frontend behavior:

- Clicking `Add task` starts the task immediately.
- The task list acts as a live run list plus history, not as a waiting queue.
- Duplicate tasks are detected by `URL + browser type` and prompt before adding.
- `Start all unfinished tasks` still exists and now respects the configured concurrency value.
- `myreadingmanga.info` is blocked at add time and shows `暂不支持此站点`.

## Runtime files

Each task writes files under `runtime/`:

- `runtime/tasks/task-<id>/state.json`
- `runtime/tasks/task-<id>/report.json`
- `runtime/logs/task-<id>.log`
- `runtime/output/task-<id>/`
- `runtime/thumbnails/task-<id>/thumb.jpg`

The task report file is the source of truth for the details dialog, and the thumbnail path stored in the report is what the task card tries to draw.

The portable build stores persistent frontend data, logs, tasks, thumbnails, browser profiles, and downloaded output directly under `portable-data/`. The temporary unpack directory is created inside `portable-data/` and removed when the launcher exits.
The portable build also writes per-task reports to `portable-data/tasks/task-<id>/report.json` and thumbnails to `portable-data/thumbnails/task-<id>/thumb.jpg`.

## UI data flow

- The task list is built from live task state plus report snapshots.
- Task details read `report.json` and append recent `task.log` content.
- The selected task log panel appends `task.log` to the in-memory log buffer.
- The frontend persists window placement and current settings on exit.
- The frontend also persists the current task list to the legacy history file on exit, so restart can restore the current tasks without a manual import.
- The browser menu includes a Playwright driver directory picker, and the chosen path is saved with the rest of the frontend state.
- The frontend sets both the window-class icon and the titlebar/taskbar icon from `runtime/app.ico`, while the built exe also carries embedded icon resources.
- Task cards prefer the actual manga title from the task result, and legacy imports keep that title when it exists.
- The lower "browser and configuration status" panel is only refreshed when the content changes, which avoids scroll-time redraw artifacts.
- The URL input uses a cue banner placeholder, not a literal text value.
- The task board refreshes itself when resized so card rendering stays aligned.

## Legacy history import

The Win32 frontend can load an older persisted history snapshot on startup, and it also exposes a manual import action.
Before the import is committed, the frontend shows a preview dialog with the number of new tasks and duplicate tasks that will be skipped.

Lookup order:

1. `COMIC_DOWNLOADER_STATE_PATH`, if the environment variable is set.
2. `D:\tools\crawler_NH\20260410_Final01\runtime\comic_downloader_state.json`, if that file exists.
3. `runtime/comic_downloader_state.json` under the current workspace root for normal builds, or `portable-data/comic_downloader_state.json` for the single-file portable build.

The legacy file is treated as a read-only UI snapshot and is mapped into the current task list.
Importing legacy data appends only non-duplicate entries, using the task URL as the deduplication key.
It preserves the original task order, URL, title, output paths, timestamps, thumbnails, and task state.

## Browser data flow

- The browser layer resolves a launch spec before launch.
- The launch spec includes browser type, browser path, install root, driver dir, temporary profile path, headless, keep-open, locale, timezone, viewport, and user-agent.
- Firefox task runs currently use a fresh temporary Playwright profile per task.
- Chromium support remains in the codebase for internal/probe use, but the public UI no longer exposes Chromium-specific controls or mother-profile pickers.
- The browser session boundary uses Playwright persistent context with a fresh temporary profile directory when the route needs one.
- Browser middleware owns stealth injection, Firefox user prefs, adblock loading, and launch defaults.
- The `task-probe` and `chromium-probe` CLIs remain the current browser smoke-test entry points.
- The `zeri` summary/reader contract is documented in [`docs/zeri_flow_rules.md`](zeri_flow_rules.md).

## Browser mother profiles and temp profiles

- Firefox tasks no longer reuse the saved mother profile directly in the task runner.
- Firefox tasks launch from a fresh temporary profile and clean it when the task ends or is interrupted.
- Chromium probes also launch from a fresh temporary Playwright profile by default, so task and probe runs do not depend on a copied mother profile.
- Task cleanup removes the whole task profile root, including task-scoped temporary profile copies.
- Frontend add-task requests now send only the URL and runtime root; the task layer fills browser defaults from the saved frontend state snapshot when available, and the frontend no longer blocks task creation with a local browser-path precheck.

## Notes

- The browser layer is Firefox-first for the current public UI.
- Task runs still honor `keep-open` so a session can wait until the browser window is closed.
- Task output, thumbnails, logs, history, and browser/runtime state live under `runtime/` for normal builds, and under `portable-data/` for the single-file portable build. The portable build also keeps its temporary unpack directory inside `portable-data/` and cleans it on exit.
- The frontend stores current settings and window placement and restores them on next launch.

## Browser verification pages

When you need to verify the exact profile directory used by a run, use these built-in pages:

- `chrome://version`
  - Best for Chromium and Chromium-based probes.
  - Confirm the `Profile Path` field points at the expected temporary task profile.
- `about:support`
  - Best for Firefox probes.
  - Confirm `Application Basics -> Profile Directory` points at the expected temporary task profile.
- `about:profiles`
  - Best for Firefox if you want to inspect all profiles and the currently active one.

## Current UI notes

- The concurrency control opens a small input dialog instead of cycling preset values.
- The task list supports Explorer-like selection, including `Ctrl` multi-select.
- Task cards expose a right-click menu for retry, details, open download directory, copy task URL, delete, start, and pause.
- The add-task box is cleared as soon as a task is accepted.
- Zeri downloads are saved directly under the configured output root and manga title, without a site-name directory layer in between.

