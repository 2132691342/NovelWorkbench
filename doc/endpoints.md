# 接口清单

本文列出 NovelWorkbench 对外暴露的全部 HTTP 接口，按业务模块分组。
通用约定、响应包络、业务码、中间件链与权限码见 [api.md](api.md)。

## 1. 接口总表

约定：`POST` 请求体为 JSON（`/knowledge/upload` 除外），`GET` 参数为 query 或路径变量；权限门禁见 [api.md](api.md) 的权限码一节——未列入该表权限码的路由均为「登录用户即可访问」，数据范围由归属校验收窄。

### 1.1 认证与个人中心（6 条）

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/auth/login` | `UserLoginREQ` | `UserLoginDTO` |
| GET | `/auth/profile` | 无 | `UserVO` |
| POST | `/auth/logout` | 无（读 `Authorization` 头） | `null` |
| GET | `/profile/detail` | 无 | `UserProfileVO` |
| POST | `/profile/update` | `UserProfileUpdateREQ` | `UserProfileVO` |
| POST | `/profile/password/change` | `UserPasswordChangeREQ` | `UserPasswordChangeDTO`（新令牌） |

### 1.2 站内通知（5 条）

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/notification/list` | `NotificationListREQ` | `PageDTO<NotificationVO>` |
| GET | `/notification/unread_count` | 无 | `NotificationUnreadCountVO` |
| POST | `/notification/read` | `NotificationReadREQ` | `null` |
| POST | `/notification/read_all` | 无 | `null` |
| POST | `/notification/delete` | `NotificationDeleteREQ` | `null` |

### 1.3 项目、卷、章节与快照（19 条）

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/project/create` | `ProjectCreateREQ` | `ProjectVO` |
| POST | `/project/update` | `ProjectUpdateREQ` | `ProjectVO` |
| POST | `/project/delete` | `ProjectDeleteREQ` | `null` |
| POST | `/project/list` | `ProjectListREQ` | `PageDTO<ProjectVO>` |
| GET | `/project/detail/:ProjectULID` | 路径变量 | `ProjectVO` |
| GET | `/project/tree/:ProjectULID` | 路径变量 | `ProjectTreeVO` |
| POST | `/project/bootstrap/plan` | `ProjectBootstrapREQ` | `ProjectBootstrapPlanVO` |
| POST | `/project/bootstrap/apply` | `ProjectBootstrapApplyREQ` | `ProjectBootstrapResultVO` |
| POST | `/volume/create` | `VolumeCreateREQ` | `VolumeVO` |
| POST | `/volume/update` | `VolumeUpdateREQ` | `VolumeVO` |
| POST | `/volume/delete` | `VolumeDeleteREQ` | `null` |
| POST | `/chapter/create` | `ChapterCreateREQ` | `ChapterDetailVO` |
| POST | `/chapter/update` | `ChapterUpdateREQ` | `ChapterDetailVO` |
| POST | `/chapter/delete` | `ChapterDeleteREQ` | `null` |
| GET | `/chapter/detail/:ChapterULID` | 路径变量 | `ChapterDetailVO` |
| GET | `/chapter/snapshot/list/:ChapterULID` | 路径变量 | `[]ChapterSnapshotVO` |
| POST | `/chapter/snapshot/create` | `ChapterSnapshotCreateREQ` | `ChapterSnapshotVO` |
| POST | `/chapter/snapshot/restore` | `ChapterSnapshotRestoreREQ` | `ChapterDetailVO` |
| POST | `/chapter/check` | `ChapterCheckREQ` | `ChapterCheckVO` |

### 1.4 角色、设定与关系图（9 条）

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/character/save` | `CharacterREQ` | `CharacterVO` |
| POST | `/character/delete` | `CharacterDeleteREQ` | `null` |
| POST | `/character/list` | `CharacterListREQ` | `[]CharacterVO` |
| GET | `/character/graph/:ProjectULID` | 路径变量 | `CharacterGraphVO` |
| POST | `/character/relation/save` | `CharacterRelationREQ` | `CharacterEdgeVO` |
| POST | `/character/relation/delete` | `CharacterRelationDeleteREQ` | `null` |
| POST | `/lore/save` | `LoreREQ` | `LoreVO` |
| POST | `/lore/delete` | `LoreDeleteREQ` | `null` |
| POST | `/lore/list` | `LoreListREQ` | `[]LoreVO` |

