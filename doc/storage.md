# 存储运行时设计

本文描述 NovelWorkbench 的非关系存储、启动播种、事务边界、跨存储一致性、索引约束、迁移策略与备份恢复。
表结构与枚举取值见 [database.md](database.md)。

## 1. 非关系存储

### 1.1 MinIO

| 项 | 值 |
| --- | --- |
| 桶名 | `novelworkbench`（`minio.bucket`） |
| 桶初始化 | `infra.NewMinIO` 检查桶，不存在则创建；启动后再做一次健康检查，失败只告警 |
| 对象键 | 文档 ULID（`DOC_...`），即 `knowledge_documents.object_key` |
| 上传 Content-Type | `application/octet-stream` |

上传顺序（`KnowledgeService.Upload`）：校验归属、扩展名白名单、大小上限 → `ObjectRepo.PutObject` 写原文（失败抛 `ERR_STORAGE`）→ 插入 `knowledge_documents` 行（`doc_status_code = 'pending'`，失败则 `RemoveObject` 回滚）→ `enqueueIndexJob` 先插 `jobs` 行（`queued`）再投递 RabbitMQ（投递失败把任务与文档置 `failed`）。

删除顺序（`KnowledgeService.Delete`）：`RemoveObject`（失败即中断，抛 `ERR_STORAGE`）→ 删 Manticore 行（失败只告警）→ 事务内删 `knowledge_chunks` 与 `knowledge_documents`。

### 1.2 Manticore

承载两张表：`nw_audit`（审计全文表，列定义见 3.1）与 `nw_chunks`（知识库切片索引）。

| 项 | 值 |
| --- | --- |
| 表名 | `nw_chunks`（`manticore.table_chunks`，配置为空时回落同名默认） |
| 客户端 | `infra.ManticoreClient`，`POST {base_url}/sql?mode=raw`，HTTP 超时 5s |
| 表结构 | `chunk_ulid string, doc_ulid string, project_ulid string, chunk_text text` |

建表语句（`ensureManticoreSchema`）：

```sql
CREATE TABLE IF NOT EXISTS nw_chunks(
  chunk_ulid string,
  doc_ulid string,
  project_ulid string,
  chunk_text text
) charset_table='non_cjk,0..9,A..Z->a..z,a..z' ngram_len='1' ngram_chars='cjk'
```

`ngram_len='1'` + `ngram_chars='cjk'` 让中文按单字切分；`charset_table` 做非 CJK 字符小写映射。

worker 启动时的结构校验（`KnowledgeWorker.Run`）：执行 `CREATE TABLE IF NOT EXISTS`，再 `SHOW CREATE TABLE nw_chunks`；若结果不含 `ngram_len` 则 `DROP TABLE` 重建，并从 PG 全量回灌（`rebuildManticoreIndex` 读 `ListAllChunksForIndex`，按文档分组调用 `syncManticoreChunks`）。

单文档同步 `syncManticoreChunks` 为「先删后插」：`DELETE FROM nw_chunks WHERE doc_ulid = '...'` 后逐条 `INSERT`，字符串转义由 `escapeManticoreString` 处理。

检索（`queryManticoreHits`）：

```sql
SELECT chunk_ulid, WEIGHT() AS score FROM nw_chunks
WHERE MATCH('@chunk_text <expr>') AND project_ulid = '<PRJ_...>'
  [AND doc_ulid IN (...)]
ORDER BY score DESC LIMIT <topK>
```

`expr` 在 term 通道是逐词引号后用 ` | ` 连接的 OR 表达式，phrase 通道是整串引号包裹。命中结果按 `chunk_ulid` 回表 `knowledge_chunks`（`GetChunksByULIDs` 联 `knowledge_documents` 取文档名）。hybrid 模式对 term / phrase 两路命中做 RRF 融合（`score = Σ 1/(60+rank)`），再交默认模型重排（重排失败 fail-open 保留原顺序）。检索策略细节见 [ai.md](ai.md)。

### 1.3 Redis

两类键，均为易失缓存，可整体丢弃后由业务重建：

