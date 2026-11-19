# 数据存储设计

本文描述 NovelWorkbench 当前的存储分工、表结构、枚举取值、索引与一致性策略。
所有表名、列名、类型均取自 `backend/internal/domain/*.go` 的 GORM 标签，经 GORM 命名策略
（`backend/internal/infra/postgres.go` 的 `ULIDNamingStrategy`）归一后与实际 DDL 一致。

数据库结构由 `cmd/server/main.go` 的 `db.AutoMigrate(...)` 一次性声明 28 张表，
**没有 SQL 迁移文件**，也没有外键、软删除与数据库级级联。

## 1. 存储分工

| 存储 | 职责 | 权威级别 | 缺失时行为 |
| --- | --- | --- | --- |
| PostgreSQL 16 | 业务主数据、权限、审计、统计、任务留痕 | 权威源 | 启动即退出（`main.go` 中 Postgres 连接失败 `os.Exit(1)`） |
| MinIO | 知识库文档原文二进制 | 权威源（文件内容） | 降级：上传/读取报 `ERR_STORAGE` |
| Manticore | 知识切片全文索引 | 可重建投影 | 降级：keyword 通道不可用，检索走其他通道 |
| Redis 7 | 登出令牌黑名单 | 易失缓存 | 降级：吊销校验放行，日志兜底 |
| RabbitMQ 4 | 知识库索引任务投递 | 传输通道（非权威） | 降级：异步任务不可用 |

各依赖在 `cmd/server/main.go` 中的初始化顺序为：Postgres → AutoMigrate → Redis → RabbitMQ →
MinIO → Manticore → JWT 签名器。Redis / RabbitMQ / MinIO / Manticore 的连接失败只打印告警并降级，
不阻断启动。

## 2. 命名约定

| 对象 | 约定 | 示例 |
| --- | --- | --- |
| 表名 | snake_case 复数，由 `TableName()` 显式指定 | `projects`、`knowledge_chunks`、`lore_settings` |
| 主键 | 业务前缀 + 26 位大写 ULID | `PRJ_01HZX...`、`CHP_01HZX...` |
| 引用列 | `_ulid` 结尾 | `project_ulid`、`chapter_ulid` |
| 状态/枚举列 | `_code` 结尾 | `chapter_status`、`fact_status_code`、`call_type_code` |
| 计数字段 | `_count` 结尾 | `word_count`、`chunk_count`、`call_count` |
| 文本正文列 | `_text` 结尾 | `content_text`、`title_text` |
| 布尔字段 | `_flag` 结尾 | `enabled_flag`、`default_flag`、`builtin_flag` |
| VO 中的时间戳 | `_ms` 结尾（毫秒） | `created_at_ms`、`updated_at_ms` |
| 长文本列 | `type:text` | `chapters.content_text`、`roles.perm_codes_json` |

GORM 默认把 `XxxULID` 推导为 `xxx_ul_id`。`ULIDNamingStrategy.ColumnName` 把以 `_ul_id` 结尾的
推导名改写为 `_ulid`（`backend/internal/infra/postgres.go`）。

### 2.1 ULID 业务前缀

前缀常量定义在 `backend/internal/domain/common.go`、`continuity.go`、`outline.go`。

| 前缀 | 含义 | 常量 |
| --- | --- | --- |
| `USR_` | 用户 | `IDPrefixUser` |
| `REQ_` | 请求链路 ID | `IDPrefixRequest` |
| `PRJ_` | 小说项目 | `IDPrefixProject` |
| `VOL_` | 卷 | `IDPrefixVolume` |
| `CHP_` | 章节 | `IDPrefixChapter` |
| `SNP_` | 章节快照 | `IDPrefixChapterSnapshot` |
| `CHR_` | 角色 | `IDPrefixCharacter` |
| `RLT_` | 角色关系 | `IDPrefixRelation` |
| `LOR_` | 世界观设定 | `IDPrefixLore` |
| `DOC_` | 知识库文档 | `IDPrefixDocument` |
| `CHK_` | 知识库切片 | `IDPrefixChunk` |
| `SES_` | 会话 | `IDPrefixSession` |
| `MSG_` | 消息 | `IDPrefixMessage` |
| `SKL_` | 技能 | `IDPrefixSkill` |
| `SKR_` | 技能参考资料 | `IDPrefixSkillReference` |
| `MNU_` | 菜单 | `IDPrefixMenu` |
| `JOB_` | 异步任务 | `IDPrefixJob` |
| `SET_` | 应用配置 | `IDPrefixSetting` |
| `WCS_` | 字数统计 | `IDPrefixWordStat` |
| `ROL_` | 角色（权限） | `IDPrefixRole` |
| `URO_` | 用户-角色关联 | `IDPrefixUserRole` |
| `AUD_` | 审计日志 | `IDPrefixAudit` |
| `MPR_` | 模型供应商 | `IDPrefixModelProvider` |
| `MUS_` | 模型用量统计 | `IDPrefixModelUsage` |
| `OUT_` | 大纲节点 | `IDPrefixOutline` |
| `SCN_` | 详细场景卡片 | `IDPrefixScene` |
| `FCT_` | 时间事实 | `IDPrefixTemporalFact` |
| `HOK_` | 伏笔钩子 | `IDPrefixForeshadowHook` |
| `CSM_` | 章节摘要 | `IDPrefixChapterSummary` |

## 3. 表结构

所有表统一含 `created_at timestamptz`（`autoCreateTime`）与 `updated_at timestamptz`（`autoUpdateTime`），
来自 `domain.BaseDO`。除显式标注外，所有列均为 `NOT NULL`。

### 3.1 用户、权限与审计

#### users

