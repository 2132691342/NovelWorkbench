# 功能实现说明

本文按「用户能力 → 系统怎么实现 → 验收点」描述 NovelWorkbench 的每一个功能域。
每条实现都标注具体文件与函数，可直接对照代码验证。接口契约见 `doc/api.md`，
数据结构见 `doc/database.md`，部署与配置见 `doc/deploy.md`。

约定：

- 后端路径以 `backend/internal/` 为根，前端路径以 `frontend/src/` 为根。
- 「实现」条目写明文件与函数名；函数行为以代码为准。
- 「验收点」只写可观察的结果（响应内容、状态码、界面表现），不写实现细节。
- 所有写操作都有项目归属校验，越权返回 `ERR_FORBIDDEN`（`pkg.NewBizError(domain.RespCodeForbidden, ...)`）。

---

## 1. 系统管理

### 1.1 认证与会话

**用户能力**

- 用登录名或邮箱登录（系统不开放注册，账号由管理员分配）。
- 查看当前登录用户信息、角色编码与权限码集合。
- 退出登录，令牌立即失效。

**实现**

- `api/auth_api.go` 的 `Login` / `Profile` / `Logout` 只做参数绑定与包络输出。
- `service/auth_service.go` 的 `Login`：`userRepo.GetByAccount` 同时匹配登录名或邮箱；
  账号状态为 `disabled` 时返回 `ERR_FORBIDDEN`，密码不匹配返回 `ERR_UNAUTHORIZED`。
- `service/auth_service.go` 的 `withRoleCodes`：登录与个人信息都带出 `role_codes`（`roleRepo.ListRoleCodesByUserULID`）。
- `pkg/jwt.go` 的 `NewJWTSigner` / `SignToken` / `ParseToken`：HS256 签名，令牌载荷含 `user_ulid` 与有效期。
- `service/auth_service.go` 的 `Logout`：解析令牌有效期后调用 `tokenStore.Revoke` 写黑名单，TTL 为令牌剩余寿命。
- `repo/token_repo.go` 的 `tokenBlacklistKey`：黑名单键为 `nw:token:blacklist:<sha256hex(token)>`。
- `pkg/middleware.go` 的 `AuthMiddleware`：校验 Bearer 令牌 → 查黑名单 → 查用户状态，注入 `user_ulid`。
- `cmd/server/main.go`：免鉴权白名单只有 `/api/v1/auth/login`；不再有注册路由。

**验收点**

- 密码错误与账号不存在都返回 `ERR_UNAUTHORIZED`，提示文案不区分二者。
- 账号被禁用后登录返回 `ERR_FORBIDDEN`；该账号已签发的旧令牌再访问业务接口返回 `ERR_UNAUTHORIZED`。
- 登出后原令牌立即失效；被吊销的令牌不写库、不入业务表。
- 响应中不出现 `password_hash` 字段。
- `POST /api/v1/auth/register` 不存在（404），前端登录页不再提供注册入口。

### 1.1.1 登录与导航链路

**用户能力**

- 登录后侧边栏出现属于自己的菜单，无权菜单不显示。
- 顶栏可快速跳转到任意可见页面。

**实现**

- `service/menu_service.go` 的 `Nav`：取角色权限码并集 → 过滤禁用/隐藏/动作节点 → 无权限节点剔除 →
  `buildMenuTree` 装配递归树。
- `service/role_service.go` 的 `PermCodesOfUser`：多角色权限码取并集去重。
- `frontend/src/stores/menu.ts` 的 `loadNav`：登录后与进入业务页前拉取，权限码同步给 auth store。
- `frontend/src/router/index.ts` 的守卫：未加载导航则先加载，`meta.perm_code` 命中失败回仪表盘。
- `frontend/src/layout/AdminLayout.vue`：侧边栏按 `menu_tree_list` 渲染（分组/页面），
  项目内菜单的 `:project_ulid` 用当前项目替换，未选项目时置灰。

**验收点**

- 仅 `skill:manage` 的账号看不到「系统」分组下的管理页，访问 `/admin/users` 被重定向回仪表盘。
- 禁用某个菜单后刷新导航，该菜单从侧边栏与快速跳转中消失。
- 项目内菜单在未选择项目时不可点击。

### 1.2 用户管理

**用户能力**

- 分页检索用户（按关键词与状态过滤）。
- 新建用户并指定角色。
- 修改用户资料、状态与角色。
- 重置用户密码。
- 删除用户。

**实现**

- `api/user_admin_api.go` 的 `List` / `Create` / `Update` / `Delete` / `ResetPassword` 做绑定与输出。
- `service/user_admin_service.go` 的 `ListPage`：分页取用户后用 `roleCodeMap` 批量组装角色编码（两查防 N+1）。
- `service/user_admin_service.go` 的 `Create`：登录名、邮箱查重，`bcrypt` 哈希密码；
  角色为空时默认挂 `user`（`domain.RoleCodeUser`）；`roleRepo.ReplaceUserRoles` 覆盖挂载。
- `service/user_admin_service.go` 的 `Update`：邮箱变更需查重；`user_status_code=disabled` 且操作人等于目标用户时
  返回「不能禁用自己」；角色列表非 `nil` 时整体覆盖。
- `service/user_admin_service.go` 的 `Delete`：禁止删除自己；先 `roleRepo.DeleteUserRoles` 再删用户。
- `service/user_admin_service.go` 的 `ResetPassword`：`bcrypt` 重哈希后覆盖 `password_hash`。
- `service/user_admin_service.go` 的 `BootstrapAdmin`：仅当 `userRepo.Count` 为 0 时创建管理员 `admin`
  并挂 `admin` 角色；表非空时不动作。

**验收点**

- 重复登录名/邮箱创建用户返回 `ERR_CONFLICT`。
- 不能禁用或删除自己（返回 `ERR_BIZ_FAILED`）。
- 删除用户后其角色关联被清理。
- 用户列表项带出角色编码列表。
- 用户表非空时重启服务不再创建引导管理员。

### 1.3 角色与权限

**用户能力**

- 分页查看角色与权限码。
- 新建/编辑角色并从权限目录勾选权限码。
- 删除角色。

**实现**

- `api/role_api.go` 的 `List` / `Save` / `Delete`。
- `service/role_service.go` 的 `SeedDefaults`：启动播种内置角色 `admin`（权限码 `["*"]`）与 `user`（权限码 `[]`），
  按 `role_code` 查缺补插。
- `service/role_service.go` 的 `Save`：角色编码唯一；内置角色改编码被拒；
  权限码列表序列化为 JSON 存入 `perm_codes_json`（空列表存 `[]`）。
- `service/role_service.go` 的 `Delete`：内置角色拒删；`roleRepo.CountUsersByRoleULID` 大于 0 时拒删。
- `service/role_service.go` 的 `HasPerm`：逐请求查库，权限码含 `domain.PermAdminAll`（`*`）即放行。
- `pkg/access_middleware.go` 的 `RequirePermMiddleware`：无权限返回 `ERR_FORBIDDEN`，未登录返回 `ERR_UNAUTHORIZED`。
- `domain/role.go` 定义权限码常量：`system:user:manage`、`system:role:manage`、`system:menu:manage`、
  `system:perm:read`、`system:audit:read`、`system:setting:manage`、`system:model:manage`、
  `system:usage:read`、`system:status:read`、`skill:manage`。
- `api/router.go` 的 `registerAdminRoutes`：用户组收口到 `system:user:manage`，角色组到 `system:role:manage`，
  审计组到 `system:audit:read`；`registerMenuRoutes` 收口菜单与权限目录；
  `registerSkillRoutes` 把技能写操作收口到 `skill:manage`。
- 前端角色页的权限选项来自 `GET /menu/permission/catalog`，不再硬编码。

**验收点**

- 内置角色 `admin` / `user` 不可删除，`admin` 的编码不可修改。
- 角色下仍有用户时删除返回 `ERR_CONFLICT`。
- 普通用户访问管理接口返回 `ERR_FORBIDDEN`；权限码改动后下一次请求即生效（逐请求查库）。
- `admin` 角色持有 `*`，可访问全部管理接口。
- 在「菜单管理」新增带 `perm_code` 的菜单后，角色页无需改代码即可勾选该权限。

### 1.3.1 菜单管理

**用户能力**

