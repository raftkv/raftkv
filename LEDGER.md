# 挂账台账 LEDGER

> 维护规则：每笔挂账唯一 ID，状态为"待清"/"已清偿"，清偿时标注 cleared_at（commit SHA）
> 红线：挂账到期未清 = 红线违反（RL-09）

| debt_id | source_batch | target_batch | status | description | cleared_at |
|---------|-------------|-------------|--------|-------------|------------|
| DEBT-0001 | batch18 | batch20 | 已清偿 | 延迟分解埋点：/latency/decomp 端点需在 c=512 满载下提供 quorum_wait/batch_wait/batch_flush 三路 P50/P99 分解数据 | 453dff3 |
| DEBT-0002 | batch18 | batch20 | 已清偿 | quorum_wait P99 分解：需对 quorum_wait P99=63.5ms 给出"RPC RTT + follower append + fsync"三路归因，并附 batch19 对照（P99 29.5ms，-53.4%） | 453dff3 |
| DEBT-0003 | batch20 | batch21 | 已清偿 | decomp_c512_raw.json 四构成项补全：quorum_wait/fsync_wait/RPC/排队 四构成项 × {P50, P99, P50_ratio, P99_ratio} 全字段，4037 bytes | 947a1be |
| L-21-1 | batch21 | batch22 | 已清偿 | F1 选举 6.895s 归因取证：需采集当前选举超时配置/随机区间/轮数/选票分布，并优化至 ≤2s | batch22 |
| L-21-2 | batch21 | batch22 | 已清偿 | 观测端点缺失致 F3 无法验证：需实现 GET /raft/entry 端点，使 F3 存活率可采集 | batch22 |
| L-22-1 | batch22 | batch23 | 待清 | W-1 pre-vote 未实现：batch22 选举优化绕行项，cascading_kill_02 选举 3.44s 为选票分裂导致多轮选举，需实现 pre-vote 防选票分裂 | — |
| L-22-2 | batch22 | batch23 | 待清 | D-8 磁盘满方案仅设计不实施：batch22 任务三预演完成 disk_full_spec.md + disk_full_design.md 两件套，实施 deferred 到 batch23 | — |

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

### DEBT-0003（batch21 清偿）
- **来源**：batch20 decomp 数据仅顶层聚合，缺四构成项分解
- **应清批次**：batch21 T0.3
- **清偿条件**：docs/specs/latency_decomp/decomp_c512_raw.json 包含 quorum_wait/fsync_wait/RPC/排队 四构成项 × {P50, P99, P50_ratio, P99_ratio} 全字段，≥1KB
- **数据来源**：batch18 埋点实测（tests/evidence/d3-batch18/p99_decomp.json c512 数据）
- **清偿提交**：947a1be（batch21）

## 待清记录详情

### L-21-1（batch22 已清偿）
- **来源**：batch21 F1 选举完成时间 6.895s > 5.0s 阈值
- **清偿批次**：batch22
- **清偿方式**：选举超时 5000-7000ms→800-1200ms + 心跳阈值 5s→2s + 回退 *3→*2
- **验证结果**：E1 3-median=1.6003s ≤ 2.0s PASS
- **清偿提交**：batch22

### L-21-2（batch22 已清偿）
- **来源**：batch21 F3 存活率 0%，/raft/entry 端点未实现
- **清偿批次**：batch22
- **清偿方式**：新增 /raft/entry HTTP handler + 修复 RaftStats JSON 标签 + 修复 chaos_injector 构建路径
- **验证结果**：S1 survival=100% PASS, S2 sampled=20 PASS
- **清偿提交**：batch22
### L-22-1（batch23 待清）
- **来源**：batch22 report.md 绕行清单 W-1（pre-vote 未实现）
- **应清批次**：batch23 任务一
- **清偿条件**：pre-vote 实现后 cascading 场景选举 ≤2s，F4 无脑裂全场景复跑全绿
- **清偿方式**：新增 PreVote RPC + 状态机注入 + HandlePreVote

### L-22-2（batch23 待清）
- **来源**：batch22 decisions.md D-8（磁盘满方案仅设计不实施）
- **应清批次**：batch23 任务二
- **清偿条件**：磁盘满时集群拒绝写入但存活，恢复空间后自动回 normal，全程无数据损坏
- **清偿方式**：以 batch22 disk_full_design.md 为蓝本实施，禁止推翻重设计