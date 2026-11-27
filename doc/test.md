# 测试说明

本文说明 NovelWorkbench 的测试策略、保留的链路、运行方式与部署后的集成验证清单。

## 1. 策略

测试只保留**长链路**用例：一条用例走完一条真实业务链路，用「环节N」标注环节，断点即定位。

判据是「配错才在生产暴露」。鉴权绕过、密钥解不开、上下文装配偏移、检索召回失真、导航路由拼错、模型输出污染正文——这些静默出错、影响面大的问题值得一条用例；CRUD 字段透传、常量查表、JSON 回显、组件渲染、样式存在性这些由类型检查、编译与集成验证兜住的偏差，不建用例。

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| Go 单元测试 | 跨环节链路：请求管线与幂等、密钥封存、审计表结构与筛选条件、链路摘要与用量记账、权限变更与导航装配、用户列表角色过滤、对话装配与历史、检索融合与索引迁移、提示词资产与开书解析、模型网关与用量上报、体检门禁、工作流模板与运行与就绪度与上下文与门禁收口、模型目录去重 | CRUD 透传、JSON 回显、查表、SQL 细节 |
| 前端单元测试 | 路由装配、SSE 报文解析与事件归并、斜杠命令注册表 | 组件渲染、样式、路由跳转 |
| 类型检查与构建 | TypeScript 类型与生产构建 | -- |
| 集成验证 | 需要中间件与模型网关的完整业务链路 | 逐个控件的机械点击 |

原则：

- **一条用例一条链路**。同域环节串在同一个 `Test*` 内，用「环节N」分隔。
- **能力归属决定文件**。同一能力的断言必须在同一个文件里，否则改一处逻辑要同时开多个文件、跑多轮测试才敢确认没坏。一个文件内可以有多条用例，但每条用例必须自成一条链路。
- **断言跨环节因果**，不重复断言代码自身保证的约束。例：断言「项目提示词覆盖全局后全局提示词被标记为已覆盖」，而不是逐字段核对枚举取值表。
- **静态数据不单独断言**。内置资产的数量、编码唯一性这类改动当场就能看出的事实，用一次遍历顺带校验即可，不为它单开用例。
- **不测确定性**。同一输入两次调用结果一致是纯函数的基本性质，断言它只是让用例变长。
- **不测接线**。路由是否注册、字段是否透传由类型检查与集成验证兜住。
- HTTP 依赖用 `httptest.NewServer`，数据库用 gorm `DryRun` 捕获 SQL，两者都不连真实服务。
- 不使用 `time.Sleep`；需要顺序时用确定的输入顺序表达。
- 业务规则写成无依赖纯函数，测试直接驱动，不引入数据库与 mock 框架。

当前规模：**后端 12 个测试文件、23 条用例，前端 1 个测试文件、3 条用例**。全部用例在 1 秒内跑完（后端最慢单包约 0.3 秒，前端约 0.5 秒；`go test ./...` 含编译约 3 秒）。

## 2. 后端

### 2.1 运行

```bash
cd backend
CGO_ENABLED=0 go test -count=1 -shuffle=on ./...
```

CI 的 backend job 依次执行 `go vet ./...`、`CGO_ENABLED=0 go build -tags "netgo osusergo" ./...` 与上述测试命令，环境变量为 `GOTOOLCHAIN=local`（与 Dockerfile 一致）与 `CGO_ENABLED=0`。

定位单条链路：

```bash
go test -run TestChatAssemblyChain ./internal/service/
```

### 2.2 用例清单

