# 数据存储设计

## 1. 存储分工

NovelWorkbench 使用五种存储，各自承担明确职责：

| 存储 | 职责 | 权威级别 |
| --- | --- | --- |
| PostgreSQL 16 | 业务主数据、权限、统计、任务状态 | 权威源 |
| MinIO | 知识库文档原文、导出文件 | 权威源（文件） |
| Manticore | 知识切片全文索引 | 可重建投影 |
| Redis | JWT 黑名单、短时状态 | 易失缓存 |
| RabbitMQ | 知识库索引任务 | 传输通道 |

系统不接入向量模型，不使用 pgvector 存储业务向量。PostgreSQL 只存关系数据与文本；检索语义由关键词切词、查询构造、融合排序与 chat 模型重排承担。

## 2. 命名约定

| 对象 | 约定 | 示例 |
| --- | --- | --- |
| 表名 | 复数 snake_case | `projects`、`knowledge_chunks` |
| 主键 | 业务前缀 + 26 位 ULID | `PRJ_01HZX...`、`CHP_01HZX...` |
| 外键 | 以 `_ulid` 结尾 | `project_ulid`、`chapter_ulid` |
| 状态字段 | 以 `_code` 结尾 | `chapter_status_code` |
| 数量字段 | 以 `_count` 结尾 | `word_count` |
| 时间字段 | GORM `CreatedAt`/`UpdatedAt` 或毫秒时间戳 VO | `created_at_ms` |
| 布尔字段 | 以 `_flag` 结尾 | `is_default_flag` |

所有主键前缀集中在 `domain/common.go`：`USR_`、`PRJ_`、`VOL_`、`CHP_`、`SNP_`、`CHR_`、`LOR_`、`DOC_`、`CHK_`、`SES_`、`MSG_`、`SKL_`、`JOB_`、`ROL_`、`AUD_`、`MPR_`、`MUS_` 等。

## 3. 核心表

### 3.1 用户与权限

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `users` | `user_ulid`、`user_name`、`user_email`、`password_hash`、`user_status_code` | 用户账号 |
| `roles` | `role_ulid`、`role_code`、`permission_codes` | 角色与权限码集合 |
| `user_roles` | `user_ulid`、`role_ulid` | 用户角色关联 |
| `audit_logs` | `audit_ulid`、`operator_ulid`、`action_code`、`resource_text`、`created_at` | 写操作审计 |

管理员引导：用户表为空时用配置中的初始密码创建 admin 用户，之后不再覆盖。密码使用 bcrypt，API 永不返回哈希。

### 3.2 创作主数据

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `projects` | `project_ulid`、`owner_user_ulid`、`project_name`、`genre_code`、`style_text`、`synopsis_text`、`project_status_code` | 小说项目 |
| `volumes` | `volume_ulid`、`project_ulid`、`volume_name`、`volume_order_no`、`synopsis_text` | 卷 |
| `chapters` | `chapter_ulid`、`volume_ulid`、`project_ulid`、`chapter_name`、`chapter_order_no`、`content_text`、`word_count`、`chapter_status_code` | 章节 |
| `chapter_snapshots` | `snapshot_ulid`、`chapter_ulid`、`content_text`、`word_count`、`snapshot_reason` | 章节快照 |
| `characters` | `character_ulid`、`project_ulid`、`character_name`、`role_code`、`personality_text`、`goal_text`、`secret_text`、`voice_text`、`current_state_text` | 角色卡 |
| `character_relations` | `relation_ulid`、`project_ulid`、`from_character_ulid`、`to_character_ulid`、`relation_text` | 角色关系边 |
| `lore_entries` | `lore_ulid`、`project_ulid`、`lore_type_code`、`lore_name`、`content_text`、`sort_no` | 世界观设定 |
| `notes` | `note_ulid`、`project_ulid`、`note_title`、`content_html` | 创作笔记（不进 AI 上下文） |

项目是隔离边界：所有创作表都有 `project_ulid`，查询必须带项目条件。章节同时冗余 `project_ulid`，避免多表 join 才能鉴权。

字数统计：`chapters.word_count` 存放当前字数；`word_count_stats` 按项目与日期记录人工/AI 净增，支撑趋势图。

### 3.2.1 创作规划与连续性

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `outline_nodes` | `outline_ulid`、`project_ulid`、`parent_outline_ulid`、`node_type_code`、`node_title`、`summary_text`、`sort_order` | 大纲树节点 |
| `detailed_scenes` | `scene_ulid`、`outline_ulid`、`project_ulid`、`character_ulid_list`、`location_text`、`conflict_text`、`pace_code`、`entry_state_text`、`goal_text`、`key_choice_text`、`outcome_text`、`exit_state_text` | 场景卡 |
| `temporal_facts` | `fact_ulid`、`project_ulid`、`subject_text`、`predicate_code`、`object_text`、`fact_status_code`、`valid_from_chapter_ulid`、`source_chapter_ulid`、`source_quote_text` | 时序事实账本 |
| `foreshadow_hooks` | `hook_ulid`、`project_ulid`、`hook_type_code`、`description_text`、`expected_payoff_text`、`setup_chapter_ulid`、`payoff_chapter_ulid`、`importance_no` | 伏笔跟踪 |
| `chapter_summaries` | `summary_ulid`、`project_ulid`、`chapter_ulid`、`summary_text`、`source_hash`、`summary_status_code` | 章节摘要与失效标记 |

