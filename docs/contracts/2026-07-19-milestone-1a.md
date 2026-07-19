# Milestone 1A Persistent Swarm Meeting Backend Agent Task Contract

> 状态：Approved
>
> 批准者：Human
>
> 批准日期：2026-07-19

## 1. Goal

- Objective：完整实现持久化 Swarm Meeting 后端，使 Human 能以一个不可变 Reality
  Bundle 为共同起点，运行真实 Codex/OpenCode 设计席位，经过密封提案、Reveal、
  有界讨论、Recorder、Verifier 和人工 `select/reject/defer` 得到可追溯 Meeting Result。
- Observable success criteria：以
  [`../design/milestone-1a-meeting-backend.md`](../design/milestone-1a-meeting-backend.md)
  第 12 节为准。
- Change type：docs、code、schema、runtime、CI、external Provider execution。
- Semantic delta：实现用户确认的 DB Current State、Reality Bundle、每席位持久化隔离
  Workdir、最大 Agent 工具权限、三轮有界收敛、HTTP/WebSocket 和后续 UI 边界；
  `selected` 仍不等于 `active`。

## 2. Ownership

- Capability owner：`nemetond` 内的 Meeting、Runner、Workspace、Current State、Store
  与 API 模块。
- Consumer scope：`nemeton` CLI 和未来 Local Web UI；客户端不得复制状态机。
- Authoritative state owner：append-only Project event stream 和其引用的 CAS artifact；
  SQLite Meeting/Current State 表是确定性 projection。
- Core runtime patch reason：持久会议、Agent Run 和 WebSocket 都需要与唯一 SQLite
  writer、恢复和 replay 共用同一进程边界。
- Client-side alternative and rejection reason：CLI 或 UI 直接编排 Provider 会绕过事件
  事务、持久化屏障、恢复和收敛不变量。

## 3. Invariants and protected state

- 必须保持的不变量：Milestone 0 全部不变量；事件与 projection 原子提交；Reality
  Bundle 不可变；Reveal 前各席位互不可见；缺失席位不产生替代意见；每个冲突最多
  三轮后进入 `resolved` 或 `needs_user_input`；Recorder 不裁决；Verifier 不改写；
  selected candidate 不激活语义；replay 不调用 Provider、Git 写操作或网络。
- 允许增加的持久化状态：Meeting 事件、projection、Current State、Reality Bundle、
  Message/Evidence、Provider Session、Agent Workdir、运行日志、CI Evidence 和文档。
- 允许更新或逻辑失效的状态：Meeting/Participant/Conflict/Agent Run 状态、Current
  State projection、WebSocket runtime subscription、可回收 Workdir。
- 禁止物理删除的状态：domain events、已引用 artifact、Meeting Message/Evidence、
  Human input 和 ratification 历史。
- 受保护资源：`/mnt/data/coding/nemeton` 主 checkout、非会议目标 checkout、用户现有
  数据库和进程。Agent 采用用户批准的最大权限信任模型；Nemeton观察目标 checkout
  的前后变化并明确报告，但不声称用 sandbox 阻止 Agent 外部副作用。

## 4. Allowed scope

- Repository：`git@github.com:kachofugetsu09/nemeton.git`。
- Base commit：`09b3129d590ca0bf555b7be25b78d14a50bad95d`。
- Branch：`babel/milestone-1`。
- Worktree：`/mnt/data/coding/nemeton-worktrees/milestone-1`。
- Allowed paths：`AGENTS.md`、`go.mod`、`go.sum`、`README.md`、`cmd/**`、`internal/**`、
  `migrations/**`、`scripts/semantic-gate`、`.github/workflows/semantic-gate.yml`、
  `docs/{INDEX,NOW,WORKFLOW,projectneed}.md`、`docs/design/**`、`docs/contracts/**`、
  `docs/decisions/**`、相关 `docs/spark/**` 勘误。
