# Nemeton Swarm Meeting：时序、收敛与会议结果设计

> 状态：Approved for staged implementation
>
> 日期：2026-07-19
>
> 关联文档：`2026-07-19-nemeton-v1-design.md`
>
> 目的：定义多个 Agent 如何在相对平等的环境中独立设计、自由讨论、形成可审计的统一方案，并把最后拍板权留给人类。

## 1. 结论

Nemeton Meeting 不采用永久主 Agent，也不采用无约束群聊。推荐协议是：

```text
共同事实冻结
    ↓
平等、密封、并行的独立设计
    ↓
匿名公开全部设计
    ↓
围绕 Claim 和 Conflict 的自由讨论
    ↓
单 Recorder 生成 Synthesis Draft
    ↓
按策略覆盖 Decision 的逐项 Ratification
    ↓
Convergence Gate 判断技术上是否已经收敛
    ↓
生成一份供人类拍板的 Meeting Result
    ↓
人类接受、修改、选择或要求继续讨论
```

这里的“统一”不是所有 Agent 表态同意，也不是多数票胜出。统一表示：

- 已经有一套内部一致、可实施、可验收的 Canonical Design Draft。
- 所有事实冲突和硬约束冲突已经解决，或明确标为需要实验。
- 剩余分歧只属于人类价值取舍、风险偏好或当前未知项。
- 少数意见没有被抹掉，而是随会议结果一起呈现。

Recorder 负责生成会议结果，但没有权力宣布结论正确。Convergence Gate 由确定性 host code 执行；人类拥有最终激活权。

## 2. 角色和权力边界

### 2.1 Facilitator

Facilitator 是确定性程序，不是 LLM 主持人。它负责：

- 固定会议问题、Reality Revision 和评价标准。
- 控制时序、并行 barrier、超时和最大轮数。
- 保证每个 Proposal 得到展示、steelman、challenge 和 evidence check。
- 选择当前要讨论的 Conflict。
- 调用 Convergence Gate。

它不能提出设计或决定哪套设计更好。

### 2.2 Design Agents

第一版默认使用三个设计席位；这是已确认的产品决定，不再作为首次会议的待选项。三个是默认基数，不是容量上限：

- `Designer`：完整架构、数据流和所有权。
- `Maintainer`：演化、迁移、恢复和长期复杂度。
- `Adversary`：反例、失败路径和隐含假设。

所有设计席位使用同一参与者数据模型并拥有相同提案权，没有“核心成员”。可以使用不同模型，也可以使用同一模型的不同会话和角色提示；异构模型优先。会议可以增加任意数量的 Design Seat，而不修改 schema、API 或状态机。

### 2.3 Verifier

Verifier 不提出第四套完整设计。它检查：

- Claim 是否有 Evidence。
- 方案是否遗漏硬约束。
- 验收标准是否真的能够失败。
- Synthesis 是否悄悄删除异议或引入新决定。
- 多个 Decision 组合后是否矛盾。

### 2.4 Recorder

Recorder 是唯一的 synthesis writer，持有带 fencing token 的语义写入租约。它负责把讨论编译成结构化 Synthesis Draft 和 Meeting Result。

Recorder 可以组合已有决定，不能：

- 发明讨论中没有出现的新产品语义。
- 用多数票覆盖有证据的少数意见。
- 把模型信心当成事实。
- 把尚未解决的问题写成已解决。

### 2.5 Human

人类只处理：

- 产品方向和价值取舍。
- 不可兼得方案之间的选择。
- 风险承受范围。
- 是否接受 Candidate Semantic Items。
- 是否要求继续调查或实验。

## 3. 完整时序

