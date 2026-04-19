# Browser Profile Flow

This repository uses two browser entry paths:

- Chromium: bundled Chromium under `runtime/chromium/chrome.exe`
- Firefox: system-installed Firefox discovered from the local machine

Both paths share the same task isolation model:

1. Resolve the machine-local browser userdata path.
2. Copy that source userdata into a task-scoped `original-userdata` directory.
3. Copy `original-userdata` into a task-scoped `baseline-userdata` directory.
4. Copy `baseline-userdata` into task-scoped `content` and `verify` directories.
5. Launch the browser against the task-scoped profile directory.
6. Remove the whole task profile root when the task finishes.

## Site-to-browser mapping

- `zeri` uses the Firefox browser path and then performs its final image transfer through Go-side HTTP.
- `myreadingmanga`, `nyahentai`, and `hentai2` use Chromium by default.
- The browser middleware injects the appropriate stealth script for the active browser type before pages are opened.

## Chromium entry

- Browser executable:
  - Default: `runtime/chromium/chrome.exe`
  - Override: UI/CLI `--chromium-path`
- Source userdata:
  - Auto-resolved from the local Windows machine when `UserDataPath` is not provided
  - Default lookup first honors the Windows default browser association, then falls back to Chromium/Chrome/Edge-style profile paths
- Stealth:
  - Default script: `runtime/chrome_stealth.js`
  - Injected by the browser middleware before any page is opened

## Firefox entry

- Browser executable:
  - Auto-resolved from the local Windows install
  - Common path candidates include `C:\Program Files\Mozilla Firefox\firefox.exe`
- Source userdata:
  - Auto-resolved from `APPDATA\Mozilla\Firefox\Profiles`
  - The resolver prefers `profiles.ini` default profile entries first, then `*.default-release`, then `*.default`, then any existing profile
- Stealth:
  - Default script: `runtime/firefox_stealth.js`
  - Injected by the browser middleware before any page is opened
- First-run suppression:
  - A task-scoped `user.js` is written into the Firefox profile copy to suppress the welcome/onboarding screen

## Task isolation directories

For a task `task-123`, the browser layer creates:

- `runtime/browser-profiles/<worker>/task-123/original-userdata`
- `runtime/browser-profiles/<worker>/task-123/content`
- `runtime/browser-profiles/<worker>/task-123/verify`
- `runtime/browser-profiles/baseline-userdata`

The `original-userdata` directory is always task-scoped and disposable.
The task cleanup path removes the whole `task-123` profile root.

## Why this matters

The goal is to keep the browser runtime reproducible while still letting the task start from a real local browser profile:

- Chromium tasks start from the machine's default Chromium/Chrome-like userdata.
- Firefox tasks start from the machine's default Firefox profile.
- Both are copied into the repository runtime tree before task execution.
- That makes per-task isolation easy to clean up and easy to verify.
