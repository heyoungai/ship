# Change Plan · ship AI advisor

- Status: **planned**（未开始编码）
- Date: 2026-07-23
- Owner: TBD

> 本文档是规划，不是实现规范。稳定落地后再考虑提升为 ADR，或把长期边界写入 [product-design.md](../../product-design.md)。

## 目标

在 ship 中增加**发布顾问（advisor）**能力：用大模型辅助理解项目、生成/补全 `ship.toml` 草稿、解释 plan/doctor 结果。

AI **不**替代确定性发版路径；真正的 build / push / deploy 仍走现有命令。

## 非目标

- 不做类 Claude Code 的通用 loop agent / REPL「什么都能聊、什么都能改」
- 不在无确认时自动执行 `deploy` / `run`
- 不把密钥写入 `ship.toml`
- 不替代项目内的 Claude / Cursor + [ship skill](../../../cmd/skills/SKILL.md)

## 建议命令面（分期）

| 分期 | 命令 | 说明 |
|------|------|------|
| 1 | `ship ai init` | 探测项目 + LLM 生成可确认的 `ship.toml` 草稿 |
| 2 | `ship ai doctor` | 解释 `doctor`/`plan` 失败并给出可执行修复建议 |
| 3 | `ship ai explain …` | 用自然语言说明某次 release 会做什么 |

第一期建议 flags：`[-y] [--force] [--dry-run] [--provider openai|anthropic] [--model …]`

- 无 API key：报错，并提示使用确定性 `ship init`
- 默认 dry-run 打印草稿；写入需确认或 `-y`
- 已有配置时与 `init` 相同，需 `-f` / 确认

## 架构草图

```text
ship ai init
  → ProjectProbe（确定性探测）
  → 组装 Prompt（探测 JSON + schema 约束 + 示例摘要）
  → LLM Client 适配层
  → ship.toml 草稿（含 # TODO: 注释）
  → 确认 / -y / --dry-run
  → 校验后写入（可选）
```

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

### Prompt 约束

- 只输出合法 `schema = 2` 的 TOML + TODO 注释
- 遵守 [product-design.md](../../product-design.md) 边界（发布执行器，非通用编排）
- 写入前用现有配置加载/校验；失败则不覆盖已有好文件

## 安全边界

1. 顾问只产出草稿与建议，不执行发布副作用。
2. 不把 API key / registry 密码写入配置。
3. 无模型时主路径（`init` / `run` / `deploy`）必须仍可用。

## 建议实现分期与验收（供日后开工）

**Phase 1 — `ship ai init`**

- [ ] 共用/增强 ProjectProbe
- [ ] `internal/ai` 适配层（openai 默认 + anthropic）
- [ ] `ship ai init --dry-run` 产出可解析草稿
- [ ] `-y` 写入后配置可加载
- [ ] 无 key 时失败信息指向 `ship init`

**Phase 2+**

- [ ] `ship ai doctor` / `explain`
- [ ] 视需要更新 skill 一句说明

## 与文档分层的关系

- 规划留在本 change plan，**不单独建 ADR**（规划态 ≠ 已稳定决策）。
- 若「AI 只做顾问」日后成为产品铁律，再写入 `product-design.md` 或提升 ADR。
