# Nemeton 系统上下文

> 文档状态：Current repository facts + Accepted product design
>
> 核对日期：2026-07-20

本文把当前仓库能够证明的事实，与尚未实现但已经 Accepted 的产品设计分开。详细领域模型继续以 `projectneed.md` 和两份 Spark 设计为准。

## 1. Current repository facts

- 源码仓库位于 `/mnt/data/coding/nemeton`。
- 初始基线为 `53fc381cc9f4f76a7844ff964bd3fc86f175dff8`，分支为 `main`。
- Git remote `origin` 为 `git@github.com:kachofugetsu09/nemeton.git`。
- 初始基线只有 README 和文档。当前仓库已经实现 Go module、SQLite event/projection、Project Reality、持久化 Swarm Meeting、Codex/OpenCode Runner、隔离 Workdir、HTTP/WebSocket、CLI、Local Web UI、真实测试和 Linux/macOS CI；当前没有 Docker 运行形态或认证 Dockerfile。
- `.codegraph/` 不存在；是否建立索引由用户决定。

这些事实描述初始化时点，不是长期产品要求。以后判断当前实现必须重新读取 Git、文件和运行证据。

## 2. User-confirmed intent

Nemeton 是面向 Agent 原生软件项目的本地语义控制平面。它把项目现实、会议、语义激活、任务合同、隔离执行、硬性验收和失败反馈连接成可追踪、可重放的项目历史。

v1 已确认：

- 本地优先、单用户、单仓库、多 worktree。
- Go 模块化单体，系统 Git，本机 SQLite。
- `nemetond` 是领域状态与 SQLite 的唯一逻辑写入者。
- 语义控制面使用 append-only 领域事件；当前视图是投影。
- 人类激活产品语义；系统 Gate 验收实现，Reviewer 只提供 finding 和 Evidence。
- Agent 默认只在 Nemeton 管理的 worktree 中执行写入。

## 3. Accepted target context

```text
                    ┌───────────────────────┐
CLI / Local Web UI ─►│       nemetond        │◄── Agent / Model Runner
                    │ command + state owner │
                    └───────┬───────┬───────┘
                            │       │
                    ┌───────▼───┐   └────────► system Git / external tools
                    │  SQLite   │
                    │ events +  │
                    │ projections│
                    └───────┬───┘
                            │ artifact refs
                    ┌───────▼────────┐
                    │ artifact store │
                    └───────┬────────┘
                            │ product state / future publish projector
             ┌──────────────┴──────────────┐
             ▼                             ▼
 Current State / Meeting          target Git repository
    / future Trust                code + future active .nemeton/
```

图中的 CLI、Local Web UI、daemon、SQLite event/projection、artifact store、只读 target Git、Meeting、
Codex/OpenCode Runner、Approved Context、Handoff、数据库 Current State 和 WebSocket 已在 Milestone 0/1
实现；Campaign/Task 执行、产品 Gate、Trust State 和 `.nemeton/` 发布仍是 Accepted design。

## 4. Capability 与状态 owner

| 能力或状态 | Owner | 消费者 | 权威边界 |
| --- | --- | --- | --- |
| 领域 command、状态机与事务 | `nemetond` 内的领域服务 | CLI、Local Web UI、Agent Runner | 客户端不能复制状态机或绕过 daemon 写库 |
| 语义历史 | append-only domain event stream | reducer、projector、审计与 replay | 事件和 artifact refs 是长期事实；current table 不是第二真源 |
| 大正文与证据 | content-addressed artifact store | event、Gate、会议和恢复 | digest 校验失败必须使证据失效 |
| 当前关系视图 | SQLite projections | API、UI、调度器 | 由 reducer 生成，可重建但不能任意删除 |
| 已发布语义 | 激活事件指定的 artifact；Git 中 `.nemeton/` 为发布投影 | 编码 Agent、Git、未来 CI | selected 尚未进入 integration history 时不能标为 active |
| Git/worktree 生命周期 | worktree 模块 + system Git | scheduler、Agent Runner | metadata 操作串行；写任务使用 lease 和 fencing token |
| 外部模型和编码 Agent | Provider/Runner 窄边界 | Meeting、Campaign | 缺失依赖或失败必须暴露，不能生成替代意见或假结果 |
| Babel Skills | 用户级 Codex 环境 | 仓库协作流程 | 仓库只声明调用合同，不复制 Skill 或固定本机路径 |

## 5. Repository、worktree 与运行 workspace

- 源码 repository 保存代码、工作手册和已发布的静态语义投影。
- 用户主 checkout 是用户工作区，不是默认 Agent 执行资源。
- Meeting Agent 已使用从 Reality Bundle 创建的独立持久 Workdir；未来 governance 和 task writer 使用从准确 base SHA 创建的 Nemeton 管理 worktree。
- Nemeton 应用数据、SQLite、artifact、lease metadata 和运行日志位于仓库外的受控应用数据目录。
- 缓存、临时文件和进程现场不是长期语义真源；能够删除不代表无需明确 owner 和安全条件。

## 6. Trust boundaries

HTTP、CLI、Provider、Git、数据库反序列化、artifact 读取和外部 command 是信任边界，负责结构校验与错误转换。领域服务拥有状态机和当前业务约束。reducer 只接受已经验证的领域事件；内部契约破坏时 fail-fast、fail-loud。

## 7. 未知

仓库设置、平台、目录、integration branch 和首批执行适配器已经由 Human 确认，
见 [`milestone-0-foundation.md`](milestone-0-foundation.md) 和 `projectneed.md` 第 22 节。
仍未确认的是 governance change 使用本地 commit 还是 PR 激活的默认政策。
