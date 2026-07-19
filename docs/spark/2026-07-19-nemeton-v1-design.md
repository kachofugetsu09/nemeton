# Nemeton v1：面向多 Agent 项目的本地语义控制平面

> 状态：Approved for staged implementation
>
> 日期：2026-07-19
>
> 用途：作为 Babel 启动 Nemeton 仓库设计、任务合同和实现工作的上游输入。
>
> 当前仓库：`/mnt/data/coding/nemeton`

## 1. 结论

Nemeton 第一版应当是一个本地优先、单仓库、多 worktree 的模块化单体。它不是另一个 Issue 看板，也不是把多个 Agent CLI 包在一个聊天界面里，而是维护以下闭环：

1. 把一次讨论固定到可验证的项目现实上。
2. 让多个模型基于相同证据独立分析、互相质询并暴露分歧。
3. 把讨论编译成候选语义、任务合同和验收义务。
4. 只让人类在会议中确认新的产品语义，不要求人类逐行审查代码。
5. 把可并行的任务分发到独立 worktree，并持续管理写入所有权、依赖和集成顺序。
6. 用硬性 Gate、运行证据和持续信任状态判断项目是否仍然可预测。
7. 把失败追溯到不清晰的输入、错误边界或不足的验证能力，再递归改进。

第一版后端选用 **Go**，持久化选用本机 **SQLite**，Git 操作调用系统 `git`，模型和 Agent 通过明确的 Provider/Runner 边界接入。核心产品以一个本地 daemon 运行；先提供 CLI 和 HTTP/SSE 接口，会议 UI 在核心状态机稳定后接入。

## 2. 产品命题

Agent 写代码的速度已经超过人类逐行阅读代码的速度。继续把人工 Review 当作信任来源，会让人类成为吞吐瓶颈，或者把 Review 退化成没有真实判断能力的盖章。

Nemeton 不试图让人类重新追上代码产量。它把人的职责上移到：

- 说明项目为什么存在、允许往哪里发展。
- 确认真正的新产品语义和不可接受的风险。
- 判断系统显示出的高熵是合理扩展还是结构腐化。
- 在没有既有政策可以裁决时作出决定。

代码实现、局部 Review、测试生成、错误定位和递归改进可以由 Agent 完成，但它们必须在一套可追踪、可重放、可验证的项目语义下运行。

Nemeton 的核心不是“决策按钮”，而是决策之前的高质量讨论，以及决策之后不会失真的语义编译和执行链路。

## 3. 第一版范围

### 3.1 必须完成

- 注册一个本地 Git 仓库，建立稳定的 Nemeton Project 身份。
- 从任意 linked worktree 打开时，识别出同一个 Git repository identity。
- 读取项目代码、文档、Git 状态、测试和 CI 配置，建立带版本的 Project Reality。
- 创建固定到某个 commit SHA 的长期项目会议或变更会议。
- 保存人类发言、模型发言、证据引用、分歧、未知项和候选语义。
- 由单一 Recorder 编译候选条目，由用户选择接受、拒绝或暂缓。
- 把已选择语义转成版本化的项目静态内容和任务合同。
- 为执行任务创建受管理的独立 worktree、分支和 writer lease。
- 支持多个语义独立任务并行执行，支持依赖任务串行推进。
- 执行确定性 Gate，保存证据，并在合并后重新验证组合状态。
- 在 daemon 重启后恢复 meeting、campaign、task、run 和 worktree 的真实状态。
- 允许 Nemeton 在最小内核完成后开始管理自身仓库的后续开发。

### 3.2 第一版明确不做

- 不做云端多租户 SaaS。
- 不做通用 Issue 管理或团队绩效系统。
- 不做多个仓库共同组成一个 Project。
- 不做 Kubernetes、远程 worker 或跨机器调度。
- 不做任意文档资源容器；Project 的根事实先限定为一个 Git 仓库。
- 不把 Agent 投票数当成正确性。
- 不让 LLM 自己声明 Gate 通过。
- 不把代码覆盖率、测试通过或 Reviewer 点赞单独当成系统可信。
- 不自动修改用户当前 checkout。
- 不承诺自动处理所有 merge conflict。
- 不在核心链路引入 Python 服务、消息队列、向量数据库或微服务。

## 4. 领域定义

### 4.1 Project

`Project` 是 Nemeton 的顶层长期身份：一条 Git 历史、一套被确认的项目语义和一份持续演化的信任状态。

`Project` 不是文件夹，也不是 worktree。路径可以移动，worktree 可以创建和销毁，Project 身份必须保持稳定。

第一版约束：

- 一个 Project 绑定一个 Git repository identity。
- 一个 repository identity 可以派生多个 worktree。
- Project 使用 Nemeton 生成的稳定 ID，不使用目录名或 remote URL 充当主键。
- remote URL 可以为空；没有 remote 时仍能本地执行和提交，但不能生成远程 PR。

### 4.2 Repository Binding

Repository Binding 记录 Project 与本机 Git 仓库的关系：

