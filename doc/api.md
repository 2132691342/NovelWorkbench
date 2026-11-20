# 接口契约

本文描述 NovelWorkbench 对外暴露的全部 HTTP 接口：通用约定、响应包络、业务码映射、中间件链、权限码、按模块分组的接口总表与特殊端点。

## 1. 通用约定

| 项 | 约定 | 代码位置 |
| --- | --- | --- |
| 业务基础路径 | `/api/v1` | `router.go` 的 `v1 := router.Group("/api/v1")` |
| 探针路径 | `/health`、`/ready`，注册在 `/api/v1` 之外 | `router.go` 的 `NewRouter` |
| 接口总数 | `/api/v1` 下 109 条，另有 2 条探针，共 111 条 | `router.go` |
| 允许方法 | 仅 `GET` / `POST`；`OPTIONS` 只用于 CORS 预检 | `pkg.MethodWhitelistMiddleware` |
| 请求编码 | `POST` 默认 `application/json`（`ShouldBindJSON`）；`/knowledge/upload` 为 `multipart/form-data` | 各 `*_api.go` |
| 查询参数 | `GET` 用 query（`ShouldBindQuery`），路径变量用 `c.Param` | 各 `*_api.go` |
| 鉴权头 | `Authorization: Bearer <access_token>` | `pkg.AuthMiddleware` |
| 链路头 | 请求 `X-Request-ID` 透传；缺省生成 `REQ_` 前缀 ULID，响应头回写同值 | `pkg.RequestIDMiddleware` |
| 业务码 | 字符串 `SUCCESS` 或 `ERR_*` | `domain.RespCode*` |
| ID 形态 | 业务前缀 + ULID（`PRJ_` / `CHP_` / `DOC_` / `MDL_` …） | `domain/common.go` 的 `IDPrefix*` |
| 时间 | 视图对象统一给毫秒时间戳，字段后缀 `_at_ms`；日期参数为 `YYYY-MM-DD` | 各 VO |
| 数据归属 | 项目域资源一律经 `mustOwnProject` / `mustOwnDoc` 等校验归属，越权返回 `ERR_FORBIDDEN` | `service/project_service.go` |

CORS 放行 `Origin`、`Content-Type`、`Authorization`、`X-Request-ID`，暴露 `X-Request-ID`，预检缓存 12 小时（`router.go` 的 `cors.Config`）。

## 2. 响应包络

成功（`pkg.Ok`，HTTP 恒为 200）：

```json
{
  "resp_code": "SUCCESS",
  "resp_message": "ok",
  "trace_id": "REQ_01HZX...",
  "data": { "total": 0, "page_num": 1, "page_size": 20, "items": [] }
}
```

失败（`pkg.Fail` / `pkg.FailErr`）：

```json
{
  "resp_code": "ERR_INVALID_PARAM",
  "resp_message": "参数校验失败: Key: 'ProjectCreateREQ.ProjectName' ...",
  "trace_id": "REQ_01HZX...",
  "data": null
}
```

包络结构为 `domain.BaseRESP`（`resp_code` / `resp_message` / `trace_id`）嵌套 `domain.DataRESP[T].data`。
`pkg.FailErr` 对 `pkg.BizError` 保留其业务码与文案，其他 `error` 记日志后统一输出 `ERR_INTERNAL` 与「服务内部错误，请稍后重试」。
绑定失败由各处理器的 `bindErr` 转成 `ERR_INVALID_PARAM`，消息前缀固定为「参数校验失败: 」。
无返回体的写接口 `data` 为 `null`。

## 3. 业务码 → HTTP 状态

映射唯一入口为 `pkg.HTTPStatusForCode`（`backend/internal/pkg/response.go`），未知码归 500。

| 业务码 | HTTP | 语义 | 典型触发 |
| --- | --- | --- | --- |
| `SUCCESS` | 200 | 成功 | 正常返回 |
| `ERR_INVALID_PARAM` | 400 | 参数校验失败 | `binding` 校验失败、非 GET/POST 请求 |
| `ERR_UNAUTHORIZED` | 401 | 未登录或登录失效 | 缺令牌、JWT 解析失败、令牌已吊销、账号禁用 |
| `ERR_FORBIDDEN` | 403 | 无权限 | 缺少权限码、非资源归属人、模型未授权 |
| `ERR_NOT_FOUND` | 404 | 资源不存在 | ULID 查不到 |
| `ERR_CONFLICT` | 409 | 资源冲突 | 重名、邮箱重复、重复提交 |
| `ERR_BIZ_FAILED` | 400 | 通用业务失败 | 删除默认模型、删除含模型的供应商、内置技能删除 |
| `ERR_AI_PROVIDER` | 502 | AI 调用失败 | 无可用模型、供应商停用、上游报错 |
| `ERR_STORAGE` | 503 | 基础设施异常 | MinIO / Manticore / RabbitMQ 失败、`/ready` 依赖未就绪 |
| `ERR_INTERNAL` | 500 | 服务内部错误 | 未归类 error、panic 恢复、权限判定报错 |

`ERR_BIZ_FAILED` 与 `ERR_INVALID_PARAM` 同为 400，客户端须以 `resp_code` 而非 HTTP 状态区分。

## 4. 中间件链与白名单

`pkg.NewRouter` 的装配顺序（`backend/internal/api/router.go` 的 `NewRouter`）：

```mermaid
flowchart LR
  A[Recovery] --> B[RequestID] --> C[Logger] --> D[CORS] --> E[MethodWhitelist]
  E --> F["探针 /health /ready"]
  E --> G[Auth] --> H[Audit] --> I[RequirePerm] --> J[Handler]
```

| 序 | 中间件 | 作用 | 失败输出 |
| --- | --- | --- | --- |
| 1 | `pkg.RecoveryMiddleware` | 捕获 panic，记录堆栈并中止链 | `ERR_INTERNAL` / 500 |
| 2 | `pkg.RequestIDMiddleware` | 分配或透传 `X-Request-ID`，写入上下文键 `request_id` | 无 |
| 3 | `pkg.LoggerMiddleware` | 出口记录 method / path / status / latency_ms | 无 |
| 4 | `cors.New(...)` | 处理预检，预检直接返回 | 无 |
| 5 | `pkg.MethodWhitelistMiddleware` | 仅放行 GET / POST | `ERR_INVALID_PARAM` / 400 |
| 6 | `pkg.AuthMiddleware` | 仅挂 `/api/v1` 组；校验令牌、吊销名单、用户状态，注入上下文键 `user_ulid` | `ERR_UNAUTHORIZED` / 401 |
| 7 | `pkg.AuditMiddleware` | 仅挂 `/api/v1` 组；记录 POST 审计 | 不阻断业务 |
| 8 | `pkg.RequirePermMiddleware` | 分组级权限门禁 | `ERR_FORBIDDEN` / 403，判定报错 `ERR_INTERNAL` / 500 |

`/health`、`/ready` 注册在 `/api/v1` 组之前，只经过 1–5，不鉴权、不审计。

