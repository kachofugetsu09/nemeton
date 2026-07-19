# Nemeton 完整产品语义

Nemeton 是一套面向 Agent 原生软件项目的本地语义控制平面。

> 状态：Current synthesis
>
> 日期：2026-07-19
>
> 范围：产品命题、长期不变量、领域模型、技术架构、工作流、验收与自举路线

> 权威边界：2026-07-19，用户确认 `docs/decisions/0001–0006` 以及本文中不与这些决定冲突的内容为长期产品语义。Accepted decision 优先于本文；当前实现事实仍以代码、配置、schema 和 Git 为证据。

本文是初始化前已经形成的完整 synthesis，因此保留现有领域模型、详细设计和路线图，不做批量迁移。后续新增信息必须遵守 [`writing-rules.md`](writing-rules.md)：长期语义在本文维护，决定理由进入 `decisions/`，问题级实现设计进入 `design/`，当前工作进入 `NOW.md`。

它把项目事实、多人讨论、人类决定、任务合同、并行执行、硬性验收和失败反馈保存为一条可追踪、可重放的项目历史。用户只在会议中处理产品语义和风险取舍。Agent 在独立 worktree 中完成实现，系统根据可验证证据决定任务能否进入下一阶段。

## 1. 产品目标

Nemeton 要让一个长期使用大量 Agent 的项目保持可预测。

Agent 已经可以快速生成大量代码。代码产量增长后，人类无法继续依靠逐行阅读获得项目控制感。项目仍然需要稳定的演化范围、清楚的所有权、可验证的不变量，以及失败后能够修正输入的反馈回路。

Nemeton 负责维护以下工作循环：

1. 固定当前项目现实。
2. 让多个模型基于相同证据独立分析。
3. 让它们围绕 Claim、Evidence 和 Conflict 讨论。
4. 把讨论编译成候选语义和任务合同。
5. 让人类确认新语义、风险边界和不可编码的取舍。
6. 把任务分发到互相隔离的 worktree。
7. 用 hard gate 和运行证据验收准确 SHA。
8. 把失败追溯到目标、合同、边界、环境、验证手段或实现。
9. 在既定政策内生成下一轮改进任务。
10. 从事件历史重新生成项目语义和当前工作视图。

## 2. 用户与使用场景

第一版服务一个本地用户。用户同时使用多个模型和编码 Agent 维护一个 Git 仓库。

典型场景包括：

- 讨论新功能或架构改动。
- 规划大规模重构。
- 把独立任务交给多个 Agent 并行执行。
- 让项目经历多轮修改、验证和再规划。
- 保存长期使命、边界、决定和未知项。
- 在人类无法阅读全部 diff 时维持系统级信任。
- 通过历史会议和执行记录恢复项目语义。

第一版产品形态是本地 daemon、CLI 和 HTTP/SSE API。会议 UI 在状态机稳定后接入。

## 3. 长期判断

### 3.1 人类保留项目控制权

人类负责：

- 项目为什么存在。
- 项目允许向哪里发展。
- 哪些不变量必须长期成立。
- 哪些风险可以接受。
- 多个方案无法兼得时选择哪一个。
- 系统显示出的复杂度属于合理扩展还是结构腐化。

人类无需确认 Agent 的每个局部技术动作，也不充当默认的逐行代码 Gate。

### 3.2 信任来自持续状态

一次 Review 只能给出一次 verdict。项目需要持续回答这些问题：

- 当前哪些不变量有可靠证据？
- 哪些检查已经证明自己能够失败？
- 哪些模块持续出现 bug、冲突或 surviving mutant？
- 哪些接口让多个 Agent 反复产生同类错误？
- 哪些项目现实已经失效？
- 哪些未知项正在扩大？

Nemeton 用 Trust State 保存这些信号。Trust State 不压缩成一个总分。

### 3.3 失败用于修正输入

系统先根据 Agent 的失败结果定位对应拥有层：

- 使命和目标没有说清。
- Task Contract 遗漏了约束。
- 接口或模块边界误导了执行者。
- 环境和验证工具没有准备好。
- 实现违反了已经清楚的合同。

