# AI 子系统设计

本文描述 NovelWorkbench 的 AI 能力：模型网关、提示词资产与注入、上下文装配、无向量检索、流式协议与用量统计。

## 1. 定位与边界

AI 能力按「单轮可控生成」设计：用户给出意图，服务端装配最小充分上下文，模型流式产出，作者显式决定是否采纳。

| 维度 | 内容 |
| --- | --- |
| 协议 | OpenAI 兼容（`openai`）、Anthropic 原生（`anthropic`） |
| 调用形态 | 非流式生成、流式生成、连通性探测、候选重排 |
| 业务能力 | 续写正文、搭建大纲、润色文字、章节梗概、去 AI 味、章节质检、开篇体检、伏笔台账 |
| 检索 | Manticore CJK ngram 词法召回 + RRF 融合 + 可选对话模型重排 |

明确不做：

| 不做 | 原因 |
| --- | --- |
| 向量化检索 | 无 embedding 服务；检索完全基于词法通道与结构化数据 |
| AI 产出自动写入正文 | AI 结果只落 `chat_messages`，插入章节由前端显式触发 |

## 2. 模型网关

### 2.1 组成

| 符号 | 位置 | 职责 |
| --- | --- | --- |
| `ai.Client` | `backend/internal/ai/client.go` | 按 `ProviderSpec` 构造/复用 eino 对话组件，统一上报用量 |
| `ai.ProviderSpec` | 同上 | 单次调用所需供应商快照；`APIKey` 明文只存在于内存 |
| `ai.UsageRecorder` | `backend/internal/ai/usage.go` | 用量上报接口，由 `service.ModelUsageService` 实现 |
| `ai.AssembleContext` / `ai.RenderPrompt` / `ai.SystemPrompt` / `ai.TemplatePlaceholders` | `backend/internal/ai/prompt.go` | 上下文装配、模板渲染与校验、系统提示词 |
| `ai.BuiltinDomains` / `ai.BuiltinCommandPrompts` / `ai.BuiltinSystemPrompts` | `backend/internal/ai/prompt_assets.go` + `domain_<域>.go` | 12 个流程域的身份提示词与 24 份指令提示词、下线集合与内容指纹，与持久化结构解耦 |

`ProviderSpec` 字段：`ProviderULID`、`ProviderName`、`ProtocolCode`、`BaseURL`、`APIKey`、`ChatModelName`、`Temperature`、`MaxTokens`、`Timeout`。密钥由 `ModelProviderService.specOf` 解密后传入，`ai` 包不接触存储。

### 2.2 实例缓存与密钥失效

`ProviderSpec.cacheKey()` 拼装：

```
providerULID | protocolCode | baseURL | chatModelName | sha256(apiKey)[:8] | temperature(%.2f) | maxTokens
```

密钥以摘要入键而非明文：更换密钥后键变化，旧实例自然失效，不会用旧凭据继续调用。
`Client.chatModelOf` 用 `sync.RWMutex` 保护 `map[string]*providerModels`；缓存项 `chat == nil` 视为构造失败，下次重建。

### 2.3 协议适配

| 协议 | 常量 | eino 组件 | 备注 |
| --- | --- | --- | --- |
| OpenAI 兼容 | `ai.ProtocolOpenAI` = `openai` | `eino-ext/components/model/openai` | 适配 `/v1/chat/completions` 网关 |
| Anthropic 原生 | `ai.ProtocolAnthropic` = `anthropic` | `eino-ext/components/model/claude` | `base_url` 末尾的 `/v1` 在保存时被剥离，SDK 自行拼接 `/v1/messages` |

其他协议编码在 `buildChatModel` 返回 `unsupported protocol`。

默认参数（`internal/ai/client.go` 常量，`spec` 缺省时兜底）：`defaultMaxTokens = 2048`、`defaultTemperature = 0.7`、`defaultTimeout = 120s`。

### 2.4 五种调用

| 方法 | 行为 | 用量上报 |
| --- | --- | --- |
| `Generate` | 非流式生成，返回 `*schema.Message` | 结束即上报成功/失败、token、耗时 |
| `Stream` | 首片到达即返回 `*schema.StreamReader` | 首包记一次成功调用；流结束由 `RecordStreamUsage` 补记真实 token |
| `Probe` | 连通性探测（系统「只回复两个字」+ 用户「回复：通」，30s 超时），返回前 40 rune | 走 `Generate`，正常计费 |
| `Rerank` | 对话模型按候选 ID 重排，严格 JSON | 走 `Generate`，正常计费 |
| `GenerateBootstrapPlan` | 开书方案：流式累积全文后一次性解析 JSON | 走 `Stream`，流结束补记 token |

