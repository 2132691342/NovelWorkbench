# 测试说明

本文描述 NovelWorkbench 当前保留的测试、运行方式与集成环境验证方法。
只写现在存在的测试文件与用例，不描述不存在的用例。

## 1. 测试策略

### 1.1 分层

| 层 | 测什么 | 刻意不测什么 |
| --- | --- | --- |
| Go 单元测试（`go test -short`） | 请求生命周期、上下文装配、检索融合、切片算法、索引表迁移、密钥封存、用量聚合等跨步骤链路 | CRUD 字段透传、JSON 序列化回显、常量表查表、仓储 SQL 细节 |
| 前端单元测试（`npm run test`） | SSE 事件解析这类纯函数链路 | 组件渲染、样式存在性、路由跳转 |
| 类型检查与构建（`npm run build` = `vue-tsc && vite build`） | TypeScript 类型正确性与生产构建通过 | —— |
| 集成环境验证 | 需要中间件与模型服务的完整业务链路 | 逐个 UI 控件的机械点击 |

### 1.2 原则

- 一个用例讲一条链路：多个环节串在同一个 `Test*` 函数内，用注释标出「环节一 / 环节二 / 环节三」。
- 表驱动只用于「同一行为的多个输入」（如鉴权的四条拒绝路径、检索模式的多种取值），
  行为不同则分节断言。
- `-short` 模式不依赖任何外部服务：外部依赖一律用 `httptest.NewServer` 或闭包替身。
- 禁止 `time.Sleep`；需要顺序时用确定的输入顺序表达。
- 断言面向可观察结果：业务码、HTTP 状态、内容包含关系、顺序、条数；
  不断言内部变量名、私有函数调用次数或 SQL 语句。
- 不为单个函数单独建用例：没有链路语义的函数级用例失败时无法定位上下文。
- **业务规则下沉为纯函数**：判定与装配规则（可见性过滤、作用域优先级、变量取值、
  关键词命中、预算裁剪）从 service 方法中抽成无依赖的纯函数，测试直接驱动这些函数，
  不引入数据库或 mock 框架。service 方法只负责取数与编排。

## 2. 后端单元测试

### 2.1 运行方式

```bash
cd backend
CGO_ENABLED=0 go test -short -count=1 -shuffle=on ./...
```

CI 使用同一命令（见 `.github/workflows/ci.yml` 的 backend job），job 级环境变量为：

| 变量 | 值 | 原因 |
| --- | --- | --- |
| `GOTOOLCHAIN` | `local` | 与 Dockerfile 一致：`go.mod` 声明的 Go 版本必须能被固定工具链直接构建 |
| `CGO_ENABLED` | `0` | 全流水线静态构建，禁止 cgo |

同一 job 在测试之前还执行 `go vet ./...` 与
`CGO_ENABLED=0 go build -tags "netgo osusergo" ./...`。

### 2.2 当前用例清单（共 11 条）

