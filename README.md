# ship

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

Docker 镜像构建、推送和远程部署 CLI 工具。Go 实现，编译为单二进制，无运行时依赖。

支持 `ship.toml` 配置文件和矩阵构建（多品牌/多变体）。命令层基于 **Cobra**，输出层使用 **PTerm**，交互确认使用 **Huh**。

当前 `build → tag → push` 分阶段流程会将 `docker buildx` 产物 `--load` 回本地 Docker，再继续后续步骤，因此 **`build.platforms` 目前应保持单平台**（如 `linux/amd64`）。

## 快速开始

```bash
# 安装 Task（如尚未安装）
go install github.com/go-task/task/v3/cmd/task@latest

# 编译
task build

# 初始化配置
./ship.exe init

# 查看帮助
./ship.exe --help
```

## 子命令

| 命令 | 说明 |
|------|------|
| `init` | 在当前目录初始化 ship.toml 配置文件 |
| `version` | 显示 ship 工具版本号 |
| `current` | 显示当前项目的 git tag 版本号 |
| `build` | 构建 Docker 镜像 |
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

在项目根目录运行 `ship init` 自动生成 `ship.toml`，或参考 `config.example.toml` 手动创建。

### 配置优先级

```
环境变量 > ship.toml > 内置默认值（仅通用字段）
```

### 必填字段

以下字段缺失时会报错（一次性列出所有缺失项）：

- `image_name` — 从第一个 registry 的 image 自动推导，或通过 `IMAGE_NAME` 环境变量设置
- `registries` — 至少配置一个镜像仓库
- `deploy.host` / `deploy.path` — 仅在 `deploy.enabled = true` 时必填

### 简单项目（无矩阵）

```toml
[build]
platforms = "linux/amd64"
dockerfile = "./Dockerfile"
env_file = "./.env.local"

[[registries]]
type = "private"
url = "registry.cn-hangzhou.aliyuncs.com"
namespace = "deali"
image = "home"

[deploy]
enabled = true
host = "deali.cn"
path = "/home/deali/projects/home"

[deploy.healthcheck]
url = "https://deali.cn/api/health"
expected_status = 200
attempts = 20
interval_seconds = 3
timeout_seconds = 5
```

### 多品牌项目（矩阵构建）

```toml
[build]
platforms = "linux/amd64"
dockerfile = "./Dockerfile"
env_file = "./.env"

[[registries]]
type = "private"
url = "ccr.ccs.tencentyun.com"
namespace = "deali"
image = "canvas-studio"

[deploy]
enabled = false

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

### 健康检查

配置 `[deploy.healthcheck]` 后，`deploy` / `run` / `rollback` 会在远程 `docker compose up -d` 之后轮询 URL，只有返回期望状态码才算成功。

### 交互确认

- `init` 检测到已有 `ship.toml` 时，会弹出 Huh 确认
- `rollback` 执行前，会弹出 Huh 最终确认
- 在 CI 或非交互终端中，请使用 `-y` / `--yes` 跳过确认

## 环境变量

| 变量 | 说明 |
|------|------|
| `IMAGE_NAME` | 镜像名称（覆盖 ship.toml） |
| `PLATFORMS` | 构建目标平台 |
| `DOCKERFILE` | Dockerfile 路径 |
| `ENV_FILE` | .env 文件路径 |
| `REMOTE_HOST` | SSH Host |
| `REMOTE_PROJECT_PATH` | 远程项目路径 |

## 设计说明

- `build` 现已支持 `-p/--profile`
- `local_build` 会按平台选择 shell：Windows 使用 PowerShell，Linux/macOS 使用 `sh`
- 历史记录写入与配置校验不再静默失败，错误会直接向上返回
- 阶段输出、历史表格和提示已切到 PTerm，避免继续维护手写进度输出

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
