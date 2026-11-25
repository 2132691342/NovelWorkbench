# 测试说明

本文说明 NovelWorkbench 的测试策略、保留的链路与集成验证方式。

## 1. 策略

测试只保留**长链路**用例：一条用例走完一条真实业务链路，用「步骤N」标注环节，断点即定位。

判据是「配错才在生产暴露」。鉴权绕过、密钥解不开、上下文装配偏移、检索召回失真、导航路由拼错——这些静默出错、影响面大的问题值得一条用例；CRUD 字段透传、常量查表、JSON 回显、组件渲染、样式存在性这些由类型检查、编译与集成验证兜住的偏差，不建用例。

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| Go 单元测试 | 跨环节链路：请求管线、密钥封存、对话装配、历史归档、检索融合、索引迁移、权限变更、导航装配、内置资产、开书解析、模型网关、体检门禁 | CRUD 透传、JSON 回显、查表、SQL 细节 |
| 前端单元测试 | SSE 报文解析与事件归并、斜杠命令注册表、路由装配 | 组件渲染、样式、路由跳转 |
| 类型检查与构建 | TypeScript 类型与生产构建 | -- |
| 集成验证 | 需要中间件与模型网关的完整业务链路 | 逐个控件的机械点击 |

原则：

- **一条用例一条链路**。同域环节串在同一个 `Test*` 内，用「步骤N」或「环节N」分隔。
- **断言跨环节因果**，不重复断言代码自身保证的约束。例：断言「项目提示词覆盖全局后全局提示词被标记为已覆盖」，而不是逐字段核对枚举取值表。
- **静态数据不单独断言**。内置资产的数量、编码唯一性这类改动当场就能看出的事实，用一次遍历顺带校验即可，不为它单开用例。
- **不测确定性**。同一输入两次调用结果一致是纯函数的基本性质，断言它只是让用例变长。
- **不测接线**。路由是否注册、字段是否透传由类型检查与集成验证兜住。
- `-short` 模式不依赖外部服务：HTTP 依赖用 `httptest.NewServer`，接口用闭包替身。
- 不使用 `time.Sleep`；需要顺序时用确定的输入顺序表达。
- 业务规则写成无依赖纯函数，测试直接驱动，不引入数据库与 mock 框架。

**文件命名与切分原则**：测试文件按「被测能力的归属」而不是按「断言条数」切分——同一能力的多个断言必须在同一个文件里，否则改一处逻辑要同时开多个文件、跑多轮测试才敢确认没坏。一个文件内可以有多条用例，但每条用例必须自成一条链路；跨能力的断言不得混在同一条用例里（失败时无法判断是哪一段坏了）。当前后端 7 个测试文件、13 条用例，前端 2 个测试文件、4 条用例。

## 2. 后端

### 2.1 运行

```bash
cd backend
CGO_ENABLED=0 go test -short -count=1 -shuffle=on ./...
```

CI 的 backend job 依次执行 `go vet ./...`、`CGO_ENABLED=0 go build -tags "netgo osusergo" ./...` 与上述测试命令，环境变量为 `GOTOOLCHAIN=local`（与 Dockerfile 一致）与 `CGO_ENABLED=0`。

定位单条链路：

```bash
go test -short -run TestChatAssemblyChain ./internal/service/
```

### 2.2 用例清单

