# Release Notes

用户可见的版本说明。权威正文按版本分文件；GitHub Release 优先使用对应文件。

## 索引

| 版本 | 日期 | 摘要 |
|------|------|------|
| [v2.8.0](./v2.8.0.md) | 2026-08-03 | Push 正式 tag 幂等、`ship ai`、`--pull=false` |
| [v2.7.1](./v2.7.1.md) | 2026-07-24 | Digest pin 身份 hotfix |
| [v2.7.0](./v2.7.0.md) | 2026-07-23 | 发布会话与 digest 钉部署 |

更早版本未回溯撰写；需要时从 GitHub Release / git tag 对比补写。

## 发版时怎么写

1. **在打 tag 之前**新增 `docs/releases/vX.Y.Z.md`（本目录）。
2. 更新本页索引表。
3. 把该文件与其它发版前改动一起提交到 `master`。
4. 打 annotated tag `vX.Y.Z` 并 push；CI 构建产物、创建 GitHub Release，并把本文件用作 Release body。
5. scoop manifest 由 release workflow 在产物上传后自动更新，**不要**在打 tag 前手写 hash。

## 正文写什么

面向**使用者**，不是 commit 流水账：

| 区块 | 写 |
|------|----|
| Highlights | 1–3 条本版最重要的变化 |
| Added / Changed / Fixed | 按用户可感知行为分类 |
| Upgrade notes | 破坏性变更、配置迁移、已知缺口 |
| Not in this release | 已规划但未进本版的能力（避免误以为已含） |

对照 `docs/changes/`：把本版实际交付的 Phase 写进 notes；未完成的留在 active，并在「Not in this release」点名。

## 与分层的关系

- **releases/**：对外版本说明（本层）
- **changes/**：开发中的规划与复盘
- **engineering/**：长期发布语义契约

同一主题不要在三处各写一版细节；releases 只摘要并链接权威文档。
