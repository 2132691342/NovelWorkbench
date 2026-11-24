# 模块设计与实现

本文逐个说明 NovelWorkbench 业务模块的职责、数据结构、实现路径、边界与设计权衡。
每个模块固定两段：**职责与实现**（做什么、落在哪些表上、由哪些文件实现、关键规则是什么）与**边界与权衡**（不做什么、失败如何处理、为什么这样设计、代价是什么）；接口字段见 [api.md](api.md)，表结构见 [database.md](database.md)，AI 细节见 [ai.md](ai.md)，模块地图见 [architecture.md](architecture.md)。

后端在 `backend/internal/api/router.go` 内注册 119 条 GET/POST 路由（另加 `/health`、`/ready` 两个探针）、35 张 AutoMigrate 表（`backend/cmd/server/main.go`）、24 个内置技能（`backend/internal/ai/domain_<域>.go`，12 个流程域每域两个）、12 份内置提示词档案与 6 个内置工具。

## 1. 认证与会话

**职责与实现**

登录、查询当前用户、个人资料维护、修改密码、登出吊销。写入 `users`（`password_hash` 为 bcrypt，`json:"-"` 永不输出）与 `login_logs`；会话状态不落库，靠 JWT + Redis 黑名单维持。

| 数据 | 位置 | 说明 |
| --- | --- | --- |
| 令牌 | 客户端 `Authorization: Bearer` | HS256，声明含 `user_ulid`、`exp`、`iat`、`iss=NovelWorkbench` |
| 吊销标记 | Redis `nw:token:blacklist:<sha256(token)>` | 值 `"1"`，TTL 取 `expires_at - issued_at` |
| 登录结果 | `login_logs` | `success` / `failed` / `disabled`，含 IP 与 UA |

`service/auth_service.go` 的 `AuthService.Login`：`userRepo.GetByAccount` 同时匹配 `user_name` 与 `user_email`；账号不存在与密码错误均返回 `ERR_UNAUTHORIZED`「账号或密码错误」，`disabled` 返回 `ERR_FORBIDDEN`；成功后签发令牌并把 `role_codes` 一并返回。`AuthService.Logout`：`jwt.Parser` 非验签解析出 `exp`/`iat`，取差值作为 TTL 调 `tokenStore.Revoke`。`AuthService.ChangePassword`：先 `bcrypt.CompareHashAndPassword` 校验旧密码，再写新哈希，最后发一条 `account` 类型站内通知。`pkg/middleware.go` 的 `AuthMiddleware`：剥 `Bearer` 前缀 → `ParseToken` → `tokenStore.IsRevoked` → `userStateChecker` 查库确认账号仍存在且未禁用，全部通过后把 `user_ulid` 写入 Gin 上下文。`pkg/jwt.go` 的 `NewJWTSigner` 在密钥为空时直接返回错误，由 `cmd/server/main.go` 在启动期 fail fast。免鉴权白名单只有 `/api/v1/auth/login`。

**边界与权衡**

- 不提供注册接口，账号只能由管理员创建；不存在 `/auth/register` 路由。
- 密码错误与账号不存在返回同一文案与同一业务码，不泄露账号是否存在。
- 登录日志写入失败只打警告，不阻断登录。
- 令牌吊销依赖 Redis：`TokenRepo.IsRevoked` 在 Redis 异常时返回 `false`（视为未吊销），由后续用户状态校验兜底。
- 修改密码不吊销已签发的旧令牌，只发通知；禁用账号与删除用户则由 `AuthMiddleware` 的用户状态检查即时生效。

取舍：无状态 JWT 让横向扩容与 SSE 长连接不需要会话粘滞，代价是「主动失效」只能靠外部黑名单：键取令牌哈希，TTL 取签发时长，因此条目最多存活一个令牌周期。每请求回查用户状态多一次主库读，换来「禁用立即生效」，对当前低并发管理型系统划算；若 QPS 上升，这一跳是第一个该加短 TTL 缓存的地方。

## 2. 用户与角色权限

**职责与实现**

账号生命周期（创建、编辑、禁用、重置密码、删除）与角色权限模型（角色 CRUD、权限码勾选、权限判定）。`users.user_name` 与 `user_email` 唯一，`roles.role_code` 唯一，`perm_codes_json` 存权限码 JSON 数组（`["*"]` 表示全部），`user_roles` 是多对多关联表、无独立权限表。内置角色两条：`admin`（通配）与 `user`（空权限集），由 `service/role_service.go` 的 `roleDefaultList` 与 `SeedDefaults` 播种。

`service/user_admin_service.go` 的 `UserAdminService.Create`：登录名与邮箱分别查重 → bcrypt 哈希 → 建用户 → `roleRepo.ReplaceUserRoles` 挂角色（缺省 `user`）→ 发「账号已开通」通知。`Update`：只覆盖传入的非空字段；`roleCodes != nil` 时整体替换角色挂载；拒绝「禁用自己」。`Delete`：拒绝删除自己 → 清角色挂载 → 清本人模型授权 → 清本人通知 → 删用户行。`BootstrapAdmin`：仅当 `users` 表为空时按配置创建 `admin` 账号并挂 `admin` 角色，是系统唯一的「自动建号」路径。`service/role_service.go` 的 `Save`：`role_ulid` 为空则创建（编码重复返回 `ERR_CONFLICT`）；更新时内置角色不允许改编码。`HasPerm`：遍历用户全部角色，命中 `*` 或目标权限码即通过；`PermCodesOfUser` 对多角色权限码取并集去重保序。路由侧由 `pkg/access_middleware.go` 的 `RequirePermMiddleware` 收口：`/admin/user/*` 要 `system:user:manage`，`/admin/role/*` 要 `system:role:manage`。

**边界与权衡**

- 内置角色不可删除、不可改编码；角色下仍有用户时删除被拒（`ERR_CONFLICT`）。
- 管理员不能禁用或删除自己；邮箱与登录名唯一性由数据库唯一索引与服务层双重保证。
- 角色保存不校验权限码是否存在于权限目录，前端选项来自权限目录接口。
- 删除项目只校验归属，不额外要求权限码；提示词使用对所有登录用户开放，只有维护动作收口到 `prompt:manage`。
- 权限判定逐请求查库、不做缓存，因此角色权限修改在下一次请求即生效。

取舍：权限码存在角色行的 JSON 列里，而不是「角色-权限」关联表：读路径只有一次 `roles` 查询加内存遍历，写入是整行覆盖，模型极简。代价是无法按权限码反查角色、也无法在数据库层做外键约束，权限码拼写错误只能靠人工发现。逐请求查库省掉缓存失效问题，代价是管理接口每次多一跳主库——对「写操作少、管理端并发低」的定位可以接受。

## 3. 菜单与导航

**职责与实现**

`menus` 表同时承担两件事：前端导航树的唯一数据源，以及权限码的登记处（菜单即权限单一事实源）。表内 29 条内置菜单由 `service/menu_service.go` 的 `menuSeedList` 播种：3 个分组（创作 / AI 资产 / 系统）、25 个页面节点、1 条动作节点 `action_model_write`。其中 5 条页面置 `visible_flag=false`：`character`、`lore`、`knowledge`、`notes` 已被素材库合并取代，`profile` 由顶栏用户菜单进入。它们不进导航，但必须有菜单行——菜单即路由，不登记就没有路由，顶栏入口点下去会落到 404。`service/navigation_chain_test.go` 在同一条导航链路里校验种子的可注册性（组件路径在前端有对应页面文件、编码与路由唯一）。

| 列 | 取值 | 作用 |
| --- | --- | --- |
| `menu_type_code` | `group` / `page` / `action` | `group` 仅组织下级、`action` 不进导航只登记权限码 |
| `scope_code` | `global` / `project` | 项目内菜单路由以 `:project_ulid` 结尾 |
| `perm_code` | 权限码，空表示登录即可见 | 导航过滤与权限目录的唯一来源 |
| `visible_flag` / `enabled_flag` / `sort_order` | 布尔 / 整型 | 显隐、启停、同级排序 |

