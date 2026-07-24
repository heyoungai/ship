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