`GenerateBootstrapPlan`（`ai/bootstrap.go`）不用 `Generate`：结构化长输出（书名 + 卖点 + 卷章 + 角色 + 设定）在非流式下会长时间不返回响应头，网关 120s 首包超时必然打断。改成流式累积后，首包很快到达，总时长由调用方 `ctx` 控制（`bootstrapPlanTimeout = 300s`），`MaxTokens` 提到 8000 以容纳完整方案。

超时分层（关键：推理模型思考常超过 2 分钟，任何「整体请求超时」都会把长流拦腰切断）：

| 环节 | 约束 | 说明 |
| --- | --- | --- |
| HTTP 客户端 | 不设 `http.Client.Timeout` | 该字段含读 body，会把长流切断 |
| 连接与首包 | `newStreamHTTPClient(timeout)`：拨号 15s、TLS 10s、`ResponseHeaderTimeout` = 供应商超时 | 首包迟迟不来才判失败 |
| `Generate` | 是（`context.WithTimeout`） | 单次非流式调用有硬上限 |
| `Stream` | 否（仅 `ResponseHeaderTimeout`） | 生成时长由客户端 `ctx` 控制 |
| 流空闲看门狗 | `chatStreamIdleTimeout = 6min` | 连续无任何分片才判网关已断，经 `error` 事件告知 |

同一 HTTP 客户端被 Generate / Stream / Probe / Rerank 共用，连接池上限 `MaxIdleConns=32`、`MaxIdleConnsPerHost=8`。

### 2.5 重排解析

`parseRerankIDs` 的规则：

1. 回复以 Markdown 代码围栏开头 → 直接判失败（拒绝包裹输出）。
2. 取首个 `{` 到末个 `}` 之间文本，`json.Unmarshal` 为 `{"ids":[...]}`。
3. ID 必须属于候选集且不重复，否则整体失败。
4. 模型未返回的候选按原顺序补在尾部，保证不丢候选。

失败即 fail-open：`Rerank` 返回原顺序并给出说明（`rerun unavailable: …` / `rerun fallback: …`）。

## 3. 上下文装配

### 3.1 提示词注入：身份与指令分层

提示词只有一种资产，用 `prompt_kind_code` 分两类，**都以系统级消息下发**：

1. **身份提示词（`system`）**：`PromptService.SystemPromptOf` 解析顺序为「请求显式 `system_prompt_code` → 启用的全局默认身份提示词（排序号最小）→ `ai.SystemPrompt("")` 内置兜底」。显式编码必须启用，且为全局或属于当前项目，否则视为未指定。
2. **指令提示词（`command`）**：由 `/编码` 或请求体 `prompt_code` 命中，渲染后作为独立的系统级消息插在上下文之后、历史之前。

项目层约束：`projects.style_text` 非空时以「本项目文风约束」段落追加在身份提示词之后（`ai.ComposeSystemPrompt`）。

分工约定：身份写「你是谁、什么绝不写」，指令写「这一次做什么、按什么格式交付」。二者同时下发，互不覆盖。

内置资产按「流程域 = 1 份身份 + 2 份指令」组织，完整矩阵见 [prompts.md](prompts.md)。

| 身份编码 | 名称 | 定位 |
| --- | --- | --- |
| `novelist` | 长篇连载作者 | 默认身份：只交稿不解释、具体优先、章末留钩 |
| `architect` | 故事架构师 | 六层结构自上而下、每层给最薄弱处与补齐顺序 |
| `stylist` | 文字打磨师 | 只改怎么说不动说什么、事实锁与声线锁 |
| `deai_editor` | 去 AI 味修订 | 五层对抗、命中必引原句、误杀防护 |
| `editor` | 连载责任编辑 | 先判后改、P0/P1/P2 分级、给可直接替换的改法 |
| `continuity` | 设定守门人 | 只依据给定上下文作答、缺失即明说、冲突给三种收敛方案 |
| `final_editor` | 定稿编辑 | 只做归档与对账、不改稿、不把推测写成事实 |
| `serial_editor` | 连载平台主编 | 按第三章/第三十章/第一百章判断留存与商业化 |
| `pitch_analyst` | 选题策划 | 开书前只答值不值得写、判断必带代价 |
| `platform_operator` | 平台运营 | 书名与简介只对点击负责、不承诺数据 |
| `short_story_author` | 短篇作者 | 一次完整呼吸、结尾留余震、禁 cliffhanger |
| `screenwriter` | 改编编剧 | 可见优先、保留情感功能、材料不足即声明假设 |

提示词表 `prompts` 统一承载身份与指令（作用域 + 编码唯一 + 内置不可删 + 可停用），由 `PromptService.SeedDefaults` 幂等播种并做指纹升级，管理端在「提示词库」页维护。

### 3.2 块优先级