- 树形查看全部菜单（分组/页面/动作），含编码、路由、权限码与状态标签。
- 新建/编辑菜单：名称、编码、类型、作用域、路由、图标、权限码、展示与启用开关、排序。
- 删除菜单（有下级时拒绝）。

**实现**

- `api/menu_api.go` 的 `List` / `Detail` / `Save` / `Delete` / `Nav` / `PermCatalog`。
- `service/menu_service.go` 的 `Save`：编码唯一、分组不能配路由、权限码唯一（`CountByPermCode`）、
  `validateParent` 逐级上溯最多 32 层做环检测。
- `service/menu_service.go` 的 `Delete`：`CountChildren` 大于 0 返回 `ERR_CONFLICT`。
- 路由：`GET /menu/detail/{MenuULID}`，路径参数置于末尾。
- 前端 `views/admin/menus.vue`：`el-tree` 展示 + 右侧抽屉表单（图标下拉含预览）。

**验收点**

- 新增菜单后刷新页面，侧边栏立即出现该菜单（同一权限码的前提下）。
- 同级排序号决定侧边栏顺序。
- 删除有下级的菜单返回 `ERR_CONFLICT`。
- 菜单编码重复返回 `ERR_CONFLICT`；权限码已被登记返回 `ERR_CONFLICT`。

### 1.3.2 权限目录

**用户能力**

- 按分组浏览全部已登记权限码、名称与说明。

**实现**

- `service/menu_service.go` 的 `PermissionCatalog`：遍历菜单行，按 `perm_code` 首段分组
  （`all`/`system`/`skill`/`project`/`other`），同码去重，`*` 单独登记为「全部权限」。
- `GET /menu/permission/catalog` 收口到 `system:perm:read`。
- 前端 `views/admin/permissions.vue`：左侧分组、右侧权限表，显示总数。

**验收点**

- 权限目录条目与菜单表中非空 `perm_code` 一一对应（去重后）。
- 新增菜单登记新权限码后，目录与角色页选项同时出现该码。

### 1.4 审计日志

**用户能力**

- 按操作者、动作、目标、时间分页查看写操作审计记录。

**实现**

- `pkg/access_middleware.go` 的 `AuditMiddleware`：记录所有 POST 请求（除跳过名单外）的
  操作者 ULID、动作码（路径去掉 `/api/v1/` 前缀）、目标类型与目标 ULID、HTTP 状态与客户端 IP。
- `pkg/access_middleware.go` 的 `extractAuditTarget`：仅 `/api/v1/admin/` 路径读 body，
  body 上限 64 KiB；从 `user_ulid` / `role_ulid` / `project_ulid` / `chapter_ulid` / `doc_ulid`
  五个白名单键提取目标，读后还原 body 供 handler 绑定。
- `cmd/server/main.go`：`AuditMiddleware` 跳过名单为登录、登出与 `/api/v1/chat/send`。
- `service/audit_service.go` 的 `Record`：自动补全操作用户登录名；落库失败不阻断业务，仅记 warn 日志。
- `api/audit_api.go` 的 `List` 分页输出审计记录。

**验收点**

- 每次管理端 POST 都留下一条审计记录，含动作码与目标 ULID。
- 非 `/api/v1/admin/` 路径的写操作不产生目标类型/目标 ULID。
- 审计写入失败不影响业务响应。
- 审计查询按权限码 `system:audit:read` 收口。

### 1.5 应用配置

**用户能力**

- 查看运行时配置项。
- 修改白名单内配置的值。

**实现**

- `service/setting_service.go` 的 `settingDefaultList`：配置键白名单的唯一来源，
  默认 `chat_history_count=5`（AI 对话历史注入条数）、`rag_top_k=6`（知识库默认召回条数）、`daily_word_goal=2000`（仪表盘每日写作目标）。
- `service/setting_service.go` 的 `SeedDefaults`：按 `setting_key` 查缺补插。
- `service/setting_service.go` 的 `Update`：键必须在白名单内（否则 `ERR_INVALID_PARAM`），
  值必须为正整数；键不存在返回 `ERR_NOT_FOUND`。
- `service/chat_service.go` 的 `resolveHistoryLimit`：请求 `history_count` 大于 0 时用请求值，
  否则读 `chat_history_count` 配置，配置缺失或非法时回退 `ChatConfig.HistoryCount`。
- `service/knowledge_service.go` 的 `resolveRAGTopK`：请求 `top_k_count` 大于 0 时用请求值，
  否则读 `rag_top_k` 配置，配置缺失或非法时回退 `KnowledgeIndexConfig.RAGTopK`。
- `api/setting_api.go` 的 `List` / `Update`。

**验收点**

- 非白名单配置键返回 `ERR_INVALID_PARAM`；非正整数配置值同样被拒。
- 修改 `chat_history_count` 后，下一次 AI 对话注入的历史条数随之变化。
- 修改 `rag_top_k` 后，下一次知识库检索的默认召回条数随之变化。
- 配置项说明（`setting_desc`）随查询返回。

### 1.6 个人中心

**用户能力**

- 查看本人资料：登录名、邮箱、头像、角色、状态、创建与更新时间。
- 修改邮箱、头像地址与个人备注。
- 修改登录密码（需验证当前密码）。
- 从顶栏头像菜单进入，不占用主导航。

**实现**

- `api/auth_api.go` 的 `ProfileDetail` / `UpdateProfile` / `ChangePassword`。
- 路由：`GET /profile/detail`、`POST /profile/update`、`POST /profile/password/change`。
- `service/auth_service.go` 的 `ProfileDetail`：用户 + 角色编码组装 `UserProfileVO`。
- `service/auth_service.go` 的 `UpdateProfile`：邮箱变更查重；头像与备注允许清空。
- `service/auth_service.go` 的 `ChangePassword`：bcrypt 校验旧密码 → 重哈希 → 写安全通知。
- `frontend/src/views/profile/index.vue`：资料卡 + 三个 Tab（基本资料 / 账号安全 / 账号信息）。
- 保存成功后调用 `authStore.fetchProfile()`，顶栏头像与姓名立即刷新。

**验收点**

- 邮箱被他人占用返回 `ERR_CONFLICT`。
- 旧密码错误返回 `ERR_UNAUTHORIZED`，密码不变。
- 修改成功后通知中心出现「登录密码已修改」。
- 响应不包含 `password_hash`。

### 1.7 通知中心

**用户能力**

- 顶栏铃铛显示未读数（超过 99 显示 99+），点击查看最近 5 条。
- 通知中心按全部 / 未读 / 已读、类型与关键词筛选。
- 单条已读、全部已读、删除；带跳转地址的通知可直接打开业务页。

**实现**

- `domain/notification.go`：类型 `system/task/account`，字段含标题、正文、已读、跳转与关联业务。
- `repo/notification_repo.go`：按用户隔离的分页、未读计数、批量已读、全部已读、删除。
- `service/notification_service.go` 的 `Notify`：业务侧统一入口，标题/正文按字符截断。
- `api/notification_api.go` 与路由：`/notification/list`、`/notification/unread_count`、
  `/notification/read`、`/notification/read_all`、`/notification/delete`。
- 触发点：
  - 管理员创建账号 → 新用户收到「账号已开通」。
  - 管理员重置密码 → 目标用户收到安全提醒。
  - 本人修改密码 → 收到「登录密码已修改」。
  - 知识库索引成功 / 失败 → 项目主人收到任务结果。
- 前端 `stores/notification.ts`：未读数、最近通知、标记已读；`layout/NotificationBell.vue`
  每 60 秒轮询一次。
- 顶栏浮层与 `/notifications` 页共用同一 store。

**验收点**

- 未读数与铃铛徽标一致；标记已读后立即减少。
- 用户只能看到并操作自己的通知。
- 索引失败通知包含失败原因，可跳转知识库。
- 通知轮询失败不弹出错误提示，不打断写作。

### 1.8 登录日志

**用户能力**

- 管理端按账号 / IP、结果与日期查看登录记录。
- 区分成功、密码错误与禁用账号尝试。

**实现**

- `domain/login_log.go`：结果为 `success/failed/disabled`，记录账号、用户、IP、UA、失败原因。
- `repo/login_log_repo.go` 的 `ListPage`：账号/IP 模糊 + 结果 + 日期区间。
- `service/login_log_service.go` 的 `Record`：失败只告警，不阻断登录。
- `service/auth_service.go` 的 `Login`：三个分支分别记录；成功记录用户 ULID。
- `api/login_log_api.go` 与路由：`POST /admin/login_log/list`，收口 `system:loginlog:read`。
- 前端 `views/admin/login-logs.vue`：结果标签 + 日期范围 + 客户端 UA 展示。

