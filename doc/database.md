# 数据存储设计

本文描述 NovelWorkbench 当前的存储分工、表结构、索引与一致性策略。
表名与字段名与 `backend/internal/domain` 中的 GORM 标签一一对应。

## 1. 存储分工

| 存储 | 职责 | 权威级别 |
| --- | --- | --- |
| PostgreSQL 16 | 业务主数据、权限、统计、任务状态 | 权威源 |
| MinIO | 知识库文档原文 | 权威源（文件） |
| Manticore | 知识切片全文索引 | 可重建投影 |
| Redis 7 | 登出令牌黑名单 | 易失缓存 |
| RabbitMQ 4 | 知识库索引任务 | 传输通道 |

系统不接入向量模型，不使用 pgvector 存储业务向量。PostgreSQL 只存关系数据与文本；
检索语义由查询词抽取、通道融合排序与 chat 模型重排承担。

## 2. 命名约定

| 对象 | 约定 | 示例 |
| --- | --- | --- |
| 表名 | 复数 snake_case | `projects`、`knowledge_chunks` |
| 主键 | 业务前缀 + 26 位 ULID | `PRJ_01HZX...`、`CHP_01HZX...` |
| 外键 | 以 `_ulid` 结尾 | `project_ulid`、`chapter_ulid` |
| 状态字段 | 以 `_code` 结尾 | `chapter_status`、`fact_status_code` |
| 计数/数量 | 以 `_count` 结尾 | `word_count`、`call_count` |
| 文本字段 | 以 `_text` 结尾 | `content_text`、`title_text` |
| 时间字段 | GORM `CreatedAt`/`UpdatedAt` 或 VO 毫秒时间戳 | `created_at_ms` |
| 布尔字段 | 以 `_flag` 结尾 | `enabled_flag`、`default_flag` |

主键前缀集中在 `domain/common.go` 与各业务域的 `IDPrefix*` 常量：

`USR_` 用户、`PRJ_` 项目、`VOL_` 卷、`CHP_` 章节、`SNP_` 章节快照、`CHR_` 角色、
`RLT_` 角色关系、`LOR_` 设定、`OUT_` 大纲节点、`SCN_` 场景卡、`FCT_` 时间事实、
`HOK_` 伏笔钩子、`CSM_` 章节摘要、`DOC_` 文档、`CHK_` 切片、`SES_` 会话、`MSG_` 消息、
`SKL_` 斜杠命令、`JOB_` 任务、`SET_` 应用配置、`WCS_` 字数统计、`ROL_` 角色、
`URO_` 用户角色关联、`AUD_` 审计日志、`MPR_` 模型供应商、`MUS_` 用量统计。

## 3. 核心表

### 3.1 用户与权限

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `users` | `user_ulid`、`user_name`、`user_email`、`password_hash`、`avatar_url`、`remark_text`、`user_status_code` | 用户账号 |
| `roles` | `role_ulid`、`role_code`、`role_name`、`remark_text`、`perm_codes_json`、`builtin_flag` | 角色与权限码集合 |
| `user_roles` | `user_role_ulid`、`user_ulid`、`role_ulid` | 用户角色关联 |
| `audit_logs` | `audit_ulid`、`user_ulid`、`user_name`、`action_code`、`target_type`、`target_ulid`、`detail_text`、`ip_addr` | 写操作审计 |

`perm_codes_json` 存权限码 JSON 数组，`"*"` 表示全部权限（内置 admin 角色）。
`builtin_flag=true` 的角色不可删除。密码用 bcrypt 哈希，API 永不返回哈希。

管理员引导：用户表为空时用 `NW_APP_ADMIN_PASSWORD` 创建 admin 用户，之后不再覆盖。

### 3.2 创作主数据

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `projects` | `project_ulid`、`user_ulid`、`project_name`、`genre_code`、`style_text`、`synopsis_text`、`project_status`、`total_word_count` | 小说项目 |
| `volumes` | `volume_ulid`、`project_ulid`、`volume_name`、`volume_order`、`summary_text` | 卷 |
| `chapters` | `chapter_ulid`、`project_ulid`、`volume_ulid`、`chapter_name`、`chapter_order`、`content_text`、`summary_text`、`word_count`、`chapter_status` | 章节 |
| `chapter_snapshots` | `snapshot_ulid`、`chapter_ulid`、`content_text`、`snapshot_type`、`change_note` | 章节快照 |
| `characters` | `character_ulid`、`project_ulid`、`character_name`、`alias_text`、`role_type_code`、`personality_text`、`background_text`、`appearance_text`、`goal_text`、`sort_order` | 角色卡 |
| `character_relations` | `relation_ulid`、`project_ulid`、`from_character_ulid`、`to_character_ulid`、`relation_type_code`、`relation_note` | 角色关系边 |
| `lore_settings` | `lore_ulid`、`project_ulid`、`lore_name`、`lore_type_code`、`content_text`、`sort_order` | 世界观设定与创作笔记 |

