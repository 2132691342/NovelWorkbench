# 系统架构

本文说明 NovelWorkbench 的定位、技术选型、分层约束、请求生命周期、启动装配与核心链路。全文只描述当前已实现的设计与取舍。

## 0. 文档地图

按「先看全局、再看细节」的顺序读：

| 顺序 | 文档 | 回答什么问题 |
| --- | --- | --- |
| 1 | 本文 | 系统是什么、怎么分层、一次请求怎么走、关键取舍在哪 |
| 2 | [modules.md](modules.md) | 每个业务模块负责什么、数据落在哪、边界与代价 |
| 3 | [ai.md](ai.md) | 模型网关、提示词资产与注入、上下文装配、检索与流式协议 |
| 4 | [prompts.md](prompts.md) | 提示词资产的组织方式与维护流程 |
| 5 | [api.md](api.md) | 接口契约、包络、业务码与权限码 |
| 6 | [database.md](database.md) | 表结构、非关系存储、事务与迁移 |
| 7 | [frontend.md](frontend.md) | 前端工程结构、设计系统、状态与页面实现 |
| 8 | [security.md](security.md) | 认证、授权、密钥、审计与威胁对策 |
| 9 | [deploy.md](deploy.md) | 容器、配置、环境变量与运维 |
| 10 | [test.md](test.md) | 测试策略、保留的用例与集成验证 |

代码约定贯穿全项目：ID 为带业务前缀的 ULID，业务响应码为字符串，字段带明确后缀且有注释，源文件控制在 500 行内。详见本文第 4 节与第 11 节。

## 1. 系统定位

NovelWorkbench 是前后端分离的 AI 小说创作工作台。以「项目 → 卷 → 章节」为写作主线，把大纲、正文、角色、世界观、知识资料、AI 对话与提示词资产收拢进同一个工作台，同时提供完整的 SaaS 管理能力。

| 职责 | 内容 |
| --- | --- |
| 创作业务 | 项目导航、卷章目录、正文编辑、快照回滚、角色关系、世界观设定、创作规划、连续性看板、知识库检索、提示词资产、AI 流式生成 |
| SaaS 管理 | 用户、角色权限、菜单与权限目录、审计、登录日志、模型供应商与模型目录、用量分析、运行状态、任务中心、站内通知 |

系统不开放注册，账号只能由管理员在「用户管理」中创建并分配角色。

设计优先级：**稳定性 > 体验 > 性能**。写入动作全部由作者显式触发，AI 产出不自动落库为权威数据；中间件故障降级而非阻断写作。

## 2. 总体结构

```mermaid
flowchart LR
  FE[Vue 3 SPA] -->|HTTP/JSON + SSE| API[Gin HTTP API]
  API --> SVC[service 业务层]
  SVC --> REPO[repo 数据访问层]
  SVC --> AI[ai 模型能力域]
  REPO --> PG[(PostgreSQL 16)]
  REPO --> REDIS[(Redis)]
  REPO --> MQ[(RabbitMQ)]
  REPO --> MINIO[(MinIO)]
  SVC --> MC[(Manticore 全文索引)]
  AI -->|OpenAI 兼容 / Anthropic| LLM[模型服务]
```

部署形态：前端 nginx 容器 + 后端 Go 容器；中间件可复用宿主机既有实例，也可通过 `docker-compose.yml --profile infra` 本地拉起。后端 `CGO_ENABLED=0` 静态编译，不引入 cgo。

### 2.1 模块地图

按业务域划分的模块清单，每个模块的职责、实现路径与设计权衡见 [modules.md](modules.md)。

