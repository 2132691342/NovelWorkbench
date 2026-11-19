# 测试说明

本文描述 NovelWorkbench 当前保留的测试、运行方式与集成环境验证方法。

测试策略聚焦跨模块、跨协议的复杂链路：只保留"失败即代表真实行为损坏"的用例。
字段透传、getter/setter、常量表查表、组件快照、样式存在性一类低信息量断言一律不保留。

## 1. 分层策略

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| Go 单元测试（`go test -short`） | 请求生命周期、上下文装配、检索融合、切片算法、密钥封存、用量聚合、索引迁移等跨步骤链路 | CRUD 字段透传、JSON 序列化回显、静态常量映射表、实现细节 |
| 前端单元测试（`npm run test`） | SSE 事件解析这类纯函数链路 | 组件渲染、样式存在性 |
| 类型检查与构建（`npm run build`） | TypeScript 类型正确性与生产构建通过 | —— |
| 集成环境 E2E | 认证→项目→章节→快照、知识库索引→检索、SSE 对话全链路 | 逐个 UI 控件的机械点击 |

测试原则：

- 一个用例讲一条链路，多个环节串在同一个 `Test*` 函数里，用注释标出"环节一/二/三"。
- 表驱动只用于"同一行为的多个输入"（如鉴权的四种拒绝路径）。
- `-short` 模式不依赖外部服务；禁止 `time.Sleep`；同一行为只在一处断言。
- 断言面向可观察结果（业务码、状态码、内容包含关系、顺序），不面向内部实现细节。
- 不为单个中间件或单个函数单独建用例：它没有链路语义，失败了也无法定位上下文。

## 2. 后端单元测试

运行命令：

```bash
cd backend
CGO_ENABLED=0 go test -short -count=1 -shuffle=on ./...
```

CI 使用相同命令（见 `.github/workflows/ci.yml`），并设置 `GOTOOLCHAIN=local`
与 `CGO_ENABLED=0`，确保与容器静态构建一致。

当前保留 9 条用例，每条覆盖一条完整链路：

| 测试文件 | 用例 | 覆盖链路 |
| --- | --- | --- |
| `backend/internal/pkg/access_test.go` | `TestRequestLifecycleChain` | 请求生命周期：请求 ID 透传 → 鉴权 → 权限判定 → 审计落库 → 响应包络，五段串成一条 |
| `backend/internal/pkg/access_test.go` | `TestAuthRejectionChain` | 鉴权拒绝四条路径：无令牌 / 令牌已吊销 / 账号已禁用 → 401，权限不足 → 403 |
| `backend/internal/pkg/secret_test.go` | `TestSecretSealOpenChain` | 供应商密钥封存：同材料可解、换材料与坏密文必须失败、空材料拒绝、nonce 随机 |
| `backend/internal/ai/prompt_test.go` | `TestContextAssemblyChain` | 上下文装配：优先级排序、超预算丢块、空块跳过、同优先级稳定、占位符渲染、系统提示词注入文风 |
| `backend/internal/service/chat_service_test.go` | `TestChatContextAssemblyChain` | AI 对话上下文全链路（八个环节）：技能命中渲染 → 自由对话 → @引用分优先级注入 → 设定注入且私人笔记排除 → 预算裁剪丢块 → 历史选取（最近 N 条 + 显式引用去重置尾）→ 消息序列成型 → 显式引用会话归属校验 |
| `backend/internal/service/knowledge_service_test.go` | `TestKnowledgeRetrievalChain` | 无向量检索全链路：文本切片（滑窗重叠/边界优先/非法参数收敛）→ 查询词抽取（CJK 2-gram 与拉丁词）→ 检索模式归一 → RRF 双通道融合与单通道排序与 TopK → chat 重排结果解析与 fail-open |
| `backend/internal/service/knowledge_manticore_test.go` | `TestEnsureManticoreSchemaMigratesLegacyTable` | Manticore 中文 ngram 表结构迁移：旧表 `DROP → CREATE → 回灌` 只触发一次，新表启动不重建 |
| `backend/internal/service/model_usage_service_test.go` | `TestUsageAggregationChain` | 用量分析三环节：查询区间解析（缺省近 14 天 / 非法回退 / 起止倒置纠正）→ 缺日补零 → 按调用数加权的平均延迟 |
| `backend/internal/service/slash_service_test.go` | `TestSlashParsingChain` | 斜杠命令解析：命中与参数分离、无参数、空白归一、未命中（自由正文 / 未知命令 / 禁用命令 / 命令不在首 token）正文原样保留 |