- 用户选择时的展示路径。
- `git rev-parse --show-toplevel` 的结果。
- 规范化后的 Git common directory。
- remote 列表及规范化 remote identity。
- integration branch。
- 当前已发现的 worktree。
- Nemeton 管理的 worktree 根目录。

路径只是可重新绑定的定位信息。Project ID 才是持久身份。

### 4.3 Project Reality

Project Reality 是某个时刻 Nemeton 对项目事实的可追踪快照。它至少固定：

- commit SHA 和 tree SHA。
- integration branch。
- 工作区是否存在未提交内容；未提交内容是否被排除在快照之外。
- 已批准的 Nemeton 语义文件及其 digest。
- AGENTS、CLAUDE、README、架构文档和关键配置的 digest。
- 已发现的测试命令、CI、静态检查和部署检查。
- 已知未知项、冲突和缺失能力。

Reality Revision 不复制整个仓库。它保存精确版本、清单和证据引用，Agent 在需要时进入对应 worktree 读取真实内容。

### 4.4 Meeting

Meeting 是唯一的人类语义入口。它不是临时聊天，而是固定到一个 Reality Revision 的持久讨论空间。

两种会议共享同一模型：

- `project`：维护项目使命、长期边界、不变量和方向。
- `change`：讨论一个具体问题、重构、功能或事故。

会议中可以有多个模型参与，但只有一个 Recorder 持有语义写入租约。其他参与者只能追加观点、质询和证据，不能直接激活项目语义。

### 4.5 Semantic Item

Semantic Item 是从会议中提炼出的最小可批准语义单元。第一版支持：

- `mission`：项目为什么存在。
- `direction`：允许向哪里演化。
- `invariant`：任何实现都不得破坏的性质。
- `boundary`：职责、数据或信任边界。
- `decision`：在若干方案之间作出的选择。
- `acceptance`：可观察的完成条件。
- `risk`：必须持续观察或缓解的风险。
- `unknown`：当前不能诚实回答的问题。
- `improvement_policy`：允许 Campaign 如何递归改进、何时停止或升级。

状态：

```text
proposed ──► selected ──► active ──► superseded
    │             │
    ├──► rejected └──► withdrawn
    └──► deferred
```

- `proposed`：Recorder 从会议中生成的候选。
- `selected`：用户在会议中确认，但对应语义变更尚未进入 integration branch。
- `active`：语义文件已经进入目标 Git 历史，成为项目事实。
- 修改 active 条目必须创建新版本并 supersede 旧条目，不能原地改写历史。

### 4.6 Campaign

Campaign 是一次目标驱动的持续改进过程。它不是预先固定的 DAG，而是一张根据每轮结果动态生长的任务图。

Campaign 必须携带 Improvement Policy：

- 目标和可观测改进指标。
- 允许的转换类型。
- 必须保持的不变量。
- 每种转换的证明义务。
- 最大轮数、时间、成本或变更规模。
- 继续、停止、回滚和升级到人类会议的条件。

Agent 可以在政策范围内提出并执行下一轮任务。遇到新的产品语义、不可编码的取舍或政策之外的风险时，Campaign 暂停并回到会议，而不是静默猜测。

### 4.7 Task Contract

Task Contract 是不可变、带版本的执行合同。至少包含：

```json
{
  "schema_version": 1,
  "project_id": "...",
  "campaign_id": "...",
  "reality_revision_id": "...",
  "base_sha": "...",
  "objective": "消除重复的认证状态源，同时保持外部行为不变",
  "preconditions": ["现有认证回归测试通过"],
  "write_scope": ["internal/auth/**", "tests/auth/**"],
  "forbidden_scope": ["migrations/**"],
  "invariants": ["未认证请求仍返回既有错误契约"],
  "acceptance": [
    {"kind": "command", "command": "go test ./internal/auth/..."},
    {"kind": "semantic", "claim": "认证状态只有一个拥有层"}
  ],
  "dependencies": [],
  "stop_conditions": ["需要改变公开 API"],
  "escalation": "暂停并创建 change meeting"
}
```

合同一旦 dispatch 不能原地修改。需求变化必须创建新版本；旧 run 明确变为 stale、cancelled 或 superseded。

### 4.8 Worktree Lease

Worktree Lease 表示一个写 Agent 在某个任务、分支和路径上的临时独占权。

它不是心跳存在就算成功的软状态，而是带 fencing token 的持久租约。过期的旧进程即使恢复，也不能继续提交状态或宣称完成。

核心字段：

- project、campaign、task、agent run。
- worktree path、branch、base SHA。
- allowed write scope。
- monotonically increasing fencing token。
- holder、heartbeat、expires_at。
- `provisioning / active / submitted / released / quarantined / failed`。

### 4.9 Evidence 与 Trust State

Evidence 是一个带来源和 digest 的可验证结果，例如：

- 某个 SHA 上一条命令的退出码和完整日志。
- 一个失败用例从 red 到 green 的记录。
- mutation testing 的 surviving mutant。
- 静态边界或依赖图的变化。
- 线上或 staged rollout 指标。
- 独立 Reviewer 对某个语义 claim 的分析及引用。

Trust State 不是一个总分，也不是一次 Review verdict。它是 Project 当前可预测性的持续视图：