以上表全部带 `project_ulid`，查询与写入都由 service 做项目归属校验。事实与伏笔状态采用显式审计字段，AI 产出默认只生成候选，不直接覆盖正式账本。

### 3.3 AI 与技能

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `model_providers` | `provider_ulid`、`provider_name`、`protocol_code`、`base_url`、`api_key_cipher`、`chat_model_name`、`is_default_flag`、`enabled_flag` | 模型供应商 |
| `model_usage_stats` | `usage_ulid`、`provider_ulid`、`model_name`、`call_type_code`、`call_count`、`token_count`、`success_count`、`avg_latency_ms`、`stat_date` | 用量日聚合 |
| `chat_sessions` | `session_ulid`、`project_ulid`、`user_ulid`、`session_name` | AI 会话 |
| `chat_messages` | `message_ulid`、`session_ulid`、`role_code`、`content_text`、`reference_json`、`token_estimate` | 会话消息 |
| `slash_commands` | `command_ulid`、`command_code`、`prompt_template`、`builtin_flag`、`enabled_flag` | 斜杠技能 |

密钥使用 AES-256-GCM 加密，主密钥来自环境变量；API 只返回掩码。供应商按 `protocol_code` 分为 `openai` 与 `anthropic`，支持任意兼容网关。

### 3.4 知识库与任务

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `knowledge_documents` | `doc_ulid`、`project_ulid`、`doc_name`、`object_key`、`file_size`、`index_status_code`、`chunk_count`、`error_message` | 文档与索引状态 |
| `knowledge_chunks` | `chunk_ulid`、`doc_ulid`、`project_ulid`、`chunk_index`、`chunk_text`、`token_count`、`content_hash` | 文档切片 |
| `jobs` | `job_ulid`、`job_type_code`、`payload_json`、`job_status_code`、`retry_count`、`error_message` | 异步任务留痕 |

索引状态机：`pending → indexing → indexed`；失败为 `failed`，可通过 `reindex` 重新投递。切片内容哈希用于判断是否需要重建 Manticore 索引。

## 4. 非关系存储

### 4.1 MinIO

桶由配置 `minio.bucket` 指定，对象键使用 `doc_ulid` 作为前缀，避免文件名冲突：

```
knowledge/<doc_ulid>/<original_file_name>
```

上传接口先写对象存储，再落库；删除接口同时清理对象与索引。原文预览支持 txt/md 直接渲染，docx/pdf 通过前端组件渲染。

### 4.2 Manticore

检索表由 `manticore.table_chunks` 指定，HTTP JSON API 写索引与查询。核心字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `chunk_ulid` | string | 切片 ULID |
| `doc_ulid` | string | 文档 ULID |
| `project_ulid` | string | 项目 ULID |
| `chunk_text` | text | 切片文本（CJK ngram） |
| `keyword_text` | string | 抽取关键词 |
| `content_hash` | string | 内容哈希 |

中文检索使用 ngram（`ngram_len=1`、`ngram_chars=cjk`），使单字与二字词都能召回。检索结果只作为候选，权威文本从 PostgreSQL 读取。

### 4.3 Redis

- `nw:token:blacklist:<sha256(token)>`：登出令牌黑名单，TTL 为令牌剩余有效期。
- 其他短时状态只在确有收益时使用；不做持久化业务数据。

### 4.4 RabbitMQ

队列 `nw_doc_index` 承载知识库索引任务。消息体是 `KnowledgeIndexJobDTO`，包含文档 ULID、项目 ULID、操作类型与重试次数。消费者失败时更新 `jobs` 与文档状态，最多重试 3 次；毒消息记录原因后确认，避免无限阻塞。

## 5. 事务与一致性

| 场景 | 策略 |
| --- | --- |
| 项目删除 | 软删除或级联删除由 service 明确控制，避免误删其他项目 |
| 章节更新 | 先写正文与字数，再按需写统计；统计失败只告警 |
| 文档上传 | MinIO 写成功再落库；落库失败删除已写对象 |
| 索引任务 | 先写 jobs，再投递 MQ；投递失败标记任务失败 |
| AI 消息 | 流式结束或取消后落库，避免半成品消息污染历史 |
| 用量统计 | 内存缓冲 5 秒批量落库，进程退出前 flush |

## 6. 索引与查询约束

- 所有业务查询必须携带 `project_ulid` 或可推导的项目上下文。
- 列表查询统一分页：`page_num` 从 1 开始，`page_size` 默认 20，最大 100。
- 模糊查询不做全表扫描优先：角色、设定、知识库都有项目范围与关键词条件。
- 时间排序使用 ULID 的时间有序性或 `created_at` 索引。
- Manticore 索引可以整体删除重建；重建期间检索降级为数据库文本匹配（数据量小），不阻塞写作。

## 7. 数据迁移

启动时通过 GORM `AutoMigrate` 创建/更新表结构；生产变更按顺序追加，不静默删列。新增字段必须可空或有默认值，保证旧数据可用。结构变化涉及检索字段时，必须提供 reindex 入口重建 Manticore 投影。

## 8. 备份与恢复

- PostgreSQL：定期逻辑备份（`pg_dump`），保留项目主数据与任务状态。
- MinIO：桶版本控制或定期同步；文档原文是业务资产，必须与数据库一起备份。
- Manticore、Redis：视为可重建/易失数据，不纳入主备份。
- 恢复顺序：PostgreSQL → MinIO → 触发知识库 reindex → 校验运行状态页。