鉴权中间件内部顺序：跳过名单直接放行 → 取 `Authorization`（缺省 401「未登录」）→ `JWTSigner.ParseToken`（失败 401「登录已失效」）→ `TokenStore.IsRevoked` 命中 401「登录已失效」→ `UserStateChecker` 判定用户非禁用（禁用 401「账号不存在或已被禁用」，判定报错 500）。

### 4.1 免鉴权白名单

`backend/cmd/server/main.go` 传入 `pkg.AuthMiddleware` 的 `skipPaths`：

| 路径 | 原因 |
| --- | --- |
| `/api/v1/auth/login` | 登录请求本身无令牌 |

其余 `/api/v1` 路径一律要求登录；系统不开放注册，账号只能由管理员经 `/admin/user/create` 创建。

### 4.2 审计跳过名单

`backend/cmd/server/main.go` 传入 `pkg.AuditMiddleware` 的 `skipPaths`：

| 路径 | 原因 |
| --- | --- |
| `/api/v1/auth/login` | 尚无登录用户可归属 |
| `/api/v1/auth/logout` | 不落审计 |
| `/api/v1/chat/send` | 高频流式调用，不落审计 |

审计规则（`pkg/access_middleware.go`）：只记录 `POST`；`action_code` 取路径去掉 `/api/v1/` 前缀；`detail_text` 记 `status=<HTTP 状态>`；仅 `/api/v1/admin/` 前缀的请求会读 body 提取目标，键白名单为 `user_ulid`→user、`role_ulid`→role、`project_ulid`→project、`chapter_ulid`→chapter、`doc_ulid`→document，body 采样上限 64 KiB 且读后还原；落库失败只记 `audit_record_failed` 告警。

## 5. 权限码

权限码定义在 `backend/internal/domain/role.go`，共 14 个常量。`RequirePermMiddleware` 通过 `pkg.PermChecker`（由 `service.RoleService` 实现）判定，命中 `*` 视为全部权限。

| 权限码 | 常量 | 校验路由 |
| --- | --- | --- |
| `*` | `PermAdminAll` | 通配，内置 `admin` 角色持有；不做单点路由绑定 |
| `system:user:manage` | `PermUserManage` | `/admin/user/list`、`/create`、`/update`、`/delete`、`/reset_password` |
| `system:role:manage` | `PermRoleManage` | `/admin/role/list`、`/save`、`/delete` |
| `system:menu:manage` | `PermMenuManage` | `/menu/list`、`/menu/detail/:MenuULID`、`/menu/save`、`/menu/delete` |
| `system:perm:read` | `PermPermissionRead` | `/menu/permission/catalog` |
| `system:audit:read` | `PermAuditRead` | `/admin/audit/list` |
| `system:loginlog:read` | `PermLoginLogRead` | `/admin/login_log/list` |
| `system:model:manage` | `PermModelManage` | `/model/provider/*`、`/model/catalog/*`（共 10 条） |
| `system:usage:read` | `PermUsageRead` | `/model/usage/overview` |
| `system:status:read` | `PermSystemRead` | `/system/status` |
| `system:setting:manage` | `PermSettingManage` | `/setting/update` |
| `system:job:manage` | `PermJobManage` | `/admin/job/list`、`/admin/job/stats`、`/admin/job/retry` |
| `skill:manage` | `PermSkillManage` | `/skill/save`、`/skill/delete`、`/skill/reference/save`、`/skill/reference/delete` |
| `prompt:manage` | `PermPromptManage` | `/prompt/profile/save`、`/prompt/profile/delete` |

未列入上表的其余 `/api/v1` 路由均为「登录用户即可访问」，数据范围由归属校验收窄。

权限目录（`GET /menu/permission/catalog`）由菜单表汇总而来：`service/menu_service.go` 的 `buildPermCatalog` 先登记 `*`，再按菜单行的 `perm_code` 逐条登记（同一码只登记一次），按权限码首段分组（`system` / `skill` / `project` / `all`）并升序返回。因此新增权限码只需在菜单管理里登记一行，无需改代码。

## 6. 探针

| 方法 | 路径 | 鉴权 | 成功 data | 失败 |
| --- | --- | --- | --- | --- |
| GET | `/health` | 免鉴权 | `{"status":"up"}` | 不会失败（`healthCheck` 不检查依赖） |
| GET | `/ready` | 免鉴权 | `{"status":"ready"}` | 依赖 Ping 失败返回 `ERR_STORAGE` / 503「依赖未就绪」 |

两者均由 `pkg.Ok` 输出包络；`/ready` 的检查函数由 `main.go` 注入（`RouterDeps.ReadyCheck`，Postgres Ping）。

## 7. 接口总表

约定：`POST` 请求体为 JSON（`/knowledge/upload` 除外），`GET` 参数为 query 或路径变量；「权限」列写该路由所属分组的门禁，未标注者为登录用户。

### 7.1 认证（3）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/auth/login` | 免鉴权 | `UserLoginREQ` | `UserLoginDTO` | `AuthAPI.Login` |
| GET | `/auth/profile` | 登录用户 | 无 | `UserVO` | `AuthAPI.Profile` |
| POST | `/auth/logout` | 登录用户 | 无（读 `Authorization` 头） | `null` | `AuthAPI.Logout` |

- `UserLoginREQ`：`account_text`（登录名或邮箱，必填）、`password_text`（必填）。
- `UserLoginDTO`：`access_token`、`expires_in_sec`、`user_vo`（`UserVO`）。
- `UserVO`：`user_ulid`、`user_name`、`user_email`、`avatar_url`、`user_status_code`（`active` / `disabled`）、`role_codes`、`created_at_ms`。
- 登出把当前令牌写入吊销名单（Redis），之后同一令牌访问任意接口返回 401。

### 7.2 个人中心（3）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/profile/detail` | 登录用户 | 无 | `UserProfileVO` | `AuthAPI.ProfileDetail` |
| POST | `/profile/update` | 登录用户 | `UserProfileUpdateREQ` | `UserProfileVO` | `AuthAPI.UpdateProfile` |
| POST | `/profile/password/change` | 登录用户 | `UserPasswordChangeREQ` | `null` | `AuthAPI.ChangePassword` |

- `UserProfileUpdateREQ` 只允许改 `user_email`、`avatar_url`、`remark_text`；`user_name` 不可改。
- `UserPasswordChangeREQ`：`old_password_text`、`new_password_text`（均 6–64 字符）。旧密码不符返回 `ERR_UNAUTHORIZED`，邮箱冲突返回 `ERR_CONFLICT`。

### 7.3 站内通知（5）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/notification/list` | 登录用户 | `NotificationListREQ` | `PageDTO<NotificationVO>` | `NotificationAPI.List` |
| GET | `/notification/unread_count` | 登录用户 | 无 | `NotificationUnreadCountVO` | `NotificationAPI.UnreadCount` |
| POST | `/notification/read` | 登录用户 | `NotificationReadREQ` | `null` | `NotificationAPI.Read` |
| POST | `/notification/read_all` | 登录用户 | 无 | `null` | `NotificationAPI.ReadAll` |
| POST | `/notification/delete` | 登录用户 | `NotificationDeleteREQ` | `null` | `NotificationAPI.Delete` |

