# Nemeton

Nemeton 是一套面向 Agent 原生软件项目的本地语义控制平面。

它按顺序记录项目现实、多人讨论、语义决定、任务合同、worktree 执行、硬性验收和失败反馈。人类在会议中确认产品要解决的问题和风险边界，Agent 在已确认的语义与合同下完成实现。项目状态由事件历史重建，`NOW.md`、会议结果和当前任务视图都是可重新生成的投影。

当前仓库处于设计完成、实现尚未开始的阶段。第一轮实现只处理项目身份、Git Reality、SQLite 事件内核和 replay。

```text
Project Reality
      │
      ▼
Swarm Meeting ──► Human Decision ──► Semantic Revision
                                            │
                                            ▼
                                      Task Contracts
                                            │
                              ┌─────────────┴─────────────┐
                              ▼                           ▼
                         worktree A                  worktree B
                              │                           │
                              └─────────────┬─────────────┘
                                            ▼
                                  Gate + Evidence + Merge
                                            │
                                            ▼
                                    Trust State / Next Round
```

## 阅读顺序

1. [文档索引](docs/INDEX.md)
2. [完整产品语义](docs/projectneed.md)
3. [当前实施状态](docs/NOW.md)
4. [开发工作流](docs/WORKFLOW.md)
5. [v1 详细设计](docs/spark/2026-07-19-nemeton-v1-design.md)
6. [Swarm Meeting 详细设计](docs/spark/2026-07-19-nemeton-swarm-meeting-design.md)

## 当前实施边界

Milestone 0 必须走通以下路径：

```text
nemeton project open <path>
  -> 识别稳定 Project
  -> 记录 Repository Binding 和 Reality Revision 事件
  -> 生成当前 Project 投影
  -> 删除投影
  -> replay
  -> 重建出相同结果
```

这一阶段不调用模型，不创建 worktree，不修改用户仓库，也不构建会议 UI。
