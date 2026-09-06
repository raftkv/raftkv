#!/bin/bash
# deploy_up.sh — 一键拉起岱境235集群
# 修正A: SM4_KEY双模式（留空=生成复用, 显式=用指定值）
set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${DEPLOY_DIR}/deploy.env"
KEY_FILE="${DEPLOY_DIR}/.sm4_key"
COMPOSE_FILE="${DEPLOY_DIR}/docker-compose.yml"

if [ ! -f "$ENV_FILE" ]; then
    echo "[deploy] FAIL: deploy.env 不存在，请从 deploy.env.example 复制并填入真实值"
    exit 1
fi

source "$ENV_FILE"

IMAGE_NAME="${IMAGE_NAME:-daijin235-v26:ci-knife}"
FP_ANCHOR="${FP_ANCHOR:-tcx4-v25-test}"
GRPC_PORT="${GRPC_PORT:-9500}"
HTTP_PORT="${HTTP_PORT:-9000}"

if [ -z "${LICENSE_DIR:-}" ] || [ "${LICENSE_DIR}" = "__SET_YOUR_LICENSE_DIR__" ]; then
    echo "[deploy] FAIL: LICENSE_DIR 未设置，请编辑 deploy.env"
    exit 1
fi

if [ -z "${SM4_KEY:-}" ]; then
    if [ -f "$KEY_FILE" ]; then
        SM4_KEY=$(cat "$KEY_FILE")
        echo "[deploy] SM4_KEY: 从本地密钥文件复用"
    else
        SM4_KEY=$(openssl rand -hex 16)
        echo "$SM4_KEY" > "$KEY_FILE"
        chmod 600 "$KEY_FILE" 2>/dev/null || true
        echo "[deploy] SM4_KEY: 首次生成并写入本地密钥文件"
    fi
else
    echo "[deploy] SM4_KEY: 使用deploy.env指定值"
fi

export IMAGE_NAME FP_ANCHOR GRPC_PORT HTTP_PORT SM4_KEY LICENSE_DIR

echo "[deploy] 配置:"
echo "  IMAGE_NAME=$IMAGE_NAME"
echo "  FP_ANCHOR=$FP_ANCHOR"
echo "  GRPC_PORT=$GRPC_PORT"
echo "  HTTP_PORT=$HTTP_PORT"
echo "  LICENSE_DIR=$LICENSE_DIR"
echo "  SM4_KEY=${SM4_KEY:0:8}...(隐藏)"

echo ""
echo "[deploy] docker compose up -d..."
cd "$DEPLOY_DIR"
docker compose up -d 2>&1

echo ""
echo "[deploy] 等待Leader出现..."
for i in $(seq 1 30); do
    sleep 1
    s1=$(docker exec daijin235-node-1 curl -s http://127.0.0.1:9000/raft/stats 2>/dev/null || true)
    if echo "$s1" | grep -q "state=Leader"; then
        echo "[deploy] Leader=daijin235-node-1 after ${i}s"
        echo "[deploy] stats: $s1"
        echo "[deploy] PASS: 集群已就绪"
        exit 0
    fi
    s2=$(docker exec daijin235-node-2 curl -s http://127.0.0.1:9000/raft/stats 2>/dev/null || true)
    if echo "$s2" | grep -q "state=Leader"; then
        echo "[deploy] Leader=daijin235-node-2 after ${i}s"
        echo "[deploy] stats: $s2"
        echo "[deploy] PASS: 集群已就绪"
        exit 0
    fi
done

echo "[deploy] FAIL: Leader未在30s内当选"
exit 1