用户账号。密码只存 bcrypt 哈希，`password_hash` 不出现在任何 VO。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `user_ulid` | `varchar(40)` | PK |
| `user_name` | `varchar(64)` | UNIQUE |
| `user_email` | `varchar(128)` | UNIQUE |
| `password_hash` | `varchar(128)` | |
| `avatar_url` | `varchar(256)` | |
| `remark_text` | `varchar(256)` | |
| `user_status_code` | `varchar(16)` | `DEFAULT 'active'`，索引 |

索引：`idx_users_user_name`(UNIQUE)、`idx_users_user_email`(UNIQUE)、`idx_users_user_status_code`。

#### roles

角色与权限码集合。`perm_codes_json` 存 JSON 数组，`"*"` 表示全部权限。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `role_ulid` | `varchar(40)` | PK |
| `role_code` | `varchar(64)` | UNIQUE |
| `role_name` | `varchar(64)` | |
| `remark_text` | `varchar(256)` | |
| `perm_codes_json` | `text` | JSON 数组，空列表存 `"[]"` |
| `builtin_flag` | `boolean` | `DEFAULT false`，内置角色为 true 且不可删除 |

索引：`idx_roles_role_code`(UNIQUE)。

#### user_roles

用户-角色关联，无唯一约束（同一人同一角色可重复写入）。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `user_role_ulid` | `varchar(40)` | PK |
| `user_ulid` | `varchar(40)` | 索引 |
| `role_ulid` | `varchar(40)` | 索引 |

索引：`idx_user_roles_user_ul_id`、`idx_user_roles_role_ul_id`。
关联写入走 `RoleRepo.ReplaceUserRoles`（先删后插，单事务）。

#### menus

菜单表，同时是权限码登记处。`page` 行承载页面权限码，`action` 行只登记动作权限码（不进导航）。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `menu_ulid` | `varchar(40)` | PK |
| `parent_ulid` | `varchar(40)` | 索引，空为根节点 |
| `menu_code` | `varchar(64)` | UNIQUE |
| `menu_name` | `varchar(64)` | |
| `menu_type_code` | `varchar(16)` | group / page / action |
| `scope_code` | `varchar(16)` | global / project，索引 |
| `route_path` | `varchar(128)` | 项目内以 `:project_ulid` 结尾 |
| `icon_code` | `varchar(32)` | 前端图标表键 |
| `perm_code` | `varchar(64)` | 索引，空表示登录可见 |
| `visible_flag` | `boolean` | `DEFAULT true` |
| `enabled_flag` | `boolean` | `DEFAULT true` |
| `sort_order` | `bigint` | 同级升序 |
| `remark_text` | `varchar(256)` | |

索引：`idx_menus_menu_code`(UNIQUE)、`idx_menus_parent_ul_id`、`idx_menus_scope_code`、`idx_menus_perm_code`。
服务层保证 `perm_code` 全局唯一（同一权限码只能登记一次），数据库不做唯一约束以允许空值多行。

#### audit_logs

写操作审计记录。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `audit_ulid` | `varchar(40)` | PK |
| `user_ulid` | `varchar(40)` | 索引（未登录为空串） |
| `user_name` | `varchar(64)` | 索引 |
| `action_code` | `varchar(128)` | 索引，接口路径去前缀 |
| `target_type` | `varchar(32)` | |
| `target_ulid` | `varchar(40)` | 索引 |
| `detail_text` | `varchar(512)` | |
| `ip_addr` | `varchar(64)` | |

索引：`idx_audit_logs_user_ul_id`、`idx_audit_logs_user_name`、`idx_audit_logs_action_code`、
`idx_audit_logs_target_ul_id`。审计表**只增不删**，没有清理路径。

### 3.2 创作主数据

#### projects

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `project_ulid` | `varchar(40)` | PK |
| `user_ulid` | `varchar(40)` | 索引，归属用户 |
| `project_name` | `varchar(128)` | |
| `genre_code` | `varchar(32)` | 题材编码，无白名单校验 |
| `style_text` | `varchar(1024)` | |
| `synopsis_text` | `varchar(4096)` | |
| `project_status` | `varchar(16)` | 索引 |
| `total_word_count` | `bigint` | 由章节字数差增减维护 |

索引：`idx_projects_user_ul_id`、`idx_projects_project_status`。

#### volumes

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `volume_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `volume_name` | `varchar(128)` | |
| `volume_order` | `bigint` | 排序号 |
| `summary_text` | `varchar(2048)` | |

索引：`idx_volumes_project_ul_id`。

#### chapters

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `chapter_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `volume_ulid` | `varchar(40)` | 索引 |
| `chapter_name` | `varchar(128)` | |
| `chapter_order` | `bigint` | 索引，项目内最大值 +1 |
| `content_text` | `text` | 正文 |
| `summary_text` | `varchar(2048)` | |
| `word_count` | `bigint` | 正文计数（`pkg.CountWords`） |
| `chapter_status` | `varchar(16)` | |

索引：`idx_chapters_project_ul_id`、`idx_chapters_volume_ul_id`、`idx_chapters_chapter_order`。

#### chapter_snapshots

章节历史留档，可回滚。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `snapshot_ulid` | `varchar(40)` | PK |
| `chapter_ulid` | `varchar(40)` | 索引 |
| `content_text` | `text` | 快照正文 |
| `snapshot_type` | `varchar(16)` | 代码中只会写入 `"manual"` |
| `change_note` | `varchar(256)` | |

索引：`idx_chapter_snapshots_chapter_ul_id`。

> `snapshot_type` 的领域注释列出 `manual/ai_before/ai_after`，但当前代码只有
> `ChapterService.SnapshotCreate` 与 `SnapshotRestore` 写 `"manual"`，不存在写入 AI 前后快照的路径。

