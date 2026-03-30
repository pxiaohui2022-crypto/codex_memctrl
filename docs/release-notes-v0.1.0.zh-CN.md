# memctl v0.1.0

`memctl` 的首个公开版本。

`memctl` 是一个面向 Codex 风格 agent 工作流的本地优先记忆侧车。它把长期记忆从单一 API 服务商中剥离出来，让用户在切换服务商时，仍然可以保留偏好、项目约束、历史决策和跨会话待办事项。

## 版本亮点

- 使用 SQLite 作为默认运行时存储，支持持久化本地记忆
- 支持 JSON 导入导出，并能自动迁移旧版 `memories.json`
- 支持按作用域进行检索和上下文打包
- 提供 `memctl codex`，可在启动 Codex 时自动注入长期记忆
- 提供候选记忆审核流程，支持批量 `--accept-all` 和 `--archive-all`
- 支持从普通文本和 Codex `history.jsonl` 中启发式提取候选记忆
- 提供 `memctl status`，快速检查 store、scope 和 Codex history 状态

## 本版本包含的命令

- `memctl init`
- `memctl add`
- `memctl search`
- `memctl status`
- `memctl extract`
- `memctl review`
- `memctl pack`
- `memctl export`
- `memctl import`
- `memctl codex`
- `memctl run`
- `memctl version`

## 安装方式

从本次 Release 的附件中下载对应平台的压缩包，解压后将 `memctl` 放到系统 `PATH` 中即可。

当前提供的构建目标：

- Linux `amd64`
- Linux `arm64`
- macOS `amd64`
- macOS `arm64`
- Windows `amd64`
- Windows `arm64`

## 说明

- 默认运行时存储为 SQLite
- 如果检测到旧版 JSON store，会在首次打开时自动导入
- 对同一个 Codex session 重复执行提取，不会重复生成相同的候选记忆

## 当前已知边界

- 记忆提取目前仍然是启发式方案，偏保守
- `v0.1.0` 暂不包含 MCP server
- 除 Codex 外的 provider 适配仍在后续计划中
