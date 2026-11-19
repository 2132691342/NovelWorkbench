# NovelWorkbench

NovelWorkbench — 给长篇小说的一张书桌。项目、卷章、大纲、设定、人物与知识资料一处管理；
AI 边写边对照事实与伏笔，作者始终握着笔：起草交给模型，修订、定调与复核留给作者。

系统以「项目 → 卷 → 章节」为写作主线，把结构化故事资产、流式 AI 对话与可维护的技能资产
统一在工作台中，同时提供完整的 SaaS 管理能力：用户、角色权限、菜单与权限目录、审计、
模型供应商、用量分析与运行状态。

设计优先级：稳定性 > 体验 > 性能。系统不接入向量模型，检索基于关键词、结构化权威数据
与 chat 模型重排，结果可解释、索引可重建。数据自持、可自托管；密钥加密存储，仅回显掩码。

## 核心能力

| 模块 | 已实现能力 |
| --- | --- |
| 写作工作台 | 卷/章目录树、CodeMirror 正文编辑、实时字数、快照留档与回滚、XLSX 大纲导出 |
| AI 助手 | SSE 流式对话；技能（作用域 + 模板 + 变量 + 参考资料）；@引用文档/章节/历史消息；最近 N 条历史（可配置）；按供应商选择模型；召回依据随回复展示 |
| 技能资产 | 全局技能 / 项目技能（同编码项目覆盖全局）；变量声明与必填校验；参考资料按关键词命中注入（单技能上限 4000 字符）；9 个内置技能可改可停用 |
| 模型生态 | OpenAI 兼容与 Anthropic 双协议；多供应商管理；密钥 AES-256-GCM 加密、仅回显掩码；连通性测试与默认切换；调用量按日聚合 |
| 知识库 | txt/md/docx/pdf 上传；MinIO 存原文；RabbitMQ 异步索引；Manticore CJK ngram 关键词召回；词法混合与 chat 重排；原文预览；检索追踪 |
| 设定管理 | 角色卡、AntV X6 角色关系图、世界观设定分类与搜索；WangEditor 富文本与 MavonEditor Markdown 双模式创作笔记 |
| 创作规划 | 多层级大纲树（卷/幕/故事块/章节）、场景卡任务单、成环检测 |
| 连续性看板 | 时序事实候选与复核、伏笔钩子状态流转、章节摘要与 stale 标记 |
| 导航系统 | **后端驱动菜单**：侧边栏由 `GET /menu/nav` 下发（分组/页面/图标/排序可在线维护）；项目切换器；顶栏快速跳转；面包屑；无权限菜单不下发 |
| 权限体系 | 菜单即权限登记（`perm_code` 单一事实源）+ 权限目录自动汇总；角色按目录勾选；前端路由 `meta.perm_code` 校验；接口侧逐请求校验 |
| SaaS 管理 | JWT 认证（不开放注册，账号由管理员分配）、用户/角色/菜单/权限目录/审计日志、运行时配置、用量分析、运行状态 |
| 系统体验 | 品牌五色设计系统；按钮五态（默认/悬停放大/按下缩小/禁用/加载）；路由过渡、卡片微抬、加载/空/错误态；中英双语 |
| 质量保障 | 后端 9 条链路级单元测试（上下文装配、访问控制、密钥封存、对话准备、菜单权限、技能解析、知识检索、索引迁移、用量聚合）+ 前端 1 条 SSE 解析用例；`go vet` / `go test` / 前端 `vue-tsc` + 构建进 CI |

## 快速开始

```bash
# 1. 复制环境变量模板并填入真实配置
cp .env.example .env
# 必填：JWT 密钥、数据库/Redis/RabbitMQ/MinIO/Manticore 凭据与地址、引导管理员密码
# 可选：NW_AI_* 出厂供应商（留空则登录后在「模型供应商」页添加）

# 2a. 仅应用容器（中间件指向已有实例）
docker compose up -d --build

# 2b. 本地全栈（额外拉起中间件容器；PG 用 pgvector/pgvector:pg16）
docker compose --profile infra up -d --build

# 3. 访问（对外只有一个入口端口）
#   http://<host>:14399           前端 + /api 反向代理
#   容器内后端                  http://backend:8080/health
```