- `NotificationListREQ`：分页字段 + `read_status_code`（`all` / `unread` / `read`）、`notification_type_code`、`keyword_text`。
- `NotificationReadREQ.notification_ulid_list` 为 1–100 条的批量标记。
- 通知类型：`system` / `task` / `account`。响应项含 `notification_ulid`、`notification_title_text`、`notification_body_text`、`read_flag`、`link_url`、`ref_type_code`、`ref_ulid`、`created_at_ms`。
- 全部接口按 `user_ulid` 隔离，只作用于当前用户的通知。

### 7.4 项目（6）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/project/create` | 登录用户 | `ProjectCreateREQ` | `ProjectVO` | `ProjectAPI.Create` |
| POST | `/project/update` | 登录用户 | `ProjectUpdateREQ` | `ProjectVO` | `ProjectAPI.Update` |
| POST | `/project/delete` | 登录用户 | `ProjectDeleteREQ` | `null` | `ProjectAPI.Delete` |
| POST | `/project/list` | 登录用户 | `ProjectListREQ` | `PageDTO<ProjectVO>` | `ProjectAPI.List` |
| GET | `/project/detail/:ProjectULID` | 登录用户 | 路径变量 | `ProjectVO` | `ProjectAPI.Detail` |
| GET | `/project/tree/:ProjectULID` | 登录用户 | 路径变量 | `ProjectTreeVO` | `ProjectAPI.Tree` |

- `ProjectCreateREQ`：`project_name`（1–128）、`genre_code`、`style_text`、`synopsis_text`。
- `ProjectUpdateREQ`：`project_ulid` + 同上字段 + `project_status`（`draft` / `serial` / `completed`）。
- `ProjectListREQ`：分页字段 + `keyword_text`、`project_status`。
- `ProjectVO`：`project_ulid`、`project_name`、`genre_code`、`style_text`、`synopsis_text`、`project_status`、`total_word_count`、`volume_count`、`chapter_count`、`created_at_ms`、`updated_at_ms`。
- `ProjectTreeVO`：`project_vo` + `volume_list`（`VolumeTreeNodeVO`，含 `volume_vo` 与 `chapter_list`（`ChapterBriefVO`））。
- 删除项目在单个事务内级联清理卷、章节、快照、角色、设定、会话、知识库文档与统计（`ProjectService.Delete`）；事务提交后由 `cleanupKnowledgeExternal` 清理 MinIO 原文与 Manticore 切片，失败仅告警。

### 7.5 卷（3）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/volume/create` | 登录用户 | `VolumeCreateREQ` | `VolumeVO` | `VolumeAPI.Create` |
| POST | `/volume/update` | 登录用户 | `VolumeUpdateREQ` | `VolumeVO` | `VolumeAPI.Update` |
| POST | `/volume/delete` | 登录用户 | `VolumeDeleteREQ` | `null` | `VolumeAPI.Delete` |

`VolumeCreateREQ`：`project_ulid`、`volume_name`（1–128）、`volume_order`（0 表示取项目内最大值 +1）、`summary_text`。
`VolumeUpdateREQ` 只覆盖非空字段，`volume_order` 为 0 表示不调整。`VolumeVO`：`volume_ulid`、`volume_name`、`volume_order`、`summary_text`。

### 7.6 章节与快照（7）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/chapter/create` | 登录用户 | `ChapterCreateREQ` | `ChapterDetailVO` | `ChapterAPI.Create` |
| POST | `/chapter/update` | 登录用户 | `ChapterUpdateREQ` | `ChapterDetailVO` | `ChapterAPI.Update` |
| POST | `/chapter/delete` | 登录用户 | `ChapterDeleteREQ` | `null` | `ChapterAPI.Delete` |
| GET | `/chapter/detail/:ChapterULID` | 登录用户 | 路径变量 | `ChapterDetailVO` | `ChapterAPI.Detail` |
| GET | `/chapter/snapshot/list/:ChapterULID` | 登录用户 | 路径变量 | `[]ChapterSnapshotVO` | `ChapterAPI.SnapshotList` |
| POST | `/chapter/snapshot/create` | 登录用户 | `ChapterSnapshotCreateREQ` | `ChapterSnapshotVO` | `ChapterAPI.SnapshotCreate` |
| POST | `/chapter/snapshot/restore` | 登录用户 | `ChapterSnapshotRestoreREQ` | `ChapterDetailVO` | `ChapterAPI.SnapshotRestore` |

- `ChapterUpdateREQ` 是整体覆盖：`chapter_name`、`volume_ulid`、`chapter_status`（`draft` / `revised` / `locked`）；`content_text`、`summary_text` 为指针字段，省略表示不改，空串表示清空；字数由服务端统计。
- `ChapterDetailVO` 内嵌 `ChapterBriefVO`（`chapter_ulid`、`chapter_name`、`chapter_order`、`word_count`、`chapter_status`），并补 `project_ulid`、`volume_ulid`、`content_text`、`summary_text`、`updated_at_ms`。
- `ChapterSnapshotCreateREQ`：`chapter_ulid`、`change_note`（≤256）。`ChapterSnapshotRestoreREQ`：`snapshot_ulid`；回滚前会为当前正文再留一份快照。
- `ChapterSnapshotVO`：`snapshot_ulid`、`chapter_ulid`、`snapshot_type`、`change_note`、`word_count`、`created_at_ms`。

### 7.7 角色与关系图（6）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/character/save` | 登录用户 | `CharacterREQ` | `CharacterVO` | `CharacterAPI.Save` |
| POST | `/character/delete` | 登录用户 | `CharacterDeleteREQ` | `null` | `CharacterAPI.Delete` |
| POST | `/character/list` | 登录用户 | `CharacterListREQ` | `[]CharacterVO` | `CharacterAPI.List` |
| GET | `/character/graph/:ProjectULID` | 登录用户 | 路径变量 | `CharacterGraphVO` | `CharacterAPI.Graph` |
| POST | `/character/relation/save` | 登录用户 | `CharacterRelationREQ` | `CharacterEdgeVO` | `CharacterAPI.RelationSave` |
| POST | `/character/relation/delete` | 登录用户 | `CharacterRelationDeleteREQ` | `null` | `CharacterAPI.RelationDelete` |

- `CharacterREQ`：`character_ulid`（空为新建）、`project_ulid`、`character_name`（1–64）、`alias_text`、`role_type_code`（`protagonist` / `antagonist` / `supporting` / `minor`）、`personality_text`、`background_text`、`appearance_text`、`goal_text`。
- `CharacterGraphVO`：`node_list`（`CharacterNodeVO`：`character_ulid`、`character_name`、`role_type_code`）+ `edge_list`（`CharacterEdgeVO`：`relation_ulid`、`from_character_ulid`、`to_character_ulid`、`relation_type_code`、`relation_note`）。
- `CharacterRelationREQ`：`project_ulid`、`from_character_ulid`、`to_character_ulid`、`relation_type_code`、`relation_note`。删除角色时连带删除其关系边。

