# Nemeton 公开多 Agent 设计调查

> 日期：2026-07-19
>
> 目的：调查公开的多模型讨论、多 Agent 协作、递归调度和 worktree 执行设计，判断是否已有系统完整覆盖 Nemeton 的产品设想。

## 1. 结论

公开设计很多，且每一层都已经有值得直接借鉴的成熟模式；但本次调查没有发现一个公开系统完整实现以下闭环：

```text
版本化仓库事实
      ↓
多模型独立提案与证据约束的交叉质询
      ↓
人类只激活新的产品语义
      ↓
语义编译为任务合同与递归改进政策
      ↓
多个受租约约束的 worktree 并行执行
      ↓
合并后硬性 Gate、Evidence 与持续 Trust State
      ↓
失败回流到输入、边界和下一轮会议
```

现有系统大致分成四类：

1. **讨论系统**：能让多个模型独立回答、互评和综合，但不了解长期项目事实，也不执行代码。
2. **编排框架**：能路由、handoff、并行和循环，但不规定什么是高质量讨论和可信收敛。
3. **Agent 软件团队**：有角色、SOP、任务和制品，但通常采用固定流水线或一个主 Agent 决定计划。
4. **并行编码控制面**：有 worktree、任务、消息、恢复和合并队列，但讨论层与项目语义层较弱。

Nemeton 的潜在独特性不在“Swarm”“多 Agent”或“worktree”本身，而在把上述四类连接成一个以项目语义和持续信任为中心的本地闭环。

## 2. 调查维度

每个系统按六层判断：

| 层 | 问题 |
| --- | --- |
| 共享事实 | 所有 Agent 是否基于同一仓库版本、证据和已批准语义？ |
| 讨论 | 是否支持独立提案、交叉质询、异议保留，而非普通群聊？ |
| 收敛 | 谁能写入最终结论？是否区分综合与产品决策？ |
| 执行 | 是否有任务合同、隔离工作区、并行写入所有权和恢复？ |
| 验收 | 是否以可验证 Evidence 和组合状态为准，而非 Agent 自报？ |
| 递归 | 是否能根据失败动态重规划，并知道何时回到人类语义会议？ |

## 3. 讨论层公开设计

### 3.1 Karpathy LLM Council