`MenuService.SeedDefaults`：按 `menu_code` 查缺补插，父级编码先于子级解析，缺父直接报错；已存在的行由 `syncSeed` 校正名称、类型、作用域、路由、图标、权限码与排序，保证升级后导航与权限不漂移。`HiddenFlag` 是例外——它强制把 `visible_flag` 写成 false（用于「入口合并后仍保留旧路由」的场景），其余展示开关保留作者改动。`MenuService.Nav`：`roleSvc.PermCodesOfUser` 取权限并集 → `menuRepo.ListEnabled` → `filterVisibleMenus`（剔除动作节点与未命中权限码的节点，`*` 全放行；隐藏页保留并带 `visible_flag=false`）→ `buildMenuTree` 按 `parent_ulid` 递归装配，返回 `menu_tree_list` 与 `perm_code_list`。`MenuService.Save`：分组不能配路由；`validateParent` 阻止指向自己或自己的下级（成环检测，深度上限 32）；`menu_code` 由 `GetByCode` 判重，`perm_code` 由 `CountByPermCode` 判重（排除自身）。`MenuService.Delete`：存在下级时返回 `ERR_CONFLICT`，不做级联。`PermissionCatalog` / `buildPermCatalog`：把菜单行上的非空 `perm_code` 按首段分组，`*` 单独登记为「全部权限」，同一权限码重复登记只展示一次。`service/menu_permission.go` 承载这两组纯函数（`filterVisibleMenus`、`buildMenuTree`、`buildPermCatalog` 及权限分组映射），与 `menu_service.go` 的仓储编排分开，便于脱离数据库单测。前端 `store/modules/menu.ts` 的 `loadNav` 拉取 `GET /menu/nav`，把树与权限码同时写入 menu store 与 user store；`layout/components/Menu.vue` 按 `menu_tree_list` 渲染导航，项目内菜单在未选项目时置灰。

**边界与权衡**

- 动作节点永不进入导航树，只在权限目录与角色勾选里出现。
- 菜单删除不做级联：有下级先删下级，避免悬空 `parent_ulid`。
- `Nav` 只返回启用菜单，禁用后用户刷新导航即消失。
- 隐藏页（`visible_flag=false`）仍然下发，前端据此注册路由——这样「把四个页面合并成一个素材库入口」时，旧链接与深链不会失效。
- 导航只决定「看得见什么」，接口鉴权仍由 `RequirePermMiddleware` 独立判定。

取舍：把权限码登记在菜单表上，省掉了「权限表 + 角色-菜单关联表」两层结构：权限目录、角色勾选、导航过滤共用同一份数据，新增一个受控页面只需加一条菜单行。代价是权限码与「页面」概念绑死，纯动作权限必须伪造成 `action` 节点（如 `action_model_write`），菜单管理页因此混着导航配置与权限登记两件事；同时删除菜单会连带删除该权限码的登记。

## 4. 审计与登录日志

**职责与实现**

| 表 | 记录内容 | 触发者 |
| --- | --- | --- |
| `audit_logs` | 操作者、动作码（去掉 `/api/v1/` 的路径）、级别、链路 ID、请求路径/方法/体（脱敏）、目标类型与 ULID、业务响应码与说明、HTTP 状态、耗时、IP、UA | `pkg.AuditMiddleware` |
| `login_logs` | 账号、命中用户、结果码、失败原因、IP、UA | `AuthService.recordLogin` → `LoginLogService.Record` |

`pkg/access_middleware.go` 的 `AuditMiddleware`：仅对 `POST` 且不在跳过名单内的请求生效。请求前读取并还原 body（≤32KB，`multipart` 不采样），把凭据字段（`password_text`、`new_password_text`、`old_password_text`、`api_key_text`、`access_token`）替换为 `***`；用包装后的 `ResponseWriter` 捕获响应前 8KB，`c.Next()` 后解析包络得到 `resp_code` / `resp_message`，再按 HTTP 状态推导级别（`info` / `warn` / `error`），并写入 `trace_id`（与响应包络同值）。`extractAuditTarget` 按 `auditTargetKeys` 白名单从请求体提取目标（user / role / menu / project / chapter / document / prompt / tool / model / provider / chat_session / notification），取第一个命中键。跳过名单在 `cmd/server/main.go` 装配：`/api/v1/auth/login`、`/api/v1/auth/logout`、`/api/v1/chat/send`（SSE 流不适合等待后置写入）。`service/audit_service.go` 的 `Record`：补 `audit_ulid`，`user_name` 为空时回查用户登录名；`ListPage` 支持级别 / 链路 ID / 目标类型 / 动作码 / 关键字 / 用户 / 日期区间过滤；`Detail` 返回单条含请求体原文；历史行缺级别时按 HTTP 状态补全。`service/login_log_service.go` 承担登录日志的写入与分页查询（`Record` / `ListPage`），与审计日志分表：两者查询维度与保留策略不同。前端 `views/Admin/Audit.vue`：级别徽标、目标、响应码与说明、HTTP 状态、耗时、链路 ID（点击复制），抽屉展示脱敏请求体。

**边界与权衡**

- 审计只覆盖 `POST`，GET 查询不记录，避免读接口把表打成热点。
- 业务失败也记录（写入发生在 `c.Next()` 之后），审计写失败不阻断业务，只记 `audit_record_failed` 警告。
- 请求体采样上限 32KB，上传与超大请求体只记占位说明。
- 登录日志失败不阻断登录链路；日志里的失败原因比接口响应更具体，只对管理员可见。

取舍：同步落库保证「业务提交即有审计」的顺序一致性，代价是每个 POST 多一次写；当前管理端写操作稀疏，表不会成为瓶颈。body 采样上限 32KB 是对「审计价值 / 拷贝成本」的折中。凭据字段一律脱敏，审计表即使被读也不泄露口令。登录日志与审计日志分表，是因为查询维度与保留策略不同。

## 5. 应用配置

**职责与实现**

`app_settings` 只有三行，由 `service/setting_service.go` 的 `settingDefaultList` 定义，键、默认值、说明、合法区间同源：

| 键 | 默认值 | 区间 | 消费方 |
| --- | --- | --- | --- |
| `chat_history_count` | 5 | 0–50 | `ChatService.resolveHistoryLimit` |
| `rag_top_k` | 6 | 1–20 | `KnowledgeService.resolveRAGTopK` |
| `daily_word_goal` | 2000 | 100–100000 | 仪表盘每日目标展示 |

`SeedDefaults`：按 `setting_key` 查缺补插；已存在但说明文案变化时只更新 `setting_desc`，不覆盖用户改过的值。`Update`：`settingKeyAllowed` 白名单校验 → `strconv.Atoi` 要求整数 → `settingRangeMap` 校验区间 → 更新值。消费侧一律「应用配置 > 全局配置」两级回退：`resolveHistoryLimit` 先看请求参数，再看 `chat_history_count`，最后落到 `ChatConfig.HistoryCount`；`resolveRAGTopK` 同理。路由：`GET /setting/list` 对所有登录用户开放（前端要读每日目标），`POST /setting/update` 需要 `system:setting:manage`。

**边界与权衡**

取舍：只支持整数配置，非整数、越界值与不支持的键都被拒绝（`ERR_INVALID_PARAM`）。没有新增键的接口：新增配置必须改 `settingDefaultList` 与 `settingRangeMap` 并重启播种。不缓存，每次请求读表；读取失败时调用方回退到配置文件值并打警告。

「配置即表 + 每次读」让保存后立即生效，不需要重启或广播失效，也不需要配置中心。代价是热路径（每次对话、每次检索）多一次小表查询，并且配置项必须编译期登记——运行时只能改值、不能改结构，避免出现无人认识的配置键。前端 `views/Settings/index.vue` 里另有一份展示元数据（分组、单位、区间），与后端区间重复，改动时需要两边同步。

## 6. 站内通知

