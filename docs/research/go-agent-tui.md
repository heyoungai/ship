# Go 终端 Agent TUI 调研（对照 Pi / Claude Code）

> 状态：调研归档（2026-07-24）  
> 目的：评估 `ship ai` 从「线模式 REPL + streaming」再往「全屏会话 TUI」走时，Go 生态可参考什么、怎么实现、和产品边界如何取舍。  
> 相关活文档：[ship AI advisor change](../changes/active/ship-ai-advisor.md)、[product-design §7](../product-design.md)、[guides/ship-ai](../guides/ship-ai.md)。

## 1. 背景与问题

`ship ai` v0 已具备：

- OpenAI-compatible **SSE streaming**
- 线模式 REPL 美化（横幅、spinner、默认工具行）

仍缺用户期望的 Pi / Claude Code 类体验：

- 全屏 TUI（alt-screen）
- **底部固定输入框**
- **主体可滚动对话历史**
- **Markdown 渲染**
- **命令补全**（`/` slash、`@` 文件等）

本调研回答：Go 里有没有类似 Pi / Claude Code 的 agent？它们的 TUI 怎么做？对 `ship ai` 意味着什么？

## 2. 语言版图（先对齐预期）

| 产品 | 语言 | 定位 |
|------|------|------|
| Claude Code | TypeScript | 全家桶 coding agent |
| Pi | TypeScript（自研 `pi-tui`，差分渲染） | 极薄 harness + 可扩展 TUI / 扩展包 |
| **Crush**（原 Go 版 OpenCode） | **Go** | Charm 出品，Bubble Tea 全屏 TUI |
| OpenCode（现社区主线） | 主要 TypeScript | 与 Crush 分家后的另一条线 |

结论：若找「**Go 写的、接近 Pi/Claude Code 的终端 agent**」，主答案是 **Crush**；其余多为更小的 Bubble Tea 仿作，或「TUI 与 agent 解耦」的实验。

Pi 与 Crush 路线不同：

- **Pi**：可编程平台；扩展可改 UI / 工具 / 事件总线（TypeScript in-process）
- **Crush**：产品化全屏 TUI + 配置/MCP/LSP；体验打磨强，扩展天花板相对封闭

## 3. Go 生态项目一览

### 3.1 Crush — 标杆

- 仓库：https://github.com/charmbracelet/crush
- 许可：FSL（需注意商用/分发约束）
- 栈：`bubbletea/v2` · `lipgloss/v2` · `glamour/v2` · **fantasy**（Charm agent/streaming 库）· SQLite session · 可选本地 socket 客户端-服务端

**UI 结构（`internal/ui`，摘要）：**

- 单一顶层 `UI` Bubble Tea model；子组件多为命令式方法，不全员进 Elm Update 循环
- 中间：**Chat 列表**（懒渲染可见项、follow 自动滚底、工具消息可折叠 / 专用 renderer）
- 底部：**textarea 编辑器**；焦点在 editor ↔ chat 间切换
- **Completions**：slash / 附件等补全面板
- **Markdown**：消息项缓存 + Glamour；避免每 token 全量重渲
- Streaming：Fantasy `Agent.Stream` 回调 → 更新 list → 重绘

Crush 是完整 coding agent（多 provider、LSP、MCP、权限、session…），**不是**可直接嵌进 `ship` 的小组件。对 `ship ai` 的价值主要是**布局与渲染模式**可抄，而不是依赖整仓。

### 3.2 Fantasy — agent 内核（不是 TUI）

- 仓库：https://github.com/charmbracelet/fantasy
- 多 provider、tool loop、`Stream` 回调（text delta / tool call / step / reasoning）
- Crush 用它跑模型；**全屏 UI 仍要自己用 Bubble Tea 画**
- 对 ship：可评估是否替换自管 HTTP；**不解决全屏会话壳**

### 3.3 更小的 Go Bubble Tea agents