首次启动时，如果模型供应商表为空**且**配置了 `NW_AI_BASE_URL` 与 `NW_AI_CHAT_MODEL`，
系统播种一条默认供应商；否则跳过，登录后到「模型供应商」页添加。用户表为空时用
`NW_APP_ADMIN_PASSWORD` 创建初始管理员 `admin`（默认 `Admin@123456`，生产必改）。

本地开发：

```bash
# 后端（默认读 configs/config.yaml，可用 NW_* 覆盖）
cd backend && go run ./cmd/server

# 前端（/api 代理到 http://127.0.0.1:8080，可用 NW_DEV_PROXY_TARGET 覆盖）
cd frontend && npm install && npm run dev
```

## 技术栈

- 后端：Go 1.24 + Gin 1.11 + GORM(pgx) 1.31 + Viper 1.21 + Zap 1.28
  + eino 0.9（OpenAI 兼容 / Anthropic 双协议对话）+ Redis + RabbitMQ + MinIO + Manticore，
  `CGO_ENABLED=0` 静态编译
- 前端：Vue 3.4 + TypeScript 5.4 + Vite 5.2 + Element Plus 2.9 + Pinia 2.1
  + Vue Router 4.3 + Axios 1.6 + UnoCSS 0.58 + ECharts 5.5 + AntV X6 2.18
  + WangEditor 5.1 + MavonEditor 3.0 + CodeMirror 6 + XLSX + docx-preview + vue-pdf-embed
  + Vue I18n 9.10
- 测试：Go 内置 testing + Vitest
- 部署：Docker Compose / Podman Compose；GitHub Actions CI/CD；静态二进制 + nginx SPA

## 仓库结构

```
backend/            Go 后端：api → service → repo → domain 分层
  cmd/server        装配启动（迁移、播种、路由、优雅关闭）
  internal/         api / service / repo / domain / ai / infra / pkg
frontend/           Vue3 前端：设计系统 + 后端驱动导航 + 业务页面
deploy/podman/      Podman 发布资产：构建上下文、运行镜像、编排与发布脚本
doc/                设计、数据库、API、AI、前端、功能、部署、测试文档
docker-compose.yml  应用编排与本地中间件 profile（对外 14399）
.env.example        环境变量模板（不含真实凭据）
.github/workflows   CI 与镜像发布
```

## 文档

| 文档 | 内容 |
| --- | --- |
| [doc/architecture.md](doc/architecture.md) | 系统定位、技术选型、分层约束、请求生命周期、核心链路、设计取舍 |
| [doc/features.md](doc/features.md) | 每个模块的「用户能力 → 实现 → 验收点」 |
| [doc/database.md](doc/database.md) | 存储分工、表结构（含 `menus` / `skills` / `skill_references`）、索引与一致性 |
| [doc/api.md](doc/api.md) | 接口契约、统一包络、权限码表、字段与错误码 |
| [doc/ai.md](doc/ai.md) | 模型网关、提示词装配、技能体系、无向量检索、流式协议 |
| [doc/frontend.md](doc/frontend.md) | 前端设计系统、四个 store、路由与权限守卫、页面实现 |
| [doc/deploy.md](doc/deploy.md) | 容器、环境变量、Compose/Podman、14399 入口、CI/CD |
| [doc/test.md](doc/test.md) | 测试策略、当前保留的用例、集成验证清单 |

## 约定

- 依赖方向单向：`api → service → repo → domain`；`ai`/`infra`/`pkg` 为叶子能力层。
- ID 为带业务前缀的 ULID（`USR_` / `PRJ_` / `SKL_` / `MNU_` …）；业务响应码为字符串
  （`SUCCESS` / `ERR_*`），HTTP 状态码仍是 int；业务路由仅 GET/POST。
- 接口路径参数一定放在末尾：`/menu/detail/{MenuULID}`、`/skill/detail/{SkillULID}`。
- 字段命名带明确后缀（`project_name`、`genre_code`、`created_at_ms`）与注释。
- 文件保持可维护：Go/前端源文件 ≤500 行，导出符号有注释；大页面拆 CSS/子组件。
- 不引入 cgo；不接入向量模型；稳定性与体验优先于性能。
- 不开放注册：账号只能由管理员在「用户管理」分配。