Campaign 可以在政策允许的范围内生成下一轮任务。失败引入新产品语义或新的风险取舍时，Campaign 暂停并回到会议。

## 4. 产品位置

现有工具通常覆盖其中一个环节：

| 层 | 常见产品形态 | Nemeton 负责的增量 |
| --- | --- | --- |
| 语义 | spec、设计文档、需求模板 | 把事实、推断、决定、未知项和来源连起来 |
| 讨论 | 多模型问答、debate、council | 固定 Project Reality，保留 Conflict、Dissent 和 Human Choice |
| 规划 | Issue、DAG、任务看板 | 从已确认语义编译不可变 Task Contract 和动态 Campaign |
| 执行 | 编码 Agent、worktree runner | 管理写入所有权、lease、依赖和准确 base SHA |
| 验收 | CI、测试、Reviewer Agent | 检查是否真的能失败，保存 target SHA 和 Evidence |
| 治理 | PR、审批、审计日志 | 记录语义激活权、事件历史、replay 和 Trust State |

Nemeton 可以调用现有编码 Agent、GitHub、CI 和模型 API。它不复制这些工具的实现。

Multica 一类本地看板提供了项目选择、Issue 状态、Agent 派发和执行可见性，Nemeton 可以复用其中的操作方式。Nemeton 把看板状态放在语义历史、任务合同和 Gate 之后。看板展示当前投影，不能单独决定任务为什么成立、何时可以合并，以及失败应该修改哪项输入。

Babel skill 已经验证了 discussion-first、项目工作手册、任务合同、隔离实现、只读 Review 和语义 Gate。Nemeton 把这些流程变成持久领域对象、事件历史、共享会议和本地 daemon。OpenSpec、Superpowers 等 spec/workflow 工具可以继续作为上游输入或执行方法，Nemeton 负责跨会议、执行轮次和 Agent 保存项目级语义。

## 5. 模型变强后仍然成立的产品职责

更强的模型仍然需要处理以下问题：

- 多个执行者需要共享同一项目事实。
- 新产品语义需要明确授权。
- 并发写入需要所有权和隔离。
- 外部副作用需要记录、对账和恢复。
- 代码、测试和决定需要绑定准确版本。
- 项目需要保存历史原因和证据。
- 大量改动需要组合验收和持续信任状态。

第一版不把以下内容做成产品护城河：

- 固定 prompt 技巧。
- 针对某个模型的补救流程。
- 人工逐行 Review 界面。
- 通用聊天壳。
- 普通 Issue 看板。
- 写死的三个 Agent 字段。
- 让用户手工维护上下文摘要。
- 根据 Agent 自报信心计算通过分数。

## 6. 从《人月神话》继承的问题清单

只保留会影响多 Agent 或长轮次项目的内容。

| 问题 | 多 Agent 项目中的表现 | Nemeton 的处理方式 |
| --- | --- | --- |
| 概念完整性 | 多个 Agent 分别优化局部，组合后产品目标分裂 | Active Semantics、Canonical Decision、单一激活历史 |
| 沟通成本 | Agent 增加后互相传递大量摘要，事实在转述中失真 | Shared Blackboard、稳定 ID、Evidence 引用、稀疏 Review Graph |
| 向落后项目增加人手 | 任务边界没有理清时继续增加 Agent，冲突和返工随之增加 | Reality/Contract Gate、write scope、依赖和停止条件 |
| 外科手术队伍 | 每个参与者都改架构，写入权和裁决权混在一起 | 平等 Design Seat、单 Recorder、Verifier、确定性 Facilitator |
| 第二系统效应 | 第一版试图加入 SaaS、多仓库、远程 worker 和通用插件 | 明确 v1 非目标和里程碑边界 |
| 计划必须允许变化 | 大重构在第一轮之后出现新证据，初始 DAG 很快失效 | 动态 Campaign、Improvement Policy、可版本化合同 |
| 文档与沟通 | NOW、设计、聊天和代码各自记录不同事实 | 事件事实源、正式 Decision、可重新生成的投影 |
| 没有银弹 | 模型更强仍无法自动解决价值取舍、组织边界和系统预测 | 人类语义激活、硬性证据、持续 Trust State |
| 结构腐化 | 多轮变更让重复事实源、分支和隐式契约持续增加 | bug/conflict/mutation 信号、边界 finding、递归改进 |
| 进度错觉 | Agent 输出很多 diff，项目却没有更接近可验证目标 | Task Contract、Acceptance Obligation、Campaign stop policy |