| 测试文件 | 用例函数 | 覆盖链路 | 断言要点 |
| --- | --- | --- | --- |
| `backend/internal/ai/prompt_test.go` | `TestContextAssemblyChain` | AI 上下文装配链路：优先级排序 → 空块跳过 → 超预算整块丢弃 → 同优先级稳定 → 模板渲染 → 系统提示词 | 预算充足时不丢块且 `skill` 块排最前；空块不进结果；预算不足时从低到高丢 `history`/`chapter_summary` 且高优先级块保留；同优先级保持输入顺序；占位符缺失替换为空串；系统提示词含项目文风 |
| `backend/internal/pkg/access_test.go` | `TestAccessControlChain` | 访问控制链路：请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库 → 响应包络；拒绝路径：缺失令牌 / 令牌已吊销 / 账号已禁用 → 401，权限不足 → 403 | 放行路径断言 HTTP 200、`resp_code=SUCCESS`、`trace_id` 等于注入的 `X-Request-ID`、鉴权用户已注入、审计恰好 1 条且动作码与目标正确；拒绝路径用表驱动覆盖 4 种输入，分别断言 HTTP 状态与 `resp_code`（`ERR_UNAUTHORIZED` / `ERR_FORBIDDEN`） |
| `backend/internal/pkg/access_test.go` | `TestSecretSealOpenChain` | 供应商密钥封存链路：同材料可解 → 换材料失败 → 坏密文失败 → 空材料失败 → nonce 随机 | 密文非空且非明文；同材料解封还原原文；换材料、非法密文、空材料三者必须报错；两次封存结果必须不同 |
| `backend/internal/service/chat_service_test.go` | `TestChatRequestPreparationChain` | AI 对话请求准备链路（环节一~九）：技能编码解析 → 变量取值与必填缺失 → 参考资料关键词命中 → 技能命中渲染 → @引用分优先级注入 → 设定注入且私人笔记排除 → 预算裁剪丢块 → 历史选取 → 消息序列成型 → 显式引用会话归属校验 | 编码与参数分离、空白归一、未命中（含非首 token 与单独 `/code`）正文原样保留；变量显式值优先于缺省值、必填缺失入 `missing_list`；空关键词总是命中、关键词命中与未命中三态；`skill` 块优先级 100 且渲染结果即最终指令、`skill_reference` 优先级 85 且含资料正文；当前章优先级 90、引用章与引用文档优先级 80；`note` 类型设定不得进入 `lore` 块；超预算时丢 `chapter_summary`/`history` 而保留 `ref_doc`/`rag`；历史取最近 N 条且显式引用去重后置尾；消息序列为 system → 【上下文】→ 历史 → 最终指令；引用会话外消息必须失败 |
| `backend/internal/service/menu_service_test.go` | `TestMenuAndPermissionChain` | 菜单与权限目录链路（环节一~五）：导航可见性过滤 → 菜单树装配 → 权限目录汇总 → 重复权限码去重 → 分组编码与展示名 | 动作节点、隐藏节点与未授权权限码的菜单都不进导航树；通配权限码 `*` 放行全部；菜单树按 ULID 关联父子且下级权限码不丢；权限目录含通配组且 `system` 组同时含菜单权限与动作权限；动作权限说明为「动作权限：…」菜单权限说明为「菜单权限：父 / 子」；重复登记的权限码只出现一次；分组编码取权限码首段（无分隔符归 `other`） |
| `backend/internal/service/skill_service_test.go` | `TestSkillResolutionChain` | 技能链路（环节一~八）：编码解析 → 作用域优先级 → 变量取值 → 模板占位符校验 → 参考资料关键词命中 → 关键词归一 → 全局技能覆盖标记 → 参考资料预算截断 | 显式 `skill_code` 优先且不剥离正文，正文首 token 的 `/code` 命中才剥离，命令不在首 token 时不命中；启用项目技能优先于全局技能，停用技能一律不命中；变量取值为显式 > 缺省 > 空串，显式空值回落缺省值，必填缺失进 `missing_list` 且 `skill_code`/`user_input` 恒注入；模板未声明占位符与空变量名被拒绝；空关键词总是命中、任一关键词命中即命中；关键词去空去重；全局技能仅被同编码的启用项目技能标记覆盖；超预算资料不注入，命中资料按「【资料名】\n正文」格式注入 |
| `backend/internal/service/knowledge_service_test.go` | `TestKnowledgeRetrievalChain` | 知识库检索链路（环节一~七）：文本切片 → 查询词抽取 → 检索模式归一 → RRF 双通道融合 → RRF 单通道排序与 TopK 截断 → 重排结果解析与 fail-open → 默认召回条数解析 | 短文本产单切片、空白文本不产切片、相邻切片共享重叠区、窗口内边界优先于硬切、非法参数收敛；CJK 切 2-gram、拉丁词去重去标点；未知模式按 `keyword`；跨通道命中切片排首位且得分等于 `1/62 + 1/61`、来源标 `hybrid`；单通道按 `1/(60+rank)` 降序且截断到 topK；重排解析重排并补齐尾部，重复 ID / 越界 ID / 非 JSON / 带围栏均解析失败；无应用配置时 `resolveRAGTopK` 回退全局配置 |
| `backend/internal/service/knowledge_service_test.go` | `TestManticoreSchemaMigrationChain` | Manticore 中文 ngram 表结构迁移链路：旧表结构识别 → 重建 → 仅迁移一次 | 旧表返回 `rebuilt=true`，且语句序列为 `SHOW CREATE TABLE` → `DROP TABLE` → 含 `ngram_len='1'` 的建表；再次调用返回 `rebuilt=false`（新结构启动不重建） |
| `backend/internal/service/model_usage_service_test.go` | `TestUsageAggregationChain` | 用量分析链路（环节一~三）：查询区间解析 → 缺日补零 → 按调用数加权的平均延迟 | 缺省区间为合法日期且起点早于终点；起止倒置自动纠正；非法起点回退为合法日期且起点不大于终点；缺日补零且已有数据不被改写、尾部日期补齐；加权平均延迟等于 175，空数据为 0 |
| `backend/internal/pkg/text_test.go` | `TestTruncateText` / `TestCountWords` | 文本工具：多字节安全截断 → 零长度 → CJK 逐字计数 → 拉丁词计数 | 中文截断按字符不破字；短文本原样返回；零长度返回空串；中文 4 字计 4；英文 4 词计 4；中英混排 3 个词元 |

