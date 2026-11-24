# 测试说明

本文说明 NovelWorkbench 的测试策略、保留的链路与集成验证方式。

## 1. 策略

测试只保留**长链路**用例：一条用例走完一条真实业务链路，用「步骤N」标注环节，断点即定位。

判据是「配错才在生产暴露」。鉴权绕过、密钥解不开、上下文装配偏移、检索召回失真、导航路由拼错——这些静默出错、影响面大的问题值得一条用例；CRUD 字段透传、常量查表、JSON 回显、组件渲染、样式存在性这些由类型检查、编译与集成验证兜住的偏差，不建用例。

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| Go 单元测试 | 跨环节链路：访问控制、密钥封存、技能解析、上下文装配、工具触发、流式契约、检索融合、索引迁移、导航装配、内置资产、开书解析、模型网关、体检门禁 | CRUD 透传、JSON 回显、查表、SQL 细节 |
| 前端单元测试 | SSE 报文解析与事件归并、路由装配 | 组件渲染、样式、路由跳转 |
| 类型检查与构建 | TypeScript 类型与生产构建 | -- |
| 集成验证 | 需要中间件与模型网关的完整业务链路 | 逐个控件的机械点击 |

原则：

- **一条用例一条链路**。同域环节串在同一个 `Test*` 内，用「步骤N」或「环节N」分隔。
- **断言跨环节因果**，不重复断言代码自身保证的约束。例：断言「项目技能覆盖全局后全局技能被标记为已覆盖」，而不是逐字段核对枚举取值表。
- **静态数据不单独断言**。内置资产的数量、编码唯一性这类改动当场就能看出的事实，用一次遍历顺带校验即可，不为它单开用例。
- **不测确定性**。同一输入两次调用结果一致是纯函数的基本性质，断言它只是让用例变长。
- **不测接线**。路由是否注册、字段是否透传由类型检查与集成验证兜住。
- `-short` 模式不依赖外部服务：HTTP 依赖用 `httptest.NewServer`，接口用闭包替身。
- 不使用 `time.Sleep`；需要顺序时用确定的输入顺序表达。
- 业务规则写成无依赖纯函数，测试直接驱动，不引入数据库与 mock 框架。

## 2. 后端

### 2.1 运行

```bash
cd backend
CGO_ENABLED=0 go test -short -count=1 -shuffle=on ./...
```

CI 的 backend job 依次执行 `go vet ./...`、`CGO_ENABLED=0 go build -tags "netgo osusergo" ./...` 与上述测试命令，环境变量为 `GOTOOLCHAIN=local`（与 Dockerfile 一致）与 `CGO_ENABLED=0`。

定位单条链路：

```bash
go test -short -run TestChatContextChain ./internal/service/
```

### 2.2 用例清单

后端 7 个测试文件、11 条用例，每个文件一条链路。

| 文件 | 用例 | 链路环节 |
| --- | --- | --- |
| `internal/pkg/security_chain_test.go` | `TestAccessControlChain` | 放行：请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库 → 响应包络<br>拒绝：无令牌 / 令牌已吊销 / 账号禁用（401）与权限不足（403） |
| `internal/pkg/security_chain_test.go` | `TestSecretSealChain` | 封存 → 解封还原 → 同明文密文不同（nonce 随机）→ 篡改与材料变更被拒（认证标签校验） |
| `internal/ai/ai_chain_test.go` | `TestBuiltinAssetChain` | 域形状（每域一档案两技能）→ 档案与技能编码全局唯一 → 模板占位符全部登记 → 指纹格式稳定 |
| `internal/ai/ai_chain_test.go` | `TestOpenBookPlanChain` | 代码围栏与前后缀文字容错 → 书名/卷章/角色字段完整取出 → 无 JSON / 缺书名 / 缺卷章 / 非法 JSON 一律报错 |
| `internal/ai/ai_chain_test.go` | `TestModelGatewayChain` | 实例缓存键随密钥/协议/地址/模型/参数/超时变化 → 流式 HTTP 客户端不设整体超时（否则长流被拦腰切断） |
| `internal/ai/check/check_chain_test.go` | `TestChapterGateChain` | 空正文判阻断 → 零容忍词计数与分级 → 缺钩子警示 → 综合分扣分 → 干净正文不误判 |
| `internal/service/chat_chain_test.go` | `TestSkillResolutionChain` | 编码解析（显式优先 / 正文首 token）→ 作用域优先级与停用回退 → 变量取值与必填缺失 → 模板占位符校验 → 参考资料命中与预算 → 全局覆盖标记 |
| `internal/service/chat_chain_test.go` | `TestChatContextChain` | 技能模板渲染 → 自由对话 → @引用分优先级 → 设定注入排除私人笔记 → 显式引用摘要与档案 → 同优先级稳定顺序 → 预算裁剪丢块 → 历史选取 → 消息序列成型 → 引用会话归属校验 |
| `internal/service/chat_chain_test.go` | `TestToolTriggerChain` | 正文 `#code` 解析（行首/行中命中，词内井号与无井号不命中）→ 编码合并去重截断 → 显式选取与缺失上报 → 关键词补位跳过已选与停用 |
| `internal/service/chat_chain_test.go` | `TestChatStreamContract` | SSE `thinking` → `chunk` → `done` 顺序、seq 单调递增、`done` 携带完整思考过程、空数组引用与本轮工具编码 |
| `internal/service/knowledge_chain_test.go` | `TestKnowledgeRetrievalChain` | 文本切片（重叠窗口、非法 overlap 收敛）→ 查询词抽取 → RRF 双通道融合 → 单通道排序与 TopK → 重排解析与 fail-open |
| `internal/service/knowledge_chain_test.go` | `TestManticoreSchemaMigrationChain` | 表结构识别 → 旧表重建 → 回灌只执行一次；新结构启动不重建 |
| `internal/service/navigation_chain_test.go` | `TestNavigationChain` | 种子组件路径在前端有对应页面 + 编码与路由唯一 + 个人中心隐藏页登记 → 可见性过滤（隐藏页保留标记）→ 菜单树装配 → 权限目录汇总与权限码去重 |

