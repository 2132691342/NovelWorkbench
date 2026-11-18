# 部署文档

## 1. 部署形态

前后端分离、容器化交付：后端 Go 静态二进制镜像 + 前端 nginx 镜像；中间件使用云服务器既有实例（PostgreSQL 16 + pgvector、Redis 8.4、RabbitMQ、MinIO、Manticore）。

| 服务 | 镜像 | 端口 | 说明 |
| --- | --- | --- | --- |
| backend | novel-workbench-backend | 8080 | API + SSE；`/health` 健康探针 |
| frontend | novel-workbench-frontend | 80 | SPA + `/api` 反向代理后端 |

## 2. 镜像构建

后端（多阶段，无 cgo）：

```bash
docker build -t novel-workbench-backend:latest ./backend
```

要点：`CGO_ENABLED=0` + `-tags "netgo osusergo"` 静态编译；构建层固定 `GOTOOLCHAIN=local`，`go.mod` 声明 `go 1.24`，依赖下界必须全部 ≤ go1.24（gin v1.11 / gin-contrib-cors v1.7.6 / pgx v5.5 / minio-go v7.0.9 / go-redis v9.7 / x/crypto v0.40），勿升回 1.25+ 版本，否则镜像构建期报 `go.mod requires go >= 1.25`。运行层 alpine + CA 证书（访问 HTTPS 模型网关）+ 非 root 用户 `appuser`。

基础镜像可用构建参数覆盖（适配国内镜像仓库）：

```bash
docker build --build-arg BASE_BUILDER_IMAGE=swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/golang:1.24-alpine \
           --build-arg BASE_RUNTIME_IMAGE=swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/alpine:3.20 \
           -t novel-workbench-backend:latest ./backend
```

前端：

```bash
docker build -t novel-workbench-frontend:latest ./frontend
```

要点：node 构建层产出 `dist`，运行层 nginx 托管并内置反代配置（`frontend/nginx.conf.template`，容器启动时按 `NGINX_RESOLVER`/`API_UPSTREAM` 渲染）：SPA 回退 `try_files ... /index.html`、`/healthz` 探针、`/api` 按请求 DNS 解析代理到 `backend:8080`（容器重建不残留旧 IP）、SSE 场景 `proxy_buffering off`。

## 3. docker-compose 编排

```bash
# 应用（中间件指向云服务器，配置走环境变量，见 docker-compose.yml）
docker compose up -d --build

# 本地全栈（拉起同版本中间件容器）
docker compose --profile infra up -d --build
```

环境变量（`NW_` 前缀，覆盖 `configs/config.yaml`）：

| 变量 | 说明 |
| --- | --- |
| NW_JWT_SECRET | JWT 签名密钥（生产必改） |
| NW_POSTGRES_HOST / NW_POSTGRES_PASSWORD | 数据库地址/密码 |
| NW_REDIS_ADDR / NW_REDIS_PASSWORD | 缓存地址/密码 |
| NW_RABBITMQ_URL | amqp 连接串 |
| NW_MINIO_ENDPOINT / NW_MINIO_ACCESS_KEY / NW_MINIO_SECRET_KEY | 对象存储 |
| NW_MANTICORE_BASE_URL | 全文检索 HTTP 地址 |
| NW_AI_PROTOCOL_CODE | 出厂默认供应商协议：openai / anthropic |
| NW_AI_BASE_URL / NW_AI_API_KEY / NW_AI_CHAT_MODEL / NW_AI_EMBEDDING_MODEL | 出厂默认供应商（表为空时播种一次，之后在页面管理） |
| NW_APP_ADMIN_PASSWORD | 引导管理员初始密码（仅用户表为空时生效，首登后改密） |
| NW_APP_BUILD_VERSION | 运行状态页展示的构建版本（发布时注入 tag 或 commit） |
| NW_SERVER_PORT | 监听端口 |

> 供应商密钥在库中为 AES-256-GCM 密文，封存材料由 `NW_JWT_SECRET` 派生：**更换 JWT 密钥后需在「模型供应商」页重新填写 API Key**（否则解密失败并提示）。

