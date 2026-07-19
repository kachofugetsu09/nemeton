# Nemeton 持久化状态地图

> 文档状态：Accepted design；Milestone 0 current implementation
>
> 核对日期：2026-07-19

当前仓库已经实现数据库、artifact store 和 Project/Reality projection；其他领域状态仍是后续设计。本地图描述
Accepted decisions 0003、0004 及现有产品语义要求实现保持的状态边界；运行路径
已经由 Human 确认。

## 1. Domain event stream

| 字段 | 语义 |
| --- | --- |
| 名称与位置 | 应用数据目录的 `nemeton.db` 中的 `project_streams`、`domain_events` 和 `event_artifacts` |
| 角色 | Project、Reality、Meeting、Semantic Item、Campaign、Task Contract、Gate 与 Evidence 的长期权威事实 |
| Owner | `nemetond` 领域服务和事件存储；其他进程只能通过 command/API 提交 |
| 增加 | 通过校验的领域 command 在短事务中按 project sequence 追加事件 |
| 更新 | 已记录事件正文和顺序不允许原位修改；schema 演化通过版本和迁移处理 |
| 逻辑失效 | 新事件表达 supersede、withdraw、cancel、terminal state 或 recovery finding |
| 物理删除 | 当前没有授权策略；在新 Accepted decision 前，Agent 和客户端均无权删除历史事件 |
| 重建 | 事件本身不是派生物；可用它和 artifact 重建投影，但不能从投影反推完整事件历史 |
| 备份恢复 | SQLite backup API、integrity check、事件尾部和 artifact 引用共同定义恢复点 |
| 待确认 | 保留期、用户主动撤销或合规删除的精确协议 |

## 2. Content-addressed artifact store

| 字段 | 语义 |
| --- | --- |
| 名称与位置 | 应用数据目录中的 `artifacts/sha256/` |
| 角色 | 保存会议正文、日志、补丁、测试报告、Evidence 和已激活语义内容 |
| Owner | artifact 模块；事件和领域对象只保存 digest、媒体类型、大小与稳定引用 |
| 增加 | 写入完整内容、计算 digest、校验落盘后登记引用；相同 digest 可复用同一内容 |
| 更新 | 内容寻址对象不可原位修改；内容变化产生新 digest |
| 逻辑失效 | 上游事件可停止引用或标记 Evidence 失效；旧对象不因此自动删除 |
| 物理删除 | 当前没有 GC 和保留策略；模型输出等内容可能不可复现，不能仅因“当前未引用”删除 |
| 重建 | 只有原始内容仍存在于其他已校验来源时才能按相同 digest 重建；不能依赖模型再次生成相同正文 |
| 备份恢复 | 必须与事件数据库的引用集合一致备份；恢复后逐项校验 digest |
| 待确认 | GC、加密、配额和敏感数据撤销策略 |

## 3. SQLite current projections

| 字段 | 语义 |
| --- | --- |
| 名称与位置 | `projects`、binding、reality、meeting、semantic、campaign、task、gate、trust 等关系表 |
| 角色 | 当前查询视图和运行协调状态；不是长期语义的第二事实源 |
| Owner | `nemetond` 的 reducer、领域服务和 projection 模块 |
| 增加 | 事件事务内创建或扩展当前行；索引随 schema migration 建立 |
| 更新 | reducer 按事件推进 current state、sequence、digest 和 dirty 状态；业务服务维护需要当前现场的协调字段 |
| 逻辑失效 | status、supersedes、terminal state、stale、quarantined 等显式表达 |
| 物理删除 | 仅由明确的 replay、迁移或维护操作执行；客户端和 Agent 不得直接操作数据库 |
| 重建 | 领域投影由固定 reducer、事件和 schema 确定性重建；运行现场字段需要 reconcile，不能由旧事件伪造 |
| 备份恢复 | 数据库备份后执行 integrity check；replay 对账规范化结果、sequence 和 digest |
| 待确认 | migration 兼容窗口、投影重建命令和大库维护策略 |

## 4. File and Git projections