## 7. 顶层领域对象

### 7.1 Project

Project 代表一条 Git 历史、一组已激活语义和一份持续变化的 Trust State。

不变量：

- Project ID 由 Nemeton 生成，不能使用目录名或 remote URL 充当主键。
- 一个 Project 在 v1 中绑定一个 Git repository identity。
- 同一 repository 的主 checkout 和 linked worktree 必须解析到同一 Project。
- 路径可以移动，Project ID 保持稳定。

### 7.2 Repository Binding

Repository Binding 保存：

- 展示路径。
- 规范化 top-level。
- Git common directory。
- remote identities。
- integration branch。
- 已发现 worktree。
- Nemeton 管理的 worktree root。

路径只用于重新定位。common-dir、Git 事实和用户确认共同决定 binding。

### 7.3 Project Reality

Reality Revision 是某个时刻的项目事实快照：

- commit SHA 和 tree SHA。
- integration branch。
- dirty 状态及其排除规则。
- 已激活语义 artifact 的 digest。
- AGENTS、README、架构文档和关键配置的 digest。
- 测试、CI、静态检查和部署检查清单。
- 已知冲突、未知项和缺失工具。

会议、合同和 Gate 都必须绑定 Reality Revision 或准确 target SHA。

### 7.4 Meeting

Meeting 是唯一的人类语义入口。每个会议固定到一个 Reality Revision。

会议分为：

- `project`：使命、演化范围、长期边界和不变量。
- `change`：架构调整、功能开发、事故或技术决定。

会议保存人类原话、Agent 发言、Evidence、Conflict、Unknown、Dissent 和 Human Choice。

### 7.5 Semantic Item

Semantic Item 是可以独立批准的最小语义单元：

- `mission`
- `direction`
- `invariant`
- `boundary`
- `decision`
- `acceptance`
- `risk`
- `unknown`
- `improvement_policy`

生命周期：

```text
proposed ──► selected ──► active ──► superseded
    │             │
    ├──► rejected └──► withdrawn
    └──► deferred
```

active 条目不能原地改写。修改时创建新版本并 supersede 旧条目。

### 7.6 Campaign

Campaign 是一段目标驱动的持续改进过程。任务图根据每轮 Evidence 动态生长。

每个 Campaign 必须携带 Improvement Policy：

- 目标与可观察指标。
- 允许的转换类型。
- 必须保持的不变量。
- 每种转换的证明义务。
- 时间、成本、轮数和变更规模上限。
- continue、stop、rollback 和 escalate 条件。

### 7.7 Task Contract

Task Contract 是 dispatch 后不可修改的执行合同。它至少包含：

- schema version。
- Project、Campaign 和 Reality Revision。
- base SHA。
- objective 和 preconditions。
- write scope 与 forbidden scope。
- invariants 和 acceptance。
- dependencies。
- stop conditions。
- escalation。

需求发生变化时创建新版本。旧 run 标记为 stale、cancelled 或 superseded。

### 7.8 Worktree Lease

Worktree Lease 表示某个任务的临时写入权：

- task、run、path、branch 和 base SHA。
- allowed write scope。
- 单调递增的 fencing token。
- holder、heartbeat 和 expires_at。
- provisioning、active、submitted、released、quarantined 或 failed。

旧进程恢复后必须携带当前 token。token 失效时，所有写入和完成声明都被拒绝。

### 7.9 Evidence 与 Trust State

Evidence 保存来源、artifact digest、producer、target SHA 和可验证结果。

Trust State 保存：