#### characters

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `character_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `character_name` | `varchar(64)` | |
| `alias_text` | `varchar(128)` | |
| `role_type_code` | `varchar(16)` | |
| `personality_text` | `varchar(2048)` | |
| `background_text` | `varchar(4096)` | |
| `appearance_text` | `varchar(2048)` | |
| `goal_text` | `varchar(2048)` | |

索引：`idx_characters_project_ul_id`。

#### character_relations

角色关系边（图谱数据）。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `relation_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `from_character_ulid` | `varchar(40)` | 索引 |
| `to_character_ulid` | `varchar(40)` | 索引 |
| `relation_type_code` | `varchar(32)` | 自由文本，无白名单 |
| `relation_note` | `varchar(256)` | |

索引：`idx_character_relations_project_ul_id`、`idx_character_relations_from_character_ul_id`、
`idx_character_relations_to_character_ul_id`。

#### lore_settings

世界观设定与创作笔记共用一张表，`lore_type_code = 'note'` 表示创作笔记。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `lore_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `lore_name` | `varchar(128)` | |
| `lore_type_code` | `varchar(16)` | 索引 |
| `content_text` | `text` | |
| `sort_order` | `bigint` | |

索引：`idx_lore_settings_project_ul_id`、`idx_lore_settings_lore_type_code`。

### 3.3 创作规划与连续性

#### outline_nodes

树形大纲节点（卷/篇章/情节块/章节）。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `outline_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `parent_outline_ulid` | `varchar(40)` | 索引，根节点为空串（`DEFAULT ''`） |
| `node_type_code` | `varchar(32)` | 索引 |
| `title_text` | `varchar(128)` | |
| `summary_text` | `varchar(4096)` | |
| `node_status_code` | `varchar(16)` | 索引，新建缺省 `draft` |
| `sort_order` | `bigint` | 索引，同级升序 |

索引：`idx_outline_nodes_project_ul_id`、`idx_outline_nodes_parent_outline_ul_id`、
`idx_outline_nodes_node_type_code`、`idx_outline_nodes_node_status_code`、`idx_outline_nodes_sort_order`。

#### detailed_scenes

挂靠在大纲节点下的场景卡片。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `scene_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `outline_ulid` | `varchar(40)` | 索引 |
| `character_ulid_list` | `text` | 出场角色 ULID 的 JSON 数组 |
| `location_text` | `varchar(256)` | |
| `conflict_text` | `text` | |
| `pace_code` | `varchar(16)` | 索引 |
| `estimated_words` | `bigint` | |
| `entry_state_text` | `text` | |
| `goal_text` | `text` | |
| `key_choice_text` | `text` | |
| `outcome_text` | `text` | |
| `exit_state_text` | `text` | |
| `sort_order` | `bigint` | 索引 |

索引：`idx_detailed_scenes_project_ul_id`、`idx_detailed_scenes_outline_ul_id`、
`idx_detailed_scenes_pace_code`、`idx_detailed_scenes_sort_order`。

#### temporal_facts

带有效区间的时序事实。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `fact_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `subject_text` | `varchar(128)` | 主体 |
| `predicate_code` | `varchar(64)` | 索引 |
| `object_text` | `varchar(256)` | 客体 |
| `fact_status_code` | `varchar(16)` | 索引 |
| `valid_from_chapter_ulid` | `varchar(40)` | 生效起始章节 |
| `valid_until_chapter_ulid` | `varchar(40)` | 生效截止章节 |
| `source_chapter_ulid` | `varchar(40)` | 索引，证据来源章节 |
| `source_quote_text` | `text` | 原文证据 |
| `note_text` | `text` | |

索引：`idx_temporal_facts_project_ul_id`、`idx_temporal_facts_predicate_code`、
`idx_temporal_facts_fact_status_code`、`idx_temporal_facts_source_chapter_ul_id`。

#### foreshadow_hooks

伏笔钩子全过程跟踪。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `hook_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `hook_status_code` | `varchar(16)` | 索引 |
| `description_text` | `text` | |
| `expected_payoff_text` | `text` | |
| `setup_chapter_ulid` | `varchar(40)` | 索引 |
| `payoff_chapter_ulid` | `varchar(40)` | 索引 |
| `importance_code` | `varchar(16)` | 索引 |
| `note_text` | `text` | |

索引：`idx_foreshadow_hooks_project_ul_id`、`idx_foreshadow_hooks_hook_status_code`、
`idx_foreshadow_hooks_setup_chapter_ul_id`、`idx_foreshadow_hooks_payoff_chapter_ul_id`、
`idx_foreshadow_hooks_importance_code`。

#### chapter_summaries

章节级摘要，供连续性核对。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `summary_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `chapter_ulid` | `varchar(40)` | 索引 |
| `summary_text` | `text` | |
| `source_hash_text` | `varchar(128)` | 正文指纹 |
| `summary_status_code` | `varchar(16)` | 索引 |

索引：`idx_chapter_summaries_project_ul_id`、`idx_chapter_summaries_chapter_ul_id`、
`idx_chapter_summaries_summary_status_code`。

### 3.4 知识库

#### knowledge_documents

