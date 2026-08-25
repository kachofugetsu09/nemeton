# Milestone 1B：动态 Swarm、Recorder 审阅闭环与 Local Web UI

> 状态：Accepted and implemented
>
> 日期：2026-07-20
>
> 决定：[`0010`](../decisions/0010-protocol-v2-recorder-review-loop.md)
>
> 合同：[`2026-07-20-milestone-1b.md`](../contracts/2026-07-20-milestone-1b.md)

## 1. 交付边界

Milestone 1B 完成从模糊问题到可供后续 Coding 消费的批准设计，不执行 Coding、不写目标仓库、
不创建目标仓库 commit。协议 v2 与既有 v1 数据并存；v1 的 Verifier 历史只读保留。

## 2. 用户旅程

```text
┌────────────────────┐
│ 选择 Project、问题 │
└─────────┬──────────┘
          ▼
┌──────────────────────────────────────┐
│ 拖入 0..N Design Agent + 1 Recorder │
│ Provider / model / thinking options  │
└─────────┬────────────────────────────┘
          ▼
┌──────────────────────────────────────┐
│ 同一 Reality 上提案、自然语言讨论   │
│ 中间：讨论                  右侧：Proposal │
└─────────┬────────────────────────────┘
          ▼
┌──────────────────────────────────────┐
│ Recorder：完整 Result + Candidates  │
└─────────┬────────────────────────────┘
          ▼
┌──────────────────────────────────────┐
│ 用户逐项处置 + 可选自然语言反馈      │
└──────┬──────────────┬───────────────┘
       │              │
       │ approve      │ continue
       ▼              ▼
┌──────────────┐  ┌──────────────────────┐
│ Handoff      │  │ Recorder 自主判断     │
│ + locked ctx │  │ answer/patch/reconvene│
└──────────────┘  └──────────┬───────────┘
                             └──► 回到审阅或下一 Cycle
```

## 3. 协议与收敛

- `answer`：追加 Recorder 对话内容，Result ID/digest 不变，回到 `awaiting_user_review`。
- `patch`：输出完整替代 Result 与带 durable refs 的 Candidates，追加 revision，旧内容不可变。
- `reconvene`：追加 Recorder opening，Cycle 加一，复用同一 Participant、Provider 配置、Workdir 和 Session。
- Recorder 决策经过通用协议校验；非法结构或试图用局部动作越过明确的核心语义反对时最多纠正一次，
  仍不合法就保留当前 Result 并回到用户审阅，不无限调用模型。
- 所有 Provider 结构化输出都在 daemon 边界解码和领域校验；不合格时使用同一 Session 最多纠正一次。
  原始 NDJSON 保留为运行证据，只有重新序列化的规范 JSON 能成为 Meeting Content；第二次仍不合格时
  fail-loud，不猜测修复畸形 JSON。
- 有界讨论达到轮数上限后，v2 交给 Recorder 保存 dissent/unknown 并形成可审阅 Result；不无限循环，
  也不要求用户先替 Agent 指定动作。
- 用户 `approve` 完整 Result 时，每个设计条目都必须接受；存在反对或稍后项时只能继续交给 Recorder。
  只有 `persist` 条目进入 Approved Context。
  `result_only`、`rejected`、`deferred` 均有明确状态，不能被自然语言覆盖。

## 4. 持久化与恢复

Migration 004 增加 protocol version、Participant model/options、Cycle、Result pointer、review disposition、
approved context 和 Handoff 所需投影。所有变化先成为 append-only event，再与 projection 和 Current State
digest 在一个 SQLite 事务中提交。Result 正文和 Provider 输出保存在 CAS。

replay 只执行 reducer：不调用 Provider、不启动新 Cycle、不写 Git、不重复 WebSocket domain event。
客户端重连携带最后 committed project sequence；daemon 从唯一事件事实源补发。

## 5. Local Web UI

- 只监听显式 loopback IP，拒绝 wildcard、hostname 和非 loopback 地址。
- 静态资源由 daemon embed，同源 API 与 WebSocket；Host、Origin、内容读取的 Meeting scope 在边界校验。
- 首页配置默认三个 Design Agent 和一个 Recorder；参与者配置折叠，Provider 可拖入，也可点击添加。
- 会议页的注意力中心只有自然语言讨论；`@seat` 直接出现在对话，讨论中引用 Proposal 时把内部 Content ID
  显示为 `@seat 的提案`，Proposal 的 Candidate Items 只在右栏。
- Result Review 与会议页分离；逐项选项是主操作，自然语言是可选补充；批准后可复制自包含
  Markdown Handoff。
- WebSocket runner delta 不触发 projection 全量刷新；committed domain event 在 40ms 窗口内合并刷新，
  保持准实时感受并避免长输出制造请求风暴。
- 视觉使用冷白、墨色和单一 cobalt accent 的 OKLCH tokens；正文限制行长，精确 transition，
  40/44px 交互目标和 reduced-motion，避免渐变、玻璃、徽章堆叠与无意义装饰。

## 6. API 增量

- `POST /v1/projects/open`
- `POST /v1/projects/{id}/meetings` 的 protocol v2 Participants 配置
- `POST /v1/meetings/{id}/reviews`
- `GET /v1/meetings/{id}/contents/{content_id}`，严格限定 Meeting scope
- `GET /v1/meetings/{id}/handoff`
- `GET /v1/meetings/{id}/ws?after_sequence=<n>` 继续承载 committed event 与临时 delta

## 7. 验收矩阵

| 层 | 独立能力 | 联合语义 |
| --- | --- | --- |
| Store/reducer | migration、event payload、projection、digest | restart/replay 后 review、Result 和 context 完全相同 |
| Runner | Codex/OpenCode model/options/session、NDJSON 终止信号 | Cycle 复用配置与 Session，结构错误有界纠正后 fail-loud，规范内容与原始证据分离 |
| Meeting | answer、patch、reconvene、approve | 部分接受/拒绝、locked context、Handoff 组合成立 |
| Handoff | JSON API、Markdown renderer | CLI/UI 导出的正文包含原始问题、批准 Result、digest 和长期上下文 |
| API/WebSocket | Host/Origin/schema/content scope | committed sequence 恢复且不重复副作用 |
| Acceptance harness | 独立 data dir、系统分配的 loopback port | 本机预览运行时 Gate 仍可重复执行且不互相污染 |
| Web UI | 拖拽、移动端、审阅选项 | 讨论/Proposal/Result attention 与 daemon 状态一致 |
| 兼容性 | protocol v1 可读可 replay | v1 Verifier 历史不影响 v2 无 Verifier 流程 |

仓库硬 Gate 为 `scripts/semantic-gate`：前端 lint/build/Playwright、Go format/vet/race tests 和二进制构建。
真实 Provider 套件是交付认证，不进入每次 CI。
