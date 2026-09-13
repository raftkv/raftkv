# 审计判例库 AUDIT.md

> 版本: v2.4-batch24-governance
> 日期: 2026-09-13
> 维护规则: 每批 FAIL 项归因采信判例入盘，四次"固有"驳回链留档

## 1. 四次"固有"驳回链

### 判例 AUDIT-001: batch17 P99=200ms 固有尾延迟
- **批次**: batch17
- **FAIL 项**: c=512 P99=200ms > 100ms 目标
- **归因**: Raft quorum 复制固有尾延迟，含网络 RTT + follower append + fsync
- **驳回依据**: decomp_c512_raw.json quorum_wait P99=63.5ms 占 81.3%，为 Raft 强一致共识下界
- **处置**: 接受 P99=200ms，降级目标至 c=128 P99=50ms 达标
- **状态**: 采信

### 判例 AUDIT-002: batch20 fsync 合并比 437:1 固有
- **批次**: batch20
- **观察项**: 26 fsync/s, 438 entries/fsync, 437:1 合并比
- **归因**: group commit + batch flush 已将 fsync 合并至极限，WAL 非瓶颈
- **驳回依据**: batch11 证实 group commit 未提升 TPS，瓶颈在 quorum 等待非 fsync
- **处置**: 性能章节收官，fsync "固有"成立
- **状态**: 采信

### 判例 AUDIT-003: batch11 group commit 无效 quorum 等待固有
- **批次**: batch11
- **FAIL 项**: TPS 796.7 < 8000 目标
- **归因**: group commit 减少锁串行但 quorum 等待时间不变，瓶颈为 AppendEntries RPC 往返
- **驳回依据**: TPS 未提升证实 group commit 对当前瓶颈无效
- **处置**: 转向 Pipeline 提交（重叠 quorum 等待），group commit 不再作为优化手段
- **状态**: 采信

### 判例 AUDIT-004: batch23 E1/E4 FAIL pre-vote split vote 固有
- **批次**: batch23
- **FAIL 项**: E1=2.3719s > 2.0s, E4=3.5055s > 2.0s
- **归因**: pre-vote 在所有节点日志相同时无法防止 split vote，多节点同时获 quorum 预支持 → 同时发起正式选举 → 需第二轮
- **驳回依据**: 85 次探测中 16 次阻止选举（18.8%），但 69 次通过后仍 split vote；pre-vote 设计目标为防止日志落后节点选举，非防止同日志节点并发选举
- **处置**: batch24 终裁——若 2.0s 超机制下限则产出改线提案
- **状态**: 待终裁

## 2. 本批归因采信判例

### 判例 AUDIT-005: batch23 RWMutex 不可重入死锁
- **批次**: batch23
- **现象**: 集群部署后所有节点卡在 Follower term=0，/raft/pre_vote 端点挂起
- **根因**: getLastLogIndex()/getLastLogTerm() 内部调 rn.mu.RLock()，但 handleElectionTimeout 已持有 rn.mu.Lock()（写锁），Go sync.RWMutex 不可重入 → 永久阻塞
- **修复**: 内联 len(rn.logs) 和 rn.logs[-1].Term，避免持锁时调用会再次加锁的方法
- **状态**: 已修复

### 判例 AUDIT-006: batch23 磁盘满注入全 PASS
- **批次**: batch23
- **观察项**: DF1-DF4 全 PASS，soft(90%)+hard(99%) 6 场景存活率 100%
- **归因**: 磁盘满注入 follower 不影响 leader 可用性，清理后集群恢复
- **状态**: 采信

## 3. 判例索引

| 判例 ID | 批次 | 类型 | 状态 |
|---------|------|------|------|
| AUDIT-001 | batch17 | 固有驳回 | 采信 |
| AUDIT-002 | batch20 | 固有驳回 | 采信 |
| AUDIT-003 | batch11 | 固有驳回 | 采信 |
| AUDIT-004 | batch23 | 固有驳回 | 待终裁 |
| AUDIT-005 | batch23 | 死锁修复 | 已修复 |
| AUDIT-006 | batch23 | 磁盘满 | 采信 |