文档元数据；原文存 MinIO，索引存 PG 与 Manticore。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `doc_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引，`"0"` 表示全局库 |
| `doc_name` | `varchar(128)` | |
| `doc_type_code` | `varchar(16)` | 由文件扩展名推导 |
| `object_key` | `varchar(256)` | MinIO 对象键，等于 `doc_ulid` |
| `doc_status_code` | `varchar(16)` | 索引 |
| `chunk_count` | `bigint` | 切片数量 |
| `total_token_count` | `bigint` | 估算 token 总量 |
| `error_message` | `varchar(512)` | 失败原因 |

索引：`idx_knowledge_documents_project_ul_id`、`idx_knowledge_documents_doc_status_code`。

#### knowledge_chunks

文档切片原文与内容哈希。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `chunk_ulid` | `varchar(40)` | PK |
| `doc_ulid` | `varchar(40)` | 索引 |
| `project_ulid` | `varchar(40)` | 索引 |
| `chunk_index` | `bigint` | 文档内升序序号 |
| `chunk_text` | `text` | 切片文本 |
| `token_count` | `bigint` | 估算 token 数 |
| `content_hash` | `varchar(64)` | 索引，SHA-256 十六进制 |

索引：`idx_knowledge_chunks_doc_ul_id`、`idx_knowledge_chunks_project_ul_id`、
`idx_knowledge_chunks_content_hash`。

切片算法（`KnowledgeWorker.chunkText`，参数来自 `app.chunk_size` / `app.chunk_overlap`，
默认 800 / 120）：

| 步骤 | 行为 |
| --- | --- |
| 预处理 | `TrimSpace`；空文本返回 nil（触发「文档内容为空」错误） |
| 窗口 | 按 rune 滑窗，窗口长度 `chunkSize` 字符 |
| 边界 | 窗口内若存在换行或 `。！？；!?;` 且位置 > `chunkSize/2`，则在最后一个边界处切分 |
| 步进 | 下一窗口起点 = `end - overlap`（`overlap >= chunkSize` 时收敛为 `chunkSize-1`） |
| 哈希 | `content_hash = hex(sha256(切片文本))` |
| ULID 复用 | 新切片哈希命中该文档既有切片的 `content_hash` 且未被占用时，沿用旧 `chunk_ulid`；否则新生成 `CHK_` ULID |

文本提取（`extractDocumentText`）：`txt`/`md` 直读并剥除 UTF-8 BOM；`docx` 解析 zip 内
`word/document.xml`（段落标记转行、去标签、还原 XML 实体）；`pdf` 用 `ledongthuc/pdf` 取纯文本；
其他类型返回错误。

### 3.5 对话与技能

#### chat_sessions

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `session_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `user_ulid` | `varchar(40)` | 索引 |
| `session_name` | `varchar(128)` | 默认取首条消息摘要 |

索引：`idx_chat_sessions_project_ul_id`、`idx_chat_sessions_user_ul_id`。

#### chat_messages

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `message_ulid` | `varchar(40)` | PK |
| `session_ulid` | `varchar(40)` | 索引 |
| `role_code` | `varchar(16)` | user/assistant/system |
| `content_text` | `text` | |
| `skill_code` | `varchar(32)` | 命中的技能编码，空为自由对话 |
| `ref_doc_json` | `text` | @引用文档 ULID JSON 数组 |
| `ref_chapter_json` | `text` | @引用章节 ULID JSON 数组 |
| `ref_message_json` | `text` | 显式引用历史消息 ULID JSON 数组 |
| `token_used_count` | `bigint` | 估算 token |

索引：`idx_chat_messages_session_ul_id`。

#### skills

技能资产：带作用域的提示词模板。全局技能（`scope_code=global`）与项目技能（`scope_code=project`）
共存，同编码项目技能覆盖全局技能。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `skill_ulid` | `varchar(40)` | PK（`SKL_` 前缀） |
| `skill_code` | `varchar(32)` | 索引，作用域内唯一 |
| `skill_name` | `varchar(64)` | |
| `scope_code` | `varchar(16)` | global / project，索引，默认 global |
| `project_ulid` | `varchar(40)` | 索引，全局技能为空串 |
| `category_code` | `varchar(16)` | write / revise / outline / setting / review / other |
| `description_text` | `varchar(256)` | |
| `prompt_template` | `text` | `{{占位符}}` 模板 |
| `variable_spec_json` | `text` | 变量声明 JSON 数组，空为 `"[]"` |
| `enabled_flag` | `boolean` | `DEFAULT true` |
| `builtin_flag` | `boolean` | `DEFAULT false`，内置不可删 |
| `sort_order` | `bigint` | |

索引：`idx_skills_skill_code`、`idx_skills_scope_code`、`idx_skills_project_ul_id`。
作用域内编码唯一由服务层保证（查询 `(scope_code, project_ulid, skill_code)` 后再写）。

#### skill_references

技能参考资料：技能命中后按关键词挑选注入上下文，空关键词表示总是注入。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `ref_ulid` | `varchar(40)` | PK（`SKR_` 前缀） |
| `skill_ulid` | `varchar(40)` | 索引 |
| `ref_name` | `varchar(64)` | |
| `ref_text` | `text` | |
| `keyword_list` | `varchar(256)` | 逗号分隔，空表示总是注入 |
| `sort_order` | `bigint` | 注入顺序 |

索引：`idx_skill_references_skill_ul_id`。技能删除时级联清理（`SkillRepo.DeleteReferencesBySkill`）。

### 3.6 模型供应商与用量

#### model_providers

一个供应商 = 一套网关地址 + 凭据 + 对话模型名。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `provider_ulid` | `varchar(40)` | PK |
| `provider_name` | `varchar(64)` | |
| `protocol_code` | `varchar(16)` | 索引 |
| `base_url` | `varchar(256)` | |
| `api_key_cipher` | `varchar(512)` | AES-256-GCM 密文，不返回给前端 |
| `chat_model_name` | `varchar(128)` | |
| `temperature` | `decimal` | 采样温度 |
| `max_tokens` | `bigint` | |
| `enabled_flag` | `boolean` | `DEFAULT true` |
| `default_flag` | `boolean` | `DEFAULT false`，索引；全局唯一由服务层保证 |
| `sort_order` | `bigint` | |
| `remark_text` | `varchar(256)` | |
| `last_test_status_code` | `varchar(16)` | |
| `last_test_message` | `varchar(512)` | |
| `last_test_at_ms` | `bigint` | 毫秒时间戳 |

索引：`idx_model_providers_protocol_code`、`idx_model_providers_default_flag`。

#### model_usage_stats

