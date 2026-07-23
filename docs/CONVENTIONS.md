# 文档规范

ship 文档治理的最低约定。调整结构时先读 [documentation-design.md](./documentation-design.md)。

## 命名

- 顶层活文档：`kebab-case.md`（如 `product-design.md`）
- ADR：`adr-NNNN-short-title.md`，序号递增
- Change plan：`changes/active/<topic>.md`；完成后移到 `changes/completed/`

## 放哪一层

| 内容类型 | 目录 |
|----------|------|
| 产品定位、长期边界 | 顶层 `product-design.md` |
| 使用教程、推荐范式 | `guides/` |
| 发布语义、工程契约 | `engineering/` |
| 未完成的局部规划 / 机制设计 | `changes/active/` |
| 已拍板的长期决策 | `adr/` |
| 调研、草案、远期设想 | `research/` |

## 原则

1. **顶层保持短**：默认入口能扫完；细节下沉到子目录。
2. **research 不进默认路径**：实现任务不要先读 research。
3. **规划 ≠ ADR**：未实现或未稳定的设计放 change plan；稳定后再提升 ADR 或写入 `product-design.md`。
4. **一次一个权威来源**：避免同一主题在两处各写一版且互相漂移。
5. **改路径必改链接**：根 `README.md`、`docs/README.md` 与交叉引用一并更新。