```text
Human        Facilitator      Design A/B/C       Extractor       Recorder       Verifier
  │               │                │                 │               │              │
  │─提出问题──────►│                │                 │               │              │
  │               │─固定 Reality──►│                 │               │              │
  │               │─密封并行设计──►│                 │               │              │
  │               │                │─Proposal A/B/C─►│               │              │
  │               │                │                 │─规范化 Claim─►│              │
  │               │◄────────────── Proposal Reveal ──────────────────│              │
  │               │─并行首轮评议──►│                 │               │              │
  │               │                │─Question/Critique/Evidence─────►│              │
  │               │                │                 │─Conflict Map─►│              │
  │               │─聚焦分歧讨论──►│                 │               │              │
  │               │                │─Amendment/Dissent──────────────►│              │
  │               │                │                 │               │─Draft────────►│
  │               │                │                 │               │◄─Audit────────│
  │               │─Ratification──►│                 │               │              │
  │               │                │─逐项 verdict───────────────────►│              │
  │               │─Convergence Gate─────────────────────────────────►│              │
  │◄──────────────────── Meeting Result / Human Choices ─────────────│              │
  │─拍板或要求重议►│                │                 │               │              │
```

## 4. Phase 0：会议宪法和共同事实

### 输入

- 用户原始发言，保持原文。
- Project ID、Reality Revision、commit SHA。
- Active Semantic Items。
- 与问题直接相关的代码、文档、测试、CI 和历史 Evidence。
- 明确分开的 `facts / user_intent / inference / unknowns`。
- 会议评价标准和硬约束。

### 输出

`Meeting Brief`：

```json
{
  "question": "Swarm 应该采用永久 Lead 还是平等提案？",
  "reality_revision_id": "reality-17",
  "hard_constraints": [
    "最终只能有一份 Canonical Design",
    "人类拥有产品语义激活权",
    "Agent 异议必须可追踪"
  ],
  "evaluation_criteria": [
    "independence",
    "coherence",
    "evidence_traceability",
    "implementation_cost",
    "failure_recovery"
  ],
  "unknowns": []
}
```

### Gate

- Reality Revision 可读取且没有失效。
- 问题没有在 Evidence Pack 中被偷偷改写。
- 硬约束和评价标准能够被所有席位读取。

## 5. Phase 1：密封式独立设计

Facilitator 并行调用所有 Design Agent。每个 Agent 只能看到 Meeting Brief，不能看到其他 Agent 的输出。

每份 Proposal 使用同一 schema：

```json
{
  "proposal_id": "proposal-a",
  "problem_model": "...",
  "design_summary": "...",
  "decisions": [],
  "claims": [],
  "evidence_refs": [],
  "assumptions": [],
  "failure_modes": [],
  "migration": [],
  "validation": [],
  "rejected_alternatives": [],
  "unknowns": []
}
```

### 为什么必须密封

- 防止第一位发言者定义所有人的问题框架。
- 保留真正不同的问题模型。
- 让每个席位承担提出完整设计的责任，而不是只评论别人。

### Gate

- 每个席位都提交完整 Proposal 或明确失败。
- Provider 失败记录为缺失席位，不生成替代意见。
- Proposal 不能直接写入 Canonical Design。

## 6. Phase 2：规范化和 Reveal

Extractor 把 Proposal 中的内容拆成稳定对象：

- `Decision`：一个可选择的设计决定。
- `Claim`：支持决定的事实或推断。
- `EvidenceRef`：可核对来源。
- `Assumption`：当前未证明的前提。
- `Risk`：可能的失败路径。
- `Unknown`：当前不能回答的问题。

原作者必须确认规范化结果没有误解自己的 Proposal。确认只针对“是否忠实表达”，不能趁机看到别人设计后改写原方案。

Reveal 时：

- 全部 Proposal 在同一 barrier 后同时公开并可访问；规模较大时不要求把全部正文复制进每个 Agent 的 prompt。
- 隐藏模型和 Provider 身份。
- 随机排列 Proposal 顺序。
- 保留 Proposal ID 和内部 Decision/Claim ID。

## 7. Phase 3：第一轮自由讨论

默认小会议（`N <= 5`）中，所有 Agent 的输入是：

```text
Meeting Brief
+ 所有匿名 Proposal
+ 所有 Proposal 的 Decision/Claim/Assumption/Risk/Unknown
```

