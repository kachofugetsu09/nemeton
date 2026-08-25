# Decision 0009：Meeting 使用持久化 Coding Agent Workdir 与 WebSocket

> 状态：Accepted
>
> 日期：2026-07-19
>
> 决策者：Human
>
> Amends：Decision 0003、0004、0006 中的 NOW/SSE 示例；落实 0001、0002、0005、0008
>
> Current amendment：Decision 0010 新增 protocol v2，并修订当前会议的 Verifier、未收敛和 UI 边界；protocol v1 历史语义保留。

## 问题

Milestone 1 需要让 Design Agent 读取真实仓库、写实验代码并运行测试，同时保证会议
有持久共同事实、有界收敛、可恢复观察和 CI 回归。早期设计中的生成式产品
`NOW.md`、SSE 和只读/无工具会议 Agent 无法满足这些要求。

## 决定

- 产品 Current State 使用 SQLite structured projections 和确定性 document/digest，
  不生成产品 `NOW.md`；仓库的 `docs/NOW.md` 仍是工作手册。
- Meeting Agent 获得与 Coding Agent 相同的最大工具能力。角色差异只由职责描述和
  输出合同表达。
- 每个席位持有独立、持久化 Workdir 和 Provider Session；Reveal 前上下文隔离。
- 会议共同起点是包含 commit、staged、unstaged 和 untracked 内容的不可变 Reality
  Bundle。
- 每项 Conflict 最多讨论三轮；不能全体接受时确定进入 `needs_user_input`。
- 实时通道使用 WebSocket；SQLite event/projection 仍是唯一事实源。
- 首次交付使用真实 Codex/OpenCode 认证；后续 CI 使用测试专属程序化等价 Provider。
  未来外部版本不兼容在运行时 fail-fast，不生成 fallback 意见。
- 后端完成并稳定后再独立实现 Local Web UI；Milestone 1 不混入 UI。

## 原因

可信计划需要 Agent 能采集代码和运行实验。持久隔离 Workdir 保留这一能力，又避免
密封席位通过 Nemeton 共享文件。WebSocket 支持 Runner delta 和双向演进；事件 sequence
使断线恢复不依赖连接本身。三轮上限保证协议终止，而 Human 保留真实取舍权。

## 后果

- Nemeton 不把 Meeting Agent 当作安全 sandbox；最大权限可能产生外部副作用，系统
  负责记录和暴露，不宣称阻止。
- CI 能证明 Nemeton 自身的 Runner/Meeting 语义，不能证明未来 Provider 版本兼容。
- Decision 0003/0004 中把 SSE 连接列为 ephemeral state 的示例改为 WebSocket 连接；
  Decision 0006 的 HTTP/SSE 改为 HTTP/WebSocket。
- `selected` Candidate 仍不等于 active，不提前实现 Milestone 2。

## 验证

- 真实 Codex/OpenCode 在独立 Workdir 中读取、修改并测试一个 Reality Bundle。
- Reveal 前每个席位的输入不包含其他 Proposal。
- 三轮未解决 Conflict 确定进入 `needs_user_input`。
- WebSocket 断线后以 sequence 补齐 committed events。
- replay 不启动 Provider 或重新执行 Workdir/Git 副作用。
- `scripts/semantic-gate` 保留 Milestone 0 回归并增加上述联合旅程。
