# Decision 0003：语义控制面使用事件事实源和可重新生成的投影

> 状态：Accepted
>
> 日期：2026-07-19
>
> 决策者：Human
>
> 修订：Decision 0009 将产品 `NOW.md` 投影改为数据库 Current State，并将 SSE 运行连接改为 WebSocket；本文件中的旧名称仅保留为历史语境。

## 决定

Nemeton 的语义控制面使用 append-only 领域事件作为事实源。关系表、`NOW.md`、会议结果、任务视图、Trust State 和 UI 白板都是投影，删除后必须能够重新生成。

事件事实源覆盖：

- Project 注册、重新绑定和状态变化。
- Reality Revision。
- Meeting 发言、讨论事件和参与者结果。
- Proposal、Decision、Claim、Evidence、Conflict 和 Dissent。
- Human select、reject、defer、approve 和 reopen。
- Semantic Item 的版本与 supersession。
- Task Contract、Campaign 和领域状态迁移。
- Gate Result、Evidence 和 Trust Finding。

大正文、日志、补丁、测试报告和已激活语义文件保存到 content-addressed artifact store。事件只保存 digest、媒体类型、大小和稳定引用。

## 投影规则

- reducer 必须是确定性的纯领域逻辑。
- projector 记录 `projected_through_sequence`、schema version 和结果 digest。
- command 使用 `expected_revision` 或 `expected_stream_version` 做乐观并发控制。
- 事件追加与数据库投影更新发生在同一个 SQLite 事务中。
- Markdown 和其他文件投影在事务提交后生成，并使用临时文件、`fsync` 和原子 rename。
- 文件投影生成失败时保留 `projection_dirty`，daemon 下次启动后继续生成。
- replay 只重算状态，不能重新调用 Git 写命令、模型 Provider、外部 API、PR 操作或 Gate command。

## Git 与事件历史的关系

Git 保存代码和已经发布到项目历史的语义投影。事件历史保存这些语义如何形成、由谁确认、引用了哪些证据，以及哪个 artifact digest 被激活。

系统依靠保留的 Nemeton 数据目录和 artifact store 恢复会议、决定、Task Contract、`NOW.md` 和 `.nemeton/` 语义文件。目标仓库清空不会删除这些独立保存的内容。源代码仍然依赖 Git remote、Git bundle、对象备份、commit 或 patch artifact；语义事件不能凭空恢复未保存的代码对象。

## 不采用事件溯源的运行状态

以下状态由当前关系表、进程监督和启动 reconcile 共同维护：

- PID 和进程句柄。
- WebSocket 连接。
- 临时锁和 SQLite busy 状态。
- heartbeat 的当前观测值。
- 可以从 Git、文件系统或进程表再次探测的现场。

重要运行变化可以追加审计事件，replay 不能把过期 PID、heartbeat 或 lease 恢复成有效状态。fencing token 和租约归属必须结合当前现场重新裁决。

## 原因

Nemeton 需要回答“当前结论为什么成立”和“仓库视图丢失后能否恢复”。只保存当前行状态无法完整回答这两个问题。把所有进程细节都做成事件溯源又会制造错误恢复，因为外部世界不会随数据库 replay 回到过去。

选择性事件溯源把长期语义与临时运行现场分开：语义可以重放，外部副作用只能对账。

## 验收

- 任意投影被删除后，replay 得到相同规范化内容和 digest。
- 相同事件序列在相同 reducer/schema 下得到相同结果。
- replay 期间外部副作用调用数为零。
- 每个 active Semantic Item 能回到 Human Decision、Meeting 和来源 Evidence。
- artifact 缺失或 digest 不符时，相关投影明确失败。
- schema 无法迁移时启动失败，不能跳过未知事件。
