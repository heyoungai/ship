# Ship config reference

Read this when editing `ship.toml`. Prefer tables and shortest snippets.

## Required

- `schema = 2`
- `build.driver` / `publish.driver`
- `build.driver = "docker"` requires `build.docker.image`
- `publish.driver = "registry"` requires at least one `publish.registry.targets`
- Compose deploy requires `deploy.compose.host` / `path`

## Version

```toml
[version]
source = "git-tag"          # git-tag | env | static
fallback = "error"          # error | dev | static
override_env = "SHIP_VERSION"
```

Precedence: `SHIP_VERSION` > `version.source` > `version.fallback`.

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
# Also: post_build, pre_publish, post_publish, pre_deploy, post_deploy

[[templates]]
path = "./deploy/.env"
profiles = ["*"]
content = "APP_IMAGE_TAG={{ version }}\n"
# Or: from = "./templates/file.tpl" (mutually exclusive with content)
```

Built-in template vars: `version`, `module`, `image_name`, `project.*`, `profile.*`, `build.*`, `publish.driver`, `deploy.*`, `verify.driver`. Custom: `vars.<key>`, `env.<key>` (matrix).

## Build drivers

| Driver | Block |
|--------|-------|
| `docker` | `[build.docker]` — `image`, `platforms` (single), `dockerfile`, `env_file`, `load=true` |
| `go-binary` | `[build.go]` — `main`, `output`, `goos`, `goarch`, `ldflags` |
| `command` | `[build.command]` — `run`, `cwd`, `outputs` |

## Publish drivers

| Driver | Notes |
|--------|-------|
| `registry` | `[[publish.registry.targets]]` (`type`, `url`, `namespace`, `image`); `tag_latest_on_default_profile` |
| `scp` | `[publish.scp]` — `local`, `host`, `remote` |
| `none` | No publish |

## Deploy drivers

| Driver | Notes |
|--------|-------|
| `compose` | SSH + remote compose; see pin fields below |
| `binary-install` | `[deploy.binary_install]` — host, install path |
| `ssh` | `[deploy.ssh]` — `host`, `commands` |

### compose pin / env

| Field | Default | Meaning |
|-------|---------|---------|
| `env_file` | `.env` | Remote env filename |
| `local_env_file` | — | Upload before deploy |
| `local_file` | — | Upload local compose file |
| `auto_env_file` | `true` | Inject `--env-file` when env is non-default |
| `tag_key` | `APP_IMAGE_TAG` | Version alias in env |
| `pin` | `digest` | `digest` \| `tag` |
| `digest_key` | `APP_IMAGE_DIGEST` | Content identity when `pin=digest` |
| `image_key` | — | Optional `repo@digest` write |

## Verify drivers

`http` (`url`, `expected_status`, attempts/interval/timeout) · `ssh` · `command`. Legacy `[deploy.healthcheck]` still works; new projects should use `[verify]`.

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

`-p brand-a` selects one profile; omit `-p` for all. Non-default tags get a suffix (`v1.0.0-brand-b`). Named default profiles have **no** suffix.

## Env overrides

| Variable | Overrides |
|----------|-----------|
| `IMAGE_NAME` | `build.docker.image` |
| `PLATFORMS` | `build.docker.platforms` |
| `DOCKERFILE` | `build.docker.dockerfile` |
| `ENV_FILE` | `build.docker.env_file` |
| `REMOTE_HOST` | `deploy.compose.host` |
| `REMOTE_PROJECT_PATH` | `deploy.compose.path` |
| `SHIP_VERSION` | Release version (must be a real tag in `git-tag` mode) |
| `SHIP_UNKNOWN_KEYS` | `error` \| `warn` \| `ignore` |