用例组织方式：链路子节以 `// ---- 环节N：… ----` 注释分隔；
仅当同一行为有多个输入时才用表驱动（`TestAccessControlChain` 的拒绝路径）。

## 3. 前端单元测试

### 3.1 运行方式

```bash
cd frontend
npm ci
npm run test
```

`npm run test` 实际执行 `vitest run`；`frontend/vitest.config.ts` 配置为
`environment: 'node'`、`include: ['src/**/*.test.ts']`，并复用 vite 的 `@` 路径别名。

CI 的 frontend job 执行同一命令，随后再执行 `npm run build`（含 `vue-tsc` 类型检查）。

### 3.2 当前用例清单（共 1 条）

| 测试文件 | 用例 | 覆盖链路 | 断言要点 |
| --- | --- | --- | --- |
| `frontend/src/composables/useSSE.test.ts` | `parseSseBuffer 事件解析链路`（`describe('parseSseBuffer 事件解析链路')` 下的单个 `it`） | SSE 事件解析链路：跨 chunk 断行 → 多事件粘包 → CRLF 兼容 → 非法 JSON 跳过 → 心跳注释忽略 → 尾部半行留存 | 第一段解析出 1 个完整事件并把截断半行留在 `restText`；第二段补全半行后解析出 2 个事件且 `restText` 为空；非法 JSON 行被跳过且不影响后续事件；非 `data:` 前缀行（心跳注释）不产生事件；末尾半行必须完整留存供下一段拼接 |

前端组件渲染与样式存在性一类断言不保留；类型正确性与生产产物由 `npm run build` 保证，
CI 与镜像构建均执行它。

## 4. 集成环境验证（E2E）

### 4.1 拉起环境

E2E 在集成环境执行：后端直连 PostgreSQL / Redis / RabbitMQ / MinIO / Manticore，
模型网关使用本地 mock（提供 OpenAI 兼容的 `/v1/chat/completions` 流式响应），
验证本系统的链路行为而非模型输出质量。

中间件用根目录 `docker-compose.yml` 的 `infra` profile 拉起：