**职责与实现**

`notifications` 表按用户隔离，读后保留。类型三值：`system`（公告）、`task`（异步任务结果）、`account`（账号事件）。生产者：改密、建号、重置密码各发一条 `account`；`KnowledgeWorker.notifyOwner` 发 `task`。

`NotificationService.Notify`：`user_ulid` 为空直接返回（如项目已删除导致接收人缺失）；标题 128、正文 512、链接 256 字符截断；类型缺省 `system`。`ListPage` / `UnreadCount` / `MarkRead` / `MarkAllRead` / `Delete` 全部以 `user_ulid` 作为查询条件，不接受跨用户 ID。`DeleteByUser` 在账号删除时清理该用户全部通知。前端 `store/modules/notification.ts` 维护未读数；`layout/components/NotificationBell.vue` 挂载时刷新并每 60 秒轮询一次。

**边界与权衡**

取舍：无实时推送：延迟上限等于轮询间隔（60 秒）。通知只是提示，不承载业务状态：点开链接跳转由前端路由完成，后端不校验目标资源是否仍存在。所有写入失败只告警，绝不影响主链路。

用「落库 + 轮询」而不是消息推送，省掉长连接与网关改造成本，也天然支持离线用户与历史回看；代价是未读数有最多 60 秒延迟，且每次轮询要跑一次 `count` 查询。通知内容在写入时截断并做快照（存文本不存模板 ID），因此历史通知不会随后续文案调整而变化。

## 7. 任务中心

**职责与实现**

`jobs` 表是异步任务的留痕与运维入口，当前唯一任务族是知识库索引：`doc_index`（首次）与 `doc_reindex`（重建）。`payload_json` 存 `{"doc_ulid": "DOC_..."}`。状态取值 `queued` / `running` / `success` / `failed`，但 worker 只写 `queued`（投递时）、`success`、`failed` 三态。

`service/knowledge_service.go` 的 `enqueueIndexJob`：先建 `queued` 任务行，再 `mqRepo.PublishIndexJob`；投递失败把任务置 `failed` 并返回 `ERR_STORAGE`。`service/knowledge_worker.go` 的 `Run`：启动时 `ensureManticoreSchema`，结构升级返回 `true` 时 `rebuildManticoreIndex` 从 PostgreSQL 回灌；随后阻塞消费，消费中断按 5 秒间隔重连，`ctx` 取消时优雅退出。`handleIndexJob`：`GetPendingByDoc` 找回任务行 → 文档置 `indexing` → 内部重试至多 3 次，退避 1s/2s/3s → 成功置 `success` 并发通知，全部失败置 `failed` 并回写 `error_message`，随后 `ack`（毒消息不重投）。`service/job_service.go` 的 `ListPage` 解析载荷批量取文档名避免 N+1；`Stats` 按状态计数；`Retry` 只接受 `failed`，把任务重置为 `queued`、文档重置为 `pending` 后重新投递。

**边界与权衡**

- 只有 `failed` 任务可重试；载荷缺 `doc_ulid` 或文档已删除时拒绝。
- 文档已删除的滞留消息直接 `ack` 并置任务 `failed`。
- 文档状态与任务状态是两套：前者面向用户（`pending/indexing/indexed/failed`），后者面向运维。
- 没有超时回收器：worker 在索引中途崩溃会留下 `queued` 任务与 `indexing` 文档，这两种中间态都没有自助恢复入口，需要人工改状态或重新上传文档。

取舍：任务行在投递前创建，保证「消息丢了也能看到一条 queued/failed」，比纯 MQ 语义更可运维。重试放在 worker 内部而不是让 MQ 重投，避免毒消息在队列里打转；代价是重试次数只体现在 `retry_count`，队列层面看不到投递次数。把 Manticore 索引当作可重建副本来处理（失败只降级关键词通道，不阻断文档 `indexed`），是这条链路最重要的取舍：权威数据永远在 PostgreSQL 与 MinIO。

## 8. 项目 / 卷 / 章节

**职责与实现**

创作主线三级结构：`projects`（书名、题材、文风、简介、状态、累计字数）→ `volumes`（卷名、排序、概要）→ `chapters`（正文、梗概、字数、状态）。`chapter_snapshots` 存人工留档，`word_count_stats` 存按日字数增量（`word_delta` / `ai_word_delta`）。

- `service/project_service.go` 的 `Create` / `Update` / `List` / `Detail` / `Tree`；`Tree` 返回卷列表与每卷章节（升序），是写作台一次加载的数据源。`mustOwnProject` 是所有创作模块共用的归属校验：项目不存在 `ERR_NOT_FOUND`，非本人 `ERR_FORBIDDEN`。
- `ProjectService.Delete`：单个 `repo.WithTx` 事务内依次删快照、章节、卷、角色、关系、设定、大纲节点与场景卡、时间事实与伏笔与摘要、会话消息、知识切片与文档、字数统计，最后删项目；事务提交后 `cleanupKnowledgeExternal` 清 MinIO 原文与 Manticore 行，失败仅告警。
- `service/volume_service.go` 的 `Create` 缺省排序取 `MaxOrder+1`；`Delete` 先汇总卷内字数，删快照与章节后按总量负向回补项目字数。
- `service/chapter_service.go` 的 `Create` 校验卷属于该项目，`chapter_order` 取 `MaxOrder+1`，初始状态 `draft`。`ChapterService.Update`：单事务内按非空字段更新；正文变化时用 `pkg.CountWords` 重算，差值经 `applyWordDelta` 同时调整 `projects.total_word_count`（SQL 表达式增减）与当日 `word_count_stats.word_delta`。
- `SnapshotRestore`：回滚前先把当前正文写成一条 `manual` 快照，再覆盖正文并重算字数差。
- 字数口径在 `pkg/text.go` 的 `CountWords`：汉字逐字计数，连续拉丁字符算一个词，前端 `views/Workspace/wordCount.ts` 与后端同口径。

**边界与权衡**

- 卷必须属于目标项目，章节创建时卷不匹配返回 `ERR_INVALID_PARAM`；章节只能通过所属项目间接鉴权。
- 删除项目只校验归属，不额外要求权限码。
- 字数账目只在四条路径维护：章节更新、章节删除、快照回滚、卷删除；直接改库或绕过服务层不会自动修正。
- 外部存储清理在事务外，失败不回滚，可能留下孤儿对象。

取舍：章节表冗余 `project_ulid`，让「按项目取章节」「按项目鉴权」都不需要 join 卷表，代价是插入时必须保证两列一致（服务层强制）。累计字数用「增量表达式 + 每日增量表」而不是每次聚合全表：读路径 O(1)，代价是账目正确性依赖所有写入口都调用 `applyWordDelta`，一旦有遗漏就只能靠重算修复。快照采用「整章全文副本」而不是 diff，存储放大但回滚是纯覆盖，没有补丁冲突。

### 8.1 AI 开书

一句创意直接长出可写的项目骨架，解决「新建项目后面对空白卷章不知道从哪下手」的冷启动问题。

- `ai/bootstrap.go`：`bootstrapSystemPrompt` 固定输出严格 JSON（书名 / 卖点 / 简介 / 文风 / 卷章 / 角色 / 设定）；`ParseBootstrapPlan` 容忍代码围栏与前后缀文字，缺 JSON、缺书名或缺卷章一律报错，不产出半成品。
- `service/bootstrap_service.go` 的 `GeneratePlan`：经 `ModelProviderService.ChatSpecOf` 取当前账号有权限的模型（未指定则默认模型），流式累积全文后一次解析（非流式会撞网关首包超时）；首包超时与输出上限单独放宽到 300s / 8000 token。失败且章节数 >3 时缩半规模重试一次。`normalizeBootstrapPlan` 再收敛模型输出——卷 ≤3、角色 ≤8、设定 ≤10、章节数按请求截断、枚举值校正、名称按列宽截断、空名称补默认值。
- `ApplyPlan`：单个 `repo.WithTx` 事务内创建项目 → 卷 → 章节 → 角色 → 设定 → 两级大纲节点（卷节点下挂章节点），任一失败整体回滚。
- 前端 `views/Dashboard/components/BootstrapDialog.vue`：两步向导，第二步逐字段可改后确认开书，成功后直接进入新项目的写作台。

