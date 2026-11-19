# 部署文档

本文档只描述 NovelWorkbench 当前的部署设计：两个容器镜像（Go 后端 + nginx 前端）、
可选本地中间件编排、云主机 Podman 发布流程、CI/CD 与镜像仓库。所有凭据与地址
都从环境变量注入，仓库中的 YAML 只保留空值模板。

## 1. 总体架构

系统由两个应用容器组成，中间件（PostgreSQL、Redis、RabbitMQ、MinIO、Manticore）
不在应用镜像中，部署时二选一：

- **本地全栈开发**：用仓库根目录 `docker-compose.yml` 的 `infra` profile 拉起同版本中间件容器。
- **生产/云主机**：直连运行中的中间件实例（云服务器或已有中间件栈），地址通过 `NW_*` 环境变量传入。

| 容器 | 镜像 | 对外端口 | 关键路径 |
| --- | --- | --- | --- |
| backend | `novel-workbench-backend:latest`（本地构建）或 `ghcr.io/<repo>/backend:<tag>` | 8080 | `/health` 存活、`/ready` 依赖就绪、`/api/*` 业务接口 |
| frontend | `novel-workbench-frontend:latest` 或 `ghcr.io/<repo>/frontend:<tag>` | 80 | SPA 静态资源、`/healthz` 存活、`/api/*` 反向代理到后端 |

前端 nginx 用模板 `frontend/nginx.conf.template` 在容器启动时按环境变量渲染：

- `NGINX_RESOLVER`：容器 DNS 地址（Docker 内嵌 DNS 为 `127.0.0.11`；Podman 通常为网络网关，如 `10.89.0.1`）。
- `API_UPSTREAM`：后端上游地址（Docker 编排为 `http://backend:8080`；Podman 编排为 `http://backend.dns.podman:8080`）。
- `/api/` 使用变量 `proxy_pass` + `resolver`，每次请求重新解析上游 IP，容器重建后不留旧 IP。
- SSE 流式接口关闭缓冲（`proxy_buffering off`），读超时 3600s。

后端镜像为多阶段构建：构建层 Go 1.24 Alpine，固定 `GOTOOLCHAIN=local`、
`CGO_ENABLED=0`，使用 `-tags "netgo osusergo"` 静态编译；运行层 Alpine + CA 证书
（访问 HTTPS 模型网关需要）+ 非 root 用户。所有 Go 构建路径均禁止 cgo，
包括 CI 构建步骤。

## 2. 配置模型

### 2.1 配置文件与环境变量

后端启动时读取 `configs/config.yaml`（容器中为 `/app/configs/config.yaml`），
然后以 `NW_` 为前缀、`.` → `_` 替换键名，用环境变量覆盖同名配置项。
例如 `postgres.host` 对应 `NW_POSTGRES_HOST`，`app.admin_password` 对应
`NW_APP_ADMIN_PASSWORD`。

`backend/configs/config.yaml` 是**受版本控制的模板**：所有凭据和地址为空值，
必须靠环境变量补齐。启动期会校验关键项：JWT 密钥、数据库地址/库名、
模型网关地址/对话模型不能为空，缺失即退出（fail fast）。

> 供应商 API Key 在数据库中以 AES-256-GCM 密文保存，加密材料由
> `NW_JWT_SECRET` 派生。**更换 JWT 密钥后需在「模型供应商」页面重新填写 API Key**，
> 否则旧密文无法解密，页面会提示配置错误。

### 2.2 环境变量总表

所有编排文件（根目录 Compose、Podman Compose）使用下列同名变量；
模板文件 `.env.example` 与 `deploy/podman/.env.example` 给出占位值。
部署时复制为 `.env` 并填入真实值，`.env` 已被 `.gitignore` 忽略。

