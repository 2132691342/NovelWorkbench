# 测试说明

本文说明 NovelWorkbench 的测试策略、保留的用例与集成验证方法。测试的原则是**少而准**：只保留覆盖长链路与高风险配置的用例，其余交给类型检查与集成验证。

## 1. 测试策略

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| Go 单元测试（`go test -short`） | 跨步骤链路：访问控制、密钥封存、技能解析、上下文装配、流式事件契约、检索融合、索引迁移、菜单权限、内置资产完整性 | CRUD 字段透传、JSON 回显、常量查表、SQL 细节、单函数分支穷举 |
| 前端单元测试（`vitest run`） | 纯逻辑链路：流式报文解析与事件归并、表单校验、请求客户端 | 组件渲染、样式存在性、路由跳转 |
| 类型检查与构建（`vue-tsc && vite build`） | TypeScript 类型与生产构建 | -- |
| 集成验证 | 需要中间件与模型网关的完整业务链路 | 逐个控件的机械点击 |

原则：

- **一个用例讲一条链路**。多个环节串在同一 `Test*` 内，用「步骤N」注释分隔；失败时看注释定位到具体环节。
- **只测有决策价值的分支**。边界值、非法输入、空集合挑代表性的测，不做穷举。
- 表驱动只用于同一行为的多个输入（如鉴权的拒绝路径）。
- `-short` 模式不依赖外部服务：HTTP 依赖用 `httptest.NewServer`，接口用闭包替身。
- 不使用 `time.Sleep`；需要顺序时用确定的输入顺序表达。
- 断言面向可观察结果：业务码、HTTP 状态、包含关系、顺序、条数。
- 业务规则写成无依赖纯函数（可见性过滤、作用域优先级、变量取值、关键词命中、预算裁剪），测试直接驱动，不引入数据库或 mock 框架。
- 不测"接线"（路由是否注册、字段是否透传），这类偏差由类型检查与集成验证兜住。

## 2. 后端测试

### 2.1 运行

```bash
cd backend
CGO_ENABLED=0 go test -short -count=1 -shuffle=on ./...
```

CI（`.github/workflows/ci.yml` 的 backend job）依次执行 `go vet ./...`、
`CGO_ENABLED=0 go build -tags "netgo osusergo" ./...` 与上述测试命令，环境变量为
`GOTOOLCHAIN=local`（与 Dockerfile 一致）与 `CGO_ENABLED=0`。
单条用例定位：

```bash
go test -short -run TestChatContextChain ./internal/service/   # 定位单个链路
```

### 2.2 用例清单

| 文件 | 用例 | 覆盖链路 |
| --- | --- | --- |
| `backend/internal/pkg/security_chain_test.go` | `TestAccessControlChain` | 请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库 → 响应包络；拒绝路径覆盖无令牌 / 令牌已吊销 / 账号禁用（401）与权限不足（403） |
| `backend/internal/pkg/security_chain_test.go` | `TestSecretSealChain` | 密钥封存 → 解封还原 → 密文不可读 → 篡改与材料变更被拒（认证标签校验） |
| `backend/internal/ai/ai_chain_test.go` | `TestBuiltinAssetIntegrity` | 内置资产矩阵：9 个流程域各一档案两技能、编码全局唯一、模板占位符全部可解析、内容指纹稳定、默认档案是 `novelist` |
| `backend/internal/ai/ai_chain_test.go` | `TestProviderInstancePolicy` | 模型实例缓存键随密钥/协议/地址/模型/参数变化；流式 HTTP 客户端不设整体超时（否则长流被拦腰切断） |
| `backend/internal/service/chat_pipeline_test.go` | `TestSkillResolutionChain` | 技能编码解析（显式优先 / 正文首 token）→ 作用域优先级 → 变量取值与必填缺失 → 模板占位符校验 → 参考资料关键词命中与 4000 字符预算 → 全局技能覆盖标记 |
| `backend/internal/service/chat_pipeline_test.go` | `TestChatContextChain` | 技能模板渲染 → 自由对话 → @引用分优先级 → 设定注入排除私人笔记 → 显式引用摘要与档案 → 同优先级稳定顺序 → 12000 字符预算丢块 → 历史选取 → 消息序列成型 → 显式引用会话归属校验 |
| `backend/internal/service/chat_pipeline_test.go` | `TestChatStreamContract` | SSE `thinking` → `chunk` → `done` 顺序、seq 从 1 单调递增、`done` 携带完整思考过程与空数组引用 |
| `backend/internal/service/knowledge_chain_test.go` | `TestKnowledgeRetrievalChain` | 文本切片（重叠、边界、非法参数）→ 查询词抽取 → RRF 双通道融合 → 单通道排序与 TopK → 重排解析与 fail-open |
| `backend/internal/service/knowledge_chain_test.go` | `TestManticoreSchemaMigrationChain` | Manticore 表结构识别 → 旧表重建 → 回灌只执行一次；新结构启动不重建 |
| `backend/internal/service/menu_chain_test.go` | `TestMenuAndPermissionChain` | 导航可见性过滤 → 菜单树装配 → 权限目录汇总 → 重复权限码去重 → 分组编码与展示名 |