| 文件 | 用例 | 链路环节 |
| --- | --- | --- |
| `internal/pkg/security_chain_test.go` | `TestRequestPipelineChain` | 放行：请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库（请求体 + 响应体）→ 响应包络<br>拒绝：无令牌 / 令牌已吊销 / 账号禁用（401）与权限不足（403）<br>幂等：首请求执行一次 → 同键同体重放首响应 → 同键异体拒绝 → 5xx 不缓存允许重试 → 未带键不受影响 |
| `internal/pkg/security_chain_test.go` | `TestSecretSealChain` | 封存 → 解封还原 → 同明文密文不同（nonce 随机）→ 篡改与材料变更被拒（认证标签校验） |
| `internal/repo/audit_chain_test.go` | `TestAuditSchemaChain` | 列名反引号转义（`model_name` 是保留字）→ 补列语句同规则 → INSERT 列清单与 `auditColumnSpecList` 不漂移 → SELECT 列是 spec 子集 → text 列集合与列类型一致 → `quoteIdent` 内部反引号加倍 → 派生标记列必须是 int（退化成 text 会让链路筛选整页失准）→ 退化判定只认过滤核心列 |
| `internal/repo/audit_chain_test.go` | `TestAuditConditionChain` | 多条件拼接只允许一次 MATCH → 时间区间优先精确时刻 → 无筛选时回落恒真条件 |
| `internal/service/observability_chain_test.go` | `TestAuditTraceSummaryChain` | 链路级别取最严重 span → 根 span 提供根动作 → 耗时取首末 span 跨度 → 最慢一步可定位 → 无根 span 时按区间兜底 → 步骤级别映射（skipped 不算异常） |
| `internal/service/observability_chain_test.go` | `TestModelUsageBufferChain` | 三种口径同一累加器：成功/失败/未知用量分别计数，token 只累加真实值 → 平均延迟按每日调用数加权 |
| `internal/service/access_control_chain_test.go` | `TestAccessControlChain` | 权限码规范化（去重保序 / 空白剔除 / 非管理员禁用 `*` / 未登记报错）→ 内置角色脏数据修复（`user` 剔除 `*`、`admin` 固定 `*`）→ 权限码下线时从角色剔除 → 菜单种子在前端有对应页面且编码与路由唯一 → 可见性过滤（隐藏页保留标记）→ 菜单树装配 → 权限目录汇总与权限码去重 |
| `internal/service/access_control_chain_test.go` | `TestUserAdminListRoleFilter` | 带角色编码走 `user_roles`/`roles` 关联查询 → 未知编码返回空页而非报错 → 不带编码时不 JOIN 角色表（否则多角色用户重复出行） |
| `internal/service/chat_assembly_chain_test.go` | `TestChatAssemblyChain` | 斜杠命令解析 → 提示词作用域覆盖与停用回退 → 全局覆盖标记 → 占位符校验 → 上下文块优先级与私人笔记排除 → 显式引用摘要 → 装配不丢块 → 消息序列成型（提示词为系统级）→ @ 素材来源 → 工具声明与参数解码 → 流式工具调用归并 → 流式事件契约 → 历史全量注入（条数不截取）→ 单条超长首尾收敛 → 归档逐条编号（条数、角色、时间、工具标签）→ 归档不丢条 |
| `internal/service/knowledge_chain_test.go` | `TestKnowledgeRetrievalChain` | 文本切片（重叠窗口、非法 overlap 收敛）→ 查询词抽取 → RRF 双通道融合 → 单通道排序与 TopK → 重排解析与 fail-open → 中文 ngram 索引建表结构迁移（识别旧表 → 重建 → 回灌只执行一次；新结构启动不重建） |
| `internal/service/workflow_chain_test.go` | `TestWorkflowTemplateChain` | 模板必填字段与编码唯一 → 出稿排在入库之前 → AI 步骤必须引用提示词 → 步骤 JSON 往返不丢字段 |
| `internal/service/workflow_chain_test.go` | `TestWorkflowRunChain` | 默认值补齐（字数 / 门禁 / 审核开关）→ 越界拒绝 → 显式关闭审核不被默认值覆盖 → 运行活跃判定 → 章节下标与越界安全 → 模板用途 → 目标章节范围（起草要空白章，打磨/去 AI 味要已有正文） |
| `internal/service/workflow_chain_test.go` | `TestWorkflowReadinessChain` | 评分边界（无检查项 0、全就绪 100、部分就绪向下取整）→ 数量类未达标保留补齐建议与当前值/目标值、达标后收口 → 文本类空白视为未就绪且不阻塞 → 提示词缺口文案点名具体编码 |
| `internal/service/workflow_chain_test.go` | `TestWorkflowContextChain` | 上下文总量收敛：未超原样返回、刚好等于上限不省略、超出保留首尾并标注省略字数且总长不失控 |
| `internal/service/workflow_chain_test.go` | `TestWorkflowBodyOnlyPromptChain` | 起草类系统提示词必须禁止过程文字 → 会被写入正文的提示词不得要求先列清单 |
| `internal/service/workflow_chain_test.go` | `TestWorkflowGateStepChain` | 门禁收口分支（命中 running 门禁行原地更新、缺失才回退插入）→ 定位条件同时锁类型与状态 → 历史数据回退插入分支 |
| `internal/service/model_catalog_chain_test.go` | `TestModelCatalogDedupeChain` | 分组键与唯一索引维度一致 → 保留行优先级（被引用 > 手工创建 > 更新更晚 > ULID 升序）→ 授权迁移只补缺的键 → 默认标记收敛 → 供应商合并优先级 → 跨供应商同名模型并入同一组 → 空名供应商同样合并 |
| `internal/service/prompt_chain_test.go` | `TestPromptAssetChain` | 播种形状（4 流程 × 2 条 = 8 条 command）→ 编码集合与归属可反查 → 写作类提示词禁过程指令（否则过程文字被写进正文）→ review JSON 契约字段与判定规则齐全 |
| `internal/service/prompt_chain_test.go` | `TestPromptRetiredChain` | 现行 8 条一律保留 → 旧身份提示词即使编码与任务同名（种类不同）也要删 → 清单外的旧任务编码同样清理 → 身份提示词兜底（无库/查询失败/正文空白）→ 组合系统消息同时含人格与文风 → 走真实服务入口验证兜底 |
| `internal/ai/prompt_chain_test.go` | `TestBuiltinAssetChain` | 流程与任务编码契约（顺序、数量、唯一）→ 任务块必须声明输入与输出格式 → 长度约束 300–900 字 → 占位符全部登记 → 人格长度与关键规则齐全 → 组合系统消息回退人格 |
| `internal/ai/prompt_chain_test.go` | `TestOpenBookPlanChain` | 代码围栏与前后缀文字容错 → 书名/卷章/角色字段完整取出 → 无 JSON / 缺书名 / 缺卷章 / 非法 JSON 一律报错 |
| `internal/ai/gateway_chain_test.go` | `TestModelGatewayChain` | 实例缓存键随密钥/协议/地址/模型/参数/超时变化 → 流式 HTTP 客户端不设整体超时（否则长流被拦腰切断）→ 流式成功只记一条用量（取流阶段不记）→ 流式失败记失败且用量未知 → 非流式成功一条用量 → 分片用量取最大值汇总、无用量返回空 |
| `internal/ai/check/check_chain_test.go` | `TestChapterGateChain` | 空正文判阻断 → 零容忍词计数与分级 → 缺钩子警示 → 综合分扣分 → 干净正文不误判 |