**验收点**

- 账号不存在与密码错误都记录 `failed`，但对外文案一致。
- 禁用账号记录 `disabled` 且不签发令牌。
- 日志查询需要 `system:loginlog:read`；普通用户 403。
- 登录日志写入失败不影响登录结果。

### 1.9 任务中心

**用户能力**

- 查看异步任务状态概览：排队、执行、成功、失败。
- 按类型、状态与关键词筛选任务，查看关联文档与失败原因。
- 对失败任务执行重试，重新投递索引消息。

**实现**

- `domain/ops.go`：`JobListREQ`、`JobRetryREQ`、`JobVO`、`JobStatsVO` 与状态常量。
- `repo/job_repo.go`：新增 `GetByULID`、`ListPage`、`CountByStatus`。
- `service/job_service.go` 的 `ListPage`：解析载荷中的 `doc_ulid`，批量回填文档名。
- `service/job_service.go` 的 `Retry`：仅 `failed` 可重试；重置任务为 `queued`、文档为 `pending`，
  重新 `PublishIndexJob`；投递失败回写 `failed`。
- `api/job_api.go` 与路由：`POST /admin/job/list`、`GET /admin/job/stats`、`POST /admin/job/retry`，
  收口 `system:job:manage`。
- 前端 `views/admin/jobs.vue`：四张状态卡 + 筛选表格 + 失败重试。

**验收点**

- 只有失败任务显示「重试」；非失败任务重试返回 `ERR_BIZ_FAILED`。
- 重试后任务变 `queued`，文档变 `pending`，worker 可再次消费。
- 任务列表不产生 N+1 文档查询。
- 队列不可用时重试返回 `ERR_STORAGE`，任务保持 `failed`。

---

## 2. 创作主线

### 2.1 项目

**用户能力**

- 新建、编辑、删除项目。
- 分页查看自己的项目（按名称关键词与状态过滤）。
- 查看项目详情（卷数、章节数、累计字数）。
- 查看项目目录树（卷 + 每卷章节）。

**实现**

- `api/project_api.go` 的 `Create` / `Update` / `Delete` / `List` / `Detail` / `Tree`。
- `service/project_service.go` 的 `mustOwnProject`：按 `project_ulid` 取项目并比对 `user_ulid`，
  不存在返回 `ERR_NOT_FOUND`，非本人返回 `ERR_FORBIDDEN`。
- `service/project_service.go` 的 `Create`：新项目状态固定为 `draft`。
- `service/project_service.go` 的 `List`：`projectRepo.CountVolumesByProject` 与 `CountChaptersByProject`
  批量统计，避免逐项目查询。
- `service/project_service.go` 的 `Tree`：`volumeRepo.ListByProject` + `chapterRepo.ListByProject`，
  按 `volume_ulid` 分组挂到卷节点下。
- `service/project_service.go` 的 `Delete`：`repo.WithTx` 单事务内依次删除章节快照、章节、卷、
  角色、关系、设定、会话、消息、知识库切片与文档、字数统计、项目主表；事务提交后再清理
  MinIO 原文对象与 Manticore 切片行（`cleanupKnowledgeExternal`，失败仅告警）。

**验收点**

- 他人项目一律返回 `ERR_FORBIDDEN`。
- 项目列表项带出卷数与章节数。
- 删除项目后，其卷、章、角色、设定、会话、知识库文档与切片在库中零残留。
- 删除项目后 MinIO 桶内对应对象与 Manticore 对应行被清理（清理失败只记日志，不影响删除结果）。

### 2.2 卷

**用户能力**

- 在项目下新建卷。
- 修改卷名、排序号与摘要。
- 删除卷（含其下章节）。

**实现**

- `api/volume_api.go` 的 `Create` / `Update` / `Delete`。
- `service/volume_service.go` 的 `Create`：`volume_order` 缺省时取项目内 `volumeRepo.MaxOrder` + 1。
- `service/volume_service.go` 的 `Update`：仅覆盖非空字段（名称、排序号、摘要）。
- `service/volume_service.go` 的 `Delete`：`repo.WithTx` 内先 `SumWordCountByVolume` 取该卷总字数，
  删章节快照、删章节、删卷，最后 `applyWordDelta` 把字数从项目累计值中扣回。

**验收点**

- 新建卷默认排在项目末尾。
- 删除卷后项目累计字数减少该卷原有字数，不出现负值或残留。
- 删除卷同时删除其下章节与章节快照。

### 2.3 章节

**用户能力**

- 在卷下新建章节。
- 编辑章节名称、摘要、状态、正文，可换卷。
- 删除章节。
- 查看章节详情（含正文）。
- 为章节生成快照、查看快照列表、回滚到快照。

**实现**

- `api/chapter_api.go` 的 `Create` / `Update` / `Delete` / `Detail` / `SnapshotList` /
  `SnapshotCreate` / `SnapshotRestore`。
- `service/chapter_service.go` 的 `Create`：校验卷存在且 `volume.ProjectULID == req.ProjectULID`
  （否则 `ERR_INVALID_PARAM`）；`chapter_order` 取项目内 `chapterRepo.MaxOrder` + 1；状态默认 `draft`。
- `service/chapter_service.go` 的 `Update`：`repo.WithTx` 内更新；正文变更时用 `pkg.CountWords`
  重算 `word_count`，并按差值 `applyWordDelta` 维护项目累计字数与当日统计。
- `service/chapter_service.go` 的 `Delete`：事务内删快照、删章节、按章节字数回补项目累计值。
- `service/chapter_service.go` 的 `SnapshotCreate`：以当前正文生成 `snapshot_type=manual` 的快照，
  快照字数按快照正文重算。
- `service/chapter_service.go` 的 `SnapshotRestore`：事务内先把当前正文另存一条
  `change_note="restore 自动留档"` 的快照，再用快照正文覆盖章节正文并按差值回补字数。
- `service/chapter_service.go` 的 `mustOwnChapter`：经所属项目校验归属。

**验收点**

- 卷与项目不匹配时创建章节返回 `ERR_INVALID_PARAM`。
- 新建章节排在项目末尾，顺序号连续递增。
- 编辑正文后章节 `word_count` 与项目 `total_word_count` 同步变化；把正文清空后字数回落到 0。
- 快照列表按生成时间倒序；回滚后正文等于快照正文，且回滚前内容已被自动留档。
- 删除章节后项目累计字数同步扣减。

### 2.4 目录树导出

**用户能力**

- 把项目目录（卷 / 章节 / 排序 / 字数 / 状态）导出为 XLSX 文件。

**实现**

- `frontend/src/views/workspace/WorkspaceTree.vue` 的 `handleExportOutline`：
  按 `treeData.volume_list` 逐卷逐章拼行，`xlsx` 库按需 `import('xlsx')` 动态加载，
  `XLSX.utils.json_to_sheet` 生成工作表并 `XLSX.writeFile` 落盘。
- 空目录（`volume_list` 为空）时不导出，界面提示。

**验收点**

- 导出文件名形如 `<项目名>-<文件名>.xlsx`，内容含卷名、章节名、排序号、字数与状态。
- 空卷在表中保留一行仅含卷名的记录。
- 项目无卷时不产生文件。

---

## 3. 设定生态

### 3.1 角色卡

**用户能力**

- 新建/编辑角色卡（姓名、别名、定位、性格、背景、外貌、目标）。
- 删除角色。
- 按类型与关键词筛选角色列表。

**实现**

- `api/character_api.go` 的 `Save` / `Delete` / `List`。
- `service/character_service.go` 的 `Save`：`character_ulid` 为空则新建，否则更新；
  更新时校验 `existing.ProjectULID == req.ProjectULID`（否则 `ERR_INVALID_PARAM`）。
- `service/character_service.go` 的 `Delete`：`repo.WithTx` 内先
  `characterRepo.DeleteRelationsByCharacter` 再删角色，关系边随角色一起清理。
- `service/character_service.go` 的 `List`：`characterRepo.ListByProject` 按类型与关键词过滤。

**验收点**

