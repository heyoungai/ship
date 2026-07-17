# Git Tag 真实版本构建策略

> 状态：设计提案  
> 目标版本：v3 基础能力，可先在 v2 兼容层分阶段落地  
> 核心结论：正式发布版本必须同时锁定版本号、源码提交和发布产物，不能只给当前工作目录生成的产物贴上 Git tag 标签。

## 1. 背景与问题

ship 当前把 `git-tag` 主要作为版本字符串来源：

1. [`internal/exec.go`](../internal/exec.go) 使用 `git describe --tags --abbrev=0` 获取当前 HEAD 最近可达的 tag。
2. [`cmd/build.go`](../cmd/build.go) 使用当前进程工作目录中的 Docker context、Dockerfile、hooks、templates 和部署文件执行流水线。
3. 将第一步得到的版本字符串用于镜像 tag 和部署记录。

当 HEAD 已经领先最近 tag，或者工作目录包含未提交修改时，会出现：

```text
镜像版本：v1.2.3
实际源码：v1.2.3 之后的若干提交，甚至包含未提交文件
```

`ship run -v v1.1.0` 的风险更大：当前实现会把 `v1.1.0` 当作任意版本字符串，而不会验证它是否为真实 Git tag，也不会切换到该 tag 对应的源码。

这破坏了版本号最重要的承诺：

> 给定版本应能唯一定位到构建它的源码和产物。

因此，真实版本构建不是单纯增加一次 `git worktree` 命令，而是需要明确 release identity、source snapshot、execution roots、artifact immutability 和 deploy semantics。

## 2. 设计目标

本策略必须满足以下目标：

1. **真实**：`v1.2.3` 默认只能从 `refs/tags/v1.2.3` 指向的 commit 构建。
2. **不可变**：同一版本不能被另一份源码或另一份产物静默覆盖。
3. **不干扰开发目录**：构建旧版本时不切换用户当前分支，不覆盖未提交修改。
4. **全链路一致**：源码、Dockerfile、hooks、templates、compose 和部署物料来自同一源码快照。
5. **构建与部署解耦**：部署已有版本时不重新构建；生产部署消费已经发布的确定产物。
6. **可追溯**：记录 tag、commit SHA、artifact digest、配置来源和运行 ID。
7. **简洁**：标准 Git tag 发布不要求用户同时填写 version 和 ref 两份重复信息。
8. **可恢复**：失败、取消和进程异常后，临时 worktree 与运行状态可以安全清理或诊断。

## 3. 非目标

本策略不负责：

- 自动创建或移动 Git tag
- 自动决定业务版本号
- 替代 CI 平台触发发布
- 用 worktree 实现通用多仓库编排
- 承诺所有第三方依赖都可重复获取
- 把 `latest` 当作生产部署版本

ship 可以记录和验证构建输入，但“可追溯构建”和“字节级可重复构建”是不同能力。后者还需要锁定基础镜像、依赖仓库、构建器版本、时间戳等输入。

## 4. 必须成立的不变量

### 4.1 Git tag 与源码

在 `git-tag` 模式下：

```text
release.version == git tag name
release.source_ref == refs/tags/<version>
release.source_commit == resolve(release.source_ref^{commit})
```

- tag 必须存在于本地仓库。
- 轻量 tag 和 annotated tag 都必须最终解析为 commit。
- 必须使用完整引用 `refs/tags/<version>`，避免 tag 与 branch 同名产生歧义。
- 在计划生成后，后续阶段使用已经锁定的 commit SHA，不再重新解析可移动引用。

### 4.2 产物

正式发布产物必须至少记录：

- release version
- source ref
- source commit SHA
- artifact digest 或文件 SHA-256
- profile、platform
- run ID
- builder/ship 版本

Docker 镜像应写入标准 OCI labels：

- `org.opencontainers.image.version`
- `org.opencontainers.image.revision`
- `org.opencontainers.image.source`
- `org.opencontainers.image.created`

### 4.3 部署

- `deploy` 消费已经发布的 artifact，不触发 build。
- 生产环境优先按 digest 部署；版本 tag 是人类可读别名，不是最终身份。
- rollback 使用历史中记录的 artifact digest 或 release manifest，不根据当前源码重新构建。
- `latest` 不得成为 production deploy 或 rollback 的输入。