方案只生成不落库，落库必须显式二次提交，避免模型幻觉直接写进作品数据。不生成正文：章节只有名称与梗概。大纲只建卷 → 章两级。生成失败返回 `ERR_AI_PROVIDER`，不静默降级为手工模板。

选择「两段式（plan → apply）」而不是一次调用直接建库：多一次前端往返，换来作者能在落库前改掉不合适的书名与章节。用非流式而非 SSE：开书是一次性结构化产物，逐字流没有意义。枚举收敛放在服务层而非提示词里：模型对枚举值的遵从率不可控，落库前必须有一次确定性校正。

## 9. 角色卡与关系图

**职责与实现**

`characters` 存角色卡（姓名、别名、类型、性格、背景、外貌、目标动机），`character_relations` 存有向关系边（起点、终点、类型、说明），两者都以 `project_ulid` 归属。

`service/character_service.go` 的 `Save`：`character_ulid` 为空即新建，否则先校验存在且同项目，再整字段覆盖更新。`Delete`：单事务内先 `DeleteRelationsByCharacter` 再删角色，避免悬挂边。`Graph`：一次取全项目角色与全项目关系，装配 `CharacterGraphVO{NodeList, EdgeList}`。`RelationSave`：拒绝自环；两端角色必须存在且都属本项目；`RelationDelete` 按关系行反查项目做归属校验。前端 `views/Character/index.vue` 为卡片网格 + 编辑抽屉；关系图页签用 `await import('@antv/x6')` 懒加载渲染，卸载时 `dispose`。角色卡同时是 AI 上下文素材：`service/chat_context.go` 的 `formatCharacterBlock` 把性格截断 120 字、背景截断 200 字后注入。

**边界与权衡**

取舍：关系图按需计算，不落图数据库，也不做布局持久化。角色卡没有版本历史；删除角色会连带删除其全部关系边，不可撤销。注入 AI 的角色信息是截断摘要，角色数量多时靠上下文预算裁剪整块丢弃。

用「两张普通表 + 内存构图」替代图数据库：规模是一本书的几十个角色、几百条边，全量加载在毫秒级，省掉一套中间件与同步逻辑。代价是无法做图算法查询，关系类型也只是自由字符串。角色卡选择「全量注入 + 单项截断」而不是按相关性检索，是因为作者期望 AI 认识全部主要角色。

## 10. 世界观设定与创作笔记

**职责与实现**

`lore_settings` 一张表承载两个产品：世界观设定（`world`/`rule`/`item`/`place`/`event`）与创作笔记（`note`）。`content_mode_code` 决定正文用哪种编辑器与存储格式：`rich`（HTML）或 `markdown`。

`service/lore_service.go` 的 `Save`：新建时 `content_mode_code` 缺省 `rich`；更新时只覆盖非空字段。`Delete` / `List` 均先经 `mustOwnProject`；列表支持按类型与关键词过滤，按 `sort_order` 升序。笔记隔离有两道闸：`repo/chat_repo.go` 的 `ListLoreBriefsByProject` 在 SQL 层 `WHERE lore_type_code != 'note'`；`service/chat_context.go` 的 `formatLoreBlock` 再跳过一次。前端 `views/Lore/index.vue` 按分类展示设定卡片；`views/Notes/index.vue` 只查 `NW_LORE_TYPE_NOTE`，标题与正文 2 秒防抖自动保存，切换笔记前 `flushSave`。`views/Notes/components/NoteEditor.vue` 实现双模式：`markdown` 用 `marked` 渲染预览，`rich` 直接按 HTML 渲染原文；两种模式共用同一个 textarea 与自动保存通道。

**边界与权衡**

取舍：`note` 永不进入 AI 上下文：引用入口只覆盖知识库文档与章节，不包含设定表。正文上限 20000 字符；类型白名单非法值被参数校验拒绝。设定没有版本历史，删除即物理删除；富文本以 HTML 原样存储，不做服务端清洗（信任单作者场景）。

复用设定表承载笔记，省掉一张表、一套 CRUD 与一套前端状态；代价是「排除笔记」这条业务规则必须同时存在于 SQL 与格式化函数两处（当前刻意冗余，任一处失效仍有兜底）。私人笔记的隔离选择「SQL 层排除」而不是「提示词层过滤」，是因为前者能保证即使上下文装配被改写也不会泄漏。

## 11. 大纲树与场景卡

**职责与实现**

`outline_nodes` 是邻接表树（`parent_outline_ulid` + `sort_order`），节点类型 `volume`/`arc`/`storyBlock`/`chapter`，状态 `draft`/`planning`/`ready`/`done`。`detailed_scenes` 挂在节点下，记录出场角色 ULID 列表、地点、核心冲突、节奏、预计字数、入场状态、目标、关键选择、结果、出场状态。

`service/outline_service.go` 的 `SaveNode`：新建时 `sort_order` 为 0 则取 `MaxNodeOrder+1`，状态缺省 `draft`，并 `validateParent` 校验父节点属本项目且不成环。`validateParent`：父节点为空直接放行；父节点等于自身拒绝；随后沿 `parent_outline_ulid` 上溯，一旦发现某层的父节点是自己即判成环。`DeleteNode`：先 `CountNodesByParent`，有子节点返回 `ERR_CONFLICT`，不做级联删除。`ListTree`：一次取全项目节点，按 `parent_outline_ulid` 分桶后由 `attachOutlineChildren` 递归拼装，根节点为 `parent_outline_ulid = ''`。`SaveScene`：先校验节点属本项目，`encodeCharacterULIDs` 把角色列表序列化为 JSON；新建时排序取 `MaxSceneOrder+1`。

**边界与权衡**

- 大纲节点与真实章节是两套数据：大纲 `chapter` 类型节点不会自动创建 `chapters` 行，反之亦然。
- 删除节点必须自底向上，没有「连同子树一起删」的接口。
- 场景卡的出场角色只存 ULID 列表，不校验角色是否仍存在。
- 没有拖拽重排接口，调整顺序靠保存新的 `sort_order`。

取舍：邻接表 + 内存建树，写入是单行更新，读取是一次全项目查询，对一本书的规模足够；代价是深度查询要在应用层循环，且没有数据库级的外键或环约束。大纲与章节分离，是因为规划阶段的结构（幕、故事块）与写作阶段的卷章并不一一对应；代价是两套结构的同步只能靠作者。

## 12. 连续性

**职责与实现**

三本手工账，全部以 `project_ulid` 归属：

| 表 | 状态码 | 关键字段 |
| --- | --- | --- |
| `temporal_facts` | `candidate` / `confirmed` / `superseded` / `stale` | 主语、谓语、宾语、生效章节区间、来源章节与原文引用 |
| `foreshadow_hooks` | `planned` / `planted` / `echoed` / `resolved` / `abandoned` | 描述、预期回收、埋设/回收章节、重要度 `high/medium/low` |
| `chapter_summaries` | `pending` / `verified` / `stale` | 摘要正文、`source_hash_text`（生成时正文指纹） |

`service/continuity_service.go` 的四个校验函数是状态白名单的唯一实现，允许空值由调用方补默认（事实 `candidate`、重要度 `medium`、摘要 `pending`）。`SaveFact`：更新时状态为空则沿用原值，不允许写成非法码。`ReviewFact`：只接受 `confirmed` / `superseded` / `stale`，明确不允许退回 `candidate`。`SaveHook`：状态随显式保存推进，不做自动流转。`SaveSummary`：无 `summary_ulid` 时先按 `(project_ulid, chapter_ulid)` 查既有记录，存在则更新，保证「一章一条摘要」。

**边界与权衡**

