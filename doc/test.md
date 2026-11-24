# 测试说明

本文说明 NovelWorkbench 的测试策略、保留的用例与集成验证方法。

测试的原则是**少而准**：只保留覆盖长链路与高风险配置的用例。判据是「配错才在生产暴露」——鉴权绕过、密钥解不开、上下文装配偏移、检索召回失真这类问题，单元测试能提前挡住；而 CRUD 字段透传、常量查表、JSON 回显这类由类型检查与集成验证兜住的偏差，不值得为它写一条用例。

## 1. 测试策略

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| Go 单元测试（`go test -short`） | 跨步骤链路：访问控制、密钥封存、技能解析、上下文装配、流式事件契约、检索融合、索引迁移、菜单权限、内置资产完整性 | CRUD 字段透传、JSON 回显、常量查表、SQL 细节、单函数分支穷举 |
| 前端单元测试（`vitest run`） | 流式报文解析与事件归并 | 组件渲染、样式存在性、路由跳转 |
| 类型检查与构建（`vue-tsc && vite build`） | TypeScript 类型与生产构建 | -- |
| 集成验证 | 需要中间件与模型网关的完整业务链路 | 逐个控件的机械点击 |

原则：

- **一个用例讲一条链路**。多个环节串在同一 `Test*` 内，用「步骤N」注释分隔；失败时看注释定位到具体环节。
- **同域的链路放同一文件**。对话链路的四段（技能解析、上下文装配、工具触发、流式契约）都发生在同一次请求里，分散到多个文件会让定位变难，因此集中在 `chat_pipeline_test.go`。
- **断言跨环节的因果**，不重复断言代码自身保证的约束。例：断言「项目技能覆盖全局后全局技能被标记为已覆盖」，而不是逐字段核对枚举取值表。
- **表驱动只用于同一行为的多个输入**（如鉴权的拒绝路径）。
- `-short` 模式不依赖外部服务：HTTP 依赖用 `httptest.NewServer`，接口用闭包替身。
- 不使用 `time.Sleep`；需要顺序时用确定的输入顺序表达。
- 断言面向可观察结果：业务码、HTTP 状态、包含关系、顺序、条数。
- 业务规则写成无依赖纯函数（可见性过滤、作用域优先级、变量取值、关键词命中、预算裁剪），测试直接驱动，不引入数据库与 mock 框架。
- 不测「接线」（路由是否注册、字段是否透传），这类偏差由类型检查与集成验证兜住。

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

后端共 8 个测试文件、16 条用例，每条用例对应一条完整链路。

| 文件 | 用例 | 覆盖链路 |
| --- | --- | --- |
| `backend/internal/pkg/security_chain_test.go` | `TestAccessControlChain` | 请求 ID 透传 → 鉴权注入用户 → 权限判定 → 审计落库 → 响应包络；拒绝路径覆盖无令牌 / 令牌已吊销 / 账号禁用（401）与权限不足（403） |
| `backend/internal/pkg/security_chain_test.go` | `TestSecretSealChain` | 密钥封存 → 解封还原 → 密文不可读 → 篡改与材料变更被拒（认证标签校验） |
| `backend/internal/ai/builtin_assets_test.go` | `TestBuiltinAssetChain` | 内置资产从定义到可播种：域形状（12 域各一档案两技能）→ 编码全局唯一 → 模板占位符全部已登记 → 指纹为 sha256 |
| `backend/internal/ai/builtin_assets_test.go` | `TestProviderInstanceChain` | 模型实例缓存键随密钥/协议/地址/模型/参数变化；流式 HTTP 客户端不设整体超时（否则长流被拦腰切断） |
| `backend/internal/ai/bootstrap_test.go` | `TestParseBootstrapPlanChain` | 开书方案解析：代码围栏与前后缀文字容错 → 书名/卷章/角色字段完整取出 → 无 JSON / 缺书名 / 缺卷章 / 非法 JSON 一律报错 |
| `backend/internal/service/chat_pipeline_test.go` | `TestSkillResolutionChain` | 技能编码解析（显式优先 / 正文首 token）→ 作用域优先级 → 变量取值与必填缺失 → 模板占位符校验 → 参考资料关键词命中与 4000 字符预算 → 全局技能覆盖标记 |
| `backend/internal/service/chat_pipeline_test.go` | `TestChatContextChain` | 技能模板渲染 → 自由对话 → @引用分优先级 → 设定注入排除私人笔记 → 显式引用摘要与档案 → 同优先级稳定顺序 → 12000 字符预算丢块 → 历史选取 → 消息序列成型 → 显式引用会话归属校验 |
| `backend/internal/service/chat_pipeline_test.go` | `TestToolTriggerChain` | 正文 `#code` 解析（行首/行中命中，词内井号、大写与无井号不命中，剥离时保留换行）→ 编码合并去重截断 → 显式选取与缺失上报 → 关键词补位跳过已选与停用 |
| `backend/internal/service/chat_pipeline_test.go` | `TestChatStreamContract` | SSE `thinking` → `chunk` → `done` 顺序、seq 从 1 单调递增、`done` 携带完整思考过程、空数组引用与本轮工具编码 |
| `backend/internal/service/knowledge_chain_test.go` | `TestKnowledgeRetrievalChain` | 文本切片（重叠窗口、非法 overlap 收敛）→ 查询词抽取 → RRF 双通道融合 → 单通道排序与 TopK → 重排解析与 fail-open |
| `backend/internal/service/knowledge_chain_test.go` | `TestManticoreSchemaMigrationChain` | Manticore 表结构识别 → 旧表重建 → 回灌只执行一次；新结构启动不重建 |
| `backend/internal/service/menu_chain_test.go` | `TestMenuAndPermissionChain` | 可下发过滤（隐藏页保留标记）→ 菜单树装配与隐藏标记透传 → 权限目录汇总 → 重复权限码去重 → 动作与菜单权限说明区分 |
| `backend/internal/service/menu_seed_test.go` | `TestMenuSeedComponentFiles` | 每个内置菜单的 `component_path` 在 `frontend/src` 下有对应页面文件（防止登记了菜单却没有页面） |
| `backend/internal/service/menu_seed_test.go` | `TestMenuSeedUniqueCodes` | 菜单编码与路由路径全局唯一（前端以编码生成唯一路由名） |
| `backend/internal/service/menu_seed_test.go` | `TestProfileMenuRegistered` | 「个人中心」入口固定登记为隐藏页，顶栏用户菜单点击不会 404 |
| `backend/internal/ai/check/chapter_check_test.go` | `TestChapterCheckChain` | 空正文判阻断 → 零容忍词计数与分级 → 缺钩子警示 → 干净正文不误判 → 同一输入结论确定性 |

