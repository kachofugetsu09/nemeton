# Milestone 1A：持久化 Swarm Meeting 后端设计

> 状态：Accepted
>
> 用户确认日期：2026-07-19

## 1. 结论与边界

本里程碑一次性交付原路线图 Milestone 1A/1B 的后端能力：持久 Meeting、默认三个
平等 Design Agent、真实 Codex/OpenCode Runner、密封提案、Reveal、Conflict、
Recorder、Verifier、有界收敛、人工候选语义选择、HTTP/WebSocket 和 CLI。

Web UI 作为下一独立里程碑接入；Campaign、Task Contract、正式任务 worktree、
代码交付和语义激活仍属于 Milestone 2。

## 2. 权威状态

```text
append-only domain events + CAS artifacts
                  │
                  ▼
        deterministic reducers
                  │
          ┌───────┴────────┐
          ▼                ▼
 Meeting projections   Current State
          │                │
          └──── HTTP / CLI ┘
```

产品当前状态只存在于 SQLite structured projections 和一份确定性 Current State
document/digest 中，不生成产品 `NOW.md`。仓库自身的 `docs/NOW.md` 仍是 Babel 工作手册。

WebSocket 只传递已提交事件通知和临时 Runner delta；断线后客户端以 sequence 重新读取
projection。它不是第二事实源，也不持有长数据库事务。

## 3. Reality Bundle

会议创建时冻结目标 checkout 的：

- Project、integration branch、HEAD/tree SHA 和 remote metadata。
- staged diff、unstaged diff、tracked 文件模式。
- untracked 文件路径、模式和正文。
- 创建前目标 checkout/Git metadata observation。

Bundle 使用版本化 manifest 引用 CAS artifact。每个席位从相同 commit 建立独立
checkout，再应用相同工作区变化。目标 checkout 后续变化不改变既有 Bundle；需要新
事实时显式创建新 revision。

## 4. Agent 执行信任模型

所有 Meeting Agent 使用同一套最大 Coding Agent 能力。角色区别只写在职责描述和
输出合同中，不按角色阉割工具：

- 可读取、搜索和修改独立 Workdir。
- 可执行命令、运行测试、做小实验和数据分析。
- 可访问网络、安装依赖并使用用户已有凭据。
- 技术上可执行 Git commit/push；Meeting workflow 只把结果视为 Evidence，不把它
  自动升级为正式交付。

Nemeton 不宣称用 sandbox 阻止恶意外部副作用。它为每个席位提供独立持久 Workdir、
独立 Provider Session 和进程监督，并在运行前后观察目标 checkout，暴露偏离职责的
行为。

## 5. Provider 与席位

默认分配按确定性轮转持久化，并允许创建会议时覆盖：

| 角色 | 默认 Provider |
| --- | --- |
| Designer | Codex |
| Maintainer | OpenCode |
| Adversary | Codex fresh session |
| Recorder | OpenCode |
| Verifier | Codex fresh session |

Provider 失败显示为缺失席位，不自动 fallback。Runner 保存 Provider、model、命令、
版本、Session ID、输入/输出 artifact、退出状态和错误。

生产运行乐观支持用户安装的客户端版本；不兼容在真实调用处 fail-fast。首次交付用
真实客户端认证，后续 CI 使用测试构建专属的程序化等价 Provider，不声称验证未来
外部版本。

## 6. Workdir 与 Session

```text
<worktrees-root>/meetings/<meeting-id>/<participant-id>/
├── workdir/          # 席位持久工作区
├── logs/
├── output/
├── home/             # 为后续显式环境隔离预留
└── provider-home/    # 为后续 Provider-local 配置预留
```

同一席位在整场会议复用 Workdir 和 Session；不同席位 Reveal 前不共享文件系统。
Reveal 后通过持久 Message、Proposal 和 Evidence 交换信息，不合并 Workdir。失败环境
进入 quarantined；GC 不得与 active run 竞争。Milestone 1 为了复用用户已经登录的真实
客户端，Provider 进程继承宿主环境与凭据；因此隔离保证只覆盖 Nemeton 管理的 Workdir
和 Session，不把预留的 `home/`、`provider-home/` 或角色提示词宣称为安全边界。

## 7. 状态机与持久化屏障

```text
draft
  → preparing
  → sealed_proposals
  → revealed
  → deliberating
  → recording
  → verifying
  → awaiting_human
  → concluded

任意外部阻断 → failed / needs_user_input / recovery_required
```

HTTP 写命令先追加事件并更新 projection；Daemon 在 committed barrier 之间异步推进。
CLI `run --wait` 只是异步 API 加观察循环。Daemon 重启从最后 committed barrier 恢复；
replay 绝不重新执行 Provider 或其他外部副作用。

## 8. 密封、Reveal 与讨论

每个 Design Agent 首轮只接收同一 Meeting Brief 和 Reality Bundle，不接收其他席位
输出。所有 Proposal 持久化成功并达到 barrier 后一次 Reveal。

