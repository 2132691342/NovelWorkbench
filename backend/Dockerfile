# 后端构建镜像：多阶段构建，CGO_ENABLED=0 静态编译
FROM golang:1.24-alpine AS builder
WORKDIR /src

# 先拷贝依赖清单利用构建缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags "netgo osusergo" -ldflags="-s -w" -o /out/novel-workbench-server ./cmd/server

# 运行镜像：alpine + CA 证书（访问 HTTPS 模型网关需要）
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/novel-workbench-server /app/novel-workbench-server
COPY configs /app/configs
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/novel-workbench-server"]