- 全部由作者手工维护：AI 对话链路不写这三张表，也不读取它们（章节梗概走 `chapters.summary_text`）。
- `source_hash_text` 不参与自动比对，正文变化后系统不会自动把摘要置 `stale`。
- 状态流转没有强制前置约束，只有取值白名单。
- 章节 ULID 字段不做存在性校验。

取舍：把连续性做成「可人工复核的账本」而不是「AI 自动抽取的知识图谱」：结论可解释、可回溯到章节引用，且不会因为模型幻觉污染权威数据；代价是全部维护成本落在作者身上。事实复核不允许退回 `candidate`，是因为「候选」代表未经人判断的原始记录，一旦人工定性就不该伪装成未处理。摘要指纹只存不算，是为将来自动失效留的字段。

## 13. 知识库

**职责与实现**

原文进 MinIO（对象键即 `doc_ulid`），元数据与切片进 PostgreSQL，全文检索副本进 Manticore。文档状态 `pending → indexing → indexed | failed`，类型白名单 `txt/md/docx/pdf`，单文件上限默认 20MB。

- **上传**：`service/knowledge_service.go` 的 `Upload` 校验归属与扩展名 → `objectRepo.PutObject` 写原文 → 建 `pending` 文档行（失败则回滚对象）→ `enqueueIndexJob` 投递任务。
- **索引**：`service/knowledge_worker.go` 的 `indexDocument` 读原文 → `extractDocumentText`（`extractDocxText` / `extractPDFText`）→ `chunkText` 按 `ChunkSize` 切分、`ChunkOverlap` 重叠、优先在句末/换行处断点 → `contentHashOf` 与既有切片比对复用 ULID → 单事务删旧切片写新切片 → `syncManticoreChunks` 先删后插 → 文档置 `indexed` 并回写切片数与 token 数。
- **表结构**：`knowledge_manticore.go` 的 `ensureManticoreSchema` 建 CJK ngram 全文表；`SHOW CREATE TABLE` 结果里缺少 `ngram_len` 说明表不是当前结构，此时 drop 重建并返回 `true`，由 `rebuildManticoreIndex` 从 PostgreSQL 全量回灌。
- **检索**：`service/knowledge_search_service.go` 的 `SearchWithTrace`：`normalizeSearchMode`（只有 `hybrid` 走双通道）→ `extractQueryTerms`（CJK 2-gram + 拉丁词）→ `searchKeywordChannel` 分别以 `term`（`"a" | "b"`）与 `phrase`（整串短语）查 Manticore → `fuseSearchResults` 用 RRF（`score = Σ 1/(60+rank)`）融合 → `maybeRerank` 用系统默认对话模型重排（8 秒超时，失败 fail-open 保留原序）→ `trimHits` 截断。返回 `KnowledgeSearchDTO` 同时带 `Trace`（检索词、MATCH 表达式、候选数、是否重排、入选 ID）。
- **切片回表**：`searchKeywordChannel` 拿到 `chunk_ulid` 后 `GetChunksByULIDs` 回 PostgreSQL 取正文与文档名，Manticore 只当索引不存权威文本。
- **其它入口**：`ChunkList`、`GetContent`（按 `Content-Type` 直接输出原文二进制，上限 25MB，不走 JSON 包络）、`Stats`、`Reindex`（仅 `indexed`/`failed` 可重投）、`Delete`（先删对象与 Manticore 行，再事务删切片与文档）。

**边界与权衡**

- 不支持向量检索，也不生成 embedding；`hybrid` 指「词条 + 短语两条词法通道」，不是语义通道。
- 检索限定单项目，Manticore 查询也带 `project_ulid` 过滤；`mustOwnDoc` 对 `project_ulid` 为空或 `"0"` 的全局库文档放行。
- Manticore 是可选依赖但检索强依赖它：查询阶段单个通道失败会降级到另一通道，两个通道都失败则 `/knowledge/search` 返回 `ERR_STORAGE`，只有对话链路的 RAG 召回会吞掉该错误降级为空上下文。
- 切片入库在事务内、Manticore 同步在事务外，同步失败时两者短暂不一致，靠 `Reindex` 修复。

取舍：不引入向量模型是这套设计的根假设：部署不需要额外模型与向量库，检索结果完全可解释，索引可从 PostgreSQL 整体重建。代价是同义改写、跨语言查询召回明显弱于语义检索，只能靠 CJK 2-gram 切词、双通道 RRF 与对话模型重排补偿。用 Manticore 而不是 PostgreSQL 全文索引，是为了 CJK ngram 分词与 BM25 打分开箱可用；代价是多一个必须同步的存储与 worker、队列、任务状态机。

## 14. AI 对话

**职责与实现**

`chat_sessions`（项目 + 用户 + 会话名）与 `chat_messages`（角色、正文、思考过程、命中提示词编码、工具编码、五类引用 ULID 的 JSON、召回依据、估算 token）。会话名缺省时用首条消息前 20 字回填。

`ChatService.Send`（`service/chat_service.go`）是唯一生成入口，顺序固定：

1. `mustOwnProject` → `resolvePrompt`（项目提示词覆盖同编码全局提示词；`prompt_code` 优先，否则解析正文首 token 的 `/code`）。
2. `resolveSession`：`session_ulid` 为空自动建会话，否则 `mustOwnSession` 校验归属并要求项目一致。
3. `validateRefMessages` 校验显式引用的历史消息都属于本会话（去重保序）；`selectHistoryMessages` 取最近 `n` 条为骨干，引用消息必注入。
4. `loadContextInput`（`service/chat_context.go`）：引用章节（首项标记当前章）、引用文档全文、引用章节摘要、引用提示词、RAG 召回（失败降级为空）、项目角色、设定（排除 `note`）、当前章前后章梗概。
5. `buildContextBlocks` 组装块并给优先级：当前章 100、显式引用 90、工具结果 85、RAG 70、章节梗概 60、角色与设定 50；历史不进块。
6. `ai.AssembleContext` 按优先级装入、超 `ai.ContextBudgetChars`（12000 字符）的块整块丢弃并返回 `dropped_keys` 记日志。
7. 命中指令提示词时用 `ai.RenderPrompt` 渲染 `{{user_input}}` / `{{chapter_content}}` / `{{context}}` / `{{prompt_code}}`，得到独立的系统级指令消息。
8. 先落用户消息（含引用与 token 估算），会话名为空则回填；`promptSvc.SystemPromptOf` 解析身份提示词，`providerSvc.ChatSpecOf` 取模型规格。
9. `buildChatMessages` 组装：身份（系统）→ 上下文（系统）→ 指令（系统）→ 历史（原生角色）→ 用户输入（用户）。
10. `streamChat`（`service/chat_stream.go`）消费模型流：`thinking`/`chunk` 事件逐段下发，15 秒无输出发 `: ping` 心跳，连续 6 分钟无分片判网关已断并发 `error` 事件。
11. 生成完成后单事务写入 assistant 消息与当日 `ai_word_delta`，再发 `done` 事件并携带 RAG 依据与本轮工具编码。

SSE 协议由 `api/chat_api.go` 的 `sseChatWriter` 实现：`data: {json}` 单行事件，`begin()` 首次输出时提交 `text/event-stream`、`no-cache`、`X-Accel-Buffering: no`；事件 `seq` 由 `chatEventWriter.emit` 单调递增。

**边界与权衡**

- 单链路：没有「一键生成摘要/续写」的独立接口，所有生成都走 `/chat/send` + 提示词编码。
- 注入内容一律系统级消息，用户输入一律用户级消息：模型侧能区分"约束"与"要求"，界面侧也不会把上下文误显示成用户发言。
- 流开始前的失败（项目、提示词、会话）返回 JSON 包络且不写任何消息；但 `ChatSpecOf` 在用户消息落库之后才执行，供应商不可用时同样返回 JSON 包络，此时用户消息已留在会话里、assistant 消息缺失。
- 流开始后的失败只能经 `error` 事件下发。
- 客户端断开或生成失败时不写 assistant 消息，但已落库的用户消息保留。
- 引用文档必须是本项目或全局库，跨项目返回 `ERR_FORBIDDEN`；引用项目提示词同理。
- 历史条数优先级：请求参数 > `chat_history_count` > `ChatConfig.HistoryCount`；上下文超预算时丢弃是静默的。
- `chat/send` 在审计跳过名单内，用量统计仍由 `ai.Client` 上报。


