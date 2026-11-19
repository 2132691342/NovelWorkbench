# 部署文档

本文档描述 NovelWorkbench 当前的部署设计：两个应用容器（Go 后端 + nginx 前端）、
中间件的两种接入方式（外部实例 / 本地 `infra` profile）、云主机 Podman 发布流程、
CI/CD 与运维要点。所有凭据与地址一律由 `NW_*` 环境变量注入，
仓库内的 `backend/configs/config.yaml` 只保留空值模板。

代码依据：

| 事实来源 | 文件 |
| --- | --- |
| 配置结构与校验 | `backend/internal/infra/config.go`（`Config`、`LoadConfig`、`validate`） |
| 配置模板 | `backend/configs/config.yaml` |
| 启动顺序 | `backend/cmd/server/main.go`（`main`） |
| 本地编排 | `docker-compose.yml`、`.env.example` |
| Podman 编排 | `deploy/podman/docker-compose.yml`、`deploy/podman/.env.example`、`deploy/podman/deploy.sh` |
| 镜像 | `backend/Dockerfile`、`frontend/Dockerfile`、`deploy/podman/Dockerfile.*` |
| 前端模板 | `frontend/nginx.conf.template` |
| 流水线 | `.github/workflows/ci.yml`、`.github/workflows/release.yml` |

---

## 1. 总体架构

应用只有两个容器：后端（Go HTTP 服务）与前端（nginx 托管 SPA 并反向代理 `/api/`）。
中间件（PostgreSQL、Redis、RabbitMQ、MinIO、Manticore）**不在应用镜像内**，
部署时二选一：

| 方案 | 中间件来源 | 编排文件 |
| --- | --- | --- |
| 本地全栈 | 仓库自带的 `infra` profile 拉起同版本中间件容器 | `docker-compose.yml`（根目录） |
| 云主机 / 生产 | 直连宿主机上已运行、处于同一外部网络的中间件实例 | `deploy/podman/docker-compose.yml` |

### 1.1 容器清单

| 容器 | 镜像来源 | 监听端口 | 宿主机端口 | 关键路径 / 说明 |
| --- | --- | --- | --- | --- |
| `nw-backend` | 本地构建 `novel-workbench-backend:latest`；或 GHCR `ghcr.io/<repo>/backend:<tag>`；Podman 由 `deploy.sh` 本地构建 | 8080 | 不发布宿主端口（`expose`），仅容器网络内可达 | `/health` 存活、`/ready` 就绪、`/api/v1/*` 业务接口 |
| `nw-frontend` | 本地构建 `novel-workbench-frontend:latest`；或 GHCR `ghcr.io/<repo>/frontend:<tag>`；Podman 由 `deploy.sh` 本地构建 | 80 | `14399:80`（`NW_PUBLIC_PORT` 可覆盖） | SPA 静态资源、`/healthz` 存活、`/api/` 反向代理到后端 |

**对外只有 14399 一个入口**：浏览器访问 `http://<host>:14399`，所有 `/api/` 请求由前端 nginx
转发到 `backend:8080`。后端不再映射宿主端口，减少暴露面。

中间件容器（仅根目录 Compose 的 `infra` profile）：

| 服务 | 镜像 | 宿主机端口 | 数据卷 |
| --- | --- | --- | --- |
| `nw-postgres` | `pgvector/pgvector:pg16` | 5432 | `nw_pg_data` |
| `nw-redis` | `redis:8-alpine` | 6379 | 无 |
| `nw-rabbitmq` | `rabbitmq:4-management` | 5672 / 15672（管理台） | 无 |
| `nw-minio` | `minio/minio:latest` | 9000（API）/ 9001（控制台） | `nw_minio_data` |
| `nw-manticore` | `manticoresearch/manticore:latest` | 9306 / 9308 | `nw_manticore_data` |

### 1.2 镜像构建方式

| 镜像 | 构建方式 | 要点 |
| --- | --- | --- |
| 后端（Docker） | `backend/Dockerfile` 多阶段 | 构建层 `golang:1.24-alpine`，`GOTOOLCHAIN=local`（禁止构建期拉取其它 Go 版本）、`CGO_ENABLED=0`，`GOOS=linux GOARCH=amd64 go build -tags "netgo osusergo" -ldflags="-s -w"`；运行层 `alpine:3.20` + `ca-certificates` + `tzdata`，非 root 用户 `appuser`(uid 10001)，`EXPOSE 8080` |
| 后端（Podman） | `deploy/podman/Dockerfile.backend` | 仅打包预构建静态二进制 `nw-server`、`configs/config.yaml`、宿主机 CA bundle；`ARG BASE_IMAGE` 可由 `NW_BASE_IMAGE_PREFIX` 覆盖 |
| 前端（Docker） | `frontend/Dockerfile` 多阶段 | 构建层 `node:22-slim`（与 CI 的 Node 版本一致），`npm ci --no-audit --no-fund` + `npm run build`；运行层 `nginx:1.27-alpine`，`ENV NGINX_RESOLVER=127.0.0.11`、`ENV API_UPSTREAM=http://backend:8080` |
| 前端（Podman） | `deploy/podman/Dockerfile.frontend` | 仅打包预构建 `dist` 与 `nginx.conf.template`；`ENV NGINX_RESOLVER=10.89.0.1`、`ENV API_UPSTREAM=http://backend:8080`（由 compose 环境变量覆盖） |