### 2.3 覆盖取向

保留的用例集中在三处：

1. **权限与凭据**。鉴权绕过与密钥解不开都是「配错才在生产暴露」的问题。
2. **上下文装配**。技能、引用、设定、摘要、历史与预算的交互是全系统最复杂的部分，一旦偏移会静默产出错误上下文。
3. **检索链路**。切片、查询词、融合与重排是一条上下游链路，中途改动会直接影响召回质量。

其余（CRUD 透传、字段校验、常量查表）由类型检查与集成验证兜住，不单独建用例。

## 3. 前端测试

### 3.1 运行

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm test # vitest run
```

`frontend/vitest.config.ts`：`environment: 'node'`，`include` 覆盖 `src/**/*.test.ts` 与
`packages/*/test/**/*.test.ts`，复用 `@` 别名。

### 3.2 用例清单

| 文件 | 用例 | 覆盖链路 |
| --- | --- | --- |
| `frontend/src/views/Workspace/streamChain.test.ts` | `parseSseBuffer 报文解析链路` | 跨 chunk 断行、多事件粘包、CRLF、非法 JSON 跳过、心跳注释忽略、尾部半行留存 |
| `frontend/src/views/Workspace/streamChain.test.ts` | `applyStreamEvent 事件归并链路` | 思考与正文分别累加、`done` 用服务端全量覆盖并补齐召回依据、`error` 落兜底文案、未知事件不改写消息 |
| `frontend/packages/hooks/test/useForm.test.ts` | `useForm 表单链路` | 规则校验 → 校验失败不提交并记录错误 → 脏状态 → 联动规则单独校验 → 服务端错误回填与清除 → 校验通过才提交 → 重置回到初值 |
| `frontend/packages/request/test/request.test.ts` | `请求客户端链路` | 前置钩子注入头部 → 响应解包 → 业务错误转交 `onError` → 同键并发请求一并取消 |

组件渲染与样式存在性不设断言；类型正确性与生产产物由 `pnpm build` 保证，CI 与镜像构建都执行它。

## 4. 集成验证

需要真实中间件与模型网关的链路无法在单元测试中覆盖，通过部署后的实际请求验证：

| 场景 | 验证方式 |
| --- | --- |
| 启动与播种 | 后端启动日志显示迁移 33 张表、播种内置技能与菜单；`/health` 与 `/ready` 返回 200 |
| 登录与导航 | `POST /api/v1/auth/login` 取令牌 → `GET /api/v1/menu/nav` 返回按权限过滤后的导航树 |
| 业务接口 | 按模块调用列表/详情/写入接口，确认业务码为 `SUCCESS` 且字段符合契约 |
| AI 流式 | `POST /api/v1/chat/send` 观察 `thinking` → `chunk` → `done` 事件序列与 seq 递增 |
| 检索 | 上传文档 → 索引任务完成（`indexed`）→ 关键词检索返回命中片段与追踪信息 |
| 前端 | 浏览器逐页访问，检查渲染、空态、加载态与流式输出 |

集成验证不在 CI 中自动执行（依赖外部中间件与模型网关），由部署阶段人工确认。

## 5. CI 门禁

| 流水线 | 门禁 |
| --- | --- |
| `ci.yml` backend | `go vet ./...`、`CGO_ENABLED=0 go build`、`go test -short -count=1 -shuffle=on ./...` |
| `ci.yml` frontend | `pnpm install --frozen-lockfile`、`pnpm test`、`pnpm build`（含 `vue-tsc --noEmit`） |
| `ci.yml` docker | 后端与前端镜像构建 |
| `release.yml` | tag 推送时构建并推送镜像到 GHCR |

CI 只跑 `go test -short` 与 `pnpm test`，不连接外部服务。