- 角色归属项目不符时更新返回 `ERR_INVALID_PARAM`。
- 删除角色后其关系边在库中清零，关系图不再出现该节点。
- 列表按 `role_type_code` 与关键词过滤生效。

### 3.2 关系图

**用户能力**

- 查看项目角色关系图（节点=角色，边=关系）。
- 新建/删除角色之间的关系边。

**实现**

- `api/character_api.go` 的 `Graph` / `RelationSave` / `RelationDelete`。
- `service/character_service.go` 的 `Graph`：返回节点列表与边列表，节点只带角色 ULID、姓名与定位。
- `service/character_service.go` 的 `RelationSave`：两端角色必须存在、必须同项目、不允许自环，
  否则返回 `ERR_INVALID_PARAM`。
- `service/character_service.go` 的 `RelationDelete`：按 `relation_ulid` 删边并校验项目归属。
- `frontend/src/views/character/CharacterGraphTab.vue` 的 `renderGraph`：用 `@antv/x6` 的 `Graph`
  按节点列表布局并连边，边上标注关系类型文本；`loadGraph` 拉取 `GET /character/graph`。

**验收点**

- 自环或跨项目关系被拒绝，返回 `ERR_INVALID_PARAM`。
- 关系图节点数等于角色数，边数等于关系边数。
- 删除角色后关系图不再有该节点及其相关边。

### 3.3 世界观设定

**用户能力**

- 新建/编辑设定条目（名称、类型、内容、排序）。
- 删除设定条目。
- 按类型与关键词筛选设定列表。

**实现**

- `api/lore_api.go` 的 `Save` / `Delete` / `List`。
- `service/lore_service.go` 的 `Save`：新建走 `loreRepo.Create`，更新走 `loreRepo.Update`，
  更新时校验设定归属项目。
- `service/lore_service.go` 的 `List`：`loreRepo.ListByProject` 按 `lore_type_code` 与关键词过滤。
- `domain/lore.go`：设定类型枚举 `world` / `rule` / `item` / `place` / `event` / `note`，
  其中 `note` 为创作笔记类型。

**验收点**

- 设定类型只接受枚举内取值，非法类型在参数绑定阶段被拒。
- 更新他人项目的设定返回 `ERR_INVALID_PARAM`。
- 类型过滤与关键词过滤同时生效。

### 3.4 创作笔记

**用户能力**

- 在项目下新建创作笔记（标题 + 正文）。
- 编辑笔记并自动保存。
- 切换笔记、新建笔记、删除笔记。
- 按标题搜索笔记。

**实现**

- `frontend/src/views/notes/index.vue`：笔记列表来自 `listLoreNotes`（`api/lore.ts`，
  固定 `lore_type_code='note'`）；正文编辑用 WangEditor（`rich`）或 MavonEditor（`markdown`）两种模式。
- `frontend/src/views/notes/noteContent.ts`：两种模式的正文格式互转（`rich`↔`markdown`）。
  切换模式时把当前正文转换成目标格式再落盘，避免 HTML 标签漏进 Markdown 编辑器、或 Markdown 原文被当 HTML 渲染。
- `frontend/src/views/notes/index.vue` 的 `scheduleSave` / `saveNote` / `flushSave`：
  标题与正文走 2 秒防抖自动保存，调用 `POST /lore/save`（带 `lore_ulid` 为更新）；
  切换笔记前先 `flushSave` 落盘。
- `service/lore_service.go` 的 `Save`：笔记与设定共用同一条存储链路，`lore_type_code='note'`；
  `content_mode_code` 随笔记持久化，决定下次打开用哪种编辑器。
- `service/chat_context.go` 的 `formatLoreBlock`：注入 AI 上下文时跳过 `note` 类型条目。

**验收点**

- 笔记内容改动后自动保存，界面显示「保存中 / 已保存 / 未保存 / 失败可重试」状态。
- 切换笔记时未落盘内容先保存，不丢内容。
- 笔记不出现在 AI 对话的设定注入块中（私人笔记不进入提示词）。
- 切换编辑器模式后正文已完成格式转换：Markdown 编辑器里不出现 HTML 标签，
  富文本编辑器里 Markdown 已被渲染；重新打开该笔记仍停在上次选择的模式。

---

## 4. 创作规划

### 4.1 大纲树

**用户能力**

- 新建/编辑大纲节点（标题、摘要、类型、状态、排序、父节点）。
- 删除大纲节点。
- 查看项目大纲树（父子层级）。

**实现**

- `api/outline_api.go` 的 `SaveNode` / `DeleteNode` / `ListTree`。
- `service/outline_service.go` 的 `SaveNode`：新建时 `sort_order` 缺省取同父节点下
  `outlineRepo.MaxNodeOrder` + 1，状态缺省 `draft`；`validateParent` 校验父节点归属本项目、
  不得为自身、不得为自身后代（回溯父链判环）。
- `service/outline_service.go` 的 `DeleteNode`：`outlineRepo.CountNodesByParent` 大于 0 时
  返回 `ERR_CONFLICT`，要求先删子节点。
- `service/outline_service.go` 的 `ListTree`：`outlineRepo.ListNodesByProject` 取全量节点，
  `attachOutlineChildren` 递归组装父子关系，根节点按排序号升序。
- `frontend/src/views/outline/index.vue`：树形表格渲染节点，支持新建子节点、编辑、删除。

**验收点**

- 父节点设为自身或自身后代返回 `ERR_INVALID_PARAM`。
- 有子节点的节点删除返回 `ERR_CONFLICT`。
- 同一父节点下新增节点的排序号递增。
- 大纲树按层级返回，根节点在顶层，父节点在子节点之前。

### 4.2 场景卡

**用户能力**

- 在大纲节点下新建/编辑场景卡（地点、冲突、节奏、预计字数、入场状态、目标、关键选择、结果、退场状态）。
- 在场景卡中关联出场角色。
- 删除场景卡。
- 按节点查看场景列表。

**实现**

- `api/outline_api.go` 的 `SaveScene` / `DeleteScene` / `ListScenes`。
- `service/outline_service.go` 的 `SaveScene`：校验所属节点归属本项目；角色 ULID 列表经
  `encodeCharacterULIDs` 编码为 JSON 数组存 `character_ulid_list`；新建时排序号取同节点下
  `outlineRepo.MaxSceneOrder` + 1。
- `service/outline_service.go` 的 `ListScenes`：`outline_ulid` 为空查项目全部场景，
  非空时先校验节点归属再按节点过滤。
- `service/outline_service.go` 的 `toDetailedSceneVO`：回读时解析角色 ULID JSON 数组，
  解析失败返回 `ERR_INTERNAL`；空数组回填为 `[]`。
- `frontend/src/views/outline/index.vue` 的 `submitScene` / `removeScene` / `loadScenes`。

**验收点**

- 场景卡只能挂在本项目的大纲节点下。
- 场景卡的关联角色列表回读为数组（无关联时为空数组而非 null）。
- 按节点过滤只返回该节点下的场景。
- 删除场景后场景列表中不再出现。

---

## 5. 连续性

### 5.1 时间事实与复核

**用户能力**

- 记录时间事实（主体、谓词、客体、有效期章节、来源章节与原文引用）。
- 对事实做复核（确认 / 取代 / 标记过期）。
- 按状态与章节筛选事实列表。

**实现**

- `api/continuity_api.go` 的 `SaveFact` / `ReviewFact` / `ListFacts`。
- `service/continuity_service.go` 的 `isValidFactStatusCode`：状态只允许
  `candidate` / `confirmed` / `superseded` / `stale`，其余返回 `ERR_INVALID_PARAM`。
- `service/continuity_service.go` 的 `SaveFact`：新建状态缺省 `candidate`；
  更新时状态为空则沿用原状态。
- `service/continuity_service.go` 的 `ReviewFact`：复核目标状态只允许
  `confirmed` / `superseded` / `stale`，不能退回 `candidate`；可选写备注。
- `service/continuity_service.go` 的 `ListFacts`：按状态与章节过滤。

**验收点**

- 新建事实初始状态为 `candidate`，不会自动转正。
- 复核到非法状态（含退回 `candidate`）返回 `ERR_INVALID_PARAM`。
- 复核后事实状态变为目标值，列表按状态过滤结果一致。

### 5.2 伏笔钩子

**用户能力**

