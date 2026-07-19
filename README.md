# Nemeton

Nemeton 是一套面向 Agent 原生软件项目的本地语义控制平面。

它按顺序记录项目现实、多人讨论、语义决定、任务合同、worktree 执行、硬性验收和失败反馈。人类在会议中确认产品要解决的问题和风险边界，Agent 在已确认的语义与合同下完成实现。项目状态由事件历史重建，`NOW.md`、会议结果和当前任务视图都是可重新生成的投影。

当前仓库已经完成 Milestone 0：项目身份、只读 Git Reality、SQLite 事件内核、content-addressed artifact、投影和 replay。会议、Runner、worktree 执行和产品 Gate 引擎仍是后续里程碑。

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

```bash
go run ./cmd/nemeton project open /absolute/path/to/repository --json
go run ./cmd/nemeton project inspect <project-id> --json
go run ./cmd/nemeton project replay <project-id> --json
go run ./cmd/nemeton project relink <project-id> /new/absolute/path --json
```

数据目录优先级为 `--data-dir`、`NEMETON_DATA_DIR`、平台默认目录。Linux 默认使用 `${XDG_DATA_HOME:-$HOME/.local/share}/nemeton`，macOS 默认使用 `$HOME/Library/Application Support/Nemeton`。SQLite 数据目录只支持本地文件系统；完整 `nemetond.sock` 路径在 Linux 不得超过 107 字节，在 macOS 不得超过 103 字节，超限会在创建运行状态前明确失败。

## 验证

本地和 CI 使用同一个硬性入口：

```bash
scripts/semantic-gate
```

它验证格式、diff、静态检查、race 测试、真实 Git/SQLite/Unix socket 联合语义以及 CLI/daemon 构建。`main` 的 required check 名称是 `semantic-gate`。

## 当前实施边界

Milestone 0 必须走通以下路径：

```text
nemeton project open <path>
  -> 识别稳定 Project
  -> 记录 Repository Binding 和 Reality Revision 事件
  -> 生成当前 Project 投影
  -> 删除投影
  -> replay
  -> 重建出相同结果
```

Milestone 0 不调用模型，不创建 worktree，不修改用户仓库，也不构建会议 UI。`semantic-gate` 是仓库自举 CI，不是 Nemeton 产品 Gate 引擎。
