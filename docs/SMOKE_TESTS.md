# Manual Smoke Tests

These checks cover behavior that `go test ./...` does not exercise, especially Playwright, installed browsers, real profile paths, and remote page behavior.

## Before Starting

Run the unit test suite first:

```powershell
go test ./...
```

Confirm the configured browser paths:

- Firefox executable: set from the UI menu or pass `-browser-path` to probe commands.
- Playwright driver directory: set from the UI menu or pass `-driver-dir` to probe commands.
- Runtime root: defaults to `runtime\`, or use `COMIC_DOWNLOADER_RUNTIME_ROOT`.

## Firefox Probe

Open a simple page and keep the browser visible:

```powershell
go run -tags playwright ./cmd/task-probe `
  -url "https://example.com" `
  -browser-type firefox `
  -headless=false `
  -keep-open=true
```

Expected result:

- A Firefox window opens.
- The command prints the page title and the temporary Playwright profile path.
- `about:support` shows `Application Basics -> Profile Directory` pointing at the printed temp profile.
- Closing the browser lets the command exit.

## Zeri Download Probe

Run a known-good Zeri URL with an isolated output directory:

```powershell
go run -tags playwright ./cmd/task-probe `
  -url "https://www.zerobywzip.com/..." `
  -browser-type firefox `
  -headless=false `
  -download-root ".\runtime\smoke-output" `
  -output-dir ".\runtime\smoke-output"
```

Expected result:

- The task resolves the summary page and reader page.
- Downloaded images appear under the configured output root.
- A task report is written under `runtime\tasks\`.
- A thumbnail is written under `runtime\thumbnails\` when images were downloaded.

## Win32 Frontend

Start the desktop app:

```powershell
go run -tags playwright ./cmd/win32-frontend
```

Expected result:

- The window opens with the saved settings restored.
- Adding a Zeri URL creates a task immediately.
- Duplicate URL + browser type prompts before adding.
- Right-clicking a task shows retry, details, open directory, copy URL, delete, start, and pause actions.
- Closing and reopening restores window placement and current settings.

## Portable Build

Build and run the portable launcher:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build_portable.ps1
dist\portable.exe
```

Expected result:

- `dist\portable.exe` starts the frontend.
- Persistent data is written under `dist\portable-data\`.
- Temporary `payload-*` directories are cleaned after the launcher exits.
- Logs are written under `dist\portable-data\logs\`.