取舍：单链路把所有生成收口到一处，上下文装配、容错、流式协议、落库与统计只有一份实现，排错路径唯一；代价是交互上多一步「生成 → 手动插入」。上下文用「优先级 + 整块丢弃」而不是「逐块摘要压缩」：行为确定、不会二次调用模型，但预算不足时可能丢掉整块角色信息。RAG 失败降级为空上下文而不是报错，是保证「对话永远可用」的取舍，代价是回答质量静默下降。

## 15. 提示词资产

**职责与实现**

`prompts` 是可编辑的提示词资产：编码（对话中以 `/code` 触发）、名称、种类（`system` 身份 / `command` 指令）、作用域、分类、正文 `prompt_text`、启停与内置标记。身份与指令同表同构，靠 `prompt_kind_code` 区分——它们本就是同一种东西（一段可编辑、可注入、可停用的提示词），拆成两张表只会让"哪一段真的生效"变难查。

`PromptService.List`：`project_ulid` 非空时合并「项目提示词 + 全局提示词」，并用 `isOverriddenGlobalPrompt` 给被覆盖的全局提示词打 `overridden_flag`。`PromptService.Save`：作用域缺省 `global`；项目提示词必须带项目并经 `mustProjectAccess` 校验项目存在（不校验创建者，方便团队协作）；`validatePromptPlaceholders` 要求正文占位符都在内置白名单内；同一作用域下 `prompt_code` 唯一；内置提示词不允许改编码、作用域与种类。`resolveByCode` / `pickPromptByScope`：启用中的项目提示词优先，其次启用中的全局提示词，都不命中则报「提示词不存在或已停用」。`resolveSlashCode` / `parseSlashToken`：显式 `prompt_code` 优先，否则只解析正文**首个 token** 的 `/code`，命中则剥离命令，未命中按自由对话处理。

**边界与权衡**

- 内置提示词不可删除（可停用），可改名称、说明、正文与排序号。
- 只识别正文首个 token 的斜杠命令，正文中间出现的 `/xxx` 不会触发提示词。
- 项目提示词的作用域校验只看「项目存在」，不看归属——同一项目的协作者都能维护项目提示词，这是刻意的团队协作放宽。
- 占位符只支持四个内置变量；未登记的占位符在保存期就被拒绝，避免运行期静默渲染成空串。
- 旧的 `skills` / `skill_references` / `prompt_profiles` 三张表已合并为 `prompts`：技能参考资料（关键词命中的长资料）随技能一起删除，改为 `@` 引用提示词，作者显式挑选，行为更可预期。

取舍：把提示词做成数据而不是代码，让作者不改代码、不发版就能调提示词；代价是占位符一致性必须在保存期校验，运行期还要处理上下文预算裁剪。作用域覆盖规则让「全局默认 + 项目特化」成为可能，代价是列表页必须显式标注「被覆盖」。放弃关键词命中参考资料，换来更短的链路与更可解释的上下文：作者 @ 了什么，模型就看到什么。

## 16. 提示词注入与身份解析

**职责与实现**

身份提示词决定「AI 是谁」，指令提示词决定「这一单活怎么干」。二者都以**系统级消息**进入消息序列，用户在对话框输入的内容始终是**用户级消息**。

`SystemPromptOf(projectULID, systemPromptCode)`：显式身份提示词编码（须启用且为全局或属于当前项目）→ 启用的全局默认身份提示词（`GetDefaultSystem`，排序号最小）→ `ai.SystemPrompt("")` 内置兜底。项目层再叠加 `projects.style_text`（`ai.ComposeSystemPrompt`）。

**边界与权衡**

- 身份提示词不参与上下文预算裁剪：它走 `system` 角色，不受 `ContextBudgetChars` 约束，因此正文上限设为 12000 字符（与指令提示词同一上限）。
- 项目提示词的归属校验只看项目存在，与提示词库一致；对话期 @ 引用项目提示词时会再校验一次 `project_ulid` 匹配，防止跨项目引用。
- 身份提示词与指令提示词同时下发、互不覆盖：身份写底线，指令写动作，模型看到的是"以什么身份 + 这次做什么"。
- 旧版把上下文塞进 `user` 消息，与真实用户输入混在一起；现在上下文、指令都是系统级消息，模型侧的约束边界清晰，前端也能正确区分"我发的"和"系统注入的"。

取舍：把系统提示词做成可编辑资产，让「换一种 AI 身份」不需要改代码；代价是每次对话多一次身份提示词查询（单行按编码/默认排序取，代价可忽略）。

## 16.1 工具资产

**职责与实现**

`tools` 表存「对话期可被调用的能力单元」。与提示词的分工是：提示词让模型**按格式写**，工具让服务端**先算出事实**，结果以「工具结果」上下文块注入，模型据此作答，避免自己心算或编造。

| 列 | 说明 |
| --- | --- |
| `tool_code` | 唯一编码，`#编码` 触发；内置不可改 |
| `trigger_keywords` | 逗号分隔的自动触发词，正文命中即运行 |
| `param_schema_json` | 参数声明（`ToolParamDO` 数组），前端表单与调用校验共用 |
| `builtin_flag` / `customized_flag` / `asset_hash_text` | 与提示词同构的出厂资产三件套 |

`ai/builtin_tools.go`：6 个内置工具定义（`chapter_check`、`deai_scan`、`chapter_digest`、`character_scan`、`foreshadow_scan`、`lore_lookup`）与 `ContentHash` 指纹。`service/tool_service.go` 的 `SeedDefaults`：按 `tool_code` 查缺补插；指纹变化且未自改时覆盖；已下线编码删除。`Invoke`：按编码取工具 → 校验启用 → `execute` 分发。`chapter_check` / `deai_scan` 复用 `ai/check.Analyze`；`chapter_digest` 抽取每段首句；`character_scan` 统计角色卡姓名（含别名）在本章的命中；`foreshadow_scan` 用双字片段粗匹配伏笔相关词；`lore_lookup` 按关键词查设定并剥掉富文本标签。结果统一截断到 4000 字符。`ResolveTools`：解析本轮工具。显式编码来自请求体 `tool_code_list` 与正文内联 `#code`，合并后去重保序、上限 3 个；编码不存在或已停用时返回 `missingList`，由 `ChatService` 直接报错（用户点名却不执行比报错更糟）。余量再按 `trigger_keywords` 自动补位，已显式选中的不重复触发。`service/tool_command.go`：`parseHashToolCodes` 只认行首或空白后的 `#code`（`C#` 之类不误判），剥离触发词时保留其余换行与缩进；`selectToolsByCode` / `matchToolsByKeyword` 是纯函数，可脱离仓储单测。`service/tool_helper.go` 承载五个内置工具的执行器（正文抽取、段落首句摘要、角色姓名命中统计、伏笔粗匹配、设定检索与富文本剥离）与结果截断，与 `tool_service.go` 的编排职责分开，避免单文件超过 500 行。`ChatService.runTools`：在上下文装配前调用，失败只 `Warnw` 不阻断对话；返回的工具编码写入消息的 `tool_code_json` 并随 `done` 事件下发，前端据此在消息头展示 `#编码` 徽标，历史回看同样保留。

**边界与权衡**

取舍：只支持内置类型：自定义工具需要服务端实现，因此新建入口仅对管理员开放且类型固定。工具不写业务数据：全部是只读计算，`/tool/invoke` 不产生章节、消息或统计变更。自动触发不看权限码，只看启用状态；参数缺失时由执行器给明确提示（如「请提供正文或指定章节」），不抛 panic。