```bash
cp .env.example .env
# 编辑 .env，填入本地测试值：
#   NW_JWT_SECRET=<本地随机串>
#   NW_POSTGRES_* / NW_REDIS_* / NW_RABBITMQ_* / NW_MINIO_* 全部指向 infra profile 的服务名与本地口令
#   NW_MANTICORE_BASE_URL=http://manticore:9308
#   NW_AI_BASE_URL=http://127.0.0.1:<mock-port>/v1
#   NW_AI_API_KEY=test-key
#   NW_AI_CHAT_MODEL=test-chat-model
#   NW_APP_ADMIN_PASSWORD=<本地管理员口令>
docker compose --profile infra up -d --build
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/ready
```

说明：

- `infra` profile 启动 PostgreSQL 16、Redis 7、RabbitMQ 4、MinIO、Manticore；
  口令取自同一份 `.env`，避免应用与中间件口令不一致。
- 后端容器启动时自动建表（28 张表，含 `menus` / `skills` / `skill_references`）、播种内置技能、
  默认配置、内置角色、内置菜单；配置了 `NW_AI_BASE_URL` 与 `NW_AI_CHAT_MODEL` 时再播种默认供应商；
  用户表为空时创建管理员 `admin`（口令取 `NW_APP_ADMIN_PASSWORD`）。
- mock 网关只需实现 OpenAI 兼容的对话流式响应即可覆盖生成、重排与连通性测试三条路径。
- mock 不可用时，其余功能（项目、卷章、角色、设定、规划、连续性、知识库上传与检索）仍可验证；
  AI 生成类操作返回 `ERR_AI_PROVIDER`。

### 4.2 验证清单

| 链路 | 验证点 |
| --- | --- |
| 认证 | 无注册接口（`POST /auth/register` 404）；登录（登录名或邮箱均可）；登出拉黑后旧令牌立即失效；禁用账号后旧令牌失效；响应不含密码哈希 |
| 菜单与权限 | `GET /menu/nav` 只下发启用 + 展示 + 有权限的非动作节点；仅持 `skill:manage` 的账号看不到系统分组、访问 `/admin/users` 被重定向；新增菜单后导航与权限目录同步出现；删除有下级菜单被拒；重复 `perm_code` 被拒 |
| 技能 | 全局技能列表 9 条；新建项目技能后 `GET /skill/list?project_ulid=` 合并返回并标记 `overridden_flag`；`/skill/resolve` 解析编码、变量与缺失项；参考资料关键词命中才进 `ref_text_list`；模板未声明占位符保存被拒；内置技能删除被拒 |
| 项目→卷→章 | 项目 CRUD、目录树、卷章排序号；字数账目（改正文 / 清空正文 / 删章 / 删卷均正确增减）；章节快照留档、列表、回滚（回滚前自动留档）；删除项目后各子表零残留且 MinIO/Manticore 副本被清理 |
| 角色设定 | 角色卡 CRUD 与类型/关键词过滤；关系边增删与自环、跨项目被拒；关系图节点与边数量正确；删角色后关系边清理；设定条目与 `note` 笔记的增删查 |
| 创作规划 | 大纲树层级与排序；父节点设为自身/后代被拒；有子节点时删节点被拒；场景卡与节点绑定、角色列表回读为数组；按节点过滤场景 |
| 连续性 | 候选事实不自动转正；复核只能推进到 `confirmed`/`superseded`/`stale`，退回候选被拒；伏笔状态流转与重要程度缺省为 `medium`；摘要同章节重复保存不产生重复行 |
| 知识库 | 上传 → 队列 → 切片 → 落库 → 建索引 → 检索命中并标注 `source_mode`；同哈希切片 ULID 复用；失败重试与 `failed` 状态；重投索引；删除后对象/索引/切片/文档全清；检索过程返回查询词与入选切片 |
| 对话 | SSE 流式（`chunk` 序号单调递增 → `done` 带召回依据）；技能命中与自由对话两种路径；@引用章节与文档按优先级注入；历史条数取配置值；消息落库；AI 字数落账；指定供应商生效；停止生成后不落库助手消息；插入章节走同一自动保存通道 |
| 模型生态 | 供应商 CRUD 与密钥掩码；首次创建必须填 API Key；默认供应商唯一且不可删除；连通性测试成功/失败均回写状态与时间；用量聚合随真实调用增长、缺日补零 |
| 运行状态 | 依赖健康探测（正常 / 未配置 / 不可用三种状态）；必需依赖与可选依赖的失败级别不同；模型供应商项与能力页结论一致；页面显示运行时长与构建版本 |
| 前端 | 登录页无注册入口；守卫与令牌失效跳转；仪表盘统计与项目切换；写作台（目录树、编辑器自动保存、AI 面板 SSE、技能自动补全、模型选择、插入章节）；菜单管理增删改后侧边栏变化；权限目录与角色页选项一致；关系图；设定管理；知识库上传/检索/预览；笔记自动保存；技能管理（模板/变量/参考资料）；用量分析与运行状态页 |

