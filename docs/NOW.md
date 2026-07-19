# Nemeton 当前状态

> 更新日期：2026-07-20
>
> 当前阶段：Milestone 1B 已实现，下一阶段是 Milestone 2 单任务执行
>
> 边界：本文件是 Babel 仓库工作手册；产品 Current State 存在 SQLite，不生成产品 `NOW.md`

## 当前结论

Milestone 0、1A 和 1B 形成了从 Project Reality 到批准 Coding Design Handoff 的持久闭环。
Local Web UI 使用 protocol v2；未携带 Participants 的 legacy CLI/API 仍创建 protocol v1，既有
Verifier 数据和 replay 兼容性保留。

当前实现合同是
[`contracts/2026-07-20-milestone-1b.md`](contracts/2026-07-20-milestone-1b.md)，
详细设计是
[`design/milestone-1b-review-loop-webui.md`](design/milestone-1b-review-loop-webui.md)，
长期决定是
[`decisions/0010-protocol-v2-recorder-review-loop.md`](decisions/0010-protocol-v2-recorder-review-loop.md)。

## 已交付：Milestone 1B

```text
Project + 原始问题
        │
        ▼
3 个默认 / 0..N 可配置 Design Agent + 1 Recorder
        │
        ▼
密封 Proposal ──► Reveal ──► 有界自然语言讨论
        │
        ▼
完整 Result + Candidate Semantic Items
        │
        ▼
用户逐项处置 + 可选反馈
        │
        ├── Recorder answer ───────────────┐
        ├── Recorder full patch revision ─┤
        └── 同阵容 reconvene next Cycle ─┤
                                           ▼
                              用户批准完整方案
                                           │
                                           ▼
                         Approved Context + Handoff
```

### Runtime 与持久化

- Participant 显式保存 Provider、模型和思考 options；同一 Meeting 的后续 Cycle 复用 identity、
  Workdir、Provider Session 和完整配置。
- Result、Recorder answer/opening、用户 review、逐项 design/context disposition、Approved Context
  和 Handoff 都由 append-only event 与 CAS 驱动，可从 SQLite replay。
- Recorder 根据自然反馈和结构化处置自主选择 `answer`、`patch` 或 `reconvene`；核心 owner、boundary、
  invariant 或 failure model 的变化必须重开。
- 批准只生成数据库状态和 Coding Design Handoff；不修改目标仓库，不生成 commit，不激活 `.nemeton/` 语义。

### Local Web UI

- daemon 在 loopback 同源提供嵌入式 Web UI、HTTP API 和 WebSocket；Host、Origin 和 Meeting content scope
  在边界校验。
- 参与者配置默认折叠，Provider 可拖入并选择模型与思考强度。
- 会议中间只展示带 `@seat` 的自然语言讨论，右侧只展示 Proposal；Result Review 使用独立页面。
- 用户逐项选择长期保留、仅本方案、反对或稍后，自然语言反馈可空。

## 持续验收

- `scripts/semantic-gate` 是唯一仓库硬 Gate：前端 lint/build/Playwright、Go format/vet/race tests 和 CLI/daemon build。
- Playwright 固化真实拖拽、390px 无横向溢出、讨论/Proposal attention 分离和 option-first Result Review。
- protocol v1 数据仍可读取和 replay；v2 不创建 Verifier。
- restart/replay 不调用 Provider、不启动新 Cycle、不重复 committed event 或外部副作用。
- 真实 Provider 只在交付认证或客户端协议变化时运行，CI 使用程序化 Provider。

## 下一步

Milestone 2 才加入 Governance Writer、不可变 Task Contract、单任务 worktree、Task Gate、Diff Boundary Gate
和交付。Milestone 1B 的 Handoff 是它的只读输入；不能让前端或 Coding Agent 绕过 daemon 激活语义。

## 当前不包含

- Coding Agent 执行、Task/Campaign 调度和目标仓库写入。
- Governance worktree、`.nemeton/` 发布、PR 或 merge queue。
- Nemeton 产品 Gate、Integration Gate、Release Gate 或 Trust State。
- Docker 运行形态、Windows、PostgreSQL、消息队列、向量数据库、微服务或第三种 Provider。
