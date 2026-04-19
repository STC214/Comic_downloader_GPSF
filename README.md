# Comic Downloader

基于 Go 的 Windows 漫画下载器。

本仓库当前以 Go 重构后的应用栈作为主入口，浏览器、任务、运行时相关代码分别位于 `browser/`、`tasks/`、`runtime/`、`siteflow/` 和 `ui/`。

当前可用的浏览器栈是 Go + Playwright + Firefox 中间件 + `init script` 注入式 stealth。

## 当前约定

- 前端提交 `url` 和 `downloadRoot`。
- 点击 `添加任务` 会立即启动任务；任务列表更像运行历史，而不是等待队列。
- 浏览器任务层会先规范化请求，解析 Firefox 子配置目录，把它复制到临时 Playwright profile，再从这个临时目录启动浏览器。
- 浏览器层统一负责 `headless`、`keepOpen`、`locale`、`timezone`、`viewport`、`User-Agent`、Firefox user prefs 和 stealth 注入。
- `task-probe` 是当前用于测试浏览器流程的主要 CLI。
- 任务状态、报告和日志会写入 `runtime/`。
- UI 可以读取 `runtime/tasks/task-<id>/report.json` 和 `runtime/logs/task-<id>.log`。

## 入口

- 主应用：`go run .`
- 备用 CLI：`go run ./cmd/comic-downloader`

## 快速开始

```powershell
go test ./...
go run ./cmd/comic-downloader --workspace-root .
```

## 运行时目录

- `runtime/browser-profiles/baseline-userdata` 是项目选定的 Firefox 子配置目录。
- 系统里的 Firefox 母配置会通过手动按钮或菜单复制到这份子配置里。
- `runtime/browser-profiles/tasks/` 存放任务级和临时 Playwright profile 副本。
- `runtime/browser-profiles/tasks/firefox/<worker>/task-<id>/original-userdata` 存放任务级母配置副本。
- `runtime/browser-profiles/tasks/firefox-playwright-*` 存放直接从选定母配置启动 Firefox 时生成的临时 Playwright profile 副本。
- `runtime/output/` 存放任务输出。
- `runtime/thumbnails/` 存放任务缩略图。
- `runtime/tasks/task-<id>/state.json` 存放任务状态。
- `runtime/tasks/task-<id>/report.json` 存放规范化后的任务报告。
- `runtime/logs/task-<id>.log` 存放可读的任务日志。

## 说明

- Firefox 默认路径是 `C:\Program Files\Mozilla Firefox\firefox.exe`。
- 当前选定的 Firefox 母配置默认是 `C:\Users\stc52\AppData\Roaming\Mozilla\Firefox\Profiles\aocfvl86.default-default-3`。
- 默认 Firefox User-Agent 是 `Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:149.0) Gecko/20100101 Firefox/149.0`。
- 浏览器中间件会先把选定子配置复制到临时 Playwright profile，再用这个临时 profile 启动 Firefox，运行结束后删除临时目录。
- 浏览器中间件会在任何页面打开前注入 `runtime/firefox_stealth.js`。
- 浏览器中间件还负责 adblock 标志、语言、时区、视口和 Firefox user prefs 的配置。
- 浏览器 profile 隔离和 Firefox 母配置流程见 [`docs/browser_profile_flow.md`](docs/browser_profile_flow.md)。
- 当前浏览器接口约定见 [`docs/INTERFACE_FLOW.md`](docs/INTERFACE_FLOW.md)。
- 当前 `zeri` 的 summary/reader 规则见 [`docs/zeri_flow_rules.md`](docs/zeri_flow_rules.md)。

## 浏览器自检页

下面三个页面最适合用来确认浏览器当前正在使用的配置文件路径：

- `chrome://version`
  - 适合 Chromium / Chrome for Testing。
  - 打开后查看 `Profile Path`。
- `about:support`
  - 适合 Firefox。
  - 打开后查看 `Application Basics -> Profile Directory`。
- `about:profiles`
  - 适合 Firefox。
  - 可以查看所有 profile，并确认当前正在使用的那个 profile。
