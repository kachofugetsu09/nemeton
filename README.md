# Nemeton

Nemeton 是一套面向 Agent 原生软件项目的本地语义控制平面。

它按顺序记录项目现实、多人讨论、语义决定、任务合同、worktree 执行、硬性验收和失败反馈。人类在会议中确认产品要解决的问题和风险边界，Agent 在已确认的语义与合同下完成实现。产品 Current State、会议结果和后续任务视图都由事件历史重建；仓库的 `docs/NOW.md` 是 Babel 工作手册。

当前仓库已经完成 Milestone 0 和 Milestone 1：dirty Reality Bundle、Codex/OpenCode Runner、隔离席位 Workdir、动态 Swarm、单 Recorder、多 Cycle 用户审阅、Approved Context、Coding Design Handoff、数据库 Current State、HTTP/WebSocket 和 Local Web UI。任务执行和产品 Gate 引擎仍是后续里程碑。

```text
Project Reality
      │
      ▼
Swarm Meeting ──► Human Decision ──► Semantic Revision
                                            │
                                            ▼
                                      Task Contracts
                                            │
                              ┌─────────────┴─────────────┐
                              ▼                           ▼
                         worktree A                  worktree B
                              │                           │
                              └─────────────┬─────────────┘
                                            ▼
                                  Gate + Evidence + Merge
                                            │
                                            ▼
                                    Trust State / Next Round
```

## 阅读顺序

1. [文档索引](docs/INDEX.md)
2. [完整产品语义](docs/projectneed.md)
3. [当前实施状态](docs/NOW.md)
4. [开发工作流](docs/WORKFLOW.md)
5. [v1 详细设计](docs/spark/2026-07-19-nemeton-v1-design.md)
6. [Swarm Meeting 详细设计](docs/spark/2026-07-19-nemeton-swarm-meeting-design.md)

## 快速开始

需要 Go 1.25.0 以上、Git 2.41.0 以上，以及 Linux 或 macOS。Windows 不支持。

```bash
go run ./cmd/nemeton daemon start
```

daemon 在前台运行。另开终端后可执行：

```text
http://127.0.0.1:7373
```

Web UI 是默认会议入口。它允许拖拽配置 Provider、模型和思考强度，查看自然语言讨论与 Proposal，
并逐项审阅 Recorder 结果。也可以继续使用 CLI：

```bash
go run ./cmd/nemeton project open /absolute/path/to/repository --json
go run ./cmd/nemeton project inspect <project-id> --json
go run ./cmd/nemeton project replay <project-id> --json
go run ./cmd/nemeton project relink <project-id> /new/absolute/path --json
go run ./cmd/nemeton project state <project-id> --json
go run ./cmd/nemeton meeting create <project-id> --title "Architecture review" --brief "Review the current repository reality" --json
go run ./cmd/nemeton meeting run <meeting-id> --wait --json
go run ./cmd/nemeton meeting show <meeting-id> --json
go run ./cmd/nemeton meeting handoff <meeting-id> --format markdown > approved-handoff.md
```

数据目录优先级为 `--data-dir`、`NEMETON_DATA_DIR`、平台默认目录。Linux 默认使用 `${XDG_DATA_HOME:-$HOME/.local/share}/nemeton`，macOS 默认使用 `$HOME/Library/Application Support/Nemeton`。SQLite 数据目录只支持本地文件系统；完整 `nemetond.sock` 路径在 Linux 不得超过 107 字节，在 macOS 不得超过 103 字节，超限会在创建运行状态前明确失败。

## 验证

本地和 CI 使用同一个硬性入口：

```bash
scripts/semantic-gate
```

它验证格式、diff、前端 lint/build/Playwright、race 测试、真实 Git/SQLite/Unix socket/HTTP/WebSocket 联合语义以及 CLI/daemon 构建。`main` 的 required check 名称是 `semantic-gate`。

## 当前实施边界

Milestone 1 在保持 Milestone 0 和 protocol v1 replay 语义不变的前提下走通以下路径：

```text
nemeton meeting create <project-id>
  -> 冻结 Reality Bundle
  -> 默认 3 个、可配置 0..N 个隔离 Design Agent 密封提案
  -> Reveal + 最多 3 轮讨论
  -> 单 Recorder 生成完整 Result
  -> Human 逐项处置 + 可选反馈
  -> Recorder answer / full patch / same-roster reconvene
  -> Human approve -> Approved Context + Coding Design Handoff
  -> replay 重建相同 Meeting / Current State digest，且不调用 Provider
```

Meeting Provider 可以在 Nemeton 管理的隔离 Workdir 中读写和实验，但目标 source checkout 必须保持不变。
Local Web UI 使用 protocol v2 且不创建 Verifier；当前未携带 Participants 的 legacy CLI/API 仍创建
protocol v1，历史可读取和 replay。当前不构建 Coding task worktree、目标仓库语义写入或产品 Gate
引擎；`semantic-gate` 仍是仓库自举 CI。
