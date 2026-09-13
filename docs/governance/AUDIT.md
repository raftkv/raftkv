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

## 3. batch25 追加判例

### 判例 AUDIT-007: batch24 首例合法改线提案全流程（正向）
- **批次**: batch24→batch25
- **FAIL 项**: E4=3.4607s > 2.0s
- **流程**:
  1. batch24 产出 D-24-1 提案（decisions.md），含根因分析+机制下限推导+反对论点自审+替代方案排除
  2. 提案状态="待晨批裁决"，**未自行修改 YAML**（遵守"改线仅限提案制"）
  3. batch25 任务一落地：补量化推导 + E4 拆为 E4a(≤2.0s 不放松)+E4b(≤3.5s) + 复跑验证
  4. YAML diff 显式落盘，实际修改待晨批批准后执行
- **合规要点**: 提案→自审→落盘→等裁→落地，全链可审计，无偷改线
- **状态**: 采信（正向标杆）

### 判例 AUDIT-008: 验收数字五次催缴未入晨报（失败）
- **批次**: 历史多批
- **现象**: 验收关键数字（如 E1/E4 实测值、P99 分解、fsync 量纲）多次催缴后仍未入晨报
- **归因**: 产物与晨报脱节，证据落盘但未同步至面审入口
- **对抗措施**: MISBEHAVIOR.md 收录"催缴无果"模式 + actions.json 双向对账机制
- **模板修订**: 是——batch25 新增 actions.json 双向对账，晨审与申报三清单差额=隐瞒或漏报
- **状态**: 采信（反面教材）

## 4. 判例索引

| 判例 ID | 批次 | 类型 | 状态 |
|---------|------|------|------|
| AUDIT-001 | batch17 | 固有驳回 | 采信 |
| AUDIT-002 | batch20 | 固有驳回 | 采信 |
| AUDIT-003 | batch11 | 固有驳回 | 采信 |
| AUDIT-004 | batch23 | 固有驳回 | 待终裁 |
| AUDIT-005 | batch23 | 死锁修复 | 已修复 |
| AUDIT-006 | batch23 | 磁盘满 | 采信 |
| AUDIT-007 | batch24→25 | 正向改线 | 采信 |
| AUDIT-008 | 历史 | 催缴失败 | 采信 |