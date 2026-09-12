# 组提交实施任务

## T1: commitBroadcaster 结构体 + Notify/Wait 接口

**文件**: `raft.go`

**改动**:
- 新增 `commitBroadcaster` 结构体（mu + cond + commitIdx）
- 新增 `Notify(newCommitIdx int64)` 方法（Broadcast 唤醒所有等待者）
- 新增 `Wait(index int64) bool` 方法（条件等待至 commitIdx >= index）
- 新增 `WaitTimeout(index int64, timeout time.Duration) bool` 方法（带超时的条件等待）
- `RaftNode` 新增 `commitBroadcast *commitBroadcaster` 字段
- `NewRaftNode` 中初始化 `commitBroadcast`

**验收**: 单元测试 Notify 唤醒多个 Wait goroutine

**状态**: 待实施

## T2: advanceCommit 改用 commitBroadcast.Notify

**文件**: `raft.go`

**改动**:
- `advanceCommit` 中 `rn.notifyCommit()` 替换为 `rn.commitBroadcast.Notify(rn.commitIdx)`
- 保留 `notifyCommit()` 函数（其他地方可能仍在用，如 applyCommittedLogs）
- `commitNotify` channel 保留（向后兼容，但 waitForCommit 不再监听）

**验收**: advanceCommit 推进后所有 waitForCommit 被唤醒

**状态**: 待实施

## T3: waitForCommit 改用 commitBroadcast.WaitTimeout

**文件**: `raft.go`

**改动**:
- 移除 `pollTicker`（2ms 轮询）
- 主路径改为 `rn.commitBroadcast.WaitTimeout(index, 2*time.Millisecond)`
- 保留 `commitTimer`（3s 超时）和 `shutdownCh` 检查
- 保留 `stillLeader` 检查
- 延迟分解记录逻辑不变

**验收**: waitForCommit 在 commit 后 100μs 内返回

**状态**: 待实施

## T4: 专用复制循环 replicateLoop

**文件**: `raft.go`

**改动**:
- 新增 `replicateTrigger chan struct{}`（cap=256）和 `replicateStop chan struct{}`
- 新增 `replicateLoop()` 方法：1ms ticker + replicateTrigger 信号 → sendHeartbeats
- `startProposeBatchLocked` 中启动 replicateLoop goroutine
- `StopProposeBatch` 中停止 replicateLoop
- `proposeBatchFlush` 中 CAS 触发改为 `replicateTrigger <- struct{}{}` 非阻塞投递
- 移除 `sendHBInFlight` CAS 逻辑（不再需要）

**验收**: 专用循环 1ms 内触发 sendHeartbeats，无 CAS 竞争

**状态**: 待实施

## T5: 单元测试

**文件**: `raft_batch19_test.go`（新建）

**测试用例**:
- `TestCommitBroadcast_NotifyWakesAll`: Notify 唤醒多个 Wait goroutine
- `TestCommitBroadcast_WaitTimeout`: WaitTimeout 超时返回 false
- `TestGroupCommit_BatchQuorumConfirm`: 批量 quorum 确认后所有请求被通知
- `TestGroupCommit_LostLeadership`: 批量确认期间失去 leadership，未 commit 返回错误
- `TestReplicateLoop_TriggerAndTicker`: replicateTrigger 信号和 1ms ticker 均触发 sendHeartbeats

**验收**: 所有测试通过

**状态**: 待实施

## T6: 集成验收测试

**测试场景**:
1. c=128 3min 新鲜集群：P99 ≤ 100ms, TPS ≥ 5813, shed=0
2. c=512 3min 新鲜集群：P99 ≤ 100ms（目标），shed 上报实际值
3. 阶梯 c=8 → c=256 → c=512：三口径上报
4. cap 单调性：cap=256 vs cap=200（验证相变仍存在）
5. 5/5 节点存活

**验收标准**（逐字段对照）:
- c=128 P99 ≤ 100ms
- c=128 TPS ≥ 5813（6119.2 × 95%）
- c=128 shed = 0
- c=512 P99 ≤ 100ms
- 5/5 存活

**状态**: 待实施

## T7: 产物 + 提交

**产物**:
- `tests/evidence/d3-batch19/report.md`
- `tests/evidence/d3-batch19/decisions.md`
- `tests/evidence/d3-batch19/*.json`（测试数据）
- commit `D3-batch19-gcommit: 组提交广播+专用复制循环`
- tag `v2.4-post-batch19`

**状态**: 待实施