| # | 模块 | 主要表 | 主要入口 |
| --- | --- | --- | --- |
| 1 | 认证与会话 | `users`、`login_logs` | `api/auth_api.go`、`service/auth_service.go` |
| 2 | 用户与角色权限 | `users`、`roles`、`user_roles` | `service/user_admin_service.go`、`service/role_service.go` |
| 3 | 菜单与导航 | `menus` | `service/menu_service.go`、`service/menu_permission.go` |
| 4 | 审计与登录日志 | `audit_logs`、`login_logs` | `pkg/access_middleware.go`、`service/audit_service.go` |
| 5 | 应用配置 | `app_settings` | `service/setting_service.go` |
| 6 | 站内通知 | `notifications` | `service/notification_service.go` |
| 7 | 任务中心 | `jobs` | `service/job_service.go`、`service/knowledge_worker.go` |
| 8 | 项目 / 卷 / 章节 | `projects`、`volumes`、`chapters`、`chapter_snapshots`、`word_count_stats` | `service/project_service.go`、`service/volume_service.go`、`service/chapter_service.go` |
| 9 | AI 开书 | 同模块 8（+ `outline_nodes`） | `service/bootstrap_service.go`、`ai/bootstrap.go` |
| 10 | 角色卡与关系图 | `characters`、`character_relations` | `service/character_service.go` |
| 11 | 世界观设定与创作笔记 | `lore_settings` | `service/lore_service.go` |
| 12 | 大纲树与场景卡 | `outline_nodes`、`detailed_scenes` | `service/outline_service.go` |
| 13 | 连续性 | `temporal_facts`、`foreshadow_hooks`、`chapter_summaries` | `service/continuity_service.go` |
| 14 | 知识库 | `knowledge_documents`、`knowledge_chunks` + MinIO + Manticore | `service/knowledge_service.go`、`service/knowledge_worker.go` |
| 15 | AI 对话 | `chat_sessions`、`chat_messages` | `service/chat_service.go`、`service/chat_context.go`、`service/chat_loader.go`、`service/chat_stream.go` |
| 16 | 提示词资产 | `prompts` | `service/prompt_service.go`、`ai/prompt_assets.go` |
| 17 | 工具资产 | `tools` | `service/tool_service.go`、`service/tool_executor.go`、`ai/builtin_tools.go`、`ai/check/chapter_check.go` |
| 18 | 模型生态 | `model_providers`、`model_catalogs`、`model_role_grants`、`model_user_grants` | `service/model_provider_service.go`、`service/model_access_service.go` |
| 19 | 数据总览 | `word_count_stats`、`projects` | `service/stats_service.go` |
| 20 | 用量分析 | `model_usage_stats` | `service/model_usage_service.go` |
| 21 | 运行状态 | 无（实时探测） | `service/system_status_service.go` |
| 22 | 前端工作台装配 | 无（消费上述接口） | `frontend/src/layout`、`frontend/src/store`、`frontend/src/views/Workspace` |

## 3. 技术选型

| 层 | 选型 | 职责 |
| --- | --- | --- |
| 后端语言 | Go 1.24 | 静态编译、单二进制部署 |
| Web 框架 | Gin 1.11 | 路由、参数绑定、中间件链 |
| ORM | GORM 1.31 + pgx | 关系数据读写与启动期自动迁移 |
| 配置 | Viper 1.21 | YAML 模板 + `NW_` 环境变量覆盖 |
| 日志 | Zap 1.28 | 结构化 JSON 日志 |
| AI 编排 | eino 0.9 + eino-ext | OpenAI 兼容与 Anthropic 对话组件 |
| 关系存储 | PostgreSQL 16 | 业务主数据、权限、统计、任务状态 |
| 全文检索 | Manticore HTTP SQL API | CJK ngram 召回，索引可整体重建 |
| 缓存 | Redis | 登出令牌黑名单 |
| 队列 | RabbitMQ | 知识库异步索引任务 |
| 对象存储 | MinIO | 知识库文档原文 |
| 前端框架 | Vue 3.5 + TypeScript + Vite | SPA，pnpm workspace 单仓 |
| UI | Element Plus 2.14 | 组件库；品牌令牌与按钮五态覆盖其默认样式 |
| 状态 / 路由 / HTTP | Pinia、Vue Router、Axios | 认证、项目上下文、包络解包 |
| 可视化 / 编辑器 | ECharts、AntV X6、CodeMirror 6、marked、docx-preview、vue-pdf-embed | 趋势图、关系图、章节正文、笔记预览、文档预览 |
| 测试 | Go testing + Vitest | 链路级单元测试 |
| 部署 | Docker Compose / Podman Compose + GitHub Actions | 镜像构建与发布 |

