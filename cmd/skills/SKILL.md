# Ship — Docker 镜像 & Go 二进制构建部署工具

Ship 是一个 CLI 工具，支持 **build → tag → push → deploy → verify** 全流程，也可分步执行。配置文件为项目根目录的 `ship.toml`（schema = 2）。

---

## 快速开始

```bash
ship init          # 在当前目录生成 ship.toml（自动探测项目信息）
ship run           # 一键执行完整流水线：build → tag → push → deploy → verify
ship run -y        # 非交互模式（CI / agent 调用时必须加 -y）
```

---

## 命令速查

| 命令 | 用途 | 关键 Flags |
|------|------|-----------|
| `ship init` | 生成 ship.toml | `-f` 强制覆盖 |
| `ship build` | 构建镜像/二进制 | `-v` 版本, `-p` profile, `--env-file` |
| `ship tag` | 为镜像打远程标签 | `-v`, `-p` |
| `ship push` | 推送镜像/上传文件 | `-v`, `-p` |
| `ship deploy` | 部署到远程服务器 | `-v` |
| `ship run` | 全流水线 | `-v`, `-p`, `--env-file`, `--skip-deploy` |
| `ship rollback` | 回滚到指定版本 | `-v` 目标版本, `-y` |
| `ship history` | 查看部署历史 | `-n` 条数（默认 20） |
| `ship current` | 显示当前 git tag 版本 | — |
| `ship version` | 显示 ship 工具版本 | — |

> **重要**：agent 调用任何需要确认的命令时，必须加 `-y` 跳过交互提示。

---

## 版本策略

`[version]` 控制版本号来源：

```toml
[version]
source = "git-tag"       # git-tag（默认）| env | static
fallback = "dev"          # error | dev | static
override_env = "SHIP_VERSION"  # 环境变量优先覆盖
```

优先级：`SHIP_VERSION` 环境变量 > `version.source` > `version.fallback`

---

## ship.toml 配置参考

### 项目元信息

```toml
[project]
name = "myapp"
description = "项目描述"
```

### 功能开关

```toml
[features]
publish = true
deploy = true
rollback = true
verify = true
```

### 自定义变量

在模板中通过 `{{ vars.key }}` 引用：

```toml
[vars]
app_name = "myapp"
remote_tag_key = "APP_IMAGE_TAG"
```

### 生命周期钩子

支持 6 个阶段的步骤，每个步骤可指定 `profiles` 过滤：

```toml
[[steps.prepare]]
name = "build-frontend"
run = "bun run build"
cwd = "."
profiles = ["*"]       # "*" 表示所有 profile

# 其他阶段：post_build, pre_publish, post_publish, pre_deploy, post_deploy
```

### 模板生成

在构建前生成文件，支持 `{{ version }}`、`{{ vars.key }}` 等变量：

```toml
[[templates]]
path = "./deploy/.env"
profiles = ["*"]
content = """
APP_IMAGE_TAG={{ version }}
"""
```

也可从文件复制：`from = "./templates/docker-compose.tpl"`（与 `content` 互斥）。

### 构建配置

三种构建驱动：

#### Docker 构建

```toml
[build]
driver = "docker"

[build.docker]
image = "myapp"
context = "."
dockerfile = "./Dockerfile"
platforms = ["linux/amd64"]
env_file = "./.env.local"       # 构建时环境变量文件
build_args_from_env = true      # 将 env_file 中的变量作为 --build-arg
local_build = "bun run build"   # 可选：Docker 构建前先本地构建
load = true                     # 当前必须为 true
cache_bust = false

# 显式 build args（可选）：
# [build.docker.build_args]
# API_BASE = "https://api.example.com"
```

#### Go 二进制构建

```toml
[build]
driver = "go-binary"

[build.go]
main = "./cmd/myapp"
output = "./build/{{ executable_name }}"
goos = "linux"
goarch = "amd64"
cgo_enabled = false
executable_name = "myapp"
ldflags = ["-s", "-w", "-X main.Version={{ version }}"]
```

#### 自定义命令构建

```toml
[build]
driver = "command"

[build.command]
run = "make build"
cwd = "."
outputs = ["./dist/app"]
```

### 发布配置

```toml
# 推送到镜像仓库
[publish]
driver = "registry"

[publish.registry]
push = true

[[publish.registry.targets]]
type = "private"                        # dockerhub | private
url = "registry.cn-hangzhou.aliyuncs.com"
namespace = "mynamespace"
image = "myapp"

# 或 SCP 上传文件
# [publish]
# driver = "scp"
#
# [publish.scp]
# local = "./build/myapp"
# host = "myserver.com"
# remote = "/tmp/myapp"

# 不发布
# [publish]
# driver = "none"
```

### 部署配置

三种部署驱动：

#### Docker Compose 部署

