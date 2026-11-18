# NovelWorkbench

AI 小说创作工作台：前后端分离、容器化交付。以"项目—卷—章节"为主线，把 AI 流式生成、斜杠技能、知识库混合检索、角色设定管理统一到一条可控的创作链路上。

## 功能

| 模块 | 说明 |
| --- | --- |
| 写作工作台 | 卷/章目录树、正文编辑（CodeMirror）、字数实时统计、快照留档与回滚、XLSX 大纲导出 |
| AI 助手 | SSE 流式对话；内置 9 个斜杠技能（续写/大纲/润色/扩写/缩写/梗概/审稿/取名/设定问答）+ 自定义技能；@引用文档与章节；最近 N 条历史注入（可配置）；可按供应商切换模型 |
| 模型生态 | 多供应商管理（OpenAI 兼容 / Anthropic 双协议）；密钥 AES-256-GCM 加密存储、仅回显掩码；一键连通性测试（对话 + 向量通道）；默认供应商唯一；AI 能力就绪提示 |
| 知识库 | 文档上传（txt/md/docx/pdf）、异步切片+向量化+全文索引（Manticore CJK ngram）、RRF 混合检索、原文预览、召回依据随回复展示；无向量模型时自动降级为关键词检索 |
| 设定管理 | 角色卡、角色关系图（AntV X6）、世界观设定（分类/搜索/编辑）、创作笔记 |
| 项目导航 | 侧边栏项目切换器 + 项目内直达（写作台/角色/设定/知识库/笔记），登录后无需回仪表盘找入口 |
| 数据总览 | 人工/AI 双来源字数趋势（ECharts）、项目统计 |
| 运维管理 | 用量分析（调用/token/延迟/成功率，按供应商与模型分布）、运行状态（PG/Redis/RabbitMQ/MinIO/Manticore/模型健康）、用户/角色/审计 |
| 系统 | JWT 认证（禁用账号旧令牌立即失效）、SaaS 管理（用户/角色权限/审计日志）、项目 CRUD、运行时配置、斜杠命令管理、中英双语（Vue I18n） |

## 技术栈

- **后端**：Go 1.24 + gin + gorm(pgx) + viper + zap + eino（OpenAI 兼容 + Anthropic 双协议）+ Redis + RabbitMQ + MinIO + Manticore + pgvector，`CGO_ENABLED=0` 静态编译
- **前端**：Vue 3.4 + TypeScript 5.4 + Vite 5.2 + Element Plus 2.9 + Pinia + Vue Router + Axios + UnoCSS + ECharts + AntV X6 + CodeMirror 6 + WangEditor 5 + XLSX + Vue I18n
- **部署**：Docker / docker-compose 与 Podman / podman-compose 双形态，GitHub Actions CI/CD（CI 检查 + tag 触发镜像发布 GHCR）

## 快速开始

```bash
# 1. 启动（中间件默认指向云服务器；本地全栈加 --profile infra）
docker compose up -d --build

# 2. 配置模型供应商（二选一）
#    A. 环境变量给出"出厂默认"供应商：NW_AI_PROTOCOL_CODE / NW_AI_BASE_URL / NW_AI_API_KEY /
#       NW_AI_CHAT_MODEL / NW_AI_EMBEDDING_MODEL（供应商表为空时播种一次）
#    B. 登录后进「模型供应商」页添加（推荐：可测连通、可切默认、可加多个）

# 3. 访问
#    前端 http://localhost        后端 http://localhost:8080/health
```

本地开发：

```bash
cd backend && go run ./cmd/server        # 需可达中间件（配置见 backend/configs/config.yaml）
cd frontend && npm install && npm run dev # http://localhost:5173，/api 代理到 8080
```

## 仓库结构

```
backend/            Go 后端（api → service → repo → domain 分层）
  cmd/server        装配启动
  internal/         api / service / repo / domain / ai / infra / pkg
frontend/           Vue3 前端（设计系统 + 页面）
deploy/             部署资产（podman 编排/发布脚本；nginx 配置模板在 frontend/nginx.conf.template）
doc/                设计文档（架构/数据库/API/AI/前端/部署/测试）
docker-compose.yml  应用编排（--profile infra 拉起本地中间件）
.github/workflows   CI（ci.yml）与镜像发布（release.yml）
```

## 文档

| 文档 | 内容 |
| --- | --- |
| [doc/architecture.md](doc/architecture.md) | 总体架构、分层约束、关键链路与取舍 |
| [doc/database.md](doc/database.md) | 表结构与非关系存储设计 |
| [doc/api.md](doc/api.md) | 接口契约（统一包络、字段表） |
| [doc/ai.md](doc/ai.md) | AI 能力：提示词装配、技能、RAG、流式稳定性 |
| [doc/frontend.md](doc/frontend.md) | 前端设计系统（五色/按钮五态）、状态与页面交互 |
| [doc/deploy.md](doc/deploy.md) | 镜像构建、compose、CI/CD、运维要点 |
| [doc/test.md](doc/test.md) | 测试范围与运行方式 |

## 约定

- ID 为带业务前缀的 ULID（`PRJ_01H…`）；业务响应码为字符串（`SUCCESS`/`ERR_*`）；路由仅 GET/POST
- 字段命名带明确后缀（`user_name`/`genre_code`），时间戳为毫秒 int64（`xxx_at_ms`）
- 稳定性与体验优先于性能；不做过度封装，不重复造轮子
