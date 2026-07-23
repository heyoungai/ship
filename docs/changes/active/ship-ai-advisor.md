# Change Plan · ship AI advisor

- Status: **in progress**（v0 已落地可试用）
- Date: 2026-07-23
- Updated: 2026-07-24
- Owner: TBD

> 本文档是规划 + v0 范围说明，不是完整实现规范。稳定落地后再考虑提升为 ADR，或把长期边界写入 [product-design.md](../../product-design.md)。

## 目标

在 ship 中增加**发布顾问（advisor）**能力：用大模型辅助理解项目、生成/补全 `ship.toml`、解释 plan/doctor——以**极简 loop agent**（类 [Pi](https://pi.dev/docs/latest)）完成。

AI **不**替代确定性发版路径；真正的 build / push / deploy 仍走现有命令。

## 理念：极简 harness（对齐 pi）

| 要 | 不要 |
|----|------|
| 通用原语工具（read / write / edit / bash / grep / find） | 一堆 AI 难记的专用领域 tool schema |
| **极短** system（身份 + 硬边界）；细节靠读文件 / 用户话 | 把完整 schema、长手册塞进 system |
| 确定性能力调用现有 `ship plan --json` / `doctor` / `init` | 在顾问层重复封装 CLI |
| cwd 沙箱 + 拦截 `ship deploy/run/push/rollback` | 无确认自动发版 |

一句话：**工具通用、prompt 短、领域靠仓库与 CLI**——顾问 harness，不是第二套 Claude Code。

## 非目标

- 不做重型通用编码 agent（子 agent、plan mode、内置 MCP、permission 弹窗矩阵、后台任务系统等）
- 不在顾问内执行 `deploy` / `run` / `push` / `rollback`（代码门禁）
- 不把密钥写入 `ship.toml`
- 不替代项目内的 Claude / Cursor + [ship skill](../../../cmd/skills/SKILL.md)
- v0 不做 Anthropic、session 持久化、compaction、TUI

## v0 命令面（已实现）

| 命令 | 说明 |
|------|------|
| `ship ai` | 交互 REPL（`/quit`、`/help`） |
| `ship ai -p "..."` | 非交互单次（print） |
| `ship ai init` | 同一 loop + 固定「生成/补全 ship.toml」开场白 |

Flags：`--model`、`--base-url`、`--max-turns`（默认 20）、`--dry-run`、`--trace`；写盘确认可用全局 `-y`。

环境变量：`OPENAI_API_KEY`（必需）、`OPENAI_BASE_URL`、`SHIP_AI_MODEL`（默认 `gpt-4.1-mini`）。无 key 时失败并提示用 `ship init`。

## 架构

```text
ship ai | -p | ai init
  → internal/ai thin loop（OpenAI-compatible Chat Completions + tools）
  → 原语：read / write / edit / bash / grep / find
  → 门禁：路径不出 cwd；bash 拒绝 deploy/run/push/rollback
  → 写 ship.toml 后尝试 LoadConfig 校验，错误回灌给模型
```

实现位置：

- [`internal/ai/`](../../../internal/ai/) — provider、loop、tools、sandbox、guard、短 system
- [`cmd/ai.go`](../../../cmd/ai.go) — Cobra / REPL / print / `ai init`

## 后续（未做）

- [ ] `ship ai doctor` / `explain`（或继续用自然语言 + 现有 CLI）
- [ ] Anthropic provider
- [ ] 视需要更新 skill 一句说明
- [ ] 稳定后写入 `product-design.md` 或提升 ADR

## 安全边界

1. 发布副作用不经顾问执行（门禁 + prompt）。
2. 不把 API key / registry 密码写入配置。
3. 无模型时主路径（`init` / `run` / `deploy`）仍可用。
4. 文件工具不出项目根。

## v0 验收

- [x] `ship ai` / `-p` / `ai init` 可启动（需 API key）
- [x] 通用原语工具 + cwd 沙箱
- [x] bash 拦截 deploy/run/push/rollback（单测）
- [x] mock HTTP tool-loop 单测
- [x] `--dry-run` 不落盘
- [ ] 真人用真实模型试一轮「摸清仓库 → 起草 ship.toml」

## 试用

```bash
export OPENAI_API_KEY=...
# 可选 OPENAI_BASE_URL / SHIP_AI_MODEL
ship ai init --dry-run
ship ai -p "看下项目并起草 ship.toml，不确定标 TODO"
ship ai
```
