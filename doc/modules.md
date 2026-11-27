# 业务模块设计

本文逐个说明 NovelWorkbench 创作业务模块的职责、数据结构、实现路径、边界与设计权衡。
每个模块固定两段：**职责与实现**（做什么、落在哪些表上、由哪些文件实现、关键规则是什么）与**边界与权衡**（不做什么、失败如何处理、为什么这样设计、代价是什么）。

平台管理模块（认证、用户角色、菜单导航、审计、应用配置、通知、任务中心）见 [platform.md](platform.md)。
接口清单见 [endpoints.md](endpoints.md)，接口约定见 [api.md](api.md)，表结构见 [database.md](database.md)，AI 细节见 [ai.md](ai.md)，模块地图见 [architecture.md](architecture.md)。

## 1. 项目 / 卷 / 章节

**职责与实现**

创作主线三级结构：`projects`（书名、题材、文风、简介、状态、累计字数）→ `volumes`（卷名、排序、概要）→ `chapters`（正文、梗概、字数、状态）。`chapter_snapshots` 存人工留档，`word_count_stats` 存按日字数增量（`word_delta` / `ai_word_delta`）。

- `service/project_service.go` 的 `Create` / `Update` / `List` / `Detail` / `Tree`；`Tree` 返回卷列表与每卷章节（升序），是写作台一次加载的数据源。`mustOwnProject` 是所有创作模块共用的归属校验：项目不存在 `ERR_NOT_FOUND`，非本人 `ERR_FORBIDDEN`。
- `ProjectService.Delete`：单个 `repo.WithTx` 事务内依次删快照、章节、卷、角色、关系、设定、大纲节点与场景卡、时间事实与伏笔与摘要、会话消息、知识切片与文档、字数统计，最后删项目；事务提交后 `cleanupKnowledgeExternal` 清 MinIO 原文与 Manticore 行，失败仅告警。
- `service/volume_service.go` 的 `Create` 缺省排序取 `MaxOrder+1`；`Delete` 先汇总卷内字数，删快照与章节后按总量负向回补项目字数。
- `service/chapter_service.go` 的 `Create` 校验卷属于该项目，`chapter_order` 取 `MaxOrder+1`，初始状态 `draft`。`ChapterService.Update`：单事务内按非空字段更新；正文变化时用 `pkg.CountWords` 重算，差值经 `applyWordDelta` 同时调整 `projects.total_word_count`（SQL 表达式增减）与当日 `word_count_stats.word_delta`。
- `SnapshotRestore`：回滚前先把当前正文写成一条 `manual` 快照，再覆盖正文并重算字数差。
- 字数口径在 `pkg/text.go` 的 `CountWords`：汉字逐字计数，连续拉丁字符算一个词，前端 `views/Workspace/wordCount.ts` 与后端同口径。

**边界与权衡**

- 卷必须属于目标项目，章节创建时卷不匹配返回 `ERR_INVALID_PARAM`；章节只能通过所属项目间接鉴权。
- 删除项目只校验归属，不额外要求权限码。
- 字数账目只在四条路径维护：章节更新、章节删除、快照回滚、卷删除；直接改库或绕过服务层不会自动修正。
- 外部存储清理在事务外，失败不回滚，可能留下孤儿对象。

取舍：章节表冗余 `project_ulid`，让「按项目取章节」「按项目鉴权」都不需要 join 卷表，代价是插入时必须保证两列一致（服务层强制）。累计字数用「增量表达式 + 每日增量表」而不是每次聚合全表：读路径 O(1)，代价是账目正确性依赖所有写入口都调用 `applyWordDelta`，一旦有遗漏就只能靠重算修复。快照采用「整章全文副本」而不是 diff，存储放大但回滚是纯覆盖，没有补丁冲突。

### 1.1 AI 开书

一句创意直接长出可写的项目骨架，解决「新建项目后面对空白卷章不知道从哪下手」的冷启动问题。

- `ai/bootstrap.go`：`bootstrapSystemPrompt` 固定输出严格 JSON（书名 / 卖点 / 简介 / 文风 / 卷章 / 角色 / 设定）；`ParseBootstrapPlan` 容忍代码围栏与前后缀文字，缺 JSON、缺书名或缺卷章一律报错，不产出半成品。
- `service/bootstrap_service.go` 的 `GeneratePlan`：经 `ModelProviderService.ChatSpecOf` 取当前账号有权限的模型（未指定则默认模型），流式累积全文后一次解析（非流式会撞网关首包超时）；首包超时与输出上限单独放宽到 300s / 8000 token。失败且章节数 >3 时缩半规模重试一次。`normalizeBootstrapPlan` 再收敛模型输出——卷 ≤3、角色 ≤8、设定 ≤10、章节数按请求截断、枚举值校正、名称按列宽截断、空名称补默认值。
- `ApplyPlan`：单个 `repo.WithTx` 事务内创建项目 → 卷 → 章节 → 角色 → 设定 → 两级大纲节点（卷节点下挂章节点），任一失败整体回滚。
- 前端 `views/Dashboard/components/BootstrapDialog.vue`：两步向导，第二步逐字段可改后确认开书，成功后直接进入新项目的写作台。

方案只生成不落库，落库必须显式二次提交，避免模型幻觉直接写进作品数据。不生成正文：章节只有名称与梗概。大纲只建卷 → 章两级。生成失败返回 `ERR_AI_PROVIDER`，不静默降级为手工模板。

选择「两段式（plan → apply）」而不是一次调用直接建库：多一次前端往返，换来作者能在落库前改掉不合适的书名与章节。枚举收敛放在服务层而非提示词里：模型对枚举值的遵从率不可控，落库前必须有一次确定性校正。

## 2. 角色卡与关系图