| 键格式 | 值 | TTL | 语义 |
| --- | --- | --- | --- |
| `nw:token:blacklist:<sha256hex(token)>` | `"1"` | 令牌签发时长（`expires_at - issued_at`） | 登出拉黑；`IsRevoked` 在 Redis 异常时返回 false（视为未吊销），写入点 `AuthService.Logout` |
| `nw:idempotency:<sha256hex(user \| path \| key)>` | `IdempotencyRecord` JSON（`state_code`/`body_hash`/`http_status`/`header_map`/`body_text`） | 24 小时 | 写请求幂等：首请求 `SET NX` 写 `processing`，2xx 后 `SET XX` 覆盖 `done`，非 2xx 与 5xx `DEL` 释放；单键上限 1MB，超限放弃缓存；Redis 不可用时放行 |

### 1.4 RabbitMQ

| 项 | 值 |
| --- | --- |
| 队列名 | `nw_doc_index`（`rabbitmq.queue_doc_index`） |
| 声明 | `QueueDeclare(name, durable=true, autoDelete=false, exclusive=false, noWait=false)`，投递与消费前各声明一次（幂等） |
| 投递 | `PublishWithContext`，`DeliveryMode=Persistent`，`ContentType=application/json` |
| 消息体 | `{"doc_ulid":"DOC_..."}` |

消费确认语义（`MQRepo.ConsumeIndexJobs`）：

| 情况 | 动作 |
| --- | --- |
| 载荷 JSON 非法 | `Nack(requeue=false)`，丢弃 |
| handler 返回 nil（成功，或已在内部落库为失败） | `Ack` |
| handler 返回 error（查询异常、关停中断） | `Nack(requeue=true)`，重投 |
| 通道关闭或 ctx 取消 | 消费循环退出 |

消费循环中断后按 5s 间隔重连。`KnowledgeWorker.handleIndexJob` 对单文档最多重试 3 次，退避 `attempt` 秒；最终失败写入 `jobs` 与 `knowledge_documents` 的失败信息，并向项目主人发 `task` 类型通知后返回 nil（不重投）。

## 2. 启动播种

全部播种在 `AutoMigrate` 之后、HTTP 监听之前执行。所有播种都按业务键查缺补插，重复启动不产生重复行。

| 顺序 | 动作 | 触发条件 | 失败处理 |
| --- | --- | --- | --- |
| 1 | `ModelProviderService.SeedDefault` 播种默认供应商与默认模型 | 配置了 `ai.base_url` 与 `ai.chat_model`，且按供应商名或 BaseURL 查不到同名/同网关行（`FindByNameOrBaseURL`） | 告警，继续启动 |
| 2 | `ModelProviderService.MigrateLegacyModels`：逐 provider 判断，把「仍带旧列 `chat_model_name` 且尚无目录行」的供应商补成目录行 | 存在旧列 `chat_model_name` 且该 provider 无目录行 | 告警，继续启动 |
| 3 | `ToolService.SeedDefaults` 播种内置工具 | 按 `tool_code` 查缺补插 | `os.Exit(1)` |
| 4 | `PromptService.SeedDefaults` 播种内置全局任务提示词（4 个流程 × 2 条 = 8 条）并按指纹升级、下线清单外的内置提示词 | 按 `(global, "", prompt_code)` 查行；不存在则插入；`customized_flag` 或指纹一致则跳过；否则覆盖正文并写新指纹 | `os.Exit(1)` |
| 5 | `WorkflowService.SeedDefaults` 播种内置工作流模板 | 按 `template_code` 查缺补插；步骤定义或名称变化时覆盖步骤 JSON | `os.Exit(1)` |
| 6 | `SettingService.SeedDefaults` 播种默认配置 | 按 `setting_key` 查缺补插；已存在时仅同步 `setting_desc` | `os.Exit(1)` |
| 7 | `MenuService.SeedDefaults` 播种内置菜单 | 按 `menu_code` 查缺补插，父级编码先解析 | `os.Exit(1)` |
| 8 | `RoleService.SeedDefaults` 播种内置角色并修正权限数据 | 按 `role_code` 查缺补插；`admin` 固定 `*`，其他内置角色剔除 `*` 与未登记编码（依赖上一步菜单先登记完权限码，顺序不可颠倒） | `os.Exit(1)` |
| 9 | `UserAdminService.BootstrapAdmin` 引导管理员 | `users` 为空 | `os.Exit(1)` |

播种内容：