### 2.3 切分依据

- **请求管线与密钥封存同文件但分成两条用例**。两者都属「安全边界」，改中间件顺序或改密钥算法时只需开一个文件；拆成两条用例是因为数据无耦合、失败原因完全不同（一个是鉴权串错，一个是密文解不开）。
- **审计表结构与筛选条件同文件**。两者都服务 Manticore 审计表，列定义是筛选条件的输入源，合成一条链能验证「结构正确 → 条件拼接正确 → 筛选可用」。
- **链路摘要与用量记账同文件**。两者都属「出问题能定位到哪一步」的可观测性：摘要算错会让排障走向错误方向，用量记成两条会让统计出现伪数据。
- **访问控制与用户列表角色过滤同文件**。两者都属权限域：前者管角色能做什么，后者管用户列表按角色筛；后者验证查询路径分流（带编码 JOIN / 不带编码不 JOIN），错在「多角色用户重复出行」这类静默错误。
- **对话装配单文件**。历史注入是装配链的最后一段（装配 → 消息序列 → 历史 → 归档），拆开跑会失去「消息序列里历史确实按原生角色注入」这一关键验证。
- **检索与索引迁移合并**。两者串在同一次启动与同一次检索请求上，合并后能用一条用例覆盖「建表结构正确 → 切分正确 → 召回正确」；索引迁移会重建已有索引，其断言放在链路末尾，失败时一眼能区分是迁移还是检索。
- **工作流五条用例同文件**。模板形状、运行参数、就绪度、上下文收敛、门禁收口同属工作流这一条业务链，改引擎时只开一个文件；就绪度与上下文收敛虽然无数据耦合，但都与「开跑前是否真能跑起来」直接相关，分开会让人误以为它们是独立能力。
- **模型目录去重单文件**。它决定「删哪一行」，删错会丢授权与历史默认模型；保留行优先级、授权迁移、默认标记收敛、跨供应商同名合并在一次合并事务里逐层依赖，拆开就失去「迁移后默认只剩一个」这一端到端因果。跨供应商同名是线上唯一索引建不起来的直接回归。
- **提示词资产与身份兜底同文件**。播种、编码归属、下线清理、系统人格兜底串在「启动播种 → 库内取用 → 旧数据清理」这一条链上；写作类禁过程指令与 review JSON 契约是这条链的产出约束——提示词文本错一个字，正文就被污染或工作流解析不出结论。
- **体检门禁单文件**。`Analyze` 是纯函数，同输入同输出是语言保证，不需要占一行断言；它断言的是「问题分级 → 综合分 → 结论」这一读者直接看到的因果。
- **三处用 gorm `DryRun` 而不连库**（访问控制链的用户列表、提示词链的服务入口、工作流链的门禁定位）：仓库层无 sqlmock，DryRun 只捕获生成的 SQL 与驱动调用，不建立连接，因此「不引入数据库」的约束仍然成立——这里引入的是语句捕获，不是数据库。

## 3. 前端