- 记录伏笔钩子（描述、预期回收、埋设章节、回收章节、重要程度）。
- 推进伏笔状态（计划中 / 已埋设 / 已呼应 / 已回收 / 已放弃）。
- 按状态筛选钩子列表。

**实现**

- `api/continuity_api.go` 的 `SaveHook` / `ListHooks`。
- `service/continuity_service.go` 的 `isValidHookStatusCode` / `isValidImportanceCode`：
  状态限定五个枚举值，重要程度限定 `high` / `medium` / `low`（缺省 `medium`）。
- `service/continuity_service.go` 的 `SaveHook`：状态必须显式给出（请求绑定层 `binding:"required"`），
  更新时状态随保存推进。

**验收点**

- 非法伏笔状态或重要程度被拒（`ERR_INVALID_PARAM`）。
- 新建钩子的重要程度缺省为 `medium`。
- 按状态筛选只返回对应状态的钩子。

### 5.3 章节摘要

**用户能力**

- 为章节维护摘要与来源哈希。
- 按状态与章节筛选摘要列表。

**实现**

- `api/continuity_api.go` 的 `SaveSummary` / `ListSummaries`。
- `service/continuity_service.go` 的 `isValidSummaryStatusCode`：状态限定
  `pending` / `verified` / `stale`（缺省 `pending`）。
- `service/continuity_service.go` 的 `SaveSummary`：带 `summary_ulid` 为更新；
  不带时先按 `GetSummaryByChapter(project, chapter)` 查同章节已有摘要，存在则更新，不存在才新建。

**验收点**

- 同一章节重复保存摘要只保留一条记录，不产生重复行。
- 非法摘要状态被拒。
- 按章节过滤只返回该章节的摘要。

---

## 6. 知识库

### 6.1 上传与索引

**用户能力**

- 上传 txt / md / docx / pdf 资料到项目。
- 查看文档列表与索引状态。
- 对失败或已索引文档重新索引。
- 删除文档。

**实现**

- `api/knowledge_api.go` 的 `Upload`：`multipart/form-data`，文件字段名 `file`，
  表单字段 `project_ulid` 与 `doc_name`；其余表单字段交给 `c.ShouldBind` 绑定。
- `service/knowledge_service.go` 的 `docTypeOfFileName`：只接受 `txt` / `md` / `docx` / `pdf`，
  其他返回 `ERR_INVALID_PARAM`；上传大小超过 `UploadMaxMB`（默认 20MB）被拒。
- `service/knowledge_service.go` 的 `Upload`：原文写入 MinIO（对象键为 `doc_ulid`），
  文档行落库且 `doc_status_code='pending'`；落库失败时回滚删除 MinIO 对象；
  投递任务失败时把文档状态置为 `failed`。
- `service/knowledge_service.go` 的 `enqueueIndexJob`：先写 `jobs` 留痕（`queued`），
  再投递 RabbitMQ `nw_doc_index` 队列；投递失败把任务置 `failed`。
- `service/knowledge_worker.go` 的 `Run`：消费队列；`handleIndexJob` 内部最多重试 3 次
  （`maxIndexRetryCount`），仍失败则把任务置 `failed`、文档置 `failed` 并 ack（毒消息不重投）。
- `service/knowledge_worker.go` 的 `indexDocument`：取原文 → `extractDocumentText` 提取纯文本
  （txt/md 直读并剥 UTF-8 BOM，docx 解 zip 取 `word/document.xml` 去标签还原实体，
  pdf 用 `ledongthuc/pdf` 取纯文本）→ `chunkText` 滑窗切片 → `contentHashOf`（sha256 hex）
  去重并复用同哈希的旧切片 ULID → 事务内先删旧切片再写新切片 → 同步 Manticore →
  文档状态置 `indexed` 并写 `chunk_count` 与 `total_token_count`。
- `service/knowledge_worker.go` 的 `chunkText`：窗口 `chunkSize` 字符、步进 `chunkSize - overlap`；
  窗口内存在换行或句末标点且位置超过窗口一半时优先在边界切分；
  `overlap >= chunkSize` 收敛为 `chunkSize-1`，负 overlap 归零。
- `service/knowledge_manticore.go` 的 `ensureManticoreSchema`：建表 DDL 为
  `(chunk_ulid string, doc_ulid string, project_ulid string, chunk_text text)
  charset_table='non_cjk,0..9,A..Z->a..z,a..z' ngram_len='1' ngram_chars='cjk'`；
  `SHOW CREATE TABLE` 结果不含 `ngram_len` 时 `DROP` 后重建并返回 `true` 触发回灌。
- `service/knowledge_manticore.go` 的 `rebuildManticoreIndex`：表结构升级后从 PG 切片一次性回灌。
- `service/knowledge_service.go` 的 `Reindex`：只允许 `indexed` / `failed` 状态的文档重投，
  投递 `doc_reindex` 任务并把状态置回 `pending`。
- `service/knowledge_service.go` 的 `Delete`：先删 MinIO 对象，再删 Manticore 行，
  最后事务内删切片与文档行；Manticore 清理失败只告警（降级）。

**验收点**

- 非 txt/md/docx/pdf 文件上传被拒。
- 超出大小上限的文件上传被拒。
- 上传后文档状态为 `pending`，索引完成后变为 `indexed` 并带出切片数与 token 数。
- 文档正文重复上传时，同哈希切片的 ULID 保持不变。
- 索引失败时文档状态为 `failed` 且带出错误信息；重投后重新进入索引流程。
- 删除文档后 MinIO 对象、Manticore 行、切片行与文档行都不存在。

### 6.2 检索

**用户能力**

- 在项目知识库中按关键词检索资料片段。
- 选择 keyword 或 hybrid 检索模式。
- 查看检索过程（查询词、候选规模、入选切片、是否重排）。

**实现**

- `api/knowledge_api.go` 的 `Search` 返回结果列表与检索过程。
- `service/knowledge_service.go` 的 `normalizeSearchMode`：仅 `hybrid` 走双通道，其余（含空值）按 `keyword`。
- `service/knowledge_service.go` 的 `extractQueryTerms`：CJK 连续串按 2-gram 切分，
  拉丁串按字母数字切分并去重去标点；无词时回退原查询串。
- `service/knowledge_service.go` 的 `searchKeywordChannel`：走 Manticore MATCH，命中后回表取切片正文，
  并写入 BM25 相关度与来源通道标记。
- `service/knowledge_service.go` 的 `fuseSearchResults`：RRF 融合，
  得分 `Σ 1/(60+rank)`（`rrfRankConstant=60`）；同一切片跨通道命中得分累加且标记为 `hybrid`；
  得分降序后截断到 topK。
- `service/knowledge_service.go` 的 `maybeRerank`：用当前对话模型对候选切片重排，
  8 秒超时；解析失败或模型不可用时 fail-open 保留原顺序，并在检索过程里写明降级原因。
- `service/knowledge_service.go` 的 `SearchWithTrace`：查询词为空返回 `ERR_INVALID_PARAM`；
  `top_k` 缺省取 `RAGTopK`（默认 6）；hybrid 模式下某通道失败时降级到另一通道并记 warn。
- `service/knowledge_service.go` 的 `mustOwnDoc`：文档 `project_ulid` 为空或为 `"0"`（全局库）时放行，
  否则校验项目归属。

**验收点**

- 检索结果带出切片正文、得分与来源通道（`term` / `phrase` / `hybrid`）。
- 同一片段被两个通道命中时排在单通道命中之前，且来源标记为 `hybrid`。
- 未配置模型供应商时检索仍可用，检索过程写明未重排。
- 查询词为空时返回 `ERR_INVALID_PARAM`。
- 他人项目的文档不可被检索到。

### 6.3 切片查看与原文预览

**用户能力**

- 分页查看文档切片。
- 预览文档原文（txt / md / docx / pdf）。

**实现**

- `api/knowledge_api.go` 的 `ChunkList`：`GET /knowledge/chunk/list` 分页返回切片序号与文本。
- `api/knowledge_api.go` 的 `Content`：`GET /knowledge/content` 从对象存储取回原文，
  按类型设置 `Content-Type`（txt / md / docx / pdf），并带 `Content-Disposition: inline` 文件名。
- `service/knowledge_service.go` 的 `GetContent`：读取上限 25 MiB（`contentMaxBytes`）。
- `frontend/src/views/knowledge/ChunkListDialog.vue`：分页展示切片。
- `frontend/src/views/knowledge/KnowledgePreviewDrawer.vue`：`fetchKnowledgeContent` 取 Blob，
  txt/md 走文本渲染，pdf 走 `URL.createObjectURL` 内嵌预览，docx 走 `docx-preview`。

