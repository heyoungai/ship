# ship 设计原则与边界

这份文档不是实现细节说明，而是 ship 后续演进时必须遵守的产品边界。

## 1. 产品定位

ship 是一个发布执行器，不是一个通用任务编排平台。

它的核心职责是：

- 统一项目发布语义
- 把常见发布链路沉淀为内置 driver
- 让项目差异尽量配置化，而不是脚本化

它不负责：

- 替代 GitHub Actions / GitLab CI / Jenkins 这类 CI 平台
- 替代 Docker、Go build、pnpm、pytest 这类具体构建工具
- 提供一套无限扩展的通用 DSL

推荐职责分工：

- CI 平台负责什么时候发布
- ship 负责怎么发布
- 具体构建工具负责产物生成
- 目标环境工具负责最终运行

## 2. 一等公民场景

ship 的一等公民是 Docker 发布链路。

默认服务的核心场景是：

```text
docker build -> push registry -> ssh -> docker compose up -> verify
```

这条链路覆盖大多数前后端服务、自建服务器项目，也是 ship 的默认推荐模式。

Go 二进制发布链路是次一级但明确支持的主链路：

```text
go build -> scp -> remote install -> verify
```

## 3. 设计原则

后续所有功能演进都应遵守下面几条优先级：

1. ship 的一等公民是 Docker 发布链路
2. ship 的核心价值是统一发布语义，不是统一所有脚本
3. 内置 driver 优先于 hooks
4. hooks 优先于通用 DSL
5. 配置文件优先表达发布意图，不要表达任意过程编排

从实现角度理解，就是：

- 先扩展 `docker / registry / compose / verify` 这些稳定 driver
- driver 覆盖不了的少量差异，再交给 `steps.*` 和 `templates`
- 不要为了少数边缘场景把 hooks 扩展成脚本平台

## 4. 对 Taskfile 的态度

ship 可以借鉴 Taskfile，但不应该内嵌 Taskfile 语义。

### 应该借鉴的点

- 任务命名和分层清晰
- 变量覆盖的直觉性
- 本地开发体验友好
- 对常见命令封装轻量

### 不应该引入的点

- 重型 DSL
- 通用任务依赖图
- includes / imports 式复杂配置体系
- 把 ship 扩展成通用 task runner

推荐关系是：

- Taskfile 负责开发任务标准化
- ship 负责发布任务标准化

可以有这样的调用方式：

```text
task release -> ship run --yes
```

但不推荐把 Taskfile 的通用语义直接嵌进 ship。

## 5. drivers、hooks、templates 的边界

### drivers

drivers 是 ship 的主能力。

理想状态下，常见项目只需要选择：

- `build.driver = docker`
- `publish.driver = registry`
- `deploy.driver = compose`
- `verify.driver = http`

用户不应该先想到 hooks，而应该先想到 driver。

### hooks

`steps.prepare`、`steps.pre_publish`、`steps.pre_deploy` 等 hooks 只用来处理主链路无法覆盖的项目差异。

适合的场景：

- 前端需要先构建静态产物
- 后端需要先 publish 到 dist
- 部署前后需要做一次轻量命令

不适合的场景：

- 用 hooks 重写完整发布流程
- 用 hooks 模拟一个通用流程编排系统

### templates

templates 是发布物料生成的逃生口。

适合的场景：

- 生成 `.env`
- 生成运行时 Dockerfile
- 生成 docker-compose 文件

不适合的场景：

- 用模板系统承载复杂业务逻辑
- 把配置文件变成另一个模板语言平台

## 6. 后续优化优先级

继续优化 ship 时，优先顺序应该是：

1. 提升内置 drivers 稳定性
2. 补齐 Docker / Registry / Compose 主链路覆盖
3. 优化默认推荐模式的接入体验
4. 提升错误信息、验证反馈、CI 友好性
5. 最后才考虑 hooks / templates 的小范围增强

具体建议：

- 优先补 `docker / registry / compose / verify` 场景
- 优先提升可观测性、失败提示、参数覆盖直觉性
- 优先把“标准范式”做稳，而不是继续发散模型

## 7. 明确不做的方向

为了控制复杂度，ship 应明确避免滑向这些方向：

- 通用表达式引擎
- 任意 DAG 编排
- 插件系统优先
- 无限泛化的模板变量体系
- 以 hooks 替代 driver
- 把 ship 变成另一个 CI 平台

## 8. 一句话总结

ship 不是为了统一所有脚本，而是为了把最常见、最稳定、最值得复用的发布链路产品化。

如果后续需求和这个目标冲突，优先保护主链路的清晰性，而不是继续泛化能力。