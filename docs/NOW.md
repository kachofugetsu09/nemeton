# Nemeton 当前状态

> 更新日期：2026-07-19
>
> 当前阶段：Milestone 1 持久化 Swarm Meeting 后端已实现，下一切片是 Local Web UI
>
> 边界：本文件是 Babel 仓库工作手册；产品 Current State 存在 SQLite，不生成产品 `NOW.md`

## 当前结论

Milestone 0 基线继续由仓库级 `semantic-gate` 保护。Milestone 1 后端在其上增加了
Reality Bundle、持久 Meeting、Codex/OpenCode Provider、每席位隔离 Workdir、有界收敛、
Recorder、Verifier、候选语义人工处置、数据库 Current State、HTTP/WebSocket 和 CLI。

当前实现合同是
[`contracts/2026-07-19-milestone-1a.md`](contracts/2026-07-19-milestone-1a.md)，
详细设计是
[`design/milestone-1a-meeting-backend.md`](design/milestone-1a-meeting-backend.md)，
运行时决定是
[`decisions/0009-milestone-1-meeting-runtime.md`](decisions/0009-milestone-1-meeting-runtime.md)。
一次性真实 Provider 认证记录见
[`design/milestone-1-live-provider-certification.md`](design/milestone-1-live-provider-certification.md)。

## 已交付：Milestone 1 后端

```text
冻结 Git Reality（含 staged / unstaged / untracked）
                    │
                    ▼
       3 个 Design Seat 密封并行提案
                    │
                    ▼
       Reveal ──► 最多 3 轮有界讨论
                    │
          ┌─────────┴─────────┐
          ▼                   ▼
       全体同意          needs_user_input
          │                   │
          ▼                   └──► Human input 后新 cycle
     Recorder ──► Verifier
                    │
                    ▼
        Human select / reject / defer
                    │
                    ▼
                concluded
```

### 持久化与恢复

- Meeting、Participant、Agent Run、Content、Conflict、Candidate 和 Current State 都由
  append-only 事件生成 SQLite projection。
- 每次事件追加、所有受影响 projection 和 Current State digest 在同一事务内提交。
- replay 只重算状态，不重新调用 Provider、不执行 Git 写入，也不重复外部副作用。
- daemon 恢复时把未完成 Agent Run 标记为 interrupted，再从持久 Meeting 状态继续。
- WebSocket 发送已提交 domain event 和临时 Runner delta；客户端以 project sequence 断线补读。

### Provider 与隔离

- 默认席位：Designer=Codex、Maintainer=OpenCode、Adversary=Codex、Recorder=OpenCode、
  Verifier=Codex；创建 Meeting 时可显式覆盖为 Codex 或 OpenCode，不做 fallback。
- 每个席位拥有独立、持久的 repository clone、logs、output 和 Provider session；
  Design proposal 在 Reveal 前互不可见。`home/` 与 `provider-home/` 目录已预留，但
  当前真实客户端继承宿主环境和凭据，不把它们宣称为安全隔离。
- Provider 获得编码 Agent 的完整工具权限，可以在自己的 Workdir 读写和实验。
- Nemeton 不把角色提示词误称为安全沙箱；目标 source checkout 在调用后重新观察，发生变化则失败。
- 真实 Codex/OpenCode 只用于本次交付认证；CI 使用执行真实子进程协议的程序化测试 Provider，
  不消耗模型额度。运行时对本机已安装客户端采用乐观兼容，版本或协议不兼容时明确失败。

### API 与 CLI

- `POST /v1/projects/{id}/meetings` 创建 Meeting。
- `GET /v1/meetings/{id}` 读取完整 Meeting snapshot。
- `POST /v1/meetings/{id}/start` 启动。
- `POST /v1/meetings/{id}/inputs` 在未收敛或 Verifier 阻塞后提供 Human input。
- `POST /v1/meetings/{id}/ratifications` 处置 Candidate。
- `GET /v1/meetings/{id}/ws?after_sequence=<n>` 订阅和补读事件。
- `GET /v1/projects/{id}/current-state` 读取确定性 Current State。
- CLI 提供 `project state` 与 `meeting create/show/start/run/watch/answer/ratify`。

## 持续验收

- Milestone 0 的 Project/Reality/artifact/event/projection/replay 联合语义保持不变。
- dirty Reality 能逐字节恢复到每个席位，席位间实验互不可见，目标 source checkout 零修改。
- 三个 Design proposal 并发启动但密封；只有完全相同的规范化 canonical statement 且全员
  `accept` 才收敛。
- 第三轮仍不一致时确定进入 `needs_user_input`；Human input 开启新 cycle。
- Recorder Candidate 必须引用真实持久 content；Verifier 阻塞时不得进入人工批准。
- Candidate 必须逐项 `selected`、`rejected` 或 `deferred`；selected 不等于 active。
- 真实 SQLite 故障证明 event、projection、Current State 和 stream version 原子回滚。
- daemon/API/Unix socket/WebSocket/restart/replay 的端到端组合路径由真实边界测试覆盖。
- 仓库唯一硬性验证入口仍是 `scripts/semantic-gate`，required check 名称仍是 `semantic-gate`。

## 下一步

在不复制业务状态机的前提下，为当前 Meeting HTTP/WebSocket 后端增加 Local Web UI。
UI 只消费 daemon API 和数据库投影，不成为新的事实源。Campaign、Task Contract、任务
worktree、产品 Gate 引擎和 merge queue 仍属于 Milestone 2 及之后。

## 当前不包含

- Web UI。
- Campaign、Task Contract 和任务调度。
- 治理 worktree、任务 worktree、PR 或 merge queue。
- Nemeton 产品 Gate、Integration Gate 或 Release Gate 引擎。
- Docker、Windows、PostgreSQL、消息队列、向量数据库、微服务或通用插件系统。
