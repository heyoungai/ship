# ship.toml v2 配置模型（草案）

> 状态：设计草案  
> 说明：本文描述的是 `ship` 下一阶段的目标配置模型，**并不表示当前代码已经完整实现**。

## 目标

`ship` 需要从“固定的 Docker 镜像构建/推送/部署工具”，升级成“适用于多类项目的发布编排器”。

这份 v2 配置模型要覆盖三类典型项目：

1. **标准 Docker 镜像项目**  
   例如 `deali-home-fumadocs`、`avatar-sense`、`django-starter`
2. **多阶段产物组装项目**  
   例如 `2\chats`：后端 publish、前端 build、复制静态资源、生成运行时文件、镜像化、compose 部署
3. **Go 二进制发布项目**  
   例如 `server-tools\swag`：交叉编译、SCP 上传、远程安装

## 设计原则

1. **保留现在 ship 的易用性**  
   常规 Docker 项目不应该因为更通用而更难写。
2. **内置策略优先，Shell 作为逃生口**  
   不能让 `ship.toml` 退化成另一种脚本语言。
3. **阶段清晰**  
   外部依然按 `prepare -> build -> publish -> deploy -> verify` 理解。
4. **内部可扩展**  
   每个阶段通过 `driver` 决定具体行为，从而覆盖不同项目类型。

## 顶层结构

```toml
schema = 2

[project]
[version]
[features]
[vars]

[[matrix]]

[build]
[publish]
[deploy]
[verify]

[[steps.prepare]]
[[steps.post_build]]
[[steps.pre_publish]]
[[steps.post_publish]]
[[steps.pre_deploy]]
[[steps.post_deploy]]

[[templates]]
```

## 顶层字段说明

| 区块 | 作用 |
|---|---|
| `schema` | 配置版本，便于后续兼容迁移 |
| `[project]` | 项目名称、描述等元信息 |
| `[version]` | 版本来源与 fallback 策略 |
| `[features]` | 启用/禁用 publish、deploy、verify 等能力 |
| `[vars]` | 用户自定义变量，可供模板和命令插值 |
| `[[matrix]]` | 多品牌/多变体构建 |
| `[build]` | 构建阶段与构建产物定义 |
| `[publish]` | 发布目标定义 |
| `[deploy]` | 远程部署策略 |
| `[verify]` | 部署后验证 |
| `[[steps.*]]` | 各阶段前后自定义步骤 |
| `[[templates]]` | 生成运行时文件 / compose / Dockerfile / .env 等 |

## 版本模型

```toml
[version]
source = "git-tag"        # git-tag | env | static
fallback = "error"        # error | dev | static
static = ""               # fallback=static 时使用
override_env = "SHIP_VERSION"
```

这套设计要统一当前不同脚本里已经存在的三种版本策略：

1. 必须使用最新 git tag
2. 失败时回退到 `dev`
3. 使用固定版本或环境变量版本

## 构建模型

`[build]` 负责定义“产物怎么来”。

### Docker 镜像构建

```toml
[build]
driver = "docker"

[build.docker]
image = "home"
context = "."
dockerfile = "./Dockerfile"
platforms = ["linux/amd64"]
env_file = "./.env.local"
build_args_from_env = true
load = true
latest_on_default_profile = true
disable_buildkit = false
cache_bust = false
```

### Go 二进制构建

```toml
[build]
driver = "go-binary"

[build.go]
main = "./cmd/swag-cli"
output = "./build/{{ executable_name }}"
goos = "linux"
goarch = "amd64"
cgo_enabled = false
executable_name = "swag-cli"
ldflags = [
  "-s",
  "-w",
  "-X {{ module }}/internal/cli.Version={{ version }}"
]
```

### 自定义命令构建

```toml
[build]
driver = "command"

[build.command]
run = "make build"
cwd = "."
outputs = ["./dist/app"]
```

## 自定义步骤模型

`steps` 是给 `chats` 这类项目准备的，用来覆盖“先组装产物，再进入 build/publish/deploy”的场景。

```toml
[[steps.prepare]]
name = "build-frontend"
run = "bun run build"
cwd = "./src/FE"
env = { FE_VERSION = "{{ version }}" }
profiles = ["*"]

[[steps.prepare]]
name = "publish-backend"
run = "dotnet publish ./src/BE/web/Chats.BE.csproj -c Release -o ./dist"
profiles = ["*"]
```

每个 step 建议支持这些字段：

| 字段 | 说明 |
|---|---|
| `name` | 展示名 |
| `run` | 要执行的命令 |
| `cwd` | 工作目录 |
| `env` | 额外环境变量 |
| `shell` | `auto` / `powershell` / `sh` |
| `profiles` | 生效的 profile，`["*"]` 表示全部 |
| `enabled` | 是否启用 |

> 第一版**不建议**引入通用表达式语言。`profiles = ["*"]`、`["brand-a"]` 这种简单选择器已经足够。

## 模板文件模型