| 优先级 | 常量 | BlockKey | 内容来源 |
| --- | --- | --- | --- |
| 100 | `PriorityCurrentChapter` | `current_chapter` | 当前编辑章节正文 |
| 90 | `PriorityExplicitRef` | `ref_chapter` / `ref_doc` / `ref_summary` / `ref_character` / `ref_lore` / `ref_session` | @引用章节 / 文档全文 / 章节摘要 / 角色卡 / 设定条目 / 历史会话摘要 |
| 70 | `PriorityRAG` | `rag` | 知识库召回切片 |
| 60 | `PriorityChapterSummary` | `chapter_summary` | 相邻章节梗概 |
| 50 | `PriorityCharacter` | `character` / `lore` | 角色卡 / 设定条目（`note` 排除） |

历史不进上下文块：它以原生角色消息追加到消息序列，两处同时注入会让模型看到两份对话。

### 3.3 装配规则

`ai.AssembleContext(blocks)`：

1. 按优先级降序稳定排序，同优先级保持输入顺序，输出确定。
2. 块间以 `\n\n` 分隔；空内容块直接跳过。
3. **不按总预算丢弃**：所有块都进上下文，单块超上限时走 `ai.CondenseBlock` 保留首尾并标注省略字数。
4. 返回被收敛的 `BlockKey`，`ChatService.Send` 据此打日志（`condensed_keys`）。
5. `ai.RenderPrompt` 渲染 `{{key}}`：命中即替换，缺失键替换为空串。

单块上限（rune）：

| 优先级 | 上限 | 适用块 |
| --- | --- | --- |
| ≥ `PriorityExplicitRef`(90) | 20000 | 当前章、引用章节/文档/摘要/角色/设定/会话 |
| 其余 | 6000 | 知识库召回、章节梗概、全量角色与设定 |

块文本格式（`buildContextBlocks`）：

| 块 | 格式 |
| --- | --- |
| `current_chapter` | `【当前章节】{名}\n{正文}` |
| `ref_chapter` | `【引用章节】{名}\n{正文}` |
| `ref_doc` | `【引用文档】{名}\n{全文}` |
| `ref_summary` | `【引用章节摘要】\n` + 摘要换行拼接 |
| `ref_character` | `【角色卡】` + 每条 `\n姓名：{名}｜性格：{性格}｜背景：{背景}` |
| `ref_lore` | `【设定条目】` + 每条 `\n设定名：{名}｜内容：{内容}` |
| `ref_session` | `【引用历史会话】` + 每条 `\n《{会话名}》\n{逐条归档}` |
| `rag` | `【知识库召回】` + 每条 `\n《{文档名}》{切片}` |
| `chapter_summary` | `【章节梗概】\n` + 梗概换行拼接 |
| `character` | `【角色卡】`（项目全量） |
| `lore` | `【设定条目】`（项目全量，`note` 排除） |

### 3.4 单项截断

| 常量 | 值 | 作用 |
| --- | --- | --- |
| `characterPersonalityRunes` | 120 | 角色性格 |
| `characterBackgroundRunes` | 200 | 角色背景 |
| `loreContentRunes` | 300 | 设定条目内容 |
| `sessionNameRunes` | 20 | 会话缺省名取首条消息字数 |

角色卡与设定条目这类「一行一条」的素材按上表收紧；章节正文、文档全文走首尾保留，不按此表截。

### 3.5 指令提示词渲染

命中指令提示词后（`ChatService.Send`）：

1. `chapter_content` = 引用章节正文按「当前章在前」拼接（不再截断）。
2. `context` = `ai.AssembleContext` 装配出的上下文文本（与独立系统级上下文消息同源）。
3. `user_input` = 去掉斜杠命令后的用户正文。
4. `prompt_code` = 命中的编码。
5. 渲染结果作为独立**系统级**消息（`CommandText`），位置在上下文消息之后、历史之前。

因此 `{{context}}` 的内容会同时出现在指令提示词与独立上下文消息中，属于有意设计：
指令提示词需要就近上下文，独立消息保证装配顺序可解释。

### 3.6 历史：全量注入，只收敛超长单条

历史**不按条数截取、不按 token 预算丢弃、不换摘要替代原文**。整段会话按原生角色逐条进消息序列，
模型看到的就是作者真实说过的话。

```mermaid
flowchart LR
  A["会话全部消息"] --> B["condenseHistory"]
  B --> C{"单条超过 4000 字?"}
  C -- 否 --> D["原样注入"]
  C -- 是 --> E["保留首 2400 + 省略标记 + 尾 1600"]
  D --> F["消息序列"]
  E --> F
```

`condenseHistory(messages)`：浅拷贝后逐条过 `ai.CondenseText`，条数与顺序不变，返回被收敛的条数供日志观测。