```toml
[deploy]
driver = "compose"

[deploy.compose]
host = "myserver.com"                          # SSH Host（~/.ssh/config 别名或 user@ip）
path = "/home/user/projects/myapp"             # 远程项目路径
local_file = "./deploy/compose.prod.yaml"      # 可选：上传本地 compose 文件
remote_file = "compose.yaml"                   # 可选：远端文件名
local_env_file = "./deploy/.env.prod"          # 可选：先上传本地 env 文件
env_file = ".env"                              # 远端 env 文件名
auto_env_file = true                           # 自动注入 --env-file 到 up 命令
tag_key = "{{ vars.remote_tag_key }}"          # env 文件中存放镜像 tag 的变量名
up = "docker compose --env-file ./.env up -d --remove-orphans"
```

部署流程：上传 compose 文件 → 上传 env 文件 → 更新 tag_key 为新版本 → 执行 `up` 命令重启容器。

#### Go 二进制安装

```toml
[deploy]
driver = "binary-install"

[deploy.binary_install]
host = "myserver.com"
remote_temp_path = "/tmp"
remote_install_path = "/usr/local/bin"
use_ssh_tty = true
sudo_nopasswd = false
chmod = "+x"
```

#### SSH 命令部署

```toml
[deploy]
driver = "ssh"

[deploy.ssh]
host = "myserver.com"
commands = [
    "systemctl restart my-service",
    "systemctl status my-service --no-pager",
]
```

### 部署后校验

```toml
# HTTP 健康检查
[verify]
driver = "http"

[verify.http]
url = "https://myserver.com/api/health"
expected_status = 200
attempts = 20
interval_seconds = 3
timeout_seconds = 5

# SSH 命令校验
# [verify]
# driver = "ssh"
#
# [verify.ssh]
# host = "myserver.com"
# command = "docker compose ps"

# 本地命令校验
# [verify]
# driver = "command"
#
# [verify.command]
# run = "test -f ./deploy/.env"
```

### 矩阵构建

多品牌/多变体场景，每个 profile 可有独立的环境变量和模板变量：

```toml
[[matrix]]
name = "brand-a"
default = true                              # default profile 使用 :latest 标签
env = { NEXT_PUBLIC_APP_BRAND = "brand-a" }
vars = { brand = "brand-a" }

[[matrix]]
name = "brand-b"
env = { NEXT_PUBLIC_APP_BRAND = "brand-b" }
vars = { brand = "brand-b" }
```

使用 `-p brand-a` 选择单个 profile；不指定则构建所有 profile。非 default profile 的 tag 会带后缀，如 `v2.0.0-brand-b`。

---

## 环境变量覆盖

以下环境变量可覆盖 ship.toml 中的对应配置：

| 环境变量 | 覆盖目标 |
|----------|---------|
| `IMAGE_NAME` | `build.docker.image` |
| `PLATFORMS` | `build.docker.platforms` |
| `DOCKERFILE` | `build.docker.dockerfile` |
| `ENV_FILE` | `build.docker.env_file` |
| `REMOTE_HOST` | `deploy.compose.host` |
| `REMOTE_PROJECT_PATH` | `deploy.compose.path` |
| `SHIP_VERSION` | 版本号（可通过 `version.override_env` 自定义变量名） |

---

## 模板变量

在 `[[templates]]` 和各配置字段中可使用 `{{ variable }}` 语法：

**内置变量**：`version`、`module`、`image_name`、`project.name`、`project.description`、`profile.name`、`profile.default`、`build.platforms`、`build.driver`、`build.docker.image`、`publish.driver`、`deploy.driver`、`verify.driver`、`deploy.enabled`

**自定义变量**：`vars.<key>`（来自 `[vars]` 和 matrix `vars`）、`env.<key>`（来自 matrix `env`）

---

## 典型场景

### 场景 1：Docker 项目全流程

```bash
# 1. 初始化
ship init

# 2. 编辑 ship.toml，配置 registry 和 deploy

# 3. 一键构建部署
ship run -y

# 4. 回滚
ship rollback -v v1.0.0 -y
```

### 场景 2：Go 二进制项目

```toml
[build]
driver = "go-binary"
[build.go]
main = "./cmd/cli"
output = "./build/cli"
goos = "linux"
goarch = "amd64"

[publish]
driver = "scp"
[publish.scp]
local = "./build/cli"
host = "myserver.com"
remote = "/tmp/cli"

[deploy]
driver = "binary-install"
[deploy.binary_install]
host = "myserver.com"
remote_install_path = "/usr/local/bin"
```

### 场景 3：仅构建不部署

```toml
[features]
deploy = false
publish = false

# 或在命令行：
# ship build -y
```

---

## 注意事项

- `schema = 2` 是必须的，ship 不接受其他 schema 版本
- `.ship/` 目录存储部署历史记录，应加入 `.gitignore`
- agent / CI 环境调用时务必加 `-y` 跳过交互提示
- Docker 构建当前要求 `load = true`
- rollback 仅支持 compose 部署驱动
- 版本号格式建议使用 semver（如 `v1.2.3`），与 git tag 保持一致
