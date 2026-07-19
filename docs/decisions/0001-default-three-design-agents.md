# Decision 0001：v1 默认使用三个 Design Agent

> 状态：Accepted
>
> 日期：2026-07-19
>
> 决策者：Human

## 决定

Nemeton v1 的完整 Swarm Meeting 默认启动三个平等的 Design Agent：

1. `Designer`：负责完整架构、数据流和所有权设计。
2. `Maintainer`：负责演化、迁移、恢复和长期复杂度。
3. `Adversary`：负责反例、失败路径和隐含假设。

Recorder 和 Verifier 是另外两个会议角色，不计入三个 Design Agent：

- Recorder 只有编译 Synthesis Draft 和 Meeting Result 的权限。
- Verifier 只检查证据、硬约束、组合一致性和可验收性。

## 约束

- 三个 Design Agent 拥有相同提案权，没有永久核心成员。
- 第一轮必须密封、独立、并行提交完整 Proposal。
- Design Agent 数量必须可以水平扩展；高风险会议可以显式增加席位，但 v1 默认值保持为三个。
- 新增席位不能获得更高提案权，也不能成为永久 Lead。
- 简单、低风险、已有明确合同的任务可以不启动完整 Swarm Meeting。

## 原因

三个席位能够覆盖正向设计、长期维护和对抗性检查，同时避免为普通会议引入过高的模型调用和协调成本。单个主设计 Agent 容易形成锚定；过多平等设计席位则会扩大 Proposal、Conflict 和 Ratification 的组合数量。

## 对实现的影响

- 默认会议模板的 `design_agents` 值为 `3`，但数据结构、API、调度器和 UI 不得把三个写成容量上限。
- 创建完整 Swarm Meeting 时，系统必须生成三个 Design Seat 和独立 Agent Run。
- 每个 Design Seat 都是相同 `meeting_participant` 模型的实例；不能使用 `designer_a/designer_b/designer_c` 这类固定字段。
- UI 应把三个 Design Agent、Recorder、Verifier 分开展示。
- Meeting Result 必须记录三个设计席位是否均已提交 Proposal；Provider 失败显示为缺失席位，不能生成替代意见。
