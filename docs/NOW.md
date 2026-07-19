# Nemeton 当前状态

> 更新日期：2026-07-19
>
> 当前阶段：Milestone 0 已实现并由仓库级 Hard Gate 保护
>
> 临时说明：Nemeton 自身的 projector 完成前，这份文件由人维护。随后由事件流生成，Agent 不得并发编辑。

## 当前结论

产品边界和 Milestone 0 实现合同已经落地。仓库现有实现建立了项目身份、Project Reality、SQLite 事件事实源、artifact、投影和 replay；Swarm、模型 Provider、worktree 执行和 UI 仍排在后续里程碑。

当前实现合同是
[`contracts/2026-07-19-milestone-0.md`](contracts/2026-07-19-milestone-0.md)，
详细设计是
[`design/milestone-0-foundation.md`](design/milestone-0-foundation.md)。
本里程碑同时按 Accepted Decision 0008 初始化仓库级 `semantic-gate`，使原子事务和
Git Reality、Artifact、Event、Projection、Replay 的联合语义成为 `main` 的 required
CI check。

## 当前约束来源

本里程碑受 [`decisions/README.md`](decisions/README.md) 中的 Accepted decisions 约束，尤其是事件事实源、SQLite 单写入者和 Go 模块化单体决定。`NOW.md` 不复制长期决定正文，也不能改变其语义。

## 已交付：Milestone 0

```text
用户指定本地 Git 路径
          │
          ▼
解析 top-level / common-dir / HEAD / tree / branch
          │
          ▼
识别或创建稳定 Project ID
          │
          ▼
追加 ProjectRegistered / RepositoryBound / RealityCaptured
          │
          ▼
Reducer 生成 ProjectProjection
          │
          ▼
project inspect 返回当前状态
          │
          ▼
删除投影并 replay，得到相同结果
```

### 交付事实

- Go module。
- `nemetond` 和 `nemeton` 两个入口。
- SQLite migration runner。
- append-only domain event store。
- Project、Repository Binding、Reality Revision 的 reducer 和投影。
- `nemeton project open <path>`。
- `nemeton project inspect <project-id>`。
- daemon single-instance lock。
- 启动时的只读 reconcile 骨架。
- `scripts/semantic-gate` 和 Linux/macOS GitHub Actions Hard Gate。

### 持续验收

- 主 checkout 和 linked worktree 返回同一 Project ID。
- 路径移动后可以显式 relink，Project ID 不变。
- dirty checkout 可以被观察，dirty 内容不能进入 committed Reality。
- 非 Git 目录、损坏 common-dir 和歧义 integration branch 明确失败。
- 全流程不修改用户选择的仓库。
- daemon 重启后 Project 与 Reality 保持一致。
- 删除 Project/Reality 投影后，replay 能重建相同的规范化状态和 digest。
- replay 不执行 Git 写操作、外部 API、模型调用或其他副作用。
- 事件追加与投影更新在同一事务中；真实投影失败不得留下事件或推进 stream version。
- Git Reality、artifact、event、projection、reconcile 和 replay 的联合语义由真实边界测试覆盖。

## 已确认的仓库设置

- Go module：`github.com/kachofugetsu09/nemeton`；最低 Go `1.25.0`。
- 最低 Git `2.41.0`。
- Linux、macOS 正式支持；其他 Unix best effort；Windows 不支持。
- 应用数据目录遵循 XDG/macOS Application Support，并支持 `NEMETON_DATA_DIR`。
- managed worktree root 默认为 `$HOME/nemeton-workspaces`，支持
  `NEMETON_WORKTREES_ROOT`；Milestone 0 不创建工作树。
- integration branch 只从显式本地分支、唯一 local remote HEAD 或唯一本地分支确认，
  无法唯一判断时失败。

## 当前禁止扩张的范围

- 不实现 Swarm Meeting。
- 不接模型 Provider。
- 不创建 task worktree。
- 不创建 PR。
- 不实现 PostgreSQL backend。
- 不构建 Web UI。
- 不引入消息队列、向量数据库、微服务或通用插件系统。
- 不为未来数据库编写抽象 repository 层。

## Milestone 0 完成后的下一步

Milestone 0 完成后先实现 Codex 与 OpenCode 的 provider-native Runner 边界，再进入
Milestone 1 的单人会议、发言、候选语义和 `NOW` 投影。Runner 不提前进入当前
里程碑。