- 哪些不变量有强证据。
- 哪些 Gate 已证明能够失败。
- 哪些模块持续产生 bug、冲突或 surviving mutants。
- 哪些 Reality Revision 已经 stale。
- 哪些未知项正在扩大。

## 5. 总体流程

```text
用户选择本地 Git 仓库
          │
          ▼
建立 Project + Repository Binding
          │
          ▼
读取真实仓库，固定 Reality Revision
          │
          ▼
┌────────────────── Nemeton Meeting ──────────────────┐
│ 人类提出目标                                          │
│ 多模型基于同一 Evidence Pack 独立分析                 │
│ 模型互相质询，显式列出分歧、假设和未知项              │
│ Recorder 编译候选语义、合同与 Improvement Policy     │
│ 人类只选择是否接受新的产品语义                        │
└─────────────────────────┬────────────────────────────┘
                          ▼
                 激活版本化语义
                          │
                          ▼
                 创建动态 Campaign
                          │
                          ▼
            计算依赖与可并行写入边界
                ┌─────────┴─────────┐
                ▼                   ▼
          worktree A          worktree B
          Agent run A         Agent run B
                │                   │
                └─────────┬─────────┘
                          ▼
                 Task Gate + Evidence
                          │
                          ▼
                    串行 Merge Queue
                          │
                          ▼
            Integration Gate / staged rollout
                          │
            ┌─────────────┴─────────────┐
            ▼                           ▼
      达成政策停止条件             发现失败或新未知
            │                           │
            ▼                           ▼
         Campaign 完成        更新 Trust State 并递归改进
                                        │
                              超出政策时回到 Meeting
```

## 6. 技术选型

### 6.1 后端：Go

Go 是第一版核心后端的确定选型，原因不是模型推理速度，而是 Nemeton 的主要复杂度位于控制平面：

- 长生命周期 daemon。
- 多个 Git 和 Agent 子进程的启动、流式输出、取消和回收。
- worktree 与语义写入租约。
- 并发状态机和崩溃恢复。
- 单机部署和尽量少的运行时依赖。
- 清晰的边界类型和 fail-fast 错误传播。

Python 可以很好地并行管理子进程，也更适合快速试验模型算法，但这不是第一版的主要瓶颈。第一版不维护 Go 控制面加 Python 编排面的双运行时。

如果未来需要专门的代码图、embedding、mutation 或研究型分析器，可以把它实现成带版本、带输入输出 schema、失败可见的外部 Tool，由 Go Runner 调用；它不能成为静默 fallback。

### 6.2 形态：模块化单体

一个 `nemetond` 进程拥有：

- 本地 HTTP API 和 SSE 事件流。
- SQLite 写入权。
- Project/Meeting/Campaign 状态机。
- Git/worktree 管理。
- Agent/Model 进程监督。
- Gate 执行与证据保存。

一个 `nemeton` CLI 用于初始化、诊断、脚本化和无 UI 验收。未来会议 UI 通过同一 API 接入。

不拆微服务。模块边界通过 Go package 和领域 command 保持。

### 6.3 数据库：SQLite

SQLite 数据库只放在本机文件系统。daemon 是唯一逻辑写入者，使用 WAL、短事务和显式 busy timeout；不支持把数据库放到 NFS 或同步盘。

语义控制面使用选择性事件溯源。append-only `domain_events` 保存 Project、Reality、Meeting、Semantic Item、Campaign、Task Contract、Gate 和 Evidence 的领域事实；关系表、`NOW.md`、Meeting Result、Trust State 和 UI 白板都是可重建投影。

PID、SSE 连接、临时锁、heartbeat 当前值和可以从 Git 或进程表重新探测的现场不依赖事件 replay 恢复。daemon 启动后读取外部现实，完成 reconcile，再追加新的 recovery event。

事件追加和数据库投影更新发生在同一个 SQLite 事务中。文件投影在提交后通过临时文件、`fsync` 和原子 rename 生成；失败时保留 `projection_dirty`，不能回滚或伪造已经提交的领域事实。

replay 只能运行确定性 reducer，不能重新执行 Git 写命令、模型调用、外部 API、PR 操作或 Gate command。

按照仓库约定，数据库关系由应用服务在事务中维护，第一版不创建数据库 foreign key。每个关联 ID 都建立必要索引，删除和级联由拥有该聚合的服务显式完成。

### 6.4 Git：调用系统 Git

不使用 go-git 重新实现 Git 语义。所有命令都使用参数数组调用，不拼接 shell 字符串。命令保存：

- 可执行文件和参数。
- cwd。
- start/end time。
- exit code。
- stdout/stderr artifact。

Git metadata 变更使用 repository-scoped OS lock 串行化；Agent 在已经创建好的不同 worktree 中并行运行。

## 7. 存储所有权

### 7.1 事件流拥有语义历史

本机数据库中的领域事件拥有：

- Project 注册、重新绑定和 Reality Revision。
- 会议原始消息和模型运行记录。
- 尚未激活的候选语义。
- Human Decision、Semantic Item 版本和 supersession。
- Campaign、Task Contract 和领域状态迁移。
- Gate Result、Evidence 和 Trust Finding。

