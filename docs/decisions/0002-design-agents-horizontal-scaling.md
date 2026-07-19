# Decision 0002：Design Agent 必须支持水平扩展

> 状态：Accepted
>
> 日期：2026-07-19
>
> 决策者：Human

## 决定

Nemeton v1 默认使用三个 Design Agent，但三个不得成为领域模型、数据库、API、调度器、UI 或 Provider Runner 的容量上限。同一 Meeting 必须能够通过增加普通 Design Seat 水平扩展。

## 核心约束

- Design Seat 使用集合型 `0..N` 数据模型，不使用固定 Agent 字段。
- Agent Run 独立 fan-out，由持久化 round/barrier 协调。
- 所有席位拥有相同提案权；增加席位不产生核心成员或加权票。
- Proposal Review 使用固定 degree assignment 和 Decision/Conflict clustering，避免 `O(N²)` 全连接互评。
- 单一 Recorder 是逻辑写入权，不是单一前置处理 worker；允许并行 Shard Recorder。
- 小会议全员 Ratification，大会议采用 Decision Cluster Coverage。
- 所有 factual/constraint block 无论会议规模都必须解决，不能被多数票或 quorum 抵消。

## 原因

默认三个席位适合控制 v1 成本，但高风险、跨领域或大型重构可能需要更多独立问题模型。若实现直接围绕三个固定 Agent 编写，未来扩展会同时破坏状态机、存储和 UI；若采用全员互评，成本又会随 Agent 数平方增长。

因此需要从第一版就把“默认基数”和“系统容量”分开，并让讨论图保持稀疏。

## 验收

- 配置不同数量的 Design Agent 不需要 schema migration。
- 增加 Agent 只增加 participant/run/proposal 行和调度任务。
- Review Assignment 数量随 `N × K` 增长，`K` 为固定覆盖度。
- 每份 Proposal 均有 steelman、challenge 和 evidence check。
- Final Synthesis 始终只有一个有效 writer lease。
- Provider 失败被记录为缺失席位，不生成假 Proposal。