项目是隔离边界：所有创作表都有 `project_ulid`，查询必须带项目条件。
章节表同时冗余 `project_ulid`，鉴权不必多表 join。

`lore_type_code` 取值：`world` 世界背景、`rule` 规则、`item` 物品、`place` 地点、
`event` 事件、`note` 创作笔记。创作笔记复用同一张表，固定为 `note` 类型，
不参与检索与 AI 上下文装配。

字数统计：`chapters.word_count` 存当前字数；`projects.total_word_count` 存项目累计字数；
`word_count_stats` 按项目与日期记录人工净增与 AI 生成量，支撑趋势图。

### 3.3 创作规划与连续性

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `outline_nodes` | `outline_ulid`、`project_ulid`、`parent_outline_ulid`、`node_type_code`、`title_text`、`summary_text`、`node_status_code`、`sort_order` | 大纲树节点 |
| `detailed_scenes` | `scene_ulid`、`project_ulid`、`outline_ulid`、`character_ulid_list`、`location_text`、`conflict_text`、`pace_code`、`estimated_words`、`entry_state_text`、`goal_text`、`key_choice_text`、`outcome_text`、`exit_state_text`、`sort_order` | 场景卡 |
| `temporal_facts` | `fact_ulid`、`project_ulid`、`subject_text`、`predicate_code`、`object_text`、`fact_status_code`、`valid_from_chapter_ulid`、`valid_until_chapter_ulid`、`source_chapter_ulid`、`source_quote_text` | 时序事实账本 |
| `foreshadow_hooks` | `hook_ulid`、`project_ulid`、`hook_status_code`、`description_text`、`expected_payoff_text`、`setup_chapter_ulid`、`payoff_chapter_ulid`、`importance_code` | 伏笔跟踪 |
| `chapter_summaries` | `summary_ulid`、`project_ulid`、`chapter_ulid`、`summary_text`、`source_hash_text`、`summary_status_code` | 章节摘要与失效标记 |

`outline_nodes.parent_outline_ulid` 存父节点 ULID，根节点为空串；`node_type_code` 取值
`volume`/`arc`/`storyBlock`/`chapter`。`detailed_scenes.character_ulid_list` 存角色 ULID 的 JSON 数组。

事实状态 `fact_status_code`：`candidate` 候选、`confirmed` 已确认、`superseded` 已被取代、
`stale` 已过期。复核只能推进到 confirmed/superseded/stale，不能退回 candidate。
钩子状态 `hook_status_code`：`planned`/`planted`/`echoed`/`resolved`/`abandoned`。
重要程度 `importance_code`：`high`/`medium`/`low`。
摘要状态 `summary_status_code`：`pending`/`verified`/`stale`。

### 3.4 AI 与技能

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `model_providers` | `provider_ulid`、`provider_name`、`protocol_code`、`base_url`、`api_key_cipher`、`chat_model_name`、`temperature`、`max_tokens`、`enabled_flag`、`default_flag`、`sort_order`、`last_test_status_code`、`last_test_message`、`last_test_at_ms` | 模型供应商 |
| `model_usage_stats` | `usage_stat_ulid`、`stat_date`、`provider_ulid`、`provider_name`、`model_name`、`call_type_code`、`call_count`、`success_count`、`fail_count`、`prompt_tokens`、`completion_tokens`、`total_tokens`、`latency_ms_sum` | 用量日聚合 |
| `chat_sessions` | `session_ulid`、`project_ulid`、`user_ulid`、`session_name` | AI 会话 |
| `chat_messages` | `message_ulid`、`session_ulid`、`role_code`、`content_text`、`skill_code`、`ref_doc_json`、`ref_chapter_json`、`ref_message_json`、`token_used_count` | 会话消息 |
| `slash_commands` | `command_ulid`、`command_code`、`command_name`、`prompt_template`、`builtin_flag`、`enabled_flag` | 斜杠技能 |

密钥使用 AES-256-GCM 加密（`api_key_cipher`），主密钥由 `NW_JWT_SECRET` 派生；
API 只返回掩码。供应商按 `protocol_code` 分为 `openai` 与 `anthropic`。
`default_flag=true` 的供应商全局唯一，由服务层在切换时保证。

`chat_messages` 的三组 `ref_*_json` 存 ULID 列表 JSON 数组，用于还原消息的
@引用文档、@引用章节与显式引用历史消息。

