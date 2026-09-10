# E05 重跑 PASS 报告 — R-04修复后

## 测试时间
2026-09-08 01:24–01:27 (UTC+8)

## 测试环境
- 镜像: daijin235-v26:ci-knife (含R-04修复A/B/C/D)
- 集群: 5节点 (node-1..5), Leader=node-4→node-2
- 故障注入: docker network disconnect/connect node-1

## 故障期压测结果

| 指标 | 实测 | PASS线 | 判定 |
|------|------|--------|------|
| TPS | 1,148 req/s | ≥905 | PASS |
| 成功率 | 100.00% | 100% | PASS |
| P99 | 503.4ms | — | — |
| P50 | 920.1µs | — | — |
| Max | 559.9ms | <5s (R-03) | PASS |
| 总请求 | 68,901 (写3,445 读65,456) | — | — |

## 收敛结果

| 指标 | 实测 | PASS线 | 判定 |
|------|------|--------|------|
| 收敛时间 | <5s | ≤30s | PASS |
| node-1 commit | 7,918 | =leader commit | PASS |
| 全集群一致 | 5/5节点 commit=7,918 | 全部一致 | PASS |
| degraded | [] (无降级) | 空 | PASS |

## R-03状态
Max=559.9ms < 5s → R-03未触发（符合预期）

## 修复验证项
- 修复A (gRPC DNS重连): reconnect带alias后DNS解析成功，gap迅速→0
- 修复B (degraded标记): 故障期degraded=[node-1]正确标记，恢复后清除
- 修复C (gap告警): /raft/stats正确输出gaps和degraded字段
- 修复D (compose别名): docker-compose-5node.yml显式aliases，reconnect带--alias

## 结论
**E05重跑 PASS** — 故障期TPS≥905 + 收敛<30s + 100%成功率