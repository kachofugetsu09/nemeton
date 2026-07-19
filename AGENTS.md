# Nemeton Agent 工作入口

## 每次会话的固定读取顺序

1. 先读 [`docs/INDEX.md`](docs/INDEX.md)，确认权威顺序和任务阅读路由。
2. 再读 [`docs/WORKFLOW.md`](docs/WORKFLOW.md)，确认当前阶段、授权边界和完成定义。
3. 按任务类型读取 `projectneed.md`、`NOW.md`、accepted decisions、相关设计以及真实代码、配置、schema 和 Git 证据。

不要凭旧会话摘要、记忆或单份设计文档跳过上述入口。工作手册缺失或互相冲突时停止，并建议先运行 `$babel-build` 修复共享现实。

## 权威与证据

- 当前用户明确指令限定本次任务范围；仓库内事实优先级以 `docs/INDEX.md` 为准。
- 严格区分当前实现事实、用户确认意图、Agent 推断和未知。代码只能证明“现在是什么”，不能证明“应该是什么”。
- Accepted decision 与当前代码、配置或 schema 冲突时，不自行选择一方；列出冲突并停止需要该语义的工作。
- 长期产品语义只写入 `docs/projectneed.md` 或 Accepted decision。当前进度只写入 `docs/NOW.md`。

## 授权与隔离

- 调查、设计和 Review 默认只读。实现必须有已批准的任务合同。
- 写入工作默认从正确基线创建专用 worktree；不要在用户主 checkout 中修改。
- 修改持久化文件、Git 状态、数据库、运行 workspace 或外部系统前，先创建清晰、可恢复的备份。
- 只修改合同允许的 repository、branch 和 path。发现范围外问题时记录证据，不顺手修复。
- 不自动 commit、push、创建或合并 PR，除非任务合同明确授权。

## 实现与失败

- 保持最小可成立方案，不为未来假设增加抽象、兼容层、依赖或静默 fallback。
- 在 HTTP、CLI、Provider、Git、数据库反序列化、artifact 和外部进程边界集中校验；边界之后信任已经验证的领域类型和不变量。
- 违反内部契约时 fail-fast、fail-loud。只捕获当前位置能真正恢复、转换或补充上下文的具体异常。
- 非平凡函数使用一句话 docstring 和必要的编号阶段注释；简单函数不强制添加形式化注释。
- 默认使用中文沟通；代码、标识符、注释和提交信息保持简洁、明确。

## 验证与交付

- 验证必须直接对应任务合同的可观察验收条件，并绑定准确 diff 或 target SHA。
- 仓库级硬性验证入口是 `scripts/semantic-gate`，GitHub required check 名称是 `semantic-gate`。它保护 Milestone 0/1 的原子事务和跨组件联合语义；不要把它误称为尚未实现的 Nemeton 产品 Gate 引擎。
- 当前仓库只有交付认证用 Dockerfile，不提供 Docker 运行形态。不要虚构命令或通过状态；以 `docs/WORKFLOW.md` 记录的当前能力为准。
- 交付前对账代码、文档、Accepted decisions 和 `NOW.md`，说明已验证、未验证、备份、回滚和未完成事项。
- 使用 `$babel <需求>` 进入设计到交付流程；只有修复工作手册时使用 `$babel-build`。
