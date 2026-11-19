# API 契约

本文描述 NovelWorkbench 当前对外暴露的接口。路径以 `backend/internal/api/router.go`
的注册为准，请求与响应结构以 `backend/internal/domain` 为准。

## 1. 通用约定

| 项 | 约定 |
| --- | --- |
| 基础路径 | `/api/v1` |
| 方法 | 仅 `GET` / `POST`；`OPTIONS` 仅用于 CORS 预检 |
| 编码 | `application/json; charset=utf-8`（上传为 `multipart/form-data`） |
| 鉴权 | `Authorization: Bearer <access_token>` |
| 链路 ID | 请求头 `X-Request-ID`，响应包络 `trace_id` |
| 业务码 | 字符串：`SUCCESS` 或 `ERR_*` |
| 时间 | VO 中毫秒时间戳 `*_at_ms`；日期 `YYYY-MM-DD` |
| ID | 业务前缀 ULID，例如 `PRJ_01HZX...` |
| 分页 | `page_num` 从 1 开始，`page_size` 默认 20、最大 100 |

成功包络：

```json
{
  "resp_code": "SUCCESS",
  "resp_message": "ok",
  "trace_id": "REQ_01HZX...",
  "data": { "items": [] }
}
```

失败包络：

```json
{
  "resp_code": "ERR_INVALID_PARAM",
  "resp_message": "项目名不能为空",
  "trace_id": "REQ_01HZX...",
  "data": null
}
```

业务码到 HTTP 状态的映射集中在 `pkg.HTTPStatusForCode`：

| 业务码 | HTTP 状态 |
| --- | --- |
| `SUCCESS` | 200 |
| `ERR_INVALID_PARAM` | 400 |
| `ERR_UNAUTHORIZED` | 401 |
| `ERR_FORBIDDEN` | 403 |
| `ERR_NOT_FOUND` | 404 |
| `ERR_CONFLICT` | 409 |
| `ERR_BIZ_FAILED` | 400 |
| `ERR_AI_PROVIDER` | 502 |
| `ERR_STORAGE` | 503 |
| `ERR_INTERNAL` | 500 |

## 2. 健康检查

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| GET | `/health` | 否 | 存活探针，返回 `{"status":"up"}` |
| GET | `/ready` | 否 | 就绪探针，Ping 关键依赖，失败返回 `ERR_STORAGE` |

## 3. 认证

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/auth/register` | `UserRegisterREQ` | `UserVO` |
| POST | `/auth/login` | `UserLoginREQ` | `UserLoginDTO` |
| GET | `/auth/profile` | 无 | `UserVO` |
| POST | `/auth/logout` | 无 | `null` |

`/auth/register` 与 `/auth/login` 由鉴权中间件的免鉴权白名单放行。

`UserLoginDTO` 含 `access_token`、`expires_in_sec`、`user_vo`。`UserVO.role_codes`
由用户角色关联查出，前端据此显示管理入口。

## 4. 项目与目录

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/project/create` | `ProjectCreateREQ` | `ProjectVO` |
| POST | `/project/update` | `ProjectUpdateREQ` | `ProjectVO` |
| POST | `/project/delete` | `ProjectDeleteREQ` | `null` |
| POST | `/project/list` | `ProjectListREQ` | `PageDTO<ProjectVO>` |
| GET | `/project/detail` | `project_ulid` | `ProjectVO` |
| GET | `/project/tree` | `project_ulid` | `ProjectTreeVO` |
| POST | `/volume/create` | `VolumeCreateREQ` | `VolumeVO` |
| POST | `/volume/update` | `VolumeUpdateREQ` | `VolumeVO` |
| POST | `/volume/delete` | `VolumeDeleteREQ` | `null` |

`ProjectTreeVO` 含 `project_vo` 与 `volume_list`（卷节点内嵌章节摘要），
写作工作台左侧目录树直接消费。删除项目时由 service 编排级联删除，不波及其他项目。