很多项目不只是“构建产物”，而是要“**生成发布物料**”，例如：

- 运行时 Dockerfile
- `docker-compose.yaml`
- `.env`

所以需要 `[[templates]]`：

```toml
[[templates]]
path = "./dist/Dockerfile"
content = """
FROM mcr.microsoft.com/dotnet/aspnet:10.0
WORKDIR /app
COPY . /app
ENTRYPOINT ["dotnet", "Chats.BE.dll"]
"""

[[templates]]
path = "./dist-docker/.env"
content = """
APP_IMAGE_TAG={{ version }}
"""

[[templates]]
path = "./dist-docker/docker-compose.yaml"
from = "./scripts/templates/docker-compose.tpl"
```

建议字段：

| 字段 | 说明 |
|---|---|
| `path` | 输出路径 |
| `content` | 直接内联模板内容 |
| `from` | 从模板文件渲染 |
| `profiles` | 生效 profile |
| `mode` | 文件权限，可选 |

## 发布模型

### 发布到镜像仓库

```toml
[publish]
driver = "registry"

[[publish.registry.targets]]
type = "private"
url = "registry.cn-hangzhou.aliyuncs.com"
namespace = "deali"
image = "home"

[[publish.registry.targets]]
type = "dockerhub"
namespace = "dealiaxy"
image = "home"

[publish.registry]
push = true
tag_latest_on_default_profile = true
```

### 发布为 SCP 上传

```toml
[publish]
driver = "scp"

[publish.scp]
local = "./build/swag-cli"
host = "deali.cn"
remote = "/tmp/swag-cli"
```

## 部署模型

### Compose 部署

```toml
[deploy]
driver = "compose"

[deploy.compose]
host = "deali.cn"
path = "/home/deali/projects/home"
env_file = ".env"
tag_key = "APP_IMAGE_TAG"
up = "docker compose --env-file ./.env up -d --remove-orphans"
```

这一步要覆盖：

- 当前 ship 的 `deploy.path`
- `chats` 里的 `REMOTE_TAG_KEY`
- `--remove-orphans`
- 自定义 compose 启动命令

### Go 二进制安装

```toml
[deploy]
driver = "binary-install"

[deploy.binary_install]
host = "deali.cn"
remote_temp_path = "/tmp"
remote_install_path = "/usr/local/bin"
use_ssh_tty = true
sudo_nopasswd = false
chmod = "+x"
```

### 任意 SSH 命令

```toml
[deploy]
driver = "ssh"

[deploy.ssh]
host = "deali.cn"
commands = [
  "systemctl restart my-service",
  "systemctl status my-service --no-pager"
]
```

## 验证模型

### HTTP 健康检查

```toml
[verify]
driver = "http"

[verify.http]
url = "https://deali.cn/api/health"
expected_status = 200
attempts = 20
interval_seconds = 3
timeout_seconds = 5
```

### SSH 校验

```toml
[verify]
driver = "ssh"

[verify.ssh]
host = "deali.cn"
command = "docker compose ps"
```

### 本地命令校验

```toml
[verify]
driver = "command"

[verify.command]
run = "test -f ./dist/Dockerfile"
shell = "sh"
```

## matrix 模型

建议保留现在的思路，但增强为同时支持 `env` 和 `vars`：

```toml
[[matrix]]
name = "brand-a"
default = true
env = { NEXT_PUBLIC_APP_BRAND = "brand-a" }
vars = { brand = "brand-a" }
```

这样：

- `env` 用于真正影响构建
- `vars` 用于模板渲染 / 远程路径拼接 / tag 模板

## 建议保留与迁移的旧字段

### 建议保留

- `matrix`
- `env_file`
- `local_build`
- registry target 的基础字段
- compose deploy 的 host/path 思路

### 建议迁移

| v1 字段 | v2 去向 |
|---|---|
| `registries` | `publish.registry.targets` |
| `deploy.enabled` | `features.deploy` 或 `deploy.driver = "none"` |
| `image_name` | `build.docker.image` |
| `build.platforms` 字符串 | `build.docker.platforms` 数组 |

## 第一版刻意不做的事情

为了避免 `ship` 复杂度失控，第一版建议**先不做**：

1. 通用条件表达式引擎
2. 任意 DAG 依赖图
3. 插件系统
4. 多 artifact 并行产出
5. 过度泛化的模板变量系统

## 实现优先级建议

1. **阶段一**：先把当前 image-first 扩成 pipeline-first  
   重点做 `steps.prepare`、`version.fallback`、`deploy.compose.tag_key`、`publish.registry`
2. **阶段二**：补 `go-binary` + `binary-install`  
   吃掉 `swag` 这类项目
3. **阶段三**：补 `templates`  
   吃掉 `chats` 这类“发布物料生成”项目

## 一句话总结

v2 的核心不是再给现有模型加几个 flag，而是把 `ship` 升级成：

**“阶段式配置 + 内置 driver + 少量 step hooks” 的通用发布编排器。**
