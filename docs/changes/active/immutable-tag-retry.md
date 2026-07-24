# Change Plan · 正式 tag 不可变校验与 ship run 重试

- Status: **planned**（未开始编码）
- Date: 2026-07-24
- Owner: TBD
- Related:
  - Deploy 侧 digest pin：[`hotfix-v2.7.1-digest-pin.md`](../completed/hotfix-v2.7.1-digest-pin.md)（已修）
  - 外部输入 sync：[`deploy-sync-and-external-inputs.md`](./deploy-sync-and-external-inputs.md)

> 规划文档，不是实现规范。稳定后可把「正式 tag 幂等语义」写入 [engineering/git-tag-release-strategy.md](../../engineering/git-tag-release-strategy.md) 或 ADR。

## 背景

消费方（xingwei-car-mods）典型失败链：

1. `ship run -v v0.2.3`：build / tag / **push 已成功**，deploy 因服务器防火墙等失败。
2. 修好网络后再次 `ship run -v v0.2.3`。
3. 在 push 阶段硬失败：

```text
拒绝覆盖远端版本 …:v0.2.3：已有 digest index:sha256:09a3df…，
本地 digest sha256:091948de…；请发布新版本而不是覆盖正式 tag
```

用户预期：正式 tag 不可变没问题，但 **重试同一版本应幂等**——若远端已有等价（或可确认可用）镜像，应跳过 push 继续 deploy，而不是整条流水线报错停住。

当前临时正确操作是改跑：

```bash
ship deploy -v v0.2.3 -y
```

但 `ship run` 没有给出这一提示，也没有「从失败阶段续跑」能力，体感「不够智能」。

## 设计意图（已有）

`EnsureRegistryTagImmutable` 的契约本应是：

| 远端同名 tag | 行为 |
|--------------|------|
| 不存在 | 允许 push |
| 存在且内容等价 | **跳过 push**，流水线继续（幂等） |
| 存在且内容不同 | **拒绝覆盖**，要求新版本号 |

即：**不可变 ≠ 重试必失败**；不可变只禁止「用不同内容覆盖同一正式 tag」。

## 问题拆解

### 1. 等价判定在 push 路径仍不稳（主因）

v2.7.1 已把 **deploy pin** 统一到 `ResolveRegistryPinDigest` / `DigestsMatch`，但 **push 前不可变检查**仍是：

- 本地：`docker image inspect` 的 config Id + layers（`localImageDigest`）
- 远端：`docker manifest inspect` 解析出的指纹（常为 `index:成员1,成员2`）

buildx `--load` 且带 attestation 时，同一次构建会同时出现：

- platform **manifest** digest（如 `09a3df…`）
- **manifest list / index** digest（如 `091948de…`）
- **config** digest（如 `c67491…`）

错误信息里的「本地 digest」与「远端 index 成员」经常是 **同一构建的不同身份切片**。`DigestsMatch` 虽支持「pin ∈ index 成员」，但若一侧是 **list digest**、另一侧是 **成员列表（不含 list 自身）**，仍会判成不等 → 误报「拒绝覆盖」。

用户安装的二进制若仍是 **v2.7.0**，deploy 侧旧 bug 与 push 侧该问题会叠加；即使升到含 v2.7.1 的 master，**本 change 所述 push/重试缺口仍在**，需单独修。

### 2. 失败体验缺少「下一步」

硬失败文案只要求「请发布新版本」，没有区分：

- A. 远端已是本次要发的内容 → 应 `ship deploy -v VER`（或自动跳过 push）
- B. 远端是旧内容、本地是新构建 → 必须新 tag，不能静默用远端

无脑「有 tag 就拿远端去部署」在 B 情况下会 **以为发了新版、实际仍是旧镜像**，比报错更危险。

### 3. 缺少阶段续跑

`ship run` = build → tag → publish → deploy → verify。publish 已成功、deploy 失败后，再次 `ship run` 会重做 build/tag/push。即使等价判定修好，仍浪费时间；判定未修好则直接卡死。

## 目标

1. **Push 幂等可靠**：同一正式 tag、同一发布内容再次 `ship run` / `ship push` 时，跳过 push 并继续后续阶段（至少不误杀）。
2. **冲突可操作**：真正内容冲突时，错误信息区分场景，并提示 `ship deploy -v VER` vs 打新 tag。
3. **（可选）阶段续跑**：支持从 publish/deploy 续跑，避免「只差部署」时整链重来。

## 非目标

- 不取消正式 tag 不可变策略（覆盖不同内容仍应拒绝）
- 不默认「远端有 tag 就静默部署远端」（防止旧镜像冒充新发布）
- 不在本 change 重做 deploy pin（已见 v2.7.1）；不在本 change 做 sync / InvocationRoot（见 sibling plan）
- 不解决 registry 网络 / 防火墙本身（仅改善 ship 控制流与提示）

