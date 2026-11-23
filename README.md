# NovelWorkbench

面向长篇小说的 AI 创作工作台。项目、卷章、大纲、设定、人物与知识资料统一管理，AI 负责起草，作者负责修订与定稿。

系统以「项目 → 卷 → 章节」为写作主线，把结构化故事资产、流式 AI 对话与可维护的技能资产放进同一工作台，同时提供完整的 SaaS 管理能力：用户、角色权限、菜单与权限目录、审计、登录日志、模型生态、用量分析与运行状态。

设计优先级：**稳定性 > 体验 > 性能**。不接入向量模型，检索基于关键词、结构化权威数据与对话模型重排，结果可解释、索引可重建。数据自持、可自托管；模型密钥加密存储，仅回显掩码。

## 核心能力

| 模块 | 能力 |
| --- | --- |
| 写作工作台 | 卷/章目录树（搜索、折叠）、CodeMirror 6 正文编辑、实时字数与保存状态、AI 续写/润色/梗概、快照留档与回滚 |
| AI 开书 | 一句创意生成开书方案（书名、卖点、简介、文风、卷章梗概、角色卡、世界观规则卡），方案可逐字段修改后一次性落库（项目 + 卷章 + 角色 + 设定 + 大纲），单事务提交 |
| AI 助手 | SSE 流式对话；等待计时 + 推理模型思考过程实时展开、正文出现后自动收起；默认附带当前章节上下文；技能（作用域 + 模板 + 变量 + 参考资料）与系统提示词档案（身份 + 底线）分层下发；@引用文档/章节/摘要/提示词/历史消息五类来源；可配历史条数；按账号可用模型选择；召回依据随回复展示；一键插入章节或替换整章 |
| 技能资产 | 全局技能与项目技能（同编码项目覆盖全局）；变量声明与必填校验；参考资料按关键词命中注入（单技能上限 4000 字符）；9 个流程域、18 个内置技能可改可停用 |
| 提示词档案 | 系统提示词做成可编辑资产：全局档案与项目档案；内置 9 份（起草 / 架构 / 打磨 / 去 AI 味 / 质检 / 连续性 / 定稿 / 上架 / 立项，与技能一一配对）；对话期可临时切换；内置档案可改正文不可删 |
| 模型生态 | OpenAI 兼容与 Anthropic 双协议；供应商 → 模型目录 → 按角色分配三层结构；密钥 AES-256-GCM 加密、仅回显掩码；模型级连通性测试与系统默认切换；调用量按日聚合 |
| 知识库 | 文档库与检索双页签；txt/md/docx/pdf 上传；MinIO 存原文；RabbitMQ 异步索引；Manticore CJK ngram 召回；RRF 融合与可选对话模型重排；原文预览抽屉；检索追踪 |
| 设定管理 | 角色卡、AntV X6 关系图、世界观设定分类与搜索；创作笔记双栏 Markdown 编辑与预览 |
| 创作规划 | 多层级大纲树（卷/幕/故事块/章节）、场景卡任务单、成环检测 |
| 连续性看板 | 时序事实候选与复核、伏笔钩子状态流转、章节摘要与 stale 标记 |
| 导航与权限 | 后端驱动菜单（分组/页面/图标/排序在线维护）；菜单即权限登记（`perm_code` 单一事实源）+ 权限目录自动汇总；前端路由 `meta.permCode` 校验；接口逐请求校验 |
| SaaS 管理 | JWT 认证（不开放注册，账号由管理员分配）、用户/角色/菜单/权限目录/审计、登录日志、运行时配置、用量分析、运行状态、任务中心、站内通知 |
| 系统体验 | 品牌五色设计系统、镂空线条按钮五态（悬停放大 / 按下缩小 / 加载扫光）、Element Plus 菜单驱动的四种布局（侧边 / 顶部 / 混合 / 双列，顶栏即时切换）、明暗双主题、统一页面骨架、表格加载与空态、弹窗与抽屉过渡 |

## 技术栈

- 后端：Go 1.24 + Gin 1.11 + GORM(pgx) 1.31 + Viper 1.21 + Zap 1.28 + eino 0.9（OpenAI 兼容 / Anthropic 双协议）+ Redis + RabbitMQ + MinIO + Manticore，`CGO_ENABLED=0` 静态编译
- 前端：Vue 3.5 + TypeScript + Vite + Element Plus 2.14 + Pinia + Vue Router + Axios + UnoCSS + ECharts + AntV X6 + CodeMirror 6 + marked + docx-preview + vue-pdf-embed + Vue I18n
- 测试：Go `testing`（11 条链路级用例）+ Vitest（流式链路），详见 [doc/test.md](doc/test.md)
- 部署：Docker Compose / Podman Compose，GitHub Actions CI/CD，静态二进制 + nginx SPA