### 2.3 覆盖取向

保留的用例集中在四处：

1. **权限与凭据**。鉴权绕过与密钥解不开都是「配错才在生产暴露」的问题。
2. **上下文装配**。技能、引用、设定、摘要、历史与预算的交互是全系统最复杂的部分，一旦偏移会静默产出错误上下文。
3. **检索链路**。切片、查询词、融合与重排是一条上下游链路，中途改动会直接影响召回质量。
4. **出厂资产**。内置技能、档案与工具的形状、编码唯一性与占位符合法性直接决定播种产物能否使用。
5. **量化门禁**。体检阈值与分级一旦漂移，写作台与对话期工具会给出互相矛盾的结论。

其余（CRUD 透传、字段校验、常量查表）由类型检查与集成验证兜住，不单独建用例。

## 3. 前端测试

### 3.1 运行

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm test # vitest run
```

`frontend/vitest.config.ts`：`environment: 'node'`，`include` 覆盖 `src/**/*.test.ts`，复用 `@` 别名。

### 3.2 用例清单

前端保留两条链路：流式对话的报文解析与事件归并，以及路由路径装配。前者决定 AI 回复能否正确落到界面——解析偏差会丢事件或重复事件，归并偏差会让正文重复累加或思考被清空；后者决定「点菜单能不能打开页面」——路径拼接多一个结尾斜杠，`strict` 路由就会整片落到 404，而控制台不会报任何错。

| 文件 | 用例 | 覆盖链路 |
| --- | --- | --- |
| `frontend/src/views/Workspace/streamChain.test.ts` | 报文解析链路 | 跨 chunk 断行、多事件粘包、CRLF、非法 JSON 跳过、心跳注释忽略、尾部半行留存 |
| `frontend/src/views/Workspace/streamChain.test.ts` | 事件归并链路 | 思考与正文分别累加、`done` 用服务端全量覆盖并补齐召回依据、`error` 落兜底文案、未知事件不改写消息 |
| `frontend/src/utils/routerHelper.test.ts` | `pathResolve` | 空子路径不产生结尾斜杠、根路径保持单斜杠、绝对路径与外部链接原样返回 |
| `frontend/src/utils/routerHelper.test.ts` | `routeNameOf` | 菜单编码转 PascalCase 唯一路由名（下划线与连字符都归一） |
| `frontend/src/utils/routerHelper.test.ts` | `substituteProject` | `:project_ulid` 占位替换保持树形结构；无当前项目时保留占位符 |

组件渲染与样式存在性不设断言；类型正确性与生产产物由 `pnpm build` 保证，CI 与镜像构建都执行它。

## 4. 集成验证

需要真实中间件与模型网关的链路无法在单元测试中覆盖，通过部署后的实际请求验证：

| 场景 | 验证方式 |
| --- | --- |
| 启动与播种 | 后端启动日志显示迁移 35 张表、播种内置技能与菜单；`/health` 与 `/ready` 返回 200 |
| 登录与导航 | `POST /api/v1/auth/login` 取令牌 → `GET /api/v1/menu/nav` 返回按权限过滤后的导航树 |
| 业务接口 | 按模块调用列表/详情/写入接口，确认业务码为 `SUCCESS` 且字段符合契约 |
| AI 流式 | `POST /api/v1/chat/send` 观察 `thinking` → `chunk` → `done` 事件序列与 seq 递增 |
| AI 开书 | `POST /api/v1/project/bootstrap/plan` 返回方案（卷章 / 角色 / 设定齐全）→ `POST /project/bootstrap/apply` 后 `GET /project/tree/:id` 与 `POST /outline/list` 能查到对应卷章与大纲节点 |
| 检索 | 上传文档 → 索引任务完成（`indexed`）→ 关键词检索返回命中片段与追踪信息 |
| 审计闭环 | 任意写接口调用后 `POST /admin/audit/list` 能查到该记录，`GET /admin/audit/detail/:id` 返回脱敏请求体与响应码 |
| 工具 | `GET /tool/list` 返回 6 个内置工具（`chapter_check`、`deai_scan`、`chapter_digest`、`character_scan`、`foreshadow_scan`、`lore_lookup`）；`POST /tool/invoke`（`chapter_check`）对含零容忍词的正文返回 `verdict_code=block` |
| 章节体检 | `POST /chapter/check` 对同一正文给出稳定分数与问题清单 |
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
