# API 文档

- 基础路径：`/api/v1`；仅 `GET`（查询）/`POST`（动作），删除走 `POST .../delete`。
- 鉴权：除 `register/login` 外均需 `Authorization: Bearer <access_token>`。
- 时间字段一律毫秒时间戳（`xxx_at_ms`，int64）；ID 一律带前缀 ULID 字符串。

## 1. 响应包络

```json
{
  "resp_code": "SUCCESS",
  "resp_message": "ok",
  "trace_id": "REQ_01J...",
  "data": {}
}
```

| resp_code | HTTP | 含义 |
| --- | --- | --- |
| SUCCESS | 200 | 成功 |
| ERR_INVALID_PARAM | 400 | 参数校验失败/业务失败 |
| ERR_UNAUTHORIZED | 401 | 未登录/令牌失效 |
| ERR_FORBIDDEN | 403 | 越权访问他人资源 |
| ERR_NOT_FOUND | 404 | 资源不存在 |
| ERR_CONFLICT | 409 | 重名/重复注册 |
| ERR_AI_PROVIDER | 502 | 模型网关异常 |
| ERR_STORAGE | 503 | 存储/队列/检索异常 |
| ERR_INTERNAL | 500 | 服务内部错误 |

分页响应 `data` 统一为：`{ "total": int64, "page_num": int, "page_size": int, "items": [...] }`（空结果 `items` 为 `[]`）。

## 2. 认证

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /auth/register | 注册 |
| POST | /auth/login | 登录 |
| GET | /auth/profile | 当前用户信息 |
| POST | /auth/logout | 登出（令牌拉黑） |

### POST /auth/register

请求：`{ "user_name": "string", "user_email": "string", "password_text": "string" }`（登录名 3–64、密码 6–64、邮箱合法）。成功 `data=null`；登录名或邮箱重复返回 `ERR_CONFLICT`。

### POST /auth/login

请求：`{ "account_text": "登录名或邮箱", "password_text": "string" }`

响应 `data`（UserLoginDTO）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| access_token | string | JWT 令牌 |
| expires_in_sec | int64 | 有效期（秒） |
| user_vo | UserVO | 用户信息 |

### GET /auth/profile → data=UserVO

UserVO：`user_ulid / user_name / user_email / avatar_url / created_at_ms`。

## 3. 项目 / 卷 / 章节

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /project/create | 创建项目（ProjectCreateREQ） |
| POST | /project/update | 更新项目（ProjectUpdateREQ） |
| POST | /project/delete | 删除项目（body: project_ulid，级联删卷/章/设定） |
| POST | /project/list | 项目分页（ProjectListREQ → PageDTO[ProjectVO]） |
| GET | /project/detail?project_ulid= | 项目详情 → ProjectVO |
| GET | /project/tree?project_ulid= | 目录树 → ProjectTreeVO |
| POST | /volume/create | 建卷（project_ulid, volume_name, summary_text） |
| POST | /volume/update | 改卷（volume_ulid, volume_name, summary_text） |
| POST | /volume/delete | 删卷（volume_ulid；级联删卷下章节并回补字数） |
| POST | /chapter/create | 建章（ChapterCreateREQ） |
| POST | /chapter/update | 改章（ChapterUpdateREQ；content_text/summary_text 缺省不改、显式空串清空） |
| POST | /chapter/delete | 删章（chapter_ulid，回补字数） |
| GET | /chapter/detail?chapter_ulid= | 章节详情（含正文）→ ChapterDetailVO |
| GET | /chapter/snapshot/list?chapter_ulid= | 快照列表 → []ChapterSnapshotVO |
| POST | /chapter/snapshot/create | 人工留档（chapter_ulid, change_note） |
| POST | /chapter/snapshot/restore | 回滚（snapshot_ulid；回滚前自动留档当前正文） |

ProjectVO：`project_ulid / project_name / genre_code / style_text / synopsis_text / project_status / total_word_count / volume_count / chapter_count / created_at_ms / updated_at_ms`。
ProjectTreeVO：`project_vo + volume_list[]`，每个卷节点含 `volume_vo + chapter_list[]`（ChapterBriefVO 升序：`chapter_ulid / chapter_name / chapter_order / word_count / chapter_status`）。

字数口径：CJK 逐字 + 拉丁按词；章节增删改与 `total_word_count`、当日 `word_count_stats` 同事务联动。

