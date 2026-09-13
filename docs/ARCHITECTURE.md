# 架构 — ARCHITECTURE

> 摘编自既有 evidence，禁止新编造

## 组件

| 组件 | 文件 | 职责 |
|------|------|------|
| Raft 状态机 | raft.go | 选举/日志复制/提交 |
| HTTP 端点 | main.go | /raft/stats, /raft/entry, /raft/election_metrics 等 |
| 类型定义 | types.go | RaftStats, RaftLog 等 |
| 故障注入 | cmd/chaos_injector/ | chaos_injector 工具 |
| 判定脚本 | tests/contracts/judge_batch23.py | verdict 判定（引用 regression.yaml 线 ID） |
| 回归门 | tests/contracts/regression_gate.py | 跨批回归检查 |

## Raft 配置

- 选举超时: 800-1200ms (batch22 优化)
- 心跳间隔: 50ms
- pre-vote: 启用 (batch23)
- pipeline: 深度 8, batch=64, window=2ms

## 可观测性端点

| 端点 | 用途 | 来源批次 |
|------|------|---------|
| /raft/stats | 节点状态 | batch21 |
| /raft/entry | 日志采样 | batch22 |
| /raft/pre_vote | pre-vote 状态 | batch23 |
| /latency/decomp | 延迟分解 | batch18 |
| /raft/election_metrics | 选举/心跳计数 | batch26 (batch27 接入递增) |