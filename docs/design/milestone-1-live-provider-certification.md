# Milestone 1 真实 Provider 交付认证

> 状态：Delivery evidence
>
> 日期：2026-07-19
>
> 边界：真实 Provider 只执行这一次；后续代码与 CI 只运行程序化等价 Provider

## 结果

真实 Codex 与 OpenCode 完成了一场完整 Meeting。会议在第 3 轮形成全体一致结论，
Recorder 生成 9 个带持久 content 引用的 Candidate，Verifier clear，Human 将 9 项全部
`deferred` 后 Meeting 进入 `concluded`。

| Evidence | Value |
| --- | --- |
| Target baseline | `09b3129d590ca0bf555b7be25b78d14a50bad95d` |
| Project ID | `f25daf0e-2b0c-4143-a313-62e256fbcd3d` |
| Meeting ID | `453b7999-0606-4885-89e8-b5037d104bc1` |
| Codex | `codex-cli 0.144.5` |
| OpenCode | `1.17.18` |
| Agent Runs | 14：3 proposal、9 deliberation、1 recording、1 verifying |
| Final status | `concluded`，round 3，9 deferred Candidates |
| Current State digest | `38c76e16b91580b1b1e971ab127f203ca5f92be050604a6b4af049c1dfa4e235` |
| Source checkout | 调用前后 clean，HEAD 不变 |

Designer 与 Adversary 使用不同 Codex Session，Maintainer 与 Recorder 使用不同 OpenCode
Session，Verifier 使用新的 Codex Session。所有席位均在不同持久 Workdir 中读取仓库并
执行真实命令；没有 Provider fallback。

## 最终代码上的无模型对账

真实调用完成后，Runner 又补强了首个协议事件即持久 Session、结构化协议失败状态、
stdout/stderr/版本/规范化命令 Evidence 和 source checkout 末端观察。按用户要求没有再次
调用真实模型；这些增量由程序化 Provider 和进程协议测试覆盖。

最终代码随后打开真实会议产生的 schema v2 数据库：

1. SQLite online backup 在 migration 003 前生成并通过 integrity check。
2. 数据库升级到 schema v3；最终 daemon 没有启动任何 Provider。
3. replay 前后 Meeting status、14 个 Run、9 个 Candidate 和 Current State digest 完全一致。
4. 迁移备份转换为 `journal_mode=DELETE`，没有遗留 `-wal` 或 `-shm`。

最终无模型对账时：

- 数据库 SHA-256：`c3e23ae0b8822aaa39f0b04d8befbafec63b30d5cd0389324f1c5f1b8725d74f`。
- pre-migration backup SHA-256：`255e64e0a2d6b8148d5902bc23e8400da074ec5dc3bb0424dbfe168f86656399`。
- `nemetond` binary SHA-256：`80ec17e3a842ae063499a9e2b34f986d84336f11bc56ea4d3704f756a205336b`。
- `nemeton` binary SHA-256：`a72b130a561c43205547afad74ccc126e557fd1ec4da99c20c1892d5224115d4`。

原始 CAS、SQLite 与 Workdir 位于本机临时认证目录
`/tmp/nemeton-live-Jj5z4X/`，不是仓库或发布 artifact；本文件只保存可审计摘要，不把
临时目录承诺为长期备份。

## 声明边界

- 这次认证证明上述客户端版本和本次本机环境中的真实调用路径可用。
- 它不保证未来 Codex/OpenCode 版本兼容；运行时协议不兼容必须 fail-loud。
- CI 不读取用户凭据、不访问模型服务，也不把 mock 注册为生产 fallback。
- 会议内部关于“Milestone 0 baseline 不包含 Provider”的候选项描述的是被观察的目标
  checkout，不是否认运行本次会议的 Milestone 1 host 已经调用真实 Provider。
