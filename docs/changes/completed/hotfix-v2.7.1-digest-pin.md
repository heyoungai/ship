# Hotfix v2.7.1 · digest pin 身份记错 / 比错

- Status: **fixed**（hotfix 线：`v2.7.0` → `v2.7.1`；已 cherry-pick 回 master）
- Date: 2026-07-24
- Branch: `hotfix/v2.7.1`
- Base tag: `v2.7.0`
- Release tag: `v2.7.1` → `8accc20`
- Hotfix commits: `ae0b394`（fix）、`8accc20`（docs）

> 本文供发布复盘与后续排查。主线归档位置：`docs/changes/completed/`。

## 现象

使用默认 `deploy.compose.pin = "digest"` 时，`ship deploy` / `ship run` 的 deploy 阶段可能失败，报远端 digest 与 release manifest 不一致，或网络抖动时硬失败。

表面上看像「腾讯云（或其他 registry）把同名 tag 覆盖了」。

## 误判

**不是** registry 静默覆盖正式 tag。

真正触发路径是：ship 的 digest pin 在 **buildx multi-arch index** 与 **远端 inspect 网络抖动** 下会：

1. **记错身份**：把本地 `docker image inspect` 的 config digest（`.Id`）写进 manifest，当作 pin。
2. **比错身份**：deploy 时再用远端 index / manifest digest 去比，两者本就不是同一类 digest。

config digest ≠ registry content digest（`image@sha256:...` 需要的是 manifest 或 index digest）。

## 根因

| 环节 | v2.7.0 行为 | 问题 |
|------|-------------|------|
| push 后写 manifest | 远端 `manifest inspect` 失败时，**静默回退**本地 config digest | 把不可 `@digest` 的身份当 pin |
| multi-arch | `manifest inspect` 得到 index 成员列表，或 invent `index:...` 聚合串 | 不是可拉取的 index digest |
| deploy 校验 | 字符串 / 首 token 硬比；网络失败在 `pin=digest` 时 hard-fail | 比错或误杀 |
| compose 实际拉取 | 只写了 `APP_IMAGE_DIGEST`，但 compose 仍用 `:tag` | 名义 digest、实际 tag，却仍按 digest 硬校验 |

一句话：digest 功能在 buildx index + 网络抖动下不完整，两次失败都是它触发的。

## 临时绕过（未升级前）

在 `ship.toml`：

```toml
[deploy.compose]
pin = "tag"
```

已用 v2.7.0 发布、manifest 可能已记错 digest 的版本：同样先 `pin = "tag"` 部署，或升级到 v2.7.1 后重新 `ship push` 写入正确 registry digest。

## 修复（v2.7.1）

1. **统一 pin 身份**：`ResolveRegistryPinDigest` 优先 `docker buildx imagetools inspect --format '{{.Digest}}'`，只接受可钉扎的 `sha256:...`。
2. **禁止静默回退**：远端拿不到 pin digest 时 **不写** local config digest；deploy 自动降级 `pin=tag` 并警告。
3. **比对**：`DigestsMatch` 支持相等、config/layer 兼容、以及 pin digest ∈ index/list 成员。
4. **网络 / 名义 digest**：有效 pin 为 tag（含自动降级）时，远端校验失败或 mismatch **只 warn**。
5. **compose 真用 @digest**：配置了 `image_key`，或本地 compose 含 `@${DIGEST_KEY}` 等；否则降级 tag 并警告。
6. **`ImageDigestRef` / `ResolveComposePin`**：拒绝 `index:...`、带 layer 的本地指纹等不可钉扎串。

涉及文件：`internal/registry_immutability.go`、`internal/artifact_store.go`、`cmd/push.go`、`cmd/deploy.go` 及对应测试。

## 正确用法（生产）

```yaml
# compose
image: registry.example.com/ns/app@${APP_IMAGE_DIGEST}
```

```toml
[deploy.compose]
pin = "digest"
digest_key = "APP_IMAGE_DIGEST"
# 或直接写完整引用：
# image_key = "APP_IMAGE"
```

若 compose 仍是 `image: ...:${APP_IMAGE_TAG}`，请显式 `pin = "tag"`，或改成 `@digest` 引用。

## 验证清单

- [ ] `git log v2.7.0..v2.7.1` 仅含本 hotfix 提交（无主干后续无关提交）
- [ ] `go test ./...` 通过
- [ ] 多 arch buildx 推送后，manifest 中 digest 为 imagetools 的 `sha256:...`（非本地 `.Id`）
- [ ] 远端 inspect 失败时 push 警告且不写入错误 digest；deploy 降级 tag 可继续
- [ ] compose 仅用 tag 时自动降级，不再因 digest mismatch hard-fail
- [ ] 用 `ship run -v v2.7.1`（或对应发布流程）确认 SourceRoot 来自新 tag

## 合回主干

已完成：在 master 上 `cherry-pick` hotfix 两笔提交；复盘文档归档到本路径，并更新 `docs/changes/README.md`。

## 后续（非本 hotfix 范围）

- 对「仅有本地 daemon、无 imagetools」环境给出更明确的 doctor 检查
- 旧 v2.7.0 manifest 的自动迁移 / 明确提示「请重新 push」
- 将 digest pin 契约写入 ADR 或 engineering 策略正文（与 `git-tag-release-strategy` 对齐）
- **Push 不可变误杀与 `ship run` 重试幂等**：deploy pin 已修，但 `EnsureRegistryTagImmutable` 仍可能用不同身份切片误判；见 [`../active/immutable-tag-retry.md`](../active/immutable-tag-retry.md)
