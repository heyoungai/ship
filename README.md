# ship

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Docker 镜像构建、推送和远程部署 CLI 工具。Go 实现，编译为单二进制，无运行时依赖。

仅支持 **`ship.toml` v2 schema**，支持矩阵构建（多品牌/多变体）。命令层基于 **Cobra**，输出层使用 **PTerm**，交互确认使用 **Huh**。

快速上手入口：查看 [docs/quick-start.md](docs/quick-start.md)。

设计与接入约束：

- [docs/design-principles.md](docs/design-principles.md)
- [docs/docker-default-pattern.md](docs/docker-default-pattern.md)

当前命令层已跑通两条主路径，并补齐了 v2 的阶段执行能力：

- **Docker 发布链路**：`build.driver = "docker"` → `publish.driver = "registry"` → `deploy.driver = "compose"`
- **Go 二进制链路**：`build.driver = "go-binary"` → `publish.driver = "scp"` → `deploy.driver = "binary-install"`
- **阶段扩展能力**：`steps.*`、`templates`、`verify.*`、`version.source/fallback/override_env`

其中 Docker 的 `build → tag → push` 分阶段流程会将 `docker buildx` 产物 `--load` 回本地 Docker，再继续后续步骤，因此 **`build.docker.platforms` 目前应保持单平台**（如 `linux/amd64`）。

同样基于这条 staged flow，当前 `build.docker.load` 必须保持为 `true`，`build.docker.disable_buildkit` 暂不支持。

## 安装

### Scoop (Windows)

```bash
# 添加 bucket
scoop bucket add ship https://github.com/heyoungai/ship

# 安装
scoop install ship
```

### Go

```bash
go install github.com/heyoungai/ship@latest
```

### 从源码编译

```bash
# 安装 Task（如尚未安装）
go install github.com/go-task/task/v3/cmd/task@latest

# 编译
task build
```

## 快速开始

```bash
# 初始化配置
ship init

# 查看帮助
ship --help
```

## 子命令

| 命令 | 说明 |
|------|------|
| `init` | 在当前目录初始化 ship.toml 配置文件 |
| `version` | 显示 ship 工具版本号 |
| `current` | 显示当前项目的 git tag 版本号 |
| `build` | 执行 prepare / templates / build |
| `tag` | 给镜像打 tag |
| `push` | 推送镜像到所有配置的仓库 |
| `deploy` | 远程部署：更新版本号并重启容器 |
| `rollback` | 回滚到上一个成功部署的版本 |
| `history` | 查看部署历史记录 |
| `run` | 执行完整流程: build → tag → push → deploy |

## 用法

```bash
# 初始化配置（自动探测项目信息）
./ship.exe init
./ship.exe init --force  # 覆盖已有配置

# 单独执行各阶段
./ship.exe build
./ship.exe build -v v2.0.0
./ship.exe tag -v v2.0.0
./ship.exe push -v v2.0.0
./ship.exe deploy -v v2.0.0

# 一键全流程
./ship.exe run
./ship.exe run --skip-deploy
./ship.exe run -v v2.0.0 --env-file ./.env.local
./ship.exe run -p brand-a
./ship.exe init --yes
./ship.exe rollback --yes

# 矩阵构建：指定 profile
./ship.exe build -p brand-a
./ship.exe tag -v v2.0.0 -p brand-a
./ship.exe push -v v2.0.0 -p brand-a

# 回滚部署
./ship.exe rollback              # 回滚到上一个成功版本
./ship.exe rollback -v v1.9.0    # 回滚到指定版本

# 查看部署历史
./ship.exe history               # 最近 20 条
./ship.exe history -n 5          # 最近 5 条
```

## 配置

在项目根目录运行 `ship init` 自动生成 `ship.toml`，或参考 `config.example.toml` 手动创建。当前只接受 `schema = 2`。

### 配置优先级

```
环境变量 > ship.toml > 内置默认值（仅通用字段）
```

### 必填字段

以下字段缺失或非法时会报错（一次性列出所有缺失项）：

- `schema = 2`
- `build.driver`
- `publish.driver`
- `build.docker.image` — 当 `build.driver = "docker"` 时必填
- `publish.registry.targets` — 当 `publish.driver = "registry"` 时至少配置一个目标仓库
- `deploy.compose.host` / `deploy.compose.path` — 当 `features.deploy = true` 且 `deploy.driver = "compose"` 时必填