### 1.3 前端 nginx 模板

`frontend/nginx.conf.template` 由 nginx 官方镜像的 `docker-entrypoint` 脚本用 `envsubst` 渲染：

| 变量 | 作用 | Docker 取值 | Podman 取值 |
| --- | --- | --- | --- |
| `NGINX_RESOLVER` | 容器 DNS 地址 | `127.0.0.11`（内嵌 DNS） | `10.89.0.1`（网络网关） |
| `API_UPSTREAM` | 后端上游地址 | `http://backend:8080` | `http://backend.dns.podman:8080` |
| `NW_LISTEN_PORT` | 容器内监听端口 | `80` | `80` |

宿主端口由 compose 的 `ports: "${NW_PUBLIC_PORT:-14399}:80"` 决定，用户访问端口固定 14399；
容器内始终保持 80，便于镜像跨环境复用。

- `listen ${NW_LISTEN_PORT}`：容器内监听端口可配置，默认 80。
- `/healthz` 直接返回 `ok`，不依赖后端。
- `/` 使用 `try_files $uri $uri/ /index.html` 兜底为 SPA 路由。
- `/assets/` 带 hash 的产物长缓存：`expires 30d` + `Cache-Control: public, immutable`。
- `/api/` 用变量形式的 `proxy_pass $api_backend$request_uri` 配合 `resolver ... valid=10s`，
  每次请求重新解析上游 IP，容器重建后不残留旧 IP。
- SSE 流式响应：`proxy_buffering off`、`proxy_read_timeout 3600s`。

---

## 2. 配置模型

### 2.1 加载顺序与覆盖规则

`infra.LoadConfig(configPath)`（`backend/internal/infra/config.go`）：

1. `configPath` 为空时取 `configs/config.yaml`（容器内工作目录为 `/app`，即 `/app/configs/config.yaml`）。
2. Viper `SetConfigFile` + `ReadInConfig` 读取 YAML；读取失败返回错误（启动退出）。
3. `SetEnvPrefix("NW")` + `SetEnvKeyReplacer(strings.NewReplacer(".", "_"))` + `AutomaticEnv()`：
   键 `postgres.host` 对应环境变量 `NW_POSTGRES_HOST`，`app.admin_password` 对应 `NW_APP_ADMIN_PASSWORD`，
   `server.read_timeout_sec` 对应 `NW_SERVER_READ_TIMEOUT_SEC`，依此类推。
4. 遍历 `v.AllKeys()` 逐个 `BindEnv`，使环境变量参与 `Unmarshal`（`AutomaticEnv` 与 `AllSettings` 合并存在盲区，必须显式绑定）。
5. `Unmarshal` 到 `Config`，随后 `validate()` 做启动期校验与默认值补齐。

结论：**环境变量优先于 YAML**；YAML 中所有敏感项（`jwt.secret`、`postgres.password`、
`redis.password`、`rabbitmq.url`、`minio.*_key`、`ai.api_key`、`app.admin_password`）在仓库中一律留空。

### 2.2 启动期校验（fail fast）

`validate()` 中**为空即返回错误、导致进程退出**的字段：

| 字段 | 对应环境变量 | 报错信息 |
| --- | --- | --- |
| `jwt.secret` | `NW_JWT_SECRET` | `jwt.secret is required` |
| `postgres.host` 且 `postgres.dbname` | `NW_POSTGRES_HOST` / `NW_POSTGRES_DBNAME` | `postgres.host/dbname is required` |
| `ai.base_url` 且 `ai.chat_model` | `NW_AI_BASE_URL` / `NW_AI_CHAT_MODEL` | 不校验：模型网关不是启动必需项，留空则不播种默认供应商 |

`validate()` 中**缺失时补默认值**（不报错）的字段：

| 字段 | 默认值 | 对应环境变量 |
| --- | --- | --- |
| `ai.protocol_code` | `openai` | `NW_AI_PROTOCOL_CODE` |
| `app.build_version` | `dev` | `NW_APP_BUILD_VERSION` |
| `server.port` | `8080` | `NW_SERVER_PORT` |
| `ai.history_count`（≤0） | `5` | `NW_AI_HISTORY_COUNT` |
| `app.rag_top_k`（≤0） | `6` | `NW_APP_RAG_TOP_K` |
| `app.admin_password` | `Admin@123456`（生产必须覆盖） | `NW_APP_ADMIN_PASSWORD` |