| 参数 | 值 | 含义 |
| --- | --- | --- |
| `condenseHeadRunes` | 2400 | 超长单条保留的开头字符数 |
| `condenseTailRunes` | 1600 | 超长单条保留的结尾字符数 |
| 阈值 | 4000 | 未超过 4000 字的正文一个字符都不改 |

为什么不做高压缩率：小说对话里「第几条说过什么」本身就是创作素材，压成摘要句会丢设定、丢语气、丢作者的取舍过程。
代价是上下文变长——这是明确接受的成本，不是遗漏。

上下文块同样不再按总预算丢弃：`ai.AssembleContext(blocks)` 只排序拼接，单块超上限时走
`ai.CondenseBlock` 保留首尾（显式引用 20000 字、背景素材 6000 字），所有块都会进上下文。

### 3.7 `@历史会话`：逐条归档

`POST /chat/send` 的 `ref_session_ulid_list` 指向历史会话时，`ai.ArchiveHistory` 把整段对话导出成
逐条编号的记录（`loadSessionArchive`）：

```
【历史会话归档】共 42 条，按时间顺序完整保留
1. 作者（2026-11-25T10:00:00+08:00）
这章开头太闷，重写
2. AI（2026-11-25T10:00:12+08:00）［工具：chapter_check］
……
```

| 字段 | 来源 | 说明 |
| --- | --- | --- |
| 序号 | 会话内顺序 | 从 1 起，回看时能定位「第几条」 |
| 角色 | `role_code` | `作者` / `AI` / `系统` |
| 时间 | `created_at` | RFC3339，可空 |
| 工具标签 | `tool_code_json` | 本轮模型实际调用的编码，可空 |
| 正文 | `content_text` | 超长时同样保留首尾并标注省略字数 |

## 4. 消息序列

`buildChatMessages(chatMessageInput)`：

| 顺序 | 角色 | 内容 |
| --- | --- | --- |
| 1 | `system` | `ai.ComposeSystemPrompt(systemPromptText, styleText)`：身份提示词 + 项目文风约束 |
| 2 | `system` | `【上下文】\n{contextText}`，为空时整条省略 |
| 3 | `system` | 指令提示词渲染结果，为空时整条省略 |
| 4… | 按历史顺序 | 逐条历史（全量，超长单条保留首尾），`chatRoleOf` 映射角色 |
| 末 | `user` | 用户输入（去命令后的正文） |

关键约束：**注入内容一律是系统级消息，用户输入一律是用户级消息**。这让"约束"与"要求"在模型侧与界面侧都可区分，
也避免了旧版把上下文塞进 `user` 消息、与真实用户输入混淆的问题。

`chatRoleOf`：`assistant` → `Assistant`；`system` → `System`；其余（含未知值）→ `User`。

### 4.1 工具往返

工具声明随每轮请求下发（`ai.ToolInfos`，function name 即工具编码），模型回 `tool_calls` 时服务端执行并回填：

```
streamChat 循环（≤ chatToolRoundLimit = 4）
  ├─ 无 tool_calls → 落库，done.tool_code_list = 本轮全部编码
  └─ 有 tool_calls → 执行 → assistant(tool_calls) + tool(result) 追加 → 再流一轮
```

消息序列因此是「系统消息 → 历史 → 用户 → (assistant tool_calls → tool result)* → assistant 正文」。
工具结果以原生 `tool` 消息回给模型，不再额外塞进上下文块——模型只该看到自己主动要来的事实。

防打转三道机制：单轮内「同工具同参数」去重；相邻两轮调用签名相同即摘掉工具声明；
轮数上限触发后若仍无正文则发 `error` 事件而不是静默返回空串。

## 5. 提示词

### 5.1 数据模型

提示词 = 作用域 + 种类 + 正文，落一张表：

| 列 | 说明 |
| --- | --- |
| `prompt_ulid`(`PRM_`) | 主键 |
| `prompt_code` | 编码（对话中以 `/code` 触发），同作用域内唯一 |
| `prompt_name` | 名称 |
| `prompt_kind_code` | `system`（身份）/ `command`（指令） |
| `scope_code` / `project_ulid` | `global` / `project`；同编码项目提示词覆盖全局 |
| `domain_code` | 流程域编码（内置资产按域分组展示） |
| `category_code` | `write` / `revise` / `outline` / `setting` / `review` / `other` |
| `description_text` | 一句话说明 |
| `prompt_text` | 提示词正文（可含 `{{占位符}}`） |
| `enabled_flag` / `builtin_flag` | 启停与内置标记 |
| `asset_hash_text` / `customized_flag` | 出厂指纹与作者改动标记 |
| `sort_order` | 排序号 |

旧版的 `skills` + `skill_references` + `prompt_profiles` 三张表已被 `prompts` 一张表取代：
技能与提示词本是同一种东西（一段可编辑的提示词），拆成两张表只会让"哪一段真的生效"变得难查。
技能参考资料（关键词命中的长资料）随技能一并删除，改为 `@` 引用提示词——作者显式挑选，链路更短、行为更可预期。

