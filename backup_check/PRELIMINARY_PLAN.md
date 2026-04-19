# 前期落地说明

本文档是基于 [REFACTOR_BLUEPRINT.md](/f:/Project/comic_downloader_GO_Playwright_stealth/REFACTOR_BLUEPRINT.md) 的前期执行说明，目标是先把 Go + Playwright + stealth 这条路线的边界定清楚，再进入具体实现。

## 1. 前期目标

这一阶段不追求完整重构，只先确认三件事：

1. Go 侧 Playwright 的最小可运行骨架
2. 浏览器上下文、任务隔离和状态持久化的实现方式
3. stealth 注入、Chromium 路径、userdata 路径的配置约定
4. 任务路由入口和站点 worker 的最小联通

## 2. 路线前提

实现时主要参考以下内容：

- Playwright 官方文档，重点看 `BrowserContext`、`Storage State`、`addInitScript`
- `playwright-community/playwright-go` 仓库文档
- 项目现有的 refactor blueprint

### 2.1 关键结论

- `BrowserContext` 本身就是隔离单元，适合“每个任务一个上下文”的设计。
- `storageState` 适合做登录态、验证态和 cookie 的导入导出。
- `addInitScript` 适合在页面脚本执行前注入 stealth。
- Go 版 Playwright 与驱动版本需要匹配，安装和升级都要显式管理。

## 3. 目录与职责

建议继续沿用现有目录边界：

- `browser/`：Playwright 封装、上下文、Chromium / Firefox 启动、userdata 复制、stealth 注入
- `sites/`：各站点解析和下载逻辑
- `tasks/`：任务调度、状态机、清理
- `runtime/`：运行时目录、临时文件、日志、输出
- `ui/`：尽量保持现状

### 3.1 浏览器层职责

浏览器层应统一负责：

- 启动和关闭 Playwright
- 启动和关闭 Chromium
- 启动和关闭 Firefox
- 创建普通上下文
- 创建验证上下文
- 注入 stealth
- 导入和导出 storageState
- 管理任务结束后的清理

## 4. Chromium / Firefox 与 userdata 约定

程序需要支持两类输入：

- 可自定义的 Chromium 路径
- 可自定义的 Firefox 路径
- 可自定义的 userdata 路径

默认行为应为：

- 使用项目自带的 Chromium
- 使用系统默认 browser userdata 路径
- Firefox 任务使用系统安装的 Firefox 和系统默认 Firefox profile 路径

在 Windows 上，系统默认 userdata 的解析会优先检查 `LOCALAPPDATA` 下的 Chrome、Edge 和 Chromium 常见目录。
Firefox userdata 则会从 `APPDATA\Mozilla\Firefox\Profiles` 里寻找 `*.default-release`、`*.default` 或其他可用 profile。
如果这些目录暂时不存在，程序会先创建首选系统默认目录，再把它作为原始 userdata 的基线目录。

### 4.1 原始 userdata 的定义

无论哪种组合，系统都要有一个“原始 userdata”概念：

- 如果是“项目自带 Chromium + 系统默认 userdata”，那系统默认 userdata 就是原始 userdata
- 如果是“自定义 Chromium + 自定义 userdata”，那这个自定义 userdata 就是原始 userdata
- 如果是 Firefox 任务，那本机 Firefox profile 就是原始 userdata，后续会被复制到任务级临时目录

这个原始 userdata 需要承担验证托底职责。

### 4.2 验证流程

验证期间应使用原始 userdata 作为回退基线：

1. 先把原始 userdata 复制到验证用上下文
2. 验证完成后，把验证成功后的原始 userdata 状态保留下来
3. 再把这份“验证后的原始 userdata”复制给每个任务条目使用

### 4.3 推荐目录结构

建议在 runtime 下组织成类似结构：

- `runtime/browser-profiles/baseline-userdata`
- `runtime/browser-profiles/<worker>/task-<id>/original-userdata`
- `runtime/browser-profiles/<worker>/task-<id>/content`
- `runtime/browser-profiles/<worker>/task-<id>/verify`

其中：

- `original-userdata` 是本机 userdata 的任务级母本副本
- `baseline-userdata` 是原始 userdata 的本地工作副本
- `content` 是任务实际使用的 profile
- `verify` 是 headed 验证时使用的 profile

### 4.4 启动参数

程序入口建议支持以下参数：

- `workspace-root`：工作区根目录
- `chromium-path`：自定义 Chromium 可执行文件路径
- `firefox-path`：自定义 Firefox 可执行文件路径
- `userdata-path`：自定义 userdata 路径
- `download-root`：可选的任务输出目录，未传时默认使用 runtime `output`

启动后应先创建 runtime 目录，再根据是否传入 `userdata-path` 决定是否把原始 userdata 复制到任务级母本目录并继续派生 baseline。

## 5. 实现顺序

### 第一步：Playwright 骨架

- 固定 `playwright-go` 版本
- 打通 `playwright.Run()`、`browser.Launch()`、`context.NewPage()` 的最小链路
- 确认 Windows 环境可以启动

