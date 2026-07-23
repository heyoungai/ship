# Change Plan · ship AI advisor

- Status: **planned**（未开始编码）
- Date: 2026-07-23
- Updated: 2026-07-24
- Owner: TBD

> 本文档是规划，不是实现规范。稳定落地后再考虑提升为 ADR，或把长期边界写入 [product-design.md](../../product-design.md)。

## 目标

在 ship 中增加**发布顾问（advisor）**能力：用大模型辅助理解项目、生成/补全 `ship.toml` 草稿、解释 plan/doctor 结果，并在需要时以**极简 loop agent** 多轮完成这些任务。

AI **不**替代确定性发版路径；真正的 build / push / deploy 仍走现有命令。

## 理念：极简 harness，不是厨房水槽

对照 [Pi](https://pi.dev/docs/latest)：核心保持小、可理解；默认只给 frontier model 够用的原语，把重型工作流留给扩展或确定性 CLI，而不是做成 Claude Code 式「什么都内置」的通用编码 REPL。

ship 的 loop agent 应对齐同一条线：

| 要 | 不要 |
|----|------|
| 小核心 + 少量工具（读探测结果、读配置、跑只读 doctor/plan、写草稿） | 子 agent、plan mode、内置 MCP、后台 bash 厨房水槽 |
| 领域收窄到 **ship / 发布配置与诊断** | 通用「什么都能聊、什么都能改」的编码 IDE |
| 上下文可控、路径可预期 | 隐式往 prompt 塞大量魔法上下文 |
| 真正副作用走现有 `ship` 命令 + 用户确认 | 无确认自动 `deploy` / `run` |

一句话：**可以有 loop，但 loop 要像 pi 一样薄**——顾问 harness，不是第二套 Claude Code。

## 非目标

- 不做重型通用编码 agent（子 agent、plan mode、内置 MCP、permission 弹窗矩阵、后台任务系统等）
- 不在无确认时自动执行 `deploy` / `run`
- 不把密钥写入 `ship.toml`
- 不替代项目内的 Claude / Cursor + [ship skill](../../../cmd/skills/SKILL.md)（skill 仍教外部 agent 怎么调 ship；本能力是 ship 自带的薄顾问）

## 建议命令面（分期）

| 分期 | 命令 | 说明 |
|------|------|------|
| 1 | `ship ai init` | 探测项目 +（单轮或极简多轮）生成可确认的 `ship.toml` 草稿 |
| 2 | `ship ai doctor` | 解释 `doctor`/`plan` 失败并给出可执行修复建议 |
| 3 | `ship ai explain …` | 用自然语言说明某次 release 会做什么 |
| 可选 | `ship ai` / `ship ai chat` | 同域内的极简 REPL；工具面固定、窄 |

第一期建议 flags：`[-y] [--force] [--dry-run] [--provider openai|anthropic] [--model …]`

- 无 API key：报错，并提示使用确定性 `ship init`
- 默认 dry-run 打印草稿；写入需确认或 `-y`
- 已有配置时与 `init` 相同，需 `-f` / 确认

## 架构草图

```text
ship ai <子命令 | REPL>
  → 确定性输入（ProjectProbe / doctor·plan 结构化输出 / 现有 ship.toml）
  → 极简 agent loop（可选多轮）
       tools ⊆ { 读探测/配置, 跑只读 ship 诊断, 产出草稿或建议 }
  → 用户确认 / -y / --dry-run
  → 校验后写入（可选）；发布副作用仍走 ship run/deploy
```

Phase 1 可先做「探测 → 单轮 LLM → 草稿」，把 loop / 工具面留成同一套薄 runtime 的自然延展，避免先堆成重型 agent。

### ProjectProbe（确定性）

扩展现有 `ship init` 探测，建议抽取共用：

- Dockerfile / `go.mod`+`cmd/` → 推荐 driver
- image name（git remote / 目录名）
- local_build、env 文件、compose 候选路径
- **不猜** registry URL、SSH host、远程 path（标 `# TODO:`）

### LLM 适配层

薄 `Provider` 接口，同时支持：

- **默认**：OpenAI-compatible Chat Completions（`OPENAI_API_KEY`，可选 `OPENAI_BASE_URL`）
- Anthropic Messages API（`ANTHROPIC_API_KEY`）

建议环境变量：

| 变量 | 含义 |
|------|------|
| `SHIP_AI_PROVIDER` | `openai`（默认）\| `anthropic` |
| `SHIP_AI_MODEL` | 模型名 |
| `OPENAI_API_KEY` / `OPENAI_BASE_URL` | OpenAI-compatible |
| `ANTHROPIC_API_KEY` | Anthropic |

### Prompt / 工具约束

- 领域：ship 配置与发布诊断；不假装通用 coding agent
- 草稿只输出合法 `schema = 2` 的 TOML + TODO 注释
- 遵守 [product-design.md](../../product-design.md) 边界（发布执行器，非通用编排）
- 写入前用现有配置加载/校验；失败则不覆盖已有好文件
- 工具默认只读；写配置必须过确认 / `-y`

## 安全边界

1. 顾问产出草稿与建议；发布副作用只经现有命令 + 确认。
2. 不把 API key / registry 密码写入配置。
3. 无模型时主路径（`init` / `run` / `deploy`）必须仍可用。
4. loop 保持可审计：少工具、显式确认，不引入隐式后台执行。

## 建议实现分期与验收（供日后开工）

**Phase 1 — `ship ai init`（可先单轮）**

- [ ] 共用/增强 ProjectProbe
- [ ] `internal/ai` 适配层（openai 默认 + anthropic）
- [ ] `ship ai init --dry-run` 产出可解析草稿
- [ ] `-y` 写入后配置可加载
- [ ] 无 key 时失败信息指向 `ship init`

**Phase 2+**

- [ ] 同 runtime 上的极简多轮 loop + 窄工具面（若单轮不够）
- [ ] `ship ai doctor` / `explain`
- [ ] 视需要：域内 `ship ai` REPL（仍非通用编码 agent）
- [ ] 视需要更新 skill 一句说明

## 与文档分层的关系

- 规划留在本 change plan，**不单独建 ADR**（规划态 ≠ 已稳定决策）。
- 稳定后可写入产品边界的是：**AI 是极简发布顾问 harness（可有薄 loop），不替代确定性发版，也不做成重型通用 agent**——再进 `product-design.md` 或提升 ADR。