**职责与实现**

`characters` 存角色卡（姓名、别名、类型、性格、背景、外貌、目标动机），`character_relations` 存有向关系边（起点、终点、类型、说明），两者都以 `project_ulid` 归属。

`service/character_service.go` 的 `Save`：`character_ulid` 为空即新建，否则先校验存在且同项目，再整字段覆盖更新。`Delete`：单事务内先 `DeleteRelationsByCharacter` 再删角色，避免悬挂边。`Graph`：一次取全项目角色与全项目关系，装配 `CharacterGraphVO{NodeList, EdgeList}`。`RelationSave`：拒绝自环；两端角色必须存在且都属本项目；`RelationDelete` 按关系行反查项目做归属校验。前端 `views/Character/index.vue` 为卡片网格 + 编辑抽屉；关系图页签用 `await import('@antv/x6')` 懒加载渲染，卸载时 `dispose`。角色卡同时是 AI 上下文素材：`service/chat_context.go` 的 `formatCharacterBlock` 把性格截断 120 字、背景截断 200 字后注入。

**边界与权衡**

取舍：关系图按需计算，不落图数据库，也不做布局持久化。角色卡没有版本历史；删除角色会连带删除其全部关系边，不可撤销。注入 AI 的角色信息是截断摘要（性格 120 字、背景 200 字），角色卡选择「全量注入 + 单项截断」而不是按相关性检索，是因为作者期望 AI 认识全部主要角色。

用「两张普通表 + 内存构图」替代图数据库：规模是一本书的几十个角色、几百条边，全量加载在毫秒级，省掉一套中间件与同步逻辑。代价是无法做图算法查询，关系类型也只是自由字符串。

## 3. 世界观设定与创作笔记

**职责与实现**

`lore_settings` 一张表承载两个产品：世界观设定（`world`/`rule`/`item`/`place`/`event`）与创作笔记（`note`）。`content_mode_code` 决定正文用哪种编辑器与存储格式：`rich`（HTML）或 `markdown`。

`service/lore_service.go` 的 `Save`：新建时 `content_mode_code` 缺省 `rich`；更新时只覆盖非空字段。`Delete` / `List` 均先经 `mustOwnProject`；列表支持按类型与关键词过滤，按 `sort_order` 升序。笔记隔离有两道闸：`repo/chat_repo.go` 的 `ListLoreBriefsByProject` 在 SQL 层 `WHERE lore_type_code != 'note'`；`service/chat_context.go` 的 `formatLoreBlock` 再跳过一次。前端 `views/Lore/index.vue` 按分类展示设定卡片；`views/Notes/index.vue` 只查 `NW_LORE_TYPE_NOTE`，标题与正文 2 秒防抖自动保存，切换笔记前 `flushSave`。`views/Notes/components/NoteEditor.vue` 实现双模式：`markdown` 用 `marked` 渲染预览，`rich` 直接按 HTML 渲染原文；两种模式共用同一个 textarea 与自动保存通道。

**边界与权衡**

取舍：`note` 永不进入 AI 上下文：引用入口只覆盖知识库文档与章节，不包含设定表。正文上限 20000 字符；类型白名单非法值被参数校验拒绝。设定没有版本历史，删除即物理删除；富文本以 HTML 原样存储，不做服务端清洗（信任单作者场景）。

复用设定表承载笔记，省掉一张表、一套 CRUD 与一套前端状态；代价是「排除笔记」这条业务规则必须同时存在于 SQL 与格式化函数两处（当前刻意冗余，任一处失效仍有兜底）。私人笔记的隔离选择「SQL 层排除」而不是「提示词层过滤」，是因为前者能保证即使上下文装配被改写也不会泄漏。

## 4. 大纲树与场景卡

**职责与实现**

`outline_nodes` 是邻接表树（`parent_outline_ulid` + `sort_order`），节点类型 `volume`/`arc`/`storyBlock`/`chapter`，状态 `draft`/`planning`/`ready`/`done`。`detailed_scenes` 挂在节点下，记录出场角色 ULID 列表、地点、核心冲突、节奏、预计字数、入场状态、目标、关键选择、结果、出场状态。

`service/outline_service.go` 的 `SaveNode`：新建时 `sort_order` 为 0 则取 `MaxNodeOrder+1`，状态缺省 `draft`，并 `validateParent` 校验父节点属本项目且不成环。`validateParent`：父节点为空直接放行；父节点等于自身拒绝；随后沿 `parent_outline_ulid` 上溯，一旦发现某层的父节点是自己即判成环。`DeleteNode`：先 `CountNodesByParent`，有子节点返回 `ERR_CONFLICT`，不做级联删除。`ListTree`：一次取全项目节点，按 `parent_outline_ulid` 分桶后由 `attachOutlineChildren` 递归拼装，根节点为 `parent_outline_ulid = ''`。`SaveScene`：先校验节点属本项目，`encodeCharacterULIDs` 把角色列表序列化为 JSON；新建时排序取 `MaxSceneOrder+1`。

**边界与权衡**

- 大纲节点与真实章节是两套数据：大纲 `chapter` 类型节点不会自动创建 `chapters` 行，反之亦然。
- 删除节点必须自底向上，没有「连同子树一起删」的接口。
- 场景卡的出场角色只存 ULID 列表，不校验角色是否仍存在。
- 没有拖拽重排接口，调整顺序靠保存新的 `sort_order`。

取舍：邻接表 + 内存建树，写入是单行更新，读取是一次全项目查询，对一本书的规模足够；代价是深度查询要在应用层循环，且没有数据库级的外键或环约束。大纲与章节分离，是因为规划阶段的结构（幕、故事块）与写作阶段的卷章并不一一对应；代价是两套结构的同步只能靠作者。

## 5. 连续性