**降级项**（配置缺失或连接失败只告警，不阻断启动）：
`redis`（令牌吊销降级）、`rabbitmq`（异步索引任务降级）、`minio`（对象存储降级）、
`manticore`（全文检索降级）。其中 `redis.addr` 为空属于「Redis 配置无效」，仍会退出；
`postgres` 相关配置无效或连不上直接退出。

### 2.3 JWT 密钥兼作密钥封存材料（重要）

模型供应商的 API Key 以 **AES-256-GCM 密文**保存在数据库 `model_providers.api_key_cipher`，
封存材料由 `NW_JWT_SECRET` 经 SHA-256 派生
（`backend/internal/pkg/secret.go` 的 `SealSecret` / `OpenSecret`，
调用方 `service.NewModelProviderService(..., cfg.JWT.Secret, ...)`）。

> **更换 `NW_JWT_SECRET` 后，数据库中已封存的供应商 API Key 全部无法解密**，
> 解封失败时页面提示「供应商密钥无法解密，请重新填写 API Key」。
> 轮换后必须在「模型供应商」页面重新填写各供应商的 API Key 并保存。
> 同时所有已签发 JWT 失效，用户需重新登录。

---

## 3. 环境变量总表

下表列出编排文件与代码中**实际被消费**的全部 `NW_*` 变量。
「必填」列的含义：编排文件中用 `${VAR:?}` 强制校验（缺值直接启动失败）或代码校验为必填。

### 3.1 server

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_SERVER_PORT` | 否 | 后端监听端口，缺省 `8080` | `8080` |
| `NW_SERVER_MODE` | 否 | gin 运行模式（`debug`/`release`/`test`）；Podman 编排固定 `release` | `release` |
| `NW_SERVER_HOST` | 否 | 监听地址，缺省 `0.0.0.0` | `0.0.0.0` |

### 3.2 jwt

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_JWT_SECRET` | 是 | JWT 签名密钥；同时作为供应商 API Key 的封存材料（见 2.3） | `replace-with-a-long-random-secret` |

`jwt.expire_hours` 默认 `168`（7 天），仓库内不通过环境变量覆盖。

### 3.3 postgres

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_POSTGRES_HOST` | 是 | 数据库主机 | `postgres` |
| `NW_POSTGRES_PORT` | 否 | 端口，缺省 `5432` | `5432` |
| `NW_POSTGRES_USER` | 否 | 账号，缺省 `postgres`（根目录 Compose 的 `infra` profile 中为必填） | `postgres` |
| `NW_POSTGRES_PASSWORD` | 是 | 口令 | `replace-with-db-password` |
| `NW_POSTGRES_DBNAME` | 否 | 库名，缺省 `postgres`（根目录 Compose 的 `infra` profile 中为必填） | `postgres` |

`postgres.sslmode`（缺省 `disable`）、`max_open_conns`（20）、`max_idle_conns`（5）、
`conn_max_lifetime_min`（30）由 YAML 提供，无对应环境变量。

### 3.4 redis

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_REDIS_ADDR` | 是 | Redis 地址 `host:port`；为空则进程退出 | `redis:6379` |
| `NW_REDIS_PASSWORD` | 是（编排层） | Redis 口令；代码层不校验，为空则连接被拒并降级 | `replace-with-redis-password` |

`redis.db` 固定 `0`。

### 3.5 rabbitmq

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_RABBITMQ_URL` | 是 | AMQP 连接串 `amqp://user:pass@host:5672/` | `amqp://localdevuser:replace-with-rabbitmq-password@rabbitmq:5672/` |
| `NW_RABBITMQ_USER` | 视编排而定 | RabbitMQ 账号：根目录 Compose 的 `infra` profile 中用于初始化中间件容器并为必填；Podman 编排中拼进 `NW_RABBITMQ_URL`，缺省 `admin` | `localdevuser` / `admin` |
| `NW_RABBITMQ_PASSWORD` | 视编排而定 | RabbitMQ 口令：同上 | `replace-with-rabbitmq-password` |

`rabbitmq.queue_doc_index` 固定 `nw_doc_index`。

### 3.6 minio

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_MINIO_ENDPOINT` | 是 | 对象存储地址 `host:port` | `minio:9000` |
| `NW_MINIO_ACCESS_KEY` | 是 | AccessKey | `replace-with-minio-access-key` |
| `NW_MINIO_SECRET_KEY` | 是 | SecretKey | `replace-with-minio-secret-key` |
| `NW_MINIO_BUCKET` | 否 | 存储桶名，缺省 `novelworkbench`（根目录 Compose 支持覆盖；Podman 编排固定该值） | `novelworkbench` |

`minio.use_ssl` 固定 `false`。

### 3.7 manticore

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_MANTICORE_BASE_URL` | 是 | 全文检索 HTTP SQL API 地址（形如 `http://host:9308`） | `http://manticore:9308` |

`manticore.table_chunks` 固定 `nw_chunks`。

