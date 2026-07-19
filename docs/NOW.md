# Nemeton 当前状态

> 更新日期：2026-07-19
>
> 当前阶段：设计收敛完成，Milestone 0 尚未实现
>
> 临时说明：Nemeton 自身的 projector 完成前，这份文件由人维护。随后由事件流生成，Agent 不得并发编辑。

## 当前结论

产品边界已经稳定，可以开始实现第一个自举切片。当前工作只建立项目身份、Project Reality、SQLite 事件事实源和 replay。Swarm、模型 Provider、worktree 执行和 UI 都排在后续里程碑。

## 已接受决定

- 默认三个平等 Design Agent。
- Design Seat 按 `0..N` 建模，可以水平扩展。
- 语义控制面的领域事件是事实源，关系表、`NOW.md` 和 UI 状态都是投影。
- 本地 v1 使用 SQLite，`nemetond` 是唯一逻辑写入者。
- Go 承担 daemon、CLI、状态机、Git 子进程和本地 API。
- 人类只在会议中确认新的产品语义、风险边界和不可编码的取舍。
- Agent 执行发生在 Nemeton 管理的独立 worktree 中。
- hard gate 依靠可失败的检查、运行证据和目标 SHA，不依靠 Reviewer 的主观 verdict。

## 当前目标：Milestone 0

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

### 交付

- Go module。
- `nemetond` 和 `nemeton` 两个入口。
- SQLite migration runner。
- append-only domain event store。
- Project、Repository Binding、Reality Revision 的 reducer 和投影。
- `nemeton project open <path>`。
- `nemeton project inspect <project-id>`。
- daemon single-instance lock。
- 启动时的只读 reconcile 骨架。

### 验收

- 主 checkout 和 linked worktree 返回同一 Project ID。
- 路径移动后可以显式 relink，Project ID 不变。
- dirty checkout 可以被观察，dirty 内容不能进入 committed Reality。
- 非 Git 目录、损坏 common-dir 和歧义 integration branch 明确失败。
- 全流程不修改用户选择的仓库。
- daemon 重启后 Project 与 Reality 保持一致。
- 删除 Project/Reality 投影后，replay 能重建相同的规范化状态和 digest。
- replay 不执行 Git 写操作、外部 API、模型调用或其他副作用。

## 开工前只剩下的仓库设置

这些内容属于仓库初始化，不会改变产品设计：

- Go module path；当前仓库没有 remote，不能从现状推断。
- v1 的公开支持平台；建议先声明 Linux，代码在没有额外成本时保持 macOS 可移植。
- 应用数据目录和 managed worktree root 的具体路径。
- 初始 integration branch；当前空仓库只有未提交的 `master`，不能把它当作未来默认值。

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

Milestone 1 先走通单人会议、发言、候选语义和 `NOW` 投影，再接入三个 Design Agent。这个顺序先验证语义对象、来源引用和 human activation，再支付多模型协调成本。