**职责与实现**

三本手工账，全部以 `project_ulid` 归属：

| 表 | 状态码 | 关键字段 |
| --- | --- | --- |
| `temporal_facts` | `candidate` / `confirmed` / `superseded` / `stale` | 主语、谓语、宾语、生效章节区间、来源章节与原文引用 |
| `foreshadow_hooks` | `planned` / `planted` / `echoed` / `resolved` / `abandoned` | 描述、预期回收、埋设/回收章节、重要度 `high/medium/low` |
| `chapter_summaries` | `pending` / `verified` / `stale` | 摘要正文、`source_hash_text`（生成时正文指纹） |

`service/continuity_service.go` 的四个校验函数是状态白名单的唯一实现，允许空值由调用方补默认（事实 `candidate`、重要度 `medium`、摘要 `pending`）。`SaveFact`：更新时状态为空则沿用原值，不允许写成非法码。`ReviewFact`：只接受 `confirmed` / `superseded` / `stale`，明确不允许退回 `candidate`。`SaveHook`：状态随显式保存推进，不做自动流转。`SaveSummary`：无 `summary_ulid` 时先按 `(project_ulid, chapter_ulid)` 查既有记录，存在则更新，保证「一章一条摘要」。

**边界与权衡**

- 全部由作者手工维护：AI 对话链路不写这三张表，也不读取它们（章节梗概走 `chapters.summary_text`）。
- `source_hash_text` 不参与自动比对，正文变化后系统不会自动把摘要置 `stale`。
- 状态流转没有强制前置约束，只有取值白名单。
- 章节 ULID 字段不做存在性校验。

取舍：把连续性做成「可人工复核的账本」而不是「AI 自动抽取的知识图谱」：结论可解释、可回溯到章节引用，且不会因为模型幻觉污染权威数据；代价是全部维护成本落在作者身上。事实复核不允许退回 `candidate`，是因为「候选」代表未经人判断的原始记录，一旦人工定性就不该伪装成未处理。

## 6. 知识库

**职责与实现**

原文进 MinIO（对象键即 `doc_ulid`），元数据与切片进 PostgreSQL，全文检索副本进 Manticore。文档状态 `pending → indexing → indexed | failed`，类型白名单 `txt/md/docx/pdf`，单文件上限默认 20MB。

- **上传**：`service/knowledge_service.go` 的 `Upload` 校验归属与扩展名 → `objectRepo.PutObject` 写原文 → 建 `pending` 文档行（失败则回滚对象）→ `enqueueIndexJob` 投递任务。
- **索引**：`service/knowledge_worker.go` 的 `indexDocument` 读原文 → `extractDocumentText`（`extractDocxText` / `extractPDFText`）→ `chunkText` 按 `ChunkSize` 切分、`ChunkOverlap` 重叠、优先在句末/换行处断点 → `contentHashOf` 与既有切片比对复用 ULID → 单事务删旧切片写新切片 → `syncManticoreChunks` 先删后插 → 文档置 `indexed` 并回写切片数与 token 数。
- **表结构**：`knowledge_manticore.go` 的 `ensureManticoreSchema` 建 CJK ngram 全文表；`SHOW CREATE TABLE` 结果里缺少 `ngram_len` 说明表不是当前结构，此时 drop 重建并由 `rebuildManticoreIndex` 从 PostgreSQL 全量回灌。
- **检索**：`service/knowledge_search_service.go` 的 `SearchWithTrace`：`normalizeSearchMode`（只有 `hybrid` 走双通道）→ `extractQueryTerms`（CJK 2-gram + 拉丁词）→ `searchKeywordChannel` 分别以 `term`（`"a" | "b"`）与 `phrase`（整串短语）查 Manticore → `fuseSearchResults` 用 RRF（`score = Σ 1/(60+rank)`）融合 → `maybeRerank` 用系统默认对话模型重排（8 秒超时，失败 fail-open 保留原序）→ `trimHits` 截断。返回 `KnowledgeSearchDTO` 同时带 `Trace`（检索词、MATCH 表达式、候选数、是否重排、入选 ID）。
- **切片回表**：`searchKeywordChannel` 拿到 `chunk_ulid` 后 `GetChunksByULIDs` 回 PostgreSQL 取正文与文档名，Manticore 只当索引不存权威文本。
- **其它入口**：`ChunkList`、`GetContent`（按 `Content-Type` 直接输出原文二进制，不走 JSON 包络）、`Stats`、`Reindex`（仅 `indexed`/`failed` 可重投）、`Delete`（先删对象与 Manticore 行，再事务删切片与文档）。

**边界与权衡**

- 不支持向量检索，也不生成 embedding；`hybrid` 指「词条 + 短语两条词法通道」，不是语义通道。
- 检索限定单项目，Manticore 查询也带 `project_ulid` 过滤；`mustOwnDoc` 对 `project_ulid` 为空或 `"0"` 的全局库文档放行。
- Manticore 是可选依赖但检索强依赖它：查询阶段单个通道失败会降级到另一通道，两个通道都失败则 `/knowledge/search` 返回 `ERR_STORAGE`，只有对话链路的 RAG 召回会吞掉该错误降级为空上下文。
- 切片入库在事务内、Manticore 同步在事务外，同步失败时两者短暂不一致，靠 `Reindex` 修复。

取舍：不引入向量模型是这套设计的根假设：部署不需要额外模型与向量库，检索结果完全可解释，索引可从 PostgreSQL 整体重建。代价是同义改写、跨语言查询召回明显弱于语义检索，只能靠 CJK 2-gram 切词、双通道 RRF 与对话模型重排补偿。用 Manticore 而不是 PostgreSQL 全文索引，是为了 CJK ngram 分词与 BM25 打分开箱可用；代价是多一个必须同步的存储与 worker、队列、任务状态机。