## 5. 核心领域模型

内部应明确区分以下概念，即使兼容层仍读取 schema 2：

```go
// ReleaseIdentity 在计划阶段一次性解析完成，后续阶段只消费结果。
type ReleaseIdentity struct {
    Version      string // 对外版本，例如 v1.2.3
    SourceMode   string // git-tag | git-ref | current
    SourceRef    string // refs/tags/v1.2.3，current 模式可为空
    SourceCommit string // 完整 commit SHA
}

// ExecutionRoots 避免通过全局 cwd 隐式混用源码、外部输入和运行状态。
type ExecutionRoots struct {
    InvocationRoot string // 用户执行 ship 的原始项目目录
    SourceRoot     string // 已锁定源码快照所在的 worktree
    StateRoot      string // .ship/runs、history、manifest 等持久状态
}
```

这里最重要的边界是：

- `Version` 是人类可读发布身份。
- `SourceCommit` 是源码事实。
- artifact digest 是部署事实。

三者必须被 release manifest 连接起来。

## 6. 用户可见语义

### 6.1 标准正式发布

当配置为：

```toml
[version]
source = "git-tag"
fallback = "error"
```

执行：

```powershell
ship run -v v1.2.3 -y
```

语义必须是：

1. 验证 `refs/tags/v1.2.3` 存在。
2. 锁定其 commit SHA。
3. 从该 commit 创建 detached worktree。
4. 在该快照中执行 build、publish 和需要版本化物料的 deploy 阶段。
5. 保存 release manifest。
6. 清理临时 worktree。

`-v` 在 `git-tag` 模式下不再是任意标签字符串，而是正式 release tag。

`SHIP_VERSION` 等版本环境变量覆盖遵循相同规则：它可以选择 tag，但不能绕过 tag 存在性和 commit 解析校验。如果确实需要“显示版本与源码 ref 不同”，必须使用后文的显式 `--ref` 模式。

### 6.2 未显式指定版本

执行：

```powershell
ship run -y
```

可以继续使用 `git describe --tags --abbrev=0` 选择当前 HEAD 最近可达的 tag，但构建内容必须来自该 tag，而不是当前 HEAD。

当 HEAD 领先该 tag 时，必须显示高可见度提示：

```text
release: v1.2.3
source:  refs/tags/v1.2.3 @ 0123456...
current: HEAD @ abcdef0... (ahead by 12 commits; ignored)
```

这使“没有打新 tag 就不会发布新代码”成为稳定、直观的发布节奏控制机制。

### 6.3 显式构建当前开发代码

当前工作树构建必须是显式开发行为，例如：

```powershell
ship build --source current -v dev-abcdef0 -y
```

建议规则：

- 未提供版本时自动生成 `dev-<short-sha>`。
- 工作树 dirty 时再增加 `-dirty`，并打印包含未提交输入的警告。
- production environment 默认拒绝 `source=current`、`version=dev-*`。
- current 模式不允许复用正式 semver tag，避免把开发代码伪装成正式版本。

现有 `version.fallback = "dev"` 只能作为本地开发兼容行为，不能让带 publish/deploy 副作用的非交互 `ship run` 从正式 tag 策略静默降级为 current。正式流水线无法解析 tag 时应失败；开发者需要显式选择 `--source current`。迁移期如果保留交互降级，也必须展示将要使用的 commit 和 dirty 状态，并要求确认。

### 6.4 高级 Git ref

少数预发布场景可能需要版本和源码 ref 不同，可以提供显式逃生口：

```powershell
ship run --ref refs/heads/release/1.3 -v v1.3.0-rc.1 -y
```

该模式必须明确显示 version/ref/commit 的差异，不应成为默认推荐方式。

### 6.5 仅部署已有版本

```powershell
ship deploy -v v1.2.3 -y
```

该命令应该：

- 查找已经发布的 release/artifact manifest。
- 验证 registry 中的 digest。
- 使用该版本对应的部署物料或已固化 deployment bundle。
- 不 build、不重新 push、不移动 `latest`。