| 变量 | 必填 | 说明 | 示例占位 |
| --- | --- | --- | --- |
| `NW_SERVER_PORT` | 否 | 后端监听端口 | `8080` |
| `NW_SERVER_MODE` | 否 | 运行模式（Podman 编排固定为 `release`） | `release` |
| `NW_JWT_SECRET` | 是 | JWT 签名密钥，同时作为供应商密钥封存材料 | `replace-with-a-long-random-secret` |
| `NW_POSTGRES_HOST` | 是 | 数据库主机名 | `postgres` |
| `NW_POSTGRES_PORT` | 否 | 数据库端口 | `5432` |
| `NW_POSTGRES_USER` | 否 | 数据库账号 | `postgres` |
| `NW_POSTGRES_PASSWORD` | 是 | 数据库口令 | `replace-with-db-password` |
| `NW_POSTGRES_DBNAME` | 否 | 数据库名 | `postgres` |
| `NW_REDIS_ADDR` | 是 | Redis 地址 `host:port` | `redis:6379` |
| `NW_REDIS_PASSWORD` | 否 | Redis 口令 | `replace-with-redis-password` |
| `NW_RABBITMQ_URL` | 是 | AMQP 连接串 | `amqp://user:pass@rabbitmq:5672/` |
| `NW_RABBITMQ_USER` | 本地 profile | RabbitMQ 账号（供中间件容器初始化） | `replace-with-rabbitmq-user` |
| `NW_RABBITMQ_PASSWORD` | 本地 profile | RabbitMQ 口令（供中间件容器初始化） | `replace-with-rabbitmq-password` |
| `NW_MINIO_ENDPOINT` | 是 | 对象存储地址 `host:port` | `minio:9000` |
| `NW_MINIO_ACCESS_KEY` | 是 | MinIO AccessKey | `replace-with-minio-access-key` |
| `NW_MINIO_SECRET_KEY` | 是 | MinIO SecretKey | `replace-with-minio-secret-key` |
| `NW_MANTICORE_BASE_URL` | 是 | 全文检索 HTTP 地址 | `http://manticore:9308` |
| `NW_AI_PROTOCOL_CODE` | 否 | 出厂默认供应商协议：`openai` / `anthropic` | `openai` |
| `NW_AI_BASE_URL` | 是 | 模型网关地址（OpenAI 兼容需含 `/v1`，Anthropic 填根地址） | `https://model-gateway.example.com/v1` |
| `NW_AI_API_KEY` | 是 | 模型网关密钥 | `replace-with-model-api-key` |
| `NW_AI_CHAT_MODEL` | 是 | 对话模型名 | `replace-with-chat-model` |
| `NW_AI_TEMPERATURE` | 否 | 采样温度 | `0.7` |
| `NW_AI_MAX_TOKENS` | 否 | 单次输出上限 | `2048` |
| `NW_AI_TIMEOUT_SEC` | 否 | 单次模型调用超时（秒） | `120` |
| `NW_AI_HISTORY_COUNT` | 否 | 历史注入默认条数 | `5` |
| `NW_APP_UPLOAD_MAX_MB` | 否 | 上传大小上限（MB） | `20` |
| `NW_APP_CHUNK_SIZE` | 否 | 知识库切片字符数 | `800` |
| `NW_APP_CHUNK_OVERLAP` | 否 | 切片重叠字符数 | `120` |
| `NW_APP_RAG_TOP_K` | 否 | RAG 默认召回条数 | `6` |
| `NW_APP_ADMIN_PASSWORD` | 是 | 引导管理员初始密码（仅用户表为空时生效，登录后改密） | `replace-with-a-strong-admin-password` |
| `NW_APP_BUILD_VERSION` | 否 | 运行状态页展示版本 | `dev` |
| `NW_INFRA_NETWORK` | Podman | 既有中间件栈网络名 | `novelworkbench_default` |
| `NW_NGINX_RESOLVER` | Podman | nginx DNS 地址 | `10.89.0.1` |
| `NW_API_UPSTREAM` | Podman | nginx 上游地址 | `http://backend.dns.podman:8080` |
| `NW_BASE_IMAGE_PREFIX` | Podman | 基础镜像仓库前缀 | 见 `deploy/podman/.env.example` |
| `NW_BACKEND_IMAGE` | Podman | 本地后端运行镜像名 | `localhost/novel-workbench-backend:latest` |
| `NW_FRONTEND_IMAGE` | Podman | 本地前端运行镜像名 | `localhost/novel-workbench-frontend:latest` |

AI 相关变量仅用于**首次播种默认供应商**：`model_providers` 表为空时写入一条，
之后在「模型供应商」页面维护多供应商、连通性测试与切换，配置文件不再参与。

### 2.3 启动时的初始化行为

后端启动顺序（`backend/cmd/server/main.go`）：