## 5. 章节与快照

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/chapter/create` | `ChapterCreateREQ` | `ChapterDetailVO` |
| POST | `/chapter/update` | `ChapterUpdateREQ` | `ChapterDetailVO` |
| POST | `/chapter/delete` | `ChapterDeleteREQ` | `null` |
| GET | `/chapter/detail` | `chapter_ulid` | `ChapterDetailVO` |
| GET | `/chapter/snapshot/list` | `chapter_ulid` | `[]ChapterSnapshotVO` |
| POST | `/chapter/snapshot/create` | `ChapterSnapshotCreateREQ` | `ChapterSnapshotVO` |
| POST | `/chapter/snapshot/restore` | `ChapterSnapshotRestoreREQ` | `ChapterDetailVO` |

章节更新是正文整体覆盖：正文与梗概为指针字段，缺省表示不改，空串表示清空。
字数在服务端计算，客户端不可伪造。快照用于 AI 写入前留档与人工回滚，
回滚前自动为当前正文再留一份快照。

## 6. 角色、设定与笔记

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/character/save` | `CharacterREQ` | `CharacterVO` |
| POST | `/character/delete` | `CharacterDeleteREQ` | `null` |
| POST | `/character/list` | `CharacterListREQ` | `[]CharacterVO` |
| GET | `/character/graph` | `project_ulid` | `CharacterGraphVO` |
| POST | `/character/relation/save` | `CharacterRelationREQ` | `CharacterEdgeVO` |
| POST | `/character/relation/delete` | `CharacterRelationDeleteREQ` | `null` |
| POST | `/lore/save` | `LoreREQ` | `LoreVO` |
| POST | `/lore/delete` | `LoreDeleteREQ` | `null` |
| POST | `/lore/list` | `LoreListREQ` | `[]LoreVO` |

创作笔记复用 `/lore/*` 接口，`lore_type_code` 固定为 `note`；
`note` 类型不注入 AI 上下文，也不参与知识库检索。

## 7. AI 对话

| 方法 | 路径 | 请求 | 响应 |
| --- | --- | --- | --- |
| POST | `/chat/send` | `ChatSendREQ` | `text/event-stream` |
| POST | `/chat/history` | `ChatHistoryREQ` | `[]ChatMessageVO` |
| POST | `/chat/session/create` | `ChatSessionCreateREQ` | `ChatSessionVO` |
| POST | `/chat/session/list` | `ChatSessionListREQ` | `[]ChatSessionVO` |
| POST | `/chat/session/delete` | `ChatSessionDeleteREQ` | `null` |

`ChatSendREQ` 关键字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `project_ulid` | string | 所属项目（必填） |
| `session_ulid` | string | 会话 ULID，空则自动创建 |
| `content_text` | string | 用户输入正文（必填） |
| `skill_code` | string | 技能编码，可空；正文以 `/xxx` 开头也可命中 |
| `ref_doc_ulid_list` | []string | @引用的知识库文档 |
| `ref_chapter_ulid_list` | []string | @引用的章节，首项视为当前章 |
| `ref_message_ulid_list` | []string | 显式引用的历史消息，不受条数限制 |
| `history_count` | int | 注入历史条数，0 用应用配置 |
| `provider_ulid` | string | 指定供应商，空用默认供应商 |

SSE 事件（`ChatStreamEventDTO`）：

| `event_type` | 说明 |
| --- | --- |
| `chunk` | 增量文本，`seq` 单调递增 |
| `done` | 完整回复、助手消息 ULID、召回依据 `reference_list` |
| `error` | 错误信息 `error_message` |

客户端断开会取消上游模型调用；生成失败不落库助手消息，只保留用户消息作为对话痕迹。