### 3.1 运行

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm test # vitest run
```

`frontend/vitest.config.ts`：`environment: 'node'`，`include` 覆盖 `src/**/*.test.ts`，复用 `@` 别名。

### 3.2 用例清单

`src/appChain.test.ts` 三条用例：

| 用例 | 链路环节 |
| --- | --- |
| 路由装配 | 路径拼接不产生结尾斜杠 → 绝对路径与外部链接原样返回 → 路由名唯一 → `:project_ulid` 占位替换与无项目时保留占位 |
| 流式输出 | 跨 chunk 断行、多事件粘包、CRLF、非法 JSON 跳过、心跳注释忽略、尾部半行留存 → 思考与正文分别累加、`done` 用服务端全量覆盖并补齐召回依据与工具、`error` 落兜底文案、未知事件不改写消息 |
| 斜杠命令 | 首 token 解析与正文剥离 → 编码唯一且可按编码取回 → 关键词过滤命中编码与名称 → 动作编码不与内置提示词编码冲突 |

三段合在一个文件：它们都是「作者操作 → 页面响应」这一条链的前后端两半，拆开跑会失去「命令解析口径与后端一致」这一关键交叉验证。

组件渲染与样式存在性不设断言；类型正确性与生产产物由 `pnpm build` 保证，CI 与镜像构建都执行它。

## 4. 集成验证

需要真实中间件与模型网关的链路无法在单元测试中覆盖，通过部署后的实际请求验证：

| 场景 | 验证方式 |
| --- | --- |
| 启动与播种 | 后端启动日志显示迁移表完成、播种 3 个工作流模板 / 8 条任务提示词 / 6 个工具与菜单；`/health` 与 `/ready` 返回 200 |
| 登录与导航 | `POST /api/v1/auth/login` 取令牌 → `GET /api/v1/menu/nav` 返回按权限过滤后的导航树 |
| 业务接口 | 按模块调用列表/详情/写入接口，确认业务码为 `SUCCESS` 且字段符合契约 |
| AI 流式 | `POST /api/v1/chat/send` 观察 `thinking` → `chunk` → `done` 事件序列与 seq 递增 |
| AI 开书 | `POST /api/v1/project/bootstrap/plan` 返回方案（卷章/角色/设定齐全）→ `POST /project/bootstrap/apply` 后 `GET /project/tree/:id` 与 `POST /outline/list` 能查到对应卷章与大纲节点 |
| 检索 | 上传文档 → 索引任务完成（`indexed`）→ 关键词检索返回命中片段与追踪信息 |
| 历史全量注入 | 长会话中追问「最开始聊的是什么」，回答应覆盖会话最早的内容（滑窗实现会答不出） |
| 审计闭环 | 任意写接口调用后 `POST /admin/audit/trace/list` 能查到该链路（`span_count`/`total_latency_ms`/`slowest_span_ms` 有值）；`POST /admin/audit/list` 查到逐 span 记录；`GET /admin/audit/detail/:id` 返回脱敏请求体、响应码**与响应体**；`GET /admin/audit/trace/:TraceID` 返回同一链路的全部记录；`POST /admin/audit/stats` 的区间统计与列表条数口径一致 |
| 审计留存 | `GET /admin/audit/retention` 返回留存天数、审计与登录日志条数、真实计算的 `next_purge_at`；`POST /admin/audit/purge` 清理过期记录后 `total_count` 与 `login_log_total_count` 同时下降 |
| 工作流链路追踪 | 一次运行在审计链路里可看到创建（http）→ 步骤（workflow_step）→ 模型调用（ai_call）→ 审核决定（workflow/gate/*）→ 收尾（workflow/run/*），瀑布图能定位最慢一步 |
| 一键开跑 | `POST /workflow/run/quick_start` 只给 `project_ulid` 即可创建运行，参数等于项目 `auto_*` 默认值，素材缺口以 `ready_flag=false` + `blocking_hint_list` 返回而不阻塞 |
| 项目三态更新 | `POST /project/update` 只带 `auto_gate_score` 时 `style_text` / `synopsis_text` / `genre_code` 保持不变；显式传空串才清空 |
| 工作流全链路 | `POST /workflow/run/create`（`chapter_draft` + `new_chapter_count=1`）→ 轮询 `/workflow/run/detail/:id` 直到 `waiting_review` → `POST /workflow/run/approve` → 状态变 `succeeded`，章节正文与字数已写入，摘要已生成 |
| 工作流退回 | 待审核时 `POST /workflow/run/reject` 带意见 → 回到 `queued` 并从本章第一步重跑，意见进入下一轮起草 |
| 工作流章节范围 | `chapter_deai` 且不指定章节 → 自动取已有正文的章节；项目无正文时报「本模板处理已有正文」而不是空跑 |
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
| `ci.yml` backend | `go vet ./...`、`CGO_ENABLED=0 go build`、`go test -count=1 -shuffle=on ./...` |
| `ci.yml` frontend | `pnpm install --frozen-lockfile`、`pnpm test`、`pnpm build`（含 `vue-tsc --noEmit`） |
| `ci.yml` docker | 后端与前端镜像构建 |
| `release.yml` | tag 推送时构建并推送镜像到 GHCR |

CI 全程不连接外部服务。
