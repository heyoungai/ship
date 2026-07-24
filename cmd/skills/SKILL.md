---
name: ship
description: >-
  Builds, pushes, and deploys Docker images or Go binaries with the ship CLI
  (ship.toml schema 2). Use when the user mentions ship, ship.toml, image build
  and push, compose deploy, rollback, release tags, matrix profiles, or git-tag
  based releases.
version: dev
---

# Ship

CLI 工具，流程为 **build → tag → push → deploy → verify**。配置：项目根目录 `ship.toml`（`schema = 2`）。

`ship skill` 安装时会把 frontmatter `version` 写成当前 ship 二进制版本。若 `build` / `run` / `doctor` 提示 skill 过期，执行 `ship skill -f`。

## 默认工作流

```bash
ship plan -v v1.0.0
ship doctor -v v1.0.0
ship run -v v1.0.0 -y
```

也可分步：`build` → `tag` → `push` → `deploy`。正式发版优先用 `run`。

## 命令速查

| 命令 | 用途 | 关键 flags |
|------|------|-----------|
| `init` | 生成 `ship.toml` | `-f` |
| `plan` | 展示 release 计划（无副作用） | `-v`, `-p`, `--json` |
| `doctor` | 检查 release 条件 | `-v`, `-p` |
| `build` / `tag` / `push` | 构建、打 tag、发布 | `-v`, `-p`, `--promote-latest`（push） |
| `deploy` / `rollback` / `history` | 部署 / 回滚 / 历史 | `-v`, `-y`, `-n` |
| `run` | 全流水线 | `-v`, `-p`, `--skip-deploy`, `--promote-latest` |
| `current` / `version` / `skill` | 当前 git tag / ship 版本 / 安装本 skill | `-f`（skill） |

## 硬性规则

- agent / CI 调用可能弹确认的命令时，**必须加 `-y`**。
- 默认 `version.source = "git-tag"`：`-v` / `SHIP_VERSION` 必须是**本地真实 Git tag**。构建使用 tag 源码快照（worktree），不包含未提交修改。
- 独立 `push` / `deploy` / `rollback` 消费 `.ship/releases/` 中的 **release manifest**；尚未发布过的版本会失败（不会从当前目录偷偷补构建）。
- 默认按 **digest** 钉部署（`APP_IMAGE_DIGEST`）。compose 镜像须用 `@${APP_IMAGE_DIGEST}`（否则会降级 `pin=tag`）。`deploy`/`rollback` **不会**移动 registry `:latest`；需要时用 `--promote-latest`。
- Docker：`build.docker.load = true`，且 **单平台**（如 `linux/amd64`）。
- `ship.toml` 未知字段默认报错；可用 `[config] unknown_keys = "warn"` 或 `SHIP_UNKNOWN_KEYS=warn` 降级。
- `.ship/` 为运行状态（runs / releases / history），应加入 `.gitignore`。

## 按需深入

- 配置字段与驱动：见 [REFERENCE.md](REFERENCE.md)
- 可复制场景：见 [EXAMPLES.md](EXAMPLES.md)
- v2.7.1 digest pin 修复复盘：仓库内 `docs/hotfix-v2.7.1-digest-pin.md`