- 不变量的当前证据强度。
- Gate 的可失败证明。
- bug、冲突和 surviving mutant 的聚类。
- Reality stale 情况。
- 反复被多个 Agent 误解的边界。
- 正在扩大的未知项。

## 8. Swarm Meeting

### 8.1 角色

| 角色 | 权限 |
| --- | --- |
| Human | 提出问题，确认产品语义和风险取舍 |
| Facilitator | 固定事实、控制时序、分配讨论、运行 Convergence Gate |
| Design Agent | 提交完整方案、质询、修正、提供证据和异议 |
| Recorder | 编译带来源的 Synthesis Draft 和 Meeting Result |
| Verifier | 检查事实、约束、组合一致性和验收是否成立 |

Facilitator 是确定性程序。Recorder 只有编译权。Verifier 只能提交 finding。

### 8.2 默认会议时序

```text
共同事实冻结
    ↓
三个平等 Design Agent 密封并行设计
    ↓
匿名同时 Reveal
    ↓
Steelman / Challenge / Evidence Check
    ↓
Conflict Map
    ↓
围绕 blocking Conflict 讨论
    ↓
Recorder 生成 Synthesis Draft
    ↓
Verifier Audit
    ↓
逐项 Ratification
    ↓
Convergence Gate
    ↓
Meeting Result
    ↓
Human approve / edit / request evidence / reopen / reject
```

### 8.3 Agent 通信

所有关键讨论写入 Shared Blackboard。Agent 可以提交：

- question
- support
- challenge
- counterexample
- new_evidence
- amendment
- new_proposal
- dissent

关键消息必须引用 Proposal、Decision、Claim、Conflict 或 Evidence ID。通知通道只负责唤醒相关 Agent，不能承载私下决定。

小会议向所有 Design Agent 提供全部 Proposal。大会议把 Proposal 放在可查询白板中，并按 Decision Cluster 和固定 review degree 分配上下文，避免全连接互评。

### 8.4 收敛

Convergence Gate 只接受布尔条件：

```text
CONVERGED =
    canonical_decisions_complete
    AND hard_constraints_covered
    AND no_factual_block
    AND no_constraint_block
    AND decision_graph_consistent
    AND ratification_complete
    AND verifier_clear
    AND all_dissent_classified
```

多数票不能抵消有效 factual block 或 constraint block。剩余分歧只能进入 Human Choice、Experiment Required、Needs Evidence 或明确保留的 Dissent。

### 8.5 水平扩展

- 默认三个 Design Agent。
- `meeting_participants` 使用 `0..N` 集合。
- Proposal Review 使用固定 `K` 个 assignment。
- Decision/Conflict 按主题聚类。
- Shard Recorder 可以并行整理 cluster。
- Final Recorder 保持唯一有效 writer lease。
- `N <= 5` 时全员 Ratification。
- `N > 5` 时使用 Decision Cluster Coverage，所有硬阻塞仍需逐项解决。

## 9. 静态语义的生成与激活

Recorder 从会议中提炼 Candidate Semantic Item。每个候选保存：

- statement。
- rationale。
- scope。
- source message IDs。
- supporting 和 opposing evidence。
- consequences。
- unknowns。
- proposed acceptance obligations。

用户逐项选择 `select`、`reject` 或 `defer`。selected 条目进入 governance change。对应 artifact 进入 integration history 后，系统追加 `SemanticItemActivated` 事件并把条目标记为 active。

目标仓库中的静态语义投影建议使用：

```text
.nemeton/
├── project.yaml
├── charter.md
├── gates.yaml
├── decisions/
│   └── <semantic-item-id>.md
└── policies/
    └── <policy-id>.yaml
```

这些文件面向 Git、编码 Agent 和 CI。会议原文、候选项、运行记录和 Trust State 保存在 Nemeton 数据目录中。

## 10. 事件事实源、投影和 replay

### 10.1 事实层次

```text
领域事件流
  ├── 引用 content-addressed artifacts
  ├── 生成 SQLite current projections
  ├── 生成 NOW.md / Meeting Result / Trust State
  └── 生成目标仓库中的 .nemeton/ 静态语义
```

