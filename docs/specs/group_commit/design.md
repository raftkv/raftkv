# 组提交技术设计

## 架构概览

```
proposeBatchLoop → proposeBatchFlush → logs append
                                          ↓
                              专用复制循环 (1ms ticker)
                                          ↓
                              sendHeartbeats (ALL pending entries)
                                          ↓
                              follower AppendEntries RPC
                                          ↓
                              advanceCommit (批量推进)
                                          ↓
                              commitBroadcast (唤醒所有已提交 goroutine)
                                          ↓
                              waitForCommit (条件等待，非轮询)
```

## 改动点

### D1: 提交广播机制（替换 commitNotify）

**现状**：
```go
commitNotify chan struct{} // cap=1, notifyCommit 非阻塞投递 1 个信号
// waitForCommit: case <-rn.commitNotify: 或 2ms poll ticker
```

**设计**：
```go
type commitBroadcaster struct {
    mu     sync.Mutex
    cond   *sync.Cond
    commitIdx int64  // 最新已知 commitIdx（原子可见）
}

func (b *commitBroadcaster) Notify(newCommitIdx int64) {
    b.mu.Lock()
    atomic.StoreInt64(&b.commitIdx, newCommitIdx)
    b.cond.Broadcast() // 唤醒所有等待者
    b.mu.Unlock()
}

func (b *commitBroadcaster) Wait(currentIdx int64) bool {
    // 返回 true 表示 commitIdx >= currentIdx
    b.mu.Lock()
    for atomic.LoadInt64(&b.commitIdx) < currentIdx {
        b.cond.Wait()
    }
    b.mu.Unlock()
    return true
}
```

**waitForCommit 改造**：
```go
func (rn *RaftNode) waitForCommit(index int64, resultCh chan proposeResult, req *proposeRequest) {
    commitTimer := time.NewTimer(3 * time.Second)
    defer commitTimer.Stop()

    for {
        // 快速路径：检查是否已提交
        rn.mu.RLock()
        committed := rn.commitIdx >= index
        stillLeader := rn.state == StateLeader
        rn.mu.RUnlock()

        if committed {
            // 记录延迟分解 + 返回成功
            ...
            return
        }
        if !stillLeader {
            resultCh <- proposeResult{err: ...}
            return
        }

        // 慢速路径：等待广播通知
        select {
        case <-commitTimer.C:
            resultCh <- proposeResult{err: ...}
            return
        case <-rn.shutdownCh:
            resultCh <- proposeResult{err: ...}
            return
        default:
        }

        // 条件等待（带超时）
        rn.commitBroadcast.WaitTimeout(index, 2*time.Millisecond)
    }
}
```

**关键决策**：保留 2ms 超时作为兜底（防 cond.Broadcast 遗漏），但主路径是 Broadcast 唤醒。
cond.Wait 内部不消耗 CPU，消除 256K polls/sec 轮询开销。

**advanceCommit 改造**：
```go
func (rn *RaftNode) advanceCommit(term int64) {
    ...
    if committed {
        rn.commitIdx = N
        rn.commitBroadcast.Notify(N) // 替换 rn.notifyCommit()
    }
    ...
}
```

### D2: 专用复制循环（替换 per-batch CAS 触发）

**现状**：
```go
// proposeBatchFlush 中：
if atomic.CompareAndSwapInt32(&rn.sendHBInFlight, 0, 1) {
    go func() {
        defer atomic.StoreInt32(&rn.sendHBInFlight, 0)
        rn.sendHeartbeats()
    }()
} else {
    select {
    case rn.replicateCh <- struct{}{}:
    default: // 丢弃！
    }
}
```

**设计**：
```go
// 新增专用复制 goroutine，Leader 当选时启动
func (rn *RaftNode) replicateLoop() {
    ticker := time.NewTicker(1 * time.Millisecond) // 1ms 复制间隔
    defer ticker.Stop()
    for {
        select {
        case <-rn.shutdownCh:
            return
        case <-rn.replicateStop:
            return
        case <-rn.replicateTrigger: // proposeBatchFlush 投递信号
            rn.sendHeartbeats()
        case <-ticker.C:
            rn.sendHeartbeats()
        }
    }
}

// proposeBatchFlush 改为非阻塞信号投递：
func (rn *RaftNode) proposeBatchFlush(batch []*proposeRequest) {
    ...
    // 追加日志后，投递复制信号（非阻塞，cap=256 足够）
    select {
    case rn.replicateTrigger <- struct{}{}:
    default:
        // 信号槽满，1ms ticker 会兜底
    }
    ...
}
```

**关键决策**：
- 1ms ticker 兜底，即使信号丢失最多 1ms 后自动复制
- replicateTrigger cap=256，高并发下不丢信号
- 消除 sendHBInFlight CAS 竞争
- sendHeartbeats 本身是幂等的（发送 ALL pending entries），多次调用无副作用