关系表保存当前投影和运行协调状态。大正文、日志、补丁、测试报告和已激活语义文件按 digest 存入 content-addressed artifact store。

### 7.2 Git 仓库发布已批准的静态语义

已激活、应随代码传播的项目语义放在仓库：

```text
.nemeton/
├── project.yaml              # schema version、project identity、integration branch
├── charter.md                # mission、direction、长期边界和不变量
├── gates.yaml                # 可执行 Gate 与证明义务
├── decisions/
│   └── <semantic-item-id>.md # 已激活决策，可被后续条目 supersede
└── policies/
    └── <policy-id>.yaml      # 可复用 Improvement Policy
```

所有权必须唯一：

- `proposed/selected` 内容由事件和其引用的 artifact 拥有。
- `active` 内容由 `SemanticItemActivated` 事件指定的 artifact digest 拥有。
- 目标 Git revision 中的 `.nemeton/` 是已发布投影，供代码、Agent 和 CI 读取。
- SQLite 当前关系和 Markdown 文档都不能成为可独立编辑的第二事实源。

用户在会议中选择条目后，Nemeton 在专用 governance worktree 中生成语义变更。变更进入 integration branch 后才成为 active。这样不会产生“数据库已经批准，但仓库中的 Agent 看不到”的双重事实。

仓库语义文件丢失后，projector 可以根据激活事件和 artifact digest 重新生成。源代码恢复仍然依赖 Git remote、Git bundle、commit objects、patch 或 delivery artifact。

## 8. SQLite 逻辑模型

字段名称可以在实现前由 Babel 调整，但聚合边界和不变量不可丢失。

### 8.1 Project 与 Reality

| 表 | 关键字段 | 约束 |
| --- | --- | --- |
| `projects` | `id`, `name`, `status`, timestamps | `id` 稳定，不来自路径 |
| `repository_bindings` | `project_id`, `display_path`, `canonical_root`, `git_common_dir`, `remote_identity`, `integration_branch`, `managed_worktree_root` | v1 每个 Project 一条 active binding |
| `reality_revisions` | `id`, `project_id`, `commit_sha`, `tree_sha`, `manifest_digest`, `dirty_observed`, `status`, timestamps | immutable；状态为 current/stale/invalidated |
| `reality_artifacts` | `reality_revision_id`, `kind`, `path_or_uri`, `digest`, `metadata_json` | 保存清单，不复制仓库正文 |

### 8.2 Meeting 与语义

| 表 | 关键字段 | 约束 |
| --- | --- | --- |
| `meetings` | `id`, `project_id`, `reality_revision_id`, `kind`, `title`, `status`, timestamps | 所有讨论固定到 Reality Revision |
| `meeting_messages` | `id`, `meeting_id`, `author_kind`, `author_id`, `model`, `content`, `created_at` | append-only |
| `message_evidence` | `message_id`, `evidence_id`, `relation` | 引用必须可追踪 |
| `semantic_items` | `id`, `project_id`, `meeting_id`, `kind`, `scope`, `statement`, `rationale`, `status`, `supersedes_id`, timestamps | active 正文只索引 Git artifact |
| `semantic_sources` | `semantic_item_id`, `message_id`, `evidence_id` | 候选必须能回到原始上下文 |
| `semantic_writer_leases` | `meeting_id`, `holder_run_id`, `fencing_token`, `expires_at`, `status` | 一个会议同一时刻只有一个有效 Recorder |

### 8.3 Campaign 与执行

| 表 | 关键字段 | 约束 |
| --- | --- | --- |
| `campaigns` | `id`, `project_id`, `meeting_id`, `reality_revision_id`, `goal`, `policy_ref`, `status`, budget fields | 动态任务图的根 |
| `task_contracts` | `id`, `campaign_id`, `version`, `contract_json`, `contract_digest`, `status`, timestamps | dispatch 后 immutable |
| `task_dependencies` | `task_contract_id`, `depends_on_task_id`, `kind` | 创建时拒绝环 |
| `agent_runs` | `id`, `role`, `provider`, `model`, `session_id`, `input_digest`, `status`, timestamps | 失败必须保存真实原因 |
| `worktree_leases` | `id`, `task_contract_id`, `agent_run_id`, `path`, `branch`, `base_sha`, `fencing_token`, `status`, heartbeat fields | active path、branch、task 唯一 |
| `deliveries` | `task_contract_id`, `head_sha`, `remote`, `pr_number`, `status`, merge fields | remote 可为空 |

### 8.4 事件、投影与 Artifact

| 表 | 关键字段 | 约束 |
| --- | --- | --- |
| `project_streams` | `project_id`, `current_sequence`, timestamps | 每个 Project 的 sequence 单调递增 |
| `domain_events` | `event_id`, `project_id`, `sequence`, `aggregate_type`, `aggregate_id`, `event_type`, `schema_version`, `payload_json`, `actor`, `causation_id`, `correlation_id`, timestamps | append-only；`project_id + sequence` 唯一 |
| `event_artifacts` | `event_id`, `artifact_digest`, `relation` | 大正文和运行产物通过 digest 引用 |
| `projection_states` | `project_id`, `projection_name`, `projected_through_sequence`, `schema_version`, `result_digest`, `dirty` | 标明每个投影的重放进度 |
| `artifacts` | `digest`, `media_type`, `size`, `storage_uri`, timestamps | 内容寻址；读取时校验 digest |

