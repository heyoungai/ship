# ship 文档目录

本文档是 `docs/` 的默认阅读入口。只列活文档；`research/` 为归档/背景，除非任务明确要求，否则不要先读。

## 活文档

| 文档 | 类型 | 什么时候读 |
|------|------|------------|
| [产品设计与边界](./product-design.md) | 产品设计 | 理解定位、职责边界、演进原则 |
| [文档体系设计](./documentation-design.md) | 文档设计 | 调整 docs 结构、理解分层 |
| [文档规范](./CONVENTIONS.md) | 文档治理 | 新增、重命名、归档文档 |
| [使用指南索引](./guides/README.md) | 指南 | 快速上手、Docker 默认模式、ship ai |
| [工程文档索引](./engineering/README.md) | 工程契约 | 发布语义、git-tag 策略 |
| [Change Plans](./changes/README.md) | 变更规划 | 局部演化与未实现规划 |
| [ADR 索引](./adr/README.md) | 决策记录 | 已稳定的长期决策 |

## Change Plans（active）

| 文档 | 状态 | 定位 |
|------|------|------|
| [ship AI advisor](./changes/active/ship-ai-advisor.md) | in progress | AI 顾问 + [使用指南](./guides/ship-ai.md) |
| [ship remote environments](./changes/active/ship-remote-environments.md) | planned | 环境名册 + `ship remote` |
| [deploy sync 与外部输入](./changes/active/deploy-sync-and-external-inputs.md) | planned | InvocationRoot sync；digest 见 v2.7.1 hotfix |

## ADR

| 文档 | 状态 | 定位 |
|------|------|------|
| [ADR 0001 · 文档分层](./adr/adr-0001-docs-layering.md) | accepted | docs 分层结构 |

## 默认阅读顺序

- 上手发版：`guides/quick-start.md` → 需要时再读 `product-design.md`
- 改产品边界：`product-design.md`
- 改发布语义 / digest / git-tag：`engineering/git-tag-release-strategy.md`
- 局部规划（如 AI）：`changes/README.md` → 对应 active 文档
- 改文档结构：`documentation-design.md` → `CONVENTIONS.md`

## 文档分层

```text
README.md
  ├─ product-design.md
  ├─ documentation-design.md
  ├─ CONVENTIONS.md
  ├─ guides/           # 使用指南
  ├─ engineering/      # 工程契约
  ├─ changes/          # 局部变更 / 规划
  └─ adr/              # 已稳定决策

research/              # 背景归档，非默认路径
```

## 路径迁移对照

| 旧路径 | 新路径 |
|--------|--------|
| `docs/design-principles.md` | `docs/product-design.md` |
| `docs/quick-start.md` | `docs/guides/quick-start.md` |
| `docs/docker-default-pattern.md` | `docs/guides/docker-default-pattern.md` |
| `docs/git-tag-release-strategy.md` | `docs/engineering/git-tag-release-strategy.md` |
| `docs/refactor-context.md` | `docs/research/refactor-context.md` |
| `docs/ship-toml-v2-draft.md` | `docs/research/ship-toml-v2-draft.md` |
| `docs/ship-v3.md` | `docs/research/ship-v3.md` |