### 5.2 内置提示词（12 份身份 + 24 份指令）

由 `ai.BuiltinDomains()` 汇总 12 个流程域（`backend/internal/ai/domain_<域>.go`，一个域一个文件），
`PromptService.SeedDefaults` 以 `scope_code='global'`、`builtin_flag=true` 播种并做指纹升级。
完整矩阵见 [prompts.md](prompts.md)，此处只列编码与用途：

| code | 种类 | 名称 | 所属域 | 用途 |
| --- | --- | --- | --- | --- |
| `continue` | command | 续写正文 | draft | 承接当前章节往下写：衔接、具体、推进、章末留钩 |
| `conflict` | command | 冲突加码 | draft | 目标互斥 + 代价递增 + 不可逆选择的重写 |
| `outline` | command | 搭建大纲 | architect | 卖点 / 世界观 / 主线 / 人物 / 分卷 / 章纲六层，附结构风险 |
| `worldrule` | command | 世界观规则卡 | architect | 规则 / 边界 / 代价 / 例外四栏 + 一致性检查 |
| `polish` | command | 润色文字 | style | 只改「怎么说」：去翻译腔、去重复、把定性换成现场 |
| `dialogue` | command | 对白打磨 | style | 每句台词争夺信息、权力或关系；去寒暄留潜台词 |
| `deai` | command | 去 AI 味 | deai | 五层对抗诊断 + 改写全文 + 改动说明 |
| `humanness` | command | 活人感改写 | deai | 材料 / 位置 / 语气 / 不完美四步 |
| `review` | command | 章节质检 | review | 六道检查出 P0-P2 与可发结论 |
| `hitscan` | command | 量化门禁 | review | 八项硬指标给实测值与最小修复 |
| `foreshadow` | command | 伏笔台账 | continuity | 伏笔台账六列 + 悬空与过期预警 |
| `ledger` | command | 连续性台账 | continuity | 五本台账 + 冲突清单 + 三种收敛方案 |
| `summary` | command | 章节梗概 | finalize | 四段式事实条目，供上下文注入 |
| `finalcheck` | command | 终审对账 | finalize | 六维对账 + 不一致清单 + 待补项 |
| `golden3` | command | 开篇体检 | operation | 八项字位指标 + 三个最贵问题 |
| `platform` | command | 平台适配 | operation | 调性对齐 / 开篇改造 / 节奏排布 / 包装四组结论 |
| `pitch` | command | 立项评估 | pitch | 十二维打分 + 总分门槛 + 最弱两维补法 |
| `deconstruct` | command | 拆书 | pitch | 前三百字逐句 / 章节卡 / 可迁移配方 |
| `screenplay` | command | 剧本拆场 | adapt | 场次表 + 可读剧本 + 改编标注 |
| `voiceover` | command | 有声旁白 | adapt | 去视觉依赖、补听觉线索、控口播节奏 |
| `title` | command | 标题封面 | publish | 书名候选 + 点击力排序 + 避雷检查 |
| `blurb` | command | 上架文案 | publish | 四句简介 + 标签三组 + 合规申报 |
| `shortdraft` | command | 短篇起稿 | short | 形态判定 + 情绪曲线 + 一次成稿 |
| `ending` | command | 结尾余震 | short | 结尾问题判定 + 余震改写 |
| `pitch_analyst` | system | 选题策划 | pitch | 立项判断的身份与纪律 |
| `architect` | system | 故事架构师 | architect | 分层结构的身份与纪律 |
| `novelist` | system | 长篇连载作者 | draft | 默认身份：只交稿不解释 |
| `stylist` | system | 文字打磨师 | style | 只改说法不改事实 |
| `deai_editor` | system | 去 AI 味修订 | deai | 改变生成分布而非删词 |
| `editor` | system | 连载责任编辑 | review | 先判后改、分级给改法 |
| `continuity` | system | 设定守门人 | continuity | 只依据给定上下文作答 |
| `final_editor` | system | 定稿编辑 | finalize | 只归档对账不改稿 |
| `serial_editor` | system | 连载平台主编 | operation | 按平台口径判断留存 |
| `platform_operator` | system | 平台运营 | publish | 只对点击负责 |
| `short_story_author` | system | 短篇作者 | short | 一次完整呼吸 |
| `screenwriter` | system | 改编编剧 | adapt | 可见优先、标注改编 |

判据均可核对：卖点必须能填满「一个什么人，因为什么事，必须做到什么，否则失去什么」；世界观只认「规则 / 边界 / 代价」三栏；质检每条判定必须引原句，给不出原句的判定删掉；去 AI 味只看可机检指标（句长分布、套话密度、情绪直给次数），不做主观评价。