如果版本尚未发布，应报错并提示先执行 release/run，而不是偷偷从当前目录补一次构建。

## 7. 为什么使用 Git worktree

推荐使用 detached worktree：

```powershell
git rev-parse --verify "refs/tags/v1.2.3^{commit}"
git worktree add --detach $SourceRoot $CommitSha
```

相比直接 `git switch --detach`，worktree：

- 不改变用户当前分支和 index。
- 不要求工作目录 clean。
- 可以同时构建不同版本。
- hooks 和构建工具仍然能看到一个正常 Git worktree。

相比 `git archive`，worktree 更适合需要 Git 元数据、hooks、模板、包管理器和多阶段发布物料的真实项目。

## 8. Worktree 生命周期

建议每次 run 使用唯一目录：

```text
<os-temp>/ship/<repository-id>/<run-id>/source
```

生命周期：

1. 生成 run ID。
2. 解析并锁定 source commit。
3. 创建 detached worktree。
4. 校验 worktree HEAD 等于锁定 commit。
5. 执行流水线。
6. 先终止仍在运行的子进程。
7. 使用 `git worktree remove --force` 清理。
8. 清理失败时保留诊断路径并给出恢复命令。

实现要求：

- 成功、普通失败、Ctrl+C 都必须进入 cleanup。
- 不使用固定目录，支持并发 run。
- 不在每次启动时无差别执行全局 `git worktree prune`，避免影响其他进程。
- 可以记录 ship 自己创建的 worktree，并提供定向的 `ship clean`/doctor 修复。
- tag 在计划后即使被移动，本次 run 仍继续使用已经锁定的 commit，并记录 ref drift 警告。

## 9. 配置加载与执行根目录

### 9.1 两阶段配置加载

为保证 tag 当时的发布配方也被版本化，建议：

1. 从 InvocationRoot 加载最小 bootstrap 配置，用于解析版本策略、source mode 和必要覆盖。
2. 创建 SourceRoot。
3. 从 SourceRoot 重新加载 `ship.toml`，作为实际 release recipe。
4. 保留 CLI 与允许的环境变量覆盖，但版本和 commit 不再重新解析。

这样 Dockerfile、build context、hooks、templates、compose 配置与源码属于同一提交。

如果目标 tag 中没有 `ship.toml`，或者它与当前 ship 不兼容，应明确失败。未来如果支持“使用当前控制面配置构建旧源码”，必须是显式选项，不能静默回退。

### 9.2 路径分类

所有相对路径必须按语义解析，不能依赖进程当前目录碰巧是什么：

| 类型 | 根目录 | 示例 |
|---|---|---|
| 版本化源码输入 | SourceRoot | Dockerfile、context、hooks cwd、template source、compose、monitoring |
| 外部运行时输入 | InvocationRoot 或显式绝对路径 | 未跟踪 `.env`、凭据文件、签名密钥 |
| 持久运行状态 | StateRoot | `.ship/runs`、history、release manifest、locks |
| 临时构建输出 | Run workspace | 中间文件、临时 worktree、缓存元数据 |

需要特别处理 `build.docker.env_file` 和 `deploy.compose.local_env_file`：它们通常未被 Git 跟踪，因此不会自动出现在 worktree。ship 应在创建 worktree 前把这类外部输入解析为绝对路径，并在 manifest 中记录路径类型和内容摘要；不得记录 secret 明文。

## 10. 整条流水线必须绑定同一源码快照

不能只让 `docker build` 使用 SourceRoot。以下阶段都可能影响最终 release：

- prepare hooks
- templates
- local build
- Dockerfile 与 build context
- post-build hooks
- pre/post publish hooks
- compose 文件
- 部署前同步的配置文件
- command verify 使用的本地文件

原则是：

> 只要一个文件或命令会改变 artifact 或 deployment bundle，它就必须有明确、可记录的输入根目录。

网络健康检查等不读取本地源码的 verify action 可以不依赖 SourceRoot，但仍属于同一个 release plan。

## 11. 不可变发布与 `latest`

### 11.1 正式版本禁止静默覆盖

push 前应检查远端版本：

