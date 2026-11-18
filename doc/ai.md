# AI 能力设计

AI 能力全部收敛到一条链路：`/chat/send`（SSE 流式）+ 斜杠技能 + 可见的上下文装配。编辑器里的"续写/润色"等动作也走这条链路，只是预置了技能码与 @引用。

## 1. 模型接入（eino + 供应商表）

模型不再硬编码为单一网关，而是按「供应商」解析（`model_providers` 表 + `internal/ai/client.go` 网关）：

| 协议 | 组件 | 能力 |
| --- | --- | --- |
| `openai` | `eino-ext/components/model/openai` + `embedding/openai` | 对话（Generate/Stream）+ 向量（EmbedStrings） |
| `anthropic` | `eino-ext/components/model/claude` | 对话（Generate/Stream） |

- 每个供应商 = `协议 + base_url + api_key + chat_model_name +（可选）embedding_model_name`，支持多供应商与全局唯一默认供应商；页面上可测试连通性、切默认、启停。
- 网关 `ai.Client` 只接收 `ProviderSpec`（service 解密密钥后传入），**不接触存储层**；模型实例按「供应商 + 密钥摘要 + 参数」缓存，改密钥自动失效重建。
- 对话/向量调用统一路由：`Generate` / `Stream` / `Embed` / `Probe`，每次调用自动上报用量（`ai.UsageRecorder`），流式在结束时补记真实 token usage（缺省按字数估算）。
- 配置文件的 `ai.*` 只在供应商表为空时播种一条"出厂默认"供应商，之后全部由页面管理。

## 2. 提示词装配（internal/ai/prompt.go）

系统提示词 = 固定创作助手人设 + 项目 `style_text` 文风约束；正文部分由上下文块按优先级装配，超预算丢弃低优先级块：

| 优先级 | 块 | 说明 |
| --- | --- | --- |
| 100 | 技能模板 | 斜杠命令的提示词模板（渲染后） |
| 90 | 当前章节 | 正在编辑的正文 |
| 80 | @显式引用 | 用户点选的文档/章节原文 |
| 70 | 知识库召回 | 混合检索 TopK 切片 |
| 60 | 前文梗概 | 相邻章节 summary_text |
| 50 | 角色卡 / 设定 | 项目角色卡、世界观设定（`note` 类私人笔记不注入） |
| 40 | 最近历史 | 最近 N 条会话消息 |

- 字符预算 `ContextBudgetChars = 12000`（约 8K token 输入侧）；`AssembleContext` 稳定排序（同优先级保持输入顺序），返回丢弃块标识，便于问题排查。
- 模板渲染 `RenderPrompt`：`{{占位符}}` 命中替换、缺失替换为空串（避免模板残渣进提示词）。

装配策略的取舍：优先"当前创作现场"（技能 + 当前章 + 显式引用），其次"证据"（RAG + 梗概），最后"氛围"（角色/历史）。这样即使预算紧张，模型拿到的也是一致、可用的最小完整上下文，而不是被腰斩的长文本。

## 3. 斜杠技能（slash_commands 表）

- 技能 = 提示词资产：`command_code`（触发编码）+ `prompt_template`（含 `{{context}}/{{chapter_content}}/{{user_input}}` 占位符）+ 分类 + 开关。
- 内置 9 个技能由 `internal/ai/skills.go` 播种（builtin 不可删）：续写/大纲/润色/扩写/缩写/梗概/审稿/取名/设定问答；用户可在"斜杠命令管理"页新增自定义技能（创作笔记等 `note` 类条目由设定模块承载）。
- 解析规则：正文以 `/code` 开头即命中技能，剩余文本为 `user_input`；`skill_code` 请求字段优先于正文解析；未命中按自由对话处理。
- 技能只改变提示词模板与占位符填充，不改变链路本身（单链路，行为可预期）。

## 4. 引用机制

| 引用方式 | 载体 | 说明 |
| --- | --- | --- |
| @引用 | `ref_doc_ulid_list` / `ref_chapter_ulid_list` | 前端选择器显式带 ULID，服务端取原文注入优先级 80 块；引用列表随消息落库可追溯 |
| 历史引用 | `history_count`（默认 5，可配置/请求覆盖）+ `ref_message_ulid_list` | 常规历史按条数注入；显式引用的消息必注入（不受条数限制） |

显式 ID 优于文本猜测：引用全部由前端带 ULID，服务端不做"从自然语言猜引用"的不确定性解析。

## 5. RAG 检索

1. 索引侧：文档上传 → MinIO 原文 → RabbitMQ 异步任务 → 切片（`chunk_size=800`/`chunk_overlap=120`）→ 批量向量化 → pgvector + Manticore 双写。
2. 检索侧：`hybrid` 模式 = pgvector 余弦 TopK ∪ Manticore 关键词 TopK → RRF 融合（`score = Σ 1/(60+rank)`）→ 重排取 TopK（默认 6）。Manticore 使用 CJK ngram（`ngram_len=1` + `ngram_chars=cjk`）支持中文子串；请求只给 `query_text` 时关键词通道复用该文本建 MATCH。
3. 召回依据随 `done` 事件返回 `reference_list`（doc_name/切片文本/得分），前端展示"AI 依据"。
4. 降级策略：未配置向量模型（或为 Anthropic 协议）时，`hybrid`/`vector` 自动只用关键词通道；索引侧同样降级为"仅关键词索引"（切片不写向量列），配置向量模型后对该文档 `reindex` 即补齐语义能力。**绝不写入伪造向量。**

混合的原因：人名/功法名等专有名词靠关键词通道兜底，同义改写靠向量通道兜底；RRF 无权重参数，行为稳定。

## 6. 流式与稳定性

| 机制 | 实现 |
| --- | --- |
| SSE 流 | `text/event-stream`，事件带单调 `seq`；chunk 增量、done 完整回复 + 召回依据、error 错误原因 |
| 取消传播 | 客户端断开 → ctx 取消 → eino Stream 中止，不空转 |
| 超时 | 网关 `timeout_sec`；写超时 0（SSE 长连接）；nginx `proxy_read_timeout 3600s` |
| 失败语义 | 模型异常 → `ERR_AI_PROVIDER`；RAG 异常降级为无召回继续生成（生成可用性优先） |
| 用量 | 每条助手消息按字符估算 `token_used_count` 落库，供用量感知 |

## 7. 边界与已知限制

- 不做多轮工具调用/Agent 循环：工作台场景以"可控的单轮生成"为主，确定性与可预期性优先。
- 不做自动记忆写回：AI 产出不会自动改写章节/设定，写入动作一律由作者显式触发（防数据污染）。
- 上下文按字符预算估算 token（中文约 1 字 1 token 级别），不做精确 tokenizer 计数。