## 7. AI 对话

**职责与实现**

`chat_sessions`（项目 + 用户 + 会话名）与 `chat_messages`（角色、正文、思考过程、命中提示词编码、工具编码、六类引用 ULID 的 JSON、估算 token）。会话名缺省时用首条消息前 20 字回填。

`ChatService.Send`（`service/chat_service.go`）是唯一生成入口，顺序固定：

1. `mustOwnProject` → `resolvePrompt`（项目提示词覆盖同编码全局提示词；`prompt_code` 优先，否则解析正文首 token 的 `/code`）。
2. `resolveSession`：`session_ulid` 为空自动建会话，否则 `mustOwnSession` 校验归属并要求项目一致。
3. 读全会话消息，`condenseHistory` 只收敛单条超长正文（条数一条不删），返回被收敛的条数供日志观测。
4. `loadContextInput`（`service/chat_loader.go`）：引用章节（首项标记当前章）、引用文档全文、引用章节摘要、引用角色与设定、RAG 召回（失败降级为空）、项目角色、设定（排除 `note`）、当前章前后章梗概、被引用的历史会话（逐条归档）。
5. `buildContextBlocks` 组装块并给优先级：当前章 100、显式引用 90、RAG 70、章节梗概 60、角色与设定 50；历史不进块。
6. `ai.AssembleContext` 按优先级排序拼接全部块，不按总量丢弃；单块超上限时保留首尾并返回 `condensed_keys` 记日志。
7. 命中任务提示词时用 `ai.RenderPrompt` 渲染 `{{user_input}}` / `{{chapter_content}}` / `{{context}}` / `{{prompt_code}}`，得到独立的系统级指令消息。
8. 落库用户消息（含引用与 token 估算），会话名为空则回填。
9. `promptSvc.SystemPromptOf` 解析身份提示词；`buildChatMessages` 组装：身份（系统）→ 上下文（系统）→ 指令（系统）→ 历史（原生角色）→ 用户输入（用户）。
10. `providerSvc.ChatSpecOf` 取模型规格（供应商不可用时流未开始，返回 JSON 包络）；`toolDeclarations` 取启用工具的声明，读失败只告警并退化为不带工具。
11. `streamChat`（`service/chat_stream.go`）消费模型流：`thinking`/`chunk` 事件逐段下发，15 秒无输出发 `: ping` 心跳，连续 6 分钟无分片判网关已断并发 `error` 事件。
12. 生成完成后单事务写入 assistant 消息与当日 `ai_word_delta`，再发 `done` 事件并携带 RAG 依据与本轮工具编码。

SSE 协议由 `api/chat_api.go` 的 `sseChatWriter` 实现：`data: {json}` 单行事件，`begin()` 首次输出时提交 `text/event-stream`、`no-cache`、`X-Accel-Buffering: no`；事件 `seq` 由 `chatEventWriter.emit` 单调递增。

**边界与权衡**

- 单链路：没有「一键生成摘要/续写」的独立接口，所有生成都走 `/chat/send` + 提示词编码。
- 注入内容一律系统级消息，用户输入一律用户级消息：模型侧能区分「约束」与「要求」，界面侧也不会把上下文误显示成用户发言。
- 流开始前的失败（项目、提示词、会话）返回 JSON 包络且不写任何消息；但 `ChatSpecOf` 在用户消息落库之后才执行，供应商不可用时同样返回 JSON 包络，此时用户消息已留在会话里、assistant 消息缺失。
- 流开始后的失败只能经 `error` 事件下发。
- 客户端断开或生成失败时不写 assistant 消息，但已落库的用户消息保留。
- 引用文档必须是本项目或全局库，跨项目返回 `ERR_FORBIDDEN`；引用项目提示词同理。
- 历史全量注入：不按条数截取、不按总预算丢弃；单条正文超 4000 字时保留首尾并标注省略字数。会话与上下文块都不再「超预算静默丢弃」。
- 工具声明读失败时退化为「本轮不带工具」，不阻断对话。
- `chat/send` 在审计跳过名单内，用量统计仍由 `ai.Client` 上报。

取舍：单链路把所有生成收口到一处，上下文装配、容错、流式协议、落库与统计只有一份实现，排错路径唯一；代价是交互上多一步「生成 → 手动插入」。上下文用「全量注入 + 单条首尾收敛」而不是「条数滑窗 + 预算丢弃」：不会丢掉任何一条历史或任何一个块，行为确定且不用二次调用模型；代价是上下文更长、token 消耗更高，这是刻意选择的成本。RAG 失败降级为空上下文而不是报错，是保证「对话永远可用」的取舍，代价是回答质量静默下降。

## 8. 提示词资产

**职责与实现**

`prompts` 是可编辑的提示词资产：编码（对话中以 `/code` 触发，工作流按编码引用）、名称、种类（`system` 身份 / `command` 任务）、作用域、分类、正文 `prompt_text`、启停与内置标记。身份与指令同表同构，靠 `prompt_kind_code` 区分——它们本就是同一种东西（一段可编辑、可注入、可停用的提示词），拆成两张表只会让"哪一段真的生效"变难查。

`PromptService.List`：`project_ulid` 非空时合并「项目提示词 + 全局提示词」，并用 `isOverriddenGlobalPrompt` 给被覆盖的全局提示词打 `overridden_flag`。`PromptService.Save`：作用域缺省 `global`；项目提示词必须带项目并经 `mustProjectAccess` 校验项目存在（不校验创建者，方便团队协作）；`validatePromptPlaceholders` 要求正文占位符都在内置白名单内；同一作用域下 `prompt_code` 唯一；内置提示词不允许改编码、作用域与种类。`resolveByCode` / `pickPromptByScope`：启用中的项目提示词优先，其次启用中的全局提示词，都不命中则报「提示词不存在或已停用」。`resolveSlashCode` / `parseSlashToken`：显式 `prompt_code` 优先，否则只解析正文**首个 token** 的 `/code`，命中则剥离命令，未命中按自由对话处理。