### 1.5 知识库（8 条）

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/knowledge/upload` | multipart：`project_ulid`、`doc_name`、`file` | `KnowledgeDocumentVO` |
| POST | `/knowledge/list` | `KnowledgeListREQ` | `PageDTO<KnowledgeDocumentVO>` |
| POST | `/knowledge/delete` | `KnowledgeDeleteREQ` | `null` |
| POST | `/knowledge/search` | `KnowledgeSearchREQ` | `KnowledgeSearchDTO` |
| POST | `/knowledge/reindex` | `KnowledgeReindexREQ` | `null` |
| GET | `/knowledge/chunk/list/:DocULID` | query：`page_num`、`page_size` | `PageDTO<KnowledgeChunkVO>` |
| GET | `/knowledge/content/:DocULID` | 路径变量 | 原始字节，非包络（见 §8.2） |
| GET | `/knowledge/stats/:ProjectULID` | 路径变量 | `KnowledgeStatsVO` |

### 1.6 提示词与工具（9 条）

| 方法 | 路径 | 权限门禁 | 请求 | 响应 data |
| --- | --- | --- | --- | --- |
| GET | `/prompt/list` | — | query：`scope_code`、`project_ulid`、`prompt_kind_code`、`category_code`、`keyword_text`、`enabled_only` | `[]PromptVO` |
| GET | `/prompt/detail/:PromptULID` | — | 路径变量 | `PromptVO` |
| POST | `/prompt/resolve` | — | `PromptResolveREQ` | `PromptResolveDTO` |
| POST | `/prompt/save` | `prompt:manage` | `PromptREQ` | `PromptVO` |
| POST | `/prompt/delete` | `prompt:manage` | `PromptDeleteREQ` | `null` |
| GET | `/tool/list` | — | query：`keyword_text`、`enabled_only` | `[]ToolVO` |
| POST | `/tool/invoke` | — | `ToolInvokeREQ` | `ToolInvokeDTO` |
| POST | `/tool/save` | `tool:manage` | `ToolREQ` | `ToolVO` |
| POST | `/tool/delete` | `tool:manage` | `ToolDeleteREQ` | `null` |

### 1.7 菜单、配置与总览（9 条）

| 方法 | 路径 | 权限门禁 | 请求 | 响应 data |
| --- | --- | --- | --- | --- |
| GET | `/menu/nav` | — | 无 | `MenuNavDTO` |
| GET | `/menu/list` | `system:menu:manage` | query：`keyword_text`、`menu_type_code`、`scope_code` | `[]MenuVO` |
| GET | `/menu/detail/:MenuULID` | `system:menu:manage` | 路径变量 | `MenuVO` |
| POST | `/menu/save` | `system:menu:manage` | `MenuREQ` | `MenuVO` |
| POST | `/menu/delete` | `system:menu:manage` | `MenuDeleteREQ` | `null` |
| GET | `/menu/permission/catalog` | `system:perm:read` | 无 | `[]PermGroupVO` |
| GET | `/setting/list` | — | 无 | `[]SettingVO` |
| POST | `/setting/update` | `system:setting:manage` | `SettingUpdateREQ` | `SettingVO` |
| GET | `/stats/overview` | — | query：`project_ulid`（可空，空为全局） | `StatsOverviewVO` |

### 1.8 AI 对话（5 条）

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/chat/send` | `ChatSendREQ` | `text/event-stream`（见 §8.1） |
| POST | `/chat/history` | `ChatHistoryREQ` | `PageDTO<ChatMessageVO>` |
| POST | `/chat/session/create` | `ChatSessionCreateREQ` | `ChatSessionVO` |
| POST | `/chat/session/list` | `ChatSessionListREQ` | `[]ChatSessionVO` |
| POST | `/chat/session/delete` | `ChatSessionDeleteREQ` | `null` |

