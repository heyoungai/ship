# Change Plan · ship remote environments

- Status: **planned**（未开始编码）
- Date: 2026-07-23
- Owner: TBD

> 本文档是规划，不是实现规范。稳定落地后再考虑提升为 ADR，或把长期边界写入 [product-design.md](../../product-design.md)。

## 背景

线上排查是发布后的高频场景。当前常见做法是：每个服务器一份 Claude/Cursor skill（如 `debug-on-server-yunan-1`），写明 SSH 别名、远程 path、本地↔远程目录映射，再让 agent 反复执行裸 `ssh`。

问题：

1. **双源真相**：host/path 已在 `ship.toml`（或 `vars`）里，又在 per-server skill 里抄一份，易漂移。
2. **N 机 = N skill**：内容同构，只有名片字段不同。
3. **选机靠人**：先启用对的 skill，agent 才知道去哪台机。
4. **缺发布上下文**：skill 不知道最近 release、digest、compose 文件名。
5. **连接成本**：每次新 `ssh` 进程重复握手（可用 OpenSSH mux 缓解，但未默认开启）。

本规划用**轻量环境名册 + `ship remote` CLI** 收口上述问题；不把 ship 做成常驻 SSH / 通用运维平台。

相关：已有 AI 规划见 [ship-ai-advisor.md](./ship-ai-advisor.md)（本地配置顾问，与本能力正交）。profile vs environment 的远期拆分见 [ship-v3.md](../../research/ship-v3.md)（research，非本 change 的实现前提）。

## 目标

1. 在 `ship.toml` 中声明可部署/可排查的 **environments**（至少 host + path）。
2. 提供确定性命令：`ship remote …`，按 `-e ENV`（或默认环境）执行 status / logs / exec 等。
3. 用**一个** ship skill 说明排查协议，**替代**项目内 per-server `debug-on-server-*` skill。
4. 所有走系统 `ssh`/`scp` 的路径（含 deploy 与 remote）默认启用 **ControlMaster** 复用，降低重复握手成本。

## 非目标

- 不做常驻 daemon / 连接池产品化（不内嵌 `x/crypto/ssh` 连接管理器作为一等方案）
- 不做通用「任意 remote shell」MCP；MCP 若做，仅为窄工具薄壳（Phase 可选）
- 不做服务器端 ship agent
- 不替代 Teleport / Bastion / 完整运维平台
- 不在本 change 中完成 research v3 的完整 profile/environment 发布语义拆分（deploy 仍可继续用现有 host/path；environments 先服务 remote，并可与 deploy 对齐）
- 不把 layout / notes 做成通用 DSL 或表达式引擎

## 建议命令面

| 命令 | 说明 |
|------|------|
| `ship remote ls` | 列出环境：name / host / path / description |
| `ship remote context [-e ENV]` | 结构化输出：环境名片 + 可选最近 release / compose 提示 |
| `ship remote status [-e ENV]` | 远程健康快照（compose ps 或等价的最小检查） |
| `ship remote logs [-e ENV] [-s svc] [-f]` | 绑定环境 path / 服务名 |
| `ship remote exec [-e ENV] -- <cmd…>` | 在环境 `path` 下执行；走 mux |
| `ship remote resolve [-e ENV] <local>` | 按 layout 把本地相对路径解析为远程绝对路径 |

公共 flags（建议）：

- `-e, --environment`：目标环境；省略则用 default（见下）
- `--json`：`ls` / `context` / `status` 等适合 agent 消费的输出
- `-y`：若某子命令将来需要确认（默认 remote 以只读/显式 exec 为主）

## 配置草图

### 显式 environments

```toml
[environments.default]
host = "yunan-1"
path = "/root/projects/ai-design-platform"
description = "主站"

[environments.sg]
host = "deali.sg"
path = "/home/deali/projects/ai-design-platform"
description = "新加坡"

# 可选：本地相对路径 → 远程相对（相对 path）或绝对路径
[environments.sg.layout]
"apps/api" = "api"
"apps/web" = "web"
"apps/prototype" = "prototype"
```

建议字段（够用即停）：

| 字段 | 必需 | 用途 |
|------|------|------|
| `host` | 是 | SSH config 别名或可达主机名 |
| `path` | 是 | 远程项目根；`exec` 的默认 cwd |
| `description` | 否 | `ls` / agent 选机 |
| `layout` | 否 | 替代 per-server skill 里的目录对照 |
| `compose_file` | 否 | 覆盖默认 compose 文件名 |
| `services` | 否 | 常用服务名提示（logs 默认） |
| `notes` | 否 | 短文本坑点，进 `context` 给 agent |

### 隐式 default（Phase 1 兼容）

未声明 `[environments.*]` 时，从现有配置合成名为 `default` 的环境，避免强迫立刻改 schema：

1. `deploy.compose.host` + `deploy.compose.path`（模板渲染后）
2. 或已解析的 `vars` / 等价覆盖（如 `REMOTE_HOST` / `REMOTE_PROJECT_PATH`，若项目已在用）

有显式 `environments` 时：以配置为准；deploy 是否改为「默认环境的 host/path」可作为后续对齐项，**本 change 不强制第一期改 deploy 语义**。

