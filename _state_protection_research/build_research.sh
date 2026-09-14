#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "=========================================="
echo "  RaftKV State Protection Research Build"
echo "=========================================="

echo "[1/4] 下载依赖..."
go mod download

echo "[2/4] 交叉编译 amd64..."
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X 'main.Version=v2.4-research' -X 'main.BuildTime=2026-08-31'" \
    -o dist/gateway_research_amd64 ./cmd/state-protection-research/

echo "[3/4] 交叉编译 arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags="-s -w -X 'main.Version=v2.4-research' -X 'main.BuildTime=2026-08-31'" \
    -o dist/gateway_research_arm64 ./cmd/state-protection-research/

echo "[4/4] 构建 Docker 镜像..."
docker build -t raftkv-state-protection-research:v2.4-research \
    -f Dockerfile.research .

echo "=========================================="
echo "  构建完成"
echo "  产物:"
echo "    - dist/gateway_research_amd64"
echo "    - dist/gateway_research_arm64"
echo "    - Docker镜像: raftkv-state-protection-research:v2.4-research"
echo "=========================================="