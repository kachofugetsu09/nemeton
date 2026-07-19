# Decision 0006：v1 使用 Go 模块化单体和系统 Git

> 状态：Accepted
>
> 日期：2026-07-19
>
> 决策者：Human

## 决定

Nemeton v1 使用 Go 编写本地控制面。一个 `nemetond` 进程拥有 SQLite 写入、领域状态机、Git/worktree 管理、Agent 子进程监督、Gate 执行和 HTTP/SSE API。`nemeton` CLI 通过同一应用服务执行初始化、诊断和脚本化操作。

Git 操作调用系统 `git`，不使用 go-git 重新实现 worktree、branch、fetch 和 merge 语义。

## 原因

v1 的主要工程问题包括：

- 长生命周期 daemon。
- 多个 Git 和 Agent 子进程的启动、流式输出、取消和回收。
- 并发状态机、lease 和 fencing token。
- SQLite 事务和崩溃恢复。
- 单机分发和较少的运行时依赖。

Go 可以用一个运行时承载这些职责。Python 研究工具仍可通过带版本 schema 的外部 Tool 接入，失败必须向 Runner 暴露。

## 模块边界

- 领域 command 和 reducer 放在清楚的 Go package 中。
- HTTP、CLI、Provider、Git、artifact 和数据库反序列化负责边界校验。
- Git、Provider、时钟和 command runner 使用窄接口。
- 每张表不单独创建 repository interface。
- v1 不拆微服务，也不增加 Go 与 Python 两套控制面。

## 系统 Git 约束

- 命令使用参数数组，不能拼接 shell 字符串。
- 每次调用保存 executable、args、cwd、时间、退出码和输出 artifact。
- Git metadata 变更使用 repository-scoped OS lock。
- Agent 在已经创建好的不同 worktree 中并行运行。
- Git 命令失败必须返回真实退出状态，不能切换到内存模拟或假成功路径。

## 验收

- daemon 和 CLI 共享领域服务与事件语义。
- 退出码、取消和子进程回收可以被集成测试观察。
- 同一 repository 的 Git metadata 操作不会并发破坏 common-dir。
- 外部 Tool 缺失或 schema 错误时，相关 run 明确失败。
- Web UI 的加入不需要复制业务状态机。