扩展会议（`N > 5`）中，所有 Proposal 仍在 Shared Blackboard 同时公开并可查询，但每个 Agent 的初始输入改为：

```text
Meeting Brief
+ Proposal Index / Decision Cluster Map
+ 自己被分配 Review 的 Proposal 全文
+ 与自己 Proposal 发生直接冲突的 Decision/Claim
```

跨 Cluster 的“所有方案共同遗漏了什么”由专门的 coverage assignment、Shard Recorder 和 Verifier 共同完成，不要求每个 Agent 重复阅读所有全文。

每个 Agent 并行完成：

1. Steelman 一个其他 Proposal。
2. 找出最关键的真实分歧。
3. 针对具体 Claim 提出 challenge 或 counterexample。
4. 小会议指出所有 Proposal 共同遗漏的内容；扩展会议指出自己覆盖范围和相邻 Cluster 共同遗漏的内容。
5. 说明什么 Evidence 会让自己修改原设计。

输出仍可以是自然语言，但系统提取为：

- `question`
- `support`
- `challenge`
- `counterexample`
- `new_evidence`
- `amendment`
- `new_proposal`

每个关键事件必须引用 Proposal、Decision、Claim 或 Evidence ID。

## 8. Phase 4：Conflict Map

Extractor/Recorder 生成 Conflict Map，但不能解决 Conflict。

Conflict 类型：

| 类型 | 例子 | 解决方式 |
| --- | --- | --- |
| `terminology` | 两个名字描述同一拥有层 | 统一术语 |
| `factual` | 是否存在绕过 SessionService 的调用 | 读取真实代码和日志 |
| `testable` | 双写是否会造成竞争 | 实验、测试或原型 |
| `architectural` | 状态应由 DB 还是 Git 拥有 | 对照不变量和所有权原则 |
| `value_tradeoff` | 更快交付还是更强迁移保证 | 人类选择 |
| `complementary` | DB 保存运行态、Git 保存批准语义 | 合并决定 |
| `unknown` | 当前没有足够证据 | 保留或创建调查任务 |

每个 Conflict 包含：

```json
{
  "conflict_id": "conflict-7",
  "type": "architectural",
  "question": "Active semantics 的正文由 DB 还是 Git 拥有？",
  "positions": ["decision-a-3", "decision-c-2"],
  "evidence_refs": [],
  "blocking": true,
  "status": "open"
}
```

## 9. Phase 5：聚焦式自由讨论

此时不再把全部原始会议历史重复传给每个 Agent。Facilitator 按 open blocking Conflict 建立线程，输入：

```text
稳定上下文：Meeting Brief
相关上下文：Conflict + Decisions + Claims + Critiques + 新 Evidence
```

参与者可以自由加入，但至少唤醒：

- Conflict 两侧的原提案者。
- 一个没有提出这两个决定的第三方 Challenger。
- 必要时 Verifier。

每轮允许：

- 提交新 Evidence。
- 修改自己的 Decision，形成 Amendment。
- 承认对方反例并撤回 Claim。
- 把 Conflict 转成 experiment required。
- 明确保留 dissent。

默认最多两轮 focused deliberation。超过轮数仍有 blocking Conflict 时，不制造假共识，转为：

- `needs_evidence`
- `experiment_required`
- `human_choice`

## 10. Phase 6：Synthesis Draft

Recorder 不是从整份 Proposal 中选赢家，而是按设计维度组合 Decision。

Synthesis Draft 必须包含：

- Problem Statement。
- Canonical Decisions。
- 各 Decision 的来源 Proposal、Claim 和 Evidence。
- 组合后的数据流、状态机和所有权。
- 被拒绝的 Alternatives 及原因。
- 未解决的 Dissent、Unknown 和 Human Choices。
- Implementation Outline 和 Acceptance Obligations。

任何无法追溯到讨论内容的 Decision 标记为 `orphan_decision`，不能通过 Verifier。

## 11. Phase 7：Verifier Audit

Verifier 检查：

