# 测试说明

## 1. 测试策略

只保留"失败即代表跨模块/跨协议行为损坏"的测试：复杂链路、复杂场景、长链路。简单断言、实现细节回显、结构体转圈类测试一律不保留，避免每次改动都背一堆无信息量的用例。

| 层 | 测什么 | 不测什么 |
| --- | --- | --- |
| 单元（`go test -short`） | 上下文装配预算裁剪、斜杠解析、切片滑窗、RRF 融合、模板渲染 | CRUD 字段透传、getter/setter、JSON 序列化回显 |
| 单元（前端） | SSE 解析器（跨 chunk 断行/粘包）、字数口径与后端对齐 | 组件快照、样式存在性 |
| E2E（集成环境） | 认证→项目→章节→快照、知识库索引→检索、SSE 对话全链路 | UI 控件逐一点击 |

测试原则（与后端规范一致）：表驱动 + `t.Run` 一个用例讲一条链路；不依赖外部服务（`-short` 全绿）；禁 `time.Sleep`；同一行为只断言一处。

## 2. 保留的测试清单

| 测试文件 | 覆盖链路 |
| --- | --- |
| `backend/internal/ai/prompt_test.go` | 上下文块优先级保留、超预算丢弃（丢弃块可追溯）、同优先级确定性、模板占位符渲染（缺失→空串） |
| `backend/internal/service/slash_service_test.go` | 斜杠解析全链路：命中/未命中/参数剥离/禁用命令/非首 token |
| `backend/internal/service/knowledge_service_test.go` | 切片滑窗（短文本/重叠/边界优先/非法重叠收敛）；RRF 融合（双通道/单通道/TopK 截断/来源标注） |
| `backend/internal/service/chat_service_test.go` | 对话上下文装配：技能命中/未命中渲染、@引用注入、历史 N 条截断 + 显式引用必注入、预算丢块标注、设定注入（含 note 排除） |
| `backend/internal/service/knowledge_worker_test.go` | Manticore 中文 ngram 表结构迁移：旧表 `DROP → CREATE → 回灌` 只触发一次，新表启动不重建 |
| `backend/internal/pkg/middleware_test.go` | 鉴权用户状态：活跃放行、禁用/删除旧令牌 401、状态查询失败 500 |
| `backend/internal/pkg/secret_test.go` | 供应商密钥封存链路：同材料可解、换材料/坏密文必须失败、每次 nonce 随机、空材料拒绝 |
| `backend/internal/service/model_usage_service_test.go` | 用量区间解析（缺省近 14 天/非法回退/起止倒置纠正）与趋势补零（缺失日零值、已有数据不改写） |

运行方式：

```bash
cd backend && go test -short -count=1 -shuffle=on ./...
```

## 3. E2E 验证方法

E2E 在集成环境执行：后端直连中间件（Postgres/Redis/RabbitMQ/MinIO/Manticore），AI 网关用本地 mock（OpenAI 兼容：`/v1/chat/completions` 流式 + `/v1/embeddings` 确定性伪向量），验证的是本系统链路而非模型质量。

已验证链路（接口级 + 浏览器级）：

| 链路 | 验证点 |
| --- | --- |
| 认证 | 注册/登录/鉴权/登出拉黑 |
| 项目→卷→章 | CRUD、目录树、字数账目（含清空正文与删卷的字数回补）、快照留档/回滚/列表、删除项目/卷后子表零残留 |
| 角色/设定 | 角色卡、关系边、关系图数据；设定增删查（含 note 类型） |
| 知识库 | 上传→队列→切片→向量化→落库/建索引→混合检索命中（source_mode 标注）；向量化失败时重试→failed 与检索降级 keyword |
| 对话 | SSE 流式（chunk seq 递增→done 带召回依据）、技能/引用/历史注入、消息落库、AI 字数落账、指定供应商生效 |
| 模型生态 | 供应商 CRUD 与密钥掩码、连通性测试（成功/失败均回写）、默认切换唯一性、用量聚合（调用/失败/token/延迟）随真实调用增长 |
| 运行状态 | 依赖健康探测（正常/未配置降级/不可用）、AI 能力结论与供应商配置一致 |
| 前端 | 登录守卫、侧边栏项目切换与项目内导航（写作台/角色/设定/知识库/笔记直达）、仪表盘统计、工作台（目录树/编辑器自动保存/SSE 面板/模型选择/插入章节）、X6 关系图、世界观设定 CRUD、知识库上传/检索/预览、WangEditor 笔记、模型供应商页、用量分析与运行状态页，浏览器实测 |

集成阶段发现并已修复的典型问题（说明这些链路测试有效）：GORM 列名推导 `XxxULID→xxx_ul_id` 与手写 SQL 不一致（显式 column 标签 + NamingStrategy 双保险）、向量列扫描类型不匹配、章节空串无法清空（指针语义）、关系图挂载竞态导致不适配。

## 4. 测试环境要点

- 中间件连通：开发机经 SSH 隧道访问云中间件（端口 15432/16379/19308/19000/15672 → 服务器 5432/6379/9308/9000/5672）。
- AI 网关：真实模型调用需配置 `NW_AI_API_KEY`（OpenAI 兼容网关）；未配置时对话/向量化返回 `ERR_AI_PROVIDER`，其余功能不受影响。
- CI 中仅跑 `go test -short`（无外部依赖）；镜像构建验证见 `.github/workflows/ci.yml`。