把「可机检的门禁」从提示词搬进代码，换来三件事：结果可复现（同一正文永远同一结论）、可单测（`ai/check` 是纯函数）、零模型成本。代价是规则改动需要发版，因此阈值集中定义在 `ai/check/chapter_check.go` 顶部常量。并且工具只做「发现问题」，改稿仍由提示词完成——工具给依据，提示词给产出。

## 17. 模型生态

**职责与实现**

三层结构：供应商（`model_providers`：名称、协议、BaseURL、加密 API Key、启停、测试状态）→ 模型目录（`model_catalogs`：模型名、标签、温度、max tokens、上下文窗口、默认标记、测试状态）→ 授权（`model_role_grants` 角色 × 模型、`model_user_grants` 用户 × 模型）。

| 规则 | 实现 |
| --- | --- |
| 协议 | `openai`（`/v1/chat/completions` 兼容）与 `anthropic`（`/v1/messages`），Anthropic 的 BaseURL 自动剥掉尾部 `/v1` |
| 密钥 | `pkg.SealSecret` / `OpenSecret` 用 JWT 密钥做 AES-GCM 封存，读取只回掩码 |
| 默认模型 | 全局唯一，设置前先 `ClearDefaultModel`；默认模型不可删除 |
| 授权 | 角色授权与用户直授都可整体覆盖，可用集合取并集；管理员（`*`）无需授权即可见全部启用模型 |
| 关联清理 | 删用户 → `DeleteUserGrantsByUser`；删角色 → `DeleteRoleGrantsByRole`；删模型 → `DeleteGrantsByModel`。不存在悬空授权 |

`service/model_provider_service.go` 的 `SeedDefault`：供应商表为空且配置了网关时播种「出厂默认」，并授权给全部内置角色；`MigrateLegacyModels` 在模型目录为空时把仅带 `chat_model_name` 的供应商行补成目录行（幂等）。`SaveProvider`：首次创建必须填 Key；更新时 Key 传空表示保持不变；协议与 BaseURL 规范化后落库。`TestProvider` / `TestModel` → `probe`：构造 `ProviderSpec` 后调 `gateway.Probe` 跑一次极小对话，结果（状态、消息、延迟、回复）回写 `last_test_*` 字段。`DeleteProvider`：其下仍有模型时拒绝。`service/model_catalog_service.go` 的 `SaveModel` / `DeleteModel` / `SetDefaultModel` / `TestModel` / `GrantModel`：`temperatureOf` 缺省 0.7；`toModelVO` 把供应商快照、角色授权与用户直授一起返回；`GrantModel` 同时覆盖两张授权表。`service/model_access_service.go` 的 `usableModelList`：管理员看全部「模型启用且供应商启用」的模型，其他用户取「角色授权 ∪ 用户直授」再过滤；`pickModel` 依次尝试指定 ULID、`default_flag`、第一个可用。`ChatSpecOf` / `DefaultChatSpec` / `Capability`：前者给对话链路（含授权与停用校验），中者给检索重排等后台链路（只看系统默认模型），后者给前端模型下拉。

**边界与权衡**

- 不支持向量模型与 embedding 协议，只有对话协议。
- 没有可用模型时 `ChatSpecOf` 返回 `ERR_AI_PROVIDER`，指定了未授权模型返回 `ERR_FORBIDDEN`。
- 授权粒度是「角色 + 用户」两档：单用户特例直接给用户直授，不必新建角色；用户直授独立于角色，改角色不影响它。
- 密钥解密失败时返回 `ERR_BIZ_FAILED` 并提示重新填写。
- 供应商/模型的测试结果只是快照，真正可用性判断看 `enabled_flag` 与授权。

取舍：「供应商承载凭据与网关、模型是最小使用与授权单位」，让一个 Key 下挂多个模型成为常态；代价是管理端要维护三张表与两个抽屉，配置路径变长。授权每次对话实时计算，换来「改授权立即生效、无需重新登录」；管理端流量低，这个成本可以接受。密钥用 JWT 密钥做对称封存，避免额外密钥管理服务，代价是轮换 JWT 密钥会导致已存密钥无法解密。

## 18. 数据总览

**职责与实现**

只读聚合：项目数、累计字数、今日净增字数、AI 累计字数、近 30 天趋势。数据来自 `projects.total_word_count` 与 `word_count_stats`，趋势天数常量 30。

`service/stats_service.go` 的 `Overview`：`project_ulid` 非空时先 `mustOwnProject`；随后五次查询取项目数、累计字数、今日净增、AI 累计与趋势。`trendList` 取区间内有数据的日期，再按天循环补零，保证 30 个点连续。`repo/stats_repo.go` 的所有查询都 `JOIN projects` 并按 `user_ulid` 过滤。前端 `views/Dashboard/index.vue` 用 ECharts 画柱（`word_delta`）+ 折线（`ai_word_delta`）双系列。

**边界与权衡**

取舍：不做实时重算：数值完全信任 `projects.total_word_count` 与 `word_count_stats`。没有缓存与预聚合表，每次打开仪表盘都跑聚合查询。`ai_word_delta` 只统计对话生成的字数，不含「插入章节」后的正文。

用「写入时记账 + 读取时求和」替代「读取时扫章节表」：趋势查询只碰一张窄表，30 天数据量恒定；代价是写入路径必须维护两处账，任何绕过服务层的写入都会让统计漂移。补零放在服务层而不是 SQL，是为了让「没有数据的那天」也能返回完整序列。

## 19. 用量分析

**职责与实现**

`model_usage_stats` 按「日期 × 供应商 × 模型 × 调用类型」唯一聚合调用次数、成功/失败数、输入/输出/总 token、耗时总和。当前调用类型只有 `chat`。

`service/model_usage_service.go` 的 `NewModelUsageService` 启动 `flushLoop`；`RecordUsage` 在内存 map 中按维度累加，`sync.Mutex` 保护。`flushLoop` 每 5 秒 `Flush` 一次，`ctx` 取消时做最后一次落库；`Flush` 先整体换出新 map 再逐条 `UpsertIncrement`，单条失败只告警并丢弃该批。`Overview`：查询前先 `Flush`；`resolveUsageRange` 缺省近 14 天、非法日期回退默认、起止倒置自动交换；`fillUsageDaily` 补零；`weightedAvgLatency` 用每日调用数加权求平均延迟。上报方：`ai/client.go` 通过 `ai.UsageRecorder` 接口调用，`cmd/server/main.go` 用 `aiClient.SetUsageRecorder(usageService)` 注入，避免 `ai` 依赖存储层。前端 `views/Admin/Usage.vue` 提供 7/14/30 天预设、四项指标、双轴趋势图与两张分布表。

**边界与权衡**

- 调用类型只有 `chat` 一种：连通性探测与检索重排内部都走 `Client.Generate`，因此同样以 `chat` 计入，无法区分「用户对话」与「系统自用」。
- 进程崩溃会丢失未 flush 的缓冲（最多 5 秒窗口）；`Flush` 失败直接丢弃该批，不做重试队列。
- 不做用户维度统计。
- 需要 `system:usage:read` 权限。

取舍：内存缓冲 + 批量 upsert 把高频小请求的写放大压到 5 秒一次，代价是弱持久性与内存态（多实例部署时各实例各自缓冲、无法合并同维度累加）。查询前强制 flush 是补上「刚调用完看不到」的体验缺口。按天聚合而不是存原始调用日志，让表体量恒定、查询便宜，但丧失了事后按任意维度重算的能力。

## 20. 运行状态

**职责与实现**

实时探测五个外部依赖 + 一个模型能力结论，附带进程信息（启动时刻、运行时长、Go 版本、构建版本）。不落库。

| 依赖 | 必需 | 探针实现 |
| --- | --- | --- |
| PostgreSQL | 是 | `sqlDB.PingContext` |
| Redis | 否 | `rdb.Ping`（未配置时报错） |
| RabbitMQ | 否 | 连接状态 + `DeclareQueue` |
| MinIO | 否 | `BucketExists` |
| Manticore | 否 | `manticoreClient.Ping` |
| 模型供应商 | 否 | 复用 `ModelProviderService.Capability` |

