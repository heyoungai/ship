# Ship 配置参考

编辑 `ship.toml` 时再读本文件。优先用表格与最短片段。

## 必填

- `schema = 2`
- `build.driver` / `publish.driver`
- `build.driver = "docker"` 时需要 `build.docker.image`
- `publish.driver = "registry"` 时至少一个 `publish.registry.targets`
- 启用 compose 部署时需要 `deploy.compose.host` / `path`

## 版本

```toml
[version]
source = "git-tag"          # git-tag | env | static
fallback = "error"          # error | dev | static
override_env = "SHIP_VERSION"
```

优先级：`SHIP_VERSION` > `version.source` > `version.fallback`。

## features / vars / hooks / templates

```toml
[features]
publish = true
deploy = true
rollback = true
verify = true

[vars]
app_name = "myapp"
remote_tag_key = "APP_IMAGE_TAG"

[[steps.prepare]]
name = "build-frontend"
run = "bun run build"
cwd = "."
profiles = ["*"]
# 另有：post_build, pre_publish, post_publish, pre_deploy, post_deploy

[[templates]]
path = "./deploy/.env"
profiles = ["*"]
content = "APP_IMAGE_TAG={{ version }}\n"
# 或：from = "./templates/file.tpl"（与 content 互斥）
```

模板内置变量：`version`、`module`、`image_name`、`project.*`、`profile.*`、`build.*`、`publish.driver`、`deploy.*`、`verify.driver`。自定义：`vars.<key>`、`env.<key>`（matrix）。

## Build 驱动

| Driver | 区块 |
|--------|------|
| `docker` | `[build.docker]` — `image`、`platforms`（单平台）、`dockerfile`、`env_file`、`load=true` |
| `go-binary` | `[build.go]` — `main`、`output`、`goos`、`goarch`、`ldflags` |
| `command` | `[build.command]` — `run`、`cwd`、`outputs` |

## Publish 驱动

| Driver | 说明 |
|--------|------|
| `registry` | `[[publish.registry.targets]]`（`type`、`url`、`namespace`、`image`）；`tag_latest_on_default_profile` |
| `scp` | `[publish.scp]` — `local`、`host`、`remote` |
| `none` | 不发布 |

## Deploy 驱动

| Driver | 说明 |
|--------|------|
| `compose` | SSH + 远端 compose；见下方 pin 字段 |
| `binary-install` | `[deploy.binary_install]` — host、安装路径 |
| `ssh` | `[deploy.ssh]` — `host`、`commands` |

### compose pin / env

| 字段 | 默认 | 含义 |
|------|------|------|
| `env_file` | `.env` | 远端 env 文件名 |
| `local_env_file` | — | 部署前上传 |
| `local_file` | — | 上传本地 compose |
| `auto_env_file` | `true` | 非默认 env 时注入 `--env-file` |
| `tag_key` | `APP_IMAGE_TAG` | env 中版本别名 |
| `pin` | `digest` | `digest` \| `tag` |
| `digest_key` | `APP_IMAGE_DIGEST` | `pin=digest` 时的内容身份 |
| `image_key` | — | 可选写入 `repo@digest` |

## Verify 驱动

`http`（`url`、`expected_status`、attempts/interval/timeout）· `ssh` · `command`。旧 `[deploy.healthcheck]` 仍兼容；新项目用 `[verify]`。

## Matrix

```toml
[[matrix]]
name = "brand-a"
default = true
env = { NEXT_PUBLIC_APP_BRAND = "brand-a" }
vars = { brand = "brand-a" }

[[matrix]]
name = "brand-b"
env = { NEXT_PUBLIC_APP_BRAND = "brand-b" }
vars = { brand = "brand-b" }
```

`-p brand-a` 选单个 profile；不传 `-p` 则全部。非 default 的 tag 带后缀（`v1.0.0-brand-b`）。具名 default profile **无**后缀。

## 环境变量覆盖

| 变量 | 覆盖目标 |
|------|----------|
| `IMAGE_NAME` | `build.docker.image` |
| `PLATFORMS` | `build.docker.platforms` |
| `DOCKERFILE` | `build.docker.dockerfile` |
| `ENV_FILE` | `build.docker.env_file` |
| `REMOTE_HOST` | `deploy.compose.host` |
| `REMOTE_PROJECT_PATH` | `deploy.compose.path` |
| `SHIP_VERSION` | 发布版本（`git-tag` 模式下须为真实 tag） |
| `SHIP_UNKNOWN_KEYS` | `error` \| `warn` \| `ignore` |
