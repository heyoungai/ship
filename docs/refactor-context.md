# ship 重构上下文总览

> 状态：工作上下文导出  
> 目的：把本次 `ship` 重构相关的背景、已完成项、分析结论、设计方向和后续建议统一沉淀到 `docs/`，避免后续继续推进时丢失上下文。

## 1. 背景

`ship` 并不是从零开始设计的，而是从多个项目里长期存在的 `scripts/build_push.py` 演化出来的通用工具。

过去的工作方式是：

- 每个项目放一个 `build_push.py`
- 根据项目类型（Docker 镜像、前后端组装、Go 二进制、远程部署方式）做定制
- 脚本内部维护自己的默认配置、命令流程、输出样式和远程部署逻辑

现在的目标是：

**把这些共性提炼进 `ship`，通过 `ship.toml` 让项目特化点配置化，而不是继续每个项目一份脚本。**

## 2. 已分析的历史脚本

本次分析过的脚本如下：

1. `C:\code\deali-home-next\deali-home-fumadocs\scripts\build_push.py`
2. `C:\code\2\chats\scripts\build_push.py`
3. `C:\code\django-starter\django-starter-web\scripts\build_push.py`
4. `C:\code\server-tools\swag\scripts\build_push.py`
5. `C:\code\ledger-bot\LedgerBot\scripts\build_push.py`
6. `C:\code\avatar-sense\avatar-sense-next\scripts\build_push.py`

## 3. 从历史脚本里提炼出的项目类型

### A. 标准 Docker 镜像项目

典型项目：

- `deali-home-fumadocs`
- `avatar-sense`
- `django-starter`
- `LedgerBot`

共同点：

- 获取版本（通常是 git tag）
- 从 `.env` 读取 build args
- 构建 Docker 镜像
- 打 tag / 推送 registry
- 远程更新 `.env` 中的镜像 tag
- `docker compose up -d`

这类项目是当前 `ship` 已经覆盖得比较好的区域。

### B. 多阶段产物组装项目

典型项目：

- `2\chats`

共同点：

- 不只是“构建 Docker 镜像”
- 需要先做多步准备：
  - 后端 publish
  - 前端 build
  - 复制静态文件
  - 生成运行时 Dockerfile
  - 生成 compose / `.env` 发布物料
- 最后才进入镜像化与部署

这类项目说明 `ship` 不能只围绕单个 `docker build` 展开，而必须支持**阶段式编排**。

### C. Go 二进制发布项目

典型项目：

- `server-tools\swag`

共同点：

- 产物不是 Docker 镜像，而是可执行文件
- 需要交叉编译
- 通过 SCP 上传
- 远程 `sudo mv`、`chmod +x`
- 可能涉及 TTY / sudo password / `NOPASSWD`

这类项目说明 `ship` 必须从 image-first 升级成更通用的发布工具。

## 4. 当前 ship 已完成的代码重构（迁移到 `C:\code\ship` 前后的一致上下文）

以下内容已经作为本轮重构的一部分落地到了当前仓库：

### 4.1 可靠性与行为修正

- `build` 补齐了 `-p/--profile`
- `run` 也支持了 `-p/--profile`
- Windows 本地构建从硬编码 `sh -c` 改成按平台选 shell
- `buildx` 产物策略显式收紧为 `--load`，并对不安全的多平台 staged flow 直接报错
- `rollback` 的版本回退逻辑修正，不再错误地回到最新 tag
- `deploy` 的远程 `.env` 更新改成更安全的写法
- 加入了可选健康检查
- 配置和历史写入错误不再 `os.Exit` / 静默忽略，而是向上返回

### 4.2 命令行界面

本轮已经把界面层切到了：

- **Cobra**：命令结构
- **PTerm**：输出层
- **Huh**：交互确认

其中：

- 旧的手写 `internal/progress.go` 已删除
- `history` 输出改为使用 PTerm table
- `init` 覆盖已有 `ship.toml` 时改用 Huh 确认
- `rollback` 执行前改用 Huh 确认
- 新增全局 `-y / --yes`

### 4.3 界面偏好上的结论

这一轮 UI 调整里有一个明确结论：

**用户偏好的是“简洁、克制、信息密度高”的 CLI，而不是大面积块状 UI。**

