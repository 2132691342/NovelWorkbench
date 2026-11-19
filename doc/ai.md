# AI 能力设计

本文描述 NovelWorkbench 当前的 AI 能力设计：模型网关、提示词与上下文装配、
斜杠技能、无向量检索、流式协议与用量统计。

## 1. 目标与边界

AI 能力围绕"可控的单轮生成"设计：用户提出意图，系统装配最小充分上下文，
模型流式产出，作者显式决定是否采纳。

支持范围：

- 模型协议：OpenAI 兼容协议、Anthropic 消息协议。
- 调用方式：对话生成、流式输出、供应商连通性探测、候选重排。
- 业务能力：续写、生成大纲、润色、扩写、缩写、生成梗概、审稿、取名、设定问答、自由对话。
- 检索能力：关键词召回 + 融合排序 + 可选 chat 模型重排，不依赖 embedding。

边界：

- 不做多轮 Agent 循环与工具调用。
- 不做向量化，不写入伪造向量。
- 不自动把 AI 回复写进章节或设定；插入正文必须用户显式点击。

## 2. 模型网关

模型能力集中在 `internal/ai`：

| 组件 | 职责 |
| --- | --- |
| `ai.Client` | 按 `ProviderSpec` 构造 eino 模型实例，提供 Generate/Stream/Probe/Rerank |
| `ProviderSpec` | 协议、地址、密钥、模型名、温度、输出上限、超时 |
| `UsageRecorder` | 调用开始入队，成功/失败补记用量 |
| `prompt.go` | 系统提示词、上下文块优先级与预算裁剪、模板渲染 |
| `skills.go` | 内置技能定义 |

供应商由页面管理，持久化在 `model_providers`；密钥 AES-256-GCM 加密，API 只返回掩码。
模型实例按"协议 + 地址 + 密钥摘要 + 模型名"缓存（`ProviderSpec.cacheKey`），
换密钥自动失效。配置文件 `ai.*` 只在供应商表为空时播种一条默认供应商。

协议差异由 eino 组件吸收：

- OpenAI 兼容：`eino-ext/components/model/openai`，任意兼容 `/v1/chat/completions` 的网关可用。
- Anthropic：`eino-ext/components/model/claude`，使用 Anthropic Messages API。

调用失败归类为 `ERR_AI_PROVIDER`；超时与取消传播到上游，不空转等待。

## 3. 提示词与上下文装配

系统提示词 = 固定创作助手人设 + 项目文风约束（`ai.SystemPrompt`）。
正文上下文按优先级装配（`ai.AssembleContext`）：

| 优先级 | 块 | 内容 |
| --- | --- | --- |
| 100 | `skill` | 命中斜杠命令的渲染后提示词 |
| 90 | `current_chapter` | 用户正在编辑的当前章正文 |
| 80 | `ref_chapter` / `ref_doc` | @引用的章节与文档原文 |
| 70 | `rag` | 知识库召回切片 |
| 60 | `chapter_summary` | 相邻章节摘要 |
| 50 | `character` / `lore` | 角色卡、世界观设定；私人笔记不注入 |
| 40 | `history` | 会话最近 N 条消息 |

装配规则：

1. 按优先级从高到低排序，同优先级保持输入顺序（稳定排序）。
2. 总字符预算固定 12000（`ai.ContextBudgetChars`），超预算从低优先级开始丢弃整块，不截断半句。
3. 空块直接跳过，不占预算。
4. 返回被丢弃块的标识，便于排查"模型为什么没看到某资料"，service 记告警日志。
5. 模板占位符使用 `{{key}}`（`ai.RenderPrompt`）；缺失键替换为空串，不留模板残渣。

技能块渲染时，`{{context}}` 取其余块先装配的文本，`{{chapter_content}}` 取引用章节正文拼接，
`{{user_input}}` 取解析后的用户正文。渲染后的技能块优先级最高，永远不会被丢弃。

历史条数默认 5（应用配置 `chat_history_count`），可在请求中按会话覆盖。
显式引用的消息不受条数限制，去重后按时间旧→新追加到历史尾部。

