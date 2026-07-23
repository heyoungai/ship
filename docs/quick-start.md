# ship 快速上手

这份说明只讲最短路径，目标是让用户在 5 分钟内把 ship 跑起来。

如果你要在团队里长期推广 ship，建议再一起阅读：

- [design-principles.md](design-principles.md)
- [docker-default-pattern.md](docker-default-pattern.md)

## 1. ship 是什么

ship 是一个阶段式发布工具，当前已经可以覆盖两类常见场景：

- Docker 项目：构建镜像、推送镜像、远程 compose 部署、部署后校验
- Go 二进制项目：编译二进制、SCP 上传、远程安装、部署后校验

它不是把 CI/CD 写死在代码里，而是把流程放到 ship.toml 里配置，所以现在已经可以做比较灵活的 CI/CD。

## 2. 最短使用流程

先在仓库根目录编译 ship：

```powershell
go build -o .\ship.exe .
```

然后初始化配置：

```powershell
.\ship.exe init
```

接着按你的项目类型修改 ship.toml。

最常用的命令只有这几个：

```powershell
.\ship.exe plan -v v1.0.0
.\ship.exe doctor -v v1.0.0
.\ship.exe build -v v1.0.0
.\ship.exe push -v v1.0.0
.\ship.exe deploy -v v1.0.0
.\ship.exe run -v v1.0.0
```

默认 `version.source = "git-tag"` 时，`-v` 必须是本地真实 Git tag；构建来自该 tag 的源码快照。`deploy` / 独立 `push` 消费 `.ship/releases/` 中的 release manifest，不会从当前目录偷偷补构建。

如果在 CI 里跑，建议加上：

```powershell
.\ship.exe run -v v1.0.0 --yes
```

## 3. Docker 项目最小配置

适合前后端、Next.js、Python、Node、Java 等镜像部署项目。

```toml
schema = 2

[project]
name = "myapp"

[features]
publish = true
deploy = true
verify = true

[build]
driver = "docker"

[build.docker]
image = "myapp"
context = "."
dockerfile = "./Dockerfile"
platforms = ["linux/amd64"]
env_file = "./.env.local"
build_args_from_env = true
load = true

[publish]
driver = "registry"

[[publish.registry.targets]]
type = "private"
url = "registry.example.com"
namespace = "team"
image = "myapp"

[deploy]
driver = "compose"

[deploy.compose]
host = "prod-server"
path = "/home/team/myapp"
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
url = "https://example.com/health"
expected_status = 200
attempts = 20
interval_seconds = 3
timeout_seconds = 5
```

如果你希望首次部署时由 ship 自动把本地部署资产带上去，直接使用 `deploy.compose.local_file` 和 `deploy.compose.local_env_file`。ship 会先在远端创建 `deploy.compose.path`，上传文件，再更新 `env_file` 中的镜像 tag 并执行 `docker compose up`。

如果你的远端 env 文件不是默认的 `.env`（比如 `.env.prod`），只需修改 `env_file` 字段，ship 会自动将 `--env-file` 注入到 `up` 命令中（由 `auto_env_file = true` 控制，这是默认行为）。

最常用执行方式：

```powershell
.\ship.exe run -v v1.0.0 --yes
```

这条命令会按下面顺序执行：

```text
prepare -> templates -> build -> tag -> publish -> pre_deploy -> deploy -> post_deploy -> verify
```

## 4. Go 二进制项目最小配置

适合单个 Go 服务、CLI、守护进程发布。

```toml
schema = 2

[project]
name = "mycli"

[features]
publish = true
deploy = true
verify = true

[build]
driver = "go-binary"

[build.go]
main = "./cmd/mycli"
output = "./build/mycli"
goos = "linux"
goarch = "amd64"
cgo_enabled = false
ldflags = [
  "-s",
  "-w",
  "-X {{ module }}/internal/cli.Version={{ version }}"
]

[publish]
driver = "scp"

[publish.scp]
local = "./build/mycli"
host = "prod-server"
remote = "/tmp/mycli"

[deploy]
driver = "binary-install"

[deploy.binary_install]
host = "prod-server"
remote_temp_path = "/tmp/mycli"
remote_install_path = "/usr/local/bin"
use_ssh_tty = true
sudo_nopasswd = false
chmod = "+x"

[verify]
driver = "ssh"

[verify.ssh]
host = "prod-server"
command = "mycli --version"
```

最常用执行方式：

```powershell
.\ship.exe run -v v1.0.0 --yes
```

## 5. 想要灵活 CI/CD，重点用这几个能力

### 版本控制

- 可以直接传 `-v v1.0.0`
- 也可以在配置里用 `version.source`、`version.fallback`
- 也可以在 CI 里设置 `SHIP_VERSION`

例如：

```powershell
$env:SHIP_VERSION = "v1.2.3"
.\ship.exe run --yes
```

### 阶段 hooks

如果你的项目不是“直接 docker build”，可以在阶段前后插命令：

```toml
[[steps.prepare]]
name = "build-frontend"
run = "pnpm run build"
cwd = "./web"
profiles = ["*"]
```

这适合：

- 前端先打包
- 后端先 publish
- 发布前生成配置文件
- 部署后做清理动作

### 模板渲染

如果发布前要生成 .env、Dockerfile、compose 文件，可以用 templates：

```toml
[[templates]]
path = "./deploy/.env"
content = "APP_IMAGE_TAG={{ version }}"
```

### 多 profile / 多品牌

如果一个仓库要打多套配置，可以用 matrix：

```toml
[[matrix]]
name = "brand-a"
default = true
env = { APP_BRAND = "brand-a" }
vars = { brand = "brand-a" }

[[matrix]]
name = "brand-b"
env = { APP_BRAND = "brand-b" }
vars = { brand = "brand-b" }
```

执行时：

```powershell
.\ship.exe run -p brand-a -v v1.0.0 --yes
```

## 6. 推荐的 CI 用法

CI 里一般只需要这几条原则：

- 显式传版本，或者设置 `SHIP_VERSION`
- 使用 `--yes`，避免交互确认阻塞流水线
- Docker 项目保持单平台 `linux/amd64`
- 把 SSH 登录能力和 registry 凭据交给 CI 环境管理

一个典型命令就是：

```powershell
.\ship.exe run --yes
```

前提是：

- `SHIP_VERSION` 已注入
- `ship.toml` 已配置好 registry、host、path、verify

## 7. 常见问题

### 什么时候用 build / push / deploy，什么时候直接用 run？

- 本地排查某一个阶段，用单独命令
- CI/CD 正式发布，优先用 `run`

### verify 和 deploy.healthcheck 用哪个？

- 新项目优先用 `[verify]`
- 老项目如果还在用 `[deploy.healthcheck]`，当前仍兼容

### 为什么 Docker 项目建议单平台？

因为当前默认流程是 `build -> tag -> push`，中间依赖 `docker buildx --load` 把镜像先装回本地 Docker。

## 8. 最后建议

第一次接入时，不要一上来就把所有 hooks、templates、matrix 全开。

推荐顺序：

1. 先让最小配置跑通 `build`
2. 再跑通 `push`
3. 再配置 `deploy`
4. 最后补 `verify`、`steps`、`templates`

这样定位问题最快。

对大多数 Docker 项目，推荐直接采用 [docker-default-pattern.md](docker-default-pattern.md) 里的默认范式，不要先从 hooks 和 templates 开始。