## 4. 后端分层

依赖方向严格单向：`api → service → repo → domain`；`ai`、`infra`、`pkg` 为叶子能力层，不反向依赖 `api` 或 `service`。

```
backend/
├── cmd/server        装配启动、迁移播种、路由、优雅关闭
├── configs/          配置模板（凭据留空，环境变量注入）
└── internal/
    ├── api           HTTP 处理器与路由注册（26 个文件）
    ├── service       业务规则、事务边界、上下文装配（40 个文件）
    ├── repo          SQL 与中间件访问的唯一出口（含 WithTx，26 个文件）
    ├── domain        DO / REQ / RESP / DTO / VO 唯一定义处（23 个文件）
    ├── ai            模型网关、提示词装配、内置提示词资产、用量契约（21 个文件）
    ├── infra         配置加载、连接构造、启动校验
    └── pkg           ULID、包络、错误、JWT、中间件、密钥封存、审计字段回调与时间工具
```

`pkg` 各文件：`ulid.go`（带业务前缀的 ULID 生成）、`errors.go`（业务码与 HTTP 状态映射）、`logger.go`（Zap 全局日志）、`time.go`（RFC3339 格式化）、`text.go`（字数统计与文本工具）、`actor.go`（GORM 审计字段回调）、`jwt.go`（签名与校验）、`secret.go`（密钥封存）、`access_middleware.go`（鉴权、权限、审计）、`middleware.go`（请求 ID、CORS、方法白名单）、`response.go`（统一包络）。

`ai` 目录的 `domain_<域>.go` 共 12 个文件，一个流程域一个文件，与 [prompts.md](prompts.md) 的域矩阵一一对应。

| 约束 | 原因 |
| --- | --- |
| 请求/响应结构只在 `domain` 定义 | 前后端契约单一事实源 |
| SQL 只写在 `repo` | service 只调仓储方法，不拼查询条件 |
| service 持有 `*gorm.DB` 仅作事务入口 | 跨表写入需同一事务，`repo.WithTx` 是唯一封装 |
| 判定与装配规则下沉为纯函数 | 可见性过滤、作用域优先级、占位符校验、历史收敛可独立单测，不依赖数据库 |
| Manticore 客户端只在知识库相关 service 使用 | 融合、重排、追踪属于业务语义，不是通用数据访问 |
| service 不写 HTTP 状态码 | 业务码到状态码的映射集中在 `pkg.HTTPStatusForCode` |
| api 不写业务判断 | 避免同一规则散落多个处理器 |
| 业务路由只开放 GET/POST | 删除走 `POST /xxx/delete`，方法白名单强制 |
| 路径参数放末尾 | `/menu/detail/{MenuULID}`、`/knowledge/content/{DocULID}` |
| ID 为带业务前缀的 ULID | `PRJ_` / `CHP_` / `MSG_` …，日志可定位业务类型 |
| 字段使用显式后缀 | `project_name`、`genre_code`、`created_at`、`created_by` |
| 时间出参为 RFC3339 字符串 | VO/DTO 的时间字段一律 `string`，由 `pkg.FormatTime` 转换；不用毫秒整数，避免与 `time.Time` 同名字段语义冲突 |
| 审计字段自动填充 | `BaseDO` 带 `created_by` / `updated_by`；`AuthMiddleware` 把用户 ULID 注入 context，GORM 回调在写入前补齐，service 不手写 |
| 权限码由菜单表登记 | 新增权限只需在「菜单管理」登记一行，角色页自动可见 |
| 单文件 ≤500 行 | 控制认知负载，超限按同域拆分 |

## 5. 请求生命周期

中间件顺序（`api.NewRouter`）：

`Recovery → RequestID → Logger → CORS → MethodWhitelist → Auth → Audit → RequirePerm`

