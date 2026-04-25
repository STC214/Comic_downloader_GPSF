# Comic Downloader

一个基于 Go + Playwright 的 Windows 漫画下载器。当前公开主线是 **Firefox + Zeri**：Win32 前端负责添加和管理任务，任务层使用 Playwright 打开页面、解析 Zeri 阅读流、下载图片并生成缩略图。Chromium 仍保留在代码和探针里，主要用于兼容性验证，不作为当前公共 UI 的主入口。

## 当前状态

- 支持站点：Zeri。
- 暂不支持站点：`myreadingmanga.info`，前端添加任务时会提示 `暂不支持此站点`。
- 浏览器路线：公共 UI 默认走 Firefox；Zeri URL 会被任务层强制归一到 Firefox。
- 下载流程：摘要页 -> 阅读页 -> `100%` -> 懒加载 -> 下载图片 -> 生成 JPG 缩略图。
- 缩略图输入支持 `webp`、`avif` 等常见非 JPEG 格式。
- 任务列表支持虚拟列表、右键菜单、重试、详情、打开下载目录、复制 URL、删除、开始和暂停。
- 便携版是单文件 `dist\portable.exe`，持久数据写入同级 `portable-data\`。

## 快速开始

运行测试：

```powershell
go test ./...
```

运行 Win32 前端：

```powershell
go run -tags playwright ./cmd/win32-frontend
```

运行任务探针：

```powershell
go run -tags playwright ./cmd/task-probe -url "https://www.zerobywzip.com/..." -keep-open=false
```

运行便携版：

```powershell
dist\portable.exe
```

重新构建便携版：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build_portable.ps1
```

## 运行环境

- Go：`go.mod` 声明 `go 1.24.0`。
- 平台：Windows。主前端使用 Win32 API，并带有 `//go:build windows`。
- Playwright：通过 `github.com/playwright-community/playwright-go` 调用。
- 默认 Firefox 可执行文件：`C:\Program Files\Mozilla Firefox\firefox.exe`。
- 默认 Playwright 浏览器根目录：`runtime\playwright-browsers\`。
- 默认 Playwright driver 目录：`runtime\playwright-browsers\driver\`。

本地开发时可以在前端菜单中设置 Firefox 可执行文件和 Playwright driver 目录。任务启动时也会读取已保存的前端状态。

## 常用环境变量

- `COMIC_DOWNLOADER_WORKSPACE_ROOT`：覆盖工作区根目录，便携启动器会把它设为 `portable-data\`。
- `COMIC_DOWNLOADER_RUNTIME_ROOT`：覆盖运行时根目录。
- `COMIC_DOWNLOADER_DOWNLOAD_DIR`：覆盖默认下载目录。
- `COMIC_DOWNLOADER_FRONTEND_STATE_PATH`：覆盖前端设置文件路径。
- `COMIC_DOWNLOADER_STATE_PATH`：覆盖旧版历史/任务状态文件路径。
- `PLAYWRIGHT_BROWSERS_PATH`：Chromium 探针可用的 Playwright 浏览器安装根目录。

## 运行时目录

普通工作区默认写入 `runtime\`：

- `runtime\tasks\task-<id>\report.json`
- `runtime\logs\task-<id>.log`
- `runtime\output\`
- `runtime\thumbnails\task-<id>\thumb.jpg`
- `runtime\browser-profiles\`
- `runtime\frontend_state.json`

便携版默认写入 `portable-data\`：

- `portable-data\tasks\task-<id>\report.json`
- `portable-data\logs\`
- `portable-data\output\`
- `portable-data\thumbnails\`
- `portable-data\browser-profiles\`
- `portable-data\frontend_state.json`
- `portable-data\comic_downloader_state.json`

便携启动器会把内部载荷解包到 `portable-data\payload-*`，退出后清理临时解包目录。

## 浏览器自检页

确认当前浏览器实际使用的 profile：

- Firefox：`about:support`，查看 `Application Basics -> Profile Directory`。
- Firefox：`about:profiles`，查看当前激活 profile 和所有 profile。
- Chromium：`chrome://version`，查看 `Profile Path`。

## 文档索引

- [项目审计](docs/PROJECT_AUDIT.md)
- [手工冒烟测试](docs/SMOKE_TESTS.md)
- [界面与任务流](docs/INTERFACE_FLOW.md)
- [浏览器 Profile 流程](docs/browser_profile_flow.md)
- [Zeri 流程规则](docs/zeri_flow_rules.md)

## 当前审计摘要

- `go test ./...` 已通过。
- README 里的旧入口 `go run ./cmd/comic-downloader` 已移除，当前入口是 `cmd/win32-frontend`。
- 公共 UI、任务层和文档都应以 Firefox + Zeri 为当前主线。
- 仍需后续处理的风险见 [docs/PROJECT_AUDIT.md](docs/PROJECT_AUDIT.md)。