| 对象 | 内容 |
| --- | --- |
| 供应商 | 名称「默认模型网关」，`enabled_flag=true`，`last_test_status_code='untested'`，API Key 用 JWT 密钥派生材料做 AES-256-GCM 封存 |
| 默认模型 | 取 `ai.chat_model`，`default_flag=true`，`model_label` 回填模型名；播种后授权给全部内置角色 |
| 内置提示词 | 4 个创作流程共 8 条任务提示词（`ai.BuiltinDomains()`，无内置身份提示词）；`scope_code='global'`、`builtin_flag=true`、`enabled_flag=true`，`asset_hash_text` 为出厂指纹。启动时按指纹升级：已自改或指纹一致不覆盖，其余覆盖正文；清单之外的旧内置提示词删除 |
| 内置工具 | 6 个（`ToolService.SeedDefaults`）：`chapter_check`、`deai_scan`、`chapter_digest`、`character_scan`、`foreshadow_scan`、`lore_lookup`；全局、启用、带参数声明 |
| 内置工作流模板 | 3 个（`WorkflowService.SeedDefaults`）：`chapter_draft`（逐章成稿）、`chapter_polish`（存量打磨）、`chapter_deai`（批量去 AI 味）；`builtin_flag=true`，步骤定义随代码升级同步 |
| 应用配置 | `rag_top_k=6`、`daily_word_goal=2000`；键为白名单，值须为整数并落在 `settingRangeMap` 区间 |
| 内置角色 | `admin`（`perm_codes_json=["*"]`）、`user`（`perm_codes_json="[]"`），`builtin_flag=true`，已存在不覆盖 |
| 内置菜单 | 来源 `menuSeedList`，共 29 行：3 个分组（创作 / AI 资产 / 系统）、25 个页面、1 个动作权限（`action_model_write`）。`material` 是素材库唯一入口；`character`、`lore`、`knowledge`、`notes` 标记 `HiddenFlag`（入口已并入素材库），`profile` 从顶栏用户菜单进入同样隐藏，五者仍注册路由但不进导航；`workflow` 是自动化工作流入口 |
| 管理员 | 登录名 `admin`，邮箱 `admin@novelworkbench.local`，密码取 `app.admin_password`（缺省 `Admin@123456`），挂载 `admin` 角色 |

## 3. 事务与一致性

事务封装统一为 `repo.WithTx(db, fn)`（`backend/internal/repo/tx.go`，即 `db.Transaction`）。仓储层内另有三处自建事务：`ChatRepo.DeleteSessionWithMessages`、`RoleRepo.ReplaceUserRoles`、`ModelProviderRepo.ReplaceGrants`。

### 3.1 有事务边界的操作

| 操作 | 事务内覆盖范围 | 位置 |
| --- | --- | --- |
| 项目删除 | 章节快照 → 章节 → 卷 → 角色关系与角色 → 世界观设定 → 会话消息与会话 → 知识切片 → 知识文档 → 字数统计 → 项目行 | `ProjectService.Delete` |
| 卷删除 | 汇总卷内字数 → 章节快照 → 章节 → 卷 → 项目字数回补 | `VolumeService.Delete` |
| 章节更新 | 章节字段更新 + 项目累计字数增减 + 当日字数统计 | `ChapterService.Update` |
| 章节删除 | 章节快照 → 章节 → 项目字数回补 | `ChapterService.Delete` |
| 快照回滚 | 自动留档 → 章节正文与字数更新 → 项目字数增减 | `ChapterService.SnapshotRestore` |
| 角色删除 | 关系边 → 角色卡 | `CharacterService.Delete` |
| 知识文档删除 | 切片行 → 文档行 | `KnowledgeService.Delete` |
| 切片重建 | 删该文档既有切片 → 逐条插入新切片 | `KnowledgeWorker.indexDocument` |
| AI 回复落库 | assistant 消息 → 当日 AI 字数统计 | `ChatService.Send` |
| 用户角色覆盖 | 清空用户全部关联 → 逐条插入 | `RoleRepo.ReplaceUserRoles` |
| 模型授权覆盖 | 清空模型全部授权 → 逐条插入 | `ModelProviderRepo.ReplaceGrants` |
| 会话删除 | 消息 → 会话 | `ChatRepo.DeleteSessionWithMessages` |
| 工作流定稿入库 | 章节快照留档 → 章节正文与字数更新 → 项目字数增减 → 章节摘要写入 → 运行状态推进 | `WorkflowService` 的 `apply` 步骤 |

字数账目维护 `applyWordDelta`（`chapter_service.go`）在事务内做两件事：`ProjectRepo.AdjustTotalWordCount` 用表达式增减（`total_word_count = total_word_count + delta`）避免读改写覆盖；`ProjectRepo.UpsertWordDelta` 累加当日 `word_count_stats.word_delta`。

### 3.2 项目删除的级联覆盖面