1. 每个硬约束是否被一个具体 Decision 满足。
2. Decision 之间是否存在所有权、接口、时序或迁移冲突。
3. 每个事实 Claim 是否有可访问 Evidence。
4. 是否删除了有证据的少数意见。
5. Acceptance 是否能够证明失败，而不是只有“测试通过”。
6. Synthesis 是否引入讨论中没有出现的语义。

Verifier 只能产生 finding，不能自行改写 Synthesis。Recorder 根据 finding 生成下一版 Draft。

## 12. Phase 8：逐项 Ratification

每个 Design Agent 看到 Synthesis Draft 和 Verifier Findings，对每个 blocking Decision 返回：

```text
support
support_with_amendment
factual_block
constraint_block
value_dissent
abstain
```

规则：

- `factual_block` 必须引用 Evidence 或明确的可验证路径。
- `constraint_block` 必须指出违反哪个已批准硬约束。
- `value_dissent` 被保留，但不伪装成事实阻塞。
- `abstain` 表示没有足够知识，不计为同意。
- Ratification 不是投票；两个 support 不能抵消一个有效 factual block。

## 13. Convergence Gate

### 13.1 三种结果

```text
technical_convergence
    已有技术上一致、可实施的 Canonical Draft

decision_ready
    技术结构已收敛，只剩明确的人类取舍

not_converged
    仍有事实、硬约束或组合一致性阻塞
```

### 13.2 判定条件

只有同时满足以下条件，才能离开讨论阶段：

1. 每个 implementation-blocking 设计维度都有一个 Canonical Decision，或明确标为 Human Choice/Experiment Required。
2. 没有 unresolved `factual_block`。
3. 没有 unresolved `constraint_block`。
4. 所有硬约束映射到至少一个具体 Decision 和 Acceptance Obligation。
5. Decision Graph 不存在已知接口、所有权、时序和迁移矛盾。
6. Ratification Policy 要求的 Decision 覆盖已经完成；小会议要求所有 active participant，扩展会议要求每个 Decision Cluster 的提案者、支持者和 Challenger 均有代表，同时所有失败/缺席已记录。
7. 所有 blocking dissent 都已分类，不存在“Recorder 忽略了它”的状态。
8. Synthesis 没有 orphan decision。
9. Verifier 没有 unresolved blocking finding。
10. 最后一轮没有出现尚未处理的新 blocking Claim。

可以表示成：

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

不使用单一 convergence score。数值可以用于排序待处理 Conflict，不能替代这些布尔 Gate。

## 14. Human Meeting Result

Convergence Gate 通过后，由 Recorder 生成一个面向人类的会议结果，而不是把整段对话交给人类阅读。

结构：

```json
{
  "meeting_id": "meeting-42",
  "reality_revision_id": "reality-17",
  "status": "decision_ready",
  "question": "...",
  "recommendation": "...",
  "canonical_decisions": [],
  "human_choices": [],
  "dissent": [],
  "unknowns": [],
  "rejected_alternatives": [],
  "evidence_summary": [],
  "implementation_outline": [],
  "acceptance_obligations": [],
  "ratification_summary": [],
  "convergence_report": {}
}
```

人类可以：

- `approve`：激活候选语义并编译 Task Contracts。
- `approve_with_edits`：修改明确条目，再进行一次轻量 Ratification。
- `choose_option`：解决 Human Choice。
- `request_evidence`：创建调查或实验任务。
- `reopen`：带新问题返回 focused deliberation。
- `reject`：保留会议记录，不激活语义。

## 15. 示例：一份 Agent 生成的 Meeting Result

### Meeting

```text
标题：Nemeton Swarm 是否需要永久主 Agent
Reality：reality-17 @ 3f4a9c1
参与席位：Designer / Maintainer / Adversary / Recorder / Verifier
状态：decision_ready
```

### 推荐结论

采用“平等独立提案 + 自由质询 + 单 Recorder 编译 + 逐项 Ratification”的会议协议。探索阶段不设主方案；Convergence Gate 通过后，Synthesis Draft 才成为唯一 Canonical Design 候选。