### 3.8 ai（**仅用于一次性出厂默认供应商播种**）

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_AI_PROTOCOL_CODE` | 否 | 协议：`openai` / `anthropic`，缺省 `openai` | `openai` |
| `NW_AI_BASE_URL` | 否 | 模型网关地址；OpenAI 兼容需含 `/v1`，Anthropic 填根地址。留空则不播种默认供应商 | `https://model-gateway.example.com/v1` |
| `NW_AI_API_KEY` | 否 | 模型网关密钥。留空则不播种默认供应商 | `replace-with-model-api-key` |
| `NW_AI_CHAT_MODEL` | 否 | 对话模型名。留空则不播种默认供应商 | `replace-with-chat-model` |
| `NW_AI_TEMPERATURE` | 否 | 采样温度，缺省 `0.7` | `0.7` |
| `NW_AI_MAX_TOKENS` | 否 | 单次输出上限，缺省 `2048` | `2048` |
| `NW_AI_TIMEOUT_SEC` | 否 | 单次模型调用超时（秒），缺省 `120`；≤0 时 main.go 兜底为 `60` | `120` |
| `NW_AI_HISTORY_COUNT` | 否 | `@历史` 引用默认注入条数，缺省 `5` | `5` |

> **作用域限制**：这一组变量只在 `model_providers` 表为空时写入一条「默认模型网关」记录
> （`providerService.SeedDefault`）。播种后模型供应商由「模型供应商」页面维护
> （多供应商、OpenAI 兼容 / Anthropic 双协议、连通性测试），配置文件不再参与。
> 因此播种失败只会打印告警，不会阻断启动。

### 3.9 app

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_APP_UPLOAD_MAX_MB` | 否 | 知识库上传大小上限（MB），缺省 `20` | `20` |
| `NW_APP_CHUNK_SIZE` | 否 | 知识库切片字符数，缺省 `800` | `800` |
| `NW_APP_CHUNK_OVERLAP` | 否 | 知识库切片重叠字符数，缺省 `120` | `120` |
| `NW_APP_RAG_TOP_K` | 否 | RAG 默认召回条数，缺省 `6` | `6` |
| `NW_APP_ADMIN_PASSWORD` | 是 | 引导管理员初始密码，仅用户表为空时生效；代码兜底 `Admin@123456` | `replace-with-a-strong-admin-password` |
| `NW_APP_BUILD_VERSION` | 否 | 运行状态页展示的构建版本，缺省 `dev` | `dev` |

### 3.10 Podman 专用变量

| 变量 | 必填 | 含义 | 占位值 |
| --- | --- | --- | --- |
| `NW_INFRA_NETWORK` | 否 | 既有中间件栈的外部网络名，缺省 `novelworkbench_default` | `novelworkbench_default` |
| `NW_NGINX_RESOLVER` | 否 | 前端 nginx 的 resolver 地址，缺省 `10.89.0.1`（podman 网络网关） | `10.89.0.1` |
| `NW_API_UPSTREAM` | 否 | 前端 nginx 的后端上游，缺省 `http://backend.dns.podman:8080` | `http://backend.dns.podman:8080` |
| `NW_BASE_IMAGE_PREFIX` | 否 | 基础镜像仓库前缀，缺省 `swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library`（`deploy.sh` 使用） | `swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library` |
| `NW_BACKEND_IMAGE` | 否 | 后端运行镜像名，缺省 `localhost/novel-workbench-backend:latest` | `localhost/novel-workbench-backend:latest` |
| `NW_FRONTEND_IMAGE` | 否 | 前端运行镜像名，缺省 `localhost/novel-workbench-frontend:latest` | `localhost/novel-workbench-frontend:latest` |
| `NW_PUBLIC_PORT` | 否 | 前端容器映射到宿主机的端口，缺省 `14399`（唯一对外入口） | `14399` |

`deploy.sh` 另支持两个非 `NW_` 的可选变量：`ENV_FILE`（默认 `.env`）、
`COMPOSE_LOG`（默认 `/var/log/novelworkbench-compose.log`）。

---

## 4. 启动初始化行为

`backend/cmd/server/main.go` 的 `main()` 顺序：

