# API 契约

本文描述 NovelWorkbench 当前对外暴露的 HTTP 接口。路由以 `backend/internal/api/router.go`
的注册为准，请求/响应结构以 `backend/internal/domain` 为准，包络与状态映射以
`backend/internal/pkg/response.go`、`backend/internal/pkg/errors.go` 为准。

## 1. 通用约定

| 项 | 约定 | 代码位置 |
| --- | --- | --- |
| 业务基础路径 | `/api/v1` | `router.go` 的 `v1 := router.Group("/api/v1")` |
| 探针路径 | `/health`、`/ready`（不在 `/api/v1` 下） | `router.go:58-59` |
| 允许方法 | 仅 `GET` / `POST`；`OPTIONS` 仅用于 CORS 预检 | `pkg.MethodWhitelistMiddleware` |
| 编码 | `application/json; charset=utf-8`；上传为 `multipart/form-data` | 各 handler 的 `ShouldBindJSON` / `c.FormFile` |
| 鉴权头 | `Authorization: Bearer <access_token>` | `pkg.AuthMiddleware` |
| 链路头 | 请求 `X-Request-ID`（缺省服务端生成 `REQ_` 前缀 ULID），响应头回写同值，包络 `trace_id` 同值 | `pkg.RequestIDMiddleware` |
| 业务码 | 字符串：`SUCCESS` 或 `ERR_*` | `domain.RespCode*` |
| 时间 | VO 中毫秒时间戳字段后缀 `_at_ms`；日期字段 `YYYY-MM-DD` | `domain` 各 VO |
| ID | 业务前缀 ULID（`PRJ_` / `CHP_` / `DOC_` …），见 `domain.IDPrefix*` | `domain/common.go` |
| 分页 | `page_num` 从 1 开始；`page_size` 缺省 20、上限 100 | `domain.PageREQ.Normalize` |
| 接口总数 | `/api/v1` 下 87 条，另有 2 条探针，共 89 条 | `router.go` |

响应头 CORS 放行 `Origin`、`Content-Type`、`Authorization`、`X-Request-ID`，
暴露 `X-Request-ID`。

## 2. 响应包络

成功（`pkg.Ok`）：