### 1.8.1 自动化工作流（13 条）

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| GET | `/workflow/template/list` | 无 | `[]WorkflowTemplateVO` |
| POST | `/workflow/run/create` | `WorkflowRunCreateREQ` | `WorkflowRunVO` |
| POST | `/workflow/run/quick_start` | `WorkflowQuickStartREQ` | `WorkflowQuickStartVO` |
| POST | `/workflow/run/list` | `WorkflowRunListREQ` | `PageDTO<WorkflowRunVO>` |
| GET | `/workflow/run/detail/:RunULID` | 路径变量 | `WorkflowRunDetailVO` |
| GET | `/workflow/run/stats` | `?project_ulid=` 可选 | `WorkflowRunStatsVO` |
| GET | `/workflow/readiness/:ProjectULID` | 路径变量 | `WorkflowReadinessVO` |
| GET | `/workflow/run/pending` | 无 | `[]WorkflowRunVO` |
| POST | `/workflow/run/approve` | `WorkflowRunApproveREQ` | `WorkflowRunVO` |
| POST | `/workflow/run/reject` | `WorkflowRunRejectREQ` | `WorkflowRunVO` |
| POST | `/workflow/run/pause` | `WorkflowRunIDREQ` | `WorkflowRunVO` |
| POST | `/workflow/run/resume` | `WorkflowRunIDREQ` | `WorkflowRunVO` |
| POST | `/workflow/run/cancel` | `WorkflowRunIDREQ` | `WorkflowRunVO` |
| POST | `/workflow/run/retry` | `WorkflowRunIDREQ` | `WorkflowRunVO` |

规则：创建运行校验项目归属；其余运行接口校验 `run.user_ulid` 归属。模板列表对登录用户开放，不额外要求权限码。审核通过会先把候选正文写回章节（覆盖前自动留快照）再继续；退回会清空候选正文并把 `feedback_text` 注入下一轮起草。详见 [workflow.md](workflow.md)。