| 序号 | 步骤 | 失败后果 |
| --- | --- | --- |
| 1 | `pkg.SetLogger(zap.NewProduction())` 设定全局 JSON 日志 | — |
| 2 | `infra.LoadConfig("")` 读 YAML + 应用 `NW_*` 覆盖 + `validate()` | **致命**：退出码 1，日志「加载配置失败」 |
| 3 | `signal.NotifyContext` 注册 SIGINT/SIGTERM（供后续优雅关闭） | — |
| 4 | `infra.NewPostgres` 建立连接 | **致命**：退出码 1，日志「Postgres 连接失败」 |
| 5 | `db.AutoMigrate(...)` 建表（28 张表：`UserDO`、`ProjectDO`、`VolumeDO`、`ChapterDO`、`ChapterSnapshotDO`、`WordCountStatDO`、`CharacterDO`、`CharacterRelationDO`、`LoreDO`、`KnowledgeDocumentDO`、`OutlineNodeDO`、`DetailedSceneDO`、`TemporalFactDO`、`ForeshadowHookDO`、`ChapterSummaryDO`、`KnowledgeChunkDO`、`SkillDO`、`SkillReferenceDO`、`MenuDO`、`JobDO`、`AppSettingDO`、`ChatSessionDO`、`ChatMessageDO`、`RoleDO`、`UserRoleDO`、`AuditLogDO`、`ModelProviderDO`、`ModelUsageStatDO`） | **致命**：退出码 1，日志「数据库迁移失败」 |
| 6 | `infra.NewRedis` 连接并 Ping（5s 超时） | 地址为空 → **致命**（「Redis 配置无效」）；Ping 失败 → **降级告警**「令牌吊销降级」，仍返回可用客户端 |
| 7 | `infra.NewRabbitMQ` 建立连接 | **降级告警**「异步任务降级」 |
| 8 | `infra.NewMinIO` 构造客户端 + `BucketExists`（桶不存在时自动 `MakeBucket` 创建） | **降级告警**「对象存储降级」 |
| 9 | `infra.NewManticore` 构造 + `Ping`（`SELECT 1`，HTTP 超时 5s） | **降级告警**「全文检索降级」 |
| 10 | `pkg.NewJWTSigner(cfg.JWT.Secret, cfg.JWTExpireTTL())` | **致命**：退出码 1 |
| 11 | 装配 repo / service / ai client（含 `aiTimeoutSec` ≤0 时兜底 `60`）；`providerService.SeedDefault` 播种默认供应商 | 播种失败 → **告警**「默认模型供应商播种失败，可在页面手工添加」 |
| 12 | `skillService.SeedDefaults`（内置技能） | **致命**：退出码 1 |
| 12.1 | `menuService.SeedDefaults`（内置菜单与权限登记） | **致命**：退出码 1 |
| 13 | `settingService.SeedDefaults`（默认应用配置） | **致命**：退出码 1 |
| 14 | `roleService.SeedDefaults`（内置角色 admin / user） | **致命**：退出码 1 |
| 15 | `userAdminService.BootstrapAdmin(ctx, "admin", "admin@novelworkbench.local", cfg.App.AdminPassword)`（仅用户表为空时创建） | **致命**：退出码 1 |
| 16 | 启动知识库索引 worker goroutine，消费 RabbitMQ 完成文档索引 | 异常退出 → **告警**「知识库索引 worker 异常退出」 |
| 17 | 注册 Auth / Audit 中间件与全部路由，`http.Server`（`WriteTimeout: 0`，SSE 不受写超时限制）并 `ListenAndServe` | 监听失败 → 进入关闭流程并记录错误 |

---

## 5. 本地 Docker Compose 部署

前置条件：Docker + Docker Compose 插件。

```bash
# 1. 准备环境变量（.env 已在 .gitignore 中，切勿提交）
cp .env.example .env
# 编辑 .env，填入 NW_JWT_SECRET、数据库/缓存/队列/对象存储口令、模型网关信息

# 2a. 仅应用容器（中间件指向已有实例或云服务器）
docker compose up -d --build

# 2b. 本地全栈（额外拉起 PostgreSQL/Redis/RabbitMQ/MinIO/Manticore）
docker compose --profile infra up -d --build

# 3. 健康检查（后端不映射宿主端口，只能经前端入口反代访问）
curl http://127.0.0.1:14399/healthz         # 前端存活
# 后端探针挂在根引擎（不在 /api/v1 下），nginx 未反代，只能进容器内查：
docker compose exec backend wget -qO- http://127.0.0.1:8080/health
docker compose exec backend wget -qO- http://127.0.0.1:8080/ready

# 4. 日志与停止
docker compose logs -f backend
docker compose down                   # 加 -v 会删除 infra 数据卷
```

### 5.1 `infra` profile 与应用共用同一组凭据

`infra` profile 的中间件容器与应用容器读同一份 `.env`，避免两侧口令不一致：

| 中间件容器 | 由哪个变量初始化 | 应用容器读哪个变量 |
| --- | --- | --- |
| `nw-postgres` | `NW_POSTGRES_USER` / `NW_POSTGRES_PASSWORD` / `NW_POSTGRES_DBNAME` | 同名三项 |
| `nw-redis` | `NW_REDIS_PASSWORD`（`redis-server --requirepass`，`--maxmemory 256mb --maxmemory-policy allkeys-lru`） | `NW_REDIS_ADDR` + `NW_REDIS_PASSWORD` |
| `nw-rabbitmq` | `NW_RABBITMQ_USER` / `NW_RABBITMQ_PASSWORD` | `NW_RABBITMQ_URL` |
| `nw-minio` | `NW_MINIO_ACCESS_KEY` / `NW_MINIO_SECRET_KEY`（`MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD`） | 同名两项 |
| `nw-manticore` | 无凭据（无鉴权） | `NW_MANTICORE_BASE_URL` |