**边界与权衡**

- 内置提示词不可删除（可停用），可改名称、说明、正文与排序号。
- 只识别正文首个 token 的斜杠命令，正文中间出现的 `/xxx` 不会触发提示词。
- 项目提示词的作用域校验只看「项目存在」，不看归属——同一项目的协作者都能维护项目提示词，这是刻意的团队协作放宽。
- 占位符只支持四个内置变量；未登记的占位符在保存期就被拒绝，避免运行期静默渲染成空串。
- 提示词不按关键词自动命中参考资料：作者用 `@` 显式挑选素材或历史会话，模型就只看到被选中的内容，行为更可预期。

取舍：把提示词做成数据而不是代码，让作者不改代码、不发版就能调提示词；代价是占位符一致性必须在保存期校验。作用域覆盖规则让「全局默认 + 项目特化」成为可能，代价是列表页必须显式标注「被覆盖」。内置资产刻意只保留 4 个创作流程共 8 条（见 [prompts.md](prompts.md)）：多步骤编排交给[工作流](workflow.md)，提示词只描述单次任务，作者在 `/` 菜单里一眼能看懂。

## 9. 提示词注入与身份解析

**职责与实现**

身份提示词决定「AI 是谁」（可选自建），任务提示词决定「这一单活怎么干」（内置 8 条）。二者都以**系统级消息**进入消息序列，用户在对话框输入的内容始终是**用户级消息**。

`SystemPromptOf(projectULID, systemPromptCode)` 的解析顺序：显式身份提示词编码（须启用且为全局或属于当前项目）→ 启用的全局默认身份提示词（`GetDefaultSystem`，排序号最小）→ `ai.BaseSystemPrompt` 内置作者人格。项目层再叠加 `projects.style_text`（`ai.ComposeSystemPrompt`）。库里没有任何身份提示词时，兜底人格保证系统消息非空。

**边界与权衡**

- 身份提示词不受上下文块收敛影响：它走 `system` 角色，是独立的一条消息，不参与 `ai.AssembleContext` 的单块收敛。
- 项目提示词的归属校验只看项目存在，与提示词库一致；对话期 @ 引用项目提示词时会再校验一次 `project_ulid` 匹配，防止跨项目引用。
- 身份与任务提示词同时下发、互不覆盖：身份写底线，任务写动作，模型看到的是「以什么身份 + 这次做什么」。
- 身份提示词不再内置：旧 `novelist` / `reviser` / `planner` 由 `dropRetiredBuiltins` 在启动播种时清掉，作者要固定风格需自建一条 `system` 提示词。

取舍：把系统提示词做成可编辑资产，让「换一种 AI 身份」不需要改代码；代价是每次对话多一次身份提示词查询（单行按编码/默认排序取，代价可忽略）。

### 9.1 工具资产

**职责与实现**

`tools` 表存「对话期可被调用的能力单元」。与提示词的分工是：提示词让模型**按格式写**，工具让服务端**先算出事实**，结果以 tool 消息回给模型，模型据此作答，避免自己心算或编造。

`ai/builtin_tools.go`：6 个内置工具定义（`chapter_check`、`deai_scan`、`chapter_digest`、`character_scan`、`foreshadow_scan`、`lore_lookup`）与 `ContentHash` 指纹。`service/tool_service.go` 的 `SeedDefaults`：按 `tool_code` 查缺补插；指纹变化且未自改时覆盖；已下线编码删除。`Invoke`：按编码取工具 → 校验启用 → `execute` 分发（各执行器在 `service/tool_executor.go`）。`chapter_check` / `deai_scan` 复用 `ai/check.Analyze`；`chapter_digest` 抽取每段首句并取章末钩子；`character_scan` 统计角色卡姓名（含别名）在本章的命中；`foreshadow_scan` 用双字片段粗匹配伏笔相关词；`lore_lookup` 按关键词查设定并剥掉富文本标签。结果统一截断到 4000 字符。

**调用由模型决定，不由命令注入**：`ToolDeclarations` 把启用工具转成 eino `ToolInfo`（function name 即工具编码）随请求下发，模型据此自行决定调不调、调哪个、传什么参数。服务端不按关键词猜测、不接受请求体点名，`#` 触发符已移除；`tool_code_list` 只在响应侧（`done` 事件与消息头）回显本轮实际调用的工具，不是请求入参。

一次往返的形态：

```
消息序列 + 工具声明 → 模型流式生成
  ├─ 只出正文          → 直接落库，done 事件带 tool_code_list=[]
  └─ 回 tool_calls     → 服务端执行 → tool 消息回填 → 继续生成（最多 4 轮）
```

四道防线防止模型在工具上打转：

| 机制 | 作用 |
| --- | --- |
| 单轮去重 | 同一轮里「相同工具 + 相同参数」只执行一次，其余复用首次结果 |
| 跨轮检测 | 相邻两轮的调用签名相同即判定原地打转，摘掉工具声明逼模型用已有事实作答 |
| 轮数上限 | `chatToolRoundLimit = 4`，超限后交回已有正文 |
| 失败降级 | 单个工具失败把原因写进 tool 消息，由模型自己说明，不中止对话 |

**边界与权衡**

取舍：只支持内置类型：自定义工具需要服务端实现，因此新建入口仅对管理员开放且类型固定。工具不写业务数据：全部是只读计算，`/tool/invoke` 不产生章节、消息或统计变更。模型调用工具不校验权限码，只看工具 `enabled_flag`；参数缺失时由执行器给明确提示（如「请提供正文或指定章节」），不抛 panic。

