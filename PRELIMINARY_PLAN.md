# 前期落地说明

本文档记录当前已经落地的浏览器中间件路线，主要是 Go + Playwright + Firefox + stealth + 母配置复制流程。

## 1. 前期目标

当前这一阶段已经完成的重点是：

1. Go 侧 Playwright 的最小可运行骨架
2. Firefox 中间件、任务隔离和状态持久化的实现方式
3. stealth 注入、Firefox 路径、母配置路径的配置约定
4. 任务路由入口和站点 worker 的最小联通

## 2. 路线前提

实现时主要参考以下内容：

- Playwright 官方文档，重点看 `BrowserContext`、`Storage State`、`addInitScript`、`launchPersistentContext`
- `playwright-community/playwright-go` 仓库文档
- 项目现有的 refactor blueprint

### 2.1 关键结论

- `BrowserContext` 本身就是隔离单元，适合“每个任务一个上下文”的设计。
- `storageState` 适合做登录态、验证态和 cookie 的导入导出。
- `addInitScript` 适合在页面脚本执行前注入 stealth。
- Go 版 Playwright 与驱动版本需要匹配，安装和升级都要显式管理。
- Firefox 这条线现在是通过 `launchPersistentContext` + 临时 profile 复制来启动。

## 3. 目录与职责

建议继续沿用现有目录边界：

- `browser/`：Playwright 封装、Firefox 启动、母配置复制、stealth 注入
- `sites/`：各站点解析和下载逻辑
- `tasks/`：任务调度、状态机、清理
- `runtime/`：运行时目录、临时文件、日志、输出
- `ui/`：尽量保持现状

### 3.1 浏览器层职责

浏览器层应统一负责：

- 启动和关闭 Playwright
- 启动和关闭 Firefox
- 创建普通上下文
- 创建验证上下文
- 注入 stealth
- 导入和导出 storageState
- 管理任务结束后的清理

## 4. Firefox 与 profile 约定

程序当前支持三类输入：

- 可自定义的 Firefox 可执行文件路径
- 可自定义的母配置目录
- 可自定义的临时 Playwright profile 来源目录

默认行为应为：

- 使用 `C:\Program Files\Mozilla Firefox\firefox.exe`
- 使用 `C:\Users\stc52\AppData\Roaming\Mozilla\Firefox\Profiles\jo2klram.default-release`
- 启动前先把母配置完整复制到临时 Playwright profile

### 4.1 原始 userdata 的定义

无论哪种运行模式，系统都要有一个“母配置”概念：

- 母配置目录是稳定存在的源目录
- 启动前会把母配置复制到临时 Playwright profile
- Playwright 永远从临时目录起，不直接碰母配置

### 4.2 验证流程

验证期间应使用复制出来的临时 profile 作为运行基线：

1. 先把母配置复制到临时 profile
2. 浏览器从临时 profile 启动
3. 会话结束后删除临时 profile

### 4.3 推荐目录结构

建议在 runtime 下组织成类似结构：

- `runtime/browser-profiles/firefox`
- `runtime/browser-profiles/tasks/firefox/<worker>/task-<id>/original-userdata`
- `runtime/browser-profiles/tasks/firefox/<worker>/task-<id>/content`
- `runtime/browser-profiles/tasks/firefox/<worker>/task-<id>/verify`
- `runtime/browser-profiles/tasks/firefox-playwright-*`

其中：

- `firefox` 是项目选择的母配置目录
- `original-userdata` 是任务级母本副本
- `content` 是任务实际使用的 profile
- `verify` 是 headed 验证时使用的 profile
- `firefox-playwright-*` 是直接 probe 时的临时 Playwright profile

### 4.4 启动参数

程序入口当前支持以下参数：