事件包含：

- event ID 和 project stream sequence。
- aggregate type 和 aggregate ID。
- event type 和 payload schema version。
- actor、causation ID 和 correlation ID。
- expected stream version。
- artifact refs。
- occurred_at 和 recorded_at。

### 10.2 写入规则

1. daemon 在 HTTP/CLI 边界校验 command schema。
2. 领域服务读取当前 stream revision。
3. expected version 不匹配时返回 conflict。
4. 领域服务检查状态机和不变量。
5. 事务追加事件并更新数据库投影。
6. 文件 projector 在提交后生成临时文件并原子替换目标。
7. 生成失败时保留 dirty 标记，后台或重启后重试。

内部函数信任已经验证的领域类型，不重复添加无法触发的防御性检查。

### 10.3 Replay

Replay 必须满足：

- reducer 确定且没有外部副作用。
- 相同事件和 schema 生成相同规范化状态。
- 未知事件版本使 replay 明确失败。
- artifact 丢失或 digest 不符使相关投影失败。
- replay 不重新执行 Git 命令、模型调用、PR、部署或 Gate command。
- 外部系统的当前状态由 reconcile 读取后生成新事件。

### 10.4 仓库清空后的恢复边界

Nemeton 数据目录和 artifact store 保存恢复所需的内容：

- 项目使命、演化范围、不变量和决定。
- 历史会议、冲突、异议和 Human Choice。
- Task Contract、Campaign 和验收记录。
- `NOW.md`、Meeting Result 和 `.nemeton/` 文件。

代码恢复需要额外保存 Git remote、Git bundle、commit objects、patch 或 delivery artifact。Nemeton 不根据自然语言会议重新生成并声称得到原代码。

## 11. SQLite 与本地 daemon

第一版使用 Go 模块化单体：

```text
nemeton CLI ─────┐
future Web UI ───┼──► nemetond ───► SQLite
Agent Runner ────┘        │            │
                          │            └── domain events + projections
                          ├── Git/worktree
                          ├── model providers
                          ├── gate commands
                          └── artifact store
```

约束：

- `nemetond` 是唯一逻辑数据库写入者。
- Agent 通过 command/API 提交变化。
- SQLite 位于本机文件系统，使用 WAL、短事务和 busy timeout。
- UI 和 SSE 不持有长 read transaction。
- NOW projector 是唯一文件 writer。
- 数据库不放入 NFS 或同步目录。
- 在线备份使用 SQLite backup API，并执行 integrity check。

PostgreSQL 留给团队共享、多机写入、高可用和远程服务形态。Agent 数量增加不会单独触发数据库迁移。

## 12. Worktree 并行执行

### 12.1 创建

1. Task Contract 处于 ready，依赖已经满足。
2. Reality Revision 仍然有效。
3. 调度器获得 repository-scoped Git metadata lock。
4. 系统解析准确 base SHA。
5. 系统创建唯一 branch 和 managed worktree。
6. 系统写入合同投影和 runtime metadata。
7. 系统签发带 fencing token 的 Worktree Lease。
8. Agent 在 worktree 中运行。

### 12.2 写入边界

- 调度前检查 write scope overlap。
- 重叠 scope 默认阻止并行。
- 运行后用真实 diff 校验 write scope。
- 越界变更进入 failed gate 或 quarantined。
- 文本可以无冲突，组合语义仍需 Integration Gate 验证。

### 12.3 合并

- Agent 可以并行生成多个 delivery 或 PR。
- integration branch 使用串行 merge queue。
- 每次合并前校验 head、base、合同和 Gate digest。
- 每次合并后生成新的 integration SHA。
- 未合并任务重新检查 base 假设并重跑受影响 Gate。
- 各分支分别通过不能推导出组合状态通过。

### 12.4 清理

自动清理必须同时证明：

- 路径位于 managed worktree root。
- 当前 holder 和 fencing token 匹配。
- Git identity、branch 和 task 匹配记录。
- 没有未保存的修改和证据。
- delivery 已合并、取消或按政策终止。

