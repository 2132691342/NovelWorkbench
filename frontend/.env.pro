# 生产模式：关闭 sourcemap，输出 dist（Dockerfile 与 deploy.sh 都取这个目录）
VITE_BASE_PATH=/
VITE_APP_TITLE=NovelWorkbench
VITE_API_BASE_PATH=/api/v1
VITE_SOURCEMAP=false
VITE_OUT_DIR=dist
VITE_USE_CSS_SPLIT=true