**验收点**

- 切片弹窗按序号与正文分页展示。
- 预览抽屉能渲染 txt/md 文本；pdf 可在浏览器内查看。
- 越权访问他人文档原文返回 `ERR_FORBIDDEN`。

---

## 7. AI 工作台

### 7.1 会话管理

**用户能力**

- 在项目下新建 AI 会话。
- 查看会话列表、切换会话、删除会话。
- 查看会话历史消息。

**实现**

- `api/chat_api.go` 的 `SessionCreate` / `SessionList` / `SessionDelete` / `History`。
- `service/chat_service.go` 的 `SessionCreate`：校验项目归属；会话名可显式指定。
- `service/chat_service.go` 的 `SessionList`：`chatRepo.ListSessionsByProject`，返回非 nil 列表。
- `service/chat_service.go` 的 `SessionDelete`：`mustOwnSession` 校验归属后
  `chatRepo.DeleteSessionWithMessages` 级联删消息。
- `service/chat_service.go` 的 `History`：按 `created_at` 升序返回全量消息，
  包在 `PageDTO` 中，`total` 为消息条数。
- `service/chat_service.go` 的 `resolveSession`：`session_ulid` 为空时自动建会话；
  非空时校验归属且会话项目必须与请求项目一致，否则 `ERR_INVALID_PARAM`。
- `frontend/src/views/workspace/AiPanel.vue` 的 `loadSessions` / `handleCreateSession` /
  `handleDeleteSession` / `selectSession`。

**验收点**

- 会话名在首条消息发出后按正文前 20 字自动回填（`sessionNameRunes`）。
- 首次发送消息若未指定会话，服务端自动创建会话并通过 SSE 事件回传 `session_ulid`。
- 删除会话后其消息一并删除；他人会话返回 `ERR_FORBIDDEN`。
- 历史消息按时间正序返回。

### 7.2 发送与流式生成

**用户能力**

- 输入正文发起 AI 生成，实时看到增量输出。
- 中途停止生成。
- 生成失败后重试。
- 在生成完成后查看 AI 依据并把回复插入章节正文。

**实现**

- `api/chat_api.go` 的 `Send`：`POST /chat/send` 返回 `text/event-stream`；
  流开始前的失败输出 JSON 错误包络，流开始后的失败只能随流下发。
- `api/chat_api.go` 的 `sseChatWriter`：事件行 `data: {json}`，心跳为 `: ping` 注释行；
  `begin()` 标记响应头已提交。
- `service/chat_service.go` 的 `Send` 主链路：技能解析 → 会话解析 → 读取会话消息 →
  显式引用校验 → 历史选取 → 上下文素材加载 → 上下文装配 → 预算裁剪 → 落库用户消息 →
  构造消息序列 → 取供应商规格 → 流式生成 → 落库助手消息与 AI 字数 → 发 `done` 事件。
- `service/chat_service.go` 的 `chatEventWriter.emit`：`seq` 从 1 单调递增；
  `reference_list` 缺省为空数组。
- `service/chat_service.go` 的 `streamChat`：15 秒无输出发心跳（`chatPingInterval`）；
  客户端断开即中止生成且不落库助手消息；流结束或失败时通过
  `aiClient.RecordStreamUsage` 上报用量。
- `service/chat_service.go` 的 `resolveSkill`：请求带 `skill_code` 时按编码取启用中的技能；
  否则解析正文首 token 的技能命令；未命中技能则全文作为用户正文。
- `service/chat_service.go` 的 `loadContextInput`：装配引用章节（首项为当前章）、
  当前章前后章梗概、@引用文档全文、项目角色卡、项目设定条目；
  RAG 召回失败时降级为空上下文，不阻断生成；引用章节不属于本项目或文档越权直接报错。
- `service/chat_context.go` 的 `buildContextBlocks`：产出上下文块，
  优先级为 skill(100) > current_chapter(90) > skill_reference(85) > ref_chapter / ref_doc(80) >
  rag(70) > chapter_summary(60) > character / lore(50) > history(40)；技能命中时渲染
  `{{user_input}}` / `{{chapter_content}}` / `{{context}}` 并把渲染结果作为最高优先级块。
- `ai/prompt.go` 的 `AssembleContext`：按优先级降序装配，超出字符预算（`ContextBudgetChars=12000`）
  的块整块丢弃并返回被丢弃的块键；空块直接跳过；同优先级保持输入顺序。
- `ai/prompt.go` 的 `RenderPrompt`：命中占位符替换为值，缺失占位符替换为空串。
- `ai/prompt.go` 的 `SystemPrompt`：生成系统提示词，并注入项目文风约束。
- `service/chat_context.go` 的 `buildChatMessages`：消息序列为
  system → 【上下文】装配文本（为空则省略）→ 历史消息（按 role 映射）→ 最终指令。
- `service/chat_context.go` 的 `selectHistoryMessages`：取最近 n 条为常规历史（旧→新）；
  显式引用消息必注入、去重后按时间旧→新追加到尾部；n ≤ 0 时不注入常规历史。
- `service/chat_context.go` 的 `validateRefMessages`：显式引用消息必须属于当前会话，去重保序。
- `service/chat_service.go` 的 `estimateTokenCount`：按 rune 数 × 0.75 估算 token。
- `frontend/src/composables/useSSE.ts` 的 `streamSsePost`：用 `fetch` + `ReadableStream` 读流，
  `parseSseBuffer` 逐块解析；`useSSE` 提供 `startStream` / `stopStream`，
  重复调用会先中止上一条流。
- `frontend/src/views/workspace/AiPanel.vue` 的 `runStream`：`chunk` 事件增量追加，
  `done` 事件覆盖为完整回复并挂 RAG 依据，`error` 事件落错误气泡可重试。
- `frontend/src/views/workspace/ChapterEditor.vue` 的 `insertTextAtCursor`：在光标处插入文本；
  `frontend/src/views/workspace/index.vue` 的 `handleInsertToChapter`：把 AI 回复插入当前章节。

**验收点**

- SSE 事件的 `seq` 从 1 起单调递增，事件类型为 `chunk` / `done` / `error`。
- 流开始前的失败（如供应商未配置）返回 JSON 错误包络，而不是 SSE 事件。
- 点击停止后不再追加增量输出，助手消息不落库（用户消息仍保留）。
- 生成完成后助手消息与用户消息都出现在会话历史中。
- `done` 事件的 `reference_list` 带回本次 RAG 召回依据。
- 插入章节后正文包含 AI 回复内容，并走同一自动保存通道。

### 7.3 技能（Skill）

**用户能力**

- 在输入框用 `/` 触发技能补全，选中后以徽标形式附带技能编码提交。
- 以技能发起创作任务（续写、大纲、润色、扩写、缩写、梗概、审稿、取名、设定问答）。
- 在「技能管理」中维护全局技能与项目技能：编码、名称、分类、提示词模板、变量声明、
  参考资料、启用开关与排序。
- 同一编码的项目技能覆盖全局技能，团队可按项目定制而不污染公共资产。
- 技能可声明变量（必填/缺省值），对话前置解析返回缺失项提示。

**实现**

- `ai/skills.go` 的 `builtinSkillList`：9 个内置技能（`continue` / `outline` / `polish` /
  `expand` / `condense` / `summary` / `review` / `name` / `setting`），含分类、说明与模板；
  `BuiltinSkill` 与持久化结构解耦，落库由 service 完成。
- `service/skill_service.go` 的 `SeedDefaults`：按 `(global, "", skill_code)` 查缺补插，
  已存在不覆盖（保留作者对提示词的定制），`builtin_flag=true`。
- `service/skill_service.go` 的 `Resolve`：显式 `skill_code` 优先，否则解析正文首 token 的 `/code`；
  `resolveByCode` 先查项目技能（启用）再回退全局技能；命中后返回 `remainder_text`、
  `variable_map`、`missing_list` 与命中的 `ref_text_list`。
- `service/skill_service.go` 的 `validateSkillTemplate`：模板占位符只能是
  `user_input` / `chapter_content` / `context` / `skill_code` 或已声明变量，
  否则保存返回 `ERR_INVALID_PARAM`。
