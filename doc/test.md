# 测试说明

本文档描述 NovelWorkbench 当前实际保留的测试、运行方式，以及集成环境的验证方法。
测试策略聚焦跨模块/跨协议链路：只保留"失败即代表真实行为损坏"的用例，
不保留字段透传、getter/setter、组件快照一类低信息量断言。

## 1. 分层策略

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| Go 单元测试（`go test -short`） | 上下文预算裁剪、斜杠命令解析、切片滑窗、RRF 融合、模板渲染、鉴权状态、密钥封存、用量统计 | CRUD 字段透传、JSON 序列化回显、实现细节 |
| 前端构建检查（`npm run build`） | TypeScript 类型检查与生产构建通过 | 组件快照、样式存在性 |
| 集成环境 E2E | 认证→项目→章节→快照、知识库索引→检索、SSE 对话全链路 | 逐个 UI 控件的机械点击 |

测试原则：表驱动 + `t.Run` 一个用例讲一条链路；`-short` 模式不依赖外部服务；
禁止 `time.Sleep`；同一行为只在一处断言。

## 2. 后端单元测试

运行命令：

```bash
cd backend
CGO_ENABLED=0 go test -short -count=1 -shuffle=on ./...
```

CI 使用相同命令（见 `.github/workflows/ci.yml`），并设置 `GOTOOLCHAIN=local`
与 `CGO_ENABLED=0`，确保与容器静态构建一致。

当前保留的测试文件及覆盖链路：

| 测试文件 | 覆盖链路 |
| --- | --- |
| `backend/internal/ai/prompt_test.go` | 上下文块优先级保留、超预算丢弃（丢弃块可追溯）、同优先级确定性、模板占位符渲染（缺失 → 空串） |
| `backend/internal/service/slash_service_test.go` | 斜杠解析全链路：命中/未命中、参数剥离、禁用命令、非首 token |
| `backend/internal/service/knowledge_service_test.go` | 切片滑窗（短文本/重叠/边界优先/非法重叠收敛）；RRF 融合（双通道/单通道/TopK 截断/来源标注） |
| `backend/internal/service/chat_service_test.go` | 对话上下文装配：技能命中/未命中渲染、@引用注入、历史 N 条截断 + 显式引用必注入、预算丢块标注、设定注入（含 note 排除） |
| `backend/internal/service/knowledge_worker_test.go` | Manticore 中文 ngram 表结构迁移：旧表 `DROP → CREATE → 回灌` 只触发一次，新表启动不重建 |
| `backend/internal/pkg/middleware_test.go` | 鉴权用户状态：活跃放行、禁用/删除旧令牌返回 401、状态查询失败返回 500 |
| `backend/internal/pkg/secret_test.go` | 供应商密钥封存链路：同材料可解、换材料/坏密文必须失败、每次 nonce 随机、空材料拒绝 |
| `backend/internal/service/model_usage_service_test.go` | 用量区间解析（缺省近 14 天/非法回退/起止倒置纠正）与趋势补零（缺失日零值、已有数据不改写） |

## 3. 前端检查

```bash
cd frontend
npm ci
npm run build
```

该命令同时完成 TypeScript 类型检查与 Vite 生产构建，CI 与镜像构建均执行它。
对 SSE 解析器、字数口径等纯逻辑，若需回归，写成可被 `npm run build` 覆盖的
类型安全实现，不引入快照型测试。

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
| 认证 | 注册/登录/鉴权/登出拉黑 |
| 项目→卷→章 | CRUD、目录树、字数账目（含清空正文与删卷的字数回补）、快照留档/回滚/列表、删除后子表零残留 |
| 角色/设定 | 角色卡、关系边、关系图数据；设定增删查（含 note 类型） |
| 知识库 | 上传→队列→切片→落库/建索引→检索命中（source_mode 标注）；失败重试与检索降级 |
| 对话 | SSE 流式（chunk seq 递增 → done 带召回依据）、技能/引用/历史注入、消息落库、AI 字数落账、指定供应商生效 |
| 模型生态 | 供应商 CRUD 与密钥掩码、连通性测试（成功/失败均回写）、默认切换唯一性、用量聚合（调用/失败/token/延迟）随真实调用增长 |
| 运行状态 | 依赖健康探测（正常/未配置降级/不可用）、AI 能力结论与供应商配置一致 |
| 前端 | 登录守卫、项目切换与项目内导航、仪表盘统计、写作台（目录树/编辑器自动保存/SSE 面板/模型选择/插入章节）、关系图、设定管理、知识库上传/检索/预览、笔记、模型供应商、用量分析与运行状态页 |

本次交付已在本地执行并通过的检查：`CGO_ENABLED=0 go test ./...`（后端全部包）、`npx vue-tsc --noEmit` 与 `npm run build`（前端类型检查与生产构建）。浏览器视觉冒烟受当前环境的浏览器网络策略限制，未作为交付证据；UI 验证以生产构建与源码编译为准。

## 5. 环境与注意事项

- CI 只运行 `go test -short`，不连任何外部服务；镜像构建验证见
  `.github/workflows/ci.yml`。
- E2E 不需要真实模型服务；`NW_AI_API_KEY` 为空时 AI 功能返回 `ERR_AI_PROVIDER`，
  其余功能不受影响。
- 所有测试环境变量与部署共用同一套 `NW_*` 名称，详见 `doc/deploy.md` 的环境变量表。
- 测试环境中的口令同样是占位值；不要把真实凭据写入仓库文件，使用 `.env`
  （已被 `.gitignore` 忽略）。
