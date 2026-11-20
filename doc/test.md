# 测试说明

本文描述 NovelWorkbench 保留的测试、运行方式与集成验证方法，只写当前存在的用例。
事实来源为 `backend/internal/**/*_test.go`、`frontend/src/**/*.test.ts`、`.github/workflows/ci.yml`。

## 1. 测试策略

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| Go 单元测试（`go test -short`） | 跨步骤链路：请求访问控制、上下文装配、技能解析、检索融合、索引迁移、密钥封存、菜单权限 | CRUD 字段透传、JSON 回显、常量查表、SQL 细节 |
| 前端单元测试（`vitest run`） | 纯函数链路：SSE 解析、笔记正文双模式转换 | 组件渲染、样式存在性、路由跳转 |
| 类型检查与构建（`vue-tsc && vite build`） | TypeScript 类型与生产构建 | —— |
| 集成验证 | 需要中间件与模型网关的完整业务链路 | 逐个控件的机械点击 |

原则：

- 一个用例讲一条链路，多个环节串在同一 `Test*` 内，用「环节N」注释分隔。
- 表驱动只用于同一行为的多个输入（如鉴权的拒绝路径）。
- `-short` 模式不依赖外部服务：HTTP 依赖用 `httptest.NewServer`，接口用闭包替身。
- 不使用 `time.Sleep`；需要顺序时用确定的输入顺序表达。
- 断言面向可观察结果：业务码、HTTP 状态、包含关系、顺序、条数。
- 业务规则写成无依赖纯函数（可见性过滤、作用域优先级、变量取值、关键词命中、预算裁剪），测试直接驱动，不引入数据库或 mock 框架。
- 只保留长链路与复杂场景；单函数、常量表、简单 CRUD 不单独建用例。

## 2. 后端测试

### 2.1 运行

```bash
cd backend
CGO_ENABLED=0 go test -short -count=1 -shuffle=on ./...
```

CI（`.github/workflows/ci.yml` 的 backend job）依次执行 `go vet ./...`、
`CGO_ENABLED=0 go build -tags "netgo osusergo" ./...` 与上述测试命令，环境变量为
`GOTOOLCHAIN=local`（与 Dockerfile 一致）与 `CGO_ENABLED=0`。

### 2.2 用例（4 个文件 / 7 条）

| 文件 | 用例 | 覆盖链路 |
| --- | --- | --- |
| `backend/internal/pkg/access_test.go` | `TestAccessControlChain` | 请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库 → 响应包络；拒绝路径覆盖无令牌 / 令牌已吊销 / 账号禁用（401）与权限不足（403） |
| `backend/internal/pkg/access_test.go` | `TestSecretSealOpenChain` | 密钥封存：同材料可解、换材料失败、坏密文失败、空材料失败、nonce 随机 |
| `backend/internal/service/chat_pipeline_test.go` | `TestSkillResolutionChain` | 技能编码解析（显式优先 / 正文首 token）→ 作用域优先级 → 变量取值与必填缺失 → 模板占位符校验 → 参考资料关键词命中与 4000 字符预算 → 关键词归一 → 全局技能覆盖标记 |
| `backend/internal/service/chat_pipeline_test.go` | `TestChatContextChain` | 技能模板渲染 → 自由对话 → @引用分优先级 → 设定注入排除私人笔记 → 同优先级稳定顺序 → 12000 字节预算丢块 → 历史选取 → 消息序列成型 → 显式引用会话归属校验 |
| `backend/internal/service/knowledge_service_test.go` | `TestKnowledgeRetrievalChain` | 文本切片（重叠、边界、非法参数）→ 查询词抽取 → 检索模式归一 → RRF 双通道融合 → 单通道排序与 TopK → 重排解析与 fail-open → 默认召回条数回退 |
| `backend/internal/service/knowledge_service_test.go` | `TestManticoreSchemaMigrationChain` | Manticore 表结构识别 → 旧表重建 → 回灌只执行一次；新结构启动不重建 |
| `backend/internal/service/menu_service_test.go` | `TestMenuAndPermissionChain` | 导航可见性过滤 → 菜单树装配 → 权限目录汇总 → 重复权限码去重 → 分组编码与展示名 |

关键断言举例：

