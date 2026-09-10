#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

PASS=0
FAIL=0

check() {
    local name="$1"
    local result="$2"
    if [ "$result" = "true" ]; then
        echo "[PASS] $name"
        PASS=$((PASS + 1))
    else
        echo "[FAIL] $name"
        FAIL=$((FAIL + 1))
    fi
}

echo "=========================================="
echo "  沙箱验证开始"
echo "=========================================="

echo ""
echo "--- SV-01: 启动3节点集群 ---"
docker-compose -f docker-compose.research.yml down -v 2>/dev/null || true
docker-compose -f docker-compose.research.yml up -d
sleep 5

LEADER_COUNT=0
for port in 9000 9001 9002; do
    state=$(curl -s "http://localhost:$port/raft/status" 2>/dev/null | grep -o '"state":"[^"]*"' | cut -d'"' -f4)
    if [ "$state" = "Leader" ]; then
        LEADER_COUNT=$((LEADER_COUNT + 1))
    fi
done
check "SV-01: Leader数量==1" "$([ $LEADER_COUNT -eq 1 ] && echo true || echo false)"

echo ""
echo "--- SV-02: 篡改state.bin并验证检测 ---"
docker exec research-node-3 sh -c 'echo "TAMPERED_BY_SV02" > /app/data/state.bin'
docker restart research-node-3 > /dev/null 2>&1
sleep 3

CRITICAL_LOG=$(docker logs research-node-3 2>&1 | grep -c "\[CRITICAL\] state.bin integrity check failed")
check "SV-02: 检测到篡改并输出[CRITICAL]" "$([ $CRITICAL_LOG -ge 1 ] && echo true || echo false)"

echo ""
echo "--- SV-03: 验证corrupted状态 ---"
CORRUPTED_LOG=$(docker logs research-node-3 2>&1 | grep -c "local-cache-corrupted\|检测到篡改")
check "SV-03: 进入corrupted状态" "$([ $CORRUPTED_LOG -ge 1 ] && echo true || echo false)"

echo ""
echo "--- SV-04: 验证重同步恢复 ---"
sleep 10
node3_state=$(curl -s "http://localhost:9002/raft/status" 2>/dev/null | grep -o '"state":"[^"]*"' | cut -d'"' -f4)
node3_caught=$(curl -s "http://localhost:9002/raft/status" 2>/dev/null | grep -o '"log_caught_up":[a-z]*' | cut -d':' -f2)
check "SV-04: 节点恢复为Follower" "$([ "$node3_state" = "Follower" ] && echo true || echo false)"

echo ""
echo "--- SV-05: 全程无脑裂验证 ---"
NO_SPLIT=true
for i in $(seq 1 5); do
    sleep 1
    lc=0
    for port in 9000 9001 9002; do
        state=$(curl -s "http://localhost:$port/raft/status" 2>/dev/null | grep -o '"state":"[^"]*"' | cut -d'"' -f4)
        if [ "$state" = "Leader" ]; then
            lc=$((lc + 1))
        fi
    done
    if [ $lc -ne 1 ]; then
        NO_SPLIT=false
    fi
done
check "SV-05: 全程Leader数量始终==1" "$NO_SPLIT"

echo ""
echo "--- SV-06: 验证使用研究镜像 ---"
IMAGE=$(docker inspect research-node-3 --format='{{.Config.Image}}' 2>/dev/null)
check "SV-06: 使用研究镜像" "$(echo "$IMAGE" | grep -q "state-protection-research" && echo true || echo false)"

echo ""
echo "=========================================="
echo "  沙箱验证结果: PASS=$PASS, FAIL=$FAIL"
echo "=========================================="

if [ $FAIL -eq 0 ]; then
    echo "所有验证通过!"
else
    echo "存在失败项，请检查日志"
fi

echo ""
echo "--- 节点状态汇总 ---"
for port in 9000 9001 9002; do
    echo "  端口 $port: $(curl -s "http://localhost:$port/raft/status" 2>/dev/null)"
done

echo ""
echo "--- 完整性指标 ---"
for port in 9000 9001 9002; do
    echo "  端口 $port /metrics:"
    curl -s "http://localhost:$port/metrics" 2>/dev/null | sed 's/^/    /'
done

docker-compose -f docker-compose.research.yml down -v 2>/dev/null || true

exit $FAIL