# 后端构建镜像：多阶段构建，CGO_ENABLED=0 静态编译
# 两个基础镜像参数必须声明在首个 FROM 之前，才能被后续 FROM 使用
ARG BASE_BUILDER_IMAGE=golang:1.24-alpine
ARG BASE_RUNTIME_IMAGE=alpine:3.20
FROM ${BASE_BUILDER_IMAGE} AS builder
WORKDIR /src

# 固定本地工具链，禁止构建期拉取其它 Go 版本
ENV GOTOOLCHAIN=local \
    CGO_ENABLED=0

# 先拷贝依赖清单利用构建缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOOS=linux GOARCH=amd64 go build -tags "netgo osusergo" -ldflags="-s -w" -o /out/novel-workbench-server ./cmd/server

# 运行镜像：alpine + CA 证书（访问 HTTPS 模型网关需要）
FROM ${BASE_RUNTIME_IMAGE}
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=builder /out/novel-workbench-server /app/novel-workbench-server
COPY configs /app/configs
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/novel-workbench-server"]