按日聚合的模型用量。唯一维度为 `(stat_date, provider_ulid, model_name, call_type_code)`。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `usage_stat_ulid` | `varchar(40)` | PK |
| `stat_date` | `varchar(10)` | UNIQUE 组合列，`YYYY-MM-DD` |
| `provider_ulid` | `varchar(40)` | UNIQUE 组合列 |
| `provider_name` | `varchar(64)` | 供应商名称快照 |
| `model_name` | `varchar(128)` | UNIQUE 组合列 |
| `call_type_code` | `varchar(16)` | UNIQUE 组合列 |
| `call_count` | `bigint` | |
| `success_count` | `bigint` | |
| `fail_count` | `bigint` | |
| `prompt_tokens` | `bigint` | |
| `completion_tokens` | `bigint` | |
| `total_tokens` | `bigint` | |
| `latency_ms_sum` | `bigint` | 耗时合计，算均值用 |

索引：`uk_mus_dim`(UNIQUE, 四列组合)。

### 3.7 运维

#### jobs

RabbitMQ 消费的落库凭据。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `job_ulid` | `varchar(40)` | PK |
| `job_type_code` | `varchar(32)` | 索引 |
| `payload_json` | `text` | `KnowledgeIndexJobDTO` 序列化 |
| `job_status_code` | `varchar(16)` | 索引 |
| `retry_count` | `bigint` | |
| `error_message` | `varchar(512)` | |

索引：`idx_jobs_job_type_code`、`idx_jobs_job_status_code`。

#### app_settings

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `setting_ulid` | `varchar(40)` | PK |
| `setting_key` | `varchar(64)` | UNIQUE |
| `setting_value` | `varchar(512)` | |
| `setting_desc` | `varchar(256)` | |

索引：`idx_app_settings_setting_key`(UNIQUE)。

#### word_count_stats

每日字数统计。

| 列 | 类型 | 约束 |
| --- | --- | --- |
| `word_stat_ulid` | `varchar(40)` | PK |
| `project_ulid` | `varchar(40)` | 索引 |
| `stat_date` | `varchar(10)` | 索引，`YYYY-MM-DD` |
| `word_delta` | `bigint` | 人工净增字数 |
| `ai_word_delta` | `bigint` | AI 生成字数 |

索引：`idx_word_count_stats_project_ul_id`、`idx_word_count_stats_stat_date`。

> `(project_ulid, stat_date)` **没有唯一约束**。写入路径 `ProjectRepo.UpsertWordDelta` /
> `ChatRepo.UpsertAIWordDelta` 采用「先查后写再累加」，并发下可能插入重复行。

## 4. 枚举与状态码

### 4.1 用户与权限

| 字段 | 取值 | 常量 |
| --- | --- | --- |
| `users.user_status_code` | `active` / `disabled` | `UserStatusActive` / `UserStatusDisabled` |
| `roles.role_code` | `admin` / `user`（内置两种，自定义角色不限） | `RoleCodeAdmin` / `RoleCodeUser` |
| `roles.perm_codes_json` | `*` | `PermAdminAll` |
| | `system:user:manage` | `PermUserManage` |
| | `system:role:manage` | `PermRoleManage` |
| | `system:audit:read` | `PermAuditRead` |
| | `system:model:manage` | `PermModelManage` |
| | `system:usage:read` | `PermUsageRead` |
| | `system:status:read` | `PermSystemRead` |

### 4.2 创作

| 字段 | 取值 |
| --- | --- |
| `projects.project_status` | `draft` / `serial` / `completed`（创建时固定 `draft`） |
| `chapters.chapter_status` | `draft` / `revised` / `locked`（创建时固定 `draft`） |
| `chapter_snapshots.snapshot_type` | 代码中仅写入 `manual` |
| `characters.role_type_code` | `protagonist` / `antagonist` / `supporting` / `minor` |
| `character_relations.relation_type_code` | 自由文本（示例注释：friend/enemy/lover/kin/teacher） |
| `lore_settings.lore_type_code` | `world` / `rule` / `item` / `place` / `event` / `note` |
| `outline_nodes.node_type_code` | `volume` / `arc` / `storyBlock` / `chapter` |
| `outline_nodes.node_status_code` | `draft` / `planning` / `ready` / `done`（缺省 `draft`） |
| `detailed_scenes.pace_code` | `slow` / `medium` / `fast` / `climax` |

### 4.3 连续性

| 字段 | 取值 | 常量 |
| --- | --- | --- |
| `temporal_facts.fact_status_code` | `candidate` / `confirmed` / `superseded` / `stale`（缺省 `candidate`） | `FactStatusCode*` |
| `foreshadow_hooks.hook_status_code` | `planned` / `planted` / `echoed` / `resolved` / `abandoned` | `HookStatusCode*` |
| `foreshadow_hooks.importance_code` | `high` / `medium` / `low`（缺省 `medium`） | `ImportanceCode*` |
| `chapter_summaries.summary_status_code` | `pending` / `verified` / `stale`（缺省 `pending`） | `SummaryStatusCode*` |

### 4.4 知识库与对话

| 字段 | 取值 |
| --- | --- |
| `knowledge_documents.doc_type_code` | `txt` / `md` / `docx` / `pdf`（由扩展名白名单推导） |
| `knowledge_documents.doc_status_code` | `pending` / `indexing` / `indexed` / `failed` |
| `chat_messages.role_code` | `user` / `assistant` / `system` |
| `skills.category_code` | `write` / `revise` / `outline` / `setting` / `review` / `other` |
| `skills.scope_code` | `global` / `project` |
| `menus.menu_type_code` | `group` / `page` / `action` |
| `menus.scope_code` | `global` / `project` |
| 检索模式（请求参数，非列） | `keyword` / `hybrid`（默认 hybrid，其他值按 keyword 处理） |

### 4.5 运维

| 字段 | 取值 |
| --- | --- |
| `jobs.job_type_code` | `doc_index` / `doc_reindex` |
| `jobs.job_status_code` | `queued` / `running` / `success` / `failed`；代码实际写入 `queued`、`success`、`failed`（`running` 仅出现在 `JobRepo.GetPendingByDoc` 的查询条件中） |
| `model_providers.protocol_code` | `openai` / `anthropic` |
| `model_providers.last_test_status_code` | `untested` / `success` / `failed` |
| `model_usage_stats.call_type_code` | `chat` |