[LLM Council](https://github.com/karpathy/llm-council) 是与 Nemeton 会议最接近的简单公开实现：

1. 多个模型并行生成独立意见。
2. 隐去模型身份，让模型互相 Review 和排名。
3. 一个 Chairman 模型综合最终回答。

可直接借鉴：

- 第一轮独立 fan-out，避免由第一个回答锚定全部讨论。
- 匿名互评，减少模型品牌和位置偏见。
- 最终只有一个 synthesis writer，避免多个互相冲突的结论同时生效。
- 本地应用形态和多模型 Provider 接入。

不能直接照搬：

- 排名适合一次性问答，不足以表达软件设计中的多个可同时成立 claim。
- Chairman 可能压平正确的少数意见。
- 没有证据对象、项目版本、候选语义、任务合同和执行闭环。

Nemeton 应保留其三阶段骨架，但把“回答排名”替换为 `Claim / Evidence / Critique / Dissent / Candidate`。

### 3.2 Multi-Agent Debate 与 ReConcile

[Multiagent Debate](https://arxiv.org/abs/2305.14325) 让多个模型实例经过多轮查看和修订彼此推理；[ReConcile](https://aclanthology.org/2024.acl-long.381/) 使用多种模型进行圆桌讨论，并用 confidence-weighted voting 收敛。

可直接借鉴：

- 讨论应有明确轮次，而不是无限自由聊天。
- 异构模型比简单复制同一模型更可能提供真正不同的盲区。
- 后一轮只应围绕已暴露的关键分歧，而不是重新回答全部问题。

不能直接照搬：

- 共识和信心不是事实证明。
- 模型可能因多数压力放弃原本正确的少数观点。
- benchmark 问答通常有单一答案，产品和架构判断往往没有。

Nemeton 不应以票数自动激活语义。没有被证据解决的异议必须保留，并在必要时交还人类。

## 4. 编排层公开设计

### 4.1 AutoGen

AutoGen 公开提供多种清晰模式：

- [`SelectorGroupChat`](https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/selector-group-chat.html)：模型根据共享上下文选择下一位发言者。
- [`Swarm`](https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/swarm.html)：Agent 通过 handoff 自主转交控制权，共享群聊上下文。
- [`GraphFlow`](https://microsoft.github.io/autogen/dev/user-guide/agentchat-user-guide/graph-flow.html)：用有向图表达串行、并行、条件和循环。

可直接借鉴：

- 简单问题用少量 Agent，复杂流程才进入显式图。
- 轮次、并行 barrier、条件边和 termination 由代码控制。
- LLM 负责开放判断，host code 负责不可违反的流程约束。

不能直接照搬：

- 广播完整群聊上下文会快速膨胀，并放大锚定和叙事漂移。
- Selector 或 handoff 解决“谁说话”，不解决“讨论产物是否有证据”。
- GraphFlow 是执行原语，不提供 Nemeton 的领域模型。

Nemeton 应实现自己的 Meeting state machine，但可以使用相同的图式思想：轮次之间串行、轮次内部并行、分歧驱动下一轮。

### 4.2 OpenAI Agents SDK

[OpenAI Agents SDK orchestration](https://openai.github.io/openai-agents-python/multi_agent/) 明确区分：

- Manager 把 specialist 当作 tools，保留最终控制权。
- Handoff 把后续对话交给 specialist。
- LLM 驱动编排与代码驱动编排可以混合。

这是一套成熟、轻量的 Agent 调用原语，但它不是会议协议。Nemeton 可以从中借鉴 Provider/Runner 接口、trace 和 manager/tool 边界，不应把 handoff 当作讨论质量机制。

## 5. 主从并行与递归规划

### 5.1 Anthropic Research

[Anthropic 多 Agent Research 系统](https://www.anthropic.com/engineering/multi-agent-research-system) 使用 orchestrator-worker：Lead Agent 拆解研究方向，多个 subagent 并行搜索，Lead 最终综合。

可直接借鉴：

- 证据搜集适合 fan-out，并行探索互相独立的方向。
- Subagent 应返回压缩后的高价值结果和引用，而不是把全部上下文灌回 Lead。
- 多 Agent 只在任务可并行、价值足以覆盖 token 和协调成本时启用。

局限：

- Worker 主要向 Lead 汇报，不进行结构化的互相质询。
- Lead 同时拥有问题分解和最终综合权，容易形成单点叙事。
- 面向一次研究答案，不维护长期项目语义和执行状态。

### 5.2 Magentic-One

[Magentic-One](https://www.microsoft.com/en-us/research/articles/magentic-one-a-generalist-multi-agent-system-for-solving-complex-tasks/) 是递归规划层最值得借鉴的公开设计：

- Orchestrator 维护 Task Ledger：事实、推测和计划。
- 内层维护 Progress Ledger：当前进度与 Agent 分配。
- 检测无进展；连续停滞后回到外层更新事实和计划。

可直接借鉴：

- 把相对稳定的任务事实与快速变化的执行进度分开。
- 把 stall detection 作为一等状态，不让 Agent 无限重试。
- 允许执行结果反向改变计划。

Nemeton 需要进一步区分：

- `Project Reality`：仓库和已批准语义事实。
- `Campaign Policy`：允许怎样递归改进。
- `Progress State`：当前任务和 Evidence。
- `Semantic Escalation`：何时必须回到人类会议。

Magentic-One 的 Orchestrator 可以自行重写计划；Nemeton 只能在已批准 Improvement Policy 内这样做。

## 6. 软件开发角色与 SOP

### 6.1 MetaGPT

[MetaGPT](https://arxiv.org/abs/2308.00352) 把软件公司的 SOP 编码成 prompt sequence，让产品经理、架构师、工程师等角色按流水线产出中间制品。

可直接借鉴：

- 自由群聊应被可检查的中间制品替代。
- 角色应该拥有明确输入、输出和检查责任。
- 下游 Agent 不应从模糊聊天中自行猜测上游决定。

局限：

- 更接近固定软件流水线。
- 一个角色通常生产一种权威制品，缺少独立并行提案和异议保留。
- 不以真实长期仓库的持续 Trust State 为中心。

### 6.2 ChatDev

[ChatDev](https://arxiv.org/abs/2307.07924) 使用 Chat Chain，把设计、编码、测试和文档阶段进一步拆成角色间对话，并使用 communicative dehallucination 促进角色澄清。

可借鉴角色对话和阶段化产物；但它仍偏向预设 waterfall/chat chain，无法直接解决 Nemeton 的动态 Campaign、长期语义漂移和合并后系统信任。

## 7. 共享任务与 Agent 直接通信

### 7.1 Claude Code Agent Teams

[Claude Code Agent Teams](https://code.claude.com/docs/en/agent-teams) 公开实现了：

- 一个固定 Team Lead。
- 每个 teammate 独立 context window。
- 共享任务列表、依赖和 file-lock task claiming。
- teammate 之间直接消息。
- plan approval。
- TaskCreated、TaskCompleted、TeammateIdle 等 hooks。

这是“Agent 是否真的能互相讨论”的直接肯定答案：公开产品已经提供 Agent-to-Agent messaging 和共享任务状态。

但官方同时标注其为 experimental，并列出 session resumption、任务状态滞后、shutdown、固定 lead、不能嵌套 team 等限制。更关键的是，[并行模式文档](https://code.claude.com/docs/en/agents) 明确说明 Agent Teams 本身不会为 teammates 提供 worktree 隔离，需要自行按文件分区；worktree 和 team 是两套需要组合的机制。

对 Nemeton 的启示：

- 消息、任务和 hooks 可以作为底层 transport。
- 不能把 runtime team task list 当作持久项目事实。
- 不能依赖自由私信形成可审计结论。
- 持久 Evidence、Candidate 和 Contract 必须在 Nemeton 外部 ledger 中。

## 8. Worktree 与执行控制面

### 8.1 Codex App

[Codex App](https://openai.com/index/introducing-the-codex-app/) 已经把“一个项目下多个 Agent thread + 内置 worktree 隔离”产品化。它证明并行 worktree 是正在成为基础能力的 UX，而不是 Nemeton 的独特卖点。

公开介绍仍以人类切换 thread、查看 diff 和协作多个 Agent 为中心，没有展示 Nemeton 所需的多模型结构化会议、语义激活和持续 Trust State。

### 8.2 Vibe Kanban

[Vibe Kanban Workspaces](https://www.vibekanban.com/docs/workspaces/repositories) 为 workspace 创建独立 Git worktree，让原始仓库保持不变，并支持多个 workspace 同时工作。它是本地 worktree 生命周期和多 Agent CLI 适配的重要实现参考。

其产品重心仍是任务规划、运行监控和代码 Review，不是讨论如何产生可被整个项目长期信任的语义。

### 8.3 Gas Town

[Gas Town](https://github.com/gastownhall/gastown) 是目前与 Nemeton 执行层最接近的公开系统：

- `Rig` 绑定 Git 项目。
- `Mayor` 负责全局协调。
- `Polecat` 是带持久身份、临时 session 的 worker。
- `Beads` 保存持久任务 ledger。
- mailboxes、handoff、Convoy 和 Molecule 管理协作。
- Git worktree 保存 worker 工作。
- Witness/Deacon 监控卡住或死亡的 Agent。
- Refinery 使用类似 Bors 的合并队列，批量验证并二分隔离失败变更。
- escalation 把阻塞按严重级别向上路由。

其[公开架构文档](https://raw.githubusercontent.com/gastownhall/gastown/main/docs/design/architecture.md)还明确区分 Town-level coordination 与 Rig-level implementation，并采用 fail-fast 的持久化依赖。

这是 Nemeton 最应该深入阅读代码的项目，因为它已经处理了大量真实运行问题：身份、持久化、mailbox、心跳、session 恢复、worktree、merge queue 和升级。

但 Gas Town 与 Nemeton 的中心仍不同：

- Gas Town 是 execution-first，Nemeton 是 semantics/trust-first。
- Gas Town 让 Mayor 分解并派发；没有多模型独立审议和人类候选语义激活。
- Beads 主要保存工作状态，不是版本化 Project Reality 和不变量 ledger。
- Refinery 有验证队列，但没有“一个 check 必须证明自己能够失败”的 Trust State。
- Agent 失败主要触发恢复/升级，未系统追溯到项目输入和边界。

Nemeton 不应重写 Gas Town 已经做好的全部基础设施，而应先判断哪些执行概念可以借鉴甚至适配；产品差异应集中在 Meeting、Semantic Item、Campaign Policy 和 Trust State。

## 9. 对 Nemeton Swarm 的推荐组合

```text
LLM Council
  └── 独立并行提案、匿名互评、单一 synthesis writer

ReConcile / Debate
  └── 异构模型、多轮分歧修订

AutoGen GraphFlow
  └── 代码控制的并行 barrier、条件边和 termination

Magentic-One
  └── Reality/Task Ledger 与 Progress Ledger 分离、stall 后重规划

Claude Agent Teams
  └── 共享任务、直接消息、hooks；只作为运行 transport

MetaGPT
  └── 把讨论结果编译成下游可消费的结构化制品

Gas Town
  └── 持久身份、mailbox、worktree、watchdog、escalation、merge queue

Codex App / Vibe Kanban
  └── 本地 worktree 产品体验和主 checkout 隔离
```

Nemeton 自己必须补上的层：

1. 固定到 Git revision 的 Project Reality。
2. `Claim / Evidence / Critique / Dissent` 讨论协议。
3. 人类选择候选语义，Git 激活后才成为 active truth。
4. 从语义到不可变 Task Contract 的编译。
5. Improvement Policy 约束下的递归 Campaign。
6. 合并后组合验证和持续 Trust State。
7. 把失败追溯到目标、边界、环境、验证能力或实现。

## 10. 推荐的 Swarm v1，而不是自由群聊

```text
Round 0  Host code 固定 Evidence Pack
             │
Round 1  3 个异构模型独立提案（并行、互相不可见）
             │
Round 2  Host/Extractor 生成 Claim 与 Conflict Map
             │
Round 3  针对 blocking claim 匿名交叉质询（并行）
             │
Round 4  原提案者根据 critique 修订（并行）
             │
Round 5  单 Recorder 编译 Agreement/Dissent/Candidate
             │
Round 6  独立 Verifier 检查引用、遗漏和不可验证结论
             │
          人类选择新的产品语义
```

硬规则：

- 不用多数票消灭 dissent。
- Recorder 只有编译权，没有新的产品决策权。
- Agent 消息必须引用 claim/evidence ID，关键结论不能只存在于自由文本。
- Round 1 必须独立，避免锚定。
- 轮次最多 2 次修订；继续产生重要新分歧时交给人类或标记 unknown。
- Agent 缺席或 Provider 失败显示为缺失席位，不能生成替代意见。
- Meeting output 不直接执行；先通过 Contract Gate。

## 11. 最终判断

有人公开提供了 Nemeton 每个关键部件的设计，而且其中一些已经非常深入：

- 想实现“多人讨论”，先读 LLM Council、ReConcile、Claude Agent Teams。
- 想实现“代码控制的 Swarm”，先读 AutoGen GraphFlow 和 OpenAI Agents SDK。
- 想实现“失败后动态重规划”，先读 Magentic-One。
- 想实现“软件团队制品流水线”，先读 MetaGPT 和 ChatDev。
- 想实现“本地多 Agent + worktree + 恢复 + 合并”，优先深挖 Gas Town，其次 Codex App 和 Vibe Kanban。

但目前没有发现一个公开成熟产品把这些能力组织成 Nemeton 的完整语义闭环。这一判断是基于上述公开文档和代码的推断，不代表不存在未公开或未被本次调查覆盖的内部系统。

因此，Nemeton 不需要发明新的 Agent transport，也不应把“能启动多个 Agent”当作产品价值。它应该复用已经公开验证的协作模式，把研发集中在公开系统仍缺失的语义层、讨论层和信任闭环。