1. 加载配置并做启动期校验：JWT 密钥、数据库地址与库名、模型网关地址与对话模型
   为空即退出（fail fast）；协议默认 `openai`，构建版本默认 `dev`，
   历史条数默认 5，RAG 条数默认 6。
2. 连接 PostgreSQL，失败即退出；随后 `AutoMigrate` 建表（26 张表）。
3. 连接 Redis / RabbitMQ / MinIO / Manticore。**这四者的失败只降级告警，不阻断启动**：
   Redis 不可用则令牌吊销降级，RabbitMQ 不可用则异步索引任务降级，
   MinIO 桶不存在则对象存储降级，Manticore 不可用则全文检索降级。
4. 播种默认模型供应商（`model_providers` 表为空时，用 `NW_AI_*` 配置写入一条）。
5. 播种内置斜杠技能、默认应用配置、内置角色（admin/user）。这三项失败视为
   数据库异常，直接退出。
6. 用户表为空时引导创建管理员 `admin`（口令取 `NW_APP_ADMIN_PASSWORD`）。
7. 启动知识库索引 worker，消费 RabbitMQ 完成文档索引；随 ctx 取消优雅退出。

## 3. 本地 Docker Compose 部署

前置条件：Docker 与 Docker Compose 插件。

```bash
# 1. 准备环境变量
cp .env.example .env
# 编辑 .env：填入 NW_JWT_SECRET、数据库/缓存/队列/对象存储口令、模型网关信息等。
# 本地全栈需保持 NW_POSTGRES_HOST=postgres、NW_REDIS_ADDR=redis:6379、
# NW_MINIO_ENDPOINT=minio:9000、NW_RABBITMQ_URL=...rabbitmq...、
# NW_MANTICORE_BASE_URL=http://manticore:9308，与 infra profile 的服务名一致。

# 2a. 仅应用容器（中间件指向已有实例/云服务器）
docker compose up -d --build

# 2b. 本地全栈（额外拉起 PostgreSQL / Redis / RabbitMQ / MinIO / Manticore）
docker compose --profile infra up -d --build

# 3. 健康检查
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/ready
curl http://127.0.0.1/healthz

# 4. 查看日志 / 停止
docker compose logs -f backend
docker compose down            # 加 -v 会删除 infra 数据卷
```

说明：根目录 Compose 默认将后端 8080、前端 80 暴露到宿主机；`infra` profile 的
中间件容器使用 `.env` 中同一组凭据初始化，避免应用与中间件口令不一致。
本地 `infra` 使用普通 PostgreSQL 16 镜像，不启用任何数据库扩展；
检索能力由 Manticore 的全文索引提供。

## 4. 云主机 Podman 部署

目标主机运行 Podman + podman-compose，中间件栈已在宿主机运行并位于一个外部网络中
（例如网络名 `novelworkbench_default`，服务别名 `postgres/redis/manticore/minio/rabbitmq`）。
应用容器以 guest 加入该网络，通过服务名直连中间件，不发布中间件端口。

目录 `deploy/podman/` 包含：

| 文件 | 作用 |
| --- | --- |
| `docker-compose.yml` | Podman 编排，仅应用容器；凭据全部由 `.env` 注入，无可用默认值 |
| `Dockerfile.backend` | 运行镜像，仅打包预构建静态二进制 + 配置模板 + 宿主机 CA bundle |
| `Dockerfile.frontend` | 运行镜像，仅打包预构建 `dist` + nginx 模板 |
| `deploy.sh` | 发布脚本：校验产物 → 构建镜像 → 重建容器 → 健康自检 |
| `.env.example` | 环境变量占位模板 |

### 4.1 开发机准备产物

```bash
# 后端：交叉编译 linux/amd64 静态二进制（禁止 cgo）
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -tags "netgo osusergo" -ldflags="-s -w" \
  -o build/nw-server-linux-amd64 ./cmd/server

# 前端：构建静态资源
cd ../frontend
npm ci
npm run build

# 将整个仓库目录上传到云主机（例如 /opt/novelworkbench），包含上述产物
```

### 4.2 云主机发布

```bash
cd /opt/novelworkbench/deploy/podman
cp .env.example .env
vi .env                       # 填入密钥、模型网关信息；按实际网络名设置 NW_INFRA_NETWORK
bash deploy.sh
```

`deploy.sh` 依次执行：