### 8.5 Gate、Evidence 与信任

| 表 | 关键字段 | 约束 |
| --- | --- | --- |
| `gate_definitions` | `id`, `project_id`, `name`, `kind`, `definition_json`, `definition_digest`, `status` | Gate 定义版本化 |
| `gate_runs` | `id`, `gate_definition_id`, `target_sha`, `task_id`, `status`, result fields | 结果只对 target SHA 有效 |
| `evidence` | `id`, `project_id`, `kind`, `claim`, `artifact_uri`, `digest`, `producer_run_id`, `target_sha`, timestamps | artifact 必须可校验 |
| `trust_findings` | `id`, `project_id`, `scope`, `signal`, `severity`, `status`, evidence refs | 不压成单一分数 |

JSON 字段必须带 schema version，并在 HTTP、Provider、数据库反序列化等边界集中校验。边界之后使用已经验证的领域类型，不在每个内部函数重复防御。

## 9. 状态机

### 9.1 Meeting

```text
draft ─► open ─► deliberating ─► awaiting_human ─► concluded
           │           │                │
           ├──────────► failed          └──► deliberating
           └──────────► cancelled

Reality 变化后：open/deliberating/awaiting_human ─► stale
```

stale 不代表全部结论自动错误，但禁止直接据此 dispatch。必须建立新 Reality Revision，并由 Recorder 标注哪些结论仍成立、哪些需要重议。

### 9.2 Task

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

### 9.3 Worktree Lease

```text
provisioning ─► active ─► submitted ─► released
      │             │           │
      └──► failed   ├──► expired
                    └──► quarantined
```

任何无法证明安全清理的 worktree 进入 quarantined，不能被自动删除或复用。

## 10. 会议协议

### 10.1 Evidence Pack

每个参与模型收到同一份最小 Evidence Pack：

- 用户原始目标，保持原文。
- Reality Revision ID、commit SHA 和关键清单。
- active project semantics。
- 与议题直接相关的代码、文档、测试和历史 finding。
- 已确认事实、用户意图、系统推断和未知项的明确分区。

模型可以请求更多证据，但新增证据必须记录引用和 digest。不能把某个模型的总结冒充仓库事实。

### 10.2 Deliberation

推荐流程：

1. 默认由 `Designer`、`Maintainer`、`Adversary` 三个平等 Design Agent 独立给出完整问题模型、证据和方案，降低早期锚定；Recorder 和 Verifier 不计入这三个设计席位。三个只是默认值，会议参与者、Agent Run、Proposal、Review Assignment 和 Ratification Assignment 都必须按 `0..N` 建模，不得硬编码三个字段或三个并发槽。
2. 系统聚类一致点和冲突点，不用多数票消除少数意见。
3. 模型针对最关键分歧交叉质询，要求指出可证伪条件。
4. Reviewer 检查是否遗漏约束、失败模式和验收能力。
5. Recorder 只编译，不自行裁决新的产品价值取舍。
6. 用户看到的是候选条目、支持/反对证据、后果和未知项，而不是冗长聊天记录。

Design Agent 水平扩展时，不能让每个 Agent Review 其他所有 Proposal。系统按固定 review degree 分配 steelman/challenge/evidence check，并按 Decision/Conflict 聚类讨论，使调用量从全连接的 `O(N²)` 互评降为近似 `O(N × K)`；`K` 是每个 Proposal 的固定覆盖数。所有 Proposal 对会议公开可访问，但只把与当前 Conflict 相关的内容放进 Agent 本轮上下文。

### 10.3 Human in the Loop

人类操作统一发生在 Meeting：

- 接受、拒绝、暂缓或改写候选语义。
- 在多个不可兼得的产品方向之间选择。
- 批准无法由既有 Improvement Policy 推导的扩展。
- 判断某种高熵是设计需要还是腐化。

人类不作为默认的逐行代码 Gate，也不需要确认 Agent 的每个局部技术动作。

## 11. Worktree 并行模型

### 11.1 注册项目

用户执行等价于：

```text
nemeton project open /absolute/path/to/repo
```

后端依次：

1. 规范化路径并验证它是目录。
2. 调用 Git 获取 top-level、common-dir、HEAD、tree、branch、remotes 和 worktree list。
3. 如果 common-dir 已绑定 Project，返回既有 Project，而不是创建重复项目。
4. 如果是新仓库，生成 Project ID，选择或要求确认 integration branch。
5. 在操作系统应用数据目录中创建受控 worktree root；不使用仓库根、用户目录根或宽泛目录。
6. 建立 Reality Revision。当前 checkout 的 dirty 内容只记录，不自动复制到执行基线。
7. 不修改当前 checkout，不自动写 `.nemeton/`，直到用户在会议中确认首次语义激活。

### 11.2 创建任务 worktree

