# 功能实现说明

## 1. 阅读方式

本文按“用户能做什么 → 系统怎么实现 → 验收点”描述 NovelWorkbench 的每一个功能域。与架构、数据库、API、AI、前端文档配合阅读，可从产品功能追溯到文件与数据表。

## 2. 系统管理

### 2.1 认证与会话

用户能力：注册、登录、查看个人信息、退出登录。

实现：

- `api/auth_api.go` 绑定 `UserRegisterREQ` / `UserLoginREQ`，输出 `UserVO` / `UserLoginDTO`。
- `service/auth_service.go` 校验密码（bcrypt）、签发 JWT、查询角色码。
- `pkg/jwt.go` 负责令牌生成与解析；`pkg/middleware.go` 注入当前用户。
- 登出把 token 摘要写入 Redis 黑名单；每次请求检查用户状态，禁用账号旧令牌立即失效。

验收点：禁用账号的旧 token 不能再访问；登出后 token 立即失效；密码哈希不出现在响应中。

### 2.2 用户与角色

用户能力：管理端创建/编辑/删除用户、重置密码、分配角色；创建/编辑/删除角色与权限码。

实现：

- `domain/user.go`、`domain/role.go` 定义 DO/REQ/VO。
- `service/user_admin_service.go`、`service/role_service.go` 处理分页、唯一性校验与角色覆盖。
- `api/user_admin_api.go`、`api/role_api.go` 只做绑定与响应。
- 路由按 `system:user:manage` / `system:role:manage` 收口。

验收点：内置角色不可删；删除用户后关联角色清理；权限码变更立即生效。

### 2.3 审计与配置

用户能力：查看写操作审计日志；查看/修改白名单内的运行时配置。

实现：审计中间件记录写请求的操作者、路径、资源、业务码、耗时；配置以 key-value 存 `app_settings`，修改后立即影响相应 service。

验收点：写操作有审计记录；配置修改后对应行为变化；非法配置键被拒绝。

## 3. 创作主线

### 3.1 项目管理

用户能力：新建、编辑、删除项目；切换当前项目；查看项目详情与统计。

实现：

- `domain/project.go` 定义 `ProjectDO/VolumeDO` 及 REQ/VO。
- `service/project_service.go` 处理归属校验、状态更新、目录树组装。
- `repo/project_repo.go` 执行 GORM 查询。
- 前端 `stores/project.ts` 保存当前项目；侧栏切换器列出全部项目。

验收点：所有子资源查询都带项目条件；删除项目不波及其他项目数据；URL 中的项目 ULID 与 store 同步。

### 3.2 卷与章节

用户能力：创建/编辑/删除卷与章节；调整顺序；编辑正文；实时字数；创建/回滚快照。

实现：

- `domain/chapter.go` 定义章节与快照结构。
- `service/chapter_service.go` 计算字数、更新统计、处理快照回滚。
- `repo/chapter_repo.go` 与 `repo/volume_repo.go` 负责持久化。
- 前端 `views/workspace` 使用 CodeMirror 编辑，自动保存，字数本地即时统计后以服务端结果为准。

验收点：章节正文整体覆盖；快照能完整回滚；字数统计与正文一致；删除卷时子章节处理方式明确。

### 3.3 大纲导出

用户能力：从写作台导出 XLSX 大纲。

实现：前端按 `ProjectTreeVO` 组装卷/章行，XLSX 库生成工作簿，字段为卷名、章名、顺序、字数、状态。

验收点：导出顺序与目录树一致；中文不乱码；空项目导出表头完整。

## 4. 设定生态

### 4.1 角色卡

用户能力：创建/编辑/删除角色，填写姓名、定位、性格、目标、秘密、声音特征、当前状态等字段。

实现：`domain/character.go` + `service/character_service.go` + `repo/character_repo.go`。字段全部带明确后缀，列表按项目过滤。

验收点：角色字段完整保存；删除角色时关系边同步清理；角色列表按项目隔离。

### 4.2 角色关系图

用户能力：在 X6 画布上查看/编辑角色关系，拖拽节点，保存关系边。