无法证明安全时进入 quarantined。系统永不递归删除用户主 checkout。

## 13. Gate 与持续信任

### 13.1 Gate 层级

1. Contract Gate：合同、Reality、base、依赖和写入边界有效。
2. Task Gate：任务测试、类型检查、静态检查和语义验收通过。
3. Diff Boundary Gate：真实修改没有越过授权范围。
4. Integration Gate：合并后的组合系统满足不变量。
5. Release Gate：高风险变更经过 staged rollout、监控或人工语义决定。

### 13.2 Hard Check 标准

可靠的检查需要：

- assertion 针对可观察行为。
- 覆盖行为实际经过的关键路径。
- 失败信息能定位具体不变量。
- 保存 target SHA、输入 digest 和执行环境。
- 使用 mutation 或受控 fault 证明关键检查能够失败。

### 13.3 系统信号

- Bug 按模块、接口和契约聚类。
- Mutation testing 标出测试仍然接受的错误程序。
- 静态结构标出复杂分支、重复事实源和边界漂移。
- 多 Agent 的反复冲突标出不稳定 seam。
- staged rollout 和监控标出外部、时序和负载问题。

这些信号帮助人类判断系统是否仍然可预测，也为下一轮 Campaign 提供输入。

## 14. 失败、恢复和外部副作用

daemon 启动后执行 reconcile：

- 检查非终态 meeting、campaign、task、run 和 lease。
- 读取 `git worktree list --porcelain`。
- 对账 branch、head、diff 和数据库记录。
- 验证进程身份，不能只依赖旧 PID。
- 递增 fencing token，终止失联 holder 的旧写权限。
- 把无法证明归属的资源放入 quarantined。
- 追加 recovery event 和 trust finding。

边界失败必须暴露：

- Provider 不可用：run 失败并显示缺席。
- Git fetch 失败：任务按合同停止或明确使用固定本地 SHA。
- Gate command 不存在：Gate 失败。
- schema 无法解析：拒绝输入。
- artifact 缺失或 digest 不符：证据失效。
- 数据库写入失败：command 失败，不能用内存状态假装提交成功。

## 15. 数据结构

### 15.1 事件内核

| 对象 | 关键字段 |
| --- | --- |
| `project_streams` | `project_id`, `current_sequence`, timestamps |
| `domain_events` | `event_id`, `project_id`, `sequence`, `aggregate_type`, `aggregate_id`, `event_type`, `schema_version`, `payload_json`, `actor`, `causation_id`, `correlation_id`, timestamps |
| `event_artifacts` | `event_id`, `artifact_digest`, `relation` |
| `projection_states` | `project_id`, `projection_name`, `projected_through_sequence`, `schema_version`, `result_digest`, `dirty` |
| `artifacts` | `digest`, `media_type`, `size`, `storage_uri`, timestamps |

### 15.2 Project 与 Reality 投影

| 对象 | 关键字段 |
| --- | --- |
| `projects` | `id`, `name`, `status` |
| `repository_bindings` | `project_id`, paths, `git_common_dir`, remote identity, integration branch |
| `reality_revisions` | `id`, `project_id`, commit/tree SHA, manifest digest, dirty observed, status |
| `reality_artifacts` | reality ID, kind, path/URI, digest, metadata |

### 15.3 Meeting 投影

| 对象 | 关键字段 |
| --- | --- |
| `meetings` | project/reality ID, kind, title, status |
| `meeting_participants` | meeting ID, seat, role, provider, model, status |
| `round_runs` | round, participant, input/output digest, status |
| `proposals` | author, problem model, design artifact, status |
| `claims` | proposal, statement, kind, status |
| `evidence` | project, claim, artifact digest, producer, target SHA |
| `conflicts` | type, question, positions, blocking, status |
| `discussion_events` | kind, author, refs, content artifact, round |
| `synthesis_shards` | cluster, recorder run, source refs, artifact |
| `ratification_assignments` | decision, participant, verdict, refs |
| `semantic_items` | kind, statement, source refs, status, supersedes ID |

