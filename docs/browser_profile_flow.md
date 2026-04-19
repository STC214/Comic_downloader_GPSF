# Browser Profile Flow

This repository currently uses a Firefox-first browser middleware.
The browser layer starts from the selected project-owned Firefox working profile, copies it into a temporary Playwright profile, and launches Firefox from that temporary copy.

## Current launch flow

1. Resolve the selected Firefox working profile directory.
2. Copy that entire directory into a fresh temporary Playwright profile under `runtime/browser-profiles/tasks/`.
3. Launch Firefox with `playwright-go` using the copied temp directory as `userDataDir`.
4. Inject `runtime/firefox_stealth.js` before any page runs.
5. Open the target URL.
6. Wait until the page or window closes when `keep-open` is enabled.
7. Remove the temporary Playwright profile after the session ends.

## Current defaults

- Browser executable: `C:\Program Files\Mozilla Firefox\firefox.exe`
- Selected mother profile source for refresh: `C:\Users\stc52\AppData\Roaming\Mozilla\Firefox\Profiles\jo2klram.default-release`
- Selected working profile used by tasks: `runtime/browser-profiles/baseline-userdata`
- Default User-Agent: `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:149.0) Gecko/20100101 Firefox/149.0`
- Default locale: `en-US`
- Default timezone: `Asia/Shanghai`
- Default viewport: `1365x768`

## Direct browser entry

The browser middleware exposes the following request-time overrides:

- `profile-dir`
- `user-data-dir`
- `user-agent`
- `locale`
- `timezone-id`
- `viewport-width`
- `viewport-height`
- `firefox-user-prefs-json`
- `headless`
- `keep-open`

## Task-scoped profile directories

When a task needs its own copied profile, the runtime creates:

- `runtime/browser-profiles/tasks/firefox/<worker>/task-<id>/original-userdata`
- `runtime/browser-profiles/tasks/firefox/<worker>/task-<id>/content`
- `runtime/browser-profiles/tasks/firefox/<worker>/task-<id>/verify`

When a direct browser probe runs without task ownership, the runtime creates a temporary Playwright profile directory under:

- `runtime/browser-profiles/tasks/firefox-playwright-*`

Both styles are disposable and are removed after the run finishes.

## Why this matters

The goal is to keep the browser run reproducible while still starting from a real local Firefox profile:

- The selected working profile is copied, not modified in place.
- The browser always launches from a temporary directory.
- Temporary directories are cleaned up at the end of the run.
- The browser middleware is the single place that owns stealth injection and launch defaults.