- 版本不存在：允许发布。
- 版本存在且 digest 相同：视为幂等成功。
- 版本存在且 digest 不同：默认拒绝，报告冲突。

不建议为正式 Git tag 提供普通的 `--force` 覆盖。真正需要修复时，应创建新版本；如果保留管理员逃生口，也必须要求二次确认并留下审计记录。

### 11.2 `latest` 只是显式 promotion alias

发布旧版本时不能把 `latest` 自动指回旧版本。推荐：

- 默认不维护 `latest`，生产 compose 使用具体版本或 digest。
- 只有显式 `--promote-latest` 才移动该别名。
- deploy、rollback 永远不修改 `latest`。
- matrix/profile 的 `latest-*` 也遵循相同原则。

### 11.3 本地临时镜像 tag

不要用共享的本地 `latest` 作为 build → tag → push 的阶段输入。建议使用包含 run ID 的临时 tag：

```text
<image>:ship-build-<run-id>-<profile>
```

这可以避免两个并发 run 互相覆盖本地镜像。

## 12. Release Manifest

每次成功 build/publish 应生成结构化 manifest，例如：

```json
{
  "schema": 1,
  "run_id": "01J...",
  "version": "v1.2.3",
  "source": {
    "ref": "refs/tags/v1.2.3",
    "commit": "0123456789abcdef...",
    "dirty": false
  },
  "artifacts": [
    {
      "type": "container-image",
      "profile": "default",
      "platform": "linux/amd64",
      "ref": "registry.example.com/team/app:v1.2.3",
      "digest": "sha256:..."
    }
  ]
}
```

manifest 是 publish、deploy、promotion、history 和 rollback 的共同事实来源。不能让这些阶段重新通过版本字符串猜测产物。

## 13. 独立命令与文件型产物

`ship run` 可以在发布完成后删除临时 worktree，因为所有阶段处于同一次 run 中。

独立执行 `ship build` 后再执行 `ship push` 时需要额外设计：

- Docker `--load` 后镜像存在于本地 daemon，但仍要用 run ID 和 manifest 找到它。
- Go binary、directory、command outputs 不能随着临时 worktree一起删除。

因此文件型产物必须：

1. 复制或移动到 `StateRoot/artifacts/<run-id>/`；
2. 计算 SHA-256；
3. 写入 manifest；
4. 后续 push/deploy 按 run/release manifest 消费。

第一阶段可以优先支持 Docker `ship run`，但内部模型不能假定所有产物都会永久存在于 Docker daemon。

## 14. Git、依赖与平台边界

### 14.1 本地 tag 与 fetch

- 默认只解析本地 tag，保证 plan 阶段无隐式网络副作用。
- tag 缺失时提示用户 fetch。
- 如果未来增加 `--fetch-tags`，必须作为显式联网行为。

### 14.2 Submodule 与 Git LFS

- 仓库包含 submodule 时，doctor/plan 必须报告其状态。
- 是否执行 `git submodule update --init --recursive` 应是显式策略，因为它可能访问网络。
- Git LFS 文件必须在 build 前验证已物化，避免把 pointer 文件构建进产物。

### 14.3 Windows

- 使用 `filepath` 管理本地路径，不手工拼接分隔符。
- 临时路径必须兼容 Docker Desktop 可访问的磁盘范围。
- worktree cleanup 应处理文件占用并给出占用路径，而不是静默遗留。
- PowerShell 仅用于用户 hook；Git 命令本身应通过参数数组执行，避免字符串转义问题。

## 15. 失败策略

以下情况必须在 build 前失败：

- `git-tag` 模式下版本不是现有 tag
- tag 无法解析为 commit
- worktree HEAD 与锁定 commit 不一致
- 目标 tag 的 release recipe 无法加载
- 必需外部输入不存在
- production 使用 current/dev/latest
- 已发布版本存在不同 digest

以下情况可以警告后继续：

- InvocationRoot dirty，但 tag 模式会忽略这些修改
- 当前 HEAD 领先所选 tag
- tag 在 plan 后发生 ref drift，但锁定 commit 未变化
- 临时 worktree cleanup 失败；主操作结果必须保留，同时给出人工清理方式

错误信息至少包含 version、ref、commit、run ID 和相关路径，便于复现。