| 项目 | 特点 | 参考价值 |
|------|------|----------|
| [cfbender/hygge](https://github.com/cfbender/hygge) | Go + Bubble Tea；SQLite；slash / MCP / subagent | 同族完整度，体量仍大 |
| [Edcko/techne-code](https://github.com/Edcko/techne-code) | Bubbletea v2；流式 + 工具可视化 + 滚动 | 更适合对照「最小全屏 chat」 |
| [anton-abyzov/ccx-go](https://github.com/anton-abyzov/ccx-go) | Glamour + `tui/chat` / `tui/prompt` 分包 | 输入区与消息区拆分清晰 |
| [magicwubiao/go-magic](https://github.com/magicwubiao/go-magic) | TUI + Web 双前端 | agent 与 UI 解耦示例 |

### 3.4 GACT — 只做漂亮 TUI，agent 另说

- 仓库：https://github.com/iowarp/gact-tui
- 思路：每个 agent 都重做 streaming / 权限 / 补全 TUI 太贵 → 统一 **REST + SSE 契约**，一份 TUI 接多种后端
- 已有 Crush / OpenCode / Claude Code / Goose 等 adapter
- 对 ship：长期可做成「`ship ai` 暴露事件流 + 外挂 TUI」；短期要契约 + 适配，成本不低

### 3.5 同生态但不算同级

- **mods**（Charm）：管道向 LLM，不是 agent 全屏会话
- **Pi**：体验对标对象，但是 TS；扩展模型值得读理念，实现栈不同

## 4. 共性实现模式

用户点名的能力，在 Go 全屏 agent 里几乎都落成同一布局：

```text
┌─────────────────────────────────────┐
│  header / status（model、session）   │
├─────────────────────────────────────┤
│                                     │
│  viewport / list（消息历史）          │  ← 滚轮；流式追加；MD 缓存渲染
│  · user / assistant / tool blocks   │
│                                     │
├─────────────────────────────────────┤
│  completions popup（/ @ 补全）        │  ← 可选叠加层
├─────────────────────────────────────┤
│  textarea（多行输入、粘贴、发送）      │  ← 底部固定
└─────────────────────────────────────┘
         ▲
         │ tea.Msg: delta / tool / resize / key
         │
    agent loop（自管 SSE 或 Fantasy.Stream）
```

| 能力 | 常见实现 |
|------|----------|
| 全屏壳 | Bubble Tea alt-screen |
| 滚动历史 | bubbles viewport 或懒渲染 list |
| 底部输入 | bubbles textarea |
| Markdown | glamour（定稿 / 分块缓存；忌每 token 全量重渲） |
| `/` 补全 | 输入前缀触发 list / fuzzy overlay |
| 工具块 | 独立 message item，可折叠 |
| Streaming | 回调 → 发 `tea.Msg` → 局部更新 list item |

技术选型几乎垄断：**Bubble Tea + Lip Gloss + Glamour**（Charm 栈）。ship 已间接依赖 bubbletea/lipgloss（经 huh），直接依赖升级成本可控。

## 5. 与当前 `ship ai` 的差距

| 层 | 现状（2026-07-24） | 全屏 agent 常态 |
|----|-------------------|-----------------|
| 协议 | 自管 SSE streaming | 同左或 Fantasy |
| 呈现 | 线模式 Reporter + pterm | Bubble Tea 会话窗 |
| 输入 | `bufio` 单行提示符 | 底部多行 textarea |
| 历史 | 终端滚动缓冲区 | 应用内 viewport + 可回看 |
| MD | 流式纯文本 | Glamour 渲染（常缓存） |
| 补全 | `/quit` `/help` 手打 | slash palette / `@` 文件 |

结论：streaming 层已具备；缺的是**第二层——全屏会话壳**。这不是「再引一个 TUI 库」的增量，而是一类独立应用面。

## 6. 可选路线（未拍板）

| 路线 | 做法 | 成本 | 与产品边界 |
|------|------|------|------------|
| **A. 自研轻量 Bubble Tea 壳** | 复用现有 `Agent` + 把 Reporter 改为发 `tea.Msg`；viewport + textarea + glamour + `/` 补全；不做 session/LSP/MCP | 中～高 | 需改 product-design「不做全屏 TUI」的表述 |
| **B. 仿 Crush 子集** | 抄 chat/editor/completions，砍掉 LSP/MCP/多 session | 高 | 易滑向迷你 Crush |
| **C. 不自研，外挂** | 文档推荐 Crush/Pi 做通用 coding；`ship ai` 保持发布顾问 + `-p` | 低 | 最贴原边界；交互上限仍是线模式 |
| **D. GACT 式解耦** | ship 出 JSON/SSE 事件，TUI 另进程 | 中长期 | 架构干净；短期最重 |

**务实倾向（调研意见，非决策）：**

- 若坚持「底部输入 + 滚动历史 + MD + `/` 补全」→ 走 **路线 A（最小全屏 chat 壳）**，明确范围：顾问会话 UI，不是第二 Claude。
- 若坚持「薄发布顾问、不当第二 coding agent」→ **路线 C**，线模式见顶，全屏体验外包给 Crush/Pi。

## 7. 若做路线 A，建议的最小范围

**做：**

1. Bubble Tea 全屏：上消息 viewport，下 textarea  
2. 接现有 `ChatStream` / tool loop（事件 → `tea.Msg`）  
3. 助手定稿（或分块）Glamour 渲染；流式阶段可先纯文本  
4. `/quit` `/help` 等 slash 补全面板（少量固定命令即可）  
5. 非 TTY / `-p` 仍走现有线模式或纯 stdout（不进 alt-screen）

**不做（本阶段）：**

- Session 树 / 持久化 / compaction  
- LSP、MCP、多 provider UI、中途换模型  
- 子 agent、plan mode、扩展市场  
- 引入整仓 Crush 或 Fantasy 全家桶（除非单独评估 provider 层）

## 8. 参考链接

- Crush：https://github.com/charmbracelet/crush  
- Fantasy：https://github.com/charmbracelet/fantasy  
- Pi：https://pi.dev/ · https://github.com/badlogic/pi-mono  
- GACT：https://github.com/iowarp/gact-tui  
- techne-code：https://github.com/Edcko/techne-code  
- ccx-go：https://github.com/anton-abyzov/ccx-go  
- hygge：https://github.com/cfbender/hygge  
- Agent harness 横向对比（第三方）：https://wuu73.org/aiguide/infoblogs/coding_agents/index.html  

## 9. 下一步

1. 在产品层明确选 **A / C**（或分阶段：先 A 最小壳）。  
2. 若选 A：开新 change plan（例如 `ship-ai-fullscreen-tui.md`），把 §7 最小范围写成验收清单；同步改 `product-design.md` §7 措辞。  
3. 本文件保持 research：实现细节以 change plan / 代码为准，避免双源漂移。
