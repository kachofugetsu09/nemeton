# Nemeton 文档写作规则

## 1. 信息落点

| 信息 | 唯一权威落点 |
| --- | --- |
| 用户确认的长期产品语义 | `projectneed.md` |
| 正式决定、理由、后果和勘误 | `decisions/` |
| 问题级调用链、数据流、状态机和验收设计 | `design/`；既有阶段设计保留在 `spark/` |
| 已接受且未完成的当前工作 | `NOW.md` |
| 当前任务范围、步骤和副作用 | 任务合同或 handoff |
| 当前实现事实 | 代码、配置、schema、迁移、运行证据和 Git |
| 外部方案调查 | `research/`，只能作为 Evidence 或 Proposed input |

`projectneed.md` 是初始化前形成的完整 synthesis，现有混合结构予以保留；这不是继续把新 API 草案、临时 TODO 或实现细节写入长期需求的授权。

## 2. 证据分层

每份设计或调查必须区分：

- `Current implementation`：当前代码、配置、schema、Git 或运行证据可以证明。
- `User-confirmed intent`：用户明确确认的长期语义或禁止事项。
- `Proposed`：尚待批准的方案。
- `Inference`：根据命名、历史或实现作出的推断。
- `Unknown`：当前证据无法回答。
- `Historical`：只用于解释过去，不能覆盖当前真源。

不能从代码现状反推用户意图，也不能把旧文档或 Agent 总结直接升级为 requirement。

## 3. 状态词

- `Accepted`：已经获得用户确认，未来改动受其约束。
- `Current`：当前适用；若描述实现，必须能回到真实证据。
- `Proposed`：尚未批准，不能据此扩大实现范围。
- `Superseded`：已由后续决定替代，保留历史，不删除原记录。
- `Historical`：仅作线索，不参与当前裁决。

设计文档同时包含多种状态时，必须按章节标明，不能只在文件顶部写一个含糊状态。

## 4. 去重与链接

- 摘要只写职责和链接，不复制整段需求、决定或设计。
- `NOW.md` 引用 Accepted decision，不重复决定正文。
- 改名、移动或新增文档时，同一改动更新 `INDEX.md` 和所有入站链接。
- 同一语义出现冲突时保留双方原文，记录冲突并停止；不要选择更方便实现的一份。
- 错误决定通过新记录 amendment 或 supersede，不原地删除历史。

## 5. 工作手册与产品 Current State

仓库的 `docs/NOW.md` 始终是 Babel 工作手册，由任务交付者串行维护。Nemeton 产品的 Current State 是 SQLite 事件流生成的 structured projection 和确定性 document/digest；两者不是同一对象。产品状态只能通过领域 command 改变，不能通过编辑 `docs/NOW.md` 改变。

“可重建”只说明存在确定输入和算法，不等于任何 Agent 可以物理删除。删除、恢复和 replay 权限以持久化状态地图和 Accepted decision 为准。

## 6. 表达要求

- 默认使用中文，技术名词、schema 字段和命令保持准确。
- 先写结论、边界和可观察结果，再写理由。
- 使用稳定对象名和精确路径，避免“相关内容”“适当处理”“尽量支持”等不可验收表述。
- 图示使用等宽文本和 Unicode box-drawing；图示不能替代 owner、输入、输出和失败语义。