### 2.3 为什么这样切分

- **同域环节合并**。对话链路的四段（技能解析、上下文装配、工具触发、流式契约）都发生在同一次请求里，拆到多个文件会让定位变难，因此集中在 `chat_chain_test.go`。
- **导航种子的静态校验并入导航链路**。种子是否可注册、编码是否唯一，只有与「过滤 → 装配 → 目录」放在一条用例里才有意义——它们是导航请求的同一段。
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

前端 2 个测试文件、3 条用例，每条一条链路。

| 文件 | 用例 | 链路环节 |
| --- | --- | --- |
| `src/views/Workspace/chatStreamChain.test.ts` | 报文解析 | 跨 chunk 断行、多事件粘包、CRLF、非法 JSON 跳过、心跳注释忽略、尾部半行留存 |
| `src/views/Workspace/chatStreamChain.test.ts` | 事件归并 | 思考与正文分别累加、`done` 用服务端全量覆盖并补齐召回依据与工具、`error` 落兜底文案、未知事件不改写消息 |
| `src/utils/routerChain.test.ts` | 路由装配 | 路径拼接不产生结尾斜杠 → 绝对路径与外部链接原样返回 → 路由名唯一 → `:project_ulid` 占位替换与无项目时保留占位 |

两条链路对应两个真实故障：解析与归并偏差会让 AI 回复重复累加或思考被清空；路径拼接多一个结尾斜杠会让 strict 路由整片落到 404，而控制台不会报任何错。

组件渲染与样式存在性不设断言；类型正确性与生产产物由 `pnpm build` 保证，CI 与镜像构建都执行它。

## 4. 集成验证

需要真实中间件与模型网关的链路无法在单元测试中覆盖，通过部署后的实际请求验证：

| 场景 | 验证方式 |
| --- | --- |
| 启动与播种 | 后端启动日志显示迁移 35 张表、播种内置技能与菜单；`/health` 与 `/ready` 返回 200 |
| 登录与导航 | `POST /api/v1/auth/login` 取令牌 → `GET /api/v1/menu/nav` 返回按权限过滤后的导航树 |
| 业务接口 | 按模块调用列表/详情/写入接口，确认业务码为 `SUCCESS` 且字段符合契约 |
| AI 流式 | `POST /api/v1/chat/send` 观察 `thinking` → `chunk` → `done` 事件序列与 seq 递增 |
| AI 开书 | `POST /api/v1/project/bootstrap/plan` 返回方案（卷章/角色/设定齐全）→ `POST /project/bootstrap/apply` 后 `GET /project/tree/:id` 与 `POST /outline/list` 能查到对应卷章与大纲节点 |
| 检索 | 上传文档 → 索引任务完成（`indexed`）→ 关键词检索返回命中片段与追踪信息 |
| 审计闭环 | 任意写接口调用后 `POST /admin/audit/list` 能查到该记录，`GET /admin/audit/detail/:id` 返回脱敏请求体与响应码 |
| 工具 | `GET /tool/list` 返回 6 个内置工具；`POST /tool/invoke`（`chapter_check`）对含零容忍词的正文返回 `verdict_code=block` |
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
