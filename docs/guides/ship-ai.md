# ship ai 发布顾问

极简 loop agent：用通用工具探索仓库、起草/修改 `ship.toml`、借助现有 `ship` 命令做只读诊断。理念对齐 [Pi](https://pi.dev/docs/latest)（薄 harness、短 system、原语工具）。

权威产品边界见 [product-design.md](../product-design.md) §7。

## 准备

```bash
export OPENAI_API_KEY=...
# 可选
export OPENAI_BASE_URL=https://api.openai.com/v1   # 任意 OpenAI-compatible
export SHIP_AI_MODEL=gpt-4.1-mini
```

无 API key 时请用确定性 `ship init`。

## 常用命令

| 命令 | 作用 |
|------|------|
| `ship ai` | 交互 REPL（`/quit`、`/help`） |
| `ship ai -p "..."` | 非交互单次 |
| `ship ai init` | 预设「生成/补全 ship.toml」任务 |
| `ship ai init --dry-run` | 同上，但不落盘 |

常用 flags：`--model`、`--base-url`、`--max-turns`、`--trace`、`-y`（跳过写 `ship.toml` 确认）。

## 工具与门禁

工具：`read` / `write` / `edit` / `bash` / `grep` / `find`。

- 文件路径不出当前项目根
- `bash` 中的 `ship deploy` / `run` / `push` / `rollback` 会被拒绝
- 写完 `ship.toml` 后会尝试加载校验，错误回给模型

若项目根有 `AGENTS.md`，启动时会并入上下文（与 pi 类似的项目说明，保持简短为宜）。

## 推荐用法

```bash
ship ai init --dry-run --trace
ship ai -p "根据仓库起草 ship.toml，不确定的标 # TODO:"
ship ai   # 再问：解释当前 plan，或改 deploy.driver
```

发版仍走：

```bash
ship plan -v v1.0.0
ship doctor -v v1.0.0
ship run -v v1.0.0 -y
```

## 与外部 agent / skill

`ship skill` 安装的 skill 教 **Claude/Cursor 等**如何调用 ship CLI。`ship ai` 是 **ship 自带**的薄顾问，二者互补，不互相替代。