因此当前方向应该是：

- 保留 PTerm
- 但只使用轻量的 section / info / table / success / warning
- 避免大块黑底 Header、重 Box、重 Panel

这个偏好对后续 UI 继续演进非常重要。

## 5. 当前技术状态里需要注意的一点

接入 PTerm / Huh 后，`go mod tidy` 将 `go.mod` 中的 `go` 指令提升到了：

```txt
go 1.25.8
```

这是当前依赖解析后的结果。  
后续如果希望回压到 `1.24`，需要专门做一轮兼容性整理，而不是顺手改回去。

## 6. 新的 v2 配置模型设计结论

本次已经单独写入：

- `docs/ship-toml-v2-draft.md`
- `config.example.toml`

v2 的核心方向不是“再加几个 flag”，而是：

**把 ship 升级成“阶段式配置 + 内置 driver + 少量 step hooks”的通用发布编排器。**

### 6.1 v2 的核心结构

```toml
schema = 2

[project]
[version]
[features]
[vars]

[[matrix]]

[build]
[publish]
[deploy]
[verify]

[[steps.prepare]]
[[steps.post_build]]
[[steps.pre_publish]]
[[steps.post_publish]]
[[steps.pre_deploy]]
[[steps.post_deploy]]

[[templates]]
```

### 6.2 v2 的关键思想

1. **阶段式理解不变**  
   用户仍然按 `prepare -> build -> publish -> deploy -> verify` 理解流程。
2. **driver 决定行为**  
   每个阶段由 `driver` 选择具体策略。
3. **内置策略优先**  
   不让 `ship.toml` 退化为任意脚本集合。
4. **step hooks 作为逃生口**  
   只在项目特化点时使用。

### 6.3 build / publish / deploy / verify 的目标 driver

#### build

- `docker`
- `go-binary`
- `command`

#### publish

- `registry`
- `scp`
- `none`

#### deploy

- `compose`
- `binary-install`
- `ssh`
- `none`

#### verify

- `http`
- `ssh`
- `command`
- `none`

## 7. 为什么 v2 能覆盖这些历史脚本

### 标准 Docker 项目

直接使用：

- `build.driver = "docker"`
- `publish.driver = "registry"`
- `deploy.driver = "compose"`

### 组装型项目（如 chats）

使用：

- `steps.prepare`
- `templates`
- `build.driver = "docker"`
- `publish.driver = "registry"`
- `deploy.driver = "compose"`

### Go 二进制项目（如 swag）

使用：

- `build.driver = "go-binary"`
- `publish.driver = "scp"` 或 `none`
- `deploy.driver = "binary-install"`

## 8. 推荐的实现顺序

### 第一阶段：把当前 image-first 扩成 pipeline-first

优先做：

- `steps.prepare`
- `version.fallback`
- `publish.registry.targets`
- `deploy.compose.tag_key`
- `deploy.compose.up`
- `verify.http`

这一步可以先吃掉：

- `deali-home-fumadocs`
- `avatar-sense`
- `django-starter`
- `LedgerBot`

### 第二阶段：补 Go 二进制能力

优先做：

- `build.driver = "go-binary"`
- `publish.driver = "scp"`
- `deploy.driver = "binary-install"`

这一步可以吃掉：

- `server-tools\swag`

### 第三阶段：补模板与物料生成

优先做：

- `[[templates]]`
- 更完整的 `steps.prepare`

这一步可以吃掉：

- `2\chats`

## 9. 第一版刻意不做的事情

为了控制复杂度，第一版 v2 **不建议**做：

1. 通用表达式引擎
2. 任意 DAG 依赖图
3. 插件系统
4. 多 artifact 并行产出
5. 过度泛化的模板变量系统

## 10. 当前仓库里与这次上下文直接相关的文件

- `README.md`
- `config.example.toml`
- `docs/ship-toml-v2-draft.md`
- `internal/config.go`
- `internal/history.go`
- `internal/ui.go`
- `cmd/run.go`
- `cmd/init.go`
- `cmd/rollback.go`

## 11. 当前结论一句话版

`ship` 下一步不应该继续只围绕 Docker 镜像流程修修补补，而应该正式升级成：

**“阶段式、可配置、内置多种 driver 的通用发布工具”。**