## 建议方案

### A. 统一 push 不可变比较的身份（P0）

`EnsureRegistryTagImmutable` 与 deploy 对齐，优先比较 **registry content digest**：

1. 对 `remoteRef`：`ResolveRegistryPinDigest`（imagetools 的 index/manifest digest）。
2. 对本地已 tag 的 `remoteRef` 或即将 push 的引用：在可能时同样取 registry 视角 digest；若仅有本地 daemon，再用 config/layer 指纹，并与远端 index **成员 / config** 做 `DigestsMatch`。
3. 明确：`manifest list digest` 与「解析自同一 list 的成员集合」在文档与测试中定义为等价（或比较前先规范化到 pin digest）。

**验收：**

- 重现「push 成功 → deploy 失败 → 再 ship run」：第二次跳过 push，进入 deploy。
- 故意改 Dockerfile 再 push 同 tag：仍拒绝覆盖。
- 单测覆盖：list digest vs `index:成员…`、config ∈ 成员、真正不同内容。

### B. 冲突时的分级提示（P0，可先于 A 落地）

拒绝覆盖时打印结构化建议，例如：

```text
远端已存在正式 tag v0.2.3，且与本地指纹不一致（或无法证明一致）。

若上次 push 已成功、仅 deploy 失败，且确认远端镜像可用：
  ship deploy -v v0.2.3 -y

若本地包含尚未发布的变更：
  请打新 tag 后重新 ship run（不要覆盖正式 tag）
```

可选：`ship doctor -v VER` 增加「远端 tag 是否已存在 / 本地 release manifest 是否已 published」。

### C. `ship run` 阶段续跑（P1）

示意：

| 机制 | 说明 |
|------|------|
| `ship run -v VER --from deploy` | 跳过 build/tag/push，要求 release manifest 已有 published image |
| `ship run -v VER --resume` | 读取 `.ship/runs/<run_id>` 或最近失败阶段，从下一阶段继续 |
| publish 幂等跳过 | A 落地后，全量 `ship run` 重试也能自然通过 |

**验收：** push 已成功的版本，`--from deploy` 不再触碰 registry 写路径。

### D. 与「静默用远端」的边界（产品规则）

写入策略文档，避免实现时摇摆：

1. **可自动跳过 push**：仅当等价判定为真。
2. **不可自动改用「仅远端、忽略本地构建」**：除非用户显式 `ship deploy`（表示消费已发布产物）或未来的 `--use-remote` 之类显式 flag。
3. `ship run` 表示「从本机发布会话产出并推进」；`ship deploy` 表示「消费已记录/已发布的 release」。

## 分阶段

| Phase | 内容 | 优先级 |
|-------|------|--------|
| 0 | 本文档 + quick-start / 失败手册中补充「push 已成功则 ship deploy」 | P0 |
| 1 | 冲突提示文案（B） | P0 |
| 2 | 统一 push 等价判定（A）+ 测试 | P0 |
| 3 | `--from` / `--resume`（C） | P1 |

## 与现有文档的关系

| 文档 | 关系 |
|------|------|
| [hotfix-v2.7.1-digest-pin.md](../completed/hotfix-v2.7.1-digest-pin.md) | 修的是 **deploy 钉扎身份**；本 change 管 **push 不可变 + run 重试** |
| [git-tag-release-strategy.md](../../engineering/git-tag-release-strategy.md) | 正式 tag 不可变；本 change 补「幂等重试」细则 |
| [deploy-sync-and-external-inputs.md](./deploy-sync-and-external-inputs.md) | 正交；都来自同一消费方故障周，但问题域不同 |

## 风险

| 风险 | 缓解 |
|------|------|
| 放宽等价判定导致误跳过 push、旧镜像留在 tag 上 | 单测 + 优先 imagetools pin；不确定则 fail 并提示 deploy/新 tag |
| `--from deploy` 在无 published artifact 时误用 | `RequireReleaseManifest` + `HasPublishedImage` |
| 用户以为升级 v2.7.1 已包含本修复 | 文档写明范围；release note 分开列 |

## 完成定义

1. 同版本「push 成功、deploy 失败」后再次 `ship run`，在内容未变时不再因不可变检查误杀。
2. 真冲突时用户能从错误信息直接选对：`ship deploy` 或新 tag。
3. 工程文档写清：不可变、幂等、`ship run` vs `ship deploy` 的分工。
4. 有回归测试覆盖 list/index/config 指纹组合。

## 消费方临时绕过（文档备查）

在 ship 本 change 落地前：

```bash
# 镜像已在 registry 时
ship deploy -v <version> -y

# 必须重新构建推送时
git tag vX.Y.Z   # 新版本
ship run -v vX.Y.Z -y
```

建议消费方安装 **≥ v2.7.1**（或当前已含 digest hotfix 的构建），以避免 deploy 侧旧 pin 问题与本问题混淆。