### 3.5 知识库与任务

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `knowledge_documents` | `doc_ulid`、`project_ulid`、`doc_name`、`doc_type_code`、`object_key`、`doc_status_code`、`chunk_count`、`total_token_count`、`error_message` | 文档与索引状态 |
| `knowledge_chunks` | `chunk_ulid`、`doc_ulid`、`project_ulid`、`chunk_index`、`chunk_text`、`token_count`、`content_hash` | 文档切片 |
| `jobs` | `job_ulid`、`job_type_code`、`payload_json`、`job_status_code`、`retry_count`、`error_message` | 异步任务留痕 |
| `app_settings` | `setting_ulid`、`setting_key`、`setting_value`、`setting_desc` | 运行时配置 |
| `word_count_stats` | `word_stat_ulid`、`project_ulid`、`stat_date`、`word_delta`、`ai_word_delta` | 每日字数统计 |

索引状态机：`pending → indexing → indexed`；失败为 `failed`，可通过 `reindex` 重新投递。
切片内容哈希（`content_hash`）用于判断是否需要重建 Manticore 索引。

`project_ulid='0'` 表示全局知识库文档，不属于任何具体项目。

## 4. 非关系存储

### 4.1 MinIO

桶由配置 `minio.bucket` 指定，对象键以 `doc_ulid` 为键名：

```
<doc_ulid>
```

上传接口先写对象存储再落库；落库失败时删除已写对象。删除文档时同时清理对象、切片、
Manticore 行与文档行。原文预览支持 txt/md 直接渲染，docx/pdf 通过前端组件渲染。

### 4.2 Manticore

检索表由 `manticore.table_chunks` 指定（默认 `nw_chunks`），通过 HTTP SQL API 写索引与查询。
核心字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `chunk_ulid` | string | 切片 ULID |
| `doc_ulid` | string | 文档 ULID |
| `project_ulid` | string | 项目 ULID |
| `chunk_text` | text | 切片文本（CJK ngram 索引） |

中文检索使用 `ngram_len='1'`、`ngram_chars='cjk'`，使单字与二字词都能召回。
启动时若检测到旧版表结构（不含 ngram 参数），自动 DROP 并重建，再从 PostgreSQL 回灌切片。
检索结果只作为候选，权威文本始终从 PostgreSQL 读取。

### 4.3 Redis

- `nw:token:blacklist:<sha256(token)>`：登出令牌黑名单，TTL 为令牌剩余有效期。
- 不存放持久化业务数据。

### 4.4 RabbitMQ

队列 `nw_doc_index`（配置项 `rabbitmq.queue_doc_index`）承载知识库索引任务。
消息体是 `KnowledgeIndexJobDTO`，包含文档 ULID。消费者失败时更新 `jobs` 与文档状态并重试，
毒消息记录原因后确认，避免无限阻塞。

## 5. 事务与一致性

| 场景 | 策略 |
| --- | --- |
| 项目删除 | 由 service 显式编排级联删除，先删子表再删主表 |
| 章节更新 | 事务内写正文与字数，并按差值写项目字数与当日统计 |
| 章节删除 | 事务内删快照、删章节、回补项目字数差 |
| 文档上传 | MinIO 写成功再落库；落库失败删除已写对象 |
| 索引任务 | 先写 `jobs` 再投递 MQ；投递失败标记任务失败 |
| AI 消息 | 助手消息与 AI 字数统计同事务落库；生成失败不落库 |
| 用量统计 | 内存缓冲 5 秒批量落库，进程退出前 flush |

## 6. 索引与查询约束

- 所有业务查询必须携带 `project_ulid` 或可推导的项目上下文。
- 列表查询统一分页：`page_num` 从 1 开始，`page_size` 默认 20，最大 100。
- 模糊查询都有项目范围与关键词条件，不做全表扫描优先。
- 时间排序使用 `created_at` 索引；ULID 本身时间有序，可用于顺序取用。
- Manticore 索引可整体删除重建，重建期间检索降级，不阻塞写作。

## 7. 数据迁移

启动时通过 GORM `AutoMigrate` 创建/更新表结构；生产变更按顺序追加，不静默删列。
新增字段必须可空或有默认值，保证旧数据可用。结构变化涉及检索字段时，
必须提供 reindex 入口重建 Manticore 投影。

## 8. 备份与恢复

- PostgreSQL：定期逻辑备份（`pg_dump`），保留项目主数据与任务状态。
- MinIO：桶版本控制或定期同步；文档原文是业务资产，必须与数据库一起备份。
- Manticore、Redis：视为可重建/易失数据，不纳入主备份。
- 恢复顺序：PostgreSQL → MinIO → 触发知识库 reindex → 校验运行状态页。