- Forbidden paths：Web UI、Docker、Windows 支持、Milestone 2 Campaign/Task Contract/
  governance worktree/Task Gate/delivery engine、release 和部署文件。

## 5. Side effects

| 边界 | 允许的副作用 | 明确禁止 |
| --- | --- | --- |
| 文件 | 隔离实现 worktree、测试目录、Meeting Workdir、CAS、日志 | 在主 checkout 实施；静默删除 Evidence |
| Git | 实现 branch、commit、push、PR、merge；Meeting Agent 最大权限 | force-push、改写 Nemeton 历史 |
| 数据库 | migration 前 backup、测试库、运行时 events/projections | 删除事件、CLI/UI 绕过 daemon 写库 |
| 进程 | daemon、真实 Codex/OpenCode 首次认证、测试等价 Provider | 失败后静默替代意见 |
| 网络与外部 API | 首次真实 Provider 调用、依赖下载、GitHub CI | 后续 CI 真实模型调用；伪称兼容未知客户端版本 |
| 消息与发布 | PR、required check、merge 记录 | Web UI、release、package 发布 |

## 6. Dependencies

- WebSocket 使用单一维护中的 Go library；版本固定在 `go.mod`，缺失时构建失败。
- 生产 Provider 依赖 PATH 中真实 `codex` 和 `opencode`；运行时乐观调用，协议或版本
  不兼容时保留版本、命令、stderr 和退出状态并使 Agent Run 失败。
- CI 使用仅存在于测试构建中的程序化等价 Provider；它不能注册为生产 fallback。

## 7. Validation

- 可执行验证：扩展后的 `scripts/semantic-gate`；Linux/macOS 均运行 format、diff、
  vet、race tests、build、Milestone 0 回归、Milestone 1 单项/联合/恢复旅程。
- 语义验收：设计第 12 节；最终 target SHA 额外使用真实 Codex/OpenCode 完成一次
  会议，后续 CI 不进行真实模型调用。
- Target SHA 或 diff：相邻 base/head、最终 cumulative head 和 PR merge head。
- Evidence 输出位置：测试日志、Meeting CAS、真实认证 Meeting ID/Result、PR checks。
- 明确未验证项：未来 Provider 版本兼容性由运行时感知；Web UI 和 M2 不在范围。

## 8. Stop and rollback

- Stop conditions：Accepted decision 冲突、需要 M2 才能成立、必须静默绕过 Provider
  错误、主 checkout 有未知改动、无法维持 M0 不变量或范围必须超出 allowed paths。
- Escalation：停止写入，保留 worktree、失败命令、Provider 输出和真实状态，请 Human
  裁决。
- Backup：`/mnt/data/coding/nemeton-backups/20260719-before-milestone-1/`，其中完整
  Git bundle 已通过 `git bundle verify`。
- Rollback steps：合并前保留 branch/worktree；合并后创建 revert commit，不改写
  `main`；运行时数据库用 migration 前 verified backup 恢复。
- Rollback verification：核对 main SHA、worktree、remote refs、SQLite integrity 和
  bundle 可读性。

## 9. Delivery authorization

- 交付目标：commit、push、PR、required CI、merge 到 `main` 并同步本地主 checkout。
- 已授权动作：备份、隔离、实现、首次真实 Provider 验收、只读 Review、文档对账、
  commit、push、创建 PR、等待 CI、merge 和本地同步。
- 需要再次确认的动作：仅限触发 stop condition 或扩大到 Web UI/Milestone 2。

## 10. Delivery evidence

- 一次性真实 Provider 认证及其版本、Meeting ID、Result digest、迁移和 replay 对账见
  [`../design/milestone-1-live-provider-certification.md`](../design/milestone-1-live-provider-certification.md)。
- 真实调用之后不再调用模型；最终增量和未来 CI 只运行程序化等价 Provider。
- 最终仓库验证以 `scripts/semantic-gate`、PR target SHA 和 GitHub required check 为准。