### 7.8 世界观设定与创作笔记（3）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/lore/save` | 登录用户 | `LoreREQ` | `LoreVO` | `LoreAPI.Save` |
| POST | `/lore/delete` | 登录用户 | `LoreDeleteREQ` | `null` | `LoreAPI.Delete` |
| POST | `/lore/list` | 登录用户 | `LoreListREQ` | `[]LoreVO` | `LoreAPI.List` |

- `lore_type_code` 取 `world` / `rule` / `item` / `place` / `event` / `note`；`note` 即创作笔记，复用同一组接口。
- `content_mode_code` 标记正文格式：`rich`（富文本 HTML）或 `markdown`，落库默认 `rich`；更新时省略表示保持原值。
- `LoreListREQ`：`project_ulid`、`lore_type_code`、`keyword_text`（名称/内容模糊）。

### 7.9 知识库（8）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/knowledge/upload` | 登录用户 | multipart：`project_ulid`、`doc_name`、`file` | `KnowledgeDocumentVO` | `KnowledgeAPI.Upload` |
| POST | `/knowledge/list` | 登录用户 | `KnowledgeListREQ` | `PageDTO<KnowledgeDocumentVO>` | `KnowledgeAPI.List` |
| POST | `/knowledge/delete` | 登录用户 | `KnowledgeDeleteREQ` | `null` | `KnowledgeAPI.Delete` |
| POST | `/knowledge/search` | 登录用户 | `KnowledgeSearchREQ` | `KnowledgeSearchDTO` | `KnowledgeAPI.Search` |
| POST | `/knowledge/reindex` | 登录用户 | `KnowledgeReindexREQ` | `null` | `KnowledgeAPI.Reindex` |
| GET | `/knowledge/chunk/list/:DocULID` | 登录用户 | query：`page_num`、`page_size` | `PageDTO<KnowledgeChunkVO>` | `KnowledgeAPI.ChunkList` |
| GET | `/knowledge/content/:DocULID` | 登录用户 | 路径变量 | 原始字节，非包络（见 §8.2） | `KnowledgeAPI.Content` |
| GET | `/knowledge/stats/:ProjectULID` | 登录用户 | 路径变量 | `KnowledgeStatsVO` | `KnowledgeAPI.Stats` |

- `KnowledgeUploadREQ`（表单字段）：`project_ulid`（必填）、`doc_name`（必填，≤128）；文件字段名固定 `file`。仅接受 `txt` / `md` / `docx` / `pdf`，其他后缀返回 `ERR_INVALID_PARAM`；超过 `NW_APP_UPLOAD_MAX_MB` 上限同样返回 `ERR_INVALID_PARAM`。上传成功即写入 MinIO 原文、落库 `pending` 并投递 `doc_index` 任务。
- `KnowledgeListREQ`：分页字段 + `project_ulid`（必填）+ `doc_status_code`（`pending` / `indexing` / `indexed` / `failed`）。
- `KnowledgeSearchREQ`：`project_ulid`（必填）、`keyword_text`（≤512，去空白后为空返回 `ERR_INVALID_PARAM`）、`search_mode`（仅 `hybrid` 走双通道，其余值一律按 `keyword`）、`top_k_count`（1–20，缺省取应用配置 `rag_top_k`，再退回全局配置）、`doc_ulid_list`（限定文档范围，空为全部）。
- `KnowledgeSearchDTO`：`result_list`（`KnowledgeChunkVO`，含 `chunk_ulid`、`doc_ulid`、`doc_name`、`chunk_index`、`chunk_text`、`score_value`、`source_mode`）+ `trace`（`query_terms`、`query_expr`、`candidate_count`、`reranked`、`rerank_note`、`selected_ids`、`source_mode`）。重排失败为 fail-open，保留原顺序并在 `rerank_note` 说明。
- `KnowledgeReindexREQ.doc_ulid`：仅 `indexed` / `failed` 状态可重投，否则返回 `ERR_BIZ_FAILED`；重投把文档置回 `pending` 并投递 `doc_reindex`。
- `KnowledgeStatsVO`：`total_count`、`indexed_count`、`pending_count`（含 `indexing`）、`failed_count`、`chunk_count`。

### 7.10 技能与提示词档案（12）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/skill/list` | 登录用户 | query：`scope_code`、`project_ulid`、`category_code`、`keyword_text`、`enabled_only` | `[]SkillVO` | `SkillAPI.List` |
| GET | `/skill/detail/:SkillULID` | 登录用户 | 路径变量 | `SkillVO` | `SkillAPI.Detail` |
| POST | `/skill/resolve` | 登录用户 | `SkillResolveREQ` | `SkillResolveDTO` | `SkillAPI.Resolve` |
| GET | `/skill/reference/list` | 登录用户 | query：`skill_ulid` | `[]SkillReferenceVO` | `SkillAPI.ReferenceList` |
| POST | `/skill/save` | `skill:manage` | `SkillREQ` | `SkillVO` | `SkillAPI.Save` |
| POST | `/skill/delete` | `skill:manage` | `SkillDeleteREQ` | `null` | `SkillAPI.Delete` |
| POST | `/skill/reference/save` | `skill:manage` | `SkillReferenceREQ` | `SkillReferenceVO` | `SkillAPI.ReferenceSave` |
| POST | `/skill/reference/delete` | `skill:manage` | `SkillReferenceDeleteREQ` | `null` | `SkillAPI.ReferenceDelete` |
| POST | `/prompt/profile/list` | 登录用户 | `PromptProfileListREQ`（`project_ulid`、`enabled_only`） | `[]PromptProfileVO` | `PromptProfileAPI.List` |
| GET | `/prompt/profile/detail/:PromptProfileULID` | 登录用户 | 路径变量 | `PromptProfileVO` | `PromptProfileAPI.Detail` |
| POST | `/prompt/profile/save` | `prompt:manage` | `PromptProfileREQ` | `PromptProfileVO` | `PromptProfileAPI.Save` |
| POST | `/prompt/profile/delete` | `prompt:manage` | `PromptProfileDeleteREQ` | `null` | `PromptProfileAPI.Delete` |