把「可机检的门禁」从提示词搬进代码，换来三件事：结果可复现（同一正文永远同一结论）、可单测（`ai/check` 是纯函数）、零模型成本。代价是规则改动需要发版，因此阈值集中定义在 `ai/check/chapter_check.go` 顶部常量。并且工具只做「发现问题」，改稿仍由提示词完成——工具给依据，提示词给产出。

### 9.2 自动化工作流

**职责与实现**

工作流是产品主路径：作者准备素材与要求，选模板、填几个参数，AI 按步骤跑完，人只在审核点做「通过 / 退回」。完整设计见 [workflow.md](workflow.md)，此处只记实现位置与边界。

| 文件 | 职责 |
| --- | --- |
| `domain/workflow.go` | 模板 / 运行 / 步骤的 DO、REQ、VO 与状态枚举 |
| `repo/workflow_repo.go` | 三张表的 CRUD、按状态查、按项目统计 |
| `service/workflow_template.go` | 三条内置模板定义与步骤 JSON 序列化 |
| `service/workflow_service.go` | 模板播种、运行创建、列表、详情、统计 |
| `service/workflow_params.go` | 参数收敛、目标章节解析、空白章节创建 |
| `service/workflow_engine.go` | 执行器：goroutine 调度、逐章推进、审核门禁、失败处理 |
| `service/workflow_steps.go` | 各步骤实现（出稿/体检/复审/摘要/入库）与门禁判定 |
| `service/workflow_context.go` | 章节上下文装配：项目设定、人物关系、世界观、大纲节点、场景卡、前情摘要、伏笔 |
| `service/workflow_readiness.go` | 开跑就绪度：九项素材/配置检查、评分、补齐路由 |
| `service/workflow_control.go` | 审核通过/退回、暂停/继续/取消/重试、中断恢复 |
| `service/workflow_helper.go` | 步骤记录、门禁收口动作（原地更新 / 回退插入）、参数解码与状态判定 |
| `api/workflow_api.go` | 13 个接口的 HTTP 处理器 |

**边界与权衡**

- 候选正文（`workflow_runs.candidate_text`）与章节正文分离：AI 产出不直接写作品数据，只有 `apply` 步骤或人工审核通过才落库，覆盖前自动留快照。
- 体检复用 `ai/check.Analyze`，与对话期 `chapter_check` 工具、写作台体检面板共用同一实现，三处判定不会漂移。
- 上下文装配（`workflow_context.go`）逐块取数、逐块限长、取不到就跳过：缺素材只让产出变差，不该让整次运行失败。除正文外各块合计控制在 3000 字内。
- 就绪度只提示不硬拦：`Readiness` 给出分数、阻塞项与 `fix_route`，前端在页面顶部和向导第 2 步展示；作者坚持要跑时仍可继续，但必须先看到警告。检查项编码是前后端契约，改动要同步前端图标映射。
- 待审核收件箱（`GET /workflow/run/pending`）把「AI 停下来等人」收敛成一个固定落点，工作台首页直接给「去审核」入口，并用 `?run=` 让工作流页自动打开对应抽屉。
- 执行器是进程内 goroutine，不是分布式队列：单实例部署足够，代价是重启时需 `RecoverInterrupted` 把 running 退回 queued。
- 同一运行用进程内 `activeSet` 去重，重复 start 直接忽略；多实例部署时需换成分布式锁（当前未做，因为部署形态是单实例）。
- 运行总时长 = 60 分钟 + 每章 20 分钟：推理模型单章可能跑十几分钟，固定上限会让长任务必然中断。
- 模板结构由代码定义并在启动时同步，数据库只存快照；这样执行引擎与模板定义不会出现版本错配。

取舍：不引入外部编排引擎（Temporal / Airflow）与 MCP。步骤是代码里的数组，状态在三张 PG 表里，用最简单的机制换「可读、可调、可测」——本项目的工作流复杂度远未到需要通用引擎的程度。

## 10. 模型生态

**职责与实现**

三层结构：供应商（`model_providers`：名称、协议、BaseURL、加密 API Key、启停、测试状态）→ 模型目录（`model_catalogs`：模型名、标签、温度、max tokens、上下文窗口、默认标记、测试状态）→ 授权（`model_role_grants` 角色 × 模型、`model_user_grants` 用户 × 模型）。

| 规则 | 实现 |
| --- | --- |
| 协议 | `openai`（`/v1/chat/completions` 兼容）与 `anthropic`（`/v1/messages`），Anthropic 的 BaseURL 自动剥掉尾部 `/v1` |
| 密钥 | `pkg.SealSecret` / `OpenSecret` 用 JWT 密钥做 AES-GCM 封存，读取只回掩码 |
| 默认模型 | 全局唯一，设置前先 `ClearDefaultModel`；默认模型不可删除 |
| 授权 | 角色授权与用户直授都可整体覆盖，可用集合取并集；管理员（`*`）无需授权即可见全部启用模型 |
| 关联清理 | 删用户 → `DeleteUserGrantsByUser`；删角色 → `DeleteRoleGrantsByRole`；删模型 → `DeleteGrantsByModel`。不存在悬空授权 |
| 去重 | `service/model_catalog_dedupe.go` 按 `(provider_ulid, model_name)` 与 `provider_name` 收敛重复行（保留有引用/手工/更新的行，迁移授权与默认标记），`main.go` 在 AutoMigrate 前后各跑一次；唯一冲突返回 `ERR_CONFLICT` |