`deploy.compose.local_file` 和 `deploy.compose.local_env_file` 为可选项；启用后 ship 会先确保远端目录存在，再通过 `scp` 上传本地 compose / env 文件，然后继续更新 `tag_key` 和执行 `docker compose up`。

### env_file 配置说明

`deploy.compose` 下有三个与环境文件相关的字段：

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `env_file` | 远端 env 文件名，用于写入镜像 tag 并作为 compose 的环境变量来源 | `.env` |
| `local_env_file` | 本地 env 文件路径，部署前通过 scp 上传到远端（可选） | 空 |
| `auto_env_file` | 当 `env_file` 不是 `.env` 时，自动将 `--env-file` 注入 `up` 命令 | `true` |

`auto_env_file` 是一个便捷特性：当你将 `env_file` 设置为 `.env.prod` 等非默认值时，ship 会自动在 `up` 命令中注入 `--env-file ./.env.prod`，无需手动修改 `up` 命令。如果你的 `up` 命令已经显式包含 `--env-file`，或者你希望完全手动控制，可以设置 `auto_env_file = false` 关闭此行为。

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

### 多品牌项目（矩阵构建）

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

矩阵构建时，每个 profile 会：
- 带 profile 环境变量执行构建
- 生成带后缀的镜像 tag（如 `canvas-studio:v2.0.0-brand-a`）
- 默认 profile 额外打 `:latest` tag

### 阶段编排

- `build` 会先执行 `steps.prepare`、渲染 `templates`，再进入真正的 build driver
- `run` 会按 `build → tag → publish → deploy → verify` 顺序执行，并在 publish / deploy 两侧插入对应 hooks
- `deploy` / `rollback` 会复用同一套 `pre_deploy → deploy → post_deploy → verify` 流程

### 健康检查与校验

优先使用 `[verify]` 配置部署后校验：

- `[verify.http]`：HTTP 健康检查
- `[verify.ssh]`：SSH 命令校验
- `[verify.command]`：本地命令校验

当前仍兼容旧的 `[deploy.healthcheck]`，但新配置推荐统一迁移到 `[verify]`。

### 交互确认

- `init` 检测到已有 `ship.toml` 时，会弹出 Huh 确认
- `rollback` 执行前，会弹出 Huh 最终确认
- 在 CI 或非交互终端中，请使用 `-y` / `--yes` 跳过确认

## 环境变量

| 变量 | 说明 |
|------|------|
| `IMAGE_NAME` | 覆盖 `build.docker.image` |
| `PLATFORMS` | 覆盖 `build.docker.platforms` |
| `DOCKERFILE` | 覆盖 `build.docker.dockerfile` |
| `ENV_FILE` | 覆盖 `build.docker.env_file` |
| `REMOTE_HOST` | 覆盖 `deploy.compose.host` |
| `REMOTE_PROJECT_PATH` | 覆盖 `deploy.compose.path` |

## 设计说明

- `build` 现已支持 `-p/--profile` 和 `-v/--version`
- `local_build` 会按平台选择 shell：Windows 使用 PowerShell，Linux/macOS 使用 `sh`
- `steps.*`、`templates`、`verify.*` 已接入命令执行链路
- `version.source`、`version.fallback`、`version.override_env` 已参与实际版本解析
- 历史记录写入与配置校验不再静默失败，错误会直接向上返回
- 阶段输出、历史表格和提示已切到 PTerm，避免继续维护手写进度输出
- 配置解析层已收敛为 **v2-only**，不再接受旧版 `registries / deploy.enabled / build.platforms` 写法

## 开发

```bash
# 运行测试
task test

# 查看测试覆盖率
task cover

# 开发编译（版本号为 dev）
task build

# 发布编译（注入版本号）
task build VERSION=v1.0.0

# 清理构建产物
task clean
```

## 依赖

- [cobra](https://github.com/spf13/cobra) — CLI 框架
- [pterm](https://github.com/pterm/pterm) — 终端输出、section、table
- [huh](https://github.com/charmbracelet/huh) — 交互式确认与表单
- [toml](https://github.com/BurntSushi/toml) — TOML 配置解析
- [godotenv](https://github.com/joho/godotenv) — .env 文件解析