- `service/skill_reference_service.go` 的 `matchReferences`：参考资料关键词为空总是注入，
  否则任一关键词出现在正文即注入；单技能累计注入上限 4000 字符。
- `service/chat_service.go` 的 `resolveSkill`：把 `SkillResolveDTO` 转成对话链路输入
  （模板 + 变量 + 参考资料），模板中的 `{{context}}` / `{{chapter_content}}` 仍由上下文装配填充。
- `service/chat_context.go` 的 `buildContextBlocks`：技能参考资料作为 `skill_reference` 块
  （优先级 85，低于当前章节、高于显式引用）。
- `api/router.go` 的 `registerSkillRoutes`：读写分离，写操作收口 `skill:manage`。
- 前端 `views/skills/index.vue` + `SkillReferencePanel.vue`：表格 + 抽屉 + 参考资料子面板；
  `ChatComposer.vue` 改为按项目拉取技能（含全局技能）。

**验收点**

- 正文首 token 命中启用技能时命令被剥离，剩余正文进入 `{{user_input}}`。
- 未知命令、已停用技能、命令不在首 token 三种情况正文原样保留（不进入技能分支）。
- 停用某技能后 `/code` 不再命中；项目技能停用后回退同编码全局技能。
- 同编码项目技能存在时，对话与列表都以项目技能为准（列表里全局项标 `overridden_flag`）。
- 内置技能不可删除、不可改编码与作用域，可改模板/变量/参考资料。
- 必填变量缺失时 `resolve` 返回 `missing_list`，对话链路直接拒绝渲染并提示补齐。
- 模板里写了未声明的占位符时保存失败。

### 7.4 引用与插入

**用户能力**

- 用 `@` 选择知识库文档或本项目章节作为引用素材。
- 在引用选择器中按名称过滤。
- 移除已选引用。

**实现**

- `frontend/src/views/workspace/ChatComposer.vue` 的 `AT_TOKEN_REGEXP` 与 `refPicker`：
  候选来源为 `listKnowledgeDocuments` 的文档列表与工作台扁平章节列表，两类二选一切换。
- `service/chat_service.go` 的 `loadContextInput`：引用章节必须属于本项目，
  否则 `ERR_INVALID_PARAM`；引用文档必须属于本项目或为全局库（`project_ulid == "0"`），
  否则 `ERR_FORBIDDEN`。
- `service/chat_context.go` 的 `loadDocFullText`：按 `chunk_index` 升序分页（每页 200 条）
  拼接文档全部切片文本。
- `service/chat_context.go` 的 `buildContextBlocks`：当前章块优先级 90，
  引用章与引用文档块优先级 80。

**验收点**

- 引用当前章节时上下文块优先级为 90；引用其他章节或文档为 80。
- 引用他人文档返回 `ERR_FORBIDDEN`；引用不存在的章节返回 `ERR_INVALID_PARAM`。
- 移除引用后请求体不再携带对应 ULID。
- 引用文档的全文由该文档全部切片拼接而成。

---

## 8. 模型生态

### 8.1 供应商管理

**用户能力**

- 新建/编辑模型供应商（名称、协议、网关地址、API Key、对话模型、温度、输出上限）。
- 设为默认供应商。
- 连通性测试。
- 删除供应商。

**实现**

- `api/model_provider_api.go` 的 `List` / `Save` / `Delete` / `Test`。
- `service/model_provider_service.go` 的 `SeedDefault`：`model_providers` 表为空时按配置播种一条默认供应商
  （`default_flag=true`、`enabled_flag=true`、测试状态 `untested`）；表非空时不播种。
- `service/model_provider_service.go` 的 `Save`：首次创建必须给 API Key（否则 `ERR_INVALID_PARAM`）；
  密钥经 `pkg.SealSecret` 封存为密文；`default_flag` 为真时先 `providerRepo.ClearDefault` 保证全局唯一；
  Anthropic 协议下自动去掉地址末尾的 `/v1`（SDK 自行拼接）。
- `service/model_provider_service.go` 的 `Delete`：默认供应商拒删，返回 `ERR_BIZ_FAILED`。
- `service/model_provider_service.go` 的 `Test`：`gateway.Probe` 只验证对话通道，
  结果（状态、延迟、说明）回写供应商记录 `last_test_status_code` / `last_test_message` / `last_test_at_ms`。
- `service/model_provider_service.go` 的 `toVO`：API Key 只回显掩码
  （`maskSecret`：保留首 4 位与末 3 位，长度 ≤ 10 时全掩码）；无法解密时提示重新填写。
- `service/model_provider_service.go` 的 `specOf`：`pkg.OpenSecret` 解封密钥，
  解封失败返回「供应商密钥无法解密，请重新填写 API Key」。
- `api/router.go` 的 `registerModelRoutes`：供应商写操作收口到 `system:model:manage`，
  列表与能力探测对所有登录用户开放。

**验收点**

- 列表与详情中不出现明文 API Key，只出现掩码。
- 首次创建不填 API Key 返回 `ERR_INVALID_PARAM`。
- 默认供应商全局唯一，删除默认供应商被拒。
- 连通性测试成功与失败都回写状态，界面能看到最近一次测试结论与时间。
- 更换 `NW_JWT_SECRET` 后旧密文无法解封，页面提示重新填写 API Key。

### 8.2 模型选择与能力

**用户能力**

- 在 AI 面板选择本次对话使用的供应商。
- 查看当前 AI 能力结论与提示。

**实现**

- `api/model_provider_api.go` 的 `Capability` 返回 `ModelCapabilityVO`；
  `ChatOptions` 返回能力视图与供应商列表。
- `service/model_provider_service.go` 的 `Capability`：存在默认供应商则 `chat_ready=true`
  并带出其 ULID、名称与对话模型名；未配置时给出提示文案（知识库仍可关键词检索）。
- `service/model_provider_service.go` 的 `ChatSpec` / `ChatSpecOf`：
  未指定供应商时取默认；指定供应商不存在或已停用时返回 `ERR_INVALID_PARAM`。
- `service/chat_service.go` 的 `Send`：`providerSpec` 解析失败发生在流开始前，
  由 API 层输出 JSON 错误包络。
- `frontend/src/views/workspace/AiPanel.vue` 的 `loadModelOptions`：模型下拉只列启用中的供应商，
  默认选中默认供应商；加载失败时降级为空列表，面板照常可用。

**验收点**

- 未配置供应商时 `chat_ready=false`，AI 生成返回 `ERR_AI_PROVIDER`。
- 选择停用中的供应商发起对话返回 `ERR_INVALID_PARAM`。
- AI 面板的模型下拉只显示启用中的供应商。
- 能力提示文案与实际供应商配置一致。

---

## 9. 可视化与运维

### 9.1 数据总览

**用户能力**

- 查看项目数、累计字数、今日新增、AI 累计字数。
- 查看近 30 天字数趋势。
- 在项目列表中新建、编辑、删除项目。

**实现**

- `api/stats_api.go` 的 `Overview`：`GET /stats/overview`，`project_ulid` 为空为全局统计。
- `service/stats_service.go` 的 `Overview`：`CountProjects` / `SumTotalWordCount` /
  `SumWordDeltaByDate`（今日） / `SumAIWordTotal`。
- `service/stats_service.go` 的 `trendList`：覆盖近 30 天（含今日），缺日期补零。
- `frontend/src/views/dashboard/index.vue`：`loadStatsOverview` 拉数据，
  `renderTrendChart` 用 ECharts 渲染趋势图，`loadProjectList` 渲染项目表。

**验收点**

- 全局视图与项目视图的统计口径不同：指定项目时只统计该项目。
- 趋势数组固定 30 个点，缺失日期补零而非缺项。
- 仪表盘项目表可新建、编辑、删除项目。

### 9.2 用量分析

**用户能力**

- 查看区间内每日调用数、token 用量与平均延迟。
- 查看按供应商、按模型的分组用量。
- 切换统计区间。

**实现**

- `api/model_usage_api.go` 的 `Overview`：`GET /model/usage/overview`，权限码 `system:usage:read`。
- `service/model_usage_service.go` 的 `RecordUsage`：内存缓冲按
  「日期 | 供应商 | 模型 | 调用类型」聚合计数、token 与延迟累加。
- `service/model_usage_service.go` 的 `flushLoop`：每 5 秒落库一次，进程退出时做最后一次落库；
  落库失败只告警并丢弃该批缓冲。