- 作用域：`global` 全局可用，`project` 仅所属项目可用；同编码的项目技能覆盖全局技能（`SkillVO.overridden_flag` 标记被覆盖项）。
- `SkillVO`：`skill_ulid`、`skill_code`、`skill_name`、`scope_code`、`project_ulid`、`category_code`（`write` / `revise` / `outline` / `setting` / `review` / `other`）、`description_text`、`prompt_template`、`variable_list`、`ref_count`、`enabled_flag`、`builtin_flag`、`overridden_flag`、`sort_order`、`created_at_ms`、`updated_at_ms`。
- `SkillResolveREQ`：`project_ulid`、`content_text`（≤20000）、`skill_code`（显式指定优先）、`variable_value`。`SkillResolveDTO`：`skill_ulid`、`skill_code`、`skill_name`、`remainder_text`、`variable_map`、`missing_list`、`ref_text_list`；`missing_list` 非空表示必填变量缺失。
- 变量取值优先级：显式传值 > 声明缺省值 > 必填缺失记入 `missing_list`（`resolveSkillVariables`）。
- 模板占位符只允许内置占位符（`user_input` / `chapter_content` / `context` / `skill_code`）与已声明变量，出现未声明占位符时保存返回 `ERR_INVALID_PARAM`（`validateSkillTemplate`）。
- 内置技能 `builtin_flag=true`：不可删除（返回 `ERR_BIZ_FAILED`），可停用或改提示词；`scope_code` 与 `skill_code` 不可改。
- `PromptProfileVO`：`prompt_profile_ulid`、`prompt_profile_code`、`prompt_profile_name`、`scope_code`、`project_ulid`、`description_text`、`system_prompt_text`、`builtin_flag`、`enabled_flag`、`sort_order`、`updated_at_ms`。作用域规则与技能一致；项目档案的归属校验比技能更严（`mustOwnProject`，校验创建者）。内置档案不可删、编码不可改，可改正文与名称。

### 7.11 菜单与权限目录（6）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/menu/nav` | 登录用户 | 无 | `MenuNavDTO` | `MenuAPI.Nav` |
| GET | `/menu/list` | `system:menu:manage` | query：`keyword_text`、`menu_type_code`、`scope_code` | `[]MenuVO` | `MenuAPI.List` |
| GET | `/menu/detail/:MenuULID` | `system:menu:manage` | 路径变量 | `MenuVO` | `MenuAPI.Detail` |
| POST | `/menu/save` | `system:menu:manage` | `MenuREQ` | `MenuVO` | `MenuAPI.Save` |
| POST | `/menu/delete` | `system:menu:manage` | `MenuDeleteREQ` | `null` | `MenuAPI.Delete` |
| GET | `/menu/permission/catalog` | `system:perm:read` | 无 | `[]PermGroupVO` | `MenuAPI.PermCatalog` |

- 菜单表同时是权限码登记处：`page` 行承载页面权限码，`action` 行只登记动作权限码、不进导航。
- `MenuNavDTO`：`menu_tree_list`（`MenuTreeNodeVO` 递归 `child_list`）+ `perm_code_list`。只返回启用、可见、非 `action` 且当前用户有权限（或持有 `*`）的节点。
- `MenuREQ`：`menu_ulid`、`parent_ulid`、`menu_code`（2–64，唯一）、`menu_name`、`menu_type_code`（`group` / `page` / `action`）、`scope_code`（`global` / `project`）、`route_path`、`icon_code`、`perm_code`、`visible_flag`、`enabled_flag`、`sort_order`、`remark_text`。
- 约束：`group` 不可配路由（`ERR_INVALID_PARAM`）；菜单编码唯一（重复 `ERR_CONFLICT`）；权限码全局唯一，同码只能被一行登记（重复 `ERR_CONFLICT`）；上级必须存在且不能是自己或自己的下级（`ERR_INVALID_PARAM`）；存在下级时不可删除（`ERR_CONFLICT`，提示先删下级）。
- `PermGroupVO`：`group_code`、`group_name`、`perm_list`（`PermItemVO`：`perm_code`、`perm_name`、`perm_desc`、`menu_code`、`menu_name`、`builtin_flag`）。

### 7.12 应用配置（2）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/setting/list` | 登录用户 | 无 | `[]SettingVO` | `SettingAPI.List` |
| POST | `/setting/update` | `system:setting:manage` | `SettingUpdateREQ` | `SettingVO` | `SettingAPI.Update` |

白名单与取值范围（`service/setting_service.go` 的 `settingDefaultList` 与 `settingRangeMap`）：

| 键 | 默认值 | 取值范围 | 生效位置 |
| --- | --- | --- | --- |
| `chat_history_count` | 5 | 0–50 | `ChatService.resolveHistoryLimit`：请求 `history_count` > 本配置 > 全局配置 |
| `rag_top_k` | 6 | 1–20 | `KnowledgeService.resolveRAGTopK`：请求 `top_k_count` > 本配置 > 全局配置 |
| `daily_word_goal` | 2000 | 100–100000 | 前端仪表盘读取，仅影响展示 |

`SettingUpdateREQ`：`setting_key`（须在白名单内）、`setting_value`（须为正整数且落在区间），越界或键不在白名单返回 `ERR_INVALID_PARAM`。

### 7.13 数据总览（1）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/stats/overview` | 登录用户 | query：`project_ulid`（可空，空为全局） | `StatsOverviewVO` | `StatsAPI.Overview` |

`StatsOverviewVO`：`project_count`、`total_word_count`、`today_word_delta`、`ai_word_total`、`trend_list`（近 30 天 `WordStatVO`：`stat_date`、`word_delta`、`ai_word_delta`）。

### 7.14 AI 对话（5）

| 方法 | 路径 | 权限 | 请求 | 响应 | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/chat/send` | 登录用户 | `ChatSendREQ` | `text/event-stream`（见 §8.1） | `ChatAPI.Send` |
| POST | `/chat/history` | 登录用户 | `ChatHistoryREQ` | `PageDTO<ChatMessageVO>` | `ChatAPI.History` |
| POST | `/chat/session/create` | 登录用户 | `ChatSessionCreateREQ` | `ChatSessionVO` | `ChatAPI.SessionCreate` |
| POST | `/chat/session/list` | 登录用户 | `ChatSessionListREQ` | `[]ChatSessionVO` | `ChatAPI.SessionList` |
| POST | `/chat/session/delete` | 登录用户 | `ChatSessionDeleteREQ` | `null` | `ChatAPI.SessionDelete` |

`ChatSendREQ`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `project_ulid` | string | 所属项目，必填 |
| `session_ulid` | string | 会话 ULID，空则自动创建 |
| `content_text` | string | 消息正文，必填，≤8000 |
| `skill_code` | string | 技能编码（≤32），空为自由对话 |
| `skill_variable_map` | object | 技能变量取值 `{变量名: 值}` |
| `ref_doc_ulid_list` | []string | @引用的知识库文档 |
| `ref_chapter_ulid_list` | []string | @引用的章节 |
| `ref_message_ulid_list` | []string | 显式引用的历史消息 |
| `history_count` | int | 注入历史条数 0–50，0 用应用配置 |
| `model_ulid` | string | 指定模型，空取「系统默认模型 → 第一个可用模型」 |
| `prompt_profile_ulid` | string | 指定系统提示词档案，空取「启用的全局默认档案 → 内置兜底人设」 |

- `ChatHistoryREQ`：`session_ulid`（必填）+ 分页字段；结果按 `created_at` 升序。
- `ChatSessionCreateREQ`：`project_ulid`、`session_name`（空则取首条消息前 20 字）。`ChatSessionVO`：`session_ulid`、`project_ulid`、`session_name`、`message_count`、`updated_at_ms`。
- `ChatMessageVO`：`message_ulid`、`session_ulid`、`role_code`（`user` / `assistant` / `system`）、`content_text`、`skill_code`、`ref_doc_ulid_list`、`ref_chapter_ulid_list`、`ref_message_ulid_list`、`token_used_count`、`created_at_ms`。
- 所选模型未授权给当前账号返回 `ERR_FORBIDDEN`；账号无任何可用模型返回 `ERR_AI_PROVIDER`。

### 7.15 创作规划（6）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/outline/node/save` | 登录用户 | `OutlineNodeREQ` | `OutlineNodeVO` | `OutlineAPI.SaveNode` |
| POST | `/outline/node/delete` | 登录用户 | `OutlineNodeDeleteREQ` | `null` | `OutlineAPI.DeleteNode` |
| POST | `/outline/list` | 登录用户 | `OutlineTreeREQ` | `[]OutlineNodeVO` | `OutlineAPI.ListTree` |
| POST | `/outline/scene/save` | 登录用户 | `DetailedSceneREQ` | `DetailedSceneVO` | `OutlineAPI.SaveScene` |
| POST | `/outline/scene/delete` | 登录用户 | `DetailedSceneDeleteREQ` | `null` | `OutlineAPI.DeleteScene` |
| POST | `/outline/scene/list` | 登录用户 | `DetailedSceneListREQ` | `[]DetailedSceneVO` | `OutlineAPI.ListScenes` |

