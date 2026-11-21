#!/usr/bin/env bash
# NovelWorkbench 云主机发布（podman + podman-compose）：组装构建上下文 → 构建运行镜像 → 重建容器 → 健康自检。
# 用法：在本目录执行 bash deploy.sh；依赖 ../../ 下的预构建产物（见 doc/deploy.md）。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"
REPO_ROOT="$(cd ../.. && pwd)"
BACKEND_BIN="$REPO_ROOT/backend/build/nw-server-linux-amd64"
FRONTEND_DIST="$REPO_ROOT/frontend/dist"
NGINX_CONF="$REPO_ROOT/frontend/nginx.conf.template"
BACKEND_CONF="$REPO_ROOT/backend/configs/config.yaml"
ENV_FILE="${ENV_FILE:-.env}"
# 基础镜像默认走国内镜像仓库（可被 .env 的 NW_BASE_IMAGE_PREFIX 覆盖）
BASE_PREFIX="${NW_BASE_IMAGE_PREFIX:-swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library}"
READY_TRIES=30
READY_INTERVAL=2

log() { echo "[deploy] $*"; }
die() { echo "[deploy] $*" >&2; exit 1; }

# ---- 预检：产物与密钥 ----
[ -f "$BACKEND_BIN" ] || die "缺少后端二进制 $BACKEND_BIN（先在开发机交叉编译并上传）"
[ -f "$FRONTEND_DIST/index.html" ] || die "缺少前端产物 $FRONTEND_DIST（先在 frontend 执行 pnpm build 并上传）"
[ -f "$NGINX_CONF" ] || die "缺少 $NGINX_CONF"
[ -f "$BACKEND_CONF" ] || die "缺少 $BACKEND_CONF"
[ -f "$ENV_FILE" ] || die "缺少 $ENV_FILE（先 cp .env.example .env 并填好密钥）"
set -a
# shellcheck disable=SC1090
. "./$ENV_FILE"
set +a

# ---- rootless podman 重建容器后遗留的陈旧 DNAT 规则会导致宿主端口不可达，先行清理 ----
drop_stale_dnat() {
  command -v iptables >/dev/null 2>&1 || return 0
  while iptables -t nat -C CNI-HOSTPORT-DNAT -d 127.0.0.1/32 -j DNAT --to-destination 127.0.0.1 2>/dev/null; do
    iptables -t nat -D CNI-HOSTPORT-DNAT -d 127.0.0.1/32 -j DNAT --to-destination 127.0.0.1 2>/dev/null || break
  done
}

# cleanup_port_dnat 删除某宿主端口的全部 CNI DNAT 跳转规则（容器重建前调用）；
# 容器启动时 podman 会为新容器重新写入唯一规则，避免旧容器 IP 抢占端口。
cleanup_port_dnat() {
  local port="$1"
  command -v iptables >/dev/null 2>&1 || return 0
  local chain_list chain line_no
  chain_list="$(iptables -t nat -S CNI-HOSTPORT-DNAT 2>/dev/null | grep -- "--dports ${port} " \
    | sed -n 's/.*-j \(CNI-DN-[^ ]*\).*/\1/p' | sort -u || true)"
  for chain in $chain_list; do
    while line_no="$(iptables -t nat -L CNI-HOSTPORT-DNAT --line-numbers -n 2>/dev/null \
      | awk -v chain="$chain" '$2 == chain {print $1; exit}')"; [ -n "$line_no" ]; do
      iptables -t nat -D CNI-HOSTPORT-DNAT "$line_no" || break
    done
  done
}

# ---- 组装构建上下文 ----
BUILD_DIR="$SCRIPT_DIR/build"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/backend" "$BUILD_DIR/frontend"
cp "$BACKEND_BIN" "$BUILD_DIR/backend/nw-server"
chmod +x "$BUILD_DIR/backend/nw-server"
cp "$BACKEND_CONF" "$BUILD_DIR/backend/config.yaml"
cp /etc/ssl/certs/ca-certificates.crt "$BUILD_DIR/backend/ca-certificates.crt" \
  || die "缺少宿主机 CA bundle（/etc/ssl/certs/ca-certificates.crt）"
cp -r "$FRONTEND_DIST" "$BUILD_DIR/frontend/dist"
cp "$NGINX_CONF" "$BUILD_DIR/frontend/nginx.conf.template"

# ---- 构建运行镜像 ----
BACKEND_IMAGE="${NW_BACKEND_IMAGE:-localhost/novel-workbench-backend:latest}"
FRONTEND_IMAGE="${NW_FRONTEND_IMAGE:-localhost/novel-workbench-frontend:latest}"
log "构建后端镜像 $BACKEND_IMAGE"
podman build -f "$SCRIPT_DIR/Dockerfile.backend" \
  --build-arg "BASE_IMAGE=${BASE_PREFIX}/alpine:3.20" \
  -t "$BACKEND_IMAGE" "$BUILD_DIR/backend"
log "构建前端镜像 $FRONTEND_IMAGE"
podman build -f "$SCRIPT_DIR/Dockerfile.frontend" \
  --build-arg "BASE_IMAGE=${BASE_PREFIX}/nginx:1.27-alpine" \
  -t "$FRONTEND_IMAGE" "$BUILD_DIR/frontend"

# ---- 重建容器 ----
drop_stale_dnat
cleanup_port_dnat "${NW_PUBLIC_PORT:-14399}"
cleanup_port_dnat 8080
export NW_BACKEND_IMAGE="$BACKEND_IMAGE" NW_FRONTEND_IMAGE="$FRONTEND_IMAGE"
COMPOSE_LOG="${COMPOSE_LOG:-/var/log/novelworkbench-compose.log}"
mkdir -p "$(dirname "$COMPOSE_LOG")"
# podman-compose 会把含明文口令的 podman run 命令打到 stdout，输出只落服务器本地日志
# podman-compose 1.x 不会因镜像 ID 变化自动重建容器，必须显式 --force-recreate
if ! podman-compose -f "$SCRIPT_DIR/docker-compose.yml" up -d --force-recreate >"$COMPOSE_LOG" 2>&1; then
  tail -40 "$COMPOSE_LOG"
  die "podman-compose up 失败"
fi
drop_stale_dnat

# ---- 健康自检：/health（存活）+ /ready（依赖就绪）+ 首页 ----
PUBLIC_PORT="${NW_PUBLIC_PORT:-14399}"
for i in $(seq 1 "$READY_TRIES"); do
  # 后端不映射宿主端口：直接进容器探活，避免 rootless podman 的陈旧 DNAT 干扰。
  health="$(podman exec nw-backend wget -qO- http://127.0.0.1:8080/health 2>/dev/null || true)"
  ready="$(podman exec nw-backend wget -qO- http://127.0.0.1:8080/ready 2>/dev/null || true)"
  index="$(curl -fsS -m 3 "http://127.0.0.1:${PUBLIC_PORT}/healthz" 2>/dev/null || true)"
  if [ -n "$health" ] && [ -n "$ready" ] && [ "$index" = "ok" ]; then
    log "部署完成：/health /ready /healthz 全部通过"
    podman ps --format '{{.Names}} | {{.Status}} | {{.Ports}}'
    exit 0
  fi
  sleep "$READY_INTERVAL"
done
log "健康自检未通过，容器现场："
podman ps --format '{{.Names}} | {{.Status}} | {{.Ports}}' || true
podman logs --tail 60 nw-backend 2>&1 | grep -av 'password' || true
die "部署失败，请检查上方日志"