对应地，本地全栈时 `.env` 中的地址必须写成**服务名**，与 `infra` profile 一致：
`NW_POSTGRES_HOST=postgres`、`NW_REDIS_ADDR=redis:6379`、`NW_MINIO_ENDPOINT=minio:9000`、
`NW_RABBITMQ_URL=amqp://<user>:<pass>@rabbitmq:5672/`、`NW_MANTICORE_BASE_URL=http://manticore:9308`。

### 5.2 本地中间件说明

- 本地 PostgreSQL 用 `pgvector/pgvector:pg16` 镜像（自带 pgvector 扩展的官方派生镜像）；
  全文检索能力由 Manticore（`nw_chunks` 表）提供，不依赖 Postgres 侧的全文索引。
- 根目录 Compose 的后端健康检查为
  `wget -qO- http://127.0.0.1:8080/health`，`interval 30s / timeout 5s / retries 3`；
  前端健康检查为 `wget -qO- http://127.0.0.1/healthz`，参数同上。
- 前端 `NGINX_RESOLVER=127.0.0.11`（Docker 内嵌 DNS），
  `API_UPSTREAM=http://backend:8080`（内嵌 DNS 应答裸服务名）。

---

## 6. 云主机 Podman 部署

目标主机运行 **Podman + podman-compose**（无 Docker），中间件栈已在宿主机运行，
并处于一个外部网络中（例如 `novelworkbench_default`），
服务别名为 `postgres` / `redis` / `manticore` / `minio` / `rabbitmq`。
应用容器加入该网络直连中间件，不发布中间件端口。

### 6.1 `deploy/podman/` 文件清单

| 文件 | 作用 |
| --- | --- |
| `docker-compose.yml` | Podman 编排，仅 backend / frontend 两个服务；`infra` 网络 `external: true`，名称取 `NW_INFRA_NETWORK`；所有凭据由 `.env` 注入，文件内无任何可用默认值 |
| `Dockerfile.backend` | 后端运行镜像：`ARG BASE_IMAGE`（默认 `docker.io/library/alpine:3.20`），拷贝 `ca-certificates.crt`、`nw-server`、`config.yaml`，创建非 root `appuser`(10001)，`EXPOSE 8080`，`ENTRYPOINT ["/app/nw-server"]` |
| `Dockerfile.frontend` | 前端运行镜像：`ARG BASE_IMAGE`（默认 `docker.io/library/nginx:1.27-alpine`），拷贝 `dist` 与 `nginx.conf.template`，`EXPOSE 80` |
| `deploy.sh` | 发布脚本：预检 → 组装构建上下文 → 构建镜像 → 清理陈旧 DNAT → 重建容器 → 健康自检 |
| `.env.example` | 环境变量占位模板（复制为 `.env` 后填写） |

Podman 编排的其余要点：

- `NW_SERVER_MODE: "release"`；backend 只 `expose: 8080`（不映射宿主端口），
  frontend 映射 `${NW_PUBLIC_PORT:-14399}:80`；`mem_limit: 384m`；`restart: always`。
- backend 容器内健康检查 `wget -qO- http://127.0.0.1:8080/health`；frontend 健康检查
  `wget -qO- http://127.0.0.1:80/healthz`；两者均为 `interval 30s / timeout 5s / retries 3`。
- Podman 编排不映射后端宿主端口；发布脚本用 `podman exec nw-backend wget ...` 探活，
  避免 rootless podman 重建后的陈旧 DNAT 规则导致宿主 8080 不可达。
- Podman 编排中 `NW_POSTGRES_HOST/PORT`、`NW_REDIS_ADDR`、`NW_MINIO_ENDPOINT`、
  `NW_MANTICORE_BASE_URL` 全部固定为中间件网络内的服务名，
  不需要配置公网主机地址。

### 6.2 开发机准备产物

```bash
# 后端：交叉编译 linux/amd64 静态二进制（禁 cgo）
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -tags "netgo osusergo" -ldflags="-s -w" \
  -o build/nw-server-linux-amd64 ./cmd/server

# 前端：构建静态资源
cd ../frontend
npm ci
npm run build
```

然后把整个仓库目录上传到云主机（例如 `/opt/novelworkbench`），产物路径需与
`deploy.sh` 的期望一致：`backend/build/nw-server-linux-amd64`、`frontend/dist/index.html`、
`frontend/nginx.conf.template`、`backend/configs/config.yaml`。

### 6.3 云主机发布

```bash
cd /opt/novelworkbench/deploy/podman
cp .env.example .env
# 编辑 .env：填入密钥、模型网关信息；按实际网络名设置 NW_INFRA_NETWORK；
# 确认 NW_PUBLIC_PORT=14399（唯一对外入口，云安全组需放行该端口）
bash deploy.sh
```

`deploy.sh` 逐步执行：