1. Task Contract 必须是 dispatchable，依赖已满足，Reality 未失效。
2. 获取 repository-scoped Git metadata lock。
3. 解析准确 base SHA；不因 fetch 失败而静默使用未知旧版本。
4. 创建唯一分支 `nemeton/<campaign-short>/<task-short>`。
5. 在 Nemeton 管理目录下创建 worktree。
6. 写入只属于本次 run 的合同投影和 runtime metadata；避免污染业务仓库提交。
7. 创建带 fencing token 的 active Worktree Lease。
8. 释放 Git metadata lock，Agent 在 worktree 内运行。

多个 Agent 的代码执行可以并行；`worktree add/remove/prune/fetch` 等共享 Git metadata 操作短暂串行。

### 11.3 写入边界

并行不只看文件是否不同。Task Contract 必须声明职责边界和 `write_scope`：

- 调度前，重叠的 write scope 默认不能并行，除非会议明确证明共享写入协议。
- 运行后，把真实 diff 与 write scope 比较。
- 超出范围不是自动丢弃，而是 failed gate 或 quarantined，要求重新规划。
- 两个分支没有文本 conflict 也可能有语义 conflict，Integration Gate 必须验证组合状态。

### 11.4 合并队列

- PR 可以并行产生，进入 integration branch 的操作串行。
- 每次合并前验证 head、base、合同和 Gate digest。
- 每次合并后生成新的 integration SHA。
- 尚未合并任务重新判断 base 假设；需要时 rebase 并重跑受影响 Gate。
- 不把各分支分别 green 推导为组合状态 green。

### 11.5 清理

只有同时满足以下条件才自动释放 worktree：

- 路径位于 Nemeton 记录的 managed worktree root。
- lease token 与当前 holder 一致。
- worktree 的 Git identity、branch 和 task 匹配数据库记录。
- 没有未捕获的变更和未保存证据。
- delivery 已合并、取消或由明确政策终止。

否则进入 quarantined 并保留现场。Nemeton 永不递归删除用户选择的主 checkout。

## 12. Gate 和持续信任

### 12.1 Gate 层级

1. **Contract Gate**：合同字段完整，Reality、base SHA、依赖和写入边界有效。
2. **Task Gate**：任务自己的测试、类型、静态检查和语义验收。
3. **Diff Boundary Gate**：实际修改没有越过合同授权。
4. **Integration Gate**：合并后的组合系统满足不变量。
5. **Release Gate**：不可逆或高风险变更经过 staged rollout、监控或人工语义决定。

Reviewer Agent 可以提供证据和反例，但“Reviewer 说没问题”不能单独构成 hard gate。

### 12.2 Hard Check 标准

一个 check 只有证明过自己能够失败才值得信任：

- assertion 针对真实可观察行为，而不是实现形状。
- 覆盖行为实际经过的关键路径。
- 失败输出能定位具体不变量。
- 保存 target SHA、输入 digest 和执行环境。
- 对关键测试可以用 mutation 或受控 fault 证明它会变红。

### 12.3 失败闭环

```text
Gate/运行失败
      │
      ▼
确定失败对应的不变量和证据
      │
      ▼
追溯：目标、合同、边界、环境、测试面还是实现错误？
      │
      ├── 政策范围内 ─► 生成下一轮任务
      │
      └── 新产品语义 ─► 暂停并回到 Meeting
```

失败不是只给模型打分。它首先是系统输入不够干净的诊断信号。

## 13. 崩溃恢复与失败语义

daemon 启动时执行只读 reconcile：

- 读取数据库中非终态 meeting、campaign、task、run 和 lease。
- 调用 `git worktree list --porcelain` 对账真实 worktree。
- 检查 Agent 子进程是否仍存在，不能仅凭旧 PID 认领进程。
- 检查 branch/head/diff 与记录是否一致。
- 对失去 holder 的 active lease 使用新的 fencing token 终止旧写权限。
- 无法证明归属的目录或分支进入 quarantined。
- 生成显式 recovery event 和 finding，不静默归一化成成功。

边界错误必须 fail-fast、fail-loud：

- 模型 Provider 不可用：对应 run 失败，会议显示缺席；不伪造意见。
- Git fetch 失败：若任务要求远端最新版本则停止；若合同明确固定本地 SHA，可以继续并记录这一事实。
- Gate command 不存在：Gate 失败；不能跳过。
- schema 无法解析：拒绝输入；不能使用默认空合同。
- Evidence artifact 丢失或 digest 不匹配：证据无效，相关 verdict 失效。

## 14. 后端代码结构

第一版建议：

```text
nemeton/
├── cmd/
│   ├── nemeton/              # CLI
│   └── nemetond/             # local daemon
├── internal/
│   ├── project/              # Project、Repository Binding、Reality
│   ├── meeting/              # 消息、deliberation、Recorder、候选语义
│   ├── semantics/            # 激活、版本化和仓库投影
│   ├── campaign/             # 动态任务图和 Improvement Policy
│   ├── worktree/             # Git 管理、lease 和 reconcile
│   ├── runner/               # Model/Agent/command 子进程监督
│   ├── gate/                 # Gate 编译、执行和 Evidence
│   ├── trust/                # finding 和 Trust State 投影
│   ├── projection/           # 确定性 reducer 和文件 projector
│   ├── artifact/             # content-addressed artifacts
│   ├── store/                # SQLite、迁移和事务
│   ├── api/                  # HTTP/SSE 边界 schema
│   └── events/               # append-only 领域事件和 stream revision
├── migrations/
├── schemas/                  # versioned JSON schema
├── web/                      # 后续会议 UI；不阻塞最小内核
└── docs/
```