| 文件 | 用例 | 链路环节 |
| --- | --- | --- |
| `internal/pkg/request_chain_test.go` | `TestRequestPipelineChain` | 放行：请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库（请求体 + 响应体）→ 响应包络<br>拒绝：无令牌 / 令牌已吊销 / 账号禁用（401）与权限不足（403） |
| `internal/pkg/request_chain_test.go` | `TestIdempotencyChain` | 首请求只执行一次 → 同键同体重放首响应（`Idempotent-Replay`）→ 同键不同体拒绝 → 5xx 不缓存允许同键重试 → 未带键的请求不受影响 |
| `internal/pkg/request_chain_test.go` | `TestSecretSealChain` | 封存 → 解封还原 → 同明文密文不同（nonce 随机）→ 篡改与材料变更被拒（认证标签校验） |
| `internal/service/access_control_chain_test.go` | `TestPermissionChain` | 权限码规范化（去重保序 / 空白剔除 / 非管理员禁用 `*` / 未登记报错）→ 内置角色脏数据修复（`user` 剔除 `*`、`admin` 固定 `*`）→ 权限码下线时从角色剔除 |
| `internal/service/access_control_chain_test.go` | `TestNavigationChain` | 种子组件路径在前端有对应页面 + 编码与路由唯一 + 个人中心隐藏页登记 → 可见性过滤（隐藏页保留标记）→ 菜单树装配 → 权限目录汇总与权限码去重 |
| `internal/service/chat_assembly_chain_test.go` | `TestChatAssemblyChain` | 斜杠命令解析 → 提示词作用域覆盖与停用回退 → 全局覆盖标记 → 占位符校验 → 上下文块优先级与私人笔记排除 → 显式引用摘要 → 装配不丢块 → 消息序列成型（提示词为系统级）→ @ 素材来源 → 工具声明与参数解码 → 流式工具调用归并 → 流式事件契约 |
| `internal/service/chat_history_chain_test.go` | `TestHistoryChain` | 历史全量注入（条数不截取）→ 单条超长首尾收敛 → 归档逐条编号（条数、角色、时间、工具标签）→ 归档不丢条 |
| `internal/service/knowledge_chain_test.go` | `TestKnowledgeRetrievalChain` | 文本切片（重叠窗口、非法 overlap 收敛）→ 查询词抽取 → RRF 双通道融合 → 单通道排序与 TopK → 重排解析与 fail-open |
| `internal/service/knowledge_chain_test.go` | `TestManticoreSchemaMigrationChain` | 表结构识别 → 旧表重建 → 回灌只执行一次；新结构启动不重建 |
| `internal/ai/ai_chain_test.go` | `TestBuiltinAssetChain` | 域形状（每域一身份两指令）→ 身份与指令编码全局唯一 → 占位符全部登记 |
| `internal/ai/ai_chain_test.go` | `TestOpenBookPlanChain` | 代码围栏与前后缀文字容错 → 书名/卷章/角色字段完整取出 → 无 JSON / 缺书名 / 缺卷章 / 非法 JSON 一律报错 |
| `internal/ai/ai_chain_test.go` | `TestModelGatewayChain` | 实例缓存键随密钥/协议/地址/模型/参数/超时变化 → 流式 HTTP 客户端不设整体超时（否则长流被拦腰切断） |
| `internal/ai/check/check_chain_test.go` | `TestChapterGateChain` | 空正文判阻断 → 零容忍词计数与分级 → 缺钩子警示 → 综合分扣分 → 干净正文不误判 |

### 2.3 为什么这样切分

- **按能力归文件**。请求管线（鉴权 / 权限 / 审计 / 幂等 / 密钥）都在 `pkg` 层，改中间件顺序时只动一个测试文件；权限与导航都在「访问控制」这条业务链上，合并后失败时能立刻判断是权限数据错了还是导航装配错了。
- **对话与历史分开**。对话装配要注入上下文、提示词、工具；历史处理是完全本地的文本拼装，没有数据耦合，单独成文件后可以只跑这一条来验证历史策略。
- **检索与索引迁移分开**。索引迁移发生在每次启动、会重建已有索引，必须能单独守住，不能与检索链路绑在一起。
- **体检确定性不单独断言**。`Analyze` 是纯函数，同输入同输出是语言保证，不需要占一行断言。

## 3. 前端

