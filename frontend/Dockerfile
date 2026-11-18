# 前端构建镜像：多阶段，node 构建 + nginx 托管
FROM node:22-slim AS builder
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
# nginx.conf.template 由 docker-entrypoint 的 envsubst 渲染；变量缺省见 ENV
ENV NGINX_RESOLVER=127.0.0.11 \
    API_UPSTREAM=http://backend:8080
COPY --from=builder /src/dist /usr/share/nginx/html
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
EXPOSE 80