不要为每张表创建一层无意义 repository interface。领域服务拥有事务和不变量，外部 Provider、Git command runner、时钟等真正边界再使用窄接口。

## 15. 最小 API 和 CLI

API 名称是初稿，可以在实现设计中调整：

```text
POST /v1/projects/open
GET  /v1/projects/{id}
POST /v1/projects/{id}/reality-revisions
POST /v1/projects/{id}/replay

POST /v1/projects/{id}/meetings
POST /v1/meetings/{id}/messages
POST /v1/meetings/{id}/deliberations
GET  /v1/meetings/{id}/candidates
POST /v1/semantic-items/{id}/select
POST /v1/semantic-items/{id}/reject

POST /v1/meetings/{id}/campaigns
POST /v1/tasks/{id}/dispatch
POST /v1/tasks/{id}/cancel
GET  /v1/campaigns/{id}

GET  /v1/events                         # SSE
GET  /v1/projects/{id}/trust-state
```

CLI 最小命令：

```text
nemeton daemon start
nemeton project open <path>
nemeton project inspect <project-id>
nemeton project replay <project-id>
nemeton meeting create <project-id> --title <title>
nemeton meeting show <meeting-id>
nemeton doctor
```

Agent dispatch、合并和 destructive cleanup 在状态机稳定前不急着暴露为随意可调用的底层 CLI。

## 16. 自举实施顺序

### Milestone 0：仓库与持久化内核

目标：可靠地认识“这是哪个项目”，而不修改项目。

交付：

- Go module、`nemetond`、`nemeton`。
- SQLite migrations、domain event append、reducer 和 projection state。
- `project open`、Git identity discovery、Repository Binding。
- Reality Revision 的最小版本：SHA、tree、dirty、文档与检查清单。
- content-addressed artifact store 的最小写入与 digest 校验。
- 删除 Project/Reality 投影后的 replay。
- daemon single-instance lock 和启动 reconcile 骨架。

验收：

- 从主 checkout 和任意 linked worktree 打开，返回同一 Project。
- 移动展示路径后可以显式 relink，不改变 Project ID。
- dirty checkout 可以注册，但 dirty 内容不会被误写为 committed Reality。
- 非 Git 目录、损坏 common-dir、歧义 integration branch 明确失败。
- 重启后 Project 和 Reality 状态一致。
- 删除 Project/Reality 投影后，replay 生成相同规范化内容和 digest。
- replay 不执行 Git 写入、模型调用、外部 API 或其他副作用。
- 全流程不修改所选仓库。

### Milestone 1：持久会议与候选语义

目标：让会议成为真实的语义编译空间，而不是普通聊天。

交付：

- Meeting、Message、Evidence 引用和 SSE。
- 至少两个 Model Provider adapter 的真实调用边界。
- 默认三个 Design Agent 的独立首轮、交叉质询、Recorder 和 Verifier。
- Semantic Item candidate、用户 select/reject/defer。
- semantic writer lease 和 fencing token。

验收：

- 所有模型看到相同 Reality Revision 和 Evidence Pack。
- Provider 失败时该席位明确失败，会议仍保留完整事实，不能生成假意见。
- 每个候选能追溯到人类发言、模型论证和证据。
- 同一会议两个 Recorder 不能同时写候选。
- Reality 改变后会议变 stale，不能直接 dispatch。

### Milestone 2：语义激活与单任务执行

目标：从会议结论走通一次真实代码变更。

交付：

- governance worktree 和 `.nemeton/` 静态语义投影。
- Task Contract schema 与编译。
- 单个 task worktree、Agent Runner、日志和取消。
- Task Gate、Diff Boundary Gate、Evidence。
- 本地 commit；配置 remote 时可生成 PR。

验收：

- 主 checkout 始终不被修改。
- selected 语义在进入 integration branch 前不显示为 active。
- Agent 只能在持有有效 lease 时报告状态。
- 超出 write scope 的变更明确失败或 quarantined。
- Gate 结果绑定准确 target SHA。
- daemon 崩溃后能恢复或隔离 worktree，不产生假成功。

到这里 Nemeton 具备最小自举能力：后续 Milestone 应优先由 Nemeton 自己创建会议、合同、worktree 和证据完成。

### Milestone 3：并行与合并队列

目标：在语义独立时获得真实并行收益。

交付：

- Campaign、Task dependency graph。
- 多 worktree lease 和并行 Agent run。
- write scope 冲突检查。
- PR/delivery tracking 和串行 merge queue。
- merge 后 rebase、失效分析和 Integration Gate。

验收：