### 3.1 运行

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm test # vitest run
```

`frontend/vitest.config.ts`：`environment: 'node'`，`include` 覆盖 `src/**/*.test.ts`，复用 `@` 别名。

### 3.2 用例清单

| 文件 | 用例 | 链路环节 |
| --- | --- | --- |
| `src/views/Workspace/workspaceChain.test.ts` | 报文解析 | 跨 chunk 断行、多事件粘包、CRLF、非法 JSON 跳过、心跳注释忽略、尾部半行留存 |
| `src/views/Workspace/workspaceChain.test.ts` | 事件归并 | 思考与正文分别累加、`done` 用服务端全量覆盖并补齐召回依据与工具、`error` 落兜底文案、未知事件不改写消息 |
| `src/views/Workspace/workspaceChain.test.ts` | 斜杠命令 | 首 token 解析与正文剥离 → 编码唯一且可按编码取回 → 关键词过滤命中编码与名称 → 动作编码不与内置提示词编码冲突 |
| `src/utils/routerChain.test.ts` | 路由装配 | 路径拼接不产生结尾斜杠 → 绝对路径与外部链接原样返回 → 路由名唯一 → `:project_ulid` 占位替换与无项目时保留占位 |

工作台的三段断言合在一个文件：它们都是「作者发出指令、模型流式回话」这一条链路的前后端两半，拆开跑会失去「命令解析口径与后端一致」这一关键交叉验证。

组件渲染与样式存在性不设断言；类型正确性与生产产物由 `pnpm build` 保证，CI 与镜像构建都执行它。

## 4. 集成验证

需要真实中间件与模型网关的链路无法在单元测试中覆盖，通过部署后的实际请求验证：

| 场景 | 验证方式 |
| --- | --- |
| 启动与播种 | 后端启动日志显示迁移 33 张表、播种内置提示词与菜单；`/health` 与 `/ready` 返回 200 |
| 登录与导航 | `POST /api/v1/auth/login` 取令牌 → `GET /api/v1/menu/nav` 返回按权限过滤后的导航树 |
| 业务接口 | 按模块调用列表/详情/写入接口，确认业务码为 `SUCCESS` 且字段符合契约 |
| AI 流式 | `POST /api/v1/chat/send` 观察 `thinking` → `chunk` → `done` 事件序列与 seq 递增 |
| AI 开书 | `POST /api/v1/project/bootstrap/plan` 返回方案（卷章/角色/设定齐全）→ `POST /project/bootstrap/apply` 后 `GET /project/tree/:id` 与 `POST /outline/list` 能查到对应卷章与大纲节点 |
| 检索 | 上传文档 → 索引任务完成（`indexed`）→ 关键词检索返回命中片段与追踪信息 |
| 历史全量注入 | 长会话中追问「最开始聊的是什么」，回答应覆盖会话最早的内容（滑窗实现会答不出） |
| 审计闭环 | 任意写接口调用后 `POST /admin/audit/list` 能查到该记录，`GET /admin/audit/detail/:id` 返回脱敏请求体、响应码**与响应体** |
| 角色权限闭环 | 编辑「普通用户」角色取消一项权限 → 保存 → 重新打开弹窗，勾选状态与列表权限数一致；连改两次仍可继续编辑 |
| 幂等键 | 同一 `Idempotency-Key` 连发两次 `POST /admin/role/save`：第二次响应带 `Idempotent-Replay: true`、`trace_id` 与首响应相同，库里只有一行 |
| 查询不被缓存 | 写完数据后带同一 `Idempotency-Key` 调两次 `POST /admin/role/list`，第二次必须读到新数据 |
| 删除幂等 | 同一角色删两次，两次都返回 `SUCCESS`；越权删除他人资源仍返回 `ERR_FORBIDDEN` |
| 改密失效 | 改密后旧令牌访问任意接口返回 401「密码已变更」，改密响应里的新令牌可正常访问 |
| 工具 | `GET /tool/list` 返回 6 个内置工具；`POST /tool/invoke`（`chapter_check`）对含零容忍词的正文返回 `verdict_code=block`；`POST /tool/save` 返回值含真实 `updated_at` |
| 前端 | 浏览器逐页访问，检查渲染、空态、加载态与流式输出；写作台点「体检」应出现评分与问题清单 |

集成验证不在 CI 中自动执行（依赖外部中间件与模型网关），由部署阶段人工确认。

## 5. CI 门禁

| 流水线 | 门禁 |
| --- | --- |
| `ci.yml` backend | `go vet ./...`、`CGO_ENABLED=0 go build`、`go test -short -count=1 -shuffle=on ./...` |
| `ci.yml` frontend | `pnpm install --frozen-lockfile`、`pnpm test`、`pnpm build`（含 `vue-tsc --noEmit`） |
| `ci.yml` docker | 后端与前端镜像构建 |
| `release.yml` | tag 推送时构建并推送镜像到 GHCR |

CI 只跑 `go test -short` 与 `pnpm test`，不连接外部服务。