- `OutlineNodeREQ`：`outline_ulid`（空为新建）、`project_ulid`、`parent_outline_ulid`（根节点留空）、`node_type_code`（`volume` / `arc` / `storyBlock` / `chapter`）、`title_text`、`summary_text`、`node_status_code`（`draft` / `planning` / `ready` / `done`）、`sort_order`（0 表示追加到末尾）。
- `OutlineNodeVO.children` 递归承载子树，按 `sort_order` 升序。
- `DetailedSceneREQ`：`scene_ulid`、`project_ulid`、`outline_ulid`（必填）、`character_ulid_list`、`location_text`、`conflict_text`、`pace_code`（`slow` / `medium` / `fast` / `climax`）、`estimated_words`、`entry_state_text`、`goal_text`、`key_choice_text`、`outcome_text`、`exit_state_text`、`sort_order`。
- `DetailedSceneListREQ.outline_ulid` 留空表示项目下全部场景。

### 7.16 连续性看板（7）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/continuity/fact/save` | 登录用户 | `TemporalFactSaveREQ` | `TemporalFactVO` | `ContinuityAPI.SaveFact` |
| POST | `/continuity/fact/review` | 登录用户 | `TemporalFactReviewREQ` | `TemporalFactVO` | `ContinuityAPI.ReviewFact` |
| POST | `/continuity/fact/list` | 登录用户 | `TemporalFactListREQ` | `[]TemporalFactVO` | `ContinuityAPI.ListFacts` |
| POST | `/continuity/hook/save` | 登录用户 | `ForeshadowHookSaveREQ` | `ForeshadowHookVO` | `ContinuityAPI.SaveHook` |
| POST | `/continuity/hook/list` | 登录用户 | `ForeshadowHookListREQ` | `[]ForeshadowHookVO` | `ContinuityAPI.ListHooks` |
| POST | `/continuity/summary/save` | 登录用户 | `ChapterSummarySaveREQ` | `ChapterSummaryVO` | `ContinuityAPI.SaveSummary` |
| POST | `/continuity/summary/list` | 登录用户 | `ChapterSummaryListREQ` | `[]ChapterSummaryVO` | `ContinuityAPI.ListSummaries` |

- 事实 `TemporalFactSaveREQ`：`fact_ulid`（空为新建）、`project_ulid`、`subject_text`、`predicate_code`、`object_text`、`fact_status_code`（`candidate` / `confirmed` / `superseded` / `stale`）、`valid_from_chapter_ulid`、`valid_until_chapter_ulid`、`source_chapter_ulid`（必填）、`source_quote_text`、`note_text`。
- 复核 `TemporalFactReviewREQ`：`fact_ulid`、`project_ulid`、`fact_status_code`（只接受 `confirmed` / `superseded` / `stale`）、`note_text`。
- 伏笔 `ForeshadowHookSaveREQ`：`hook_status_code`（`planned` / `planted` / `echoed` / `resolved` / `abandoned`）、`description_text`、`expected_payoff_text`、`setup_chapter_ulid`、`payoff_chapter_ulid`、`importance_code`（`high` / `medium` / `low`）。
- 摘要 `ChapterSummarySaveREQ`：`summary_ulid`（空时按章节覆盖）、`chapter_ulid`、`summary_text`（1–20000）、`source_hash_text`、`summary_status_code`（`pending` / `verified` / `stale`）。

### 7.17 模型生态（11）

