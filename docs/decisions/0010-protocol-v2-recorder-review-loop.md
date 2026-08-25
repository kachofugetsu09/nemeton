# Decision 0010：Protocol v2 使用单 Recorder 驱动可迭代的人类审阅闭环

> 状态：Accepted
>
> 日期：2026-07-20
>
> 决策者：Human
>
> Amends：Decision 0009 的当前会议流程；落实 0001、0002、0003、0005

## 问题

一次性生成 Meeting Result 不能覆盖真实设计工作：用户可能只接受部分条目、询问细节、要求局部
补全，或否定状态 owner 等核心设计。让用户显式指定“回答、修订还是重开”会把协议判断责任推给
用户；让浏览器维护领域 reducer 又会产生第二事实源。

## 决定

- Local Web UI 和显式 Participants API 创建 protocol v2 Meeting：`0..N` 个平等 Design Agent、
  恰好一个 Recorder，不创建 Verifier；默认完整会议仍使用三个 Design Agent。未携带 Participants 的
  legacy CLI/API 创建路径暂时保留 protocol v1，直到 CLI 获得 roster 配置合同。
- Recorder 观看原始问题、Project Reality、完整会议、当前 Result、结构化处置、可选文字反馈和
  已批准项目上下文，自己选择最小正确动作：`answer`、`patch` 或 `reconvene`。
- `answer` 不改变 Result；`patch` 追加完整、自包含的新 Result revision；核心 owner、boundary、
  invariant、failure model 或已接受约束发生变化时必须 `reconvene`。
- 后续 Cycle 仍属于同一 Meeting，并复用原 roster、Participant identity、Provider、模型、options、
  Workdir 和 Provider Session。Recorder 为下一 Cycle 提供开场。
- 用户先逐项选择设计处置与上下文处置，再选择批准或继续；自然语言只补充选择，不能覆盖结构化命令。
- 持久化上下文进入数据库 Current State，供未来会议读取；被拒绝条目不得静默回流。它不写目标仓库、
  不生成 commit，也不成为已激活 Git 语义。
- 批准完整 Result 后生成确定性 Coding Design Handoff。本里程碑到此停止，不执行编码任务。
- Local Web UI 只消费同源 HTTP/WebSocket API；daemon、事件和 projection 继续拥有状态机与恢复。
- protocol v1 数据仍可读取和 replay，历史 Verifier 内容不删除、不伪造升级。

## 原因

Recorder 已拥有全程上下文，最适合判断反馈是澄清、局部缺口还是核心冲突。把决定持久化为事件，
可以在 daemon restart 和 replay 后保持 Result revision、用户选择、locked context 与 Cycle continuity；
浏览器只展示投影，不承担领域所有权。

## 后果

- Recorder 的 Provider 输出是外部信任边界，必须严格校验 action、完整 Result 和 durable source refs。
- 无 Design Agent 的会议允许 Recorder 直接基于 Project Reality 形成 Result；需要多方判断时由用户配置 Swarm。
- `approved context` 表示未来会议必须看到的数据库上下文，不等于 Decision 0005 中已经激活的项目语义。
- 真实 Provider 兼容性只在交付认证中验证；每次 CI 使用程序化 Provider 和浏览器确定性场景。

## 验证

- 直接批准可持久化选中上下文并生成 Handoff。
- 用户自然提问时 Recorder 能回答且 Result digest 不变。
- 局部验收缺口产生完整新 revision，旧 Result 不变。
- 部分接受、部分拒绝后，核心冲突由 Recorder 使用同一阵容重开下一 Cycle。
- restart/replay 不调用 Provider，不重复事件或外部副作用，Current State digest 保持一致。
- Playwright 验证 Provider 拖拽、移动端无横向溢出、会议讨论与 Proposal 分栏、逐项审阅与可选反馈。