## 4. 斜杠技能

技能本质是提示词模板资产，存在 `slash_commands` 表：

| 内置编码 | 名称 | 用途 |
| --- | --- | --- |
| `continue` | 续写 | 接续当前章节正文 |
| `outline` | 生成大纲 | 根据意图生成卷/章大纲 |
| `polish` | 润色 | 改进表达与节奏 |
| `expand` | 扩写 | 扩展场景与细节 |
| `condense` | 缩写 | 压缩冗余内容 |
| `summary` | 生成梗概 | 生成章节或项目梗概 |
| `review` | 审稿 | 指出逻辑、设定与语言问题 |
| `name` | 取名 | 生成角色/地点/功法名称 |
| `setting` | 设定问答 | 基于世界设定回答 |

解析优先级：请求体 `skill_code` > 正文开头的 `/code`（`parseSlashContent`）。
命中时剥离命令 token 与参数，剩余正文作为 `user_input`；
未命中时正文原样保留，`skill_code` 为空、按自由对话处理。
内置技能不可删除，可停用；用户可在斜杠技能页新增自定义技能。

## 5. 无向量检索设计

### 5.1 为什么不用向量模型

系统只接入 OpenAI / Anthropic 对话协议，不接入向量模型。
因此检索链路必须由词法信号、结构化权威数据与 chat 模型重排组成，不能依赖 embedding 服务。

设计原则：

- 权威文本在 PostgreSQL 与 MinIO，检索索引是可重建投影。
- Manticore 提供 CJK ngram 召回，解决中文子串与专名匹配。
- chat 模型只在候选集明确、数量有限时参与重排；失败自动回退原排序。
- 每次检索输出可解释的 trace：查询词、候选数、选中 ID、来源通道。

### 5.2 索引链路

```mermaid
flowchart LR
  U[上传文档] --> M[MinIO 原文]
  M --> J[RabbitMQ 索引任务]
  J --> C[文本提取与切片]
  C --> H[内容哈希]
  H --> I[Manticore CJK ngram 索引]
  I --> D[文档状态 indexed]
```

步骤：

1. 上传接口按项目隔离，把原文写入 MinIO（对象键为 `doc_ulid`）。
2. 文档行落库 `pending`，投递索引任务到 `nw_doc_index` 队列。
3. 消费者提取 txt/md/docx/pdf 文本；按标题、段落、字符上限切片，
   默认 `chunk_size=800`、`chunk_overlap=120`。
4. 切片边界优先在句末/段落处切（`\n`、`。`、`！`、`？`、`；` 及对应半角符号），
   边界在窗口后半段时才采用，否则硬切。
5. 计算 `content_hash`；相同内容的切片不重复索引。
6. 写入 Manticore 表；更新文档状态 `indexed` 与切片数。

失败进入 `failed` 并记录原因；`/knowledge/reindex` 可重新投递。

启动时 `ensureManticoreSchema` 检查表结构：若缺少 `ngram` 参数（DROP 并按新结构重建），
再从 PostgreSQL 回灌全部切片。

### 5.3 查询构造

查询词由 `extractQueryTerms` 抽取：

- CJK 连续串按 2-gram 切分（"主角的名字" → 主角/角的/的名/名字）。
- 拉丁串按字母数字连续段切分并转小写。
- 去标点、去重。

AI 对话链路中的检索以解析后的用户正文为检索文本（`loadContextInput`），
检索范围限定在当前项目；章节正文、角色卡与设定条目不进入检索查询，
而是作为独立上下文块直接注入——这样模型看到的是确定的原文，而不是召回后的片段。

Manticore 查询（`queryManticoreHits`）分两个通道：

| 通道 | MATCH 表达式 | 说明 |
| --- | --- | --- |
| `term` | `"词1" \| "词2" \| ...` | 逐词 OR 匹配，召回面广 |
| `phrase` | `"完整查询文本"` | 整串匹配，精度高 |

两条通道都带 `project_ulid` 过滤；传了 `doc_ulid_list` 时再按文档收窄。