### 15.4 Campaign 与执行投影

| 对象 | 关键字段 |
| --- | --- |
| `campaigns` | project/meeting/reality ID, goal, policy, status, budget |
| `task_contracts` | campaign, version, contract digest, status |
| `task_dependencies` | task, dependency, kind |
| `agent_runs` | role, provider, model, session, input digest, status |
| `worktree_leases` | task/run/path/branch/base, fencing token, status, heartbeat |
| `gate_definitions` | project, kind, definition digest, status |
| `gate_runs` | definition, target SHA, task, status, result artifact |
| `deliveries` | task, head SHA, remote, PR, merge status |
| `trust_findings` | project, scope, signal, severity, evidence refs, status |

第一版不创建固定 `designer_a/designer_b/designer_c` 字段，也不为每张表创建 repository interface。

## 16. 状态机

### Meeting

```text
draft ─► open ─► deliberating ─► awaiting_human ─► concluded
           │           │                │
           ├──────────► failed          └──► deliberating
           └──────────► cancelled

Reality 改变后：open/deliberating/awaiting_human ─► stale
```

### Task

```text
draft ─► blocked ─► ready ─► dispatching ─► running ─► submitted
                         │          │             │
                         │          ├──► failed   ├──► failed_gate
                         │          └──► cancelled│
                         │                        ▼
                         └────────────────────► verified ─► merged
                                                     │
                                                     └──► superseded
```

### Worktree Lease

```text
provisioning ─► active ─► submitted ─► released
      │             │           │
      └──► failed   ├──► expired
                    └──► quarantined
```

### Semantic Item

```text
proposed ─► selected ─► active ─► superseded
    │             │
    ├──► rejected └──► withdrawn
    └──► deferred
```

## 17. 后端模块

```text
nemeton/
├── cmd/
│   ├── nemeton/
│   └── nemetond/
├── internal/
│   ├── project/
│   ├── meeting/
│   ├── semantics/
│   ├── campaign/
│   ├── worktree/
│   ├── runner/
│   ├── gate/
│   ├── trust/
│   ├── events/
│   ├── projection/
│   ├── artifact/
│   ├── store/
│   └── api/
├── migrations/
├── schemas/
├── web/
└── docs/
```

模块边界遵守以下原则：

- HTTP、CLI、Provider、Git、数据库反序列化和 artifact 读取属于信任边界。
- 领域服务拥有状态机和事务。
- reducer 只接收验证过的领域事件。
- Git、Provider、时钟和外部 command runner 使用窄接口。
- 内部契约被破坏时 fail-fast、fail-loud。

## 18. 最小 API 和 CLI

```text
POST /v1/projects/open
GET  /v1/projects/{id}
POST /v1/projects/{id}/reality-revisions

POST /v1/projects/{id}/meetings
POST /v1/meetings/{id}/messages
POST /v1/meetings/{id}/deliberations
GET  /v1/meetings/{id}/candidates
POST /v1/semantic-items/{id}/select
POST /v1/semantic-items/{id}/reject
POST /v1/semantic-items/{id}/defer

POST /v1/meetings/{id}/campaigns
POST /v1/tasks/{id}/dispatch
POST /v1/tasks/{id}/cancel
GET  /v1/campaigns/{id}

GET  /v1/events
GET  /v1/projects/{id}/trust-state
```

```text
nemeton daemon start
nemeton doctor
nemeton project open <path>
nemeton project inspect <project-id>
nemeton project replay <project-id>
nemeton meeting create <project-id> --title <title>
nemeton meeting show <meeting-id>
```

## 19. 大规模重构示例

用户提出：“认证状态存在三份来源，希望消除重复状态，同时保持外部行为不变。”

### 会议