**指纹升级**：内置提示词属于产品资产，但允许作者改。每行存 `asset_hash_text`（出厂内容指纹）与 `customized_flag`。
启动时按「不存在→插入 / 已自改→不动 / 指纹一致→不动 / 其余→覆盖并写新指纹」处理，
所以代码里的提示词改版后上线即生效，作者改过的行不会被冲掉；`internal/ai/ai_chain_test.go` 断言域形状、编码唯一、占位符合法与指纹长度。

### 5.3 解析与覆盖

1. 请求体 `prompt_code` 非空 → 直接按编码解析。
2. 否则 `parseSlashToken` 解析正文首 token：trim 后以 `/` 开头，取到首个空白符为止；命中启用提示词则剥离命令，未命中（编码不存在、已停用、非首 token）则正文原样保留并按自由对话处理。
3. `resolveByCode`：先查 `(project, code)` 且启用，再回退 `(global, code)`；都未命中返回「提示词不存在或已停用」。
4. 列表页用 `isOverriddenGlobalPrompt` 给被项目版覆盖的全局提示词打 `overridden_flag`，避免"改了半天不生效"。

### 5.4 占位符

- 内置白名单（`ai.BuiltinPlaceholders`）：`user_input`、`chapter_content`、`context`、`prompt_code`，装配时自动注入。
- 保存时 `validatePromptPlaceholders` 用 `ai.TemplatePlaceholders` 校验：出现白名单外的占位符直接 `ERR_INVALID_PARAM`。
  未登记的占位符会被渲染成空串，等于静默丢上下文，因此必须在保存期拦下。
- 作者要注入长资料时，用 `@` 引用提示词，而不是把资料塞进正文模板。

### 5.5 生命周期

- `SeedDefaults` 幂等 + 指纹升级：按 `(global, "", prompt_code)` 查到行后，`customized_flag=true` 或指纹一致则跳过，否则用代码资产覆盖名称/说明/正文/域/分类/排序号。
- 已下线编码：`dropRetiredBuiltins` 用「指令编码集合 ∪ 身份编码集合」比对库中 `builtin_flag=true` 的全局提示词，把清单外的删除。
- 内置提示词不可删除（`Delete` 返回「内置提示词不允许删除，可停用」），可停用、可改正文；`prompt_code`、`scope_code`、`prompt_kind_code` 不可改。
- 保存时若提交内容与其出厂指纹不同，`customized_flag` 置 `true`，后续代码升级不再覆盖这一行；改回出厂内容会重新置回 `false`。
- 同作用域内编码唯一由服务层查重保证（先查后写）。

## 6. 无向量检索

### 6.1 设计前提

系统只接对话协议、无 embedding 服务，检索由词法信号、结构化权威数据与对话模型重排组成：

| 原则 | 落地 |
| --- | --- |
| 权威原文在 PostgreSQL 与 MinIO | 索引是可重建投影，可全量回灌 |
| CJK ngram 解决中文子串与专名 | Manticore 表 `ngram_len='1' ngram_chars='cjk'` |
| 模型只在候选有限时参与重排 | `maybeRerank` 全链路 fail-open |
| 检索过程可解释 | `domain.KnowledgeSearchTrace` 输出查询词、候选数、选中 ID、通道、重排说明 |

### 6.2 索引链路

```mermaid
flowchart LR
  U[上传文档] --> M[MinIO 原文<br/>object_key = doc_ulid]
  M --> D[(knowledge_documents<br/>pending)]
  D --> Q[RabbitMQ 索引队列]
  Q --> X[提取文本<br/>txt/md/docx/pdf]
  X --> S[滑窗切片]
  S --> H[sha256 内容哈希<br/>复用既有 chunk_ulid]
  H --> P[(knowledge_chunks)]
  H --> MC[Manticore nw_chunks]
  MC --> I[status=indexed]
```

| 环节 | 位置 | 要点 |
| --- | --- | --- |
| 上传 | `KnowledgeService.Upload` | 原文写 MinIO（对象键 = `doc_ulid`），文档行 `pending`，投递索引任务；落库失败时回滚对象 |
| 任务投递 | `enqueueIndexJob` | 先写 `jobs` 行（`queued`）再投递；投递失败置 `failed` |
| 文本提取 | `extractDocumentText` | txt/md 直读并剥 BOM；docx 读 `word/document.xml` 去标签还原实体；pdf 用纯 Go 解析 |
| 切片 | `chunkText` | 见 6.3 |
| 增量复用 | `indexDocument` | 与既有切片内容哈希相同且未占用 → 复用原 `chunk_ulid` |
| 落库 | `repo.WithTx` | 同一事务内先删该文档旧切片再插新切片 |
| 索引同步 | `syncManticoreChunks` | 先按 `doc_ulid` 删除该文档全部行，再逐条 INSERT |
| 状态 | `UpdateDocument` | 置 `indexed` 并写 `chunk_count`、`total_token_count` |