- `workspace-root`：工作区根目录
- `browser-path`：自定义 Firefox 可执行文件路径
- `profile-dir`：自定义母配置目录
- `user-data-dir`：自定义临时 profile 来源目录
- `user-agent`：浏览器 UA
- `locale`：浏览器语言
- `timezone-id`：浏览器时区
- `viewport-width` / `viewport-height`：浏览器视口
- `firefox-user-prefs-json`：Firefox user prefs
- `headless`：无头模式开关
- `keep-open`：是否等待手动关闭窗口

启动后应先把母配置复制到临时 Playwright profile，再把该临时 profile 交给 Firefox persistent context。

## 5. 实现顺序

### 第一步：Playwright 骨架

- 固定 `playwright-go` 版本
- 打通 `playwright.Run()`、`LaunchPersistentContext()`、`context.NewPage()` 的最小链路
- 确认 Windows 环境可以启动系统 Firefox

### 第二步：配置与目录封装

- 封装 Firefox 路径配置
- 封装母配置 / 临时 profile 路径配置
- 封装 task / verify 目录布局
- 封装任务结束后的清理

### 第三步：stealth 接入

- 先在测试页验证 `addInitScript` 生效
- 再接入项目的 stealth 逻辑
- 仅在 Firefox 中间件里统一启用

### 第四步：启动规格草案

- 在 browser 层先计算 launch spec
- launch spec 包含 Firefox 路径、临时 profile 目录、headless、UA、locale、timezone、viewport、Firefox prefs 和 stealth 标志
- launch spec 要能在启动前先做参数自检
- browser session 要在 persistent context 上启动
- 先把参数定好，再接入真正的 Playwright 启动器

### 第五步：任务 profile 准备

- 启动时先准备任务级 profile 副本
- 再把任务级母本复制到 `content` 和 `verify`
- 任务生命周期要能表达 `queued`、`preparing`、`running`、`waiting_verification`、`completed`、`failed`
- 任务元数据应该落盘到 `runtime/tasks/task-<id>/state.json`
- 任务报告应该落盘到 `runtime/tasks/task-<id>/report.json`
- 任务日志应该落盘到 `runtime/logs/task-<id>.log`
- 落盘后的任务状态应该能重新还原成内存里的 runtime
- 元数据里应该记录 created / started / finished / last error 这几项
- 任务结束后清理对应的 task profile 目录

### 第六步：站点迁移准备

- 先处理 `zeri`
- 再处理 `nyahentai`
- 再处理 `hentai2`
- 最后处理 `myreadingmanga`

### 第七步：任务路由联通

- 入口支持可选 `url`
- 任务管理器根据 URL 选择站点 worker
- 路由前先规范化 URL，减少 fragment / 大小写噪音
- 路由结果决定 `headless`、`keepOpen`、`browser type` 和任务执行路径
- 具体站点 worker 由应用组装层注入，任务管理器只保留抽象接口
- 任务管理器还能把路由结果和 browser session 合并成一个 task runtime
- HTTP-only 任务不进入 browser launcher；browser-backed HTTP 任务仍然会使用浏览器中间件
- 任务结束后清理对应的 profile 目录
- `zeri` 至少应先提供 Firefox 页面采集、文章页到 reader 页、reader 页到图片列表的解析函数
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

- Go 程序能成功启动 Playwright 和 Firefox
- 能配置 Firefox 路径
- 能配置母配置路径
- 能复制母配置到临时 Playwright profile
- 能创建独立 browser context
- 能在页面注入 init script
- 能完成一次任务级上下文的创建、使用和销毁

## 8. 风险点

- Go 版 Playwright 与浏览器驱动版本不匹配会导致安装或运行失败
- stealth 注入位置不对会导致页面脚本先执行，效果失效
- 任务状态如果不和 context 生命周期绑定，容易残留 profile
- 临时 profile 如果不是绝对路径，Playwright 会拒绝启动

## 9. 小结

这份前期说明现在更像一份“已落地约定”：

- Firefox 路径已固定
- 母配置路径已固定
- 临时 Playwright profile 复制流程已固定
- 任务结束清理流程已固定

后续如果要扩展其他浏览器，可以在这个约定之上再加，不需要推翻当前 Firefox 流程。