`service/system_status_service.go` 的 `Status`：固定五项探针放进 `sync.WaitGroup` 并发执行，结果按下标写回保证顺序；随后追加一条 `model` 健康项。`probeDependency`：探针为 `nil` 记 `degraded`「未配置」；执行成功记 `up` 并记录耗时；失败时必需依赖记 `down`、其余记 `degraded`。探针由 `cmd/server/main.go` 以闭包形式注入（`SystemProbeDeps`），service 不直接持有各中间件客户端。

**边界与权衡**

取舍：探测失败不影响接口本身：`Status` 永远返回 200 与一份完整清单。没有历史与告警，只反映调用瞬间的状态。与 `/health`、`/ready` 分工不同：前者只回存活，后者只 Ping PostgreSQL，本模块是给人看的面板。

并发探测把总延迟压到最慢一项，代价是每次刷新同时打六个外部依赖——因此前端坚持手动刷新，不做轮询。探针以闭包注入而非让 service 依赖 infra 客户端，保持了分层单向。只给「up/down/degraded」三态而不做更细的错误分类，是让运维一眼看出「能不能用」。

## 21. 前端工作台装配

**职责与实现**

前端不重复实现业务规则，只做三件事：装配后端下发的结构（导航、权限码、可用模型）、维护跨页上下文（当前项目、未读数）、承载交互（编辑器、流式渲染、抽屉表单）。设计系统与页面实现见 [frontend.md](frontend.md)，本节只说明与后端模块的装配关系。

| 装配点 | 数据来源 | 消费位置 |
| --- | --- | --- |
| 侧边栏导航 | `GET /menu/nav` 的 `menu_tree_list` / `perm_code_list` | `store/modules/menu.ts` 的 `loadNav` → `layout/components/Menu.vue` |
| 项目上下文 | 路由参数 `:project_ulid` 优先，其次 `store/modules/project.ts` 选中态 | `layout/Layout.vue` 的 `syncFromRoute` |
| 可用模型 | `GET /model/capability` | `views/Workspace/AiPanel.vue` 的 `modelOptions` |
| 会话与消息 | `POST /chat/session/list`、`POST /chat/history` | `views/Workspace/AiPanel.vue` |
| 流式生成 | `POST /chat/send`（fetch + `ReadableStream`） | `composables/useSSE.ts` 的 `streamSsePost` / `parseSseBuffer` |
| 章节正文 | `GET /chapter/detail/{ChapterULID}`、`POST /chapter/update` | `views/Workspace/ChapterEditor.vue`（CodeMirror 6，2 秒防抖保存） |
| 目录树 | `GET /project/tree/{ProjectULID}` | `views/Workspace/WorkspaceTree.vue` |
| 未读通知 | `GET /notification/unread_count`、`POST /notification/list` | `store/modules/notification.ts` → `NotificationBell.vue` |

写作台三栏：`views/Workspace/index.vue` 组合 `WorkspaceTree` / `ChapterEditor` / `AiPanel`，左右栏可独立折叠。AI 回复落正文是纯前端动作：`AiPanel` 发 `insert-to-chapter` 事件 → `index.vue` 的 `handleInsertToChapter` → `ChapterEditor.insertTextAtCursor`，随后走编辑器同一自动保存通道。权限判断只读 `perm_code_list`（含 `*`）：路由守卫调 `store/modules/user.ts` 的 `hasPerm`，菜单可见性由后端导航树预先过滤。素材库：`views/Material/index.vue` 用胶囊分段承载角色、设定、笔记、文档四页，首次点开才挂载子组件（`?tab=` 深链直达），旧路由仍注册。

**边界与权衡**

- 前端不拼装 AI 上下文：`@` 引用只传 ULID 列表，素材加载、截断、预算裁剪全在后端。
- 前端不计算字数权威值：编辑器用 `wordCount.ts` 做即时反馈，落库字数以服务端 `pkg.CountWords` 为准。
- 前端不缓存业务列表：切页即重新请求，只有导航树、权限码、当前项目与未读数跨页保留。
- `api/contracts.ts` 是契约的手工镜像，没有生成器，字段改名需两边同步。

取舍：导航完全由后端下发，改菜单即改侧边栏，代价是登录后多一次 `nav` 请求、角色变更后必须重新拉取。SSE 用原生 `fetch` 而不是 Axios（Axios 不支持流式响应体），代价是要自己实现按行解析与半行缓冲。编辑器自动保存 2 秒防抖而不是逐键保存，减少写请求并避免字数账目抖动，代价是极端情况下可能丢最后两秒输入，因此切换章节前显式调用 `flushSave`。

## 22. 模块间依赖

调用方向严格单向，下表的箭头表示「调用/依赖」，不表示数据流向：

| 依赖 | 说明 |
| --- | --- |
| 对话 → 提示词 / 知识库 / 模型 | `ChatService` 持有 `PromptService`、`KnowledgeService`、`ModelProviderService`：先解析提示词，再召回切片，最后取 `ChatSpecOf` 下发模型规格 |
| 对话 → 身份提示词 / 工具 | `SystemPromptOf` 解析身份提示词，显式编码优先、默认身份兜底；`ResolveTools` 解析本轮工具，结果注入上下文块 |
| 对话 → 角色卡 / 设定 / 项目 | `loadContextInput` 按项目装载角色、设定与章节上下文 |
| 知识库 → 模型 / 任务中心 / 通知 | 重排走 `DefaultChatSpec`（系统默认模型），无模型时 fail-open；`enqueueIndexJob` 写 `jobs` 行后投递队列，worker 回写任务与文档状态并发通知 |
| 知识库 → 应用配置 | `resolveRAGTopK` 读 `rag_top_k` 配置决定召回条数 |
| 菜单 → 角色 | `MenuService.Nav` 调 `RoleService.PermCodesOfUser` 取权限并集；`RequirePermMiddleware` 调 `RoleService.HasPerm` |
| 模型 → 角色 | 授权表按角色 ULID 关联；`isAdminUser` 用 `*` 权限码判定管理员 |
| 认证 / 用户管理 → 通知 | 改密、建号、重置密码各发一条 `account` 通知，失败只告警 |
| 认证 → 登录日志 | 三条失败分支与成功分支各写一条登录结果 |
| 提示词 → 项目 | 项目提示词需带项目并经 `mustProjectAccess` 校验 |
| 项目 → 知识库 / 角色 / 设定 / 大纲 / 连续性 | `ProjectService.Delete` 在事务内级联删这些表，提交后清 MinIO 与 Manticore |
| 数据总览 → 项目 | 所有统计查询 `JOIN projects` 按 `user_ulid` 过滤，不直接信任前端传入的归属 |
| 用量分析 ← ai.Client | `ai` 只依赖 `ai.UsageRecorder` 接口，由 `main.go` 注入 `ModelUsageService`，避免叶子层反向依赖 service |
| 运行状态 → 模型 | 复用 `Capability` 得出「AI 是否就绪」，不自建模型探测逻辑 |

三条贯穿全局的约定：

1. **归属校验统一走 `mustOwnProject`**：项目、卷、章节、角色卡、设定、大纲、连续性、知识库、对话、提示词库全部经此函数；提示词例外地只校验项目存在（`mustProjectAccess`），方便协作。
2. **事务边界在 service**：跨表写入用 `repo.WithTx`（章节更新、项目级联删除、切片重建、助手消息 + AI 字数），事务外只做可重建或可降级的外部清理。
3. **外部依赖失败按「能力边界」处理，而非一律中断**：站内通知、审计日志、登录日志、用量缓冲、Manticore 同步、令牌黑名单查询失败只降级并告警，主链路继续；而 MinIO 写入/读取、队列投递、检索双通道全失败属于该能力本身不可用，直接返回 `ERR_STORAGE`。