```json
{
  "resp_code": "SUCCESS",
  "resp_message": "ok",
  "trace_id": "REQ_01HZX...",
  "data": { "items": [], "total": 0, "page_num": 1, "page_size": 20 }
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

包络定义：`domain.BaseRESP`（`resp_code` / `resp_message` / `trace_id`）+
`domain.DataRESP[T]`（`data`）。`data` 在无返回体的写接口上为 `null`。

## 3. 业务码 → HTTP 状态

映射唯一入口为 `pkg.HTTPStatusForCode`（`backend/internal/pkg/response.go`），
未知码归 500。

| 业务码 | HTTP 状态 | 语义 |
| --- | --- | --- |
| `SUCCESS` | 200 | 成功 |
| `ERR_INVALID_PARAM` | 400 | 参数校验失败 |
| `ERR_UNAUTHORIZED` | 401 | 未登录或登录失效 |
| `ERR_FORBIDDEN` | 403 | 无权限 |
| `ERR_NOT_FOUND` | 404 | 资源不存在 |
| `ERR_CONFLICT` | 409 | 资源冲突（重名/重复提交） |
| `ERR_BIZ_FAILED` | 400 | 通用业务失败 |
| `ERR_AI_PROVIDER` | 502 | AI 模型调用失败 |
| `ERR_STORAGE` | 503 | 对象存储/搜索/队列等基础设施错误 |
| `ERR_INTERNAL` | 500 | 服务内部错误 |

`ERR_BIZ_FAILED` 与 `ERR_INVALID_PARAM` 同为 400，靠 `resp_code` 区分语义。

## 4. 中间件链

`pkg.NewRouter` 的装配顺序（`backend/internal/api/router.go`）：

| 序 | 中间件 | 作用 |
| --- | --- | --- |
| 1 | `pkg.RecoveryMiddleware` | panic 恢复，输出 `ERR_INTERNAL` 并中止链 |
| 2 | `pkg.RequestIDMiddleware` | 分配/透传 `X-Request-ID`，写入上下文 |
| 3 | `pkg.LoggerMiddleware` | 出口记录 method/path/status/latency_ms |
| 4 | `cors.New(...)` | CORS，预检直接返回 |
| 5 | `pkg.MethodWhitelistMiddleware` | 非 GET/POST 返回 `ERR_INVALID_PARAM` |
| 6 | `pkg.AuthMiddleware` | 仅挂在 `/api/v1` 组上，校验 Bearer 令牌 |
| 7 | `pkg.AuditMiddleware` | 仅挂在 `/api/v1` 组上，记录 POST 审计 |

`/health`、`/ready` 注册在 `/api/v1` 组之前，只经过 1–5，不鉴权、不审计。

鉴权中间件内部顺序：白名单路径直接放行 → 取 `Authorization` 头（缺省 → 401「未登录」）→
解析 JWT（失败 → 401「登录已失效」）→ Redis 吊销名单检查（命中 → 401）→
用户状态检查（禁用 → 401「账号不存在或已被禁用」）→ 注入 `user_ulid` 上下文键。

### 免鉴权白名单

`backend/cmd/server/main.go` 传入 `pkg.AuthMiddleware` 的 skipPaths：

| 路径 | 原因 |
| --- | --- |
| `/api/v1/auth/login` | 登录本身无令牌 |

其余 `/api/v1` 路径一律需要登录。

### 审计跳过名单

`backend/cmd/server/main.go` 传入 `pkg.AuditMiddleware` 的 skipPaths：

| 路径 | 原因 |
| --- | --- |
| `/api/v1/auth/login` | 无登录用户可归属 |
| `/api/v1/auth/logout` | 无需留痕 |
| `/api/v1/chat/send` | 高频流式调用，不落审计 |

审计只记录 `POST`；动作码为路径去掉 `/api/v1/` 前缀；仅 `/api/v1/admin/` 前缀的请求
会读 body 提取目标 ULID（键白名单：`user_ulid`→user、`role_ulid`→role、
`project_ulid`→project、`chapter_ulid`→chapter、`doc_ulid`→document，
body 采样上限 64 KiB）；审计落库失败只记日志，不阻断业务。

### 权限码

`RequirePermMiddleware` 校验失败返回 `ERR_FORBIDDEN`；`PermChecker` 报错返回
`ERR_INTERNAL`。权限码定义在 `domain/role.go`。

| 权限码 | 覆盖路由 |
| --- | --- |
| `system:model:manage` | `/model/provider/save`、`/model/provider/delete`、`/model/provider/test` |
| `system:usage:read` | `/model/usage/overview` |
| `system:status:read` | `/system/status` |
| `system:user:manage` | `/admin/user/*` |
| `system:role:manage` | `/admin/role/*` |
| `system:menu:manage` | `/menu/list`、`/menu/detail/{MenuULID}`、`/menu/save`、`/menu/delete` |
| `system:perm:read` | `/menu/permission/catalog` |
| `system:audit:read` | `/admin/audit/list` |
| `system:setting:manage` | `/setting/update` |
| `skill:manage` | `/skill/save`、`/skill/delete`、`/skill/reference/save`、`/skill/reference/delete` |
| `*` | 内置 `admin` 角色拥有全部权限 |

未标注权限码的路由一律为「登录用户」。权限目录（`GET /menu/permission/catalog`）由菜单登记
自动汇总，新增权限码无需改代码即可在角色页勾选。

## 5. 探针

| 方法 | 路径 | 鉴权 | 响应 data | 说明 |
| --- | --- | --- | --- | --- |
| GET | `/health` | 免鉴权 | `{"status":"up"}` | 只回存活，不检查依赖（`healthCheck`） |
| GET | `/ready` | 免鉴权 | `{"status":"ready"}` | Ping 关键依赖；失败返回 `ERR_STORAGE`（503）「依赖未就绪」（`readyCheck`） |

## 6. 接口总表

说明：`POST` 请求参数一律走 JSON body（`ShouldBindJSON`），除 `/knowledge/upload`
为 `multipart/form-data`；`GET` 请求参数走 query（`ShouldBindQuery`）。
「权限」列为该路由所在分组的权限门禁。

### 6.1 认证

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/auth/login` | 免鉴权 | `UserLoginREQ` | `UserLoginDTO` | `AuthAPI.Login` |
| GET | `/auth/profile` | 登录用户 | 无 | `UserVO` | `AuthAPI.Profile` |
| POST | `/auth/logout` | 登录用户 | 无（读 `Authorization` 头） | `null` | `AuthAPI.Logout` |

系统不开放注册：账号只能由管理员在用户管理里创建（`POST /admin/user/create`）。
`UserLoginREQ`：`account_text`（登录名或邮箱）、`password_text`。
`UserLoginDTO`：`access_token`、`expires_in_sec`、`user_vo`。
`UserVO.role_codes` 由用户角色关联查出；前端菜单与权限码来自 `GET /menu/nav`，
不再依赖角色编码判断入口。

### 6.2 项目与卷

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/project/create` | 登录用户 | `ProjectCreateREQ` | `ProjectVO` | `ProjectAPI.Create` |
| POST | `/project/update` | 登录用户 | `ProjectUpdateREQ` | `ProjectVO` | `ProjectAPI.Update` |
| POST | `/project/delete` | 登录用户 | `ProjectDeleteREQ` | `null` | `ProjectAPI.Delete` |
| POST | `/project/list` | 登录用户 | `ProjectListREQ` | `PageDTO<ProjectVO>` | `ProjectAPI.List` |
| GET | `/project/detail/{ProjectULID}` | 登录用户 | 路径参数 | `ProjectVO` | `ProjectAPI.Detail` |
| GET | `/project/tree/{ProjectULID}` | 登录用户 | 路径参数 | `ProjectTreeVO` | `ProjectAPI.Tree` |
| POST | `/volume/create` | 登录用户 | `VolumeCreateREQ` | `VolumeVO` | `VolumeAPI.Create` |
| POST | `/volume/update` | 登录用户 | `VolumeUpdateREQ` | `VolumeVO` | `VolumeAPI.Update` |
| POST | `/volume/delete` | 登录用户 | `VolumeDeleteREQ` | `null` | `VolumeAPI.Delete` |

`ProjectTreeVO` 含 `project_vo` 与 `volume_list`，`VolumeTreeNodeVO` 含 `volume_vo` 与
`chapter_list`（`ChapterBriefVO`），写作工作台目录树直接消费。删除项目时由
`ProjectService.Delete` 编排级联删除卷/章节/快照/统计，不影响其他项目。

### 6.3 章节与快照

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/chapter/create` | 登录用户 | `ChapterCreateREQ` | `ChapterDetailVO` | `ChapterAPI.Create` |
| POST | `/chapter/update` | 登录用户 | `ChapterUpdateREQ` | `ChapterDetailVO` | `ChapterAPI.Update` |
| POST | `/chapter/delete` | 登录用户 | `ChapterDeleteREQ` | `null` | `ChapterAPI.Delete` |
| GET | `/chapter/detail/{ChapterULID}` | 登录用户 | 路径参数 | `ChapterDetailVO` | `ChapterAPI.Detail` |
| GET | `/chapter/snapshot/list/{ChapterULID}` | 登录用户 | 路径参数 | `[]ChapterSnapshotVO` | `ChapterAPI.SnapshotList` |
| POST | `/chapter/snapshot/create` | 登录用户 | `ChapterSnapshotCreateREQ` | `ChapterSnapshotVO` | `ChapterAPI.SnapshotCreate` |
| POST | `/chapter/snapshot/restore` | 登录用户 | `ChapterSnapshotRestoreREQ` | `ChapterDetailVO` | `ChapterAPI.SnapshotRestore` |

章节更新是正文整体覆盖：`content_text`、`summary_text` 为指针字段，缺省表示不改，
空串表示清空；字数在服务端计算。快照用于 AI 写入前留档与人工回滚，回滚前会为当前正文
再留一份快照。

### 6.4 角色与关系图

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/character/save` | 登录用户 | `CharacterREQ` | `CharacterVO` | `CharacterAPI.Save` |
| POST | `/character/delete` | 登录用户 | `CharacterDeleteREQ` | `null` | `CharacterAPI.Delete` |
| POST | `/character/list` | 登录用户 | `CharacterListREQ` | `[]CharacterVO` | `CharacterAPI.List` |
| GET | `/character/graph/{ProjectULID}` | 登录用户 | 路径参数 | `CharacterGraphVO` | `CharacterAPI.Graph` |
| POST | `/character/relation/save` | 登录用户 | `CharacterRelationREQ` | `CharacterEdgeVO` | `CharacterAPI.RelationSave` |
| POST | `/character/relation/delete` | 登录用户 | `CharacterRelationDeleteREQ` | `null` | `CharacterAPI.RelationDelete` |

`CharacterGraphVO` 含 `node_list`（`CharacterNodeVO`）与 `edge_list`（`CharacterEdgeVO`）。
删除角色时连带删除其关系边。

### 6.5 世界观设定与创作笔记

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/lore/save` | 登录用户 | `LoreREQ` | `LoreVO` | `LoreAPI.Save` |
| POST | `/lore/delete` | 登录用户 | `LoreDeleteREQ` | `null` | `LoreAPI.Delete` |
| POST | `/lore/list` | 登录用户 | `LoreListREQ` | `[]LoreVO` | `LoreAPI.List` |

创作笔记复用 `/lore/*`，`lore_type_code` 取 `note`。类型枚举：
`world` / `rule` / `item` / `place` / `event` / `note`。

### 6.6 知识库

| 方法 | 路径 | 权限 | 请求 | 响应 | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/knowledge/upload` | 登录用户 | multipart：`project_ulid`、`doc_name`、`file` | `KnowledgeDocumentVO` | `KnowledgeAPI.Upload` |
| POST | `/knowledge/list` | 登录用户 | `KnowledgeListREQ` | `PageDTO<KnowledgeDocumentVO>` | `KnowledgeAPI.List` |
| POST | `/knowledge/delete` | 登录用户 | `KnowledgeDeleteREQ` | `null` | `KnowledgeAPI.Delete` |
| POST | `/knowledge/search` | 登录用户 | `KnowledgeSearchREQ` | `KnowledgeSearchDTO` | `KnowledgeAPI.Search` |
| POST | `/knowledge/reindex` | 登录用户 | `KnowledgeReindexREQ` | `null` | `KnowledgeAPI.Reindex` |
| GET | `/knowledge/chunk/list` | 登录用户 | `doc_ulid`、`page_num`、`page_size` | `PageDTO<KnowledgeChunkVO>` | `KnowledgeAPI.ChunkList` |
| GET | `/knowledge/content` | 登录用户 | `doc_ulid` | 原始字节（非包络，见 §7.2） | `KnowledgeAPI.Content` |

`KnowledgeSearchREQ`：`project_ulid`(必填)、`keyword_text`(≤512，非空，否则
`ERR_INVALID_PARAM`)、`search_mode`（`keyword`/`hybrid`，缺省或非法值一律按
`keyword` 处理）、`top_k_count`(1–20，缺省取服务端配置 `NW_APP_RAG_TOP_K`)、
`doc_ulid_list`（限定文档范围，空为全部）。

`KnowledgeSearchDTO` 含 `result_list` 与 `trace`。`trace` 字段：`query_terms`、
`query_expr`、`candidate_count`、`reranked`、`rerank_note`、`selected_ids`、
`source_mode`。

文档索引状态 `doc_status_code`：`pending` / `indexing` / `indexed` / `failed`。

### 6.7 技能（Skill）

技能是带作用域的提示词资产：`global` 全局可用，`project` 仅所属项目可用，同编码项目技能覆盖全局技能。

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/skill/list` | 登录用户 | `scope_code` / `project_ulid` / `category_code` / `keyword_text` / `enabled_only` | `[]SkillVO` | `SkillAPI.List` |
| GET | `/skill/detail/{SkillULID}` | 登录用户 | 路径参数 | `SkillVO` | `SkillAPI.Detail` |
| POST | `/skill/resolve` | 登录用户 | `SkillResolveREQ` | `SkillResolveDTO` | `SkillAPI.Resolve` |
| GET | `/skill/reference/list` | 登录用户 | `skill_ulid` | `[]SkillReferenceVO` | `SkillAPI.ReferenceList` |
| POST | `/skill/save` | `skill:manage` | `SkillREQ` | `SkillVO` | `SkillAPI.Save` |
| POST | `/skill/delete` | `skill:manage` | `SkillDeleteREQ` | `null` | `SkillAPI.Delete` |
| POST | `/skill/reference/save` | `skill:manage` | `SkillReferenceREQ` | `SkillReferenceVO` | `SkillAPI.ReferenceSave` |
| POST | `/skill/reference/delete` | `skill:manage` | `SkillReferenceDeleteREQ` | `null` | `SkillAPI.ReferenceDelete` |

- `SkillVO`：`skill_ulid`、`skill_code`、`skill_name`、`scope_code`、`project_ulid`、
  `category_code`、`description_text`、`prompt_template`、`variable_list`、`ref_count`、
  `enabled_flag`、`builtin_flag`、`overridden_flag`、`sort_order`、时间戳。
- `variable_list` 项：`variable_name`、`variable_label`、`default_value`、`required_flag`、`description_text`。
- `SkillResolveDTO`：`skill_ulid`、`skill_code`、`skill_name`、`remainder_text`、`variable_map`、
  `missing_list`、`ref_text_list`。`missing_list` 非空表示必填变量未提供，前端应提示补齐。
- 模板占位符只允许三类内置（`user_input` / `chapter_content` / `context`）与已声明变量，
  未声明的占位符保存时返回 `ERR_INVALID_PARAM`。
- 内置技能 `builtin_flag=true`，不可删除，可停用或改提示词；`scope_code` 与 `skill_code` 不可改。

### 6.7.1 菜单与权限目录

菜单表同时是权限码登记处：`page` 行承载页面权限码，`action` 行只登记动作权限码（不进导航）。

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/menu/nav` | 登录用户 | 无 | `MenuNavDTO` | `MenuAPI.Nav` |
| GET | `/menu/list` | `system:menu:manage` | `keyword_text` / `menu_type_code` / `scope_code` | `[]MenuVO` | `MenuAPI.List` |
| GET | `/menu/detail/{MenuULID}` | `system:menu:manage` | 路径参数 | `MenuVO` | `MenuAPI.Detail` |
| POST | `/menu/save` | `system:menu:manage` | `MenuREQ` | `MenuVO` | `MenuAPI.Save` |
| POST | `/menu/delete` | `system:menu:manage` | `MenuDeleteREQ` | `null` | `MenuAPI.Delete` |
| GET | `/menu/permission/catalog` | `system:perm:read` | 无 | `[]PermGroupVO` | `MenuAPI.PermCatalog` |

- `MenuNavDTO`：`menu_tree_list`（`MenuTreeNodeVO` 递归 `child_list`）+ `perm_code_list`。
  只返回 `enabled_flag=true`、`visible_flag=true`、非 `action`、且当前用户有权限的节点。
- `MenuREQ`：`menu_code`、`menu_name`、`menu_type_code`（group/page/action）、`scope_code`
  （global/project）、`route_path`、`icon_code`、`perm_code`、`visible_flag`、`enabled_flag`、
  `sort_order`、`remark_text`、`parent_ulid`。
- 约束：菜单编码唯一；权限码全局唯一（同码只能登记一次）；分组不能配路由；
  上级不能是自己或自己的下级；有下级时不能删除。

### 6.8 创作规划（大纲与场景卡）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/outline/node/save` | 登录用户 | `OutlineNodeREQ` | `OutlineNodeVO` | `OutlineAPI.SaveNode` |
| POST | `/outline/node/delete` | 登录用户 | `OutlineNodeDeleteREQ` | `null` | `OutlineAPI.DeleteNode` |
| POST | `/outline/list` | 登录用户 | `OutlineTreeREQ` | `[]OutlineNodeVO` | `OutlineAPI.ListTree` |
| POST | `/outline/scene/save` | 登录用户 | `DetailedSceneREQ` | `DetailedSceneVO` | `OutlineAPI.SaveScene` |
| POST | `/outline/scene/delete` | 登录用户 | `DetailedSceneDeleteREQ` | `null` | `OutlineAPI.DeleteScene` |
| POST | `/outline/scene/list` | 登录用户 | `DetailedSceneListREQ` | `[]DetailedSceneVO` | `OutlineAPI.ListScenes` |

`OutlineNodeVO.children` 为递归子节点（`omitempty`）。节点类型 `node_type_code`：
`volume` / `arc` / `storyBlock` / `chapter`；状态 `node_status_code`：
`draft` / `planning` / `ready` / `done`。场景节奏 `pace_code`：
`slow` / `medium` / `fast` / `climax`。

### 6.9 连续性看板（事实 / 伏笔 / 摘要）

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/continuity/fact/save` | 登录用户 | `TemporalFactSaveREQ` | `TemporalFactVO` | `ContinuityAPI.SaveFact` |
| POST | `/continuity/fact/review` | 登录用户 | `TemporalFactReviewREQ` | `TemporalFactVO` | `ContinuityAPI.ReviewFact` |
| POST | `/continuity/fact/list` | 登录用户 | `TemporalFactListREQ` | `[]TemporalFactVO` | `ContinuityAPI.ListFacts` |
| POST | `/continuity/hook/save` | 登录用户 | `ForeshadowHookSaveREQ` | `ForeshadowHookVO` | `ContinuityAPI.SaveHook` |
| POST | `/continuity/hook/list` | 登录用户 | `ForeshadowHookListREQ` | `[]ForeshadowHookVO` | `ContinuityAPI.ListHooks` |
| POST | `/continuity/summary/save` | 登录用户 | `ChapterSummarySaveREQ` | `ChapterSummaryVO` | `ContinuityAPI.SaveSummary` |
| POST | `/continuity/summary/list` | 登录用户 | `ChapterSummaryListREQ` | `[]ChapterSummaryVO` | `ContinuityAPI.ListSummaries` |

事实状态 `fact_status_code`：`candidate` / `confirmed` / `superseded` / `stale`。
钩子状态 `hook_status_code`：`planned` / `planted` / `echoed` / `resolved` / `abandoned`。
摘要状态 `summary_status_code`：`pending` / `verified` / `stale`。

### 6.10 模型供应商

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/model/capability` | 登录用户 | 无 | `ModelCapabilityVO` | `ModelProviderAPI.Capability` |
| GET | `/model/chat_options` | 登录用户 | 无 | `{capability_vo, provider_list}` | `ModelProviderAPI.ChatOptions` |
| POST | `/model/provider/list` | 登录用户 | 无 | `[]ModelProviderVO` | `ModelProviderAPI.List` |
| POST | `/model/provider/save` | `system:model:manage` | `ModelProviderREQ` | `ModelProviderVO` | `ModelProviderAPI.Save` |
| POST | `/model/provider/delete` | `system:model:manage` | `ModelProviderDeleteREQ` | `null` | `ModelProviderAPI.Delete` |
| POST | `/model/provider/test` | `system:model:manage` | `ModelProviderTestREQ` | `ModelProviderTestDTO` | `ModelProviderAPI.Test` |

协议 `protocol_code` 仅 `openai` / `anthropic`。密钥以 AES-256-GCM 加密保存，
列表仅返回掩码 `api_key_masked` 与 `api_key_configured`；`api_key_text` 留空表示保持原值。
默认供应商全局唯一，由服务层保证；删除默认供应商需先切换。

### 6.11 AI 对话

| 方法 | 路径 | 权限 | 请求 | 响应 | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/chat/send` | 登录用户 | `ChatSendREQ` | `text/event-stream`（见 §7.1） | `ChatAPI.Send` |
| POST | `/chat/history` | 登录用户 | `ChatHistoryREQ` | `PageDTO<ChatMessageVO>` | `ChatAPI.History` |
| POST | `/chat/session/create` | 登录用户 | `ChatSessionCreateREQ` | `ChatSessionVO` | `ChatAPI.SessionCreate` |
| POST | `/chat/session/list` | 登录用户 | `ChatSessionListREQ` | `[]ChatSessionVO` | `ChatAPI.SessionList` |
| POST | `/chat/session/delete` | 登录用户 | `ChatSessionDeleteREQ` | `null` | `ChatAPI.SessionDelete` |

`ChatSendREQ`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `project_ulid` | string | 所属项目（必填） |
| `session_ulid` | string | 会话 ULID，空则自动创建 |
| `content_text` | string | 消息正文（必填，≤8000） |
| `skill_code` | string | 技能编码（≤32），空为自由对话 |
| `skill_variable_map` | object | 技能变量取值（`{变量名: 值}`），必填变量缺失时后端拒绝渲染 |
| `ref_doc_ulid_list` | []string | @引用的知识库文档 |
| `ref_chapter_ulid_list` | []string | @引用的章节 |
| `ref_message_ulid_list` | []string | 显式引用的历史消息 |
| `history_count` | int | 注入历史条数（0–50），0 用应用配置 `chat_history_count` |
| `provider_ulid` | string | 指定供应商，空用默认供应商 |

`ChatHistoryREQ`：`session_ulid`（必填）+ 分页字段。

### 6.12 应用配置

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/setting/list` | 登录用户 | 无 | `[]SettingVO` | `SettingAPI.List` |
| POST | `/setting/update` | `system:setting:manage` | `SettingUpdateREQ` | `SettingVO` | `SettingAPI.Update` |

配置键白名单（`service.settingDefaultList`）：

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `chat_history_count` | 5 | AI 对话历史注入条数（生效）：`ChatService.resolveHistoryLimit` 读取，优先级为请求 `history_count` > 本配置 > 全局配置 |
| `rag_top_k` | 6 | 知识库检索默认召回条数（生效）：`KnowledgeService.resolveRAGTopK` 读取，优先级为请求 `top_k_count` > 本配置 > 全局配置 `NW_APP_RAG_TOP_K` |

更新时键须在白名单内、值须为正整数，否则返回 `ERR_INVALID_PARAM`。

### 6.13 数据总览

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/stats/overview` | 登录用户 | `project_ulid`（可空，空为全局统计） | `StatsOverviewVO` | `StatsAPI.Overview` |

### 6.14 用户管理

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/admin/user/list` | `system:user:manage` | `UserAdminListREQ` | `PageDTO<UserAdminVO>` | `UserAdminAPI.List` |
| POST | `/admin/user/create` | `system:user:manage` | `UserAdminCreateREQ` | `UserAdminVO` | `UserAdminAPI.Create` |
| POST | `/admin/user/update` | `system:user:manage` | `UserAdminUpdateREQ` | `UserAdminVO` | `UserAdminAPI.Update` |
| POST | `/admin/user/delete` | `system:user:manage` | `UserAdminDeleteREQ` | `null` | `UserAdminAPI.Delete` |
| POST | `/admin/user/reset_password` | `system:user:manage` | `UserResetPasswordREQ` | `null` | `UserAdminAPI.ResetPassword` |

`UserAdminUpdateREQ.role_codes` 为整体覆盖语义。

### 6.15 角色管理

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/admin/role/list` | `system:role:manage` | `RoleListREQ`（仅分页字段） | `PageDTO<RoleVO>` | `RoleAPI.List` |
| POST | `/admin/role/save` | `system:role:manage` | `RoleSaveREQ` | `RoleVO` | `RoleAPI.Save` |
| POST | `/admin/role/delete` | `system:role:manage` | `RoleDeleteREQ` | `null` | `RoleAPI.Delete` |

内置角色（`admin` / `user`）不可删除。

### 6.16 审计日志

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| POST | `/admin/audit/list` | `system:audit:read` | `AuditListREQ` | `PageDTO<AuditVO>` | `AuditAPI.List` |

`AuditListREQ`：`action_code`（模糊过滤）、`user_ulid`（精确过滤）+ 分页字段。

### 6.17 用量分析与运行状态

| 方法 | 路径 | 权限 | 请求 | 响应 data | Handler |
| --- | --- | --- | --- | --- | --- |
| GET | `/model/usage/overview` | `system:usage:read` | `from_date`、`to_date` | `ModelUsageOverviewVO` | `ModelUsageAPI.Overview` |
| GET | `/system/status` | `system:status:read` | 无 | `SystemStatusVO` | `SystemStatusAPI.Status` |

`from_date` / `to_date` 格式 `YYYY-MM-DD`，缺省按近 14 天。
`ModelUsageOverviewVO` 含 `call_count_total`、`fail_count_total`、`total_tokens_total`、
`avg_latency_ms`、`success_rate_value`、`daily_list`、`provider_list`、`model_list`。
`SystemStatusVO.dependency_list` 的 `dependency_code` 取
`postgres` / `redis` / `rabbitmq` / `minio` / `manticore` / `model`，
`status_code` 取 `up` / `down` / `degraded`。

## 7. 特殊端点

### 7.1 `POST /chat/send`（SSE）

流开始前的失败（参数校验失败、鉴权失败、模型不可用）仍以普通 JSON 包络返回；
流开始后的失败只能经 SSE `error` 事件下发，服务端仅记日志。

响应头（首条事件写出时提交）：

| 头 | 值 |
| --- | --- |
| `Content-Type` | `text/event-stream; charset=utf-8` |
| `Cache-Control` | `no-cache` |
| `X-Accel-Buffering` | `no` |

事件行格式：`data: {json}\n\n`；心跳为注释行 `: ping\n\n`。
每条数据是一个 `domain.ChatStreamEventDTO`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `seq` | int64 | 单调递增事件序号 |
| `event_type` | string | `chunk` / `done` / `error` |
| `session_ulid` | string | 会话 ULID |
| `message_ulid` | string | 助手消息 ULID |
| `content_text` | string | 增量文本（`chunk`）或完整文本（`done`） |
| `reference_list` | []KnowledgeChunkVO | RAG 召回依据（`done` 时给出） |
| `error_message` | string | 错误信息（`error` 时给出） |

| `event_type` | 语义 |
| --- | --- |
| `chunk` | 增量文本片段 |
| `done` | 生成结束，含完整文本、助手消息 ULID 与召回依据 |
| `error` | 生成失败，含 `error_message` |

客户端断开连接会取消上游模型调用；生成失败不落库助手消息，只保留用户消息。

### 7.2 `GET /knowledge/content/{DocULID}`（原始字节）

该端点不返回业务包络，直接输出文件原始字节（`c.Data`）：

| 项 | 行为 |
| --- | --- |
| `Content-Type` | 按文档类型映射：`txt`→`text/plain; charset=utf-8`、`md`→`text/markdown; charset=utf-8`、`docx`→`application/vnd.openxmlformats-officedocument.wordprocessingml.document`、`pdf`→`application/pdf`；其他 → `application/octet-stream` |
| `Content-Disposition` | `inline; filename*=UTF-8''<url-escaped doc_name>` |

前端以 blob 方式读取并交给 `docx-preview` / `vue-pdf-embed` 预览。

## 8. 分页约定

| 项 | 规则 |
| --- | --- |
| 请求字段 | `page_num`、`page_size`（`domain.PageREQ`，同时支持 JSON body 与 query） |
| `page_num` | 从 1 开始；`<=0` 归一为 1 |
| `page_size` | 缺省 20；`<=0` 或 `>100` 归一为 20（上限 100） |
| 响应结构 | `PageDTO<T>`：`total`、`page_num`、`page_size`、`items` |

使用分页的端点（`data` 为 `PageDTO`）：

| 端点 | 分页字段来源 |
| --- | --- |
| `POST /project/list` | JSON body |
| `POST /knowledge/list` | JSON body |
| `GET /knowledge/chunk/list/{DocULID}` | query（分页参数） |
| `POST /chat/history` | JSON body |
| `POST /admin/user/list` | JSON body |
| `POST /admin/role/list` | JSON body |
| `POST /admin/audit/list` | JSON body |

返回普通数组的端点（无分页，全量返回）：`/chapter/snapshot/list/{ChapterULID}`、`/character/list`、
`/lore/list`、`/skill/list`、`/outline/list`、`/outline/scene/list`、
`/continuity/fact/list`、`/continuity/hook/list`、`/continuity/summary/list`、
`/chat/session/list`、`/model/provider/list`、`/setting/list`。

## 9. 前端消费规则

实现位于 `frontend/src/api/http.ts` 与 `frontend/src/composables/useSSE.ts`。

| 规则 | 实现 |
| --- | --- |
| 包络解包 | Axios 响应拦截器校验 `resp_code === 'SUCCESS'`，成功时把 `envelope.data` 直接作为调用方返回值；`get<T>` / `post<T>` 返回类型即 `data` 类型 |
| 失败处理 | `resp_code !== 'SUCCESS'` 时构造 `NWApiError`（含 `resp_code`、`resp_message`、`trace_id`、`http_status`）并 reject |
| 未授权 | `resp_code === 'ERR_UNAUTHORIZED'` 时清除本地 token 与用户信息，弹出提示并跳转 `/login` |
| 单次提示 | `NWApiError.notified_flag` 保证同一错误只弹一次；请求配置 `silent_error: true` 时不弹统一提示 |
| 网络异常 | 无响应体时归一为 `resp_code = 'ERR_INTERNAL'`，`http_status = 0` |
| 不存在判定 | `isApiNotReadyError` 对 `ERR_NOT_FOUND` 或 HTTP 404 返回 true |
| SSE 不走 Axios | `/chat/send` 用 `fetch` + `ReadableStream`（`streamSsePost`）发起，手动带 `Authorization` 头；`parseSseBuffer` 只消费完整行，末尾半行留作缓冲，跨 chunk 断行与多事件粘包都能正确解析 |
| 流取消 | `useSSE` 的 `startStream` 会先中止上一条流；主动 `abort` 静默结束，不触发 `onError` |