1. Nemeton 固定当前 SHA、认证模块、测试、CI 和历史 bug。
2. Designer 提出统一 AuthStateService。
3. Maintainer 分析迁移顺序、双写窗口和回滚。
4. Adversary 查找绕过入口、并发竞争和不可观测路径。
5. Agent 共同确认三份状态源、公开 API 和必须保持的错误契约。
6. Recorder 按所有权、迁移、测试和回滚生成 Canonical Decisions。
7. Verifier 检查每项决定是否有 Evidence 和可失败的 Acceptance。
8. 人类确认是否允许短暂兼容层，以及可以接受的迁移风险。

### 执行

Campaign 不预先锁死全部任务。第一轮可以包含：

- 建立真实行为回归测试。
- 增加状态源读写观测。
- 引入单一拥有层。
- 迁移一条调用路径。

系统在每次合并完成后读取新的 bug、mutation、冲突和调用证据。仍有绕过路径时，Campaign 生成下一轮迁移任务。新的公开行为取舍出现时，Campaign 暂停并返回会议。

### 完成条件

- 所有认证状态写入只有一个拥有层。
- 旧入口已经删除或明确隔离。
- 外部错误契约保持一致。
- 关键测试已经证明能够捕获受控错误。
- 合并后的 integration SHA 通过 Gate。
- 没有未分类的 surviving mutant、未知调用路径或 blocking finding。

## 20. 自举路线

### Milestone 0：Project、Reality、Event、Replay

走通 `project open -> events -> projection -> replay`。这一阶段不修改目标仓库。

### Milestone 1A：单人会议与候选语义

走通 Human Statement、Message、Evidence、Candidate Semantic Item、select/reject/defer 和 `NOW` 投影。

### Milestone 1B：Swarm Meeting

接入默认三个 Design Agent、sealed proposal、Reveal、Conflict、Recorder、Verifier、Ratification 和 Convergence Gate。

### Milestone 2：单任务执行

加入 governance worktree、Task Contract、单个 task worktree、Agent Runner、Task Gate、Diff Boundary Gate 和 delivery。

完成 Milestone 2 后，Nemeton 使用自己的会议、合同、worktree 和 Gate 开发后续功能。

### Milestone 3：并行与合并队列

加入动态任务图、多 lease、scope conflict、并行 delivery、串行 merge queue 和 Integration Gate。

### Milestone 4：递归 Campaign 与 Trust State

加入 Improvement Policy、结果驱动任务生成、信号聚类、continue/stop/rollback/escalate 和 Gate 可失败证明。

## 21. v1 非目标

- 云端多租户 SaaS。
- 多仓库共同组成一个 Project。
- Kubernetes 和远程 worker。
- 通用 Issue 管理和团队绩效系统。
- 任意文档资源容器。
- 自动修改用户当前 checkout。
- 自动解决全部 merge conflict。
- 人工逐行 Review 作为默认 Gate。
- LLM 自己声明 Gate 通过。
- 根据覆盖率或测试 green 计算信任。
- Python 控制面、消息队列、向量数据库和微服务。
- PostgreSQL backend。
- 通用插件市场。

## 22. 开工前的仓库设置

产品设计已经收敛。Milestone 0 开始前仍需确定几个仓库级参数：

- Go module path。
- v1 公开支持的平台。
- 应用数据目录。
- managed worktree root。
- integration branch 的解析与用户选择规则。

这些参数不能由实现 Agent 根据空仓库状态猜测。

## 23. 产品成功标准

Nemeton v1 完成后必须满足：

- 每个 Agent 都知道准确 Reality、目标、写入范围和验收义务。
- 会议中的事实、意图、推断、决定、异议和未知项彼此分开。
- 人类只处理产品语义、风险边界和不可编码的取舍。
- 多 worktree 并行不会共享未经批准的写入所有权。
- 每个交付都能回到 Task Contract、target SHA、Gate 和 Evidence。
- 每个失败都能落到一个明确拥有层。
- 项目状态可以从事件历史重新生成。
- 仓库语义投影丢失后可以按 digest 恢复。
- 外部副作用不会在 replay 时重复执行。
- Nemeton 能使用自己的机制完成后续里程碑。

这些性质共同确定 Nemeton 的产品位置：Agent 原生项目的本地语义控制平面。