- 两个独立任务能够同时运行并产出独立分支。
- 重叠写入边界默认阻止并行。
- 每次 merge 后未完成任务重新校验 base 假设。
- 分支分别 green、组合失败时 Campaign 不能完成。

### Milestone 4：递归 Campaign 与 Trust State

目标：支持大型重构和长轮次项目，而不是开完会只执行固定计划。

交付：

- Improvement Policy schema。
- 结果驱动的动态任务生成。
- Trust finding 聚类：bug、冲突、mutation、边界漂移、未知项。
- continue/stop/rollback/escalate 判定。
- 关键 Gate 的“能够失败”证明。

验收：

- Campaign 可以在一次 merge 后依据新证据生成下一轮任务。
- 改进没有达到政策指标时不能仅因测试 green 而停止。
- 超过预算或出现新产品语义时自动暂停并创建会议议题。
- 每个失败都能追溯到合同、边界、环境、验证能力或实现中的至少一个明确位置。

## 17. 第一轮 Babel 任务边界

Babel 第一次进入仓库时，不应直接尝试实现完整 Nemeton。第一轮只处理 **Milestone 0**，并在实现前完成以下核对：

1. 确认 Go module path、支持平台和最低 Git 版本。
2. 确认本机数据目录策略、数据库文件和 managed worktree root 的安全边界。
3. 确认 Project ID 是否立即写入 `.nemeton/project.yaml`。本文推荐首次语义激活前不写仓库，因此 Milestone 0 仅以本机数据库为准。
4. 确认 integration branch 解析规则；无法唯一判断时要求用户选择，不能猜测。
5. 把 Milestone 0 转换成明确任务合同、测试计划和只读 Review 清单。
6. 只实现 Project、Repository Binding、Reality Revision、SQLite/Event/Replay 和 CLI/daemon 最小路径。

Milestone 0 不创建 worktree、不调用模型、不构建会议 UI、不创建 PR。它的价值是先建立后续所有讨论和并行执行共同依赖的“项目现实”。

## 18. 待第一次会议确认的问题

这些问题不能由实现 Agent 静默决定：

1. 第一版支持 Linux only，还是同时保证 macOS？Windows 是否明确延后？
2. Nemeton 自身的 Go module path 和未来 Git remote 是什么？
3. 用户选择语义条目后，是自动创建 governance commit，还是必须生成 PR 后再激活？
4. 第一批 Model Provider 是 Codex CLI、Claude Code CLI，还是直接 HTTP API？
5. 第一批执行 Agent 是否也只支持上述两个 CLI？
6. 本地无 remote 的项目是否允许完整 Campaign，只把 PR/merge 改成本地 commit/integration？本文建议允许。
7. 默认是否完全禁止两个 write scope 重叠任务并行？本文建议第一版禁止。
8. `.nemeton/charter.md` 是一份编译后的文档，还是按条目拆分？本文建议 charter 聚合长期内容，decision 按条目拆分。

## 19. 已拒绝的替代方案

### 把本地路径直接作为 Project

拒绝。路径会移动，不同 worktree 路径不同，会把同一项目错误拆成多个身份。

### 让 Agent 直接在用户 checkout 中运行

拒绝作为默认路径。它无法安全并行，会混入 dirty state，并把用户工作区变成运行时资源。

### 从第一天支持多仓库 Workspace

拒绝。跨仓库原子性、依赖、权限、Gate 和合并语义会扩大第一版边界，妨碍验证核心命题。

### Python 作为核心编排后端

暂不选择。Python 足以实现，但 Nemeton 当前风险集中在长期 daemon、子进程生命周期、并发状态机、租约和单机交付。Go 更符合第一版控制平面的主要成本。未来 Python 只作为显式工具边界进入。

### 把全部运行现场做成 Event Sourcing

拒绝。语义控制面依赖事件 replay；PID、SSE 连接、临时锁、heartbeat 当前值和可重新探测的 Git/进程现场由 reconcile 恢复。外部副作用不能随事件重放再次执行。

### LLM Reviewer 作为最终 Gate

拒绝。Reviewer 产生证据、反例和语义分析；确定性检查、系统行为和可验证不变量决定 hard gate。

## 20. 成功标准

Nemeton v1 的成功不是“同时启动了多少 Agent”，而是：

- 每个 Agent 都知道自己基于哪个项目版本、为什么修改、允许修改什么。
- 讨论中的事实、推断、决定和未知项不会混在一起。
- 人类只在真正需要产品判断的地方介入。
- 多 worktree 并行不会共享写入所有权或制造不可见的语义冲突。
- 每个结果都能回到合同、代码 SHA、Gate 和证据。
- `NOW.md`、Meeting Result、Trust State 和静态语义投影删除后能够从事件和 artifact 重建。
- 失败会使系统输入和边界变得更清晰，而不是只触发“再 Review 一次”。
- daemon 崩溃、Provider 失败、Git 状态漂移都不会被包装成成功。
- Nemeton 能用自己的会议、合同、worktree 和 Gate 完成自身后续里程碑。

当这些性质成立时，Nemeton 才真正占据了“Agent 原生项目的本地语义控制平面”这个生态位。
