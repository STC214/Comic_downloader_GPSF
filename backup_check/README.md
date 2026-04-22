# Comic Downloader

这是仓库当前状态的备份说明，内容与主 README 保持一致。

## 当前状态

- 主线为 Firefox + Zeri。
- Chromium 仅作为内部探针和兼容性实现保留。
- `myreadingmanga.info` 在前端添加任务时会被直接拦截并提示“暂不支持此站点”。
- 便携版是单文件 `portable.exe`，持久数据保存在 `portable-data/`。
- 任务卡片显示漫画标题、缩略图、进度条和实时状态。
- 缩略图支持 `webp`、`avif` 等常见的非 JPEG 格式输入。

## 运行方式

```powershell
go test ./...
go run ./cmd/comic-downloader
```

便携版：

```powershell
dist\portable.exe
```

## 当前运行目录

- 普通工作区：`runtime/`
- 单文件便携版：`portable-data/`

便携版会把状态、日志、历史、任务、缩略图和下载结果都落到 `portable-data/` 下，并在退出时清理临时解包目录。

## 浏览器自检页

- `chrome://version`
- `about:support`
- `about:profiles`

## 说明

- 当前公共 UI 以 Firefox 路线为主。
- Chromium 仍在代码中保留，用于内部探针与兼容性测试。
## Progress refresh interval

- The Win32 frontend exposes a `Set progress refresh interval...` menu item.
- The value is stored in the frontend state and restored on next launch.
- It controls how often fast task progress updates are coalesced before the task board redraws.
- Default: `80ms`.
