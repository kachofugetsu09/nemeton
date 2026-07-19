# Nemeton 文档索引

Nemeton 的文档按“正式决定、完整产品语义、当前状态、详细设计、研究证据”分层保存。

## 1. 事实优先级

冲突解释采用以下优先级：

1. `docs/decisions/` 中状态为 `Accepted` 的决定。
2. `docs/projectneed.md` 中的当前产品语义。
3. `docs/NOW.md` 中的当前实施边界。
4. `docs/spark/` 中的详细设计。
5. `docs/research/` 中的调查材料。

研究材料只提供证据和候选方案，不能覆盖正式决定。`NOW.md` 只描述当前工作，不能改写长期产品语义。

## 2. 核心文档

| 文档 | 职责 | 当前状态 |
| --- | --- | --- |
| [`projectneed.md`](projectneed.md) | 汇总产品命题、领域模型、架构、流程、数据、验收和路线图 | Current |
| [`NOW.md`](NOW.md) | 记录当前里程碑、下一步、阻塞和本轮验收 | Current |
| [`WORKFLOW.md`](WORKFLOW.md) | 约束设计、执行、Gate、合并和失败反馈流程 | Current |
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

## 4. 当前阅读路径

Milestone 0 的实现者只需依次读取：

```text
docs/NOW.md
  -> decisions/0003-semantic-event-source-and-replay.md
  -> decisions/0004-sqlite-local-single-writer.md
  -> projectneed.md 的 Project / Reality / Replay 部分
  -> v1 design 的 Milestone 0
```

会议实现者还要读取 Swarm Meeting 详细设计和公开研究材料。并行执行实现者还要读取 Worktree、Campaign、Gate 和 Trust State 章节。
