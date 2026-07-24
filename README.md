# ship

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Docker 镜像构建、推送和远程部署 CLI 工具。Go 实现，编译为单二进制，无运行时依赖。

仅支持 **`ship.toml` v2 schema**，支持矩阵构建（多品牌/多变体）。命令层基于 **Cobra**，输出层使用 **PTerm**，交互确认使用 **Huh**。

- 快速上手：[docs/quick-start.md](docs/quick-start.md)
- 设计原则：[docs/design-principles.md](docs/design-principles.md)
- Docker 默认模式：[docs/docker-default-pattern.md](docs/docker-default-pattern.md)
- 真实版本构建策略：[docs/git-tag-release-strategy.md](docs/git-tag-release-strategy.md)
- v3 设计：[docs/ship-v3.md](docs/ship-v3.md)

主路径：

- **Docker**：`build.driver = "docker"` → `publish.driver = "registry"` → `deploy.driver = "compose"`
- **Go 二进制**：`build.driver = "go-binary"` → `publish.driver = "scp"` → `deploy.driver = "binary-install"`

Docker 的 `build → tag → push` 会将 `docker buildx` 产物 `--load` 回本地，因此 **`build.docker.platforms` 目前应保持单平台**（如 `linux/amd64`），且 `build.docker.load` 须为 `true`。

## 安装

### Scoop (Windows) 推荐

```bash
scoop bucket add ship https://github.com/heyoungai/ship
scoop install ship
ship version
scoop update ship
```

### Go

```bash
go install github.com/heyoungai/ship@latest
```

### 从源码编译

```bash
go install github.com/go-task/task/v3/cmd/task@latest
task build
```

## 快速开始

```bash
ship init
ship --help
```

## 子命令

| 命令 | 说明 |
|------|------|
| `init` | 初始化 `ship.toml` |
| `version` | 显示 ship 自身版本 |
| `current` | 显示当前项目最近 git tag |
| `plan` | 展示 release 计划（不执行） |
| `doctor` | 检查 release 运行条件 |
| `build` / `tag` / `push` | 构建、打 tag、发布 |
| `deploy` / `rollback` / `history` | 部署、回滚、历史 |
| `run` | `build → tag → push → deploy` 全流程 |
| `skill` | 安装/更新 agent skill（`.claude/skills/ship/`） |

## 用法

```bash
ship init
ship init --force   # 覆盖已有配置
ship init --yes

ship build -v v2.0.0
ship tag -v v2.0.0
ship push -v v2.0.0
ship push -v v2.0.0 --promote-latest
ship deploy -v v2.0.0

ship run -v v2.0.0 -y
ship run -v v2.0.0 --env-file ./.env.local --skip-deploy
ship run -p brand-a

ship plan -v v2.0.0
ship plan -v v2.0.0 --json
ship doctor -v v2.0.0

ship rollback
ship rollback -v v1.9.0 --yes
ship history
ship history -n 5
```

### 发布语义（默认 `version.source = "git-tag"`）

- `-v` / `SHIP_VERSION` 必须是本地真实 Git tag；构建来自该 tag 的源码快照，不切换你当前分支，也不带上未提交修改。
- `ship plan` / `ship doctor` 可先预览计划与检查条件；`ship run` / `ship build` 会写入 `.ship/runs/`，成功发布后索引到 `.ship/releases/`。
- 独立 `push` / `deploy` / `rollback` 按 release manifest 消费产物；尚未发布过的版本会失败。
- 默认按 digest 钉部署（写入 `APP_IMAGE_DIGEST`，同时保留版本别名）。生产 compose **必须**用 `@digest`，推荐：

```yaml
image: registry.example.com/ns/app@${APP_IMAGE_DIGEST}
```

若 compose 仍按 `:tag` 拉取，请设 `pin = "tag"`；否则 v2.7.1 起会警告并自动降级。pin 身份来自 registry（buildx imagetools），不会再用本地 config digest 冒充。

- `deploy` / `rollback` 不修改 registry `:latest`；需要移动 `latest` 时用 `--promote-latest`（生产建议关闭 `tag_latest_on_default_profile`）。

完整设计与边界见 [docs/git-tag-release-strategy.md](docs/git-tag-release-strategy.md)。v2.7.1 digest pin 修复复盘见 [docs/hotfix-v2.7.1-digest-pin.md](docs/hotfix-v2.7.1-digest-pin.md)。

Agent skill：`ship skill` 安装整个目录到 `.claude/skills/ship/`（`SKILL.md` + `REFERENCE.md` + `EXAMPLES.md`）。`SKILL.md` 的 `version` 对齐当前 `ship version`；`build` / `run` / `doctor` 若不一致会警告，执行 `ship skill -f` 更新。

## 配置

在项目根目录运行 `ship init`，或参考 `config.example.toml`。当前只接受 `schema = 2`。