1. **预检**：`backend/build/nw-server-linux-amd64`、`frontend/dist/index.html`、
   `frontend/nginx.conf.template`、`backend/configs/config.yaml`、`.env` 必须存在，缺一即退出；
   随后 `set -a; . ./.env; set +a` 把 `.env` 导出为环境变量
   （`NW_BASE_IMAGE_PREFIX` / `NW_BACKEND_IMAGE` / `NW_FRONTEND_IMAGE` 在此生效）。
2. **组装构建上下文**：清空并重建 `deploy/podman/build/`，
   拷入后端二进制（重命名为 `nw-server`）、`config.yaml`、
   **宿主机 CA bundle** `/etc/ssl/certs/ca-certificates.crt`（缺失即退出），
   以及前端 `dist/` 与 `nginx.conf.template`。
3. **构建运行镜像**：`podman build -f Dockerfile.backend --build-arg BASE_IMAGE=${NW_BASE_IMAGE_PREFIX:-swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library}/alpine:3.20`，
   前端同理使用 `${BASE_PREFIX}/nginx:1.27-alpine`；镜像名取 `NW_BACKEND_IMAGE` /
   `NW_FRONTEND_IMAGE`，缺省 `localhost/novel-workbench-{backend,frontend}:latest`。
4. **清理陈旧 DNAT**：rootless podman 重建容器后残留的 `CNI-HOSTPORT-DNAT` 规则会导致
   宿主端口不可达。`drop_stale_dnat()` 删除指向 `127.0.0.1` 的空规则；
   `cleanup_port_dnat()` 在重建前删除 `NW_PUBLIC_PORT`（默认 14399）与 8080 的全部
   `CNI-DN-*` 跳转规则，容器启动时 podman 只为新容器写入一条规则。无 `iptables` 命令时跳过。
5. **重建容器**：`podman-compose -f docker-compose.yml up -d --force-recreate`，
   输出重定向到服务器本地日志（默认 `/var/log/novelworkbench-compose.log`）。
   之所以重定向：podman-compose 会把含明文口令的 `podman run` 命令打到 stdout；
   之所以加 `--force-recreate`：podman-compose 1.x 不会因镜像 ID 变化自动重建容器。
   失败时打印日志尾部 40 行并退出；成功后再次清理 DNAT。
6. **健康自检**：最多 `30` 次、间隔 `2` 秒轮询
   `podman exec nw-backend wget -qO- http://127.0.0.1:8080/health`、
   `podman exec nw-backend wget -qO- http://127.0.0.1:8080/ready`、
   `curl -fsS http://127.0.0.1:${NW_PUBLIC_PORT:-14399}/healthz`，
   三者全部有响应且 `/healthz` 内容为 `ok` 即判定成功，
   打印 `podman ps` 列表并退出 0；超时则打印容器状态与后端日志尾部（过滤 `password` 行）后退出非 0。

### 6.4 DNS 差异（关键）

| 环境 | DNS 行为 | `API_UPSTREAM` 写法 |
| --- | --- | --- |
| Docker | 内嵌 DNS（`127.0.0.11`）应答裸服务名 | `http://backend:8080` |
| Podman | aardvark/dnsname 只应答 `search` 域 FQDN（`resolv.conf` 的 `search dns.podman`），nginx resolver 不做后缀补全 | `http://backend.dns.podman:8080` |

因此 Podman 编排中 `API_UPSTREAM` 必须写完整域名 `backend.dns.podman`；
写裸 `backend` 会导致 nginx 解析失败、`/api/` 返回 502。

---

## 7. CI/CD

### 7.1 工作流总览

| 工作流 | 文件 | 触发 | Job | 内容 |
| --- | --- | --- | --- | --- |
| CI | `.github/workflows/ci.yml` | `push` 到 `main`；`pull_request` 目标 `main` | `backend` | `go vet ./...`；`CGO_ENABLED=0 go build -tags "netgo osusergo" ./...`；`CGO_ENABLED=0 go test -short -count=1 -shuffle=on ./...` |
| CI | 同上 | 同上 | `frontend` | `npm ci`；`npm run test`；`npm run build`（`vue-tsc && vite build`） |
| CI | 同上 | 同上 | `docker`（`needs: [backend, frontend]`） | `docker build -t novel-workbench-backend:ci ./backend`；`docker build -t novel-workbench-frontend:ci ./frontend` |
| Release | `.github/workflows/release.yml` | 推送 `v*` tag | `release` | 归一镜像前缀 → 登录 GHCR → 构建并推送 backend / frontend 镜像（同时打 `:latest`） |

CI 细节：

- `backend` job 设 `GOTOOLCHAIN: local`（与 `backend/Dockerfile` 一致：go.mod 版本必须能被固定工具链直接构建）
  与 `CGO_ENABLED: "0"`（全流水线静态构建，禁止 cgo）；`working-directory: backend`。