### 与 profile 的关系

- **profile**：构建变体（品牌、镜像矩阵等）——保持现状。
- **environment**：部署/排查目标机——本 change 引入的名册。

二者正交；不要用 profile 冒充多服务器。

## 架构草图

```text
ship remote <subcommand> [-e ENV]
  → 加载 ship.toml
  → ResolveEnvironment(ENV | default | 隐式合成)
  → （ssh/scp）带 ControlMaster 的系统客户端
  → 子命令：ls | context | status | logs | exec | resolve
  → 人类可读 或 --json
```

### SSH 复用（Phase 0）

继续外包系统 `ssh`/`scp`，为 ship 发起的调用注入（或文档约定等价的）OpenSSH 选项，例如：

- `ControlMaster=auto`
- `ControlPath`：稳定、按 `%r@%h:%p` 隔离的 socket 路径（注意 Windows OpenSSH 路径与目录权限）
- `ControlPersist`：短时（如 10m），避免长期僵尸 socket

约束：

- 按 **host** 分 socket；多环境多 host 互不抢主连接
- CI / 短生命周期环境：Persist 宜短；失败时清理/重建 socket 的错误信息要可读
- 不把「用户必须先改 `~/.ssh/config`」当成唯一方案；ship 侧默认带上更稳

### context 内容（建议最小集）

- 环境名、host、path、description、layout、notes、services
- 若 `.ship/releases` 有最近成功记录：version / digest / 时间（只读提示，不假装远程已核验）
- compose 本地/远程文件名（来自 deploy 配置或环境覆盖）

### Agent / skill

- **删除**项目内 `debug-on-server-*` 的推荐路径：改用 `ship remote`
- 更新 [ship skill](../../../cmd/skills/SKILL.md)：增加「线上排查」小节（ls → context → status/logs → 少量 exec；路径用 resolve）
- 可选：`ship skill` 根据 environments **生成环境表**进 REFERENCE（仍只有一个 skill 目录）

### 与 AI advisor 的分工

| 能力 | 职责 |
|------|------|
| `ship ai *` | 本地：生成/解释 `ship.toml`、plan/doctor |
| `ship remote *` | 远程：确定性探测与执行，绑定环境名册 |
| skill | 教 agent 何时用哪条命令 |

advisor **不**持 SSH 连接；若日后有 `ship ai diagnose`，应消费 `remote context/status/logs` 的结构化输出，仍做顾问而非 loop agent。

## 安全边界

1. **默认偏只读**：`ls` / `context` / `status` / `logs` / `resolve` 无写破坏面；`exec` 显式、可审计。
2. **不把密钥写入** `ship.toml` 或 skill；继续依赖用户 SSH config / agent。
3. **不做**默认开放的「无环境参数任意 shell」MCP 工具。
4. 可选后续：`exec` 命令前缀白名单、或限制在 `path` 下；敏感操作需确认——第一期可不做，但不要设计成难以补上。
5. mux socket 落在用户本地状态目录（或 `~/.ssh/` 下 ship 专用路径），权限与 stale socket 行为要可预期。

## 建议实现分期与验收

**Phase 0 — SSH mux**

- [ ] deploy / push(scp) / verify(ssh) 等现有远程调用走 ControlMaster
- [ ] 同一 host 连续两次远程命令，第二次无明显完整握手成本（手工或基准可接受即可）
- [ ] Windows OpenSSH 路径经验证或有明确 fallback 说明

**Phase 1 — 隐式 default + 核心 remote**

- [ ] 无 `environments` 时从 deploy host/path 合成 `default`
- [ ] `ship remote context|status|logs|exec`（`-e` 可先仅支持 default）
- [ ] `--json` 至少覆盖 `context`
- [ ] 单机项目可删除 `debug-on-server-<default-host>` skill，改用 ship skill 排查小节

**Phase 2 — 显式 environments + 多机**

- [ ] 解析 `[environments.*]`（含 layout / description 等可选字段）
- [ ] `ship remote ls` / `resolve`
- [ ] `-e` 选择非 default 环境
- [ ] 双机项目可删除全部 `debug-on-server-*`

**Phase 3 — skill 与文档**

- [ ] 更新 bundled skill：线上排查协议
- [ ] 可选：skill 安装时写入环境表摘要
- [ ] guides 短文或 quick-start 一节（按需）

**Phase 4 — 可选窄 MCP**

- [ ] 仅当有明确 IDE 需求时：`list_environments` / `get_context` / `status` / `logs` / `resolve_path`；`exec` 默认关闭或显式允许
- [ ] 与 CLI 共用同一套 ResolveEnvironment + 远程实现

## 与文档分层的关系

- 规划留在本 change plan，**不单独建 ADR**（规划态 ≠ 已稳定决策）。
- 若「environment 名册 + remote 只做发布上下文探针」成为产品铁律，再写入 `product-design.md` 或提升 ADR。
- 完整的 deploy×environment 发布语义若扩大，应另开 change 或对照 [ship-v3.md](../../research/ship-v3.md)，避免本文件膨胀成 v3 总纲。
