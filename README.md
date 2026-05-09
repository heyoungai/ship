# ship

Docker 镜像构建、推送和远程部署 CLI 工具。Go 实现，编译为单二进制，无运行时依赖。

支持 `ship.toml` 配置文件和矩阵构建（多品牌/多变体）。

## 快速开始

```bash
# 编译
go build -o ship.exe .

# 查看帮助
./ship.exe --help

# 查看当前版本
./ship.exe version
```

## 子命令

| 命令 | 说明 |
|------|------|
| `version` | 显示当前 git tag 版本号 |
| `build` | 构建 Docker 镜像 |
| `tag` | 给镜像打 tag |
| `push` | 推送镜像到所有配置的仓库 |
| `deploy` | 远程部署：更新版本号并重启容器 |
| `run` | 执行完整流程: build → tag → push → deploy |

## 用法

```bash
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

在项目根目录创建 `ship.toml`，参考 `config.example.toml`。

### 配置优先级

```
环境变量 > ship.toml > 内置默认值
```

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

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `IMAGE_NAME` | `home` | 镜像名称 |
| `PLATFORMS` | `linux/amd64` | 构建目标平台 |
| `DOCKERFILE` | `./Dockerfile` | Dockerfile 路径 |
| `ENV_FILE` | `./.env.local` | .env 文件路径 |
| `REMOTE_HOST` | `deali.cn` | SSH Host |
| `REMOTE_PROJECT_PATH` | `/home/deali/projects/home` | 远程项目路径 |

## 依赖

- [cobra](https://github.com/spf13/cobra) — CLI 框架
- [toml](https://github.com/BurntSushi/toml) — TOML 配置解析
- [godotenv](https://github.com/joho/godotenv) — .env 文件解析
- [lipgloss](https://github.com/charmbracelet/lipgloss) — 终端样式
