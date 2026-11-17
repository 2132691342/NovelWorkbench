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

要点：`CGO_ENABLED=0` + `-tags "netgo osusergo"` 静态编译；运行层 alpine + CA 证书（访问 HTTPS 模型网关）+ 非 root 用户 `appuser`。

前端：

```bash
docker build -t novel-workbench-frontend:latest ./frontend
```

要点：node 构建层产出 `dist`，运行层 nginx 托管并内置反代配置（`deploy/nginx.conf`）：SPA 回退 `try_files ... /index.html`、`/api` 代理到 `backend:8080`、SSE 场景 `proxy_buffering off`。

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
| NW_AI_BASE_URL / NW_AI_API_KEY / NW_AI_CHAT_MODEL / NW_AI_EMBEDDING_MODEL | OpenAI 兼容网关 |
| NW_SERVER_PORT | 监听端口 |

启动顺序：后端启动时 AutoMigrate 建表（含 `CREATE EXTENSION IF NOT EXISTS vector`），并确保 MinIO 桶存在、Manticore 切片表存在；中间件不可达时 PG 失败即退出（fail fast），其余中间件降级为告警日志（上传/检索等功能返回 `ERR_STORAGE`，主链路不受影响）。

## 4. CI/CD

| 工作流 | 触发 | 内容 |
| --- | --- | --- |
| .github/workflows/ci.yml | push/PR 到 main | 后端 `go vet` + `go build`(CGO_ENABLED=0) + `go test -short`；前端 `npm ci` + `vue-tsc` + `vite build`；两端镜像构建验证 |
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

## 5. 运维要点

| 事项 | 做法 |
| --- | --- |
| 健康检查 | compose healthcheck 每 30s 打 `/health`；nginx 挂掉后端时返回 502 |
| 日志 | 后端 stdout JSON（log/slog），`request_id` 贯穿请求；docker logs 收敛 |
| 优雅关闭 | SIGTERM → HTTP Shutdown(5s) + 中间件连接关闭，SSE 连接随进程结束 |
| 备份 | PG `pg_dump` 定时 + MinIO 桶版本化/镜像 |
| 扩容 | 无状态后端可横向扩；SSE 会话粘性由单实例起步，多实例前建议先上负载均衡会话保持 |