`ProjectService.Delete` 在单个事务内按下列顺序显式删除（无数据库级联，先子后父）：

`chapter_snapshots` → `chapters` → `volumes` → `character_relations` + `characters` → `lore_settings` → `detailed_scenes` → `outline_nodes` → `temporal_facts` + `foreshadow_hooks` + `chapter_summaries` → `chat_messages` + `chat_sessions` → `knowledge_chunks` → `knowledge_documents` → `word_count_stats` → `projects`。

事务提交后执行 `cleanupKnowledgeExternal`：逐文档删 MinIO 对象与 Manticore 行，失败只告警，不回滚数据库。

以下表没有按项目删除的路径，项目删除后其行保留：`jobs`（任务留痕）、`login_logs`（日志只增不删）、
Manticore `nw_audit`（按留存天数清理）。其余表为全局数据，与项目无归属关系：`users`、`roles`、`user_roles`、
`menus`、`app_settings`、`prompts`（全局行）、`tools`、`workflow_templates`、`model_providers`、`model_catalogs`、
`model_role_grants`、`model_user_grants`、`model_usage_stats`、`notifications`（按用户而非项目清理）。

### 3.3 跨存储写操作

这些操作无法纳入同一个数据库事务，只能靠应用层编排与告警降级：

| 操作 | 顺序 | 失败后果 |
| --- | --- | --- |
| 知识库上传 | MinIO 写 → PG 文档行 → PG 任务行 → RabbitMQ 投递 | PG 行失败回滚删 MinIO 对象；投递失败把文档与任务置 `failed` |
| 知识库删除 | MinIO 删原文 → Manticore 删行 → PG 事务 | MinIO 失败即中断；Manticore 失败只告警，索引残留 |
| 索引同步 | PG 切片事务 → Manticore 先删后插 → 回写文档状态 | Manticore 失败只告警，keyword 通道降级 |
| 项目删除的外部清理 | PG 事务提交后 → MinIO 删对象 → Manticore 删行 | 失败只告警，PG 一致性已保证 |
| 用户删除 | 删角色关联 → 删模型授权 → 删通知（失败只告警）→ 删用户行 | 通知清理失败会残留通知 |
| 模型用量统计 | 内存缓冲累加 → 每 5s `Flush` 批量 upsert | 落库失败丢弃该批缓冲（只告警），统计可能少计 |

`ModelUsageService` 的缓冲是唯一的最终一致路径：`RecordUsage` 只写内存，`flushLoop` 每 5s（`usageFlushInterval`）批量写库，进程退出时再 flush 一次。

## 4. 索引与查询约束

| 约束 | 说明 |
| --- | --- |
| 无外键 | 所有 DO 没有 `foreignKey` 标签，`AutoMigrate` 不生成 `FOREIGN KEY` |
| 无软删除 | 没有 `gorm.DeletedAt` 字段，删除均为物理 `DELETE` |
| 无数据库级联 | 不存在 `ON DELETE CASCADE`，级联全部由服务层编排 |
| 数据库唯一约束 10 处 | `users.user_name`、`users.user_email`、`roles.role_code`、`menus.menu_code`、`app_settings.setting_key`、`tools.tool_code`、`workflow_templates.template_code`、`model_usage_stats.uk_mus_dim`（四列组合）、`model_providers.uk_model_provider_name`（`provider_name`）、`model_catalogs.uk_model_catalog_provider_name`（`provider_ulid` + `model_name`） |
| 服务层唯一语义 | `menus.perm_code`（`CountByPermCode`）、`model_catalogs.default_flag`（`ClearDefaultModel` 后置位）、`prompts` 的 `(scope_code, project_ulid, prompt_code)`、`chapter_summaries` 的 `(project_ulid, chapter_ulid)`、`word_count_stats` 的 `(project_ulid, stat_date)`（均先查后写） |
| 归属校验在服务层 | `mustOwnProject` / `mustOwnChapter` / `mustOwnDoc` 比对 `projects.user_ulid`，不是数据库约束 |
| 跨表关联用 JOIN | 如 `RoleRepo.ListRolesByUserULID`、`KnowledgeRepo.GetChunksByULIDs`、`StatsRepo` 联 `projects` |
| 分页 | `PageREQ.Normalize`：`page_num < 1` 归一为 1；`page_size` 缺省 20、超过 100 归一为 20 |
| 排序约定 | 时间倒序用于日志/任务/通知/项目；`sort_order ASC, created_at ASC` 用于菜单/设定/提示词；`chapter_order ASC`、`chunk_index ASC`、`sort_order ASC, outline_ulid ASC` 用于章节/切片/大纲 |
| 聚合在 SQL 内 | `model_usage_stats` 的按日/按供应商/按模型聚合由 `ModelUsageRepo` 的 `GROUP BY` 完成，缺日由服务层补零；`word_count_stats` 的区间汇总由 `StatsRepo` 完成 |
| 大文本不建索引 | `content_text`、`chunk_text`、`prompt_text`、`request_body` 等 `text` 列不建索引，正文检索依赖 Manticore |
| 排序号来源 | 章节取项目内 `MAX(chapter_order)+1`；卷在请求值 ≤ 0 时取项目内最大值 +1；大纲节点与场景在请求值为 0 时取同级最大值 +1 |