## 5. 非关系存储

### 5.1 MinIO

| 项 | 值 | 来源 |
| --- | --- | --- |
| 桶名 | `novelworkbench`（`minio.bucket` 配置项） | `backend/configs/config.yaml` |
| 对象键 | 等于文档 ULID（`DOC_...`） | `KnowledgeService.Upload`：`ObjectKey: docULID` |
| 桶初始化 | 启动时检查，不存在则创建 | `infra.NewMinIO` |

上传顺序（`KnowledgeService.Upload`）：

1. 校验项目归属、扩展名白名单、大小上限；
2. `PutObject(bucket, docULID, ...)` 写原文；
3. 插入 `knowledge_documents` 行（`doc_status_code = 'pending'`）；
4. 若第 3 步失败 → **回滚**：`RemoveObject(docULID)` 删掉已上传对象；
5. 投递索引任务（先建 `jobs` 留痕 `queued`，再发 RabbitMQ）；投递失败则把文档置 `failed`，
   并把任务留痕置 `failed`。

删除顺序（`KnowledgeService.Delete`）：

1. `RemoveObject(doc.ObjectKey)` 删原文（失败即中断，抛 `ERR_STORAGE`）；
2. `DELETE FROM nw_chunks WHERE doc_ulid = ...` 删全文索引行（失败只告警，降级）；
3. 单事务内删 `knowledge_chunks` 行，再删 `knowledge_documents` 行。

项目删除时的外部清理在**数据库事务提交之后**执行（`ProjectService.cleanupKnowledgeExternal`），
先删 MinIO 对象再删 Manticore 行，失败只打告警日志，不回滚数据库。

### 5.2 Manticore

表名默认 `nw_chunks`（`manticore.table_chunks` 配置项，`manticoreTableName` 为空时也回落到此名）。

建表 DDL（`ensureManticoreSchema`）：

```sql
CREATE TABLE IF NOT EXISTS nw_chunks(
  chunk_ulid string,
  doc_ulid string,
  project_ulid string,
  chunk_text text
) charset_table='non_cjk,0..9,A..Z->a..z,a..z' ngram_len='1' ngram_chars='cjk'
```

- `charset_table`：非 CJK 字符集，小写化映射；
- `ngram_len='1'`：单字粒度；
- `ngram_chars='cjk'`：CJK 单字切分。

启动期迁移与回灌（`KnowledgeWorker.Run`）：

1. 执行 `CREATE TABLE IF NOT EXISTS`；
2. `SHOW CREATE TABLE nw_chunks`，若结果中**不含 `ngram_len`**，说明是旧结构 → `DROP TABLE` → 重新建表，
   返回 `rebuilt = true`；
3. `rebuilt` 为 true 时执行 `rebuildManticoreIndex`：从 PG `knowledge_chunks` 读全部切片
   （`ListAllChunksForIndex`，按 `doc_ulid, chunk_index` 排序），按文档分组，逐文档调用
   `syncManticoreChunks` 回灌。

单文档同步 `syncManticoreChunks` 为**先删后插**：`DELETE FROM nw_chunks WHERE doc_ulid = '...'`
之后逐条 `INSERT`。

检索（`queryManticoreHits`）：

```sql
SELECT chunk_ulid, WEIGHT() AS score FROM nw_chunks
WHERE MATCH('@chunk_text <expr>') AND project_ulid = '...'
  [AND doc_ulid IN (...)]
ORDER BY score DESC LIMIT <topK>
```

- `<expr>`：keyword 通道为整串引号包裹；term 通道为逐词引号后用 ` | ` 连接的 OR 表达式；
- 字符串转义由 `escapeManticoreString` 处理（`\`、`'`、`"`）。

### 5.3 Redis

只用于登出令牌黑名单。

| 项 | 值 |
| --- | --- |
| 键格式 | `nw:token:blacklist:<sha256hex(token)>`（`repo/token_repo.go`） |
| 值 | `"1"` |
| TTL | 令牌剩余有效期（`expires_at - issued_at`），到期自然清除 |
| 读语义 | `IsRevoked` 在 Redis 异常时返回 false（视为未吊销，放行） |

### 5.4 RabbitMQ

| 项 | 值 |
| --- | --- |
| 队列名 | `nw_doc_index`（`rabbitmq.queue_doc_index` 配置项） |
| 声明 | `QueueDeclare(name, durable=true, autoDelete=false, exclusive=false, noWait=false)` |
| 投递 | `PublishWithContext`，`DeliveryMode = Persistent`，`Content-Type = application/json` |
| 消息体 | `{"doc_ulid": "DOC_..."}`（`KnowledgeIndexJobDTO`） |

消费确认语义（`MQRepo.ConsumeIndexJobs`）：

| 情况 | 动作 |
| --- | --- |
| 载荷 JSON 非法 | `Nack(requeue=false)`，丢弃毒消息 |
| handler 返回 nil（成功，或已在内部落库为失败） | `Ack` |
| handler 返回 error（如关停中断、查询异常） | `Nack(requeue=true)`，重投 |

worker 内部对单个文档最多重试 3 次（`maxIndexRetryCount`），退避为 `attempt` 秒；
最终失败则把文档置 `failed` 并返回 nil（ack，不再重投）。

## 6. 启动播种

播种顺序见 `cmd/server/main.go`，全部在 AutoMigrate 之后、HTTP 监听之前执行。

