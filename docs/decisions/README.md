# Nemeton 决策索引

Accepted decision 记录已经由用户确认、会约束未来实现的决定。编号递增；错误或过时决定不删除，通过后续记录标记 `Amended` 或 `Superseded`。

## 当前决定

| 编号 | 标题 | 状态 | 日期 |
| --- | --- | --- | --- |
| [`0001`](0001-default-three-design-agents.md) | v1 默认使用三个 Design Agent | Accepted | 2026-07-19 |
| [`0002`](0002-design-agents-horizontal-scaling.md) | Design Agent 必须支持水平扩展 | Accepted | 2026-07-19 |
| [`0003`](0003-semantic-event-source-and-replay.md) | 语义控制面使用事件事实源和可重新生成的投影 | Accepted | 2026-07-19 |
| [`0004`](0004-sqlite-local-single-writer.md) | 本地 v1 使用 SQLite 和单逻辑写入者 | Accepted | 2026-07-19 |
| [`0005`](0005-human-semantic-activation-and-system-gates.md) | 人类负责语义激活，系统 Gate 负责实现验收 | Accepted | 2026-07-19 |
| [`0006`](0006-go-modular-monolith.md) | v1 使用 Go 模块化单体和系统 Git | Accepted | 2026-07-19 |
| [`0007`](0007-project-workbook-is-shared-reality.md) | 项目工作手册是 Agent 协作的共享现实 | Accepted；能力边界由 0008 修订 | 2026-07-19 |
| [`0008`](0008-semantic-ci-gate.md) | 原子语义与跨组件联合语义由 CI Hard Gate 保护 | Accepted；Amends 0007 | 2026-07-19 |
| [`0009`](0009-milestone-1-meeting-runtime.md) | Meeting 使用持久化 Coding Agent Workdir 与 WebSocket | Accepted；部分流程由 0010 修订 | 2026-07-19 |
| [`0010`](0010-protocol-v2-recorder-review-loop.md) | Protocol v2 使用单 Recorder 驱动可迭代的人类审阅闭环 | Accepted；Amends 0009 的当前会议流程 | 2026-07-20 |

## 记录规则

每项决定至少包含状态、日期、问题、决定、理由、后果和验证。改变长期产品语义前必须取得用户确认；实现现状不能单独把 Proposed 方案升级为 Accepted。

修订时：

1. 保留原文件和原状态历史。
2. 创建新编号，明确 `Amends` 或 `Supersedes` 哪项决定。
3. 在本索引同步新旧关系。
4. 对账 `projectneed.md`、相关设计和 `NOW.md`。

决定之间冲突时，较新的明确 amendment 优先；不存在明确修订关系时停止并请求裁决。
