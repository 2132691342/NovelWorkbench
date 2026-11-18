# 数据库设计

存储分四类：PostgreSQL（业务数据 + pgvector 向量）、MinIO（文档原文）、Manticore（切片全文索引）、Redis（令牌黑名单）。本文只描述 PostgreSQL 侧表结构；表由 gorm AutoMigrate 建立，主键均为带业务前缀的 ULID 字符串（`varchar(40)`）。

公共字段：所有表含 `created_at`、`updated_at`（gorm 自动维护）。

## 1. 表总览

| 表名 | 实体 | 前缀 | 说明 |
| --- | --- | --- | --- |
| users | 用户 | USR_ | 账号与凭据 |
| roles | 角色 | ROL_ | RBAC 角色（权限码 JSON） |
| user_roles | 用户-角色 | URO_ | 关联表 |
| audit_logs | 审计日志 | AUD_ | 变更操作留痕 |
| projects | 小说项目 | PRJ_ | 创作主线根聚合 |
| volumes | 卷 | VOL_ | 项目下的分卷 |
| chapters | 章节 | CHP_ | 正文载体 |
| chapter_snapshots | 章节快照 | SNP_ | 修改前留档/回滚 |
| characters | 角色卡 | CHR_ | 人物设定 |
| character_relations | 角色关系 | RLT_ | 关系图谱边 |
| lore_settings | 世界观设定 | LOR_ | 规则/地点/物品/事件 |
| knowledge_documents | 知识库文档 | DOC_ | 上传文档元数据 |
| knowledge_chunks | 知识库切片 | CHK_ | 向量 + 文本 |
| chat_sessions | AI 会话 | SES_ | 对话容器 |
| chat_messages | 聊天消息 | MSG_ | 消息与引用记录 |
| slash_commands | 斜杠命令 | SKL_ | 技能（提示词资产） |
| jobs | 异步任务 | JOB_ | 队列任务落库凭据 |
| app_settings | 应用配置 | SET_ | 运行时 KV 配置 |
| word_count_stats | 字数统计 | WCS_ | 每日字数增量 |

## 2. 核心表结构

### users

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| user_ulid | varchar(40) | PK | 用户 ULID |
| user_name | varchar(64) | UNIQUE | 登录名 |
| user_email | varchar(128) | UNIQUE | 邮箱 |
| password_hash | varchar(128) | | bcrypt 哈希，JSON 不输出 |
| avatar_url | varchar(256) | | 头像 |
| remark_text | varchar(256) | | 备注 |
| user_status_code | varchar(16) | INDEX | active/disabled，DB 默认 active |

### projects

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| project_ulid | varchar(40) | PK | 项目 ULID |
| user_ulid | varchar(40) | INDEX | 归属用户 |
| project_name | varchar(128) | | 书名 |
| genre_code | varchar(32) | | 题材编码 |
| style_text | varchar(1024) | | 文风设定（注入 AI 系统提示） |
| synopsis_text | varchar(4096) | | 简介 |
| project_status | varchar(16) | INDEX | draft/serial/completed |
| total_word_count | bigint | | 累计字数（章节增删改联动维护） |

### volumes

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| volume_ulid | varchar(40) | PK | 卷 ULID |
| project_ulid | varchar(40) | INDEX | 归属项目 |
| volume_name | varchar(128) | | 卷名 |
| volume_order | int | | 排序号 |
| summary_text | varchar(2048) | | 卷概要 |

### chapters

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| chapter_ulid | varchar(40) | PK | 章节 ULID |
| project_ulid | varchar(40) | INDEX | 归属项目 |
| volume_ulid | varchar(40) | INDEX | 归属卷 |
| chapter_name | varchar(128) | | 章节名 |
| chapter_order | int | INDEX | 排序号（项目内升序） |
| content_text | text | | 正文 |
| summary_text | varchar(2048) | | 梗概 |
| word_count | bigint | | 字数（CJK 逐字 + 拉丁按词） |
| chapter_status | varchar(16) | | draft/revised/locked |

### chapter_snapshots

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| snapshot_ulid | varchar(40) | PK | 快照 ULID |
| chapter_ulid | varchar(40) | INDEX | 所属章节 |
| content_text | text | | 快照正文 |
| snapshot_type | varchar(16) | | manual/auto_restore |
| change_note | varchar(256) | | 变更说明 |

### characters / character_relations

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| characters | character_ulid(PK)、project_ulid(INDEX)、character_name、alias_text、role_type_code、personality_text、background_text、appearance_text、goal_text | 角色卡 |
| character_relations | relation_ulid(PK)、project_ulid、from_character_ulid(INDEX)、to_character_ulid(INDEX)、relation_type_code、relation_note | 有向关系边，供 AntV X6 渲染 |

### lore_settings

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| lore_ulid | varchar(40) | PK | 设定 ULID |
| project_ulid | varchar(40) | INDEX | 归属项目 |
| lore_name | varchar(128) | | 设定名 |
| lore_type_code | varchar(16) | INDEX | world/rule/item/place/event/note（note=创作笔记，不注入 AI 上下文） |
| content_text | text | | 设定内容 |
| sort_order | int | | 排序号 |

### knowledge_documents / knowledge_chunks

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| knowledge_documents | doc_ulid(PK)、project_ulid(INDEX)、doc_name、doc_type_code、object_key（MinIO 键）、doc_status_code(INDEX)、chunk_count、total_token_count、error_message | 文档元数据与索引状态机 |
| knowledge_chunks | chunk_ulid(PK)、doc_ulid(INDEX)、project_ulid(INDEX)、chunk_index、chunk_text、token_count、embedding_vec vector(1024) | 切片；向量列由 pgvector 提供 |

