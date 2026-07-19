# Milestone 1B Dynamic Swarm Review Loop Agent Task Contract

> 状态：Approved
>
> 批准者：Human
>
> 批准日期：2026-07-20

## 1. Goal

- Objective：交付动态 Codex/OpenCode Swarm、单 Recorder、多 Cycle 用户审阅、Result revision、
  approved project context、Coding Design Handoff 和真实 Local Web UI 的完整设计闭环。
- Observable success：用户可拖拽配置 `0..N` Design Agent 与一个 Recorder，选择 Provider、模型
  和思考配置；Recorder 根据结构化审阅与可选文字自主 `answer`、`patch` 或 `reconvene`；用户能批准
  完整 Result，并选择哪些条目进入未来 Meeting 上下文；restart/replay 后状态与 digest 相同。
- Change type：docs、code、schema、runtime、Web UI、tests、CI gate、真实 Provider certification。
- Semantic delta：新增 Meeting protocol v2；v2 不创建 Verifier；同一 Meeting 的 Cycle 复用 roster、
  Workdir 和 Session；批准完整方案与持久化语义分离；本里程碑不写目标 repository。

## 2. Ownership

- Capability owner：daemon Meeting domain、Runner adapter、SQLite event/store、CAS、loopback UI gateway。
- Consumer scope：本地 Human、未来 Meeting 和只读 Coding Handoff consumer。
- Authoritative state owner：append-only domain events 和 CAS；projection、Current State、Web UI read model
  与 Handoff 均可重建。
- Runtime patch reason：动态 roster、Recorder review 和跨 Cycle continuity 拥有持久化与恢复不变量，
  不能由浏览器或 Provider session 单独拥有。
- Client alternative rejected：客户端编排会绕过原子事件、replay 和 Session/Workdir continuity。

## 3. Invariants and protected state

- v1 Meeting、Verifier 内容、Provider run、artifact 和 replay 历史不得删除或伪造升级。
- v2 每个 Cycle 使用相同 Participant identity、Provider、model、options、Workdir 和 Session。
- 用户结构化选择是权威命令；自然语言只补充，不能覆盖选择。
- Recorder 不能修改 locked approved context；核心 owner/boundary/invariant 改变必须 reconvene。
- Result revision 不可原地改写；Patch 创建新 revision。
- replay 不调用 Provider、写 Git、启动新 Cycle 或重复外部副作用。
- 主 checkout 和真实目标 repository 是受保护资源。

## 4. Persistence

- Append：protocol v2 Meeting/Participant、feedback、review、Recorder decision/opening、Result revision/Patch、
  approved result、semantic disposition、approved context、Handoff 和 certification evidence。
- Update：Meeting/Cycle/Participant run projection 与 Current State digest；只由事件 reducer 更新。
- Delete：无领域历史删除；认证临时凭据副本在认证结束后清理，不进入 artifact 或报告。
- Derived/rebuildable：Meeting snapshot、Result index、approved context、Handoff、UI read model。
- Rollback：保留失败数据库、artifact 和 worktree；migration 只前向，不通过删表伪装回滚。

## 5. Allowed scope and side effects

- Repository：`git@github.com:kachofugetsu09/nemeton.git`。
- Base：`9dd270aeb2b73de77c9d513b91184de6841b0efa`。
- Branch：`babel/milestone-1b`。
- Worktree：`/mnt/data/coding/nemeton-worktrees/milestone-1b`。
- Allowed paths：`cmd/**`、`internal/**`、`migrations/**`、`webui/**`、`certification/**`、
  `scripts/semantic-gate`、必要的 module/lock/workflow 文件和对应 docs。
- Allowed external effects：真实 Codex/OpenCode 调用、模型额度、认证临时目录、依赖下载、
  commit、push 和创建 PR。
- Forbidden：修改主 checkout、真实项目源码/数据库、凭据入库、真实模型进入每轮 CI、自动 merge、
  Campaign、代码 Task/worktree、Governance Writer、第三种 Provider、静默 fallback。

## 6. Validation

- Required gate：现有 `semantic-gate` 扩展为 Go format/diff/vet/race/build、前端 lint/build/test、
  protocol v1 regression 和 v2 deterministic journey；required check 名称保持不变。
- Real certification：固定场景 `direct approve`、`review answer/patch/partial context/reconvene` 和
  `locked invariant conflict`；使用显式 `opencode-go/deepseek-v4-pro` 和低成本 Codex 模型。
- Real calls are delivery-only：未来 PR CI 使用程序化 Provider；真实套件只在交付或客户端协议变化后运行。
- 认证隔离：后续用户指令取消 Docker；真实调用只使用 fixture repository、Nemeton 管理的临时 Workdir
  和现有 Provider 凭据，不挂载或修改主 checkout。报告记录版本、模型、fixture/result digest 和断言，
  不记录凭据或隐藏推理。
- UI review：功能完成后依次应用 better-colors、better-typography、better-ui 和 kill-ai-slop；会议自然语言
  与当前待决内容必须保持 attention 核心。

## 7. Delivery

- Delivery target：完整实现、验证证据、commit、push 和 GitHub PR。
- Merge：未授权；PR checks 通过后报告给 Human。
- Backup：`/mnt/data/coding/nemeton-backups/20260720-before-milestone-1b/`。
- Stop conditions：必须删除历史、真实客户端无法使用指定模型、无法保持 v1 replay、必须修改主 checkout、
  或语义判断只能靠静默 fallback 才能通过。