## 3. 前端单元测试

```bash
cd frontend
npm ci
npm run test
```

当前保留 1 条用例：

| 测试文件 | 用例 | 覆盖链路 |
| --- | --- | --- |
| `frontend/src/composables/useSSE.test.ts` | `parseSseBuffer 事件解析链路` | SSE 事件解析：跨 chunk 断行、多事件粘包、CRLF 兼容、非法 JSON 行跳过、心跳注释行忽略、尾部半行留存 |

SSE 解析直接决定 AI 流式回复能否正确落到界面，解析偏差会造成事件丢失或重复，
因此作为一条完整链路用例保留。

前端组件渲染与样式存在性一类断言不保留；类型正确性与生产产物由
`npm run build`（`vue-tsc && vite build`）保证，CI 与镜像构建均执行它。

## 4. 集成环境验证（E2E）

E2E 在集成环境执行：后端直连 PostgreSQL / Redis / RabbitMQ / MinIO / Manticore，
模型网关使用本地 mock（OpenAI 兼容 `/v1/chat/completions` 流式响应），
验证本系统链路而非模型输出质量。可用根目录 Compose 的 `infra` profile
快速拉起中间件，再以本地 mock 地址填入 `NW_AI_BASE_URL`：

```bash
cp .env.example .env
# 在 .env 中设置：
#   NW_AI_BASE_URL=http://127.0.0.1:<mock-port>/v1
#   NW_AI_API_KEY=test-key
#   NW_AI_CHAT_MODEL=test-chat-model
#   NW_JWT_SECRET、数据库/缓存/对象存储/队列口令均填本地测试值
docker compose --profile infra up -d --build
curl http://127.0.0.1:8080/ready
```

集成验证清单（需在具备中间件与 mock 模型服务的环境中执行）：

| 链路 | 验证点 |
| --- | --- |
| 认证 | 注册/登录/鉴权/登出拉黑；禁用账号旧令牌立即失效 |
| 项目→卷→章 | CRUD、目录树、字数账目（含清空正文与删卷的字数回补）、快照留档/回滚/列表、删除项目后子表零残留 |
| 角色/设定 | 角色卡、关系边、关系图数据；设定增删查（含 note 类型）；删除角色后关系边清理 |
| 创作规划 | 大纲树层级与排序、有子节点不可删、父节点成环被拒、场景卡与节点绑定 |
| 连续性 | 候选事实不自动转正、复核只能推进不能回退、伏笔状态流转、摘要 stale 标记 |
| 知识库 | 上传→队列→切片→落库/建索引→检索命中（source_mode 标注）；失败重试与检索降级 |
| 对话 | SSE 流式（chunk seq 递增 → done 带召回依据）、技能/引用/历史注入、消息落库、AI 字数落账、指定供应商生效 |
| 模型生态 | 供应商 CRUD 与密钥掩码、连通性测试（成功/失败均回写）、默认切换唯一性、用量聚合随真实调用增长 |
| 运行状态 | 依赖健康探测（正常/未配置降级/不可用）、AI 能力结论与供应商配置一致 |
| 前端 | 登录守卫、项目切换与项目内导航、仪表盘统计、写作台（目录树/编辑器自动保存/SSE 面板/模型选择/插入章节）、关系图、设定管理、知识库上传/检索/预览、笔记、模型供应商、用量分析与运行状态页 |

## 5. 环境与注意事项

- CI 只运行 `go test -short` 与 `npm run test`，不连任何外部服务；
  镜像构建验证见 `.github/workflows/ci.yml`。
- E2E 不需要真实模型服务；`NW_AI_API_KEY` 为空时 AI 功能返回 `ERR_AI_PROVIDER`，
  其余功能不受影响。
- 所有测试环境变量与部署共用同一套 `NW_*` 名称，详见 `doc/deploy.md` 的环境变量表。
- 测试环境中的口令同样是占位值；不要把真实凭据写入仓库文件，使用 `.env`
  （已被 `.gitignore` 忽略）。