### D3: waitForCommit 超时兜底保留

**设计**：cond.Broadcast 是主路径，但保留 2ms 超时 ticker 作为兜底：
```go
// 在 cond.Wait 超时后重新检查 commitIdx
rn.commitBroadcast.WaitTimeout(index, 2*time.Millisecond)
```

**理由**：
- 防 cond.Broadcast 与 advanceCommit 之间的竞态遗漏
- 兜底开销极低（仅在 Broadcast 未到达时才触发，正常情况下 Broadcast 先到）
- 保持与现有 3s commit 超时的一致性

## 正确性论证

### 前缀语义不变

**定理**：group commit 合并 N 个 propose 的 fsync 和 quorum 确认，不改变已 commit 条目的前缀完整性。

**证明**：
1. **日志追加**：proposeBatchFlush 已在一次锁内 append N 条日志，前缀连续（Index = base+1, base+2, ..., base+N）
2. **AppendEntries**：sendHeartbeats 向 follower 发送从 nextIdx 到 logEnd 的所有 entries，follower 一次性追加并 fsync。前缀连续性由 PrevLogIndex/PrevLogTerm 检查保证
3. **quorum 确认**：advanceCommit 扫描最高 N 使多数派 matchIdx >= N。由 matchIdx 单调递增和前缀规则，N 以下所有条目均已持久化在多数派上
4. **commit 推进**：commitIdx 从 oldCommit 跳至 N，等价于逐条推进 oldCommit+1, ..., N（前缀规则保证中间无空洞）
5. **广播通知**：唤醒所有 index ≤ N 的 waitForCommit，每个 goroutine 检查 commitIdx >= index 后返回。与逐条通知语义等价

### pipeline 三原则对照

| 原则 | 现状 | 组提交 | 保持 |
|------|------|--------|------|
| 日志前缀连续 | batch append Index 连续 | 不变 | ✅ |
| quorum 确认后方可 commit | advanceCommit 检查多数派 matchIdx | 不变 | ✅ |
| 已 commit 不丢失 | fsync + majority 持久化 | fsync 次数减少但 majority 不变 | ✅ |

### 故障中途批量回退

**场景**：leader 在批量 quorum 确认期间失去 leadership

**处理**：
1. advanceCommit 在锁内检查 `rn.state == StateLeader`，降级后不推进 commitIdx
2. waitForCommit 在每次检查时验证 `stillLeader`，降级后返回 "lost leadership" 错误
3. 已 commit 的请求（commitIdx >= index）在 committed 检查中先于 stillLeader 返回成功
4. **批量场景**：广播通知唤醒所有 goroutine，每个 goroutine 独立检查 committed 和 stillLeader，不遗漏个别请求

## 性能预期

### c=512 quorum_wait 分解

| 子分量 | 现状 | 组提交后 | 改善 |
|--------|------|----------|------|
| RPC RTT + follower fsync | ~30ms | ~30ms | 不变（固有） |
| sendHeartbeats 触发延迟 | 0-2ms (CAS 竞争) | 0-1ms (专用循环) | -1ms |
| commit 通知延迟 | 0-2ms × N (轮询) | ~0ms (广播) | -2ms × N |
| 轮询 CPU 开销 | 256K polls/sec | ~0 (cond.Wait) | 大幅降低 |

**预期 quorum_wait P99**：63.5ms → ~35-45ms（消除通知延迟和轮询开销）

### c=512 P99 预期

P99 = quorum_wait + batch_wait + batch_flush + HTTP
    = ~40ms + ~32ms + ~18ms + ~122ms
    = ~212ms?

**注意**：HTTP 开销 ~122ms 是 batch18 测算的固定开销。如果 quorum_wait 降至 40ms 但 P99 仍 > 100ms，则需进一步分析 HTTP 开销是否可优化。

**修正预期**：batch18 的 P99=200ms 中，server_total P99=78ms（p99_decomp.json），client P99=200ms。
差值 122ms 为 HTTP + client-side 开销。组提交优化 server_total 中的 quorum_wait：
- server_total P99: 78ms → ~50ms (quorum_wait 63.5→35, 其他不变)
- client P99: 200ms → ~170ms? (取决于 HTTP 开销是否固定)

**保守预期**：c=512 P99 从 200ms 降至 100-150ms。若仍 > 100ms，需晨审批。

## 风险

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| cond.Broadcast 唤醒所有 goroutine 导致惊群 | 中 | CPU 峰值 | goroutine 唤醒后仅做一次 RLock 检查，开销极低 |
| 专用复制循环 1ms 间隔增加 RPC 频率 | 低 | 网络开销 | sendHeartbeats 幂等，无新日志时 entries 为空 |
| cond.Wait 与 advanceCommit 竞态 | 低 | 通知遗漏 | 2ms 超时兜底 |
| c=128 性能退化 | 低 | 验收线 1 不达标 | cond.Wait 开销 < chan 轮询开销 |