## 5. 迁移策略

- 唯一迁移入口是 `cmd/server/main.go` 的 `db.AutoMigrate(...)`，声明 36 个 DO。仓库内没有 `.sql` 文件，也没有 migrations 目录。
- AutoMigrate 语义：表不存在则建表；表已存在则补缺失的列与索引。它不做破坏性变更——不删列、不改列类型、不删索引；删除字段或调整列宽必须手工执行 DDL。
- AutoMigrate 失败（连接失败或 DDL 出错）时 `os.Exit(1)`，服务不会带半迁移状态启动。
- **遗留结构在启动期显式清理**：AutoMigrate 只补不删，因此已被取代的表与列必须在启动时手工清理。`cmd/server/main.go` 在迁移后依次执行：删表 `audit_logs`（审计已迁到 Manticore，PG 不再读写，留着会变成两份数据打架）；删 `chat_sessions.summary_text` 与 `chat_sessions.compact_at`（历史改为全量注入后不再有压缩摘要概念）；删表 `skills`、`skill_references`、`prompt_profiles`、`slash_commands`（提示词档案已合并进 `prompts` 一张表）；删列 `chat_messages.skill_code`、`chat_messages.ref_message_json`、`chat_messages.ref_prompt_json`（引用只保留「素材库 + 历史会话」两类来源）。清理前先 `HasTable` / `HasColumn` 判存，失败只告警不阻断启动。
- 数据层幂等补丁在启动期以业务键判空执行：`SeedDefault`（按供应商名或 BaseURL 判重）、`MigrateLegacyModels`（逐 provider 判断目录是否为空）、四个 `SeedDefaults`（按编码/键查缺补插）、`BootstrapAdmin`（用户表为空）。
- Manticore 侧的结构管理由 `ensureManticoreSchema` 自实现：表不存在则按 CJK ngram 结构建表；已存在但 `SHOW CREATE TABLE` 结果缺少 `ngram_len` 时 drop 重建，并从 PG 回灌全部切片。这是唯一支持的结构变更路径。
- Redis 与 RabbitMQ 无结构迁移：键按 TTL 自然过期，队列在每次投递/消费前 `QueueDeclare` 幂等声明。

## 6. 备份与恢复

| 数据 | 备份对象 | 恢复要点 |
| --- | --- | --- |
| PostgreSQL | 整库逻辑备份（如 `pg_dump`） | 恢复后启动服务，`AutoMigrate` 幂等补齐缺失的表、列与索引；播种逻辑补回内置角色、菜单、配置、提示词与管理员 |
| MinIO | 桶 `novelworkbench` | 对象键即文档 ULID，与 `knowledge_documents.object_key` 一一对应；缺对象时 `KnowledgeService.GetContent` 返回 `ERR_STORAGE`，索引任务读取原文失败后把文档置 `failed` |
| Manticore | 无需备份 | 索引是投影；恢复 PG 后重启服务即可建表，`SHOW CREATE TABLE` 通过时不会自动回灌，需重建时可先 drop `nw_chunks` 再启动，或对每个文档调 `KnowledgeService.Reindex` 重投任务 |
| Redis | 无需备份 | 只存令牌黑名单，丢失后已签发令牌在 JWT 有效期内仍可用（默认 168 小时） |
| RabbitMQ | 无需备份 | 仅传输通道；队列 durable、消息 persistent，未消费消息在重启后重投；丢失的任务可用 `Reindex` 重新投递 |

恢复顺序：PostgreSQL → MinIO → 重启服务（触发 AutoMigrate、下线清理、播种与 Manticore 建表）→ 按需重投知识库索引。