- `setup-go` 用 `go-version: "1.24"`、`cache-dependency-path: backend/go.sum`。
- `setup-node` 用 `node-version: "22"`、`cache: npm`、
  **`cache-dependency-path: frontend/package-lock.json`**。
  `setup-node` 的缓存依赖路径相对**仓库根**解析，而前端锁文件在 `frontend/` 下；
  若缺省会直接报 `Dependencies lock file is not found` 并使整个 Frontend Check 失败。
- Release job 需要 `permissions: contents: read` 与 `packages: write`。

### 7.2 GHCR 小写仓库名归一

GHCR 要求镜像仓库名全小写，而 GitHub 仓库名允许含大写字母。
Release 工作流在 `Normalize image prefix` 步骤归一：

```bash
echo "prefix=ghcr.io/${GITHUB_REPOSITORY,,}" >> "$GITHUB_OUTPUT"
```

`${GITHUB_REPOSITORY,,}` 是 bash 小写展开。例如 `2132691342/NovelWorkbench`
→ `ghcr.io/2132691342/novelworkbench`，两个镜像构建步骤共用该 `prefix` 输出：

| 镜像 | 标签 |
| --- | --- |
| `<prefix>/backend` | `${{ github.ref_name }}`（tag 名）与 `latest` |
| `<prefix>/frontend` | `${{ github.ref_name }}`（tag 名）与 `latest` |

登录 GHCR 使用 `registry: ghcr.io`、`username: ${{ github.actor }}`、
`password: ${{ secrets.GITHUB_TOKEN }}`。

### 7.3 Action 版本

| Action | 版本 |
| --- | --- |
| `actions/checkout` | v5 |
| `actions/setup-go` | v6 |
| `actions/setup-node` | v5 |
| `docker/setup-buildx-action` | v4 |
| `docker/login-action` | v4 |
| `docker/build-push-action` | v7 |

### 7.4 发布与更新

```bash
# 打 tag 触发 Release 工作流
git tag v1.0.0 && git push origin v1.0.0

# 使用 GHCR 镜像的主机拉取并滚动更新
docker compose pull && docker compose up -d
docker compose ps
```

Podman 主机不拉 GHCR，由 `deploy.sh` 从预构建产物本地构建运行镜像（见第 6 节）。

---

## 8. 运维要点

| 事项 | 做法 |
| --- | --- |
| 健康检查 | 后端 `/health` 只回存活（`{"status":"up"}`）；后端 `/ready` Ping PostgreSQL（2s 超时），失败返回「依赖未就绪」；前端 `/healthz` 恒回 `ok`，不依赖后端；nginx 上游不可达时 `/api/` 返回 502 |
| 容器健康探针 | 编排中 backend 用容器内 `wget -qO- http://127.0.0.1:8080/health`，frontend 用 `wget -qO- http://127.0.0.1/healthz`（Podman 为 `:80/healthz`），均为 30s 间隔 / 5s 超时 / 3 次重试；发布脚本同样用 `podman exec` 探后端 |
| 日志 | 后端输出 zap 生产级 JSON 到 stdout；`docker compose logs -f backend` / `podman logs -f nw-backend` 查看；`podman-compose` 的编排输出落在服务器本地日志 `/var/log/novelworkbench-compose.log`（含明文口令，不要外传） |
| 优雅关闭 | 收到 SIGTERM/SIGINT 后：HTTP Server `Shutdown`（5s 超时）→ 等待知识库索引 worker 退出（5s 超时）→ 依次关闭 Postgres、Redis、RabbitMQ 连接 |
| 配置变更 | 修改 `NW_*` 环境变量后重建容器（`docker compose up -d` 或 `bash deploy.sh`）；凭据只放 `.env`，不要写回 `backend/configs/config.yaml`；YAML 中敏感项保持空值 |
| 密钥轮换 | 轮换 `NW_JWT_SECRET` 会使已签发 JWT 全部失效，并导致数据库中封存的供应商 API Key 无法解密；**必须在「模型供应商」页面重新填写各供应商 API Key**（见 2.3） |
| 中间件凭据变更 | 变更 PostgreSQL / Redis / RabbitMQ / MinIO 口令时，需同步更新 `.env` 中对应的**中间件初始化变量与应用连接变量**（如 RabbitMQ 需同时改 `NW_RABBITMQ_USER`/`NW_RABBITMQ_PASSWORD`/`NW_RABBITMQ_URL`），否则应用与中间件将使用不同凭据 |
| 备份 | 按中间件方案执行：PostgreSQL 定期 dump（`nw_pg_data` 或 `pg_dump`）、MinIO 桶数据备份（`nw_minio_data`）、Manticore 索引可由数据库切片重建 |
| 状态页 | 应用内「运行状态」接口（`service.NewSystemStatusService`）会逐项探测 PostgreSQL / Redis / RabbitMQ / MinIO / Manticore 并展示 `NW_APP_BUILD_VERSION`，可用于确认当前部署版本与依赖健康 |