### 第二步：配置与目录封装

- 封装 Chromium 路径配置
- 封装 userdata 路径配置
- 封装 baseline / task / verify 目录布局
- 封装任务结束后的清理

### 第三步：stealth 接入

- 先在测试页验证 `addInitScript` 生效
- 再接入项目的 stealth 逻辑
- 仅在需要的站点启用

### 第四步：启动规格草案

- 在 browser 层先计算 launch spec
- launch spec 包含 Chromium 路径、userdata 目录、验证目录、headless 和 stealth 标志
- 对于需要验证的任务，再生成 content / verification 两份 launch spec
- launch spec 要能在启动前先做参数自检
- browser session 要能导入 / 导出 storageState，并在会话边界注入 stealth init script
- 先把参数定好，再接入真正的 Playwright 启动器

### 第五步：任务 profile 准备

- 启动时先准备 `baseline-userdata`
- 再把 `baseline-userdata` 复制到每个任务的 `content` 和 `verify`
- 验证完成后，让任务继续使用这份已验证的任务副本
- 验证状态机要能表达 `waiting`、`cleared`、`committed`
- 任务生命周期要能表达 `queued`、`preparing`、`running`、`waiting_verification`、`completed`、`failed`
- 任务元数据应该落盘到 `runtime/tasks/task-<id>/state.json`
- 任务报告应该落盘到 `runtime/tasks/task-<id>/report.json`
- 任务日志应该落盘到 `runtime/logs/task-<id>.log`
- 落盘后的任务状态应该能重新还原成内存里的 runtime
- 元数据里应该记录 created / started / finished / last error 这几项
- 验证完成后要能回灌 baseline 并继续原任务
- 验证型任务启动后应停在 waiting 状态，等用户完成恢复

### 第六步：站点迁移准备

- 先处理 `zeri`
- 再处理 `nyahentai`
- 再处理 `hentai2`
- 最后处理 `myreadingmanga`

### 第七步：任务路由联通

- 入口支持可选 `url`
- 任务管理器根据 URL 选择站点 worker
- 路由前先规范化 URL，减少 fragment / 大小写噪音
- 路由结果决定 `headless`、`verify`、`browser type` 和任务执行路径
- 具体站点 worker 由应用组装层注入，任务管理器只保留抽象接口
- 任务管理器还能把路由结果和 browser session 合并成一个 task runtime
- HTTP-only 任务不进入 browser launcher；browser-backed HTTP 任务仍然会使用浏览器中间件
- 任务结束后清理 bootstrap/task profile 目录
- task runtime 要能暴露 session snapshot，方便跟踪 storageState 和 init script 路径
- `zeri` 至少应先提供 Firefox 页面采集、文章页到 reader 页、reader 页到图片列表的解析函数
- `zeri` 还应提供基于 manifest 的任务级下载计划生成函数
- `zeri` 还应提供基于任务级下载计划的 HTTP 下载执行器
- `myreadingmanga` 至少应先提供章节页 reader URL、reader 页图片列表和验证门检测函数
- `nyahentai` 至少应先提供 reader 页 comic ID 过滤和图片清单生成函数
- `hentai2` 至少应先提供 summary 页、reader 页、页数和图片清单生成函数
- `sites` 层应提供一个统一的 manifest 摘要接口，方便任务层记录同一种状态结构

## 6. 关键约束

### 6.1 不要一开始就做全局持久化

前期更适合 task-scoped 的目录和状态文件，而不是把所有浏览器状态堆到一个共享目录里。

### 6.2 不要把 stealth 当成万能解法

stealth 只是降低识别概率，不应该替代站点自身的解析逻辑、状态机和验证流程。

### 6.3 不要把验证态和内容态混在一起

`myreadingmanga` 这类站点必须把“验证上下文”和“内容上下文”拆开，验证完成后再同步状态。

## 7. 验收标准

前期文档对应的最小验收标准如下：

- Go 程序能成功启动 Playwright、Chromium 和 Firefox
- 能配置 Chromium 路径
- 能配置 Firefox 路径
- 能配置 userdata 路径
- 能区分原始 userdata、任务 userdata、验证 userdata
- 能创建独立 browser context
- 能导入和导出 storageState
- 能在页面注入 init script
- 能完成一次任务级上下文的创建、使用和销毁

## 8. 风险点

- Go 版 Playwright 与浏览器驱动版本不匹配会导致安装或运行失败
- stealth 注入位置不对会导致页面脚本先执行，效果失效
- 任务状态如果不和 context 生命周期绑定，容易残留 profile
- 验证流程如果没有独立上下文，后续恢复会不稳定

## 9. 小结

这份前期说明的目标不是替代蓝图，而是把最容易反复改口的部分先定下来：

- Chromium 可以自定义
- userdata 可以自定义
- 原始 userdata 作为验证托底
- 验证完成后把状态复制给每个任务条目

这四条一旦固定，后续站点迁移和浏览器封装就能按同一套语义推进。

