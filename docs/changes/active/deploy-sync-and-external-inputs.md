# Change Plan · deploy sync 与外部输入路径

- Status: **planned**（未开始编码）
- Date: 2026-07-24
- Owner: TBD
- Related:
  - Digest pin 已修：[`hotfix-v2.7.1-digest-pin.md`](../completed/hotfix-v2.7.1-digest-pin.md)
  - 消费方临时绕过（xingwei-car-mods）：项目内 `docs/changes/active/deploy-sync-reliability.md` 轨 A

> 规划文档，不是实现规范。稳定后可把「外部输入 vs SourceRoot」边界写入 [product-design.md](../../product-design.md) 或 engineering 策略。

## 背景

`git-tag` 模式下 release recipe 与版本化物料来自 **SourceRoot**（tag 快照）；`.env` 等已按 **InvocationRoot** 处理。但自定义 `[[steps.pre_deploy]]` 默认在 SourceRoot cwd 执行相对路径命令。

消费项目常见做法是把 SQLite / `media/` 等 **gitignore 外部输入** 塞进 `pre_deploy` 的 `scp -r`：

1. 快照里没有这些目录 → `No such file or directory`，或只传上 `.gitkeep`
2. 全量 `scp -r` 无增量、遇权限错误整步失败（如容器生成的缩略图属主 ≠ SSH 用户）
3. 每次 deploy 重复传大目录，拖垮发布稳定性

xingwei-car-mods 已用 **方案 2** 临时绕过：默认 deploy 不同步 data/media，按需在工作区手动 scp/rsync；`pin = "tag"` 规避旧 digest 问题（升级 ship ≥ v2.7.1 后可再评估回 digest）。

本 change 要在 **ship 工具侧** 提供一等能力，替代「裸 shell scp + 误绑 SourceRoot」。

**不在本 change 范围：** digest 记错/比错身份（已由 v2.7.1 hotfix 覆盖）。若仍有 doctor / 旧 manifest 迁移缺口，见该复盘「后续」小节，可另开小单或并入本计划 Phase 0 检查项。

## 目标

1. **路径语义清晰**：步骤/同步目标可声明物料来自 `source`（版本化）或 `invocation`（工作区外部输入）。
2. **一等 sync**：增量同步（优先 rsync），支持 exclude、`when`、`on_error`；默认不必绑在每次 deploy。
3. **deploy 默认稳**：镜像 + 版本化配置可发版；大数据目录按需同步，失败策略可配置。
4. **Doctor**：提示本机无 rsync、远端不可写、sync 路径在 SourceRoot 不存在等。

## 非目标

- 不做成通用备份/网盘产品
- 不强制所有用户安装 rsync（可退化 scp，并明确能力差异）
- 不在本 change 重做 digest pin 核心逻辑（已 hotfix）
- 不解决远端容器 UID 与 SSH 用户不一致的全部运维问题（可文档 + 可选前置 hook）

## 建议能力面

### A. Step 锚定根目录（最小增量）

扩展 `[[steps.*]]`：

```toml
[[steps.pre_deploy]]
name = "upload media"
root = "invocation"   # source | invocation；默认 source（保持兼容）
run = "scp -r ./api/media/filer_public {{ vars.deploy_host }}:{{ vars.deploy_path }}/media/"
```

或渲染变量：`{{ roots.invocation }}` / `{{ roots.source }}`，令 `run` 可写绝对路径而不改 cwd 语义。

**验收：** gitignore 的 `api/media` 在 `root = "invocation"` 下可被 scp 找到；默认 `root` 行为与现网一致。

### B. 一等 `deploy.sync`（推荐主路径）

```toml
[[deploy.sync]]
name = "media"
local = "./api/media/"
remote = "{{ deploy.compose.host }}:{{ deploy.compose.path }}/media/"
root = "invocation"
mode = "incremental"                 # rsync -az；无 rsync 时退化或跳过并警告
exclude = ["filer_public_thumbnails/"]
when = "explicit"                    # always | explicit
on_error = "fail"                    # fail | warn
```

CLI 示意：

| 命令 | 说明 |
|------|------|
| `ship sync [-v VER]` | 执行 `when = always` 与（若 flag 打开）explicit 项 |
| `ship run` / `ship deploy` | 仅自动跑 `when = always`；`--sync` 打开 explicit |
| `ship doctor` | 检查 rsync、路径存在性、可选远端可写探测 |

**验收：**

- 默认 `ship deploy` 不同步 `when = explicit` 的 media
- `ship deploy --sync` 或 `ship sync` 从 InvocationRoot 增量同步，exclude 生效
- 无 rsync：明确警告 + 文档化退化策略（全量 scp 或拒绝 incremental）

### C. Doctor 增强（可与 A/B 同发或稍后）

- 本机是否有 `rsync` / `scp`
- `deploy.sync` / `root=invocation` 路径在 InvocationRoot 是否存在
- 配置了 `pin=digest` 但 compose 未使用 `@digest` 时的提示（与 v2.7.1 降级行为呼应）
- 可选：registry manifest 可达性（代理/fake-ip 导致的超时）

## 分阶段建议

| Phase | 内容 | 优先级 |
|-------|------|--------|
| 0 | 文档：在 quick-start / git-tag 策略中写明「外部输入 ≠ SourceRoot」；推荐默认不同步 gitignore 大目录 | P0 |
| 1 | `steps.root` 或 `{{ roots.invocation }}` | P0 |
| 2 | `[[deploy.sync]]` + `ship sync` / `--sync` | P1 |
| 3 | Doctor 与 Windows 退化路径打磨 | P2 |

## 与现有设计的关系

- [git-tag-release-strategy.md](../../engineering/git-tag-release-strategy.md)：SourceRoot / InvocationRoot / StateRoot 三分法；本 change 补齐「步骤与 sync 显式选根」。
- [hotfix-v2.7.1-digest-pin.md](../completed/hotfix-v2.7.1-digest-pin.md)：镜像身份钉死；本 change 管**内容型目录**同步，不混进 artifact digest。
- [ship-remote-environments.md](./ship-remote-environments.md)：remote 排查与 mux；sync 可复用同一 SSH/host 配置，但不阻塞本 change。

## 风险

| 风险 | 缓解 |
|------|------|
| `root` 默认改了破坏兼容 | 默认保持 `source` |
| Windows 无 rsync | doctor 提示；incremental 降级策略写清 |
| `--sync` 误传生产库覆盖远端 | `when = explicit` 默认；危险目标可要求 `-y` |
| 与用户裸 `pre_deploy` scp 双轨 | 文档引导迁移到 `deploy.sync`；旧步骤仍可用 |

## 完成定义

1. 消费项目可不靠本机绝对路径 hack，从 InvocationRoot 同步 gitignore 的 data/media。
2. 默认 `ship run` / `ship deploy` 在仅更新镜像+版本化配置时，不被大目录 scp 拖死。
3. 文档与 skill 说明：外部输入、`root`、`when`、与 digest pin 的分工。
4. 有基础测试：root 解析、sync exclude、无 rsync 时的行为。
