# Ship 示例

需要可复制配方时再读。字段细节见 [REFERENCE.md](REFERENCE.md)。

## 1. Docker 全流程

```bash
ship init
# 编辑 ship.toml：registry + deploy.compose（pin = "digest"）
ship plan -v v1.0.0
ship doctor -v v1.0.0
ship run -v v1.0.0 -y
ship rollback -v v1.0.0 -y   # 需要时
```

`ship.toml` 中 compose pin 最小片段：

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

生产 compose 镜像行：

```yaml
image: registry.example.com/ns/app@${APP_IMAGE_DIGEST}
```

## 2. Go 二进制

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

## 3. 仅构建

```toml
[features]
deploy = false
publish = false
```

```bash
ship build -v v1.0.0 -y
# 或：ship run -v v1.0.0 --skip-deploy -y
```

## 4. Matrix / 单个 profile

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