实现：`GET /character/graph` 返回节点与边；`POST /character/relation/save|delete` 写回。前端 `CharacterGraphTab.vue` 负责渲染与交互。

验收点：节点与角色一一对应；关系边保存后刷新仍在；删除角色后无悬空边。

### 4.3 世界观设定

用户能力：按分类维护世界背景、规则、地点、物品、事件；关键词搜索；编辑抽屉保存。

实现：`domain/lore.go` + `service/lore_service.go`；分类是 `lore_type_code`，前端分类栏显示计数。

验收点：分类与搜索组合正确；设定可编辑；私人笔记不进入 AI 上下文。

### 4.4 创作笔记

用户能力：用 WangEditor 编辑富文本笔记。

实现：笔记按项目存储 HTML，前端局部编辑并自动保存；不参与检索与 AI 上下文装配。

验收点：富文本格式保留；不污染 AI 证据链。

## 4.5 创作规划

用户能力：维护卷/幕/故事块/章节的多层级大纲树；为任一节点维护多个场景卡，记录出场角色、地点、冲突、节奏、入场状态、目标、关键选择、结果与出场状态。

实现：`domain/outline.go` 定义大纲节点与场景卡的 DO/REQ/VO；`service/outline_service.go` 校验项目归属并禁止删除仍有子节点的节点；`repo/outline_repo.go` 持久化；前端 `/outline/:project_ulid` 提供左树右场景卡编辑界面。

验收点：节点层级与排序可完整保存；有子节点的节点不可直接删除；场景卡与节点绑定且按项目隔离。

## 4.6 连续性看板

用户能力：登记与审核时序事实（候选/确认/取代/失效）；跟踪伏笔状态（计划/埋设/呼应/回收/放弃）；保存章节摘要与内容哈希。

实现：`domain/continuity.go` 定义事实、伏笔、摘要结构；`service/continuity_service.go` 校验项目归属与状态编码；前端 `/continuity/:project_ulid` 用标签页分别展示事实、伏笔与摘要。

验收点：候选事实不会自动成为正式事实；伏笔状态流转可追溯；章节摘要可标记 stale 且不自动覆盖正文。

## 5. 知识库

### 5.1 上传与索引

用户能力：上传 txt/md/docx/pdf；查看索引状态；失败后重试。

实现：

- `api/knowledge_api.go` 接收 multipart 上传。
- `service/knowledge_service.go` 写 MinIO + 落库 + 投递 RabbitMQ。
- `service/knowledge_worker.go` 消费任务，提取文本、切片、计算内容哈希、写 Manticore。
- `repo/knowledge_repo.go` 只负责数据库读写；Manticore/MinIO 客户端封装在 infra/repo 边界内。

验收点：上传成功即见 `pending`；索引完成后可检索；失败显示原因；reindex 可恢复。

### 5.2 检索

用户能力：输入查询，得到命中切片与高亮；查看检索追踪；把命中作为 AI 引用依据。

实现：

- `POST /knowledge/search` 调 `KnowledgeService.Search`。
- 查询词由用户输入、当前章节、角色名与显式引用组合。
- Manticore CJK ngram 召回；多通道 RRF 融合；可选 chat 模型按 ID 重排。
- 返回 `KnowledgeSearchVO` 含命中列表与 trace。

验收点：无 embedding 配置可用；中文专名可召回；跨项目不串数据；重排失败自动降级。

### 5.3 文档预览

用户能力：在线查看原文；txt/md 直接渲染，docx/pdf 由前端组件渲染。

实现：`GET /knowledge/content` 返回文件流；前端按扩展名选择预览组件。

验收点：预览内容与上传文件一致；无权限项目无法读取。

## 6. AI 工作台

### 6.1 会话管理

用户能力：创建、切换、删除 AI 会话；查看历史消息。

实现：`chat_sessions` / `chat_messages` 两张表，`ChatAPI` + `ChatService` 管理会话生命周期。删除会话级联删除消息。

验收点：会话按项目与用户隔离；历史消息顺序正确；删除后不可恢复。

### 6.2 发送消息

