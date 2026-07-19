# Nemeton Local Web UI

生产 Web UI 由 `nemetond` 嵌入并在 loopback 同源提供。前端只消费 HTTP/WebSocket 投影，
不复制 Meeting 状态机或直接写 SQLite。

## 开发验证

```bash
pnpm install
pnpm lint
pnpm build
pnpm test:e2e
```

Playwright 使用确定性 API fixture，但运行真实浏览器、真实拖拽和响应式布局。完整仓库验证从根目录运行
`scripts/semantic-gate`。

## 当前信息架构

```text
Project / 原始问题
├── 折叠的参与者与模型配置
├── Meeting Room
│   ├── 参与者
│   ├── 自然语言讨论
│   └── Proposals
└── Result Review
    ├── Recorder synthesis
    ├── Candidate 逐项处置
    ├── 可选反馈
    └── Coding Design Handoff
```

组件只保留 React、dnd-kit 和 Lucide。表单使用浏览器原生控件；视觉 token 和布局位于 `src/index.css`。