启动顺序：后端启动时 AutoMigrate 建表（含 `CREATE EXTENSION IF NOT EXISTS vector`），播种内置斜杠技能/默认配置/内置角色（admin/user）/默认模型供应商（表为空时取 `ai.*` 配置），用户表为空时引导创建管理员（`admin` + `NW_APP_ADMIN_PASSWORD`）；并确保 MinIO 桶存在、Manticore 切片表存在。Manticore 表结构含 CJK ngram（`ngram_len=1` + `ngram_chars=cjk`）；检测到旧结构时自动重建并从 PG 切片回灌全文索引。中间件不可达时 PG 失败即退出（fail fast），其余中间件降级为告警日志（上传/检索等功能返回 `ERR_STORAGE`，主链路不受影响；模型供应商未配置时 AI 生成返回 `ERR_AI_PROVIDER` 并在页面提示）。

## 4. podman 部署（云主机）

云主机运行时为 Podman 3.x + podman-compose（无 Docker），中间件栈已在宿主机运行（compose 项目名 `liteworkbench`，网络 `liteworkbench_default`，服务别名 `postgres/redis/manticore/minio/rabbitmq`）。应用容器以 guest 加入该网络服务名直连中间件，不发布中间件端口。

目录：`deploy/podman/`——`docker-compose.yml`（podman 编排，凭据全走 `.env`）、`Dockerfile.backend`/`Dockerfile.frontend`（运行镜像，仅打包预构建产物）、`deploy.sh`（发布脚本）、`.env.example`（环境变量模板）。

开发机准备产物并上传：

```bash
# 后端：交叉编译 linux/amd64 静态二进制（无 cgo）
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "netgo osusergo" \
  -ldflags="-s -w" -o build/nw-server-linux-amd64 ./cmd/server

# 前端：构建 dist
cd ../frontend && npm run build

# 上传仓库目录到云主机（例如 /opt/novelworkbench）
```

云主机发布：

```bash
cd /opt/novelworkbench/deploy/podman
cp .env.example .env && vi .env        # 填好 NW_JWT_SECRET 等密钥
bash deploy.sh
```

`deploy.sh` 做四件事：校验产物与 `.env` → 组装构建上下文（二进制/dist/配置/宿主机 CA bundle）→ `podman build` 运行镜像 + `podman-compose up -d`（输出只落服务器本地日志，避免口令进 CI 日志）→ `/health` + `/ready` + `/healthz` 健康自检（30×2s，失败回显容器现场与后端日志尾部）。rootless podman 重建容器遗留的陈旧 CNI-HOSTPORT-DNAT 规则会在脚本内清理，避免宿主端口不可达。

基础镜像默认走华为云镜像仓库前缀（`NW_BASE_IMAGE_PREFIX` 可覆盖），适配国内主机拉取。

网络要点：nginx 对 `/api` 采用变量 `proxy_pass` + `resolver` 按请求重解析上游，容器重建不残留旧 IP。podman 的容器 DNS 只应答 `search` 域 FQDN（`backend.dns.podman`），故 podman 编排默认 `NW_API_UPSTREAM=http://backend.dns.podman:8080`；docker 内嵌 DNS 应答裸服务名，docker 编排用 `http://backend:8080`。

## 5. CI/CD

| 工作流 | 触发 | 内容 |
| --- | --- | --- |
| .github/workflows/ci.yml | push/PR 到 main | 后端 `GOTOOLCHAIN=local` + `go vet` + `go build`(CGO_ENABLED=0) + `go test -short`；前端 `npm ci` + `vue-tsc` + `vite build`；两端镜像构建验证（与 `go.mod` 的 1.24.1 工具链一致，禁止构建期自动升级） |
| .github/workflows/release.yml | 推送 `v*` tag | 构建前后端镜像并推送 GHCR（`ghcr.io/<repo>/backend|frontend:<tag>`） |

发布步骤：

```bash
git tag -f v1.0.0 && git push -f origin v1.0.0   # 触发 Release
```

服务器拉取发布：

```bash
docker compose pull && docker compose up -d
curl http://127.0.0.1:8080/health                 # {"resp_code":"SUCCESS",...}
```

## 6. 运维要点

| 事项 | 做法 |
| --- | --- |
| 健康检查 | compose healthcheck 每 30s 打 `/health`；`/ready` 额外 Ping DB（就绪探针）；nginx 挂掉后端时返回 502 |
| 日志 | 后端 stdout JSON（zap），`request_id` 贯穿请求；docker/podman logs 收敛 |
| 优雅关闭 | SIGTERM → HTTP Shutdown(5s) + 中间件连接关闭，SSE 连接随进程结束 |
| 备份 | PG `pg_dump` 定时 + MinIO 桶版本化/镜像 |
| 扩容 | 无状态后端可横向扩；SSE 会话粘性由单实例起步，多实例前建议先上负载均衡会话保持 |
