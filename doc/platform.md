# 平台管理模块

本文逐个说明 NovelWorkbench 平台管理模块的职责、数据结构、实现路径、边界与设计权衡。
每个模块固定两段：**职责与实现**（做什么、落在哪些表上、由哪些文件实现、关键规则是什么）与**边界与权衡**（不做什么、失败如何处理、为什么这样设计、代价是什么）。

创作业务模块见 [modules.md](modules.md)。接口清单见 [endpoints.md](endpoints.md)，接口约定见 [api.md](api.md)，表结构见 [database.md](database.md)，模块地图见 [architecture.md](architecture.md)。

# 模块设计与实现

本文逐个说明 NovelWorkbench 业务模块的职责、数据结构、实现路径、边界与设计权衡。
每个模块固定两段：**职责与实现**（做什么、落在哪些表上、由哪些文件实现、关键规则是什么）与**边界与权衡**（不做什么、失败如何处理、为什么这样设计、代价是什么）；接口字段见 [api.md](api.md)，表结构见 [database.md](database.md)，AI 细节见 [ai.md](ai.md)，模块地图见 [architecture.md](architecture.md)。

后端在 `backend/internal/api/router.go` 内注册 127 条 GET/POST 路由（另加 `/health`、`/ready` 两个探针）、36 张 AutoMigrate 表、7 份内置任务提示词与 3 份内置身份提示词（3 个流程域）、6 个内置工具与 3 个内置工作流模板。

## 1. 认证与会话

**职责与实现**

登录、查询当前用户、个人资料维护、修改密码、登出吊销。写入 `users`（`password_hash` 为 bcrypt，`json:"-"` 永不输出）与 `login_logs`；会话状态不落库，靠 JWT + Redis 黑名单维持。

| 数据 | 位置 | 说明 |
| --- | --- | --- |
| 令牌 | 客户端 `Authorization: Bearer` | HS256，声明含 `user_ulid`、`exp`、`iat`、`iss=NovelWorkbench` |
| 吊销标记 | Redis `nw:token:blacklist:<sha256(token)>` | 值 `"1"`，TTL 取 `expires_at - issued_at` |
| 登录结果 | `login_logs` | `success` / `failed` / `disabled`，含 IP 与 UA |

`service/auth_service.go` 的 `AuthService.Login`：`userRepo.GetByAccount` 同时匹配 `user_name` 与 `user_email`；账号不存在与密码错误均返回 `ERR_UNAUTHORIZED`「账号或密码错误」，`disabled` 返回 `ERR_FORBIDDEN`；成功后签发令牌并把 `role_codes` 一并返回。`AuthService.Logout`：`jwt.Parser` 非验签解析出 `exp`/`iat`，取差值作为 TTL 调 `tokenStore.Revoke`。`AuthService.ChangePassword`：先 `bcrypt.CompareHashAndPassword` 校验旧密码，再写新哈希，最后发一条 `account` 类型站内通知。`pkg/middleware.go` 的 `AuthMiddleware`：剥 `Bearer` 前缀 → `ParseToken` → `tokenStore.IsRevoked` → `userStateChecker` 查库确认账号仍存在且未禁用，全部通过后把 `user_ulid` 写入 Gin 上下文。`pkg/jwt.go` 的 `NewJWTSigner` 在密钥为空时直接返回错误，由 `cmd/server/main.go` 在启动期 fail fast。

**边界与权衡**

- 不提供注册接口，账号只能由管理员创建；不存在 `/auth/register` 路由。
- 密码错误与账号不存在返回同一文案与同一业务码，不泄露账号是否存在。
- 登录日志写入失败只打警告，不阻断登录。
- 令牌吊销依赖 Redis：`TokenRepo.IsRevoked` 在 Redis 异常时返回 `false`（视为未吊销），由后续用户状态校验兜底。
- 修改密码不吊销已签发的旧令牌，只发通知；禁用账号与删除用户则由 `AuthMiddleware` 的用户状态检查即时生效。

取舍：无状态 JWT 让横向扩容与 SSE 长连接不需要会话粘滞，代价是「主动失效」只能靠外部黑名单（键取令牌哈希，TTL 取签发时长，条目最多存活一个令牌周期）。每请求回查用户状态多一次主库读，换来「禁用立即生效」，对当前低并发管理型系统划算；若 QPS 上升，这一跳是第一个该加短 TTL 缓存的地方。