失败处理：`handleIndexJob` 最多重试 `maxIndexRetryCount = 3` 次（第 n 次重试前等待 n 秒），仍失败则文档置 `failed` 并记录原因（截断 500 rune），消息 ack，避免毒消息阻塞队列；文档已删除的滞留消息直接 ack。索引成功/失败都会给项目所有者写站内通知。

### 6.3 切片算法

`chunkText(text, chunkSize, overlap)`（`backend/internal/service/knowledge_worker.go`）：

- 默认 `chunk_size = 800`、`chunk_overlap = 120`（`app` 配置段）。
- 以 rune 为单位；窗口 `chunkSize`，步进 `chunkSize - overlap`。
- 边界优先：窗口内最后一个句末符号（`\n`、`。`、`！`、`？`、`；`、`!`、`?`、`;`）位置大于 `chunkSize/2` 时在此断开，否则硬切。
- 重叠过大（`end - overlap <= start`）时下一窗起点取 `end`；空片段丢弃。

### 6.4 表结构与迁移

`ensureManticoreSchema` 执行：

```sql
CREATE TABLE IF NOT EXISTS nw_chunks(
  chunk_ulid string, doc_ulid string, project_ulid string, chunk_text text
) charset_table='non_cjk,0..9,A..Z->a..z,a..z' ngram_len='1' ngram_chars='cjk'
```

表名取 `manticore.table_chunks`，为空回退 `nw_chunks`。启动时 `SHOW CREATE TABLE` 检查是否含 `ngram_len`，不含则 `DROP` → 重建 → `rebuildManticoreIndex` 从 `knowledge_chunks` 回灌。表初始化或回灌失败只记告警，检索降级，不阻断启动。

### 6.5 查询与双通道

`extractQueryTerms`：CJK 连续串按 2-gram 切分，拉丁串按字母数字段切分并转小写，去标点去重；结果为空时退化为整串。

| 通道 | MATCH 表达式 | 特点 |
| --- | --- | --- |
| `term` | `"词1" | "词2" | …` | 逐词 OR，召回面广 |
| `phrase` | `"完整查询"` | 整串匹配，精度高 |

两条 SQL 恒带 `project_ulid` 过滤；传 `doc_ulid_list` 时追加 `AND doc_ulid IN (…)`；`ORDER BY WEIGHT() DESC LIMIT topK`。

### 6.6 融合与重排

`SearchWithTrace`：

1. 模式归一：`hybrid` 走双通道，其余按 `keyword`。
2. `keyword` 只跑 `term` 通道；`hybrid` 跑 `term` + `phrase`，单通道失败降级到另一条，两条都失败返回 `ERR_STORAGE`。
3. RRF 融合（`fuseSearchResults`，常数 `rrfRankConstant = 60`）：

   $$score = \sum_{\text{channel}} \frac{1}{60 + rank}$$

   同切片跨通道命中累加得分并标记 `source_mode = hybrid`；融合取 `topK + 8` 条作为重排候选。
4. 可选重排（`maybeRerank`）：候选摘要为 `chunk_ulid` + 截断 160 rune 的文本；超时 8s；要求严格 JSON；成功后命中项标记 `hybrid`。
5. 最终截断 TopK。默认条数 `resolveRAGTopK`：请求 `top_k_count` > 应用配置 `rag_top_k`（默认 6）> 全局配置。

### 6.7 降级

| 级别 | 触发 | 行为 | `trace.rerank_note` |
| --- | --- | --- | --- |
| 1 | 无候选 / 未装配 AI 客户端 | 按检索得分截断 | 未配置重排模型，按检索得分排序 |
| 2 | 取模型规格失败 | 跳过重排 | 重排跳过：{err} |
| 3 | 调用失败 / JSON 非法 / ID 非法 | 沿用原顺序 | rerun unavailable / rerun fallback |

| 异常 | 行为 |
| --- | --- |
| Manticore 不可用 | 检索返回 `ERR_STORAGE`；对话链路捕获后降级为空 RAG，不阻断生成 |
| 文档未索引完成 | 切片未入 Manticore，不参与召回，状态在文档列表可见 |
| 无命中 | 上下文不含 `rag` 块，模型仍基于章节与设定回答 |
| 显式引用文档读取失败 | 直接报错（用户明确要求，不静默忽略） |

## 7. 流式协议

### 7.1 事件

`domain.ChatStreamEventDTO`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `seq` | int64 | 事件序号，从 1 单调递增 |
| `event_type` | string | `thinking` / `chunk` / `done` / `error` |
| `session_ulid` | string | 会话 ULID |
| `message_ulid` | string | 助手消息 ULID |
| `content_text` | string | 增量思考（thinking）、增量正文（chunk）或完整回复（done） |
| `reasoning_text` | string | 完整思考过程，仅 `done`（模型未返回推理内容时为空串） |
| `reference_list` | []KnowledgeChunkVO | RAG 召回依据，仅 `done` |
| `error_message` | string | 错误说明，仅 `error` |