## 8. 知识库

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/knowledge/upload` | multipart：`project_ulid`、`doc_name`、`file` | `KnowledgeDocumentVO` |
| POST | `/knowledge/list` | `KnowledgeListREQ` | `PageDTO<KnowledgeDocumentVO>` |
| POST | `/knowledge/delete` | `KnowledgeDeleteREQ` | `null` |
| POST | `/knowledge/search` | `KnowledgeSearchREQ` | `KnowledgeSearchDTO` |
| POST | `/knowledge/reindex` | `KnowledgeReindexREQ` | `null` |
| GET | `/knowledge/chunk/list` | `doc_ulid` | `PageDTO<KnowledgeChunkVO>` |
| GET | `/knowledge/content` | `doc_ulid` | 文档原文（前端预览） |

`KnowledgeSearchREQ` 字段：`project_ulid`（必填）、`keyword_text`（检索文本）、
`search_mode`（`keyword`/`hybrid`，默认 hybrid，未知值按 keyword 处理）、
`top_k_count`（1–20，默认 6）、`doc_ulid_list`（限定文档范围，空为全部）。
不存在向量模式。

`KnowledgeSearchDTO` 含 `result_list` 与 `trace`。`trace` 说明查询词、Manticore 表达式、
候选数、是否重排、重排说明、最终入选切片 ULID 与主检索通道。

索引状态：`pending`、`indexing`、`indexed`、`failed`。

## 9. 创作规划与连续性

| 方法 | 路径 | 请求 | 说明 |
| --- | --- | --- | --- |
| POST | `/outline/node/save` | `OutlineNodeREQ` | 创建/更新大纲节点 |
| POST | `/outline/node/delete` | `OutlineNodeDeleteREQ` | 删除无子节点的节点 |
| POST | `/outline/list` | `OutlineTreeREQ` | 按项目查询大纲树 |
| POST | `/outline/scene/save` | `DetailedSceneREQ` | 创建/更新场景卡 |
| POST | `/outline/scene/delete` | `DetailedSceneDeleteREQ` | 删除场景卡 |
| POST | `/outline/scene/list` | `DetailedSceneListREQ` | 按项目或节点查询场景卡 |
| POST | `/continuity/fact/save` | `TemporalFactSaveREQ` | 登记/更新候选事实 |
| POST | `/continuity/fact/review` | `TemporalFactReviewREQ` | 审核事实（确认/取代/过期） |
| POST | `/continuity/fact/list` | `TemporalFactListREQ` | 按项目查询事实 |
| POST | `/continuity/hook/save` | `ForeshadowHookSaveREQ` | 创建/更新伏笔 |
| POST | `/continuity/hook/list` | `ForeshadowHookListREQ` | 按项目查询伏笔 |
| POST | `/continuity/summary/save` | `ChapterSummarySaveREQ` | 保存章节摘要 |
| POST | `/continuity/summary/list` | `ChapterSummaryListREQ` | 按项目查询章节摘要 |

所有接口都要求 `project_ulid`，service 层做项目归属校验。
字段细节以 `domain/outline.go` 与 `domain/continuity.go` 为准。

## 10. 模型与技能

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| GET | `/model/capability` | 无 | `ModelCapabilityVO` |
| GET | `/model/chat_options` | 无 | `{capability_vo, provider_list}` |
| POST | `/model/provider/list` | 无 | `[]ModelProviderVO` |
| POST | `/model/provider/save` | `ModelProviderREQ` | `ModelProviderVO` |
| POST | `/model/provider/delete` | `ModelProviderDeleteREQ` | `null` |
| POST | `/model/provider/test` | `ModelProviderTestREQ` | `ModelProviderTestDTO` |
| GET | `/slash/list` | `enabled_only` | `[]SlashCommandVO` |
| POST | `/slash/save` | `SlashCommandREQ` | `SlashCommandVO` |
| POST | `/slash/delete` | `SlashDeleteREQ` | `null` |
| POST | `/slash/parse` | `SlashParseREQ` | `SlashParseDTO` |

供应商协议仅 `openai` / `anthropic`。密钥保存时用 AES-256-GCM 加密，
列表只返回掩码 `api_key_masked`；`api_key_text` 留空表示保持原值。

`/model/capability` 与 `/model/chat_options` 对所有登录用户开放，
供 AI 面板展示可用供应商；供应商写操作收口到 `system:model:manage` 权限。

## 11. 系统管理与运维

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| POST | `/admin/user/list` | `system:user:manage` | 用户分页列表 |
| POST | `/admin/user/create` | 同上 | 创建用户 |
| POST | `/admin/user/update` | 同上 | 更新用户与角色 |
| POST | `/admin/user/delete` | 同上 | 删除用户 |
| POST | `/admin/user/reset_password` | 同上 | 重置密码 |
| POST | `/admin/role/list` | `system:role:manage` | 角色列表 |
| POST | `/admin/role/save` | 同上 | 创建/更新角色 |
| POST | `/admin/role/delete` | 同上 | 删除自定义角色 |
| POST | `/admin/audit/list` | `system:audit:read` | 审计日志 |
| GET | `/model/usage/overview` | `system:usage:read` | 用量趋势与分布 |
| GET | `/system/status` | `system:status:read` | 依赖健康与模型能力 |
| GET | `/setting/list` | 登录用户 | 运行时配置 |
| POST | `/setting/update` | 登录用户 | 更新白名单配置 |
| GET | `/stats/overview` | 登录用户 | 字数与项目统计 |

运行时配置白名单：`chat_history_count`（AI 对话历史注入条数）、
`rag_top_k`（RAG 检索默认召回条数）。值必须为正整数，非白名单键直接拒绝。

## 12. 前端消费规则

- Axios 响应拦截器统一解包 `data`；`resp_code !== SUCCESS` 时 reject `resp_message`。
- `ERR_UNAUTHORIZED` 清除本地 token 与用户信息并跳转登录页。
- 同一错误只提示一次（`NWApiError.notified_flag` 标记）。
- 表单提交期间按钮进入 LOADING 态，避免重复提交。
- 所有写操作在成功或失败后给出可见反馈；错误页提供重试入口。
- SSE 使用独立组合式函数（fetch + ReadableStream）管理流、取消与解析，
  不走普通响应拦截。