## 2. 用户与角色权限

**职责与实现**

账号生命周期（创建、编辑、禁用、重置密码、删除）与角色权限模型（角色 CRUD、权限码勾选、权限判定）。`users.user_name` 与 `user_email` 唯一，`roles.role_code` 唯一，`perm_codes_json` 存权限码 JSON 数组（`["*"]` 表示全部），`user_roles` 是多对多关联表、无独立权限表。内置角色两条：`admin`（通配）与 `user`（空权限集），由 `service/role_service.go` 的 `roleDefaultList` 与 `SeedDefaults` 播种。

`service/user_admin_service.go` 的 `UserAdminService.Create`：登录名与邮箱分别查重 → bcrypt 哈希 → 建用户 → `roleRepo.ReplaceUserRoles` 挂角色（缺省 `user`）→ 发「账号已开通」通知。`Update`：邮箱/头像/备注为三态字段（缺省不改、空串清空），`role_codes` 缺省不改、传数组整体替换，保存后回读整行；拒绝「禁用自己」。`Delete`：拒绝删除自己 → 清角色挂载 → 清本人模型授权 → 清本人通知 → 删用户行；目标已不存在时直接成功（幂等）。`BootstrapAdmin`：仅当 `users` 表为空时按配置创建 `admin` 账号并挂 `admin` 角色，是系统唯一的「自动建号」路径。`service/role_service.go` 的 `Save`：`role_ulid` 为空则创建（编码重复返回 `ERR_CONFLICT`）；更新时内置角色不允许改编码；权限码经 `normalizePermCodeList` 校验（去重保序、剔除空白、未登记编码报错、`*` 仅 `admin`），`perm_codes` 缺省表示不修改，保存后回读整行再返回。`HasPerm`…

**边界与权衡**

- 内置角色不可删除、不可改编码；角色下仍有用户时删除被拒（`ERR_CONFLICT`）。
- 管理员不能禁用或删除自己；邮箱与登录名唯一性由数据库唯一索引与服务层双重保证。
- 角色授权写入前校验权限码登记状态：未在菜单登记的编码、以及非 `admin` 角色的 `*`，一律拒绝；菜单删除/停用/改码后由 `PrunePermCode` 反向清理角色里的悬挂码。
- 内置角色权限在启动时自动修复：`admin` 固定 `*`，其他内置角色剔除 `*` 与未登记编码（普通角色带 `*` 会让前端把权限面板锁成只读，只能由启动期自愈纠正）。
- 删除项目只校验归属，不额外要求权限码；提示词使用对所有登录用户开放，只有维护动作收口到 `prompt:manage`。
- 权限判定逐请求查库、不做缓存，因此角色权限修改在下一次请求即生效。

取舍：权限码存在角色行的 JSON 列里，而不是「角色-权限」关联表：读路径只有一次 `roles` 查询加内存遍历，写入是整行覆盖，模型极简。代价是无法按权限码反查角色、也无法在数据库层做外键约束，权限码拼写错误只能靠人工发现。逐请求查库省掉缓存失效问题，代价是管理接口每次多一跳主库——对「写操作少、管理端并发低」的定位可以接受。

## 3. 菜单与导航

**职责与实现**

`menus` 表同时承担两件事：前端导航树的唯一数据源，以及权限码的登记处（菜单即权限单一事实源）。表内 28 条内置菜单由 `service/menu_service.go` 的 `menuSeedList` 播种：3 个分组（创作 / AI 资产 / 系统）、24 个页面节点、1 条动作节点 `action_model_write`。其中 5 条页面置 `visible_flag=false`：`character`、`lore`、`knowledge`、`notes` 的入口已并入素材库，`profile` 由顶栏用户菜单进入。它们不进导航，但必须有菜单行——菜单即路由，不登记就没有路由，顶栏入口点下去会落到 404。`service/access_control_chain_test.go` 在同一条导航链路里校验种子的可注册性（组件路径在前端有对应页面文件、编码与路由唯一）。