| 顺序 | 动作 | 条件 | 失败处理 |
| --- | --- | --- | --- |
| 1 | `providerService.SeedDefault` 播种默认模型供应商 | 已配置 `ai.base_url` 与 `ai.chat_model` 且表为空（`Count()==0`） | 仅告警（`Warnw`），继续启动 |
| 2 | `skillService.SeedDefaults` 播种内置技能 | 按 `(global, "", skill_code)` 查缺补插 | 致命：`os.Exit(1)` |
| 3 | `settingService.SeedDefaults` 播种默认配置 | 按 `setting_key` 查缺补插 | 致命：`os.Exit(1)` |
| 4 | `roleService.SeedDefaults` 播种内置角色 | 按 `role_code` 查缺补插 | 致命：`os.Exit(1)` |
| 5 | `menuService.SeedDefaults` 播种内置菜单 | 按 `menu_code` 查缺补插（父级编码先行解析） | 致命：`os.Exit(1)` |
| 6 | `userAdminService.BootstrapAdmin` 引导管理员 | `users` 表为空（`Count()==0`） | 致命：`os.Exit(1)` |

播种内容：

- **供应商**：名称固定「默认模型网关」，`enabled_flag=true`、`default_flag=true`、
  `last_test_status_code='untested'`，API Key 用 JWT 密钥派生的材料做 AES-256-GCM 封存。
- **技能**：来源 `ai.BuiltinSkills()`，共 9 条内置全局技能
  （`continue`/`outline`/`polish`/`expand`/`condense`/`summary`/`review`/`name`/`setting`），
  `scope_code='global'`、`builtin_flag=true`、`enabled_flag=true`；已存在的不覆盖（保留作者改动）。
- **菜单**：来源 `menuSeedList`，共 22 行：3 个分组（创作/平台/系统）、18 个页面与 1 个动作权限
  （`action_model_write`）。页面含仪表盘、系统设置、7 个项目内页（写作台/创作规划/连续性/
  角色/世界观/知识库/素材笔记）、模型供应商、技能管理与 7 个管理页（用户/角色/菜单/权限目录/
  审计/用量/运行状态）；`perm_code` 非空的页面即权限登记处。
- **应用配置**：`chat_history_count=5`、`rag_top_k=6`。配置键为白名单，值必须为正整数。
- **权限码**：`system:*`（用户/角色/菜单/权限目录/审计/设置/模型/用量/状态）与 `skill:manage`，
  全部由内置菜单登记；`*` 为通配全部权限。
- **角色**：`admin`（`perm_codes_json = ["*"]`）、`user`（`perm_codes_json = "[]"`），
  `builtin_flag=true`；已存在不覆盖。
- **管理员**：登录名 `admin`，邮箱 `admin@novelworkbench.local`，密码取 `app.admin_password`
  配置项（缺省 `Admin@123456`），挂载 `admin` 角色。

## 7. 事务与一致性

事务封装只有一种：`repo.WithTx(db, fn)`（`repo/tx.go`，即 `db.Transaction`）。
仓储层内另有两处自建事务：`ChatRepo.DeleteSessionWithMessages` 与
`RoleRepo.ReplaceUserRoles`。

### 7.1 有事务边界的操作

| 操作 | 事务内覆盖范围 | 位置 |
| --- | --- | --- |
| 项目删除 | 章节快照 → 章节 → 卷 → 角色关系与角色 → 世界观设定 → 会话消息与会话 → 知识切片 → 知识文档 → 字数统计 → 项目行 | `ProjectService.Delete` |
| 卷删除 | 章节快照 → 章节 → 卷 → 项目字数回补 | `VolumeService.Delete` |
| 章节更新 | 章节字段更新 + 项目累计字数增减 + 当日字数统计 | `ChapterService.Update` |
| 章节删除 | 章节快照 → 章节 → 项目字数回补 | `ChapterService.Delete` |
| 快照回滚 | 自动留档 → 章节正文与字数更新 → 项目字数增减 | `ChapterService.SnapshotRestore` |
| 角色删除 | 关系边 → 角色卡 | `CharacterService.Delete` |
| 知识文档删除 | 切片行 → 文档行 | `KnowledgeService.Delete` |
| 切片重建 | 先删该文档旧切片 → 逐条插入新切片 | `KnowledgeWorker.indexDocument` |
| AI 回复落库 | assistant 消息 → 当日 AI 字数统计 | `ChatService` |

字数账目维护（`applyWordDelta`）在事务内做两件事：

- `ProjectRepo.AdjustTotalWordCount`：`total_word_count = total_word_count + delta`（表达式增减，防并发覆盖）；
- `ProjectRepo.UpsertWordDelta`：当日 `word_count_stats` 累加 `word_delta`。

### 7.2 不在事务内的操作

以下跨存储写操作无法纳入同一个数据库事务，只能靠应用层编排 + 告警降级：

| 操作 | 顺序 | 失败后果 |
| --- | --- | --- |
| 知识库上传 | MinIO 写入 → PG 文档行 → RabbitMQ 投递 | PG 行失败 → 回滚删 MinIO 对象；投递失败 → 文档置 `failed` |
| 知识库删除 | MinIO 删原文 → Manticore 删行 → PG 事务 | Manticore 失败只告警，索引残留 |
| 索引同步 | PG 切片事务 → Manticore 同步 → 文档置 `indexed` | Manticore 失败只告警，keyword 通道降级 |
| 项目删除的外部清理 | DB 事务提交后 → MinIO 删对象 → Manticore 删行 | 失败只告警，DB 一致性已保证 |

### 7.3 项目删除的级联覆盖面（重要）

`ProjectService.Delete` 在单个事务内**显式删除**下列数据：

`chapter_snapshots` → `chapters` → `volumes` → `character_relations` → `characters` →
`lore_settings` → `chat_messages` → `chat_sessions` → `knowledge_chunks` →
`knowledge_documents` → `word_count_stats` → `projects`

**以下表没有删除路径，删除项目后其数据仍然保留（成为孤儿数据）：**

