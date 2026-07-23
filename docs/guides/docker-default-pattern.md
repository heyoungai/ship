# Docker 项目默认推荐范式

这份文档定义 ship 在 Docker 项目上的默认推荐模式。

目标不是覆盖所有项目，而是给绝大多数项目一个统一、稳定、低心智负担的接入模板。

## 1. 适用范围

适用于这类项目：

- 项目最终产物是 Docker 镜像
- 镜像发布到 Docker Registry
- 目标环境通过 SSH 登录
- 服务通过 Docker Compose 更新
- 部署后最好有 HTTP 健康检查

典型链路：

```text
build -> tag -> push -> deploy -> verify
```

## 2. 默认推荐模式

对新的 Docker 项目，默认推荐直接采用下面这套组合：

- `build.driver = "docker"`
- `publish.driver = "registry"`
- `deploy.driver = "compose"`
- `verify.driver = "http"`

默认推荐原则：

- 默认不写 `steps.*`
- 默认不写 `templates`
- 默认单平台 `linux/amd64`
- 默认使用 `run` 作为正式发布入口

也就是说，新项目应该先用内置 driver 跑通主链路，再决定是否需要 escape hatch。

## 3. 标准配置骨架

```toml
schema = 2

[project]
name = "myapp"

[version]
source = "git-tag"
fallback = "dev"
override_env = "SHIP_VERSION"

[features]
publish = true
deploy = true
rollback = true
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
latest_on_default_profile = true

[publish]
driver = "registry"

[publish.registry]
push = true
tag_latest_on_default_profile = true

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

推荐把远端 `compose.yaml` 和 `.env` 都当成 ship 可同步的发布资产：首次部署时由 ship 自动创建目录并上传，后续部署继续复用同一套入口更新 tag 并执行 compose。

## 4. 默认操作方式

### 本地调试

先按阶段拆开执行：

```powershell
.\ship.exe build -v v1.0.0
.\ship.exe tag -v v1.0.0
.\ship.exe push -v v1.0.0
.\ship.exe deploy -v v1.0.0
```

### 正式发布

优先使用：

```powershell
.\ship.exe run -v v1.0.0 --yes
```

### 在 CI 中发布

推荐注入环境变量后直接跑：

```powershell
$env:SHIP_VERSION = "v1.0.0"
.\ship.exe run --yes
```

## 5. 什么时候才使用 hooks

只有下面这些情况，才建议打开 `steps.*`：

- Docker build 之前必须先执行前端打包
- Docker build 之前必须先生成 dist 目录
- 发布前后有轻量命令必须执行

推荐方式：

- 优先只用 `steps.prepare`
- 只有确实需要时才加 `pre_publish` / `post_publish` / `pre_deploy` / `post_deploy`

不推荐：

- 从一开始就把完整发布链路写成一堆 hooks
- 用 hooks 替代 build / publish / deploy driver

## 6. 什么时候才使用 templates

只有下面这些情况，才建议使用 `templates`：

- 发布前要生成 `.env`
- 发布前要生成运行时 Dockerfile
- 发布前要生成 compose 文件

推荐方式：

- 只生成最终发布物料
- 只做轻量变量插值

不推荐：

- 用模板承载复杂业务逻辑
- 同时叠加大量模板和 hooks 让流程变得不可读

## 7. 变量覆盖建议

为了保持直觉性，推荐按下面顺序使用变量：

1. 平时把稳定配置写进 `ship.toml`
2. 发布版本通过 `-v` 或 `SHIP_VERSION` 覆盖
3. 只对少量字段用环境变量临时覆盖

推荐常见方式：

- 版本：`-v` 或 `SHIP_VERSION`
- Docker 字段：`IMAGE_NAME`、`DOCKERFILE`、`ENV_FILE`
- Compose 部署目标：`REMOTE_HOST`、`REMOTE_PROJECT_PATH`

原则是：

- 默认值落配置
- 临时值走覆盖
- 不要让项目依赖大量外部环境变量才能理解发布语义

## 8. env_file 配置策略

`deploy.compose` 下有三个与环境文件相关的字段，推荐按下面方式理解：

- `env_file`：远端 env 文件名，ship 会往里面写入镜像 tag，docker compose 也从它读取环境变量。默认 `.env`。
- `local_env_file`：本地 env 文件路径，首次部署时通过 scp 上传到远端。后续部署 ship 只更新其中的 tag，不再重复上传。
- `auto_env_file`：当 `env_file` 不是 `.env` 时，自动将 `--env-file` 注入 `up` 命令。默认 `true`。

典型用法：

```toml
[deploy.compose]
env_file = ".env.prod"                     # 远端使用 .env.prod
local_env_file = "./deploy/.env.prod"      # 首次部署上传本地文件
auto_env_file = true                       # 自动注入 --env-file ./.env.prod
up = "docker compose up -d --remove-orphans"  # 无需手写 --env-file
```

当 `auto_env_file = true` 时，ship 会在执行前自动将 `up` 改写为 `docker compose --env-file ./.env.prod up -d --remove-orphans`。

如果你的 `up` 命令已经显式包含 `--env-file`，ship 不会重复注入。如果你希望完全手动控制 `up` 命令，设置 `auto_env_file = false` 即可。

## 9. 和 Taskfile 的推荐关系

如果项目已经有 Taskfile，推荐这样分工：

- Taskfile 负责开发者日常命令
- ship 负责发布链路

一个典型例子：

```yaml
tasks:
  release:
    cmds:
      - ship run --yes
```

这样做的好处是：

- 开发入口统一
- 发布语义仍由 ship 维护
- 不会把 ship 变成另一个 task runner

## 10. 默认范式的核心约束

后续所有 Docker 项目接入 ship，都应优先遵守：

1. 先采用 `docker / registry / compose / http verify`
2. 先跑通主链路，再考虑例外情况
3. hooks 和 templates 只作为逃生口
4. 如果一个项目离默认范式太远，先判断它是不是 ship 的适用对象

## 11. 一句话总结

ship 在 Docker 项目上的默认推荐模式应该是：

**先用少量稳定 driver 表达发布意图，只有主链路覆盖不了时，才用 hooks 和 templates 补充。**