`MenuService.SeedDefaults`：按 `menu_code` 查缺补插，父级编码先于子级解析，缺父直接报错；已存在的行由 `syncSeed` 校正名称、类型、作用域、路由、图标、权限码与排序，保证升级后导航与权限不漂移。`HiddenFlag` 是例外——它强制把 `visible_flag` 写成 false（用于「入口合并后仍保留旧路由」的场景），其余展示开关保留作者改动。`MenuService.Nav`：`roleSvc.PermCodesOfUser` 取权限并集 → `menuRepo.ListEnabled` → `filterVisibleMenus`（剔除动作节点与未命中权限码的节点，`*` 全放行；隐藏页保留并带 `visible_flag=false`）→ `buildMenuTree` 按 `parent_ulid` 递归装配，返回 `menu_tree_list` 与 `perm_code_list`。`MenuService.Save`：分组不能配路由；`validateParent` 阻止指向自己或自己的下级（成环检测，深度上限 32）；`menu_code` 由 `GetByCode` 判重，`perm_code` 由 `CountByPermCode` 判重（排除自身）。`MenuService.Delete`：存在下级时返回 `ERR_CONFLICT`，不做级联。`PermissionCatalog` / `buildPermCatalog`：把菜单行上的非空 `perm_code` 按首段分组，`*` 单独登记为「全部权限」，同一权限码重复登记只展示一次。`service/menu_permission.go` 承载这两组纯函数（`filterVisibleMenus`、`buildMenuTree`、`buildPermCatalog` 及权限分组映射），与 `menu_service.go` 的仓储编排分开，便于脱离数据库单测。前端 `store/modules/user.ts` 的 `loadNav` 拉取 `GET /menu/nav`，把树与权限码同时写入 menu store 与 user store；`components/Menu/index.vue` 按 `menu_tree_list` 渲染导航，项目内菜单在未选项目时置灰。

**边界与权衡**

- 动作节点永不进入导航树，只在权限目录与角色勾选里出现。
- 菜单删除不做级联：有下级先删下级，避免悬空 `parent_ulid`。
- `Nav` 只返回启用菜单，禁用后用户刷新导航即消失。
- 隐藏页（`visible_flag=false`）仍然下发，前端据此注册路由——这样「把四个页面合并成一个素材库入口」时，旧链接与深链不会失效。
- 导航只决定「看得见什么」，接口鉴权仍由 `RequirePermMiddleware` 独立判定。

取舍：把权限码登记在菜单表上，省掉了「权限表 + 角色-菜单关联表」两层结构：权限目录、角色勾选、导航过滤共用同一份数据，新增一个受控页面只需加一条菜单行。代价是权限码与「页面」概念绑死，纯动作权限必须伪造成 `action` 节点（如 `action_model_write`），菜单管理页因此混着导航配置与权限登记两件事；同时删除菜单会连带删除该权限码的登记。

## 4. 审计与登录日志

**职责与实现**

| 存储 | 记录内容 | 触发者 |
| --- | --- | --- |
| Manticore `nw_audit` | 操作者、动作码（去掉 `/api/v1/` 的路径）、级别、链路 ID、请求路径/方法/体（脱敏）、目标类型与 ULID、业务响应码与说明、HTTP 状态、耗时、IP、UA | `pkg.AuditMiddleware` |
| PG `login_logs` | 账号、命中用户、结果码、失败原因、IP、UA | `AuthService.recordLogin` → `LoginLogService.Record` |

审计落在 Manticore 全文表而非 PostgreSQL：审计记录只增不删、按时间滚动清理、查询以关键字与时间范围为主，
这三件事都与全文索引的形态吻合，而写进关系库只会得到一张不断膨胀、最终还要定期 `DELETE` 的表。
PostgreSQL 里的历史 `audit_logs` 表在启动时自动删除，避免两份数据打架。

`pkg/access_middleware.go` 的 `AuditMiddleware`：仅对 `POST` 且不在跳过名单内的请求生效。请求前读取并还原 body（≤32KB，`multipart` 不采样），把凭据字段（`password_text`、`new_password_text`、`old_password_text`、`api_key_text`、`access_token`）替换为 `***`；用包装后的 `ResponseWriter` 捕获响应体（≤32KB，超限置截断标记），`c.Next()` 后解析包络得到 `resp_code` / `resp_message`，再按 HTTP 状态推导级别（`info` / `warn` / `error`），并写入 `trace_id`（与响应包络同值）。`extractAuditTarget` 按 `auditTargetKeys` 白名单从请求体提取目标（user / role / menu / project / chapter / document / prompt / tool / model / provider / chat_session / notification），取第一个命中键。跳过名单在 `cmd/server/main.go` 装配：`/api/v1/auth/login`、`/api/v1/auth/logout`、`/api/v1/chat/send`（SSE 流不适合等待后置写入）。

