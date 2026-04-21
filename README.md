# Comic Downloader

这是一个基于 Go + Playwright 的 Windows 漫画下载器。
当前项目的公开主线已经收敛为 **Firefox + Zeri**，Chromium 仅保留在代码和探针里用于兼容性测试，不再作为前端公开入口。

## 当前状态

- 前端“添加任务”会直接创建任务，不再进入等待队列。
- 重复任务会提示确认，重复判断按 `URL + browser type`。
- `myreadingmanga.info` 在前端添加任务时会直接提示“暂不支持此站点”并跳过。
- Zeri 任务使用 Firefox 路线，流程为：摘要页 -> 阅读页 -> `100%` -> 懒加载 -> 下载。
- 下载完成后会生成竖版 JPG 缩略图，并显示在任务卡片上。
- 缩略图输入支持 `webp`、`avif` 等常见的非 JPEG 格式。
- 前端会保存当前设置和窗口位置。
- 便携版是单文件 `portable.exe`，运行时数据保存在 `portable-data/`。
- 任务列表使用虚拟列表，适合几千条任务同时显示。
- 任务卡片优先显示漫画标题，而不是站点名或无头标签。
- 打包时会把图标嵌入 exe、本地窗口标题栏和任务栏图标。

## 运行方式

```powershell
go test ./...
go run ./cmd/comic-downloader
```

便携版：

```powershell
dist\portable.exe
```

## 当前浏览器路径

本机默认使用的 Playwright 浏览器路径：

- `D:\Program\playwright-browsers`

Playwright driver 目录：

- `D:\Program\playwright-browsers\driver`

Firefox 自检页：

- `about:support`
- `about:profiles`

Chromium / Chrome for Testing 自检页：

- `chrome://version`

## 运行时目录

普通工作区运行时目录：

- `runtime/tasks/task-<id>/state.json`
- `runtime/tasks/task-<id>/report.json`
- `runtime/logs/task-<id>.log`
- `runtime/output/task-<id>/`
- `runtime/thumbnails/task-<id>/thumb.jpg`
- `runtime/browser-profiles/`

Each finished task also persists a report file at:

- `runtime/tasks/task-<id>/report.json`

单文件便携版的持久数据目录：

- `portable-data/`

便携版会把状态、日志、历史、任务、缩略图和下载结果都放到 `portable-data/` 下，并且把临时解包目录也放在 `portable-data/` 内，退出时自动清理。
便携版任务报告会写到 `portable-data/tasks/task-<id>/report.json`，缩略图会写到 `portable-data/thumbnails/task-<id>/thumb.jpg`。

## 前端功能

- 顶部菜单支持：
  - 设置 Firefox 可执行文件
  - 设置 Playwright driver 目录
  - 安装浏览器
  - 导入历史记录
  - 保存当前设置和窗口位置
- 下载目录、浏览器目录、历史文件选择都使用 Windows 资源管理器风格的目录选择框。
- 任务卡片支持：
  - 右键菜单
  - 重试
  - 详情
  - 打开下载目录
  - 复制任务 URL
  - 删除
  - 开始
  - 暂停
- 任务列表支持 Explorer 风格选中和 `Ctrl` 多选。
- 前端添加任务后会立即清空 URL 输入框。

## 历史记录导入

前端可导入旧项目 `D:\tools\crawler_NH\20260410_Final01` 生成的历史记录。

导入时会：

- 先预览新增条数和重复条数
- 只追加非重复任务
- 按 URL 去重
- 保留漫画标题、输出目录、状态和缩略图信息

## 浏览器自检页

下面几个页面最适合确认当前浏览器实际使用的 profile：

- `chrome://version`
  - Chromium / Chrome for Testing
  - 看 `Profile Path`
- `about:support`
  - Firefox
  - 看 `Application Basics -> Profile Directory`
- `about:profiles`
  - Firefox
  - 看当前激活的 profile 和所有 profile 列表

## 说明

- 当前公共 UI 以 Firefox 路线为主。
- Chromium 仍保留在代码中，主要用于内部探针和后续兼容性工作。
- 便携版的持久状态现在以 `portable-data/` 为根目录，不再依赖临时解包目录。
