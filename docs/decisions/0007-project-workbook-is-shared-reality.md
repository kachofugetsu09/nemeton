# Decision 0007：项目工作手册是 Agent 协作的共享现实

> 状态：Accepted
>
> 日期：2026-07-19
>
> 决策者：Human

## 问题

Nemeton 尚未实现自己的事件流、projector、任务合同和 Gate。不同 Agent 会话仍需要从同一入口恢复产品语义、当前工作、设计依据和仓库事实，且不能让会话摘要成为新的事实源。

## 决定

仓库工作手册是 Nemeton 自举阶段的共享现实：

```text
AGENTS.md
  └─► docs/INDEX.md
         └─► docs/WORKFLOW.md
                ├─► projectneed.md / decisions/
                ├─► NOW.md
                ├─► design/ / spark/ / research/
                └─► 当前代码、配置、schema 和 Git 证据
```

- `AGENTS.md` 只保存每次会话必须遵守的入口和纪律。
- `INDEX.md` 只负责路由，并保留既有仓库文档优先级。
- `WORKFLOW.md` 定义从读取到交付的授权阶段。
- `projectneed.md` 保存用户确认的长期产品语义。
- `decisions/` 保存正式决定、理由、后果和修订关系。
- `NOW.md` 只保存已接受且未完成的当前工作。
- `design/` 保存分状态的问题级设计；既有 `spark/` 文档继续作为 Approved input。
- 代码、配置、schema、运行证据和 Git 描述当前实现，不自动证明用户意图。

## 权威关系

当前用户明确指令限定任务范围。仓库文档之间继续采用初始化前已经存在的顺序：Accepted decisions、`projectneed.md`、`NOW.md`、`spark/`、`research/`。

文档语义与当前实现证据冲突时必须显式报告；不能用文档优先级掩盖实现已经漂移，也不能用实现现状静默改写需求。

## Skill 与运行边界

仓库只记录 `$babel` 和 `$babel-build` 的调用合同。Skill 实现由用户级 Codex 环境拥有，不复制到仓库，也不固定当前机器的绝对安装路径。Nemeton 当前不拥有插件或 MCP 安装链；未来新增时必须另作决定。

## 后果

- 新会话可以用固定读取链恢复上下文。
- 信息必须写入唯一权威落点，摘要只保留链接。
- 工作手册变更需要像代码一样隔离、备份、Review 和对账。
- 本决定不初始化业务测试、CI、Docker 或自动语义 Gate。
- Nemeton projector 实现后，生成型文档的写入权将按事件与 projector 决定迁移，届时需要新的 decision 修订本决定。

## 验证

- 根入口能够路由到 `INDEX.md` 和 `WORKFLOW.md`。
- 索引中的每个相对链接都存在。
- `projectneed.md`、decision、设计、当前进度和实现证据没有被写成同一类事实。
- 新会话不依赖用户级 Skill 的绝对路径即可理解仓库规则。
