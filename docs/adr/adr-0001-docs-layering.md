# ADR 0001 · 文档分层规则

- Status: accepted
- Date: 2026-07-23

## Context

ship 早期文档平铺在 `docs/` 顶层。随着快速上手、产品原则、发布策略、重构上下文与 v3 草案增加，顶层噪音变大，默认阅读路径不清晰。

## Decision

采用精简分层：

1. 顶层只保留导航、产品权威与文档治理。
2. 使用指南进入 `guides/`。
3. 工程契约进入 `engineering/`。
4. 局部规划进入 `changes/`（active / completed）。
5. 已稳定决策进入 `adr/`。
6. 调研与远期草案进入 `research/`（非默认路径）。

不引入与 CLI 无关的 `frontend/` 等关注域目录。

## Consequences

- 入口更短，AI/人类更容易选对文档。
- 新增文档前需先判断层级。
- 旧外链需按 `docs/README.md` 迁移对照更新。

## Alternatives

- 继续平铺：短期简单，长期入口噪音更大。
- 完整镜像 specforge（含 phase/frontend）：对当前 CLI 仓库过重。
