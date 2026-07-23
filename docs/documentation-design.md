# 文档体系设计

说明 ship 的 `docs/` 分层原则与入口设计，让人和 AI 能快速找到权威文档。

## 为什么需要分层

平铺所有文档会导致：

1. 默认阅读路径过长，注意力被调研稿稀释。
2. 产品边界、使用指南、工程契约、临时规划混在一起，难判断改哪一层。
3. 聊天结论没有稳定归档位置。

参考 specforge 类文档分层精神（顶层入口 + guides / engineering / changes / adr / research），按 CLI 项目精简，不引入 frontend / 完整 phase 镜像。

## 各层职责

| 层 | 职责 |
|----|------|
| 顶层 | 产品权威、文档体系自身、导航入口 |
| `guides/` | 上手与推荐用法 |
| `engineering/` | 发布语义等工程契约 |
| `changes/` | 局部演化与未实现规划 |
| `adr/` | 已稳定的长期决策 |
| `research/` | 背景与草案，非默认规范 |

## Progressive disclosure

1. 先读 `docs/README.md`。
2. 按任务类型进入对应子索引。
3. 仅在需要背景时打开 `research/`。

## 与产品的关系

文档分层服务于 [product-design.md](./product-design.md)：ship 是发布执行器。文档体系应帮助守住边界，而不是鼓励把任意设想堆进顶层「权威」位置。