`service/model_provider_service.go` 的 `SeedDefault`：配置了网关且按供应商名或 BaseURL 查不到同名/同网关行时播种「出厂默认」，并授权给全部内置角色；`MigrateLegacyModels` 逐 provider 判断，只把「仍带旧列 `chat_model_name` 且尚无目录行」的供应商补成目录行（幂等）。`SaveProvider`：首次创建必须填 Key；更新时 Key 传空表示保持不变。`TestProvider` / `TestModel` → `probe`：构造 `ProviderSpec` 后调 `gateway.Probe` 跑一次极小对话，结果（状态、消息、延迟、回复）回写 `last_test_*` 字段。`DeleteProvider`：其下仍有模型时拒绝。`service/model_catalog_service.go` 的 `SaveModel` / `DeleteModel` / `SetDefaultModel` / `TestModel` / `GrantModel`：`temperatureOf` 缺省 0.7；`GrantModel` 同时覆盖两张授权表。`service/model_access_service.go` 的 `usableModelList`：管理员看全部「模型启用且供应商启用」的模型，其他用户取「角色授权 ∪ 用户直授」再过滤；`pickModel` 依次尝试指定 ULID、`default_flag`、第一个可用。`ChatSpecOf` / `DefaultChatSpec` / `Capability`：前者给对话链路（含授权与停用校验），中者给检索重排等后台链路（只看系统默认模型），后者给前端模型下拉。

**边界与权衡**

- 不支持向量模型与 embedding 协议，只有对话协议。
- 没有可用模型时 `ChatSpecOf` 返回 `ERR_AI_PROVIDER`，指定了未授权模型返回 `ERR_FORBIDDEN`。
- 授权粒度是「角色 + 用户」两档：单用户特例直接给用户直授，不必新建角色；用户直授独立于角色，改角色不影响它。
- 密钥解密失败时返回 `ERR_BIZ_FAILED` 并提示重新填写。
- 供应商/模型的测试结果只是快照，真正可用性判断看 `enabled_flag` 与授权。

取舍：「供应商承载凭据与网关、模型是最小使用与授权单位」，让一个 Key 下挂多个模型成为常态；代价是管理端要维护三张表与两个抽屉，配置路径变长。授权每次对话实时计算，换来「改授权立即生效、无需重新登录」；管理端流量低，这个成本可以接受。密钥用 JWT 密钥做对称封存，避免额外密钥管理服务，代价是轮换 JWT 密钥会导致已存密钥无法解密。

## 11. 数据总览

**职责与实现**

只读聚合：项目数、累计字数、今日净增字数、AI 累计字数、近 30 天趋势。数据来自 `projects.total_word_count` 与 `word_count_stats`，趋势天数常量 30。

`service/stats_service.go` 的 `Overview`：`project_ulid` 非空时先 `mustOwnProject`；随后五次查询取项目数、累计字数、今日净增、AI 累计与趋势。`trendList` 取区间内有数据的日期，再按天循环补零，保证 30 个点连续。`repo/stats_repo.go` 的所有查询都 `JOIN projects` 并按 `user_ulid` 过滤。前端 `views/Dashboard/index.vue` 用 ECharts 画柱（`word_delta`）+ 折线（`ai_word_delta`）双系列。

**边界与权衡**

取舍：不做实时重算，数值完全信任 `projects.total_word_count` 与 `word_count_stats`；没有缓存与预聚合表，每次打开仪表盘都跑聚合查询。用「写入时记账 + 读取时求和」替代「读取时扫章节表」：趋势查询只碰一张窄表，30 天数据量恒定；代价是写入路径必须维护两处账，任何绕过服务层的写入都会让统计漂移。补零放在服务层而不是 SQL，是为了让「没有数据的那天」也能返回完整序列。`ai_word_delta` 只统计对话生成的字数，不含「插入章节」后的正文。

## 12. 用量分析

**职责与实现**

`model_usage_stats` 按「日期 × 供应商 × 模型 × 调用类型」唯一聚合调用次数、成功/失败数、输入/输出/总 token、耗时总和、用量未知次数。当前调用类型只有 `chat`。

记账口径：一次模型调用只产生一条用量记录，调用次数与 token 必然落在同一条上。流式调用不在首包记账，而是在流结束（正常读完或出错）时一次性上报成功/失败标志、真实 token 与耗时（首包→流结束）；`ai.StreamUsage` 汇总各分片 `ResponseMeta.Usage` 的最大值，网关全程未返回用量时该次调用记 `unknown_usage_count + 1` 而 token 记 0，从数据上区分「网关不给用量」与「记账漏了 token」。非流式调用由 `Generate` 在返回时一次上报。

`service/model_usage_service.go` 的 `NewModelUsageService` 启动 `flushLoop`；`RecordUsage` 在内存 map 中按维度累加，`sync.Mutex` 保护。`flushLoop` 每 5 秒 `Flush` 一次，`ctx` 取消时做最后一次落库；`Flush` 先整体换出新 map 再逐条 `UpsertIncrement`，单条失败只告警并丢弃该批。`Overview`：查询前先 `Flush`；`resolveUsageRange` 缺省近 14 天、非法日期回退默认、起止倒置自动交换；`fillUsageDaily` 补零；`weightedAvgLatency` 用每日调用数加权求平均延迟。上报方：`ai/client.go` 通过 `ai.UsageRecorder` 接口调用，`cmd/server/main.go` 用 `aiClient.SetUsageRecorder(usageService)` 注入，避免 `ai` 依赖存储层。前端 `views/Admin/Usage.vue` 提供 7/14/30 天预设、四项指标、双轴趋势图与两张分布表。

**边界与权衡**

