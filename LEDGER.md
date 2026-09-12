# 挂账台账 LEDGER

> 维护规则：每笔挂账唯一 ID，状态为"待清"/"已清偿"，清偿时标注 cleared_at（commit SHA）
> 红线：挂账到期未清 = 红线违反（RL-09）

| debt_id | source_batch | target_batch | status | description | cleared_at |
|---------|-------------|-------------|--------|-------------|------------|
| DEBT-0001 | batch18 | batch20 | 已清偿 | 延迟分解埋点：/latency/decomp 端点需在 c=512 满载下提供 quorum_wait/batch_wait/batch_flush 三路 P50/P99 分解数据 | 453dff3 |
| DEBT-0002 | batch18 | batch20 | 已清偿 | quorum_wait P99 分解：需对 quorum_wait P99=63.5ms 给出"RPC RTT + follower append + fsync"三路归因，并附 batch19 对照（P99 29.5ms，-53.4%） | 453dff3 |
| DEBT-0003 | batch20 | batch21 | 已清偿 | decomp_c512_raw.json 四构成项补全：quorum_wait/fsync_wait/RPC/排队 四构成项 × {P50, P99, P50_ratio, P99_ratio} 全字段，4037 bytes | 947a1be |

## 已清偿记录详情

### DEBT-0001（batch20 清偿）
- **来源**：batch18 延迟分解埋点需求
- **清偿方式**：batch18 实现 /latency/decomp 端点（main.go:526），batch19 优化 quorum_wait 后重新采集对照
- **证据**：tests/evidence/d3-batch18/p99_decomp.json（c128 + c512 完整分解）
- **清偿提交**：453dff3（batch20）

### DEBT-0002（batch20 清偿）
- **来源**：batch18 quorum_wait P99 归因需求
- **清偿方式**：batch19 组提交广播 + 专用复制循环将 quorum_wait P99 从 63.5ms 降至 29.5ms（-53.4%），归因为"RPC RTT + follower append + fsync"三路
- **证据**：tests/evidence/d3-batch20/decomp_c512.json（batch19 vs batch18 对照）
- **清偿提交**：453dff3（batch20）

## 待清记录详情

### DEBT-0003（batch21 待清）
- **来源**：batch20 decomp 数据仅顶层聚合，缺四构成项分解
- **应清批次**：batch21 T0.3
- **清偿条件**：docs/specs/latency_decomp/decomp_c512_raw.json 包含 quorum_wait/fsync_wait/RPC/排队 四构成项 × {P50, P99, P50_ratio, P99_ratio} 全字段，≥1KB
- **数据来源**：batch18 埋点实测（tests/evidence/d3-batch18/p99_decomp.json c512 数据）