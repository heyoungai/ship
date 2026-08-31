# Ship examples

Read when you need a copy-paste recipe. Field details: [REFERENCE.md](REFERENCE.md).

## 1. Full Docker pipeline

```bash
ship init
# Edit ship.toml: registry + deploy.compose (pin = "digest")
ship plan -v v1.0.0
ship doctor -v v1.0.0
ship run -v v1.0.0 -y
ship rollback -v v1.0.0 -y   # when needed
```

Minimal compose pin snippet in `ship.toml`:

```toml
[deploy]
driver = "compose"

[deploy.compose]
host = "prod-server"
path = "/home/team/myapp"
local_file = "./deploy/compose.prod.yaml"
remote_file = "compose.yaml"
local_env_file = "./deploy/.env.prod"
env_file = ".env"
tag_key = "APP_IMAGE_TAG"
pin = "digest"
digest_key = "APP_IMAGE_DIGEST"
up = "docker compose --env-file ./.env up -d --remove-orphans"
```

Production compose image line:

```yaml
image: registry.example.com/ns/app@${APP_IMAGE_DIGEST}
```

## 2. Go binary

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

```bash
ship run -v v1.0.0 -y
```

## 3. Build only

```toml
[features]
deploy = false
publish = false
```

```bash
ship build -v v1.0.0 -y
# Or: ship run -v v1.0.0 --skip-deploy -y
```

### Skip base-image pull / registry HEAD (local mirror 429)

When the base image is already local and build stalls on registry HEAD or mirror rate limits:

```bash
ship build -v v1.0.0 --pull=false -y
# Or in ship.toml: [build.docker] pull = false
```

Keep default `pull = true` for clean CI agents and floating tags (e.g. `python:3.12`).

### Recover a failed release without rebuilding

When a run reports a checkpoint ID after a push/deploy failure, continue only that release:

```bash
ship run --resume 7f3a91 -y
```

Normal `ship run -v v1.0.0 -y` also reuses a single matching, digest-verified published run. It will not trust a registry tag that lacks the local checkpoint and manifest. Use `--restart` only to force a fresh full run.

### Retry transient registry or SSH failures

```toml
[retry]
max_attempts = 3
initial_delay_seconds = 2
max_delay_seconds = 20
```

This covers built-in registry, SCP, and SSH operations; it intentionally does not rerun hooks or arbitrary local commands.

### Push to Aliyun ACR (avoid OCI empty attestation)

Newer Docker/buildx may attach provenance that uses `application/vnd.oci.empty.v1+json`. ACR can reject the final manifest with `unknown manifest class`.

```toml
[build.docker]
provenance = false
sbom = false
```

Rebuild after changing. With `version.source = "git-tag"`, the release tag must include this `ship.toml`.

## 4. Matrix / single profile

```toml
[[matrix]]
name = "brand-a"
default = true
env = { NEXT_PUBLIC_APP_BRAND = "brand-a" }

[[matrix]]
name = "brand-b"
env = { NEXT_PUBLIC_APP_BRAND = "brand-b" }
```

```bash
ship run -p brand-a -v v1.0.0 -y
```