Facilitator 是确定性程序：Milestone 1 为每个 cycle 建立一个关于“哪项有证据的
canonical design 回答 Meeting Brief”的 blocking Conflict，并把全部结构化 Proposal
及其 Claim/Evidence 作为持久材料交给席位。Reveal 后只围绕这一个 Conflict 分配讨论，
不让模型动态生成议程，也不采用无界群聊。更细的多 Conflict 提取不在本里程碑内。

## 9. 有界收敛

每个 Conflict 默认最多三轮。相关 Design Agent 每轮必须输出：

- `verdict`：accept、reject、conditional。
- Evidence refs。
- 具体异议和改变立场的条件。
- 本轮 amendment。

所有相关 Design Agent 接受同一结论且 Verifier 没有 blocking finding 时为 resolved。
第三轮后仍不满足则 Meeting 进入 `needs_user_input`，保存一份包含最终立场、Evidence
和明确问题的 Human Input Request。用户输入作为新事件恢复同一 Conflict，并开始新
的有界周期。协议保证每个周期一定终止，但不强迫产生虚假共识。

## 10. Recorder、Verifier 与人工选择

Recorder 只能把已有 Proposal、Message、Evidence 和 resolved conflict 编译成带引用
的 Synthesis/Meeting Result；无来源 Decision 是 blocking error。

Verifier 只能提交 finding。blocking finding 使 Recorder 生成有来源的新版本或会议
回到讨论，Verifier 不能直接改写结果。

Verifier clear 后，每个 Candidate Semantic Item 进入人工：

```text
proposed ──► selected
    ├──────► rejected
    └──────► deferred
```

Milestone 1 的 `selected` 只更新持久化候选集；不写目标 Git、不生成 `.nemeton/`、不
成为 active。语义激活属于 Milestone 2 governance integration。

## 11. HTTP、WebSocket 与 CLI

HTTP command 返回已持久化 projection；启动长流程返回 `202 Accepted`。核心入口：

- `POST /v1/projects/{id}/meetings`
- `POST /v1/meetings/{id}/start`
- `GET /v1/meetings/{id}`
- `POST /v1/meetings/{id}/inputs`
- `POST /v1/meetings/{id}/ratifications`
- `GET /v1/meetings/{id}/ws?after_sequence=N`

Milestone 1 WebSocket 应用帧分为 `runner_delta` 和 `domain_event`；状态变化和
`needs_user_input` 都由已提交的 `domain_event` 表达，不再复制第二种状态协议。
连接同时使用标准 ping/pong。慢客户端队列满时断开并要求用 sequence 恢复，不能阻塞
领域写入。

CLI 提供 `meeting create/start/run --wait/show/watch/answer/ratify`，全部通过同一
daemon API。未来 `nemeton ui serve` 可在 `127.0.0.1` 建立浏览器网关；当前 daemon
继续只监听 `0600` Unix Socket。

## 12. 可观察验收

### 12.1 单项能力

- Meeting event、projection 和 artifact refs 同事务成功或回滚。
- Current State 删除后从事件得到相同 canonical digest。
- Reality Bundle 精确还原 staged、unstaged 和 untracked 内容。
- 默认生成三个普通 Design Seat，Recorder/Verifier 不计入其中。
- 每席位 Workdir/Session 持久，Reveal 前不可通过 Nemeton 上下文看到他席输出。
- Codex/OpenCode 均能读写实验代码、执行命令、流式输出并恢复 Session。
- 三轮协议只产生 resolved 或 needs_user_input。
- Recorder 无来源决定和 Verifier blocking finding 都阻止结论。
- select/reject/defer 可 replay，selected 不产生 Git/网络副作用。
- WebSocket sequence 可补发；临时 delta 丢失不损坏 projection。

### 12.2 联合旅程

- 真实 dirty repository 完成 Reality Bundle → 三席 → Reveal → Conflict → Recorder →
  Verifier → Ratification → Human Choice，并能从结果追溯全部 Evidence。
- 无共识旅程第三轮后暂停，Human input 后从原 Conflict 恢复。
- 在每个持久化屏障前后终止 Daemon/Runner，不产生重复 Proposal、幽灵席位或假成功。
- replay 重建相同 Meeting/Current State digest，Provider/Git 写入调用次数为零。
- WebSocket 多客户端断线重连后观察相同 committed sequence。

### 12.3 历史回归

- Milestone 0 的 Project identity、Reality、Artifact、原子事务、restart 和 replay 测试
  原样继续通过。
- 真实 Milestone 0 数据库在 migration 前备份，升级后 Project result digest 不变。
- `scripts/semantic-gate` 在 Linux/macOS 保持唯一 Hard Gate；format、diff、vet、race
  tests 和 build 任一失败都阻止合并。

## 13. 明确非目标

- Web UI、Windows、Docker、远程 worker。
- Campaign、Task Contract、Task worktree、Task/Integration/Release Gate。
- 自动把 Candidate Semantic Item 激活到 Git。
- 对未来 Codex/OpenCode 版本作 CI 兼容性保证。