| 中间件 | 行为 |
| --- | --- |
| Recovery | 捕获 panic，记日志并输出 `ERR_INTERNAL` |
| RequestID | 读取或生成 `REQ_` 前缀 ULID，写入日志与响应头 `X-Request-ID` |
| Logger | 出口层记录方法、路径、状态码、耗时、请求 ID |
| CORS | 放行前端访问；`OPTIONS` 预检不进入业务 |
| MethodWhitelist | 仅放行 GET / POST |
| Auth | 解析 Bearer 令牌 → 查黑名单 → 校验用户状态 → 注入 `user_ulid` |
| Audit | POST 且不在跳过名单时落审计（失败只告警） |
| RequirePerm | 按权限码收口管理接口，缺失返回 `ERR_FORBIDDEN` |

白名单：免鉴权仅 `/api/v1/auth/login`；免审计为登录、登出与 `/api/v1/chat/send`（会话正文不入审计）。`/health`、`/ready` 挂在根引擎，不经过鉴权与审计。

统一响应包络（`pkg.Ok` / `pkg.Fail`）：

```json
{
  "resp_code": "SUCCESS",
  "resp_message": "ok",
  "trace_id": "REQ_01HZX...",
  "data": {}
}
```

业务码到 HTTP 状态的映射只维护在 `pkg` 一处：

| 业务码 | HTTP | 语义 |
| --- | --- | --- |
| `SUCCESS` | 200 | 成功 |
| `ERR_INVALID_PARAM` | 400 | 参数校验失败 |
| `ERR_UNAUTHORIZED` | 401 | 未登录或登录失效 |
| `ERR_FORBIDDEN` | 403 | 无权限 |
| `ERR_NOT_FOUND` | 404 | 资源不存在 |
| `ERR_CONFLICT` | 409 | 资源冲突 |
| `ERR_BIZ_FAILED` | 400 | 通用业务失败 |
| `ERR_AI_PROVIDER` | 502 | 模型调用失败 |
| `ERR_STORAGE` | 503 | 存储 / 检索 / 队列异常 |
| `ERR_INTERNAL` | 500 | 服务内部错误 |

两个端点不使用该包络：`/chat/send` 返回 SSE 事件流，`/knowledge/content` 返回文件字节流。

## 6. 启动装配

`backend/cmd/server/main.go` 的顺序：

1. 初始化 Zap 与配置（`infra.LoadConfig`，启动期校验关键项）。
2. 连接 PostgreSQL（失败即退出）→ `AutoMigrate` 声明 33 张表（失败即退出）→ 清理已下线的列（会话摘要列）。
3. 连接 Redis / RabbitMQ / MinIO / Manticore；失败只告警并降级。
4. 构造仓储 → 服务 → 中间件 → 路由。
5. 播种：内置提示词（24 份指令 + 12 份身份，按内容指纹升级）、6 个内置工具、默认配置、内置角色、内置菜单 28 行（失败即退出）；用户表为空时创建引导管理员。
6. 可选播种出厂模型供应商（配置了网关地址与模型名且供应商表为空时）。
7. 启动知识库索引 worker（RabbitMQ 消费）。
8. 监听端口；收到 SIGINT/SIGTERM 后优雅关闭 HTTP、worker、连接池。

播种是幂等的：按业务编码查缺补插，已存在的内容按内容指纹升级，因此重启与升级不会产生重复数据，也不会覆盖作者改过的展示字段。

## 7. 核心链路

### 7.1 登录与权限

- 账号由管理员创建，密码 bcrypt 存储，API 永不返回哈希。
- 登录签发 HS256 JWT（默认 7 天），返回用户信息与角色编码；权限码由 `GET /menu/nav` 下发。
- 登出把令牌写入 Redis 黑名单，TTL 取令牌 `expires_at - issued_at`（签发时长）。
- **权限码单一事实源是菜单表**：`menus.perm_code` 即权限登记，权限目录由菜单汇总，角色勾选后立即生效，无需权限码表或角色-菜单关联表。
- 权限逐请求查库（`RoleService.HasPerm`），管理接口流量低，这一跳可接受。

### 7.2 项目与写作台