配置优先级：`环境变量 > ship.toml > 内置默认值`。

### 必填字段

- `schema = 2`
- `build.driver` / `publish.driver`
- `build.docker.image` — `build.driver = "docker"` 时
- `publish.registry.targets` — `publish.driver = "registry"` 时至少一个
- `deploy.compose.host` / `deploy.compose.path` — 启用 compose 部署时

可选：`deploy.compose.local_file` / `local_env_file`（部署前 scp 到远端）；`pin` / `digest_key` / `image_key`（digest 部署，见策略文档）。

### 未识别配置项

默认对未知字段报错。可用 `config.unknown_keys` 或 `SHIP_UNKNOWN_KEYS=warn|ignore` 调整。项目差异优先用 `steps.*`。

### `deploy.compose` 的 env 相关字段

| 字段 | 说明 | 默认 |
|------|------|------|
| `env_file` | 远端 env 文件名 | `.env` |
| `local_env_file` | 本地 env，部署前上传（可选） | 空 |
| `auto_env_file` | 非默认 `env_file` 时自动注入 `--env-file` | `true` |

### 简单项目（无矩阵）

```toml
schema = 2

[project]
name = "home"

[features]
deploy = true
verify = true

[build]
driver = "docker"

[build.docker]
image = "home"
platforms = ["linux/amd64"]
dockerfile = "./Dockerfile"
env_file = "./.env.local"

[publish]
driver = "registry"

[[publish.registry.targets]]
type = "private"
url = "registry.cn-hangzhou.aliyuncs.com"
namespace = "deali"
image = "home"

[deploy]
driver = "compose"

[deploy.compose]
host = "deali.cn"
path = "/home/deali/projects/home"
local_file = "./deploy/compose.prod.yaml"
remote_file = "compose.yaml"
local_env_file = "./deploy/.env.prod"
env_file = ".env"
auto_env_file = true
tag_key = "APP_IMAGE_TAG"
pin = "digest"
digest_key = "APP_IMAGE_DIGEST"
up = "docker compose --env-file ./.env up -d --remove-orphans"

[verify]
driver = "http"

[verify.http]
url = "https://deali.cn/api/health"
expected_status = 200
attempts = 20
interval_seconds = 3
timeout_seconds = 5
```

### 多品牌项目（矩阵）

```toml
schema = 2

[project]
name = "canvas-studio"

[features]
deploy = false
verify = false

[build]
driver = "docker"

[build.docker]
image = "canvas-studio"
platforms = ["linux/amd64"]
dockerfile = "./Dockerfile"
env_file = "./.env"

[publish]
driver = "registry"

[[publish.registry.targets]]
type = "private"
url = "ccr.ccs.tencentyun.com"
namespace = "deali"
image = "canvas-studio"

[[matrix]]
name = "brand-a"
default = true
env = { NEXT_PUBLIC_APP_BRAND = "brand-a" }

[[matrix]]
name = "brand-b"
env = { NEXT_PUBLIC_APP_BRAND = "brand-b" }
```

每个 profile 会带上自己的 env 构建，并生成带后缀的镜像 tag（如 `v2.0.0-brand-a`）。

### 阶段编排与校验

- `build`：`prepare` → `templates` → build driver → `post_build`
- `run`：`build → tag → publish → deploy → verify`，两侧可插 hooks
- 部署后校验优先用 `[verify]`（`http` / `ssh` / `command`）；仍兼容旧的 `[deploy.healthcheck]`
- `init` / `rollback` 在交互终端会确认；CI 请加 `-y` / `--yes`

## 环境变量

| 变量 | 说明 |
|------|------|
| `IMAGE_NAME` | 覆盖 `build.docker.image` |
| `PLATFORMS` | 覆盖 `build.docker.platforms` |
| `DOCKERFILE` | 覆盖 `build.docker.dockerfile` |
| `ENV_FILE` | 覆盖 `build.docker.env_file` |
| `REMOTE_HOST` | 覆盖 `deploy.compose.host` |
| `REMOTE_PROJECT_PATH` | 覆盖 `deploy.compose.path` |
| `SHIP_VERSION` | 覆盖版本（须为真实 tag，当 `version.source=git-tag`） |
| `SHIP_UNKNOWN_KEYS` | `error` / `warn` / `ignore` |

## 开发

```bash
task test
task cover
task build
task build VERSION=v1.0.0
task clean
```

## 依赖

- [cobra](https://github.com/spf13/cobra) — CLI
- [pterm](https://github.com/pterm/pterm) — 终端输出
- [huh](https://github.com/charmbracelet/huh) — 交互确认
- [toml](https://github.com/BurntSushi/toml) — 配置解析
- [godotenv](https://github.com/joho/godotenv) — `.env` 解析