### 5.4 排序与重排

1. `hybrid` 模式（默认）并行跑两个通道；单通道失败时降级到另一条并记告警。
2. RRF 融合：`score = Σ 1/(60 + rank)`，常数 `rrfRankConstant=60`。
   同一切片跨通道命中时得分累加，`source_mode` 标为 `hybrid`。
   融合时按 `topK+8` 取候选，留出重排空间。
3. `keyword` 模式只跑 `term` 通道，直接用 Manticore 得分排序。
4. 可选重排：把候选 ID + 截断到 160 rune 的文本发给 chat 模型，
   要求返回严格 JSON `{"ids":["CHK_x",...]}`；解析失败、ID 不在候选集或超时（8 秒）
   一律 fail-open，沿用融合结果。
5. 最终取 TopK，默认 6（应用配置 `rag_top_k`）。

重排提示词明确要求"只返回候选 ID 与分数，不生成正文"，避免重排阶段产生不可信内容。
重排调用计入用量统计。

### 5.5 降级路径

| 异常 | 行为 |
| --- | --- |
| Manticore 不可用 | 返回 `ERR_STORAGE`；AI 对话链路中 RAG 异常降级为空，不阻断生成 |
| 文档未索引完成 | 该文档不参与检索，前端显示索引状态 |
| 重排模型失败 | 使用融合排序结果，不中断对话，`trace.rerank_note` 说明原因 |
| 查询无命中 | 上下文不含检索块，模型仍可基于章节与设定回答 |

稳定性优先：检索失败不能阻断写作，更不能让 API 返回半截 SSE。

## 6. 流式协议

SSE 事件结构（`domain.ChatStreamEventDTO`）：

| 字段 | 说明 |
| --- | --- |
| `seq` | 单调递增序号，前端对账 |
| `event_type` | `chunk` / `done` / `error` |
| `content_text` | 增量文本（chunk）或完整回复（done） |
| `reference_list` | `done` 事件携带召回依据 |
| `message_ulid` | 助手消息 ULID |
| `error_message` | `error` 事件的错误说明 |

稳定性措施：

- 客户端断开 → context 取消 → eino Stream 中止，不落库助手消息。
- nginx 对 `/api/v1/chat/send` 关闭缓冲，`proxy_read_timeout 3600s`。
- 服务写超时为 0（`write_timeout_sec: 0`），避免长连接被 Gin 切断。
- 15 秒无输出发一次心跳注释行（`: ping`），防止中间层掐断空闲连接。
- 用量缓冲 5 秒批量落库；流结束补记真实 token，缺省按 rune 数 × 0.75 估算。

## 7. 用量与可观测性

每次模型调用记录：供应商、模型、调用类型（当前仅 chat）、输入/输出 token、耗时、成功/失败。
按"日期 + 供应商 + 模型 + 调用类型"四元组在内存缓冲累加，5 秒批量按日 upsert 到
`model_usage_stats`。管理端"用量分析"页展示：

- 总调用、失败数、总 token、平均延迟、成功率；
- 近 N 天趋势（区间内缺失日补零，保证图表连续）；
- 供应商与模型分布。

平均延迟按每日调用数加权（整数运算，避免浮点漂移）。

运行状态页探测 PG、Redis、RabbitMQ、MinIO、Manticore 与默认模型，
展示 `up`/`degraded`/`down` 与探测耗时，并给出"对话能力是否就绪"的结论。

## 8. 安全与边界

- 模型密钥只保存在服务端加密字段，前端永不接触明文。
- 供应商写操作收口到 `system:model:manage` 权限；读取能力对所有登录用户开放，供 AI 面板展示。
- 知识库检索强制带 `project_ulid`，禁止跨项目召回。
- 显式引用的文档与章节必须属于当前项目，否则直接报参数错误。
- 显式引用的历史消息必须属于当前会话，否则直接报参数错误。
- 私人笔记（`lore_type_code='note'`）不注入 AI 上下文。
- AI 产出默认只进入会话消息；插入正文必须用户显式点击。