- 调用类型只有 `chat` 一种：连通性探测与检索重排内部都走 `Client.Generate`，因此同样以 `chat` 计入，无法区分「用户对话」与「系统自用」。
- 进程崩溃会丢失未 flush 的缓冲（最多 5 秒窗口）；`Flush` 失败直接丢弃该批，不做重试队列。
- 不做用户维度统计。
- 需要 `system:usage:read` 权限。

取舍：内存缓冲 + 批量 upsert 把高频小请求的写放大压到 5 秒一次，代价是弱持久性与内存态（多实例部署时各实例各自缓冲、无法合并同维度累加），进程崩溃会丢失最近一个刷新周期的用量。查询前强制 flush 是补上「刚调用完看不到」的体验缺口。按天聚合而不是存原始调用日志，让表体量恒定、查询便宜，但丧失了事后按任意维度重算的能力。

## 13. 运行状态

**职责与实现**

实时探测五个外部依赖 + 一个模型能力结论，附带进程信息（启动时刻、运行时长、Go 版本、构建版本）。不落库。

| 依赖 | 必需 | 探针实现 |
| --- | --- | --- |
| PostgreSQL | 是 | `sqlDB.PingContext` |
| Redis | 否 | `rdb.Ping`（未配置时报错） |
| RabbitMQ | 否 | 连接状态 + `DeclareQueue` |
| MinIO | 否 | `BucketExists` |
| Manticore | 否 | `manticoreClient.Ping` |
| 模型供应商 | 否 | 复用 `ModelProviderService.Capability` |

`service/system_status_service.go` 的 `Status`：固定五项探针放进 `sync.WaitGroup` 并发执行，结果按下标写回保证顺序；随后追加一条 `model` 健康项。`probeDependency`：探针为 `nil` 记 `degraded`「未配置」；执行成功记 `up` 并记录耗时；失败时必需依赖记 `down`、其余记 `degraded`。探针由 `cmd/server/main.go` 以闭包形式注入（`SystemProbeDeps`），service 不直接持有各中间件客户端。

**边界与权衡**

取舍：探测失败不影响接口本身：`Status` 永远返回 200 与一份完整清单。没有历史与告警，只反映调用瞬间的状态。与 `/health`、`/ready` 分工不同：前者只回存活，后者只 Ping PostgreSQL，本模块是给人看的面板。

并发探测把总延迟压到最慢一项，代价是每次刷新同时打六个外部依赖——因此前端坚持手动刷新，不做轮询。探针以闭包注入而非让 service 依赖 infra 客户端，保持了分层单向。只给「up/down/degraded」三态而不做更细的错误分类，是让运维一眼看出「能不能用」。

## 14. 前端工作台装配

**职责与实现**

前端不重复实现业务规则，只做三件事：装配后端下发的结构（导航、权限码、可用模型）、维护跨页上下文（当前项目、未读数）、承载交互（编辑器、流式渲染、抽屉表单）。设计系统与页面实现见 [frontend.md](frontend.md)，本节只说明与后端模块的装配关系。

| 装配点 | 数据来源 | 消费位置 |
| --- | --- | --- |
| 侧边栏导航 | `GET /menu/nav` 的 `menu_tree_list` / `perm_code_list` | `store/modules/user.ts` 的 `loadNav` → `components/Menu/index.vue` |
| 项目上下文 | 路由参数 `:project_ulid` 优先，其次 `store/modules/project.ts` 选中态 | `layout/Layout.vue` 的 `syncFromRoute` |
| 可用模型 | `GET /model/capability` | `views/Workspace/AiPanel.vue` 的 `modelOptions` |
| 会话与消息 | `POST /chat/session/list`、`POST /chat/history` | `views/Workspace/AiPanel.vue` |
| 流式生成 | `POST /chat/send`（fetch + `ReadableStream`） | `composables/useSSE.ts` 的 `streamSsePost` / `parseSseBuffer` |
| 章节正文 | `GET /chapter/detail/{ChapterULID}`、`POST /chapter/update` | `views/Workspace/ChapterEditor.vue`（CodeMirror 6，2 秒防抖保存） |
| 目录树 | `GET /project/tree/{ProjectULID}` | `views/Workspace/WorkspaceTree.vue` |
| 未读通知 | `GET /notification/unread_count`、`POST /notification/list` | `store/modules/notification.ts` → `NotificationBell.vue` |

写作台三栏：`views/Workspace/index.vue` 组合 `WorkspaceTree` / `ChapterEditor` / `AiPanel`，左右栏可独立折叠。AI 回复落正文是纯前端动作：`AiPanel` 发 `insert-to-chapter` 事件 → `index.vue` 的 `insertToChapter` → `ChapterEditor.insertTextAtCursor`，随后走编辑器同一自动保存通道。权限判断只读 `perm_code_list`（含 `*`）：路由守卫调 `store/modules/user.ts` 的 `hasPerm`，菜单可见性由后端导航树预先过滤。

**边界与权衡**

- 前端不拼装 AI 上下文：`@` 引用只传 ULID 列表，素材加载、截断、收敛全在后端。
- 前端不计算字数权威值：编辑器用 `wordCount.ts` 做即时反馈，落库字数以服务端 `pkg.CountWords` 为准。
- 前端不缓存业务列表：切页即重新请求，只有导航树、权限码、当前项目与未读数跨页保留。
- `api/contracts.ts` 是契约的手工镜像，没有生成器，字段改名需两边同步。

取舍：导航完全由后端下发，改菜单即改侧边栏，代价是登录后多一次 `nav` 请求、角色变更后必须重新拉取。SSE 用原生 `fetch` 而不是 Axios（Axios 不支持流式响应体），代价是要自己实现按行解析与半行缓冲。编辑器自动保存 2 秒防抖而不是逐键保存，减少写请求并避免字数账目抖动，代价是极端情况下可能丢最后两秒输入，因此切换章节前显式调用 `flushSave`。