- 项目是创作资源隔离边界，所有创作表带 `project_ulid`；章节表冗余该列，鉴权不必多表 join。
- 正文更新时服务端计算字数，按差值维护项目累计字数与当日统计。
- 快照用于留档与回滚；快照恢复重新计算字数。
- 前端侧边栏有项目切换器，URL 中的 `project_ulid` 优先级最高，路由变化反写 Pinia store。

### 7.3 AI 对话

```mermaid
sequenceDiagram
  participant FE as 前端
  participant API as chat_api
  participant SVC as chat_service
  participant RAG as knowledge_service
  participant AI as ai.Client
  FE->>API: POST /chat/send
  API->>SVC: Send(ctx, req, writer)
  SVC->>SVC: 提示词解析（项目提示词覆盖全局）
  SVC->>RAG: 混合检索（词法 + RRF + 可选重排）
  RAG-->>SVC: 召回切片与追踪
  SVC->>SVC: 上下文装配（优先级排序，不丢块）
  SVC->>SVC: 落库用户消息
  SVC->>AI: Stream(身份 + 上下文 + 指令 + 历史 + 用户输入)
  AI-->>SVC: 增量文本
  SVC-->>API: SSE chunk（seq 递增）
  SVC->>SVC: 事务落库助手消息与 AI 字数
  SVC-->>API: SSE done（含召回依据与本轮工具编码）
```

关键机制见 [ai.md](ai.md)。设计取舍：单链路（所有生成走 `/chat/send` + 提示词码）保证上下文、容错与审计只有一处实现。

### 7.4 AI 开书

一句创意直接长出可写的项目骨架，解决「新建项目后面对空白卷章不知道从哪下手」的冷启动问题。

```mermaid
flowchart LR
  IDEA[一句创意] --> PLAN["plan：生成方案<br/>书名/卖点/卷章/角色/设定"]
  PLAN --> EDIT[作者逐字段修改]
  EDIT --> APPLY["apply：单事务落库<br/>项目+卷章+角色+设定+两级大纲"]
  APPLY --> WRITE[进入写作台]
```

生成走流式累积（结构化长输出在非流式下会长时间无响应头，被网关首包超时打断），方案只生成不落库；作者确认后由单个事务一次性写入，失败整体回滚。枚举收敛与规模裁剪在服务层完成，不依赖模型自觉。详见 [modules.md](modules.md) 的 AI 开书一节。

### 7.5 知识库

上传 → MinIO 原文 + `pending` 文档行 → RabbitMQ 任务 → worker 提取/切片/哈希去重 → 写 PostgreSQL 切片与 Manticore 索引 → `indexed`。检索为双词法通道 + RRF 融合 + 可选对话模型重排，全程可追踪。权威数据始终在 PostgreSQL 与 MinIO，索引可随时重建。

### 7.6 创作规划与连续性

- 大纲树以项目为根，节点覆盖卷 / 幕 / 故事块 / 章节；父子关系与成环由 service 校验，含子节点不可直接删除。
- 场景卡挂在大纲节点上，记录出场角色、地点、冲突、节奏、入场/出场状态、目标与关键选择。
- 连续性由时序事实、伏笔钩子、章节摘要三本账组成；事实需显式复核（确认/取代/过期）才成为依据，AI 产出默认只是候选。
- 章节摘要带内容哈希与状态，正文变化后可标记 stale，避免过期证据进入上下文。

### 7.7 设定与关系

角色卡记录基础信息、性格、目标、秘密、声音特征与当前状态；角色关系由起点、终点与描述构成，前端用 AntV X6 渲染。世界观设定按分类（世界背景 / 规则 / 地点 / 物品 / 事件）管理。创作笔记复用设定表，`lore_type_code = note`，私人笔记永不进入 AI 上下文。

### 7.8 可观测性

- 用量：按供应商、模型、调用类型聚合调用数、token、延迟、成功率；内存缓冲 5 秒批量落库。
- 运行状态：探测 PostgreSQL / Redis / RabbitMQ / MinIO / Manticore 与模型健康，展示耗时与结论。
- 审计：写操作记录操作者、动作码、目标、结果、IP；仅 `/api/v1/admin/` 路径读取 body 提取目标。
- 任务中心：`jobs` 表状态机 `queued → running → success|failed`，失败可重试。