## 5. 环境与注意事项

- CI 只运行 `go test -short` 与 `npm run test`，不连接任何外部服务；
  单元测试中的 HTTP 依赖用 `httptest.NewServer` 模拟。
- `-short` 为当前唯一使用的测试模式开关：单元测试本身不需要外部服务，
  `-short` 保留给后续可能引入的可选集成链路（CI 注释亦如此说明）。
- E2E 需要真实中间件与一个 mock 模型网关，不需要真实模型服务。
- 测试环境与部署环境共用同一套 `NW_*` 环境变量名，完整变量表见 `doc/deploy.md`。
- 测试环境中的口令同样是占位值；不要把真实凭据写入仓库文件，使用 `.env`
  （已被 `.gitignore` 忽略）。
- 后端 `go vet` 与静态构建在 CI 中与测试同批执行；本地若改动 `go.mod` 的 Go 版本，
  需同步确认固定工具链可用（CI 使用 `GOTOOLCHAIN=local`）。

## 11. 2026-11 回归记录

以下为本次改造后实际执行过的检查（未执行的项不写入）：

| 检查 | 命令 / 场景 | 结果 |
| --- | --- | --- |
| 后端编译 | `go build ./...` | 通过 |
| 后端静态检查 | `go vet ./...` | 通过 |
| 后端测试 | `go test ./...` | 通过（ai/pkg/service） |
| 前端类型检查 | `npx vue-tsc --noEmit` | 通过 |
| 前端构建 | `npm run build` | 通过 |
| 登录与外壳 | 浏览器登录 admin，仪表盘加载 | 通过 |
| 项目与统计 | 仪表盘项目卡、趋势、每日目标 | 通过 |
| 章节编辑 | 工作台打开第一章，字数与保存状态 | 通过 |
| AI 自动上下文 | 提问「本章主角是谁」→ 回答「林默」 | 通过 |
| 知识库上传 | 上传 md，状态 pending → indexed，PG 1 切片 | 通过 |
| 知识库检索 | 查询「潮汐号 失踪案」→ 命中 1 条并生成 trace | 通过 |
| 切片列表 | 修复 `?page_num=` 后返回 200 与切片文本 | 通过 |
| 连续性事实 | 选择来源章节并创建事实 | 通过 |
| 笔记 | 新建笔记并自动落库 | 通过 |
| 系统设置 | 修改配置并保存、读取确认 | 通过 |
| 英文界面 | 仪表盘/知识库/连续性/创作规划/设置/管理页无裸键 | 通过 |
| 文件规范 | Go/Vue/TS/CSS 单文件 ≤500 行 | 通过 |
| 接口回归 | 所有页面渲染后无 4xx/5xx（除种子数据不存在的空列表） | 通过 |

部署验证：Podman 重建 `nw-backend` / `nw-frontend` 后 `/health`、`/ready`、`/healthz` 均通过。