`service/audit_service.go` 提供五个能力：

| 方法 | 作用 |
| --- | --- |
| `Record` | 落一条审计；`user_name` 为空时回查用户登录名 |
| `ListPage` | 分页查询，支持级别 / 链路 ID / 目标类型 / 动作码 / 关键字 / 用户 / 日期区间过滤 |
| `Detail` | 单条详情，返回脱敏请求体与响应体 |
| `Trace` | 按链路 ID 聚合，返回同一次请求链路的全部记录 |
| `ActionOptions` | 返回库里出现过的动作码，供筛选下拉使用 |

留存策略：默认 30 天（`audit_retention_days` 应用配置可调）。`Retention` 返回当前天数、总条数与上次清理时间；
`Purge` 删除早于保留期的记录。每日 03:00 由定时任务自动执行一次清理。

工作流的每一步也写一条审计（动作码 `workflow/step/<类型>`），链路 ID 取运行的 `trace_id`，
因此一次运行的「创建 → 起草 → 体检 → 复审 → 审核 → 入库」能在链路时间线里串成一条完整轨迹。

**边界与权衡**

- 审计只覆盖 `POST`，GET 查询不记录，避免读接口把表打成热点。
- 业务失败也记录（写入发生在 `c.Next()` 之后），审计写失败不阻断业务，只记 `audit_record_failed` 警告。
- 请求体采样上限 32KB，上传与超大请求体只记占位说明。
- 登录日志失败不阻断登录链路；日志里的失败原因比接口响应更具体，只对管理员可见。
- Manticore 不可用时审计写入降级为丢弃，主链路不受影响。

取舍：审计存全文表换来「关键字 + 时间范围」的自然查询与按天滚动清理，代价是失去关系库的事务保证——审计是事后留痕，不参与业务一致性，因此这个代价可接受。同步写入保证「业务提交即有审计」的顺序一致性，代价是每个 POST 多一次写；当前管理端写操作稀疏，表不会成为瓶颈。body 采样上限 32KB 是对「审计价值 / 拷贝成本」的折中。凭据字段一律脱敏，审计表即使被读也不泄露口令。登录日志与审计日志分表，是因为查询维度与保留策略不同。

## 5. 应用配置

**职责与实现**

`app_settings` 只有两行，由 `service/setting_service.go` 的 `settingDefaultList` 定义，键、默认值、说明、合法区间同源：

| 键 | 默认值 | 区间 | 消费方 |
| --- | --- | --- | --- |
| `rag_top_k` | 6 | 1–20 | `KnowledgeService.resolveRAGTopK` |
| `daily_word_goal` | 2000 | 100–100000 | 仪表盘每日目标展示 |

`SeedDefaults`：按 `setting_key` 查缺补插；已存在但说明文案变化时只更新 `setting_desc`，不覆盖用户改过的值。`Update`：`settingKeyAllowed` 白名单校验 → `strconv.Atoi` 要求整数 → `settingRangeMap` 校验区间 → 更新值。消费侧「应用配置 > 全局配置」两级回退：`resolveRAGTopK` 先看应用配置 `rag_top_k`，再落到全局配置。路由：`GET /setting/list` 对所有登录用户开放（前端要读每日目标），`POST /setting/update` 需要 `system:setting:manage`。

**边界与权衡**

取舍：只支持整数配置，非整数、越界值与不支持的键都被拒绝（`ERR_INVALID_PARAM`）。新增配置必须改 `settingDefaultList` 与 `settingRangeMap` 并重启播种，运行时只能改值、不能改结构，避免出现无人认识的配置键。

「配置即表 + 每次读」让保存后立即生效，不需要重启或广播失效，也不需要配置中心。代价是热路径（每次对话、每次检索）多一次小表查询，且读取失败时调用方要回退到配置文件值。前端 `views/Settings/index.vue` 里另有一份展示元数据（分组、单位、区间），与后端区间重复，改动时需要两边同步。

## 6. 站内通知

**职责与实现**