| 用例 | 断言要点 |
| --- | --- |
| 访问控制 | 放行时 HTTP 200、`resp_code=SUCCESS`、`trace_id` 等于注入的 `X-Request-ID`、审计恰好 1 条且动作码与目标正确 |
| 上下文装配 | `skill` 块优先级 100 且渲染结果即最终指令；`skill_reference` 优先级 85；预算不足时先丢 `history` 与 `chapter_summary` |
| 检索融合 | 跨通道命中切片排首位，得分等于 `1/62 + 1/61`，来源标记 `hybrid`；单通道按 `1/(60+rank)` 降序 |
| 索引迁移 | 旧表返回 `rebuilt=true` 且语句序列为 `SHOW CREATE TABLE` → `DROP TABLE` → 含 `ngram_len='1'` 的建表；再次调用返回 `false` |
| 菜单权限 | 动作节点、隐藏节点、未授权权限码均不进导航树；通配 `*` 放行全部；权限码在目录中只出现一次 |

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
| `frontend/src/views/notes/noteContent.test.ts` | `convertNoteContent 笔记正文格式转换` | HTML→Markdown 不残留标签、Markdown→HTML、同格式原样返回、空正文保持空串 |

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

启动后自动建表（32 张）、播种内置技能（21 个）、默认配置、内置角色与菜单；配置了 `NW_AI_BASE_URL` 与 `NW_AI_CHAT_MODEL` 时再播种默认供应商；用户表为空时创建 `admin`。

mock 网关不可用时，项目、卷章、角色、设定、规划、连续性、知识库上传与检索仍可验证，AI 生成类操作返回 `ERR_AI_PROVIDER`。

### 4.2 验证清单

| 链路 | 验证点 |
| --- | --- |
| 认证 | 无注册接口；登录名或邮箱均可登录；登出后旧令牌立即失效；禁用账号后旧令牌失效；响应不含密码哈希 |
| 菜单与权限 | `/menu/nav` 只下发启用 + 展示 + 有权限的非动作节点；仅持 `skill:manage` 的账号看不到系统分组且访问管理页被重定向；新增菜单后导航与权限目录同步；删除有下级的菜单被拒；重复 `perm_code` 被拒 |
| 技能 | 内置技能 21 条；项目技能与全局技能合并返回并标记 `overridden_flag`；`/skill/resolve` 返回编码、变量与缺失项；参考资料关键词命中才进 `ref_text_list`；模板含未声明占位符保存被拒；内置技能删除被拒 |
| 项目与卷章 | CRUD、目录树、排序号；字数账目（改正文、清空、删章、删卷）；快照留档与回滚；删除项目后子表零残留且 MinIO / Manticore 副本被清理 |
| 设定 | 角色卡 CRUD 与过滤；关系边增删、自环与跨项目被拒；关系图节点与边数正确；删角色后关系边清理；设定与 `note` 笔记增删查 |
| 创作规划 | 大纲树层级与排序；父节点设为自身或后代被拒；有子节点时删除被拒；场景卡与节点绑定并按节点过滤 |
| 连续性 | 候选事实不自动转正；复核只推进到 `confirmed` / `superseded` / `stale`；伏笔状态流转；摘要同章节重复保存不产生重复行 |
| 知识库 | 上传 → 队列 → 切片 → 落库 → 索引 → 检索命中并标注 `source_mode`；同哈希切片复用 ULID；失败重试与 `failed` 状态；重投索引；删除后对象、索引、切片、文档全清 |
| 对话 | SSE `chunk` 序号单调递增 → `done` 带召回依据；技能命中与自由对话两条路径；@引用按优先级注入；历史条数取配置值；消息落库与 AI 字数落账；指定 `model_ulid` 生效；停止生成不落库助手消息 |
| 模型生态 | 供应商 CRUD 与密钥掩码；首次创建必须填 API Key；供应商下仍有模型时拒删；系统默认唯一且默认模型不可删；连通性测试回写状态与时间；按角色分配后普通用户只见授权模型，未授权调 `/chat/send` 返回 `ERR_FORBIDDEN`；旧版供应商单模型数据自动迁移；用量随真实调用增长且缺日补零 |
| 运行状态 | 依赖健康探测（正常 / 未配置 / 不可用）；必需依赖与可选依赖的失败级别不同；构建版本与运行时长展示 |
| 前端 | 登录页无注册入口；守卫与令牌失效跳转；写作台目录树、自动保存、AI 面板 SSE、技能补全、模型选择、插入章节与替换整章；知识库原文预览；菜单管理改动后侧边栏变化；关系图；笔记自动保存；技能管理；用量与运行状态页 |

## 5. 注意事项

- CI 只跑 `go test -short` 与 `npm run test`，不连接外部服务。
- `-short` 是当前唯一测试模式开关，为将来可选集成链路预留。
- 集成验证与部署共用同一套 `NW_*` 变量名，完整变量表见 `doc/deploy.md`。
- 测试环境口令同样是占位值，真实凭据只放 `.env`（已被忽略）。
