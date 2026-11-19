# API 契约

## 1. 通用约定

| 项 | 约定 |
| --- | --- |
| 基础路径 | `/api/v1` |
| 方法 | 仅 `GET` / `POST`；`OPTIONS` 仅用于 CORS 预检 |
| 编码 | `application/json; charset=utf-8` |
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

业务码到 HTTP 状态映射集中在 `pkg.HTTPStatusForCode`：参数错误 400、未登录 401、无权限 403、不存在 404、冲突 409、AI 供应商异常 502、基础设施异常 503、内部错误 500。

## 2. 健康检查

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| GET | `/health` | 否 | 存活探针，返回 `status=up` |
| GET | `/ready` | 否 | 就绪探针，Ping PostgreSQL，失败返回 `ERR_STORAGE` |

## 3. 认证

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/api/v1/auth/register` | `UserRegisterREQ` | `UserVO` |
| POST | `/api/v1/auth/login` | `UserLoginREQ` | `UserLoginDTO` |
| GET | `/api/v1/auth/profile` | 无 | `UserVO` |
| POST | `/api/v1/auth/logout` | 无 | `null` |

`UserLoginDTO` 含 `access_token`、`expires_in_sec`、`user_vo`。用户 `role_codes` 由用户角色关联查出，前端据此显示管理入口。

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

`ProjectTreeVO` 返回卷节点与每卷下的章节摘要，工作台左侧目录树直接消费。删除卷时由 service 控制章节删除范围，避免跨项目误删。

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

章节更新是正文整体覆盖；字数在服务端计算，客户端不可伪造。快照用于 AI 写入前留档与人工回滚。

## 6. 角色、设定与笔记

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/character/save` | `CharacterSaveREQ` | `CharacterVO` |
| POST | `/character/delete` | `CharacterDeleteREQ` | `null` |
| POST | `/character/list` | `CharacterListREQ` | `[]CharacterVO` |
| GET | `/character/graph` | `project_ulid` | `CharacterGraphVO` |
| POST | `/character/relation/save` | `CharacterRelationSaveREQ` | `CharacterRelationVO` |
| POST | `/character/relation/delete` | `CharacterRelationDeleteREQ` | `null` |
| POST | `/lore/save` | `LoreSaveREQ` | `LoreVO` |
| POST | `/lore/delete` | `LoreDeleteREQ` | `null` |
| POST | `/lore/list` | `LoreListREQ` | `[]LoreVO` |

笔记为前端本地富文本编辑 + 后端按项目持久化；私人笔记不注入 AI 上下文。

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
| `project_ulid` | string | 所属项目 |
| `session_ulid` | string | 会话 ULID |
| `content_text` | string | 用户输入 |
| `chapter_ulid` | string | 当前章节，可空 |
| `skill_code` | string | 技能编码，可空；正文以 `/xxx` 开头也可命中 |
| `ref_doc_ulid_list` | []string | 显式引用的知识库文档 |
| `ref_chapter_ulid_list` | []string | 显式引用的章节 |
| `history_count` | int | 注入历史条数，默认 5 |
| `provider_ulid` | string | 指定供应商，空为默认 |
| `retrieve_flag` | bool | 是否执行知识库检索 |

SSE 事件：

| event | data | 说明 |
| --- | --- | --- |
| `chunk` | `ChatStreamEventDTO` | 增量文本，`seq` 单调递增 |
| `done` | `ChatStreamEventDTO` | 完整回复、消息 ULID、召回依据 |
| `error` | `ChatStreamEventDTO` | 错误码与说明 |

客户端断开会取消上游模型调用；流结束后补记真实用量。

## 8. 知识库

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| POST | `/knowledge/upload` | multipart：`project_ulid`、`file` | `KnowledgeDocumentVO` |
| POST | `/knowledge/list` | `KnowledgeListREQ` | `PageDTO<KnowledgeDocumentVO>` |
| POST | `/knowledge/delete` | `KnowledgeDeleteREQ` | `null` |
| POST | `/knowledge/search` | `KnowledgeSearchREQ` | `KnowledgeSearchVO` |
| POST | `/knowledge/reindex` | `KnowledgeReindexREQ` | `null` |
| GET | `/knowledge/chunk/list` | `doc_ulid` | `[]KnowledgeChunkVO` |
| GET | `/knowledge/content` | `doc_ulid` | 文档下载/预览内容 |

`KnowledgeSearchREQ` 使用 `query_text`、`project_ulid`、可选 `doc_ulid_list`、`top_k`、`rerank_flag`。检索模式仅有关键词与混合词法通道；不存在向量模式。响应带 `trace` 字段，说明查询词、候选数、选中 ID 与来源通道。

索引状态：`pending`、`indexing`、`indexed`、`failed`。前端每 3 秒轮询列表，直到状态稳定。

## 8.1 创作规划与连续性

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/outline/node/save` | 创建/更新大纲节点 |
| POST | `/outline/node/delete` | 删除无子节点的节点 |
| POST | `/outline/list` | 按项目查询大纲树 |
| POST | `/outline/scene/save` | 创建/更新场景卡 |
| POST | `/outline/scene/delete` | 删除场景卡 |
| POST | `/outline/scene/list` | 按项目或节点查询场景卡 |
| POST | `/continuity/fact/save` | 登记候选事实 |
| POST | `/continuity/fact/review` | 审核事实（确认/取代/失效） |
| POST | `/continuity/fact/list` | 按项目查询事实 |
| POST | `/continuity/hook/save` | 创建/更新伏笔 |
| POST | `/continuity/hook/list` | 按项目查询伏笔 |
| POST | `/continuity/summary/save` | 保存章节摘要 |
| POST | `/continuity/summary/list` | 按项目查询章节摘要 |

所有接口都要求 `project_ulid`，service 层做项目归属校验；详细字段以 `domain/outline.go` 与 `domain/continuity.go` 为准。

## 9. 模型与技能

| 方法 | 路径 | 请求 | 响应 data |
| --- | --- | --- | --- |
| GET | `/model/capability` | 无 | `ModelCapabilityVO`（chat_ready） |
| GET | `/model/chat_options` | 无 | `[]ModelChatOptionVO` |
| POST | `/model/provider/list` | 无 | `[]ModelProviderVO` |
| POST | `/model/provider/save` | `ModelProviderSaveREQ` | `ModelProviderVO` |
| POST | `/model/provider/delete` | `ModelProviderDeleteREQ` | `null` |
| POST | `/model/provider/test` | `ModelProviderTestREQ` | `ModelProviderProbeVO` |
| GET | `/slash/list` | `enabled_only` | `[]SlashCommandVO` |
| POST | `/slash/save` | `SlashCommandREQ` | `SlashCommandVO` |
| POST | `/slash/delete` | `SlashDeleteREQ` | `null` |
| POST | `/slash/parse` | `SlashParseREQ` | `SlashParseDTO` |

供应商协议仅 `openai` / `anthropic`。密钥保存时用 AES-256-GCM 加密，列表只返回掩码；测试接口只返回对话通道结论，不暴露密钥。

## 10. 系统管理与运维

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

## 11. 前端消费规则

- Axios 响应拦截器统一解包 `data`；`resp_code !== SUCCESS` 时 reject `resp_message`。
- `ERR_UNAUTHORIZED` 清除本地 token 并跳转登录页。
- 表单提交期间按钮进入 LOADING 态，避免重复提交。
- 所有写操作在成功或失败后给出可见反馈；错误页提供重试入口。
- SSE 使用独立组合式函数管理 `EventSource`/fetch 流、取消与重连状态。
