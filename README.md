# ship

Docker 镜像构建、推送和远程部署 CLI 工具。Go 实现，编译为单二进制，无运行时依赖。

支持 `ship.toml` 配置文件和矩阵构建（多品牌/多变体）。

## 快速开始

```bash
# 编译（注入版本号）
go build -ldflags "-X ship/cmd.Version=v1.0.0" -o ship.exe .

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

# 矩阵构建：指定 profile
./ship.exe build -p linglu
./ship.exe tag -v v2.0.0 -p linglu
./ship.exe push -v v2.0.0 -p linglu
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
name = "linglu"
default = true
env = { NEXT_PUBLIC_APP_BRAND = "linglu" }

[[matrix]]
name = "fumian-denim"
env = { NEXT_PUBLIC_APP_BRAND = "fumian-denim" }
```

矩阵构建时，每个 profile 会：
- 带 profile 环境变量执行构建
- 生成带后缀的镜像 tag（如 `canvas-studio:v2.0.0-linglu`）
- 默认 profile 额外打 `:latest` tag

## 环境变量

| 变量 | 说明 |
|------|------|
| `IMAGE_NAME` | 镜像名称（覆盖 ship.toml） |
| `PLATFORMS` | 构建目标平台 |
| `DOCKERFILE` | Dockerfile 路径 |
| `ENV_FILE` | .env 文件路径 |
| `REMOTE_HOST` | SSH Host |
| `REMOTE_PROJECT_PATH` | 远程项目路径 |

## 编译

```bash
# 开发编译
go build -o ship.exe .

# 发布编译（注入版本号）
go build -ldflags "-X ship/cmd.Version=v1.0.0" -o ship.exe .
```

## 依赖

- [cobra](https://github.com/spf13/cobra) — CLI 框架
- [toml](https://github.com/BurntSushi/toml) — TOML 配置解析
- [godotenv](https://github.com/joho/godotenv) — .env 文件解析
- [lipgloss](https://github.com/charmbracelet/lipgloss) — 终端样式
