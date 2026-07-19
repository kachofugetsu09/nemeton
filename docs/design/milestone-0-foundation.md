# Milestone 0：Project Reality 与持久化内核

> 状态：Accepted
>
> 批准者：Human
>
> 批准日期：2026-07-19

本文固定 Milestone 0 的实现设计。长期产品语义仍以 Accepted decisions 和
`docs/projectneed.md` 为准；本设计只展开第一个可执行切片。

## 1. 目标与边界

Milestone 0 走通下面的本地闭环，且不修改被观察仓库：

```text
project open
  -> Project stream events
  -> Project / Binding / Reality projections
  -> project inspect
  -> project-scoped replay
  -> 相同规范化 digest
```

这一阶段不创建工作树，不调用模型，不实现 Meeting、Campaign、Gate、Runner、
Provider 或 Web UI，也不向目标仓库写入 `.nemeton/`。Codex 和 OpenCode Runner
是紧随 Milestone 0 的下一实现重点，但本阶段不为它们创建空抽象。

## 2. 平台与目录

- Go module：`github.com/kachofugetsu09/nemeton`。
- 最低 Go：`1.25.0`。
- 最低 Git：`2.41.0`。
- Linux 和 macOS 正式支持；其他 Unix best effort；Windows 不支持。
- SQLite 只能位于本机文件系统；NFS 和云同步目录不受支持。

应用数据目录优先级为 `--data-dir > NEMETON_DATA_DIR > OS default`：

```text
Linux: ${XDG_DATA_HOME:-$HOME/.local/share}/nemeton
macOS: $HOME/Library/Application Support/Nemeton
```

数据目录布局：

```text
<data-dir>/
├── nemeton.db
├── nemetond.lock
├── nemetond.sock
├── artifacts/sha256/
└── backups/
```

目录权限为 `0700`，数据文件为 `0600`。已存在但 group/other 权限过宽的目录
使启动失败，不由 daemon 擅自改权。能够识别的网络文件系统被拒绝；云同步目录
无法在所有平台可靠识别，因此保留为明确的支持边界。

Managed worktree root 默认为 `$HOME/nemeton-workspaces`，可由
`NEMETON_WORKTREES_ROOT` 覆盖。Milestone 0 只解析和展示该路径，不创建目录或
工作树。

## 3. 进程与 API

```text
┌──────────────────────┐
│ nemeton CLI          │
└──────────┬───────────┘
           │ HTTP/JSON over Unix socket
           ▼
┌─────────────────────────────┐
│ nemetond                    │
│ API -> command -> transaction│
└──────────┬──────────────────┘
           │ sole writer
           ▼
┌─────────────────────────────┐
│ SQLite + artifact store     │
└─────────────────────────────┘
```

`nemetond` 是 SQLite 和 Artifact metadata 的唯一逻辑写入者。普通 CLI 命令
只通过 Unix Socket 调用 API；`nemeton daemon start` 与 `nemetond` 进入同一个
前台服务入口，不后台化、不自动拉起子进程。

API：

```text
GET  /v1/health
POST /v1/projects/open
GET  /v1/projects/{id}
POST /v1/projects/{id}/relink
POST /v1/projects/{id}/replay
```

API 拒绝未知 JSON 字段、尾随内容、超限请求和不合法路径、ID、分支名。数据目录
权限提供本地访问边界，不提供 TCP fallback。错误使用稳定的 problem JSON code；
CLI 用法错误退出 `2`，运行失败退出 `1`。

## 4. daemon 生命周期

启动顺序：

1. 解析数据目录并验证权限、平台和文件系统支持边界。
2. 获取数据目录级单实例锁。
3. 只在持锁后处理可安全证明为残留的 Unix Socket。
4. 打开 SQLite；已有数据库需要迁移时先使用 SQLite backup 能力备份。
5. 执行嵌入式连续 migration。
6. 运行只读 reconcile。
7. 绑定 Unix Socket 并开始服务。

锁持有整个进程生命周期。Socket 路径若被普通文件、目录或无法安全判断的对象
占用，启动失败，不自动删除。`SIGINT` 和 `SIGTERM` 会停止接收请求、等待在途事务、
关闭数据库、移除 Socket，最后释放锁。

Reconcile 将失败分为两类：

```text
事件、schema 或 Artifact 不可恢复损坏
  -> hard failure，不提供 replay

投影缺失、落后或 digest 不一致
  -> maintenance mode
     ├── health/status 可用
     ├── project replay 可用
     └── 其他 command 返回 replay_required
```

Project replay 成功并重新核对后，daemon 从 maintenance 切回 ready。Reconcile
不自动 replay、不删除 orphan Artifact，也不把损坏状态归一化成成功。

## 5. Project 与 Repository Binding

Project ID 由 Nemeton 随机生成，不使用路径、remote 或分支作为主键。Repository
identity 是规范化、解析符号链接后的 Git common-dir；主 checkout 和 linked
worktree 因共享 common-dir 而解析到同一 Project。