供应商承载「网关 + 凭据」，模型是使用与授权的最小单位：`model_providers` → `model_catalogs` → `model_role_grants`。

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/model/capability` | 登录用户 | 无 | `ModelCapabilityVO` | `ModelProviderAPI.Capability` |
| POST | `/model/provider/list` | `system:model:manage` | 无 | `[]ModelProviderVO` | `ModelProviderAPI.ProviderList` |
| POST | `/model/provider/save` | `system:model:manage` | `ModelProviderREQ` | `ModelProviderVO` | `ModelProviderAPI.ProviderSave` |
| POST | `/model/provider/delete` | `system:model:manage` | `ModelProviderDeleteREQ` | `null` | `ModelProviderAPI.ProviderDelete` |
| POST | `/model/provider/test` | `system:model:manage` | `ModelProviderTestREQ` | `ModelProviderTestDTO` | `ModelProviderAPI.ProviderTest` |
| POST | `/model/catalog/list` | `system:model:manage` | `ModelCatalogListREQ` | `[]ModelCatalogVO` | `ModelProviderAPI.ModelList` |
| POST | `/model/catalog/save` | `system:model:manage` | `ModelCatalogREQ` | `ModelCatalogVO` | `ModelProviderAPI.ModelSave` |
| POST | `/model/catalog/delete` | `system:model:manage` | `ModelCatalogDeleteREQ` | `null` | `ModelProviderAPI.ModelDelete` |
| POST | `/model/catalog/test` | `system:model:manage` | `ModelCatalogTestREQ` | `ModelCatalogTestDTO` | `ModelProviderAPI.ModelTest` |
| POST | `/model/catalog/set_default` | `system:model:manage` | `ModelCatalogTestREQ` | `null` | `ModelProviderAPI.ModelSetDefault` |
| POST | `/model/catalog/grant` | `system:model:manage` | `ModelGrantREQ` | `null` | `ModelProviderAPI.ModelGrant` |

- `ModelProviderREQ`：`provider_ulid`（空为创建）、`provider_name`、`protocol_code`（仅 `openai` / `anthropic`）、`base_url`（合法 URL）、`api_key_text`（空表示保持原值）、`enabled_flag`、`sort_order`、`remark_text`。密钥以 AES-256-GCM 加密保存，`ModelProviderVO` 只回 `api_key_masked` 与 `api_key_configured`。
- `ModelCatalogREQ`：`model_ulid`、`provider_ulid`、`model_name`、`model_label`、`temperature`（0–2）、`max_tokens`、`context_window_count`、`enabled_flag`、`default_flag`、`sort_order`、`remark_text`、`role_ulid_list`（保存时整体覆盖授权）。
- `ModelCapabilityVO`：`chat_ready`、`default_model_ulid`、`default_model_label`、`default_model_name`、`default_provider_name`、`option_list`（`ModelOptionVO`）、`hint_text`；管理员（含 `*`）可见全部启用模型，其余账号按角色授权并集过滤，供应商或模型停用即从 `option_list` 剔除。
- `ModelProviderTestDTO` / `ModelCatalogTestDTO`：`status_code`（`success` / `failed`）、`message_text`、`latency_ms`、`reply_text`（供应商测试另含 `probe_model_id`）。

服务层约束：

| 约束 | 行为 |
| --- | --- |
| 系统默认模型唯一 | `set_default` 与 `save(default_flag=true)` 先清空全部默认标记 |
| 默认模型不可删 | `catalog/delete` 返回 `ERR_BIZ_FAILED`，提示先切换默认 |
| 供应商不可带模型删除 | `provider/delete` 在 `model_count > 0` 时返回 `ERR_BIZ_FAILED` |
| 授权整体覆盖 | `catalog/grant` 与 `catalog/save.role_ulid_list` 均为「先删后插」全量覆盖 |
| 停用即不可用 | 供应商停用或模型停用都会从 `/model/capability` 剔除 |

启动期：`ModelProviderService.SeedDefault` 在供应商表为空且配置了 `NW_AI_BASE_URL` / `NW_AI_CHAT_MODEL` 时播种「一条供应商 + 一个默认模型」并授权给内置角色；`ModelProviderService.MigrateLegacyModels` 在模型目录为空时把供应商自带的 `chat_model_name` 补成模型目录行，属启动期兼容逻辑，失败只告警不阻断启动。

### 7.18 用量分析与运行状态（2）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/model/usage/overview` | `system:usage:read` | query：`from_date`、`to_date` | `ModelUsageOverviewVO` | `ModelUsageAPI.Overview` |
| GET | `/system/status` | `system:status:read` | 无 | `SystemStatusVO` | `SystemStatusAPI.Status` |

- 日期格式 `YYYY-MM-DD`；缺省近 14 天，非法输入回退默认，起止倒置自动纠正（`resolveUsageRange`）。
- `ModelUsageOverviewVO`：`call_count_total`、`fail_count_total`、`total_tokens_total`、`avg_latency_ms`、`success_rate_value`（0–1）、`daily_list`、`provider_list`、`model_list`。
- `SystemStatusVO`：`server_started_at_ms`、`server_uptime_sec`、`go_version_text`、`build_version_text`、`dependency_list`、`chat_ready`、`hint_text`。`dependency_code` 取 `postgres` / `redis` / `rabbitmq` / `minio` / `manticore` / `model`，`status_code` 取 `up` / `down` / `degraded`。

### 7.19 系统管理（13）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/admin/user/list` | `system:user:manage` | `UserAdminListREQ` | `PageDTO<UserAdminVO>` | `UserAdminAPI.List` |
| POST | `/admin/user/create` | `system:user:manage` | `UserAdminCreateREQ` | `UserAdminVO` | `UserAdminAPI.Create` |
| POST | `/admin/user/update` | `system:user:manage` | `UserAdminUpdateREQ` | `UserAdminVO` | `UserAdminAPI.Update` |
| POST | `/admin/user/delete` | `system:user:manage` | `UserAdminDeleteREQ` | `null` | `UserAdminAPI.Delete` |
| POST | `/admin/user/reset_password` | `system:user:manage` | `UserResetPasswordREQ` | `null` | `UserAdminAPI.ResetPassword` |
| POST | `/admin/role/list` | `system:role:manage` | `RoleListREQ` | `PageDTO<RoleVO>` | `RoleAPI.List` |
| POST | `/admin/role/save` | `system:role:manage` | `RoleSaveREQ` | `RoleVO` | `RoleAPI.Save` |
| POST | `/admin/role/delete` | `system:role:manage` | `RoleDeleteREQ` | `null` | `RoleAPI.Delete` |
| POST | `/admin/audit/list` | `system:audit:read` | `AuditListREQ` | `PageDTO<AuditVO>` | `AuditAPI.List` |
| POST | `/admin/login_log/list` | `system:loginlog:read` | `LoginLogListREQ` | `PageDTO<LoginLogVO>` | `LoginLogAPI.List` |
| POST | `/admin/job/list` | `system:job:manage` | `JobListREQ` | `PageDTO<JobVO>` | `JobAPI.List` |
| GET | `/admin/job/stats` | `system:job:manage` | 无 | `JobStatsVO` | `JobAPI.Stats` |
| POST | `/admin/job/retry` | `system:job:manage` | `JobRetryREQ` | `null` | `JobAPI.Retry` |

- 用户：`UserAdminCreateREQ`（`user_name` 3–64、`user_email`、`password_text` 6–64、`role_codes`、`remark_text`），用户名或邮箱重复返回 `ERR_CONFLICT`，`role_codes` 为空时默认挂 `user` 角色；`UserAdminUpdateREQ.role_codes` 为整体覆盖语义；`UserResetPasswordREQ` 只收 `user_ulid` 与 `new_password_text`；不能删除自己（`ERR_BIZ_FAILED`）。
- 角色：`RoleListREQ` 仅含分页字段；`RoleSaveREQ`：`role_ulid`、`role_code`、`role_name`、`remark_text`、`perm_codes`（≤32）。内置角色（`admin` / `user`）不可删除（`ERR_BIZ_FAILED`）、不可改编码；仍有用户挂载的角色不可删除（`ERR_CONFLICT`）；角色编码重复返回 `ERR_CONFLICT`。
- 审计：`AuditListREQ`：分页字段 + `action_code`（模糊）+ `user_ulid`（精确）。`AuditVO` 含 `audit_ulid`、`user_ulid`、`user_name`、`action_code`、`target_type`、`target_ulid`、`detail_text`、`ip_addr`、`created_at_ms`。
- 登录日志：`LoginLogListREQ`：分页字段 + `keyword_text`（账号/IP）+ `login_result_code`（`success` / `failed` / `disabled`）+ `date_from` / `date_to`。
- 任务：`JobListREQ`：分页字段 + `job_type_code`、`job_status_code`、`keyword_text`。状态 `queued` / `running` / `success` / `failed`，类型 `doc_index` / `doc_reindex`；重试仅接受 `failed`，成功后任务回到 `queued`、文档回到 `pending`。`JobStatsVO` 给四态计数。