`notifications` 表按用户隔离，读后保留。类型三值：`system`（公告）、`task`（异步任务结果）、`account`（账号事件）。生产者：改密、建号、重置密码各发一条 `account`；`KnowledgeWorker.notifyOwner` 发 `task`。

`NotificationService.Notify`：`user_ulid` 为空直接返回（如项目已删除导致接收人缺失）；标题 128、正文 512、链接 256 字符截断；类型缺省 `system`。`ListPage` / `UnreadCount` / `MarkRead` / `MarkAllRead` / `Delete` 全部以 `user_ulid` 作为查询条件，不接受跨用户 ID。`DeleteByUser` 在账号删除时清理该用户全部通知。前端 `store/modules/notification.ts` 维护未读数；`layout/components/NotificationBell.vue` 挂载时刷新并每 60 秒轮询一次。

**边界与权衡**

取舍：无实时推送，延迟上限等于轮询间隔（60 秒）；所有写入失败只告警，绝不影响主链路。用「落库 + 轮询」而不是消息推送，省掉长连接与网关改造成本，也天然支持离线用户与历史回看；代价是未读数有最多 60 秒延迟，且每次轮询要跑一次 `count` 查询。通知内容在写入时截断并做快照（存文本不存模板 ID），因此历史通知不会随后续文案调整而变化。

## 7. 任务中心

**职责与实现**

`jobs` 表是异步任务的留痕与运维入口，当前唯一任务族是知识库索引：`doc_index`（首次）与 `doc_reindex`（重建）。`payload_json` 存 `{"doc_ulid": "DOC_..."}`。状态取值 `queued` / `running` / `success` / `failed`，但 worker 只写 `queued`（投递时）、`success`、`failed` 三态。

`service/knowledge_service.go` 的 `enqueueIndexJob`：先建 `queued` 任务行，再 `mqRepo.PublishIndexJob`；投递失败把任务置 `failed` 并返回 `ERR_STORAGE`。`service/knowledge_worker.go` 的 `Run`：启动时 `ensureManticoreSchema`，结构升级时 `rebuildManticoreIndex` 从 PostgreSQL 回灌；随后阻塞消费，消费中断按 5 秒间隔重连，`ctx` 取消时优雅退出。`handleIndexJob`：`GetPendingByDoc` 找回任务行 → 文档置 `indexing` → 内部重试至多 3 次，退避 1s/2s/3s → 成功置 `success` 并发通知，全部失败置 `failed` 并回写 `error_message`，随后 `ack`（毒消息不重投）。`service/job_service.go` 的 `ListPage` 解析载荷批量取文档名避免 N+1；`Stats` 按状态计数；`Retry` 只接受 `failed`，把任务重置为 `queued`、文档重置为 `pending` 后重新投递。

**边界与权衡**

- 只有 `failed` 任务可重试；载荷缺 `doc_ulid` 或文档已删除时拒绝。
- 文档已删除的滞留消息直接 `ack` 并置任务 `failed`。
- 文档状态与任务状态是两套：前者面向用户（`pending/indexing/indexed/failed`），后者面向运维。
- 没有超时回收器：worker 在索引中途崩溃会留下 `queued` 任务与 `indexing` 文档，这两种中间态都没有自助恢复入口，需要人工改状态或重新上传文档。

取舍：任务行在投递前创建，保证「消息丢了也能看到一条 queued/failed」，比纯 MQ 语义更可运维。重试放在 worker 内部而不是让 MQ 重投，避免毒消息在队列里打转；代价是重试次数只体现在 `retry_count`，队列层面看不到投递次数。把 Manticore 索引当作可重建副本来处理（失败只降级关键词通道，不阻断文档 `indexed`），是这条链路最重要的取舍：权威数据永远在 PostgreSQL 与 MinIO。


## 8. 模块间依赖

调用方向严格单向，下表的箭头表示「调用/依赖」，不表示数据流向：

| 依赖 | 说明 |
| --- | --- |
| 对话 → 提示词 / 知识库 / 模型 | `ChatService` 持有 `PromptService`、`KnowledgeService`、`ModelProviderService`：先解析提示词，再召回切片，最后取 `ChatSpecOf` 下发模型规格 |
| 对话 → 身份提示词 / 工具 | `SystemPromptOf` 解析身份提示词，显式编码优先、默认身份兜底；`toolDeclarations` 取启用工具的声明随请求下发，由模型自行决定调用 |
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