## 快速开始

```bash
# 1. 准备环境变量
cp .env.example .env
# 必填：JWT 密钥、数据库/Redis/RabbitMQ/MinIO/Manticore 凭据与地址、引导管理员密码
# 可选：NW_AI_* 出厂供应商（留空则登录后在「模型供应商」页添加）

# 2a. 仅应用容器（中间件指向已有实例）
docker compose up -d --build

# 2b. 本地全栈（额外拉起中间件容器）
docker compose --profile infra up -d --build

# 3. 访问
#   http://<host>:14399     前端与 /api 反向代理（唯一对外入口）
#   容器内后端              http://backend:8080/health
```

启动时：PostgreSQL 失败即退出；Redis / RabbitMQ / MinIO / Manticore 失败只告警并降级；自动建表 33 张；播种 9 份提示词档案与 18 个内置技能（按内容指纹升级）、默认配置、内置角色与菜单；用户表为空时用 `NW_APP_ADMIN_PASSWORD` 创建管理员 `admin`（默认 `Admin@123456`，生产必改）。若供应商表为空且配置了 `NW_AI_BASE_URL` 与 `NW_AI_CHAT_MODEL`，额外播种一条默认供应商与默认模型并授权给内置角色。

本地开发：

```bash
# 后端（默认读 configs/config.yaml，可用 NW_* 覆盖）
cd backend && go run ./cmd/server

# 前端（/api 代理到 http://127.0.0.1:8080，可用 NW_DEV_PROXY_TARGET 覆盖）
cd frontend && corepack enable && pnpm install && pnpm dev
```

## 仓库结构

```
backend/            Go 后端：api → service → repo → domain 分层
  cmd/server        装配启动（迁移、播种、路由、优雅关闭）
  internal/         api / service / repo / domain / ai / infra / pkg
frontend/           Vue 3 前端（pnpm workspace 单仓）：品牌令牌 + 按钮五态 + 后端驱动导航 + 业务页面
  packages/         内部包：@novelworkbench/components / hooks / request / styles
deploy/podman/      Podman 发布资产：构建上下文、运行镜像、编排与发布脚本
doc/                架构、模块、AI、提示词资产、API、数据库、前端、安全、部署、测试文档
docker-compose.yml  应用编排与本地中间件 profile（对外 14399）
.env.example        环境变量模板（不含真实凭据）
.github/workflows   CI 与镜像发布
```

## 文档

从 [doc/architecture.md](doc/architecture.md) 的「文档地图」开始，按顺序读即可梳理整个项目。

| 文档 | 内容 |
| --- | --- |
| [doc/architecture.md](doc/architecture.md) | 文档地图、系统定位、技术选型、分层约束、请求生命周期、启动装配、核心链路、设计取舍 |
| [doc/modules.md](doc/modules.md) | 每个业务模块的职责、数据结构、实现路径、边界与权衡 |
| [doc/ai.md](doc/ai.md) | 模型网关、上下文装配、系统提示词档案、技能体系、无向量检索、流式协议（含思考事件）、用量统计 |
| [doc/skills.md](doc/skills.md) | 提示词资产矩阵：9 个流程域 × (1 份档案 + 2 个技能)、提示词构造规则、指纹升级与维护方式 |
| [doc/database.md](doc/database.md) | 存储分工、表结构、枚举、非关系存储、事务与一致性、迁移与备份 |
| [doc/api.md](doc/api.md) | 接口契约、统一包络、权限码、分页与前端消费规则 |
| [doc/frontend.md](doc/frontend.md) | 工程结构、设计系统、状态与数据流、路由与权限、页面实现、国际化 |
| [doc/security.md](doc/security.md) | 认证、授权、密钥、审计、上传与部署边界 |
| [doc/deploy.md](doc/deploy.md) | 容器、配置与环境变量、Compose/Podman、CI/CD、运维 |
| [doc/test.md](doc/test.md) | 测试策略、当前用例清单、集成验证方式 |

## 约定

- 依赖方向单向：`api → service → repo → domain`；`ai` / `infra` / `pkg` 为叶子能力层。
- ID 为带业务前缀的 ULID（`USR_` / `PRJ_` / `SKL_` / `MNU_` …）；业务响应码为字符串（`SUCCESS` / `ERR_*`），HTTP 状态码仍为 int。
- 业务路由只开放 GET / POST；路径参数放在末尾（`/menu/detail/{MenuULID}`）。
- 字段带明确后缀（`project_name`、`genre_code`、`created_at`）并有注释；时间出参统一为 RFC3339 字符串。
- 源文件控制在 500 行内，导出符号有注释，大页面拆分子组件。
- 不引入 cgo；不接入向量模型；不开放注册。