## 16. 推荐执行架构

```text
InvocationRoot/ship.toml
        |
        v
Bootstrap config + CLI overrides
        |
        v
Resolve ReleaseIdentity (version/ref/commit)
        |
        v
Create detached SourceRoot worktree
        |
        v
Load release recipe from SourceRoot
        |
        v
Compile immutable Release Plan
        |
        v
Build -> Artifact Manifest -> Publish -> Deploy -> Verify
        |
        v
Persist run state in StateRoot -> Cleanup SourceRoot
```

不要在各 command 中零散添加 `os.Chdir`。v3 的 Config Compiler、Release Plan、Executor、Artifact Module 应共同消费显式 Execution Context。

在 v2 兼容实现中，如果为了控制改造范围暂时使用进程级 cwd 切换，也必须：

- 保存并恢复原 cwd；
- 把 StateRoot 固定为 InvocationRoot 下的 `.ship`；
- 在单线程执行边界内使用；
- 把这种实现标记为过渡层，不扩散到 driver API。

## 17. 分阶段实施计划

### 阶段一：建立正确性护栏

- 引入 `ReleaseIdentity` 和 `ExecutionRoots`。
- `git-tag` 模式严格验证 tag 并锁定 commit。
- Docker `ship run` 在 detached worktree 执行。
- 输出 version/ref/commit/HEAD delta。
- 外部 env 文件使用绝对路径。
- 本地临时镜像 tag 使用 run ID。
- 默认禁止覆盖远端同版本不同 digest。
- deploy/rollback 不修改 `latest`。

### 阶段二：进入 Plan 与 Artifact 模型

- 两阶段配置加载。
- 整条流水线绑定 SourceRoot。
- 生成 release/artifact manifest。
- publish/deploy 按 manifest 消费产物。
- history 记录 commit、digest、environment 和 run ID。
- 增加 `ship plan`/`ship doctor` 对 source snapshot 的检查。

### 阶段三：完整产物与 promotion

- 文件型 artifact 持久化。
- 多 profile/platform manifest。
- 按 digest 部署。
- staging 到 production 的同产物 promotion。
- submodule、Git LFS 和签名 tag 策略。
- SBOM、provenance 与更高等级的可重复性验证。

## 18. 验收测试

至少覆盖以下自动化场景：

1. HEAD 领先 tag：镜像内容来自 tag，而不是 HEAD。
2. InvocationRoot dirty：tag 构建不包含未提交修改。
3. 显式旧 tag：构建旧 tag 对应源码，不移动当前分支。
4. 不存在的 tag：在 Docker build 前失败。
5. annotated/lightweight tag：都解析到正确 commit。
6. tag 与 branch 同名：只接受 `refs/tags/...`。
7. tag 在 plan 后移动：本次仍使用锁定 commit，并产生警告。
8. worktree 创建、构建、发布、验证任一阶段失败：都执行安全 cleanup。
9. 两个并发 run：worktree 和本地镜像 tag 不冲突。
10. 外部 `.env` 不在 tag 中：仍能按 InvocationRoot 规则解析，并且日志不泄露内容。
11. tag 中 compose 与 HEAD 不同：部署使用目标 release 的物料。
12. 远端版本 digest 相同：幂等成功。
13. 远端版本 digest 不同：拒绝覆盖。
14. 部署旧版本：不重新 build、不更新 `latest`。
15. current/dev 模式：版本包含 commit，production policy 拒绝使用。
16. Go/custom 文件产物：worktree 清理后 artifact 仍可发布。

## 19. 成功标准

当该策略完成后，ship 应能可靠回答三个问题：

1. **这个版本来自哪份源码？**——精确到 Git commit。
2. **服务器实际运行哪个产物？**——精确到 artifact digest。
3. **能否再次部署同一产物？**——通过 release manifest 直接部署，不重新构建。

最终用户体验应保持简单：

```powershell
ship run -v v1.2.3 -y
ship deploy -v v1.2.3 -y
```

复杂性由 ship 内部的 ReleaseIdentity、ExecutionRoots、Plan 和 Artifact Manifest 承担，而不是转嫁给每个项目的脚本和操作者。