## 8. 前端架构

前端是 pnpm workspace 单仓：根应用 + 4 个内部包（`@novelworkbench/components` / `hooks` / `request` / `styles`）。

```
frontend/
├── packages/     内部包：图标与主题组件、表单 hooks、请求客户端、设计令牌与按钮五态
├── src/
│   ├── api         请求薄封装（http.ts）、后端契约类型（contracts.ts）与业务模块
│   ├── request     请求实例：注入令牌、解包包络、401 会话过期
│   ├── components  Logo / Breadcrumb / TagsView / ContextMenu / Collapse / Echart / ConfigGlobal / UserInfo / Backtop / Error
│   ├── composables usePageQuery（分页列表状态）、useSSE（流式解析与请求）
│   ├── layout      Layout 外壳 + 顶栏、后端驱动导航、项目切换、通知铃铛
│   ├── router      静态路由表（页面组件在此声明，菜单只控可见性）
│   ├── permission  唯一全局守卫：登录态、导航加载、页面权限码
│   ├── store       user / menu / project / app / locale / tagsView / notification
│   ├── locales     界面壳文案（zh-CN）
│   └── views       登录、仪表盘、写作台、创作规划、连续性、角色、设定、知识库、笔记、
│                   模型、提示词、工具、通知、个人中心、管理端各页、系统设置
```

交互约定：按钮五态（默认/悬停/按下/禁用/加载，悬停放大、按下缩小）、异步动作必有反馈、列表具备加载与空态、危险操作二次确认、路由切换有过渡、AI 流式输出可中止。侧边栏由 `GET /menu/nav` 决定显示什么，页面组件静态声明，因此改菜单不需要前端发版。

前端细节见 [frontend.md](frontend.md)。

## 9. 部署架构

- 后端镜像：多阶段构建，最终只含二进制与 CA 证书，`CGO_ENABLED=0`。
- 前端镜像：Node 构建静态资源，nginx 托管 SPA、反代 `/api/`、关闭 SSE 缓冲。
- 本地：`docker-compose.yml` 提供应用容器与可选 `infra` profile。
- 云主机：`deploy/podman/` 用 Podman Compose 加入既有中间件网络，`deploy.sh` 完成构建、重建与健康自检。
- CI：push / PR 触发 vet、build、test 与前端构建；tag 触发镜像发布到 GHCR。

部署细节见 [deploy.md](deploy.md)。

## 10. 设计取舍

| 决策 | 收益 | 代价 |
| --- | --- | --- |
| 单 AI 链路 | 行为一致、排错路径唯一 | 一键生成需前端两步：发问 + 插入 |
| 不使用向量模型 | 部署轻、成本可控、结果可解释 | 同义改写召回弱于语义向量，靠查询词扩展与 chat 重排补偿 |
| 关键词索引可重建 | 数据一致、删除可控 | 需要异步任务与状态机 |
| 审计同步落库 | 顺序一致、不丢记录 | 高写入量时表是热点；当前规模可接受 |
| 权限逐请求查库 | 改角色立即生效 | 管理接口多一跳查询；流量极低可接受 |
| 笔记复用设定表 | 少一张表与一套 CRUD | 设定与笔记共用分类字段，笔记固定为 `note` |
| 无数据库外键 | 迁移与批量写入不受约束校验拖累 | 引用完整性只能由 service 保证 |
| 菜单即权限登记 | 新增权限零代码改动 | 菜单表承担两重语义，改动需谨慎 |
| 静态路由 + 后端菜单 | 改菜单无需前端发版 | 新增页面必须同时改前端路由表 |
| 历史全量注入 | 不丢任何一条上下文，行为确定 | 上下文变长、token 消耗上升 |

## 11. 质量约束

- Go 与前端源文件控制在 500 行内；导出符号有注释；文件头一句话说明职责。
- 业务规则优先写成纯函数，测试直接驱动，不引入数据库与 mock 框架。
- 不引入 cgo、不引入向量模型、不为几行不可复用代码抽函数。
- 文档只描述当前设计与实现。