### Canonical Decisions

#### D-01：第一轮必须独立且密封

- 决定：所有 Design Agent 并行提交完整 Proposal，提交前不可见其他方案。
- 理由：避免先发锚定，保留问题模型多样性。
- 来源：Proposal A、Proposal C；Critique B-2 后修订。
- Evidence：LLM Council 的独立首轮公开实现；会议内 anchoring 风险分析。
- 验收：任一 Design Run 的输入中不得包含其他未 Reveal Proposal。

#### D-02：Reveal 后允许 Agent-to-Agent 自由讨论

- 决定：所有 Proposal 同时匿名公开；Agent 可以自由提问、反驳、支持、修正和提交 Evidence。
- 限制：关键消息必须引用 Proposal/Decision/Claim/Evidence ID。
- 验收：每个 Proposal 至少有一个 steelman、一个 challenge 和一个 evidence check。

#### D-03：不设置永久主设计 Agent

- 决定：Facilitator 为确定性程序；Recorder 只有编译权。
- 理由：固定 Lead 会把问题定义和最终综合集中到同一个模型。
- 验收：Recorder 不能创建没有 source refs 的 Decision。

#### D-04：不按整份 Proposal 选赢家

- 决定：按设计维度和 Decision 进行组合。
- 理由：最佳方案可能由多个 Proposal 的互补决定组成。
- 验收：每个 Canonical Decision 独立记录来源、替代项和 Evidence。

#### D-05：统一由 Gate 判断，不由 Agent 自报

- 决定：使用布尔 Convergence Gate；不使用多数票或单一信心分数。
- 验收：存在 unresolved factual/constraint block 时不能生成 `decision_ready`。

#### D-06：人类拥有最终激活权

- 决定：Meeting Result 只是一份可拍板候选；人类批准后才激活 Semantic Items 和 Task Contracts。
- 验收：未批准会议不能创建 active semantic revision。

#### D-07：v1 默认使用三个 Design Agent

- 决定：每次完整 Swarm Meeting 默认启动 `Designer`、`Maintainer`、`Adversary` 三个平等设计席位。
- 理由：三个席位足以形成独立方案、长期维护视角和反方压力，同时控制协调成本。
- 扩展：高风险会议未来可以显式增加席位，但不能改变 v1 默认值，也不能让新增席位获得更高提案权。
- 验收：默认会议创建后恰好包含三个 Design Agent；Recorder 和 Verifier 不计入设计席位。
- 水平扩展：三个不是 schema 或调度上限；扩展会议增加普通 Design Seat 实例，不增加永久核心角色。

### Human Choices

#### H-02：模型身份何时显示给用户

- Option A：讨论完成前始终匿名。
- Option B：Agent 之间匿名，用户始终可查看真实 Provider。
- 推荐：Agent 输入匿名；用户 UI 可以展开 provenance，但默认不突出身份。

### 保留异议

- Adversary 认为两轮 focused deliberation 对大型架构重构可能不足。
- 该异议不阻塞 v1，因为会议可以转为 `needs_evidence` 或由人类 `reopen`，但应记录轮次不足的观测指标。

### 已拒绝方案

- 永久 Lead 先设计、其他 Agent Review：锚定风险过高。
- 无限制群聊直到“大家同意”：不可终止，且无法证明共识质量。
- 多数投票选择完整 Proposal：会丢失互补决定和正确少数意见。
- Recorder 直接生成任务：缺少 Ratification 和 Convergence Gate。

### Implementation Outline

1. 实现 Meeting/Proposal/Decision/Claim/Conflict 数据模型。
2. 实现 sealed fan-out 和 simultaneous reveal。
3. 实现共享 Discussion Event stream。
4. 实现 Conflict Map 和 focused thread。
5. 实现 Synthesis、Verifier、Ratification。
6. 实现 Convergence Gate 和 Human Meeting Result。

### Convergence Report