| 表 | 原因 |
| --- | --- |
| `outline_nodes` | `OutlineRepo` 只有 `DeleteNode`（按项目 + 节点 ULID），无按项目批量删除方法 |
| `detailed_scenes` | `OutlineRepo.DeleteScene` 同上，无按项目批量删除方法 |
| `temporal_facts` | `ContinuityRepo` 只有 Create/Update/Get/List，无任何 Delete 方法 |
| `foreshadow_hooks` | 同上 |
| `chapter_summaries` | 同上 |
| `jobs` | `JobRepo` 只有 Create/Update/Get，无删除方法；任务留痕永久保留 |
| `audit_logs` | `AuditRepo` 只增不删，无任何 Delete 方法 |

其他不随项目删除清理的数据：`model_providers`、`model_usage_stats`、`skills`（全局行）、`menus`、
`app_settings`、`roles`、`user_roles`、`users`（均为全局数据，与项目无关联）。

另外，删除大纲节点前需先删除其子节点（`OutlineService.DeleteNode` 校验子节点数为 0），
删除父节点不会级联删除子节点，也不会删除挂在其下的 `detailed_scenes`。

## 8. 索引与查询约束

| 约束 | 说明 |
| --- | --- |
| 无外键 | 所有 DO 结构体没有 `gorm:"foreignKey"` 标签，AutoMigrate 不生成 `FOREIGN KEY` |
| 无软删除 | 没有 `gorm.DeletedAt` 字段，所有删除都是物理 `DELETE` |
| 无数据库级联 | `ON DELETE CASCADE` 不存在，级联全部由服务层显式编排 |
| 唯一约束共 7 处 | `users.user_name`、`users.user_email`、`roles.role_code`、`menus.menu_code`、`app_settings.setting_key`（均为单列唯一索引），`model_usage_stats.uk_mus_dim`（`stat_date`+`provider_ulid`+`model_name`+`call_type_code` 四列组合唯一索引），以及 `skills` 作用域内编码（由服务层保证，无数据库唯一索引以允许空值多行） |
| 归属校验在服务层 | `mustOwnProject` / `mustOwnChapter` 等函数比对 `projects.user_ulid`，不是数据库约束 |
| 跨表关联用 JOIN | 例如 `RoleRepo.ListRolesByUserULID` 用 `JOIN user_roles ON user_roles.role_ulid = roles.role_ulid` |
| 分页 | `PageREQ.Normalize` 归一为 `page_num >= 1`、`page_size` 默认 20、上限 100 |
| 统计聚合在 SQL 内 | `model_usage_stats` 的按日 / 按供应商 / 按模型聚合由 `ModelUsageRepo` 的 `GROUP BY` 完成，缺日由服务层补零 |
| 大文本不进索引 | `content_text`、`chunk_text` 等 `text` 列不建索引；正文检索依赖 Manticore |

## 9. 数据迁移

- 迁移唯一入口是 `cmd/server/main.go` 的 `db.AutoMigrate(...)`，一次性声明 26 个 DO：
  `UserDO`、`ProjectDO`、`VolumeDO`、`ChapterDO`、`ChapterSnapshotDO`、`WordCountStatDO`、
  `CharacterDO`、`CharacterRelationDO`、`LoreDO`、`KnowledgeDocumentDO`、`OutlineNodeDO`、
  `DetailedSceneDO`、`TemporalFactDO`、`ForeshadowHookDO`、`ChapterSummaryDO`、`KnowledgeChunkDO`、
  `SkillDO`、`SkillReferenceDO`、`MenuDO`、`JobDO`、`AppSettingDO`、`ChatSessionDO`、`ChatMessageDO`、`RoleDO`、
  `UserRoleDO`、`AuditLogDO`、`ModelProviderDO`、`ModelUsageStatDO`。
- 仓库内**没有 `.sql` 文件，也没有 migrations 目录**。
- AutoMigrate 语义：表不存在则 `CREATE TABLE`；表已存在则逐列比对并补列、补索引。
  它**不做**破坏性变更——不删列、不改列类型、不删索引。
  因此删除字段或修改字段宽度必须手工执行 DDL。
- AutoMigrate 失败（连接失败或 DDL 出错）时 `main.go` 直接 `os.Exit(1)`。
- Manticore 侧的"迁移"是自实现的：`ensureManticoreSchema` 检测 `ngram_len` 缺失时
  drop + recreate + 从 PG 回灌，属于唯一支持的结构升级路径。

## 10. 备份与恢复

| 数据 | 备份对象 | 恢复要点 |
| --- | --- | --- |
| PostgreSQL | 整库（`pg_dump`） | 恢复后重启服务，AutoMigrate 幂等补齐缺失的表/列/索引 |
| MinIO | 桶 `novelworkbench` | 对象键即文档 ULID，恢复后与 PG `knowledge_documents.object_key` 一一对应 |
| Manticore | **无需备份** | 索引是投影；恢复 PG 后重启服务，`ensureManticoreSchema` 会建表，
  但只有检测到旧结构才回灌。要强制重建，可先 drop `nw_chunks` 表再启动，
  或对每个文档调用重投索引接口 |
| Redis | **无需备份** | 仅存令牌黑名单，重启后失效即允许旧令牌（按 JWT 有效期自然过期） |
| RabbitMQ | **无需备份** | 仅传输通道；队列为 durable + persistent 消息，重启后未消费消息自动重投 |

恢复顺序建议：PostgreSQL → MinIO → 重启服务（触发 AutoMigrate 与 Manticore 建表）→
按需触发知识库重索引。

一致性注意：

- PG 与 MinIO 之间没有分布式事务。上传是「MinIO 先写、PG 后写、PG 失败删 MinIO」，
  删除是「MinIO 先删、PG 后删」，两者之间存在短暂不一致窗口。
- PG 与 Manticore 之间同理：切片先落 PG 再同步 Manticore，Manticore 同步失败只降级，
  不会回滚 PG。
- `word_count_stats` 的 `(project_ulid, stat_date)` 无唯一约束，并发写入可能产生重复行；
  统计查询使用 `SUM` 聚合，重复行的数值会重复累加。
