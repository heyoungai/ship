# Change Plan · ship AI advisor

- Status: **in progress**（v0 已落地；streaming + 线模式 REPL 美化已落地）
- Date: 2026-07-23
- Updated: 2026-07-24
- Owner: TBD

> 使用说明：[guides/ship-ai.md](../../guides/ship-ai.md)。产品边界：[product-design.md](../../product-design.md) §7。本文保留演进与差距分析。

## 目标

在 ship 中增加**发布顾问（advisor）**：极简 loop agent（类 [Pi](https://pi.dev/docs/latest)）辅助 `ship.toml` 与只读诊断。AI **不**替代确定性发版路径。

## 理念

**工具通用、prompt 短、领域靠仓库与 CLI**——不是第二套 Claude Code。

## v0 已实现

| 能力 | 说明 |
|------|------|
| `ship ai` / `-p` / `ai init` | REPL、print、固定 init 开场白 |
| Streaming | OpenAI-compatible SSE；token 边收边打；JSON 回退兼容网关/mocks |
| 线模式 UX | 横幅（model/host）、着色提示符、首 token 前 spinner、默认工具行；非 TTY 降级 |
| 原语工具 | read / write / edit / bash / grep / find |
| 门禁 | cwd 沙箱；拦截 deploy/run/push/rollback |
| 校验回灌 | 写 `ship.toml` 后 `LoadConfig` |
| 项目说明 | 若存在 `AGENTS.md` 则并入上下文（截断） |
| 文档 | product-design §7、guides/ship-ai、skill 一行 |

环境：`OPENAI_API_KEY`、可选 `OPENAI_BASE_URL` / `SHIP_AI_MODEL`。

## 与 Pi 的差距分析

对照 [pi.dev](https://pi.dev/)：**对齐的要守住，大而全的不追。**

| Pi 能力 | ship ai | 结论 |
|---------|---------|------|
| 极短 system + 通用原语 | 已对齐 | 保持 |
| 明确不做 MCP / 子 agent / plan mode / 内置 todo | 已对齐（非目标） | 不补 |
| Interactive + print | 线模式 REPL + `-p`（streaming） | 够用 |
| 多 provider / 中途换模型 | 仅 OpenAI-compatible | **暂不补**（代理已覆盖多数；中途换模型对顾问会话价值低） |
| Session 树 / 分支 / share | 无 | **不补**（顾问会话短；复杂度高） |
| Compaction | 无 | **暂不补**（max-turns 先顶住；长会话再议） |
| AGENTS.md / SYSTEM.md | 已加载 `AGENTS.md`；无 SYSTEM.md 覆盖 | SYSTEM.md **可不补**（避免与短 system 哲学冲突） |
| Skills / prompt templates / packages | 无内置扩展市场 | **不补**（ship 领域靠 CLI + skill 给外部 agent） |
| Streaming + 线模式美化 | 已做 | 保持（非全屏 TUI） |
| 全屏 TUI / steer / follow-up | 无 | **不补**（不做迷你 Crush/Pi 会话窗） |
| RPC / SDK / JSON event stream | 无 | **低优先**（有嵌入需求再做） |
| 扩展改 harness | 无 | **不补**（Go CLI 用改代码，不搞 TS 扩展生态） |

**要补的（顾问场景真正痛的）：**

1. ~~文档与产品边界落地~~（已做）
2. ~~更多无 LLM 的场景单测~~（已做）
3. ~~Streaming + 线模式 REPL 美化~~（已做）
4. 自然语言「解释 plan / doctor」——**不必新子命令**；引导用 `-p` + `bash ship plan --json` 即可，观察使用后再定
5. TLS/网络抖动重试——可选小改进，非功能差距

**明确不追成「迷你 Pi」：** session 持久化、compaction、扩展包、全屏 TUI、多 provider UI。

## 后续（可选）

- [ ] 观察真实使用后是否要 `ai doctor` 快捷开场白（仍同一 loop）
- [ ] 可选：Chat Completions 失败时有限次重试
- [ ] 有嵌入需求时再考虑 JSON event / 库导出
- [ ] 稳定很久后再考虑独立 ADR（边界已在 product-design）

## 安全边界

1. 发布副作用不经顾问执行（门禁 + prompt）。
2. 不把密钥写入配置。
3. 无模型时主路径仍可用。
4. 文件工具不出项目根。

## 试用

见 [guides/ship-ai.md](../../guides/ship-ai.md)。