```text
canonical_decisions_complete : true
hard_constraints_covered     : true
unresolved_factual_blocks    : 0
unresolved_constraint_blocks : 0
blocking_verifier_findings   : 0
ratification_complete        : true
dissent_classified           : true
human_choices                : 1
result                       : decision_ready
```

这就是“一个 Agent 生成的会议讨论结果”：由 Recorder 输出，但内容包含所有参与者的 Decision、Evidence、异议和 Ratification；它不是 Recorder 自己的一篇总结。

## 16. 已有类似公开实现

### LLM Council

[Karpathy LLM Council](https://github.com/karpathy/llm-council) 已实现非常接近的前三步：多模型独立回答、匿名互评、Chairman 综合。Nemeton 在此基础上增加 Project Reality、Claim/Evidence、Conflict、Ratification 和人类语义激活。

### ReConcile 与 Multi-Agent Debate

[ReConcile](https://aclanthology.org/2024.acl-long.381/) 和 [Multi-Agent Debate](https://arxiv.org/abs/2305.14325) 已实现多模型多轮讨论与共识形成。Nemeton 不采用 confidence voting 作为事实判定，并保留 dissent。

### AutoGen

[AutoGen GraphFlow](https://microsoft.github.io/autogen/dev/user-guide/agentchat-user-guide/graph-flow.html) 已支持串行、并行、条件和循环的多 Agent 图。Nemeton 的时序可以用同类图式执行，但采用自己的 Meeting 领域对象和 Convergence Gate。

### Claude Code Agent Teams

[Claude Code Agent Teams](https://code.claude.com/docs/en/agent-teams) 已实现共享任务、依赖、Agent-to-Agent messaging 和 hooks，证明自由讨论 transport 可以公开实现。但它使用固定 Lead，且官方标记为 experimental；任务状态和 session resume 也有已知限制。

### Magentic-One

[Magentic-One](https://www.microsoft.com/en-us/research/articles/magentic-one-a-generalist-multi-agent-system-for-solving-complex-tasks/) 使用 Task Ledger、Progress Ledger 和 stall 后重规划。Nemeton 借鉴其内外循环，但把重规划限制在用户批准的 Improvement Policy 内。

没有发现一个公开实现完整覆盖本文的 Project Reality、平等独立设计、自由讨论、逐项 Ratification、确定性 Convergence Gate、人类语义激活和后续 worktree Campaign 闭环。

## 17. v1 默认参数

```yaml
design_agents: 3
design_agents_capacity: unbounded_by_domain_model
verifier_agents: 1
recorder_agents: 1
sealed_proposals: true
anonymous_reveal: true
required_steelman_per_proposal: 1
required_challenge_per_proposal: 1
required_evidence_check_per_proposal: 1
peer_review_degree: 2
max_focused_deliberation_rounds: 2
allow_majority_vote_to_resolve_blocks: false
full_ratification_max_design_agents: 5
large_meeting_ratification: decision_cluster_coverage
human_activation_required: true
```

简单、低风险、已有明确合同的任务不启动完整 Swarm；Facilitator 可以选择单 Agent 或一次独立 Reviewer。完整会议只用于问题模型不清晰、存在真实架构取舍或变更影响跨越多个边界的任务。

## 18. Design Agent 水平扩展

### 18.1 不变量

```text
默认数量 = 3
领域容量 ≠ 3
```

水平扩展首先指同一会议可以从三个 Design Agent 增加到任意 `N` 个席位；未来也可以把独立 Agent Run 分布到多个本地 worker process，而不改变 Meeting 领域模型。

必须满足：

- `meeting_participants` 是行式 `0..N` 集合，不存在固定 `agent_a/agent_b/agent_c` 字段。
- 每个 Design Seat 独立生成 `agent_run`、`proposal` 和运行状态。
- Facilitator 通过持久化 round/barrier 协调，不等待进程内对象。
- Provider 并发和 rate limit 由 scheduler 控制，只影响吞吐，不改变会议语义。
- 增加席位不会获得额外投票权；Nemeton 本来就不以票数收敛。

### 18.2 避免全连接互评

朴素设计会让每个 Agent Review 其余 `N-1` 个 Proposal，调用和上下文成本为 `O(N²)`。Nemeton 不使用这一模型。

```text
N 个 Proposal
      │
      ▼
并行规范化 Decision / Claim
      │
      ▼
按问题维度和冲突聚类
      │
      ├── Cluster A：所有权
      ├── Cluster B：持久化
      └── Cluster C：恢复策略
      │
      ▼
每个 Proposal 只分配固定 K 个 Review
      │
      ▼
每个 Cluster 至少包含提案者、Steelman 和 Challenger
```

默认 `K=2`。另外单独分配 evidence check，确保每份 Proposal 至少拥有：

- 一个 steelman。
- 一个 challenge。
- 一个 evidence check。

调用规模接近 `O(N × K)`，而不是 `O(N²)`。所有 Proposal 仍然存在于 Shared Blackboard 并对参与者可查询，只是不在每一轮重复注入全部正文。

### 18.3 分层 Synthesis

单一逻辑 Recorder 不等于所有内容只能由一个模型串行阅读：

```text
Conflict Cluster A ─► Shard Recorder A ─┐
Conflict Cluster B ─► Shard Recorder B ─┼─► Final Recorder Lease ─► Synthesis Draft
Conflict Cluster C ─► Shard Recorder C ─┘
```

- Shard Recorder 只能产生带来源的候选摘要，不能写 Canonical Draft。
- Final Recorder 是唯一逻辑 writer，输入是各 Cluster 的有界结构化结果，而非全部原始聊天。
- Final Recorder 的写入使用 fencing token，防止两个 synthesis 同时生效。
- Verifier 并行检查不同 Decision Cluster，最后合并 blocking findings。

这样保持单一结论，又不会让一个 Recorder 成为前置处理瓶颈。

### 18.4 可扩展 Ratification

当 `N <= 5` 时，默认要求所有 active Design Agent 对 blocking Decision 完成 Ratification。

当 `N > 5` 时使用 Decision Cluster Coverage：

- 每个 Canonical Decision 至少由其原提案者或同类 Proposal 代表确认。
- 每个 Decision 至少有一个支持者和一个独立 Challenger 完成 Ratification。
- 所有已提出的 factual/constraint block 必须逐项解决，不能被 quorum 覆盖。
- 未被选入本轮 Ratification 的 Agent 仍可在 deadline 前提交 block 或 dissent。
- 缺席和未响应作为覆盖缺口写入 Meeting Result。

这不是多数票或抽样决定真相。它只减少重复 Review；任何有效硬阻塞仍具有相同效力。

### 18.5 数据结构增量

水平扩展需要以下集合型对象：

| 对象 | 用途 |
| --- | --- |
| `meeting_participants` | 任意数量席位、角色、Provider 和状态 |
| `round_runs` | 某轮每个参与者的独立 Agent Run |
| `proposal_review_assignments` | 固定 degree 的 steelman/challenge/evidence 覆盖 |
| `decision_clusters` | 把相似或冲突 Decision 聚到有界讨论单元 |
| `conflict_participants` | 每个 Conflict 实际需要唤醒的参与者 |
| `synthesis_shards` | Shard Recorder 的只读候选产物 |
| `ratification_assignments` | 小会议全员或大会议 Decision Coverage |

### 18.6 验收

- 默认会议创建三个 Design Seat。
- 配置为 4、8 或更多席位时不需要 schema migration 或新增角色字段。
- N 个 Proposal 不产生 `N × (N-1)` 个 Review Assignment。
- 每个 Proposal 的 steelman/challenge/evidence 覆盖均可证明。
- 增加 Agent 后 prompt 不包含无限增长的完整会议历史。
- Final Recorder 仍然只有一个有效 writer lease。
- 任何有效 factual/constraint block 都不能被多数或覆盖策略忽略。
- 一个 Agent Run 失败不会伪装成 Proposal；缺失席位和覆盖影响明确出现在 Meeting Result。
