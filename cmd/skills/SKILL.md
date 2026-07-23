---
name: ship
description: Use the ship CLI tool for Docker image and Go binary build, push, and deploy workflows.
version: 1
---

# Ship — Docker 镜像 & Go 二进制构建部署工具

Ship 是一个 CLI 工具，支持 **build → tag → push → deploy → verify** 全流程，也可分步执行。配置文件为项目根目录的 `ship.toml`（schema = 2）。

> **Skill 版本**：frontmatter 中的 `version` 为 skill 文档兼容版本（整数）。内容变更时递增；`ship build` / `ship run` 若检测到项目内已安装的 skill 过旧，会警告并提示执行 `ship skill -f`。

---

## 快速开始

```bash
ship init          # 在当前目录生成 ship.toml（自动探测项目信息）
ship plan -v v1.0.0   # 预览 release 计划（不执行）
ship doctor -v v1.0.0 # 检查 release 运行条件
ship run -v v1.0.0 -y # 一键执行：build → tag → push → deploy → verify
```

默认 `version.source = "git-tag"` 时，`-v` / `SHIP_VERSION` 必须是**本地真实 Git tag**。构建来自该 tag 的源码快照（worktree），不切换当前分支，也不带上未提交修改。

---

## 命令速查

| 命令 | 用途 | 关键 Flags |
|------|------|-----------|
| `ship init` | 生成 ship.toml | `-f` 强制覆盖 |
| `ship plan` | 展示 release 计划（不执行） | `-v`, `-p`, `--env-file`, `--json`, `--skip-deploy` |
| `ship doctor` | 检查 release 运行条件 | `-v`, `-p`, `--env-file` |
| `ship build` | 构建镜像/二进制 | `-v`, `-p`, `--env-file` |
| `ship tag` | 为镜像打远程标签 | `-v`, `-p` |
| `ship push` | 推送镜像/上传文件 | `-v`, `-p`, `--promote-latest` |
| `ship deploy` | 按已发布 manifest 部署 | `-v` |
| `ship run` | 全流水线 | `-v`, `-p`, `--env-file`, `--skip-deploy`, `--promote-latest` |
| `ship rollback` | 回滚到指定版本 | `-v` 目标版本, `-y` |
| `ship history` | 查看部署历史 | `-n` 条数（默认 20） |
| `ship current` | 显示当前 git tag 版本 | — |
| `ship version` | 显示 ship 工具版本 | — |
| `ship skill` | 安装/更新本 skill 到项目 | `-f` 强制覆盖 |

> **未识别配置项**：`ship.toml` 中未在 schema 定义的字段默认会报错。可用 `[config] unknown_keys = "warn"` 或 `SHIP_UNKNOWN_KEYS=warn` 降级。

> **重要**：agent 调用任何需要确认的命令时，必须加 `-y` 跳过交互提示。

---

## 发布语义（git-tag 默认）

- `ship plan` / `ship doctor` 可先预览计划与检查条件。
- `ship run` / `ship build` 写入 `.ship/runs/`；成功发布后索引到 `.ship/releases/`。
- 独立 `push` / `deploy` / `rollback` **按 release manifest 消费产物**；尚未发布过的版本会失败（不会偷偷从当前目录补构建）。
- 默认按 **digest 钉部署**（写入 `APP_IMAGE_DIGEST`，同时保留版本别名）。生产 compose 推荐：

```yaml
image: registry.example.com/ns/app@${APP_IMAGE_DIGEST}
```

- `deploy` / `rollback` 不修改 registry `:latest`；需要移动 `latest` 时用 `--promote-latest`（生产建议关闭 `tag_latest_on_default_profile`）。

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
platforms = ["linux/amd64"]   # 当前须单平台（依赖 --load）
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
# 生产建议 false；需要 :latest 时用 ship push/run --promote-latest
tag_latest_on_default_profile = true

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
tag_key = "{{ vars.remote_tag_key }}"          # env 中版本别名键（默认 APP_IMAGE_TAG）
pin = "digest"                                 # digest（默认）| tag
digest_key = "APP_IMAGE_DIGEST"                # pin=digest 时写入内容身份
# image_key = "APP_IMAGE"                      # 可选：写入 repo@digest
up = "docker compose --env-file ./.env up -d --remove-orphans"
```

部署流程：按 release manifest 取产物 → 上传 compose/env → 按 pin 写入 digest/tag → 执行 `up`。

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
default = true                              # default profile 镜像 tag 无后缀
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

| 环境变量 | 覆盖目标 |
|----------|---------|
| `IMAGE_NAME` | `build.docker.image` |
| `PLATFORMS` | `build.docker.platforms` |
| `DOCKERFILE` | `build.docker.dockerfile` |
| `ENV_FILE` | `build.docker.env_file` |
| `REMOTE_HOST` | `deploy.compose.host` |
| `REMOTE_PROJECT_PATH` | `deploy.compose.path` |
| `SHIP_VERSION` | 版本号（须为真实 tag，当 `version.source=git-tag`） |
| `SHIP_UNKNOWN_KEYS` | `error` / `warn` / `ignore` |

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

# 2. 编辑 ship.toml（registry、deploy、pin=digest）

# 3. 预检与发布
ship plan -v v1.0.0
ship doctor -v v1.0.0
ship run -v v1.0.0 -y

# 4. 回滚（消费历史 digest/manifest）
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
# ship build -v v1.0.0 -y
```

### 场景 4：更新 agent skill

```bash
ship skill        # 安装到 .claude/skills/ship/SKILL.md
ship skill -f     # 强制覆盖（skill 过期警告时使用）
```

---

## 注意事项

- `schema = 2` 是必须的，ship 不接受其他 schema 版本
- `.ship/` 目录存储 runs、releases、部署历史，应加入 `.gitignore`
- agent / CI 环境调用时务必加 `-y` 跳过交互提示
- Docker 构建当前要求 `load = true`，且 `platforms` 保持单平台
- rollback 仅支持 compose 部署驱动
- 独立 `deploy`/`push` 需要该版本已成功发布过（有 release manifest）
- 版本号格式建议使用 semver（如 `v1.2.3`），与 git tag 保持一致
- Skill frontmatter `version` 与 ship 二进制版本无关；文档变更时手动递增