## 8. 特殊端点

### 8.1 `POST /chat/send`（SSE）

流开始前的失败（绑定校验、鉴权、模型不可用）仍返回普通 JSON 包络；流开始后（响应头已提交）的失败只能经 SSE `error` 事件下发，服务端仅记日志。

响应头在首条事件写出时提交：

| 头 | 值 |
| --- | --- |
| `Content-Type` | `text/event-stream; charset=utf-8` |
| `Cache-Control` | `no-cache` |
| `X-Accel-Buffering` | `no` |

事件行格式 `data: {json}\n\n`，心跳为注释行 `: ping\n\n`；每条数据是 `domain.ChatStreamEventDTO`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `seq` | int64 | 单调递增事件序号 |
| `event_type` | string | `thinking` / `chunk` / `done` / `error` |
| `session_ulid` | string | 会话 ULID |
| `message_ulid` | string | 助手消息 ULID |
| `content_text` | string | 增量思考（`thinking`）、增量文本（`chunk`）或完整文本（`done`） |
| `reasoning_text` | string | 完整思考过程（`done` 时给出，模型未返回推理内容时为空串） |
| `reference_list` | `[]KnowledgeChunkVO` | RAG 召回依据（`done` 时给出） |
| `error_message` | string | 错误信息（`error` 时给出） |

```mermaid
stateDiagram-v2
  [*] --> 请求校验
  请求校验 --> JSON错误: 绑定/鉴权/模型不可用
  请求校验 --> 流式生成: 通过
  流式生成 --> thinking: 推理模型思考增量
  thinking --> 流式生成
  流式生成 --> chunk: 正文增量
  chunk --> 流式生成
  流式生成 --> done: 完整文本 + 完整思考 + 引用依据
  流式生成 --> error: 上游失败
  done --> [*]
  error --> [*]
```

客户端断开连接会取消上游模型调用；生成失败不落库助手消息，只保留用户消息。

`thinking` 事件只在模型返回推理内容（OpenAI 兼容协议的 `reasoning_content` / Anthropic 的 thinking block）时出现；非推理模型只会收到 `chunk` 与 `done`，前端据此在「等待首字节」与「思考中」之间自动切换。`ChatMessageVO.reasoning_text` 在拉取历史时一并返回，重开会话仍能看到上次的思考过程。

### 8.2 `GET /knowledge/content/:DocULID`（原始字节）

该端点成功时不返回业务包络，直接输出文件原始字节（`c.Data`）；失败路径仍由 `respondErr` 输出 JSON 包络。

| 项 | 行为 |
| --- | --- |
| `Content-Type` | `txt` → `text/plain; charset=utf-8`；`md` → `text/markdown; charset=utf-8`；`docx` → `application/vnd.openxmlformats-officedocument.wordprocessingml.document`；`pdf` → `application/pdf`；其他 → `application/octet-stream` |
| `Content-Disposition` | `inline; filename*=UTF-8''<url-escaped doc_name>` |

前端以 `fetch` 取 blob（`fetchKnowledgeContent`），交给 `docx-preview` / `vue-pdf-embed` 预览，`txt` / `md` 直接渲染。

## 9. 分页约定

| 项 | 规则 |
| --- | --- |
| 请求字段 | `page_num`、`page_size`（`domain.PageREQ`，JSON body 与 query 同名） |
| `page_num` | 从 1 开始；`<= 0` 归一为 1 |
| `page_size` | 缺省 20；`<= 0` 或 `> 100` 归一为 20（上限 100） |
| 归一入口 | `PageREQ.Normalize()`，由各 service 在查询前调用 |
| 响应结构 | `PageDTO<T>`：`total`、`page_num`、`page_size`、`items` |

返回 `PageDTO` 的端点（10 条）：

| 端点 | 分页字段来源 |
| --- | --- |
| `POST /project/list` | JSON body |
| `POST /knowledge/list` | JSON body |
| `GET /knowledge/chunk/list/:DocULID` | query |
| `POST /chat/history` | JSON body |
| `POST /notification/list` | JSON body |
| `POST /admin/user/list` | JSON body |
| `POST /admin/role/list` | JSON body |
| `POST /admin/audit/list` | JSON body |
| `POST /admin/login_log/list` | JSON body |
| `POST /admin/job/list` | JSON body |

返回普通数组、全量不翻页的端点：`/chapter/snapshot/list/:ChapterULID`、`/character/list`、`/lore/list`、`/skill/list`、`/skill/reference/list`、`/outline/list`、`/outline/scene/list`、`/continuity/fact/list`、`/continuity/hook/list`、`/continuity/summary/list`、`/chat/session/list`、`/model/provider/list`、`/model/catalog/list`、`/prompt/profile/list`、`/menu/list`、`/setting/list`、`/menu/permission/catalog`。

## 10. 前端消费规则

实现位于 `frontend/src/api/http.ts` 与 `frontend/src/composables/useSSE.ts`。

| 规则 | 实现 |
| --- | --- |
| 基础路径 | Axios 实例 `baseURL: '/api/v1'`，`timeout: 30000` |
| 令牌注入 | 请求拦截器从本地存储读 token，写入 `Authorization: Bearer <token>` |
| 包络解包 | 响应拦截器校验 `resp_code === 'SUCCESS'`，成功时把 `envelope.data` 直接作为调用方返回值；`get<T>` / `post<T>` 的 `T` 即 `data` 类型 |
| 失败归一 | 非成功包络或无响应体统一构造 `NWApiError`（`resp_code`、`resp_message`、`trace_id`、`http_status`）并 reject；无响应体时归为 `ERR_INTERNAL` + `http_status = 0` |
| 未授权 | `resp_code === 'ERR_UNAUTHORIZED'` 时清除 token 与用户信息，提示后跳转 `/login` |
| 单次提示 | `NWApiError.notified_flag` 保证同一错误只弹一次；请求配置 `silent_error: true` 时跳过统一提示（知识库列表/切片等使用） |
| 不存在判定 | `isApiNotReadyError` 对 `ERR_NOT_FOUND` 或 HTTP 404 返回 true |
| 二进制端点 | `/knowledge/content` 不走 Axios 拦截器，用 `fetch` 手动带 `Authorization` 头并返回 `Blob`，`!response.ok` 直接抛错 |
| SSE 不走 Axios | `/chat/send` 用 `fetch` + `ReadableStream`（`streamSsePost`）发起；`parseSseBuffer` 只消费以 `\n` 结尾的完整行，末尾半行留作缓冲，跨 chunk 断行与多事件粘包都能正确解析，非法 JSON 行跳过 |
| 流取消 | `useSSE.startStream` 会先中止上一条流；`stopStream` 触发 `AbortController.abort`，`AbortError` 静默结束、不触发 `onError` |
