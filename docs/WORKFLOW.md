# Nemeton 开发工作流

这份工作流同时约束 Nemeton 自身的开发和未来由 Nemeton 管理的目标项目。

## 0. 当前仓库能力边界

当前仓库只有已确认文档，还没有业务代码、测试、CI、Docker 或自动语义 Gate。本文后半部分的 Contract、Gate、Campaign 和 projector 是 Nemeton 的 Accepted 产品语义，不代表当前仓库已经提供对应命令。

在 Nemeton 能自举管理自身以前，仓库变更通过 Babel 工作流执行；不能用未来产品能力伪装当前验证已经完成。

## A. Babel 仓库交付流程

```text
Read → Ownership → Isolate → Contract → Implement → Verify → Review → Reconcile → Deliver
```

### A.1 Read

从 `AGENTS.md → docs/INDEX.md → docs/WORKFLOW.md` 进入，根据任务路由读取需求、决定、当前状态、相关设计和真实实现证据。出口条件是事实、用户意图、推断和未知已经分开。

### A.2 Ownership

声明能力 owner、consumer scope、权威状态 owner、持久化增减、外部依赖来源，以及为什么必须修改 core runtime。优先在拥有不变量的层解决问题；客户端替代方案可成立时，不扩大核心边界。

### A.3 Isolate

检查目标基线、dirty state、remote 和已有 worktree。从正确基线创建专用 worktree；修改持久化状态前先备份。不得把用户主 checkout 当作默认执行 workspace。

### A.4 Contract

写入前确认不可变任务合同，包括目标、可观察验收、语义变化、允许路径、副作用、不变量、停止条件、验证、回滚和交付授权。新产品语义或范围变化必须停止并重新确认合同。

### A.5 Implement

只完成合同要求的最小改动。边界校验集中在真实信任边界；内部契约失败保持可见。不得用 mock、空返回、动态依赖或静默 fallback 制造成功。

### A.6 Verify

运行与改动直接相关、当前仓库真实存在的最小验证。记录命令、target SHA 或 diff、退出状态和未验证项。没有测试或 Gate 时明确写“未初始化”，不能发明命令。

### A.7 Review

Review 默认只读，使用 `docs/templates/review-contract.md` 固定 base/head、能力 owner、持久化影响和验证证据。发现需要改动时先取得修复授权。

### A.8 Reconcile

对账实现、`projectneed.md`、Accepted decisions、相关设计和 `NOW.md`。完成事项从 `NOW.md` 删除；新的长期语义必须有用户确认或 Accepted decision。

### A.9 Deliver

说明改动、验证、未验证、备份、回滚、分支和未提交状态。只有合同明确授权时才 commit、push、创建 PR 或合并。

## B. Nemeton 产品工作流

### B.1 产品语义进入方式

新的产品语义只能从会议进入：

```text
Human Statement
      │
      ▼
Meeting Discussion
      │
      ▼
Candidate Semantic Items
      │
      ▼
Human select / reject / defer
      │
      ▼
Governance change
      │
      ▼
进入 integration history 后成为 active
```

实现 Agent 可以提出候选项，无权激活使命、边界、不变量、产品决定和风险政策。

### B.2 设计工作

以下情况需要启动完整 Swarm Meeting：问题模型不清晰、存在架构取舍，或者变更影响多个边界。

1. 固定 Reality Revision、问题和硬约束。
2. 默认三个 Design Agent 密封并行提交完整方案。
3. 同时 Reveal，按 Claim、Evidence 和 Conflict 展开讨论。
4. Recorder 生成 Synthesis Draft。
5. Verifier 检查证据、约束、组合一致性和验收义务。
6. Design Agent 逐项 Ratification。
7. 确定性 Convergence Gate 生成 Meeting Result。
8. 人类选择、修改、要求补证据或重新讨论。

已有明确合同的低风险任务可以跳过完整会议，进入任务编译。

### B.3 执行工作

每项执行任务必须绑定不可变 Task Contract：

- base SHA 和 Reality Revision。
- objective、preconditions 和 dependencies。
- write scope、forbidden scope 和 invariants。
- 可执行 acceptance 与语义 acceptance。
- stop conditions 和 escalation。

调度器只把 `ready` 任务派发到 Nemeton 管理的 worktree。Agent 不能在用户主 checkout 中执行默认写入流程。

### B.4 并行所有权

- 独立 Task 可以并行运行。
- 重叠 write scope 默认不能并行。
- Git metadata 操作使用 repository-scoped lock 短暂串行。
- 每个 writer 使用带 fencing token 的 lease。
- 过期进程不能凭旧 heartbeat 或旧 token 恢复写权限。
- PR 可以并行生成，integration branch 的合并操作保持串行。

### B.5 Gate 与交付

```text
Contract Gate
    ↓
Task Gate
    ↓
Diff Boundary Gate
    ↓
Integration Gate
    ↓
Release Gate（按风险启用）
```

每个 Gate 结果必须绑定准确 target SHA、输入 digest、执行环境、退出状态和完整证据。Reviewer Agent 只能提供 finding、反例和证据，不能单独宣布 hard gate 通过。

### B.6 失败处理

失败必须回到一个明确拥有层：

- 目标含糊：回到 Meeting Brief 或 active semantics。
- 合同遗漏：生成新版本 Task Contract。
- 边界误导：修改接口、所有权或任务拆分。
- 验证不足：增加能够真实失败的 Gate。
- 环境不可用：记录外部失败并停止，不能生成假成功。
- 实现错误：在原政策范围内生成修复任务。
- 出现新产品语义：暂停 Campaign，创建会议议题。

### B.7 文档同步

- Accepted Decision 的变化需要同步到 `projectneed.md`。
- 当前实施切片的变化需要同步到 `NOW.md`。
- 详细状态机或协议的变化需要同步到对应 `docs/spark/` 文档。
- 研究材料只追加证据，不能改变产品事实。
- projector 完成后，`NOW.md` 和其他当前状态文档由事件流生成，人工或 Agent 不再修改这些文件。
