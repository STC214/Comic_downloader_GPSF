# Project Audit

Audit date: 2026-04-25

## Scope

This audit reviewed the repository structure, Go package boundaries, current documentation, build and run entry points, runtime data layout, and the test suite. It did not perform live site downloads or browser smoke tests against remote pages.

## Verification

Command run:

```powershell
go test ./...
```

Result: pass.

Covered packages include:

- `browser`
- `runtime`
- `siteflow/zeri`
- `tasks`
- `ui`

Several command packages have no test files, which is expected for the current probe and frontend binaries.

## Architecture Snapshot

- `cmd/win32-frontend`: Windows desktop UI entry point.
- `cmd/portable-launcher`: single-file launcher that extracts the packaged frontend and points persistent data at `portable-data\`.
- `cmd/task-probe`: Playwright-backed task smoke-test entry point, built with `-tags playwright`.
- `browser`: Playwright session middleware for Firefox.
- `runtime`: runtime path, browser profile, frontend state, logging, and install helpers.
- `tasks`: task-level browser launch normalization and result reporting.
- `siteflow/zeri`: Zeri page parsing, reader flow, image download, and thumbnail helpers.
- `ui`: task list/report/front-end state helpers used by the Win32 frontend.

## Findings

### High Priority

- README had an obsolete run command: `go run ./cmd/comic-downloader`. The repository currently has no `cmd/comic-downloader`; the actual frontend entry point is `go run -tags playwright ./cmd/win32-frontend`. This has been corrected in README.
- The previous local absolute `replace` directives in `go.mod` have been removed so module resolution can work on another machine.

### Medium Priority

- Runtime and distribution artifacts were trimmed. The intended release surface is the single-file executable plus its required portable data folder.
- The duplicate `backup_check\` snapshot was removed.
- Machine-specific defaults were reduced. Firefox profile source discovery now relies on environment or standard Firefox profile discovery.

### Low Priority

- Some terminals may render UTF-8 Chinese documentation or progress text as mojibake when they use the legacy Windows code page. The files themselves read correctly with UTF-8, for example `Get-Content README.md -Encoding UTF8`.
- Documentation mixes Chinese and English. That is workable, but each document should keep a consistent language and point to the same run commands.
- `go test ./...` does not exercise real browser launch or remote download flows by default. Keep a separate manual smoke-test checklist for Playwright, browser profile, and Zeri download behavior.

## Current Operational Contract

- Public UI: Firefox-first.
- Current supported downloader route: Zeri.
- Zeri task flow: summary page -> reader page -> `100%` -> lazy-load images -> download -> thumbnail.
- Normal runtime root: `runtime\`.
- Portable runtime root: `portable-data\`.
- Per-task report: `tasks\task-<id>\report.json` under the active runtime root.
- Per-task thumbnail: `thumbnails\task-<id>\thumb.jpg` under the active runtime root.

## Recommended Next Steps

1. Remove or gate local absolute `replace` directives in `go.mod` before sharing the project across machines.
2. Decide whether `dist\`, `backup_check\`, `portable-run.log`, and `temp.txt` should remain versioned.
3. Add a small assertion around user-visible progress phases if progress text becomes part of a stricter UI contract.
4. Run the documented browser smoke tests in `docs/SMOKE_TESTS.md` before publishing a portable build.
5. Keep README, `docs/INTERFACE_FLOW.md`, and `docs/browser_profile_flow.md` aligned whenever task/runtime paths change.