依赖：建表前置 `CREATE EXTENSION IF NOT EXISTS vector`。

### chat_sessions / chat_messages

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| chat_sessions | session_ulid(PK)、project_ulid(INDEX)、user_ulid(INDEX)、session_name | 会话 |
| chat_messages | message_ulid(PK)、session_ulid(INDEX)、role_code、content_text、skill_code、ref_doc_json、ref_chapter_json、ref_message_json、token_used_count | 消息；三列 JSON 记录 @引用 与历史引用，供追溯 |

### slash_commands

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| command_ulid | varchar(40) | PK | 命令 ULID |
| command_code | varchar(32) | UNIQUE | 触发编码（`/code`） |
| command_name | varchar(64) | | 展示名 |
| category_code | varchar(16) | | write/revise/outline/setting/other |
| description_text | varchar(256) | | 一句话说明 |
| prompt_template | text | | 提示词模板（`{{占位符}}`） |
| enabled_flag | bool | | 启用开关 |
| builtin_flag | bool | | 内置（内置不可删） |
| sort_order | int | | 排序号 |

### roles / user_roles / audit_logs

| 表 | 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- | --- |
| roles | role_ulid | varchar(40) | PK | 角色 ULID |
| roles | role_code | varchar(64) | UNIQUE | 角色编码（admin/user 或自定义） |
| roles | role_name | varchar(64) | | 角色名称 |
| roles | remark_text | varchar(256) | | 备注 |
| roles | perm_codes_json | text | | 权限码 JSON 数组（`["*"]` 通配全部） |
| roles | builtin_flag | boolean | | 内置角色不可删、编码不可改 |
| user_roles | user_role_ulid | varchar(40) | PK | 关联 ULID |
| user_roles | user_ulid | varchar(40) | INDEX | 用户 ULID |
| user_roles | role_ulid | varchar(40) | INDEX | 角色 ULID |
| audit_logs | audit_ulid | varchar(40) | PK | 审计 ULID |
| audit_logs | user_ulid | varchar(40) | INDEX | 操作用户（未登录为空） |
| audit_logs | user_name | varchar(64) | INDEX | 操作用户登录名（写入时回填） |
| audit_logs | action_code | varchar(128) | INDEX | 动作码（去前缀接口路径） |
| audit_logs | target_type | varchar(32) | | 目标类型（user/role/... 白名单） |
| audit_logs | target_ulid | varchar(40) | INDEX | 目标 ULID |
| audit_logs | detail_text | varchar(512) | | 补充信息（响应状态） |
| audit_logs | ip_addr | varchar(64) | | 客户端 IP |

角色授权以 `user_roles` 整体覆盖方式维护（先删后插，事务内）；权限判定按用户角色的权限码并集，`*` 通配。审计由中间件在请求出口同步落库，失败仅告警不阻断业务。

### jobs / app_settings / word_count_stats

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| jobs | job_ulid(PK)、job_type_code(INDEX)、payload_json、job_status_code(INDEX)、retry_count、error_message | 队列任务留痕（queued/running/success/failed） |
| app_settings | setting_ulid(PK)、setting_key(UNIQUE)、setting_value、setting_desc | 运行时配置（如历史注入条数） |
| word_count_stats | word_stat_ulid(PK)、project_ulid(INDEX)、stat_date(INDEX)、word_delta、ai_word_delta | 每日字数增量，ECharts 数据源 |

## 3. 非关系存储

| 存储 | 内容 | 键/表设计 |
| --- | --- | --- |
| MinIO | 文档原文 | 桶 `novelworkbench`，对象键 = `doc_ulid`（`GET /knowledge/content` 按需取回） |
| Manticore | 切片全文索引 | 表 `nw_chunks`（chunk_ulid string 主键、doc_ulid/project_ulid string 属性、chunk_text text）；`charset_table=non_cjk,...` + `ngram_len=1` + `ngram_chars=cjk` 支持中文子串检索；旧结构启动时自动重建并从 PG 切片回灌 |
| Redis | 令牌黑名单 | `nw:token:blacklist:<sha256(token)>` = "1"，TTL = 令牌剩余寿命 |

列命名约定：所有 `XxxULID` 字段显式 `column:xxx_ulid`（并由 `infra.ULIDNamingStrategy` 兜底归一），与手写 SQL、JSON 命名严格一致。

## 4. 一致性与并发

- 章节字数变更（创建/更新/删除/快照回滚）与 `projects.total_word_count`、`word_count_stats` 同事务写入（`repo.WithTx`），杜绝字数漂移。
- 归属校验在 service 层统一做：用户 → 项目（user_ulid）→ 卷/章/角色/设定/文档（project_ulid），违规返回 `ERR_FORBIDDEN`，不存在返回 `ERR_NOT_FOUND`。
- 文档索引状态机单向流转：`pending → indexing → indexed/failed`；`reindex` 仅对 `indexed/failed` 开放，重复投递幂等（切片按 doc_ulid 先清后写）。
- 级联删除矩阵（`repo.WithTx` 单事务；外部存储在事务提交后 best-effort 清理并告警）：

| 触发 | 同事务删除 | 外部清理 |
| --- | --- | --- |
| 删除项目 | 快照 → 章节 → 卷 → 角色关系 → 角色 → 设定 → 聊天消息 → 会话 → 切片 → 文档 → 每日统计 → 项目 | MinIO 原文 + Manticore 切片 |
| 删除卷 | 快照 → 章节 → 卷，并按章节字数合计回补项目字数 | 无 |
| 删除章节 | 快照 → 章节，并回补项目字数 | 无 |
| 删除文档 | 切片 → 文档 | MinIO 原文 + Manticore 切片 |
| 删除会话 | 消息 → 会话 | 无 |
