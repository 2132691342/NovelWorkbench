# 前端构建镜像：pnpm workspace（apps 已扁平化到仓库根）构建 + nginx 托管
FROM node:22-slim AS builder
WORKDIR /src

# pnpm 由 corepack 提供，版本与本地一致，避免锁文件漂移
RUN corepack enable && corepack prepare pnpm@10.34.9 --activate

# 先装依赖，最大化层缓存命中
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY packages ./packages
RUN pnpm install --frozen-lockfile --prod=false

# 再拷源码构建（packages 为 workspace 依赖，必须在 install 前就位）
COPY . .
RUN pnpm build

FROM nginx:1.27-alpine
ENV NGINX_RESOLVER=127.0.0.11 \
    API_UPSTREAM=http://backend:8080 \
    NW_LISTEN_PORT=80 \
    NW_CLIENT_MAX_BODY_SIZE=20m
COPY --from=builder /src/dist /usr/share/nginx/html
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
EXPOSE 80