## 4. 角色 / 设定

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /character/save | 新建/更新角色（CharacterREQ，有 character_ulid 为更新） |
| POST | /character/delete | 删角色（character_ulid，连带删关系边） |
| POST | /character/list | 角色列表（project_ulid + 可选 role_type_code/keyword → []CharacterVO） |
| GET | /character/graph?project_ulid= | 关系图 → CharacterGraphVO（node_list/edge_list） |
| POST | /character/relation/save | 新建关系边（CharacterRelationREQ） |
| POST | /character/relation/delete | 删关系边（relation_ulid） |
| POST | /lore/save | 新建/更新设定（LoreREQ） |
| POST | /lore/delete | 删设定（lore_ulid） |
| POST | /lore/list | 设定列表（LoreListREQ → []LoreVO） |

## 5. 知识库

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /knowledge/upload | 上传文档（multipart：file + project_ulid + doc_name） |
| POST | /knowledge/list | 文档分页（KnowledgeListREQ → PageDTO[KnowledgeDocumentVO]） |
| POST | /knowledge/delete | 删文档（doc_ulid；删 MinIO 原文 + 切片 + 索引） |
| POST | /knowledge/search | 检索（KnowledgeSearchREQ → []KnowledgeChunkVO） |
| GET | /knowledge/chunk/list?doc_ulid= | 文档切片列表（分页参数可选） |
| POST | /knowledge/reindex | 重投索引任务（doc_ulid） |
| GET | /knowledge/content?doc_ulid= | 文档原文（Content-Type 按类型，供 docx/pdf 预览） |

上传即触发异步索引（RabbitMQ），文档 `doc_status_code` 流转 `pending → indexing → indexed/failed`；检索 `search_mode`：`vector`（语义）/`keyword`（全文）/`hybrid`（默认，RRF 融合）。`KnowledgeChunkVO.score_value` 为融合得分，`source_mode` 标注命中通道。

## 6. AI 对话（SSE）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | /chat/send | 发送消息，响应为 `text/event-stream` |
| POST | /chat/history | 会话消息分页（ChatHistoryREQ → PageDTO[ChatMessageVO]） |
| POST | /chat/session/create | 建会话（project_ulid, session_name? → ChatSessionVO） |
| POST | /chat/session/list | 会话列表（project_ulid → []ChatSessionVO） |
| POST | /chat/session/delete | 删会话（session_ulid，级联删消息） |

### POST /chat/send

请求（ChatSendREQ）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| session_ulid | string | 空则自动创建会话 |
| project_ulid | string | 必填 |
| content_text | string | 消息正文（可带 `/命令` 与 @引用） |
| skill_code | string | 直接指定技能（优先于正文解析） |
| ref_doc_ulid_list | []string | @引用文档 |
| ref_chapter_ulid_list | []string | @引用章节 |
| ref_message_ulid_list | []string | 显式引用历史消息 |
| history_count | int | 历史注入条数，0 用全局配置（默认 5） |

SSE 事件体（ChatStreamEventDTO）：

```json
{ "seq": 1, "event_type": "chunk", "session_ulid": "SES_...", "message_ulid": "MSG_...",
  "content_text": "增量文本", "reference_list": [], "error_message": "" }
```

| event_type | 说明 |
| --- | --- |
| chunk | 增量文本（seq 单调递增） |
| done | 结束；`content_text` 为完整回复，`reference_list` 为 RAG 召回依据 |
| error | 失败；`error_message` 为原因 |

## 7. 斜杠命令 / 配置 / 总览

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /slash/list | 命令列表（?enabled_only=true 仅启用） |
| POST | /slash/save | 新建/更新（SlashCommandREQ） |
| POST | /slash/delete | 删除（command_ulid；内置不可删 → ERR_BIZ_FAILED） |
| POST | /slash/parse | 解析正文中的 `/命令 参数` → SlashParseDTO |
| GET | /setting/list | 配置列表 → []SettingVO |
| POST | /setting/update | 更新配置（setting_key, setting_value） |
| GET | /stats/overview?project_ulid= | 数据总览 → StatsOverviewVO |

内置技能（`builtin_flag=true`，不可删）：`/continue` 续写、`/outline` 大纲、`/polish` 润色、`/expand` 扩写、`/condense` 缩写、`/summary` 梗概、`/review` 审稿、`/name` 取名、`/setting` 设定问答。

SlashParseDTO：`{ "skill_code": "continue", "remainder_text": "以林澈视角", "placeholder_map": {...} }`；未命中命令时 `skill_code` 为空。

StatsOverviewVO：`project_count / total_word_count / today_word_delta / ai_word_total / trend_list[]`（近 30 天：`stat_date / word_delta / ai_word_delta`）。
