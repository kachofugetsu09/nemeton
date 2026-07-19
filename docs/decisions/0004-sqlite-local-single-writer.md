# Decision 0004：本地 v1 使用 SQLite 和单逻辑写入者

> 状态：Accepted
>
> 日期：2026-07-19
>
> 决策者：Human

## 决定

Nemeton v1 使用本机 SQLite。`nemetond` 是唯一逻辑写入者，Agent、CLI 和未来 UI 通过 daemon 的 command/API 边界提交变更，禁止自行打开数据库写入。

数据库必须：

- 位于本机文件系统。
- 使用 WAL、短事务和显式 busy timeout。
- 由 daemon 维护单调递增的 project stream sequence。
- 使用 `expected_stream_version` 检测并发语义变更。
- 避免长时间持有 read transaction。
- 监控 WAL checkpoint 和异常增长。
- 使用 SQLite backup API 生成在线备份，并验证 integrity。

数据库不能放在 NFS、云同步目录或多个进程可以绕过 daemon 写入的位置。

## 并发模型

多个 Agent 可以同时思考、调用工具、运行测试和修改不同 worktree。它们提交的领域命令由 daemon 在短事务中有序落库。

单个 Project 的事实形成过程需要稳定顺序。PostgreSQL 可以同时提交更多写事务，这种数据库行为不能自动解决两个决定之间的语义冲突；Nemeton 仍然需要 revision、领域不变量、Conflict 和 human activation。SQLite 的单写事务与这条有序语义流一致。

`NOW.md` 禁止多个 Agent 编辑。projector 从事件流生成临时文件，完成校验后原子替换正式文件。

## PostgreSQL 迁移条件

以下部署变化会触发 PostgreSQL backend 设计：

- 多台机器需要写入同一个 Nemeton 数据库。
- 产品增加团队共享服务器形态。
- daemon 不再是唯一写入入口。
- 需要高可用、流复制和远程数据库运维。
- 监控证明短事务仍持续产生不可接受的写锁等待。

Agent 数量增加本身不触发迁移。执行吞吐和持久化写入是两个不同的扩展维度。

## 实现约束

- v1 不为未来 PostgreSQL 建立通用 repository 抽象。
- 领域 command、event schema 和 reducer 不依赖 SQLite 特有业务语义。
- 当前 SQLite 实现可以使用明确的 SQL、migration 和事务代码。
- 数据库错误必须向 command 调用方明确返回，不能使用内存状态或静默 fallback 假装成功。

## 验收

- Agent 进程无法绕过 daemon 获得写入权。
- 并发提交基于同一旧 revision 时，至少一个 command 明确冲突或按最新状态再次校验。
- 长时间 UI/SSE 连接不持有数据库 read transaction。
- daemon 崩溃后 SQLite integrity、事件尾部和投影进度可以对账。
- `NOW.md` 更新失败不会回滚已经提交的事件，系统会保留等待再次生成的状态。