1. 校验后端二进制、`frontend/dist`、`config.yaml`、nginx 模板和 `.env` 是否存在；
2. 组装构建上下文（二进制 / dist / 配置 / 宿主机 CA bundle）；
3. `podman build` 构建两个运行镜像（基础镜像前缀可由 `NW_BASE_IMAGE_PREFIX` 覆盖）；
4. 清理 rootless Podman 遗留的陈旧 DNAT 规则，`podman-compose up -d --force-recreate`
   重建容器（compose 输出只写服务器本地日志，避免口令进入终端/CI 日志）；
5. 轮询 `/health`、`/ready`、`/healthz`（30 次 × 2 秒），失败时打印容器状态与后端日志尾部。

Podman DNS 只应答 `search` 域 FQDN，前端 nginx 模板中的上游需写完整域名，
故 Podman 编排默认 `API_UPSTREAM=http://backend.dns.podman:8080`；
Docker 内嵌 DNS 应答裸服务名，根目录 Compose 使用 `http://backend:8080`。

## 5. CI/CD

| 工作流 | 触发 | 内容 |
| --- | --- | --- |
| `.github/workflows/ci.yml` | push / PR 到 `main` | 后端：`GOTOOLCHAIN=local` + `CGO_ENABLED=0` 下 `go vet`、`go build`、`go test -short -shuffle=on`；前端：Node 22 `npm ci` + `npm run test` + `npm run build`；随后验证两个 Docker 镜像可构建 |
| `.github/workflows/release.yml` | 推送 `v*` tag | 构建后端/前端镜像并推送到 GHCR：`ghcr.io/<repo>/backend:<tag>`、`ghcr.io/<repo>/frontend:<tag>`（同时更新 `latest`） |

镜像仓库名必须全小写（GHCR 硬性要求），而 GitHub 仓库名允许含大写字母。
发布工作流因此先用 `${GITHUB_REPOSITORY,,}` 把仓库名归一为小写再拼镜像前缀：
`2132691342/NovelWorkbench` → `ghcr.io/2132691342/novelworkbench/backend`。
归一在 `Normalize image prefix` 步骤完成，两个镜像构建步骤共用同一前缀输出。

前端锁文件在 `frontend/package-lock.json` 而非仓库根目录，因此 `setup-node` 的
`cache-dependency-path` 必须显式指向 `frontend/package-lock.json`；
`setup-node` 按仓库根解析该路径，缺省会直接报
`Dependencies lock file is not found` 并使整个 Frontend Check 失败。

工作流使用的 action 版本（均已切到 Node 24 运行时，避免 Node 20 弃用告警）：

| Action | 版本 |
| --- | --- |
| `actions/checkout` | v5 |
| `actions/setup-go` | v6 |
| `actions/setup-node` | v5 |
| `docker/setup-buildx-action` | v4 |
| `docker/login-action` | v4 |
| `docker/build-push-action` | v7 |

CI 不注入任何真实凭据：单元测试不依赖外部服务，镜像构建不使用运行期环境变量。

发布与更新镜像：

```bash
# 触发发布（tag 推送后 Release 工作流自动构建并推送 GHCR）
git tag v1.0.0 && git push origin v1.0.0

# 使用 GHCR 镜像的主机拉取并滚动更新
docker compose pull && docker compose up -d
docker compose ps
```

Podman 主机使用 `deploy.sh` 从预构建产物构建运行镜像（见第 4 节），
不依赖 GHCR 拉取。

## 6. 运维要点

| 事项 | 做法 |
| --- | --- |
| 健康检查 | 后端 `/health` 存活、`/ready` 依赖就绪；前端 `/healthz` 不依赖后端；nginx 上游不可用时 `/api` 返回 502 |
| 日志 | 后端 stdout JSON 日志，`request_id` 贯穿请求；用 `docker compose logs` / `podman logs` 查看 |
| 优雅关闭 | 收到 SIGTERM 后 HTTP Server 优雅退出，释放中间件连接；SSE 长连接随进程结束 |
| 配置变更 | 修改 `NW_*` 环境变量后重建容器（`docker compose up -d` 或 `deploy.sh`）；凭据不要写回 YAML |
| 密钥轮换 | 更新 `NW_JWT_SECRET` 后需在「模型供应商」页重新填写 API Key（见 2.1） |
| 备份 | 按中间件方案执行 PostgreSQL 备份与 MinIO 桶数据备份 |
