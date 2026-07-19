# Decision 0005：人类负责语义激活，系统 Gate 负责实现验收

> 状态：Accepted
>
> 日期：2026-07-19
>
> 决策者：Human

## 决定

Meeting 是 Nemeton 唯一必需的人类语义入口。人类确认项目使命、演化范围、不变量、风险边界和无法编码的取舍。实现完成后的默认验收由 deterministic gate、运行证据、Reviewer finding 和 Trust State 负责。

人类不承担以下默认工作：

- 阅读所有 Agent diff。
- 确认每个局部技术动作。
- 根据 Reviewer Agent 的总结盖章。
- 在多个 worktree 之间手工核对组合状态。
- 用主观感觉替代已经定义的 Acceptance Obligation。

## Human Meeting Result

系统提交给人类的 Meeting Result 必须包含：

- 推荐结论和 Canonical Decisions。
- 每项决定的来源 Claim 和 Evidence。
- Human Choices。
- 未解决 Unknown 和 Dissent。
- 被拒绝方案及原因。
- Implementation Outline。
- Acceptance Obligations。
- Convergence Report。

人类可以 approve、approve with edits、choose option、request evidence、reopen 或 reject。

## 实现验收

人类批准语义后，Task Contract 和 Gate 负责实现验收：

```text
Human-approved semantics
          │
          ▼
Task Contract
          │
          ▼
Agent implementation
          │
          ▼
Task / Diff / Integration / Release Gate
          │
          ▼
Evidence + Trust State
```

Reviewer Agent 可以分析代码、指出反例和检查语义 claim。它的 verdict 不能单独构成 hard gate。

## 例外边界

以下操作仍然可以要求用户授权：

- 操作系统、凭据或外部服务要求的权限确认。
- 不可逆发布、删除和安全敏感操作。
- Improvement Policy 已经明确要求的人类 release decision。

这些授权处理外部权限和不可逆后果，不会把逐行代码 Review 重新设为产品 Gate。

## 验收

- 未经人类批准的 Candidate Semantic Item 不能变成 active。
- 已批准且合同完整的局部实现不要求额外人工代码确认。
- hard gate 必须引用可执行检查或可验证 Evidence。
- 新产品语义出现后，Campaign 自动暂停并创建 Meeting 议题。
- UI 把 Human Choice 和系统验收分开展示。