- `service/model_usage_service.go` 的 `Overview`：查询前先 `Flush`，避免刚发生的调用不可见；
  缺日补零保证图表连续；平均延迟按每日调用数加权（`weightedAvgLatency`）。
- `service/model_usage_service.go` 的 `resolveUsageRange`：缺省近 14 天；
  非法日期回退为合法日期；起止倒置自动纠正。
- `frontend/src/views/admin/usage.vue`：`loadOverview` 拉数据，`renderTrendChart` 渲染趋势图，
  `handleRangeChange` 切换区间。

**验收点**

- 用量趋势的日期连续，缺失日显示为 0。
- 区间参数非法或倒置时界面仍能展示合法区间数据。
- 发起一次模型调用后，用量页的调用数与 token 数随之增长。
- 无权限用户访问用量接口返回 `ERR_FORBIDDEN`。

### 9.3 运行状态

**用户能力**

- 查看 PostgreSQL、Redis、RabbitMQ、MinIO、Manticore 的健康状态与延迟。
- 查看模型供应商（对话）是否可用。
- 查看服务启动时间、运行时长、Go 版本与构建版本。

**实现**

- `api/system_status_api.go` 的 `Status`：`GET /system/status`，权限码 `system:status:read`。
- `service/system_status_service.go` 的 `Status`：五个依赖探针并发执行，
  单项失败不影响其余结果。
- `service/system_status_service.go` 的 `probeDependency`：探针未配置记为 `degraded`（未配置）；
  必需依赖（PostgreSQL）失败记为 `down`，其余失败记为 `degraded`；延迟取毫秒。
- `service/system_status_service.go` 的 `Status` 末尾追加「模型供应商（对话）」一项，
  状态取自 `providerSvc.Capability` 的 `chat_ready`。
- `cmd/server/main.go`：各探针实现为 PG `PingContext`、Redis `Ping`、
  RabbitMQ 连接状态与队列声明、MinIO 桶存在性、Manticore `Ping`。
- `frontend/src/views/admin/status.vue`：`loadStatus` 拉数据并渲染依赖状态列表。

**验收点**

- 停掉某个中间件后对应依赖显示为不可用，其余依赖状态不受影响。
- 未配置某依赖时显示「未配置」而不是「正常」。
- 未配置模型供应商时，模型项显示不可用且带出提示文案。
- 页面展示服务运行时长与构建版本。

---

## 10. 可扩展点

系统的扩展能力只有一条路径：**技能**（见 7.3）。技能是带作用域的提示词资产，
模板、变量声明与参考资料全部落库，在「技能管理」页在线维护，无需改代码或重新部署。

其余可能的扩展点及其现状：

| 扩展点 | 现状 |
| --- | --- |
| 提示词与写作范式 | 技能资产承载：全局技能 + 项目技能（同编码项目覆盖全局） |
| 模型厂商接入 | 模型供应商页维护，OpenAI 兼容与 Anthropic 双协议，不改代码 |
| 导航与页面 | 菜单表登记，分组 / 页面 / 动作三级，权限码与菜单同一行登记 |
| 权限粒度 | 权限目录由菜单汇总，角色按目录勾选，无需改代码 |
| 检索召回 | 关键词 + 短语双通道，RRF 融合 + 模型重排，模型不可用时自动降级 |

系统不设「流程编排」与「微前端」类扩展位：这两类扩展要么把控制流拆到浏览器本地
（不进业务库、无法审计与协作），要么把一个写作工具拆成多应用（部署与鉴权成本高于收益）。

---

## 11. 前端工作台装配

**用户能力**

- 登录后进入仪表盘；在项目间切换；进入写作台与各项目子页面。

**实现**

- `frontend/src/router/index.ts`：路由表覆盖仪表盘、系统设置、技能管理、模型供应商，
  用户/角色/菜单/权限目录/审计/用量/运行状态，以及项目内的工作台、知识库、角色、笔记、大纲、
  连续性、世界观页面；每条业务路由带 `meta.perm_code`。
- `frontend/src/router/index.ts` 的 `beforeEach`：无 token 访问非公开页跳 `/login`；
  已登录访问 `/login` 重定向首页；首次进入业务页先 `loadNav()`，`meta.perm_code` 校验失败回仪表盘。
- `frontend/src/stores/menu.ts`：导航树与权限码的唯一来源，`loadNav` 幂等（并发只发一次请求）。
- `frontend/src/api/http.ts`：axios 实例注入 `Authorization: Bearer <token>`；
  响应拦截器解包 `data`、统一错误提示；遇 `ERR_UNAUTHORIZED` 清 token 并跳登录页。
- `frontend/src/views/workspace/index.vue`：三栏工作台（左目录树 / 中章节编辑器 / 右 AI 面板），
  切换章节前先 `flushSave` 落盘当前内容。
- `frontend/src/views/workspace/ChapterEditor.vue`：CodeMirror 编辑器 + 防抖自动保存。

**验收点**

- 未登录访问任何业务页跳转到登录页。
- 令牌失效时前端自动清 token 并回到登录页。
- 工作台切换章节时上一章的未保存内容先落盘，不丢内容。
- 同一错误只弹一次提示。

## 12. 2026-11 体验与功能补齐（当前实现）

### 12.1 仪表盘

- 欢迎标题按时间问候，展示当前登录用户；右侧提供「新建项目」「继续写作」。
- 四个统计磁贴：项目数、总字数、今日新增、AI 生成；数值与 `/stats/overview` 对齐。
- 趋势卡展示近 30 天人工与 AI 双系列图；无数据时显示空态说明。
- 今日目标卡读取 `daily_word_goal`，展示进度、剩余字数与三个快捷入口。
- 项目区支持状态页签、书名搜索、分页；项目卡片显示封面色、题材、状态、简介、字数、章节数、相对更新时间。
- 新建/编辑使用抽屉；删除二次确认并刷新统计。

### 12.2 写作工作台

- 顶栏：返回仪表盘、项目封面与状态、项目/本章字数、专注模式、目录开关、AI 面板开关、快照。
- 目录树：章节搜索、卷折叠、章节字数/状态、卷与章节操作菜单；空目录提供「新建卷」CTA。
- 章节编辑器：章节名、状态、字数、保存状态、AI 续写/润色/梗概/快照；梗概区可折叠；自动保存 2s。
- AI 面板：模型选择、会话切换/新建/删除、上下文开关（默认附带当前章节）、空态快捷指令、流式消息、AI 依据、插入到章节、失败重试与停止生成。
- AI 发送会把当前章节自动并入引用章节清单，后端按「当前章节 → 引用文档 → RAG 召回 → 相邻摘要 → 角色卡 → 设定」装配上下文。

### 12.3 知识库

- 页签：文档库 / 检索测试。
- 概览：文档总数、已索引、处理中、切片总数；索引中自动每 3s 刷新。
- 文档表格：类型图标、状态徽标、切片数、Token 数、上传时间；预览、切片、重索引、删除。
- 上传抽屉：类型/大小校验、拖拽上传、上传进度、索引说明。
- 检索测试：关键词/词法混合、召回条数、示例查询、检索追踪（查询词、候选数、重排状态）、结果高亮与来源通道。
- 不接入向量模型：索引由 Manticore 关键词表承担，PG 保存切片与哈希；检索失败可降级。

### 12.4 连续性与创作规划

- 连续性看板：事实/伏笔/摘要三页签；章节筛选与表单使用章节选择器；表格显示章节名；事实支持确认/取代/过期，伏笔与摘要支持编辑。
- 创作规划：卷/篇章/情节块/章节树 + 场景卡；场景卡出场角色改为角色多选并显示姓名；节点与场景的增删改均有确认与空态。

### 12.5 创作笔记与系统设置

- 笔记列表支持标题/正文搜索、正文片段与相对时间；富文本/Markdown 双模式，2s 自动保存，初始状态为「已保存」。
- 系统设置分组、范围校验、逐项/批量保存；关于模型与检索的说明固定在页底。

### 12.6 管理端

- 用户、角色、菜单、权限目录、审计、登录日志、任务中心统一页面标题区与筛选条。
- 任务中心：排队/执行/成功/失败四个磁贴；文档名优先展示，文档已删除时明确标注；失败原因悬停查看完整文本；失败任务可重试。