### 1.9 创作规划与连续性（13 条）

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/outline/node/save` | `OutlineNodeREQ` | `OutlineNodeVO` |
| POST | `/outline/node/delete` | `OutlineNodeDeleteREQ` | `null` |
| POST | `/outline/list` | `OutlineTreeREQ` | `[]OutlineNodeVO` |
| POST | `/outline/scene/save` | `DetailedSceneREQ` | `DetailedSceneVO` |
| POST | `/outline/scene/delete` | `DetailedSceneDeleteREQ` | `null` |
| POST | `/outline/scene/list` | `DetailedSceneListREQ` | `[]DetailedSceneVO` |
| POST | `/continuity/fact/save` | `TemporalFactSaveREQ` | `TemporalFactVO` |
| POST | `/continuity/fact/review` | `TemporalFactReviewREQ` | `TemporalFactVO` |
| POST | `/continuity/fact/list` | `TemporalFactListREQ` | `[]TemporalFactVO` |
| POST | `/continuity/hook/save` | `ForeshadowHookSaveREQ` | `ForeshadowHookVO` |
| POST | `/continuity/hook/list` | `ForeshadowHookListREQ` | `[]ForeshadowHookVO` |
| POST | `/continuity/summary/save` | `ChapterSummarySaveREQ` | `ChapterSummaryVO` |
| POST | `/continuity/summary/list` | `ChapterSummaryListREQ` | `[]ChapterSummaryVO` |

### 1.10 模型生态与运行状态（13 条）

| 方法 | 路径 | 权限门禁 | 请求 | 响应 data |
| --- | --- | --- | --- | --- |
| GET | `/model/capability` | — | 无 | `ModelCapabilityVO` |
| POST | `/model/provider/list` | `system:model:manage` | 无 | `[]ModelProviderVO` |
| POST | `/model/provider/save` | `system:model:manage` | `ModelProviderREQ` | `ModelProviderVO` |
| POST | `/model/provider/delete` | `system:model:manage` | `ModelProviderDeleteREQ` | `null` |
| POST | `/model/provider/test` | `system:model:manage` | `ModelProviderTestREQ` | `ModelProviderTestDTO` |
| POST | `/model/catalog/list` | `system:model:manage` | `ModelCatalogListREQ` | `[]ModelCatalogVO` |
| POST | `/model/catalog/save` | `system:model:manage` | `ModelCatalogREQ` | `ModelCatalogVO` |
| POST | `/model/catalog/delete` | `system:model:manage` | `ModelCatalogDeleteREQ` | `null` |
| POST | `/model/catalog/test` | `system:model:manage` | `ModelCatalogTestREQ` | `ModelCatalogTestDTO` |
| POST | `/model/catalog/set_default` | `system:model:manage` | `ModelCatalogTestREQ` | `null` |
| POST | `/model/catalog/grant` | `system:model:manage` | `ModelGrantREQ` | `null` |
| GET | `/model/usage/overview` | `system:usage:read` | query：`from_date`、`to_date` | `ModelUsageOverviewVO` |
| GET | `/system/status` | `system:status:read` | 无 | `SystemStatusVO` |

### 1.11 系统管理（18 条）

| 方法 | 路径 | 权限门禁 | 请求 | 响应 data |
| --- | --- | --- | --- | --- |
| POST | `/admin/user/list` | `system:user:manage` | `UserAdminListREQ` | `PageDTO<UserAdminVO>` |
| POST | `/admin/user/create` | `system:user:manage` | `UserAdminCreateREQ` | `UserAdminVO` |
| POST | `/admin/user/update` | `system:user:manage` | `UserAdminUpdateREQ` | `UserAdminVO` |
| POST | `/admin/user/delete` | `system:user:manage` | `UserAdminDeleteREQ` | `null` |
| POST | `/admin/user/reset_password` | `system:user:manage` | `UserResetPasswordREQ` | `null` |
| POST | `/admin/role/list` | `system:role:manage` | `RoleListREQ` | `PageDTO<RoleVO>` |
| POST | `/admin/role/save` | `system:role:manage` | `RoleSaveREQ` | `RoleVO` |
| POST | `/admin/role/delete` | `system:role:manage` | `RoleDeleteREQ` | `null` |
| POST | `/admin/audit/list` | `system:audit:read` | `AuditListREQ` | `PageDTO<AuditVO>` |
| POST | `/admin/audit/trace/list` | `system:audit:read` | `AuditTraceListREQ` | `PageDTO<AuditTraceSummaryVO>` |
| POST | `/admin/audit/stats` | `system:audit:read` | `AuditTraceListREQ` | `AuditStatsVO` |
| GET | `/admin/audit/detail/:AuditULID` | `system:audit:read` | 路径变量 | `AuditDetailVO` |
| GET | `/admin/audit/trace/:TraceID` | `system:audit:read` | 路径变量 | `AuditTraceVO` |
| GET | `/admin/audit/action_options` | `system:audit:read` | 无 | `[]AuditActionOptionVO` |
| GET | `/admin/audit/retention` | `system:audit:read` | 无 | `AuditRetentionVO` |
| POST | `/admin/audit/purge` | `system:audit:read` | 无 | `AuditRetentionVO` |
| POST | `/admin/login_log/list` | `system:loginlog:read` | `LoginLogListREQ` | `PageDTO<LoginLogVO>` |
| POST | `/admin/job/list` | `system:job:manage` | `JobListREQ` | `PageDTO<JobVO>` |
| GET | `/admin/job/stats` | `system:job:manage` | 无 | `JobStatsVO` |
| POST | `/admin/job/retry` | `system:job:manage` | `JobRetryREQ` | `null` |

### 1.12 非通用业务规则

审计相关：
- 审计数据存 Manticore `nw_audit`，不在 PostgreSQL；列表的 `keyword_text` 与 `action_code` 合并成一次全文匹配，因此两者同时给出时是「都要命中」。
- `date_from` / `date_to` 为 `YYYY-MM-DD`，服务端换算为本地时区的毫秒区间（含当日 23:59:59.999）；`time_from` / `time_to` 为 RFC3339 精确时刻，存在时优先于日期字段（前端「近 1 小时」等快捷范围用它）。
- `span_type_code` 过滤取值 `http` / `workflow_step` / `ai_call` / `job` / `system`（`binding:"omitempty,oneof=..."`），空表示全部。
- `AuditVO` / `AuditDetailVO` 的 span 字段：`span_type_code`（类型）、`action_name`（中文动作名，历史行可能为空）、`parent_span_code`（父 span 动作码，顶级为空）、`model_name`（AI 调用的模型名，非 AI 为空）、`token_count`（AI 调用 token，非 AI 为 0）。
- `AuditTraceVO.item_list` 为 `AuditDetailVO[]`（含请求/响应摘要），按 `created_at_ts` 升序，单条链路上限 200 条。
- `AuditRetentionVO` 字段：`retention_days`、`purge_hour`（每日清理整点 0–23）、`total_count`（审计条数）、`login_log_total_count`（登录日志条数）、`purged_count` / `login_log_purged_count`（本次清理删除数）、`last_purged_at`、`next_purge_at`（RFC3339，按当前配置真实计算）。
- `POST /admin/audit/purge` 立即执行一次留存清理（审计 + 登录日志），返回两类删除条数与当前条数；清理动作本身会落一条 `system/audit.purge` 审计。
- **`POST /admin/audit/trace/list`** 是审计页的主视图：一行一次入口请求，返回 `AuditTraceSummaryVO`（`root_action_code` / `root_action_name` / `root_request_path` / `span_count` / `ai_call_count` / `token_count` / `total_latency_ms` / `slowest_span_ms` / `slowest_action_name` / `log_level_code`（链路内最严重）/ `http_status` / `resp_code` / `resp_message` / `started_at` / `finished_at`）与 `trace_id`。筛选项与 `AuditListREQ` 同名同义，另加 `span_type_code`（保留含该类型 span 的链路）。
- **`POST /admin/audit/stats`** 用与链路列表相同的筛选条件返回区间概览 `AuditStatsVO`：`trace_count`、`span_count`、`error_trace_count`、`warn_trace_count`（仅含 warn 的链路）、`ai_call_count`、`token_count`、`avg_latency_ms`（按根 span 算）、`slow_trace_count`（链路耗时 >5s）。

工作流相关：
- `chapter_ulid_list` 为空时按「项目内空白章 → 按 `new_chapter_count` 新建」的顺序解析目标章节；两者都无结果时报 `ERR_BIZ_FAILED`。
- `target_word_count` 与 `gate_score` 越界直接拒绝（不是静默钳制），避免运行中途标准变化。
- `review_gate_flag` 缺省为 true；显式传 false 才会关闭人工审核门禁。
- **`POST /workflow/run/quick_start`** 是一键开跑入口：只需 `project_ulid`，其余参数从项目上的 `auto_*` 默认值取（`auto_target_word_count` / `auto_gate_score` / `auto_review_gate_flag` / `auto_requirement_text`），请求里显式给出的字段覆盖默认。`template_code` 留空时按目标章节现状自动选：有空白章或指定了 `new_chapter_count` → `chapter_draft`；全是成稿 → `chapter_polish`。返回 `WorkflowQuickStartVO`：`run_vo`、`template_code`、`chapter_count`、`ready_flag`、`blocking_count`、`blocking_hint_list`（未就绪阻塞项的前 3 条建议）。**素材缺口不阻塞开跑**，只通过 `ready_flag=false` 与提示列表告知。



字段级定义以 `backend/internal/domain/*.go` 为唯一事实源（每个 REQ/VO 字段都带注释与 `binding` 校验），此处只记录**从字段名看不出来**的规则。

| 模块 | 规则 |
| --- | --- |
| 认证 | 登录名或邮箱均可登录（`UserRepo.GetByAccount` 同时匹配两列）；账号不存在与密码错误返回同一文案与同一业务码，不泄露账号是否存在。登出把当前令牌写入吊销名单（Redis），之后同一令牌访问任意接口返回 401 |
| 个人中心 | `UserProfileUpdateREQ` 只允许改 `user_email`、`avatar_url`、`remark_text`，`user_name` 不可改。旧密码不符返回 `ERR_UNAUTHORIZED`，邮箱冲突返回 `ERR_CONFLICT` |
| 站内通知 | `NotificationReadREQ.notification_ulid_list` 为 1–100 条的批量标记。全部接口按 `user_ulid` 隔离，只作用于当前用户的通知 |
| 项目 | `project_status` 取 `draft` / `serial` / `completed`，创建时固定 `draft`。删除项目在单个事务内级联清理卷、章节、快照、角色、设定、大纲节点与场景卡、连续性数据、会话、知识库文档与统计；事务提交后由 `cleanupKnowledgeExternal` 清理 MinIO 原文与 Manticore 切片，失败仅告警 |
| AI 开书 | `target_chapter_count` 缺省 12。`plan` 只生成不落库，模型输出经 `normalizeBootstrapPlan` 收敛（卷 ≤3、角色 ≤8、设定 ≤10、枚举值校正、名称截断），作者可改字段后回传；`apply` 在单事务内创建项目、卷、章节、角色、设定与两级大纲节点（卷 → 章），失败整体回滚。生成失败返回 `ERR_AI_PROVIDER`，模型未授权返回 `ERR_FORBIDDEN` |
| 卷与章节 | 卷与章节的 `volume_order` / `chapter_order` 传 0 表示取同级最大值 +1。`ChapterUpdateREQ` 是整体覆盖；`content_text`、`summary_text` 为指针字段，省略表示不改、空串表示清空。`ChapterSnapshotRestoreREQ` 回滚前会为当前正文再留一份快照。`ChapterCheckREQ.content_text` 优先于库内正文 |
| 角色与关系图 | `CharacterREQ.character_ulid` 为空即新建。`CharacterRelationREQ` 拒绝自环，两端角色必须存在且同属本项目。删除角色时连带删除其关系边 |
| 设定与笔记 | `lore_type_code` 的 `note` 即创作笔记，与其他设定共用同一组接口。`content_mode_code` 缺省 `rich` |
| 知识库 | 上传仅接受 `txt` / `md` / `docx` / `pdf`，文件字段名固定 `file`；超过 `NW_APP_UPLOAD_MAX_MB` 返回 `ERR_INVALID_PARAM`。上传成功即写入 MinIO 原文、落库 `pending` 并投递 `doc_index` 任务。`KnowledgeSearchREQ.keyword_text` 去空白后为空返回 `ERR_INVALID_PARAM`；`search_mode` 仅 `hybrid` 走双通道，其余值一律按 `keyword`；`top_k_count` 缺省取应用配置 `rag_top_k`，再退回全局配置 |
| 提示词 | 作用域 `global` / `project`；同编码的项目提示词覆盖全局提示词，被覆盖项在列表里带 `overridden_flag`。编码解析：请求体 `prompt_code` 优先，否则只解析正文**首个 token** 的 `/code`，命中启用提示词则剥离命令，未命中按自由对话处理。模板占位符只允许 `user_input` / `chapter_content` / `context` / `prompt_code`，保存期校验，未登记占位符直接 `ERR_INVALID_PARAM`。指令类提示词若用了写作台保留动作编码（`/help` 等），保存被拒绝 |
| 工具 | 内置 6 个工具不可删、不可改编码；`customized_flag` 由名称/说明/参数的改动推导，改过后代码升级不再覆盖。`/tool/invoke` 按登录用户执行归属校验，结果只读、不写项目数据；`content_text` 为空时回落到 `chapter_ulid` 的库内正文。工具不由命令注入：服务端随请求下发启用工具声明，模型自行决定调用与否 |
| 菜单与权限目录 | `page` 行承载页面权限码，`action` 行只登记动作权限码、不进导航。约束：`group` 不可配路由；菜单编码唯一；权限码全局唯一，同码只能被一行登记；上级必须存在且不能是自己或自己的下级；存在下级时不可删除（须先删下级）。隐藏页（`visible_flag=false`）仍下发，前端据此注册路由——这样「把多个页面合并成一个入口」时直接访问旧链接与深链不会失效 |
| 应用配置 | 键为白名单，值为正整数且须落在区间内，越界或键不在白名单返回 `ERR_INVALID_PARAM` |
| AI 对话 | `ref_*_ulid_list` 只对应 `@` 的两类来源（素材库与历史会话），装配方式见下表。工具**不在**请求体里点名：`tool_code_list` 字段已移除，服务端随请求下发启用工具声明，由模型自行决定调用；响应 `done` 事件的 `tool_code_list` 是模型本轮**实际调用**的编码，仅供展示与回看。引用文档必须是本项目或全局库，跨项目返回 `ERR_FORBIDDEN` |
| 创作规划 | 大纲节点 `node_type_code` 取 `volume` / `arc` / `storyBlock` / `chapter`，`node_status_code` 取 `draft` / `planning` / `ready` / `done`。父子关系与成环由 service 校验，含子节点不可直接删除 |
| 连续性看板 | 事实复核只接受 `confirmed` / `superseded` / `stale`，不允许退回 `candidate`。伏笔状态取 `planned` / `planted` / `echoed` / `resolved` / `abandoned`。`ChapterSummarySaveREQ` 不带 `summary_ulid` 时按 `(project_ulid, chapter_ulid)` 覆盖，保证「一章一条摘要」 |
| 模型生态 | 供应商承载「网关 + 凭据」，模型是使用与授权的最小单位：`model_providers` → `model_catalogs` → `model_role_grants` / `model_user_grants`。密钥以 AES-256-GCM 加密保存，VO 只回 `api_key_masked` 与 `api_key_configured`。`api_key_text` 传空表示保持原值。`role_ulid_list` 保存时整体覆盖授权。管理员（含 `*`）可见全部启用模型，其余账号按角色授权与用户直授的并集过滤 |
| 用量与运行状态 | 日期格式 `YYYY-MM-DD`；缺省近 14 天，非法输入回退默认，起止倒置自动纠正。`success_rate_value` 取 0–1。`dependency_code` 取 `postgres` / `redis` / `rabbitmq` / `minio` / `manticore` / `model`，`status_code` 取 `up` / `down` / `degraded` |
| 系统管理 | 用户与邮箱唯一，重复返回 `ERR_CONFLICT`；`role_codes` 为空时默认挂 `user` 角色，`UserAdminUpdateREQ.role_codes` 为整体覆盖语义；不能删除自己（`ERR_BIZ_FAILED`）。内置角色 `admin` / `user` 不可删除、不可改编码；仍有用户挂载的角色不可删除（`ERR_CONFLICT`）。审计列表的 `trace_id` 为精确匹配，`action_code` 与 `keyword_text` 为模糊匹配 |

`@` 引用的装配方式（优先级数值越大越靠前）：

| 字段 | 来源 | 装配方式 |
| --- | --- | --- |
| `ref_character_ulid_list` | 素材库 · 角色 | 显式引用块（优先级 90） |
| `ref_lore_ulid_list` | 素材库 · 设定 | 显式引用块；`note`（创作笔记）被拒 |
| `ref_chapter_ulid_list` | 素材库 · 章节 | 首项为当前章（100），其余 90 |
| `ref_summary_ulid_list` | 素材库 · 章节摘要 | 显式引用块 |
| `ref_doc_ulid_list` | 素材库 · 文档 | 全文按切片拼接后进块 |
| `ref_session_ulid_list` | 历史会话 | 逐条编号归档后进块，一条不删；当前会话自身被拒 |