用户能力：输入文本或 `/技能`，携带当前章节与 @引用，选择历史条数与模型，发送消息并看到流式回复。

实现：`POST /chat/send` 返回 SSE；service 先解析技能、装配上下文、检索资料，再调用模型；流结束落库消息与用量。

验收点：流式逐字展示；停止可取消；done 事件含完整回复与召回依据；失败有明确错误码。

### 6.3 斜杠技能

用户能力：从补全列表选择技能；管理内置/自定义技能模板。

实现：`internal/ai/skills.go` 播种九个内置技能；`slash_commands` 表存模板；`/slash/parse` 解析正文中的命令；提示词模板只影响上下文，不改变链路。

验收点：内置技能不可删；自定义技能立即出现在补全列表；技能命中后模型回复符合模板意图。

### 6.4 上下文与引用

用户能力：@引用文档与章节；设置历史条数；查看模型实际依据。

实现：请求带 `ref_doc_ulid_list`、`ref_chapter_ulid_list`；`ai.AssembleContext` 按优先级与字符预算裁剪；`done` 事件携带召回列表。

验收点：显式引用优先于普通召回；超预算有裁剪记录；私人笔记不注入。

### 6.5 插入正文

用户能力：把 AI 回复插入当前章节，必要时先创建快照。

实现：前端把 `done` 事件的完整文本插入 CodeMirror；章节更新走同一 `POST /chapter/update`，服务端重新计算字数。

验收点：未点击插入时正文不变；插入后可回滚到快照。

## 7. 模型生态

### 7.1 供应商管理

用户能力：添加 OpenAI 兼容或 Anthropic 供应商；设置地址、密钥、模型；测试连通；设为默认；启停。

实现：`model_providers` 表 + AES-256-GCM 加密；`ai.Client.Probe` 发起最小对话请求；默认供应商全局唯一。

验收点：密钥只回显掩码；测试成功才允许切默认；禁用供应商不被对话链路选中。

### 7.2 模型选择

用户能力：在 AI 面板选择本次调用的供应商/模型。

实现：`GET /model/chat_options` 返回可用选项；`ChatSendREQ.provider_ulid` 指定覆盖；空值走默认供应商。

验收点：切换后立即生效；禁用项不可选；无可用供应商时页面给出配置指引。

## 8. 可视化与运维

### 8.1 数据总览

用户能力：查看项目数、字数、今日新增、AI 生成字数与近 30 天趋势。

实现：`repo/stats_repo.go` 从 `projects` 与 `word_count_stats` 聚合，ECharts 渲染双系列。

验收点：趋势与章节字数变化一致；空数据展示空态而非报错。

### 8.2 用量分析

用户能力：按 7/14/30 天查看调用次数、token、成功率、延迟与分布。

实现：`ai.UsageRecorder` 缓冲调用，`repo/model_usage_repo.go` 按日 upsert；管理端 `GET /model/usage/overview` 聚合。

验收点：流式调用结束后用量被补记；失败调用计入失败数；分布表与分析卡一致。

### 8.3 运行状态

用户能力：查看进程信息、依赖健康、模型能力结论。

实现：`service/system_status_service.go` 并行探测 PG/Redis/RabbitMQ/MinIO/Manticore 与模型；前端显示耗时与结论，失败页有重试入口。

验收点：某个依赖不可用时显示 degraded/down 而非空白；探测不影响主业务请求。

## 9. 跨模块约束

| 约束 | 实现方式 |
| --- | --- |
| 项目隔离 | 所有业务查询强制带 `project_ulid` |
| 权限收口 | 管理路由挂权限中间件，前端只隐藏入口不代替鉴权 |
| 响应一致 | 所有 API 走 `pkg.Ok` / `pkg.Fail` |
| 错误可追踪 | 响应 `trace_id` 与日志 `request_id` 同源 |
| 交互有反馈 | 写操作 toast、加载态、空态、错误重试 |
| 文件可维护 | Go 文件 ≤500 行；前端页面超限拆 CSS/子组件 |
| 无向量模型 | 检索只用词法+结构化+chat 重排；配置与 UI 均无向量项 |
