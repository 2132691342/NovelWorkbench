# NovelWorkbench

NovelWorkbench — 给长篇小说的一张书桌。项目、卷章、大纲、设定、人物与知识资料一处管理；
AI 边写边对照事实与伏笔，作者始终握着笔：起草交给模型，修订、定调与复核留给作者。

系统以"项目 → 卷 → 章节"为写作主线，把结构化故事资产、流式 AI 对话与技能统一在工作台中，
同时提供 SaaS 管理能力：用户、角色权限、审计、模型供应商、用量分析与运行状态。

设计优先级：稳定性 > 体验 > 性能。系统不接入向量模型，检索基于关键词、结构化权威数据
与 chat 模型重排，结果可解释、索引可重建。数据自持、可自托管；密钥加密存储，仅回显掩码。

## 核心能力

| 模块 | 已实现能力 |
| --- | --- |
| 写作工作台 | 卷/章目录树、CodeMirror 正文编辑、实时字数、快照留档与回滚、XLSX 大纲导出 |
| AI 助手 | SSE 流式对话；9 个内置斜杠技能与自定义技能；@引用文档/章节/历史消息；最近 N 条历史（可配置）；按供应商选择模型；召回依据随回复展示 |
| 模型生态 | OpenAI 兼容与 Anthropic 双协议；多供应商管理；密钥 AES-256-GCM 加密、仅回显掩码；连通性测试与默认切换；调用量按日聚合 |
| 知识库 | txt/md/docx/pdf 上传；MinIO 存原文；RabbitMQ 异步索引；Manticore CJK ngram 关键词召回；词法混合与 chat 重排；原文预览；检索追踪 |
| 设定管理 | 角色卡、AntV X6 角色关系图、世界观设定分类与搜索；WangEditor 富文本与 MavonEditor Markdown 双模式创作笔记 |
| 创作规划 | 多层级大纲树（卷/幕/故事块/章节）、场景卡任务单、成环检测 |
| 连续性看板 | 时序事实候选与复核、伏笔钩子状态流转、章节摘要与 stale 标记 |
| 项目导航 | 侧栏项目切换器；写作台/创作规划/连续性/角色/设定/知识库/笔记七个项目内入口；URL 与 Pinia 项目上下文同步 |
| 数据总览 | ECharts 人工/AI 字数趋势、项目统计 |
| SaaS 管理 | JWT 认证、用户/角色权限、审计日志、运行时配置、用量分析、运行状态 |
| 系统体验 | 品牌五色设计系统；按钮五态；路由过渡、卡片微抬、加载/空/错误态；中英双语 |
| 扩展能力 | BPMN-JS 创作流程设计器（浏览器草稿、导入/导出 BPMN）；Qiankun 微前端登记与按需挂载 |
| 质量保障 | 后端 7 条链路级单元测试 + 前端 1 条 SSE 解析用例；`go vet`/`go test -short`/类型检查/镜像构建全量进 CI |

## 快速开始

```bash
# 1. 复制环境变量模板并填入真实配置
cp .env.example .env
# 必须填写：JWT 密钥、数据库/Redis/RabbitMQ/MinIO/Manticore 凭据与地址、模型网关信息

# 2a. 仅应用容器（中间件指向已有实例）
docker compose up -d --build

# 2b. 本地全栈（额外拉起中间件容器）
docker compose --profile infra up -d --build

# 3. 访问
# 前端 http://localhost
# 后端 http://localhost:8080/health
```

首次启动时，如果模型供应商表为空，系统用 `NW_AI_*` 播种一条默认供应商；
之后再在"模型供应商"页面维护多供应商。用户表为空时用 `NW_APP_ADMIN_PASSWORD`
创建初始管理员 `admin`。

本地开发：

```bash
cd backend && go run ./cmd/server
cd frontend && npm install && npm run dev
```

## 技术栈

- 后端：Go 1.24 + Gin 1.11 + GORM(pgx) 1.31 + Viper 1.21 + Zap 1.28
  + eino 0.9（双协议对话）+ Redis + RabbitMQ + MinIO + Manticore，`CGO_ENABLED=0`
- 前端：Vue 3.4 + TypeScript 5.4 + Vite 5.2 + Element Plus 2.9 + Pinia 2.1
  + Vue Router 4.3 + Axios 1.6 + UnoCSS 0.58 + ECharts 5.5 + AntV X6 2.18
  + BPMN-JS 16.4 + WangEditor 5.1 + MavonEditor 3.0 + CodeMirror 6
  + XLSX + docx-preview + vue-pdf-embed + Qiankun 2.10 + Vue I18n 9.10
- 测试：Go 内置 testing（`go test -short`）+ Vitest 2.1（`npm run test`）
- 部署：Docker Compose / Podman Compose；GitHub Actions CI/CD；静态二进制 + nginx SPA

## 仓库结构

```
backend/            Go 后端：api → service → repo → domain 分层
  cmd/server        装配启动
  internal/         api / service / repo / domain / ai / infra / pkg
frontend/           Vue3 前端：设计系统 + 业务页面
deploy/podman/      Podman 发布资产：构建上下文、运行镜像、编排与发布脚本
doc/                设计、数据库、API、AI、前端、功能、部署、测试文档
docker-compose.yml  应用编排与本地中间件 profile
.env.example        环境变量模板（不含真实凭据）
.github/workflows   CI 与镜像发布
```

## 文档

| 文档 | 内容 |
| --- | --- |
| [doc/architecture.md](doc/architecture.md) | 系统定位、技术选型、分层约束、请求生命周期、核心链路、设计取舍 |
| [doc/features.md](doc/features.md) | 每个模块的"用户能力 → 实现 → 验收点" |
| [doc/database.md](doc/database.md) | 存储分工、表结构、索引与一致性策略 |
| [doc/api.md](doc/api.md) | 接口契约、统一包络、字段与错误码 |
| [doc/ai.md](doc/ai.md) | 模型网关、提示词装配、斜杠技能、无向量检索、流式协议 |
| [doc/frontend.md](doc/frontend.md) | 前端设计系统、状态、路由、页面实现与测试 |
| [doc/deploy.md](doc/deploy.md) | 容器、环境变量、Compose/Podman、CI/CD |
| [doc/test.md](doc/test.md) | 测试策略、当前保留的用例、集成验证清单 |

## 约定

- 依赖方向单向：`api → service → repo → domain`；`ai`/`infra`/`pkg` 为叶子能力层。
- ID 为带业务前缀的 ULID（如 `PRJ_01H...`）；业务响应码为字符串（`SUCCESS` / `ERR_*`）；
  业务路由仅 GET/POST。
- 字段命名带明确后缀（`project_name`、`genre_code`、`created_at_ms`）。
- 文件保持可维护：Go 源文件 ≤500 行，导出符号有注释；前端大页面拆 CSS/子组件。
- 不引入 cgo；不接入向量模型；稳定性与体验优先于性能。
