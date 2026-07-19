# Nemeton 文档索引

本文件只负责路由。每个新会话必须从根 `AGENTS.md` 进入本索引，再读取 `WORKFLOW.md`；需求、设计和当前进度正文不复制到这里。

## 0. 新会话固定顺序

```text
AGENTS.md
  -> docs/INDEX.md
  -> docs/WORKFLOW.md
  -> 按任务类型读取权威文档
  -> 回到真实代码、配置、schema 和 Git 取证
```

如果工作手册缺失、链接失效或权威文档冲突，停止会受冲突影响的工作。工作手册自身不完整时先使用 `$babel-build`；新需求使用 `$babel <需求>`。

## 1. 事实优先级

当前用户明确指令限定本次任务范围。在仓库文档之间，继续采用既有事实优先级：

1. `docs/decisions/` 中状态为 `Accepted` 的决定。
2. `docs/projectneed.md` 中的当前产品语义。
3. `docs/NOW.md` 中的当前实施边界。
4. `docs/spark/` 中的详细设计。
5. `docs/research/` 中的调查材料。

研究材料只提供证据和候选方案，不能覆盖正式决定。`NOW.md` 只描述当前工作，不能改写长期产品语义。第 1～3 项与真实代码、配置或 schema 冲突时，不按优先级静默覆盖；必须列出冲突并停止需要该语义的工作。

## 2. 核心文档

| 文档 | 职责 | 当前状态 |
| --- | --- | --- |
| [`projectneed.md`](projectneed.md) | 汇总产品命题、领域模型、架构、流程、数据、验收和路线图 | Current |
| [`NOW.md`](NOW.md) | 记录当前里程碑、下一步、阻塞和本轮验收 | Current |
| [`WORKFLOW.md`](WORKFLOW.md) | 约束设计、执行、Gate、合并和失败反馈流程 | Current |
| [`writing-rules.md`](writing-rules.md) | 约束事实分层、信息落点、状态词和去重 | Current |
| [`decisions/README.md`](decisions/README.md) | 索引正式决定，说明修订和 supersede 规则 | Current |
| [`design/system-context.md`](design/system-context.md) | 区分当前仓库事实与已接受的未来系统边界 | Current |
| [`design/persistence-state-map.md`](design/persistence-state-map.md) | 记录持久化 owner、增减、重建和恢复语义 | Accepted design map |
| [`spark/2026-07-19-nemeton-v1-design.md`](spark/2026-07-19-nemeton-v1-design.md) | v1 模块、状态机、API、存储和自举设计 | Approved input |
| [`spark/2026-07-19-nemeton-swarm-meeting-design.md`](spark/2026-07-19-nemeton-swarm-meeting-design.md) | Swarm 会议时序、讨论、收敛和扩展设计 | Approved input |
| [`research/2026-07-19-public-multi-agent-designs.md`](research/2026-07-19-public-multi-agent-designs.md) | 公开多 Agent 系统的研究快照 | Evidence |

## 3. 正式决定

| 编号 | 决定 | 状态 |
| --- | --- | --- |
| [`0001`](decisions/0001-default-three-design-agents.md) | 完整会议默认使用三个平等 Design Agent | Accepted |
| [`0002`](decisions/0002-design-agents-horizontal-scaling.md) | Design Agent 使用 `0..N` 数据模型并支持水平扩展 | Accepted |
| [`0003`](decisions/0003-semantic-event-source-and-replay.md) | 语义控制面使用事件事实源，所有视图可重建 | Accepted |
| [`0004`](decisions/0004-sqlite-local-single-writer.md) | 本地 v1 使用 SQLite 和单逻辑写入者 | Accepted |
| [`0005`](decisions/0005-human-semantic-activation-and-system-gates.md) | 人类负责语义激活，系统 Gate 负责实现验收 | Accepted |
| [`0006`](decisions/0006-go-modular-monolith.md) | v1 使用 Go 模块化单体和系统 Git | Accepted |
| [`0007`](decisions/0007-project-workbook-is-shared-reality.md) | 项目工作手册是 Agent 协作的共享现实 | Accepted |

## 4. 按任务类型阅读

| 任务 | 必读内容 |
| --- | --- |
| 接手当前工作 | `NOW.md`，再读其中引用的决定和设计 |
| 改变长期产品语义 | `projectneed.md`、`decisions/README.md`、相关 Accepted decision；必须先获得用户确认 |
| 架构或持久化 | `design/system-context.md`、`design/persistence-state-map.md`、相关 Spark 设计和当前实现证据 |
| 实现或修复 | `NOW.md`、任务合同、相关设计、真实代码/配置/schema |
| 只读 Review | Review 合同、相邻 base/head、最终累计 head、相关决定与验证证据 |
| 调研竞品或外部方案 | `research/`；结论只能作为 Evidence 或 Proposed input |

## 5. 当前阅读路径

Milestone 0 的实现者只需依次读取：

```text
docs/NOW.md
  -> decisions/0003-semantic-event-source-and-replay.md
  -> decisions/0004-sqlite-local-single-writer.md
  -> projectneed.md 的 Project / Reality / Replay 部分
  -> v1 design 的 Milestone 0
```

会议实现者还要读取 Swarm Meeting 详细设计和公开研究材料。并行执行实现者还要读取 Worktree、Campaign、Gate 和 Trust State 章节。

## 6. 工作手册文件树

```text
AGENTS.md
docs/
├── INDEX.md
├── WORKFLOW.md
├── projectneed.md
├── NOW.md
├── writing-rules.md
├── decisions/
│   ├── README.md
│   └── 0001–0007
├── design/
│   ├── system-context.md
│   └── persistence-state-map.md
├── templates/
│   ├── agent-task-contract.md
│   ├── context-handoff.yaml
│   ├── decision-record.md
│   └── review-contract.md
├── spark/
└── research/
```

## 7. 索引维护验收

- 新增、改名或移动文档时，同一改动更新本索引和所有入站链接。
- 每个链接必须指向真实文件；不创建空链接或占位真源。
- 每类信息只有一个权威落点；摘要只引用，不复制整段需求或决定。
- Proposed、Accepted、Current implementation 和 Historical 必须显式区分。