Git 发现只使用系统 Git argv 调用，不拼 shell 字符串。调用设置
`GIT_OPTIONAL_LOCKS=0`，对工作区观察禁用 fsmonitor 和 untracked cache；不 fetch、
不访问网络、不更新 ref、remote HEAD、index 或 config。bare repository 不受支持。

首次绑定的 integration branch 按以下顺序解析：

1. `--integration-branch` 指定的合法本地分支；
2. 恰好一个有效 local remote HEAD 指向的同名本地分支；
3. 恰好一个本地分支；
4. 否则列出候选并失败。

不使用当前 checkout 分支，也不猜测 `main` 或 `master`。已有 binding 始终使用已
确认分支；路径移动、分支消失或改名必须通过显式 relink。新 common-dir 已绑定
其他 Project 时 relink 失败。

## 6. Reality Revision 与 Artifact

Reality 固定 integration branch 的 committed commit 和 tree。当前 checkout dirty
状态只作为观察元数据；未提交正文永不进入 Reality 或 Artifact。同一 commit/tree
在 clean 和 dirty checkout 中得到相同 Reality Artifact digest。

Reality Artifact 是规范化 JSON，至少包含：

- integration branch、commit 和 tree；
- committed Git tree 的路径、mode、object type 与 object ID 清单；
- 确定性识别的 `AGENTS.md`、`CLAUDE.md`、README、`docs/`、关键构建配置和 CI
  配置；
- 从仓库已提交配置中可以证明的检查能力；
- 无法证明的检查项，以 `unknown` 明确记录。

Artifact 使用 SHA-256 内容寻址，先以 `0600` 写临时文件，完成 digest 校验和
`fsync` 后原子 rename，再在数据库事务中登记引用。相同 digest 复用已有内容。
已存在内容与 digest 不符时判定损坏并失败。写文件后事务失败可能留下 orphan；
reconcile 只报告，不自动删除。

## 7. 事件、事务与投影

每个 Project 拥有独立 stream。`(project_id, sequence)` 唯一且单调递增，command
使用 `expected_stream_version` 检测并发变化。

Milestone 0 事件：

- `ProjectRegistered.v1`
- `RepositoryBound.v1`
- `RepositoryRelinked.v1`
- `RealityCaptured.v1`

Artifact 通过 `event_artifacts` 与 `RealityCaptured.v1` 关联，不创建独立的
`ArtifactRecorded` 领域事件。Event envelope 保留 actor、causation、correlation、
schema version、occurred_at 和 recorded_at。

```text
BEGIN IMMEDIATE
  -> 检查 expected_stream_version
  -> 追加事件
  -> 用同一套 reducer 更新投影
  -> 更新 project stream 与 projection state
COMMIT
```

事件正文和顺序只能追加，不能更新或删除。数据库不创建 foreign key；唯一约束、
必要索引、领域服务和 reconcile 共同维护关系。未知事件或 schema version 使读取、
replay 和恢复明确失败。

## 8. Project replay

`nemeton project replay <project-id>` 在单个事务中只重建该 Project 的 Project、
Binding、Reality 和 Artifact metadata 投影：

1. 保留 domain events、project stream 和 Artifact 文件。
2. 清空该 Project 的可重建投影。
3. 按 project sequence 运行与在线路径相同的 reducer。
4. 生成规范化状态和 SHA-256 result digest。
5. 全部成功后提交，否则回滚原投影。

Replay 不调用 Git、模型、外部 API、PR、部署或 Gate，不生成新领域事件，也不创建、
删除或改写 Artifact 文件。

## 9. 验收证据

验收使用真实临时 Git repository、SQLite、Unix Socket 和构建后的 daemon/CLI，
不使用 mock 或假成功路径。必须证明：

- 主 checkout 与 linked worktree 返回同一 Project ID；
- 显式 relink 保留 Project ID；
- dirty sentinel 不存在于 Artifact，且 clean/dirty digest 相同；
- 非 Git、bare、损坏 common-dir 和歧义分支明确失败；
- daemon 重启后状态一致，第二实例失败；
- 删除投影后进入 maintenance，Project replay 后恢复 ready；
- replay 前后规范化状态与 digest 相同；
- Project open 和 replay 前后，目标仓库文件、refs、index、config 均不改变；
- Artifact 缺失、篡改、未知事件或 future schema fail-fast。

Linux 和 macOS 都运行真实 runtime 测试。仓库新增
`scripts/semantic-gate` 和 `.github/workflows/semantic-gate.yml`，本地与 CI 使用
同一入口，覆盖 format、vet、race、build 和本节全部联合语义。Workflow 不设置
path filter，并将两平台结果汇总为稳定检查名 `semantic-gate`；上游 failed、cancelled
或 skipped 都使汇总失败。

GitHub `main` 把 `semantic-gate` 配置为 strict required status check，对管理员同样
生效，并禁止 force-push 和 branch deletion。当前 Milestone 0 PR 在合并前也必须
得到该检查的真实绿色结果。这里建立的是 Nemeton 仓库自身的 bootstrap CI Gate，
不是未来 Nemeton 产品的通用 Gate engine；Docker 仍不在本里程碑范围。
