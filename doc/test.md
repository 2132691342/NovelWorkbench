# 测试说明

本文描述 NovelWorkbench 的测试策略、保留的用例与集成验证方法。测试的设计原则是**少而准**：
只保留覆盖复杂流程、长链路与高风险配置的用例，其余交给类型检查与集成验证。

## 1. 测试策略

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| Go 单元测试（`go test -short`） | 跨步骤链路：访问控制、对话上下文装配、技能解析、检索融合、索引迁移、密钥封存、菜单权限、内置资产完整性 | CRUD 字段透传、JSON 回显、常量查表、SQL 细节、单函数分支穷举 |
| 前端单元测试（`vitest run`） | 纯函数链路：SSE 报文解析、流式事件归并 | 组件渲染、样式存在性、路由跳转 |
| 类型检查与构建（`vue-tsc && vite build`） | TypeScript 类型与生产构建 | —— |
| 集成验证 | 需要中间件与模型网关的完整业务链路 | 逐个控件的机械点击 |

原则：

- **一个用例讲一条链路**。多个环节串在同一 `Test*` 内，用「环节N」注释分隔；失败时看注释定位到具体环节。
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
go test -short -run TestChatContextChain ./internal/service/   # 按用例名
go test -short -v ./internal/service/                           # 看每个环节的断言
```

### 2.2 用例（6 个文件 / 9 条）

| 文件 | 用例 | 覆盖链路 |
| --- | --- | --- |
| `backend/internal/pkg/access_test.go` | `TestAccessControlChain` | 请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库 → 响应包络；拒绝路径覆盖无令牌 / 令牌已吊销 / 账号禁用（401）与权限不足（403） |
| `backend/internal/pkg/secret_test.go` | `TestSecretSealChain` | 密钥封存 → 解封还原 → 密文不可读 → 篡改与材料变更被拒（认证标签校验） |
| `backend/internal/ai/builtin_assets_test.go` | `TestBuiltinAssetIntegrity` | 内置资产矩阵：9 个流程域各一档案两技能、编码全局唯一、前端依赖的 5 个技能编码存在、模板占位符全部可解析、默认档案是 `novelist` |
| `backend/internal/ai/client_test.go` | `TestProviderInstancePolicy` | 模型实例缓存键随密钥/协议/地址/模型/参数变化；流式 HTTP 客户端不设整体超时（否则长流被拦腰切断） |
| `backend/internal/service/chat_pipeline_test.go` | `TestSkillResolutionChain` | 技能编码解析（显式优先 / 正文首 token）→ 作用域优先级 → 变量取值与必填缺失 → 模板占位符校验 → 参考资料关键词命中与 4000 字符预算 → 全局技能覆盖标记 |
| `backend/internal/service/chat_pipeline_test.go` | `TestChatContextChain` | 技能模板渲染 → 自由对话 → @引用分优先级 → 设定注入排除私人笔记 → 显式引用摘要与档案 → 同优先级稳定顺序 → 12000 字符预算丢块 → 历史选取 → 消息序列成型 → 显式引用会话归属校验 |
| `backend/internal/service/chat_pipeline_test.go` | `TestChatStreamContract` | SSE `thinking` → `chunk` → `done` 顺序、seq 从 1 单调递增、`done` 携带完整思考过程与空数组引用 |
| `backend/internal/service/knowledge_service_test.go` | `TestKnowledgeRetrievalChain` | 文本切片（重叠、边界、非法参数）→ 查询词抽取 → RRF 双通道融合 → 单通道排序与 TopK → 重排解析与 fail-open |
| `backend/internal/service/knowledge_service_test.go` | `TestManticoreSchemaMigrationChain` | Manticore 表结构识别 → 旧表重建 → 回灌只执行一次；新结构启动不重建 |
| `backend/internal/service/menu_service_test.go` | `TestMenuAndPermissionChain` | 导航可见性过滤 → 菜单树装配 → 权限目录汇总 → 重复权限码去重 → 分组编码与展示名 |

关键断言举例：

| 用例 | 断言要点 |
| --- | --- |
| 访问控制 | 放行时 HTTP 200、`resp_code=SUCCESS`、`trace_id` 等于注入的 `X-Request-ID`、审计恰好 1 条且动作码与目标正确 |
| 密钥封存 | 同一明文两次封存得到不同密文（随机 nonce）；篡改末位、非法 base64、材料变更均返回 `ErrSecretInvalid` |
| 上下文装配 | `skill` 块优先级 100 且渲染结果即最终指令；`skill_reference` 优先级 85；预算不足时先丢 `chapter_summary` 与 `lore` |
| 检索融合 | 跨通道命中切片排首位，得分等于 `1/62 + 1/61`，来源标记 `hybrid`；单通道按 `1/(60+rank)` 降序 |
| 索引迁移 | 旧表返回 `rebuilt=true` 且语句序列为 `SHOW CREATE TABLE` → `DROP TABLE` → 含 `ngram_len='1'` 的建表；再次调用返回 `false` |
| 菜单权限 | 动作节点、隐藏节点、未授权权限码均不进导航树；通配 `*` 放行全部；权限码在目录中只出现一次 |

### 2.3 为什么这些用例留下来

| 用例 | 留它的理由 |
| --- | --- |
| 访问控制 | 中间件链顺序、鉴权与审计的耦合点最多，改一处容易连带出错 |
| 密钥封存 | 涉及密码学，改错不会编译报错，只会在生产表现为"连接性测试一直失败" |
| 内置资产完整性 | 播种会写入数据库，形状错会污染生产数据且难以回滚 |
| 模型实例策略 | 缓存键与超时策略都是"配错只在运行时暴露"的项 |
| 技能解析 | 上下文装配的前置，直接决定 `/命令` 与界面按钮是否命中正确资产 |
| 上下文装配 | 全系统最复杂的纯函数组合，优先级 + 预算 + 多来源交织，改动频繁 |
| 流式契约 | 前后端共用的事件序列约定，后端改了前端就崩 |
| 检索链路 | 切片与融合是同一条检索链的上下游，融合权重与排序错得静默 |
| 索引迁移 | 涉及 DDL 与一次性回灌，重复执行会造成数据重复 |
| 菜单权限 | 导航、权限判定、角色授权三处共用同一套过滤规则 |

## 3. 前端测试

```bash
cd frontend
npm ci
npm run test        # vitest run
```

`frontend/vitest.config.ts`：`environment: 'node'`、`include: ['src/**/*.test.ts']`，复用 `@` 别名。

| 文件 | 用例 | 覆盖链路 |
| --- | --- | --- |
| `frontend/src/composables/useSSE.test.ts` | `parseSseBuffer 事件解析链路` | 跨 chunk 断行、多事件粘包、CRLF、非法 JSON 跳过、心跳注释忽略、尾部半行留存 |
| `frontend/src/views/workspace/chatStreamState.test.ts` | `applyStreamEvent 流式事件归并链路` | 思考与正文分别累加、`done` 用服务端全量覆盖并补齐召回依据、`error` 落兜底文案、未知事件不改写消息 |

组件渲染与样式存在性不设断言；类型正确性与生产产物由 `npm run build` 保证，CI 与镜像构建都执行它。

## 4. 集成验证

### 4.1 环境

集成验证需要 PostgreSQL / Redis / RabbitMQ / MinIO / Manticore，以及一个 OpenAI 兼容的 mock 网关
（只需实现 `/v1/chat/completions` 流式响应，用于覆盖生成、重排与连通性测试）。

```bash
cp .env.example .env
# 填入：NW_JWT_SECRET、NW_POSTGRES_*、NW_REDIS_*、NW_RABBITMQ_*、NW_MINIO_*、
#       NW_MANTICORE_BASE_URL、NW_AI_BASE_URL（指向 mock）、NW_AI_API_KEY、NW_AI_CHAT_MODEL、NW_APP_ADMIN_PASSWORD
docker compose --profile infra up -d --build
```

启动后自动建表（33 张）、播种内置技能（18 个）与提示词档案（9 份）、默认配置、内置角色与菜单；
配置了 `NW_AI_BASE_URL` 与 `NW_AI_CHAT_MODEL` 时再播种默认供应商；用户表为空时创建 `admin`。

mock 网关不可用时，项目、卷章、角色、设定、规划、连续性、知识库上传与检索仍可验证，AI 生成类操作返回 `ERR_AI_PROVIDER`。

### 4.2 验证清单

| 链路 | 验证点 |
| --- | --- |
| 认证 | 无注册接口；登录名或邮箱均可登录；登出后旧令牌立即失效；禁用账号后旧令牌失效；响应不含密码哈希 |
| 菜单与权限 | `/menu/nav` 只下发启用 + 展示 + 有权限的非动作节点；仅持 `skill:manage` 的账号看不到系统分组且访问管理页被重定向；新增菜单后导航与权限目录同步；删除有下级的菜单被拒；重复 `perm_code` 被拒 |
| 技能 | 内置技能 18 条（9 个流程域各 2 条，见 [skills.md](skills.md)）；代码资产改版后按指纹升级、作者改过的行保持原样；旧内置技能在启动时连同参考资料被下线；项目技能与全局技能合并返回并标记 `overridden_flag`；`/skill/resolve` 返回编码、变量与缺失项；参考资料关键词命中才进 `ref_text_list`；模板含未声明占位符保存被拒；内置技能删除被拒 |
| 项目与卷章 | CRUD、目录树、排序号；字数账目（改正文、清空、删章、删卷）；快照留档与回滚；删除项目后子表零残留且 MinIO / Manticore 副本被清理 |
| 设定 | 角色卡 CRUD 与过滤；关系边增删、自环与跨项目被拒；关系图节点与边数正确；删角色后关系边清理；设定与 `note` 笔记增删查 |
| 创作规划 | 大纲树层级与排序；父节点设为自身或后代被拒；有子节点时删除被拒；场景卡与节点绑定并按节点过滤 |
| 连续性 | 候选事实不自动转正；复核只推进到 `confirmed` / `superseded` / `stale`；伏笔状态流转；摘要同章节重复保存不产生重复行 |
| 知识库 | 上传 → 队列 → 切片 → 落库 → 索引 → 检索命中并标注 `source_mode`；同哈希切片复用 ULID；失败重试与 `failed` 状态；重投索引；删除后对象、索引、切片、文档全清 |
| 对话 | SSE `thinking` → `chunk` → `done` 序号单调递增，`done` 带完整思考与召回依据；指定 `prompt_profile_ulid` 时系统提示词切换为档案正文；技能命中与自由对话两条路径；@引用按优先级注入；历史条数取配置值；消息落库与 AI 字数落账；指定 `model_ulid` 生效；停止生成不落库助手消息 |
| 模型生态 | 供应商 CRUD 与密钥掩码；首次创建必须填 API Key；供应商下仍有模型时拒删；系统默认唯一且默认模型不可删；连通性测试回写状态与时间；按角色分配后普通用户只见授权模型，未授权调 `/chat/send` 返回 `ERR_FORBIDDEN`；用量随真实调用增长且缺日补零 |
| 运行状态 | 依赖健康探测（正常 / 未配置 / 不可用）；必需依赖与可选依赖的失败级别不同；构建版本与运行时长展示 |
| 提示词档案 | 内置 9 份且不可删；项目档案编码重复被拒；项目档案越权被拒；停用档案后对话回退默认档案 |
| 前端 | 所有增删改查弹窗/抽屉打开后有内容；登录页无注册入口；守卫与令牌失效跳转；写作台目录树、自动保存、AI 面板 SSE（等待态秒数递增 → 思考态自动展开 → 正文出现后自动收起）、斜杠菜单分组与键盘导航、@ 五页签引用、档案选择、插入章节与替换整章；知识库原文预览；菜单管理改动后侧边栏变化；关系图；笔记自动保存；技能管理；用量与运行状态页 |

## 5. 注意事项

- CI 只跑 `go test -short` 与 `npm run test`，不连接外部服务。
- `-short` 是当前唯一测试模式开关，为将来可选集成链路预留。
- 集成验证与部署共用同一套 `NW_*` 变量名，完整变量表见 `doc/deploy.md`。
- 测试环境口令同样是占位值，真实凭据只放 `.env`（已被忽略）。
