# Decision 0008：原子语义与跨组件联合语义由 CI Hard Gate 保护

> 状态：Accepted
>
> 日期：2026-07-19
>
> 决策者：Human
>
> Amends：Decision 0007 的“当前尚未初始化自动语义 Gate”能力边界

## 问题

Milestone 0 的 Project、Git Reality、Artifact、事件流、投影和 replay 共同形成一条
语义链。一次实现通过本地测试，不能保证未来提交仍然维持事务原子性和跨组件联合
语义；普通非强制测试也不能阻止破坏性变更进入 `main`。

## 决定

Nemeton 从 Milestone 0 起建立仓库级 Semantic CI Gate：

```text
每个 PR / main push
        │
        ├─ Linux 真实全链路
        ├─ macOS 真实全链路
        ├─ SQLite 事务原子性
        ├─ Git → Reality → Artifact → Event → Projection → Replay
        ├─ 目标仓库零写入证明
        └─ format / vet / race / build
                 │
                 ▼
      required check: semantic-gate
```

- 本地与 CI 使用同一个 `scripts/semantic-gate` 入口。
- Semantic tests 使用真实 Git、SQLite、Unix Socket、daemon 和 CLI，不使用 mock、
  simulation 或假成功路径。
- 支持平台缺少必要 Git、构建器或系统能力时测试失败，不允许 skip 后标绿。
- CI 不使用 path filter；文档、依赖和 workflow 变更也必须通过 Gate。
- Linux 和 macOS 的结果由一个稳定名称 `semantic-gate` 汇总；任何上游 job 失败、
  取消或 skipped 都使汇总失败。
- GitHub `main` 将 `semantic-gate` 配置为 strict required status check，并对管理员
  生效；禁止 force-push 和 branch deletion。
- Gate 结果只对准确 commit SHA 有效；旧 SHA 的绿色结果不能授权新提交。

## 原子与联合语义

Gate 至少保护：

- event append、project stream sequence 和 projection update 同事务提交或全回滚；
- `expected_stream_version` 冲突不产生部分写入；
- dirty checkout 正文不进入 committed Reality Artifact；
- Project identity 在主 checkout、linked worktree 和显式 relink 间稳定；
- replay 使用相同 reducer，重建相同规范化状态与 digest；
- replay 不执行 Git、Artifact 或外部系统写入；
- maintenance 只允许安全的 health/status/replay 路径；
- Artifact、event schema 或数据库损坏不能被伪装成 ready。

## 原因

这些不变量跨越多个 package 和持久化 owner。只在各组件内部做单元检查，会遗漏
事务边界、协议组合和真实进程生命周期。把真实联合场景设为 required CI check，
才能让未来提交在进入 `main` 前重新证明组合系统仍成立。

## 后果

- Milestone 0 合同允许新增测试、`scripts/semantic-gate` 和 GitHub Actions workflow。
- 当前仓库仍不实现 Nemeton 产品内部的通用 Gate engine；本决定只建立 Nemeton
  仓库自身的 bootstrap CI Gate。
- GitHub Actions 或支持平台不可用时，PR 保持不可合并；不能手工把未知结果改成绿。
- 修改 Gate 本身仍要由未修改前的 required check 验证，并接受准确 diff Review。

## 验证

- PR 上存在唯一稳定 required check `semantic-gate`。
- Linux 或 macOS job 人为失败时，汇总 job 失败。
- 原子性测试能够通过真实 SQLite abort 证明事务全回滚。
- 联合测试通过真实二进制完成 open、inspect、restart、maintenance 和 replay。
- branch protection 对管理员生效，required check 使用 strict 模式。