| 字段 | 语义 |
| --- | --- |
| 名称与位置 | Nemeton 当前视图文件，以及目标仓库未来的 `.nemeton/` |
| 角色 | `NOW.md`、Meeting Result、Trust State 是当前投影；`.nemeton/` 是已激活语义的 Git 发布投影 |
| Owner | NOW/file projector；静态语义由 governance worktree 生成并通过 integration history 激活 |
| 增加 | 事件提交后写临时文件、`fsync` 并原子 rename；新语义条目进入新的 governance change |
| 更新 | 投影文件可以由唯一 projector 原子替换；active Semantic Item 不原位改写，创建新版本并 supersede |
| 逻辑失效 | projection dirty、stale、superseded 或新的 Git revision 表达 |
| 物理删除 | 只允许 projector 恢复、明确 replay 验证或经批准的 governance change；不能把“可重建”当作通用删除授权 |
| 重建 | 从 domain events、schema 和已校验 artifact digest 确定性重建；replay 不执行外部副作用 |
| 备份恢复 | 事件与 artifact 是恢复依据；Git history 保存已发布版本。源代码仍需 Git object、remote、bundle、commit 或 patch |
| 待确认 | `.nemeton/` 首次写入时机以外的默认 governance commit/PR 政策 |

## 5. Repository code and user checkout

| 字段 | 语义 |
| --- | --- |
| 名称与位置 | 用户选择的 Git repository、主 checkout 和 linked worktrees |
| 角色 | 源代码和提交历史的权威载体；主 checkout 同时是受保护的用户工作区 |
| Owner | Git 历史由获授权的交付流程修改；Nemeton worktree 模块只管理自己创建并记录的 worktree |
| 增加 | 经 Task Contract 授权的 writer 在独立 worktree 产生文件、commit、delivery 或 PR |
| 更新 | 修改必须绑定准确 base SHA、write scope、lease 和验证证据 |
| 逻辑失效 | branch supersede、delivery cancel、stale task 或 quarantine；不静默丢弃 diff |
| 物理删除 | 只有在 managed root、identity、branch、task、token 和终态均可证明时清理 managed worktree；永不递归删除用户主 checkout |
| 重建 | 代码不能由自然语言事件可靠重建；依赖 Git objects、remote、bundle、commit、patch 或 delivery artifact |
| 备份恢复 | 修改前保护 dirty state；持久交付以 Git 和额外对象备份为准，恢复后核对 SHA 与 diff |
| 待确认 | managed worktree root、remote、integration branch 解析和本地无 remote 时的交付政策 |

## 6. Runtime continuity and diagnostics

| 字段 | 语义 |
| --- | --- |
| 名称与位置 | PID、进程句柄、SSE 连接、临时锁、heartbeat 当前值、外部 Git/进程现场和运行日志 |
| 角色 | 运行连续性、诊断证据或可重新探测的现场，不是可 replay 的长期语义 |
| Owner | runner、worktree、API、进程监督和 reconcile 模块分别拥有其现场 |
| 增加 | 进程启动、连接建立、命令执行和观测事件产生当前记录或 artifact |
| 更新 | heartbeat、lease 观测和进程状态允许原位变化；重要变化可以追加审计事件 |
| 逻辑失效 | timeout、expired、released、failed、quarantined 或进程结束明确表达 |
| 物理删除 | 临时现场可在 owner 确认终态后清理；未保存日志、diff 或 Evidence 前不得清理 |
| 重建 | 从 Git、文件系统和进程表重新探测；旧 PID、heartbeat 或事件不能恢复成有效写权限 |
| 备份恢复 | 保存诊断 artifact、真实退出码和未捕获 diff；恢复后签发新 fencing token 并显式记录 finding |
| 待确认 | 日志保留期、进程监督实现和 crash recovery 的精确时限 |

## 7. 共同恢复不变量

- replay 只运行确定性 reducer，不执行 Git 写命令、模型调用、外部 API、PR、部署或 Gate command。
- artifact 缺失、digest 不符或未知事件版本必须使相关恢复明确失败。
- “可重建”不改变删除授权；恢复输入也必须被备份和校验。
- 数据库事件、artifact、Git objects 和外部现场分别恢复，不能用其中一种伪装其他种类已经恢复。