### 7.2 序列

| 阶段 | 事件 |
| --- | --- |
| 思考中 | `thinking`（推理模型的 `reasoning_content` 每片一条，仅在模型返回时出现） |
| 生成中 | `chunk`（每片一条，空片跳过） |
| 生成启动失败 | `error`：`AI 生成启动失败：{err}` |
| 生成异常 | `error`：`AI 生成失败，请稍后重试` |
| 助手消息落库失败 | `error`：`回复保存失败，请重试` |
| 正常结束 | `done`（完整文本 + `reasoning_text` + `reference_list`） |

### 7.3 错误边界

由 `sseChatWriter.started` 判定：

| 情况 | 表现 |
| --- | --- |
| 流开始前失败（鉴权、提示词解析、会话校验、历史读取、上下文装配、供应商不可用） | 响应头未提交 → JSON 错误包络 |
| 流开始后失败 | HTTP 已是 200 → 只能下发 `error` 事件并记服务端日志 |

`ChatService.Send` 约定一致：流前失败返回 error，流后失败返回 nil（只发事件）。

### 7.4 心跳与代理

| 项 | 值 |
| --- | --- |
| 心跳间隔 | 15s（`chatPingInterval`），每个非空 chunk 后重置 |
| 心跳格式 | `: ping\n\n` |
| 响应头 | `Content-Type: text/event-stream; charset=utf-8`、`Cache-Control: no-cache`、`X-Accel-Buffering: no` |
| 事件格式 | `data: {json}\n\n` |
| nginx | `/api/` 段 `proxy_buffering off`、`proxy_read_timeout 3600s` |
| 服务写超时 | `write_timeout_sec: 0`（不限） |

客户端断开 → 请求 context 取消 → `streamChat` 返回 `finish(false)` → 不落助手消息（用户消息保留），仍补记用量。

### 7.5 token 估算

`estimateTokenCount(inputRunes, outputRunes) = int((input + output) × 0.75)`。用于用户消息与助手消息的 `token_used_count`；流结束时若供应商返回 `ResponseMeta.Usage` 则优先采用真实值。

## 8. 用量统计

`ai.UsageRecord`：`ProviderULID`、`ProviderName`、`ModelName`、`CallTypeCode`（当前仅 `chat`）、`SuccessFlag`、`PromptTokens`、`CompletionTokens`、`LatencyMs`。

`ModelUsageService`：

| 环节 | 说明 |
| --- | --- |
| 缓冲键 | `statDate \| providerULID \| modelName \| callTypeCode` |
| 累加 | `call_count`、`success_count`、`fail_count`、prompt/completion/total tokens、`latency_ms_sum` |
| 刷新 | `usageFlushInterval = 5s`；ctx 取消时最后一次落库 |
| 落库 | `usageRepo.UpsertIncrement` 按唯一维度冲突累加 |
| 失败 | 仅告警，缓冲丢弃，避免无限增长 |

`Overview` 聚合：按日趋势（缺失日期补零）、按供应商、按模型、区间总计、按调用数加权的平均延迟（整数运算）、成功率。查询区间缺省近 14 天，非法日期回退默认，起止倒置自动纠正；查询前先 `Flush`。

## 9. 安全边界

| 项 | 实现 |
| --- | --- |
| 密钥存储 | AES-256-GCM 封存（`pkg.SealSecret` / `OpenSecret`），材料取 JWT 密钥；`api_key_cipher` 标记 `json:"-"` 不外泄 |
| 密钥回显 | `maskSecret`：长度 ≤10 → `****`，否则前 4 + `***` + 后 3；解密失败显示「无法解密，请重新填写」；API 只返回 `api_key_masked` 与 `api_key_configured` |
| 明文生命周期 | 仅存在于 `specOf` 解封后的 `ProviderSpec.APIKey` |
| 写权限 | `/model/provider/*` 与 `/model/catalog/*` 需 `system:model:manage`；`capability` 对所有登录用户开放 |
| 项目隔离 | Manticore 查询恒带 `project_ulid`；检索入口先校验项目归属 |
| 引用归属 | 引用章节须属当前项目；引用文档须属当前项目或全局库（`project_ulid = "0"`）；引用消息须属当前会话，去重保序 |
| 会话归属 | 会话须属当前用户且项目匹配 |
| 私人笔记 | `lore_type_code = note` 不进上下文；全部为 note 时整块不生成 |
| 显式插入 | AI 产出只写 `chat_messages`，插入章节由前端显式触发 |
