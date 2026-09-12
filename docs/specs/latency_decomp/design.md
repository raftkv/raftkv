# 尾延迟分解优化 — 技术设计

## 1. 架构概览

```
Client → HTTP /raft/propose → [InFlightLimiter] → [TokenBucket] → node.Propose()
                                                                    ↓
                                                            proposeBatchCh (queue)
                                                                    ↓
                                                            proposeBatchLoop (accumulate)
                                                                    ↓
                                                            proposeBatchFlush (lock + append)
                                                                    ↓
                                                            replicateCh (signal)
                                                                    ↓
                                                            heartbeatLoop → sendHeartbeats
                                                                    ↓
                                                            AppendEntries RPC (fan-out to followers)
                                                                    ↓
                                                            follower: append + fsync → ack
                                                                    ↓
                                                            advanceCommit (majority check)
                                                                    ↓
                                                            notifyCommit → waitForCommit → Propose return
```

## 2. 四构成实测分解

### 2.1 时间点定义

| 时间点 | 位置 | 描述 |
|--------|------|------|
| tEnqueue | Propose() 入口 | 请求进入 |
| tFlush | proposeBatchFlush() 入口 | 批开始处理 |
| tRepl | replicateCh 投递后 | 复制信号发出 |
| tCommit | waitForCommit() 检测到 commit | 多数派确认 |

### 2.2 组件定义

| 组件 | 计算 | 含义 |
|------|------|------|
| batch_wait | tFlush - tEnqueue | 攒批队列等待 |
| batch_flush | tRepl - tFlush | 锁内日志追加 + replicateCh 投递 |
| quorum_wait | tCommit - tRepl | 复制 + 多数派 ack + commit 检测 |
| commit_notify | 0 | commit 通知开销（已含在 quorum_wait 中） |

### 2.3 实测占比（P99 场景）

| 组件 | c=128 P99 | c=512 P99 | 可干预性 |
|------|-----------|-----------|----------|
| quorum_wait | 31.6ms (89.3%) | 63.5ms (81.3%) | 受限（fsync/quorum 红线） |
| batch_wait | 14.0ms (39.5%) | 32.5ms (41.7%) | **高**（batchWin 自适应） |
| batch_flush | 1.4ms (4.0%) | 18.2ms (23.3%) | **中**（锁优化） |
| snapshot_io | 0 (0%) | 0 (0%) | 已隔离 |

## 3. 攻击方案设计

### 3.1 方案 A：batch_wait 优化（自适应 flush 超时）

**原理**：当前 batchWin=2ms 固定，低负载时请求等满 2ms 才 flush，贡献 P99 延迟。
高负载时 batch 快速满 64 条立即 flush，batch_wait 小。
但 P99 请求恰好落在 batch 切换边界，等待时间被放大。

**设计**：
- 引入自适应 flush 超时：根据在途并发动态调整 batchWin
- 当 inFlight < cap*50% 时：batchWin = 1ms（低负载快速 flush）
- 当 inFlight >= cap*50% 时：batchWin = 2ms（高负载保持攒批效率）
- 不改变 batchSize=64（保持吞吐）

**预期效果**：
- c=128 batch_wait P99: 14ms → ~7ms（低负载快速 flush）
- c=512 batch_wait P99: 32.5ms → ~20ms（高负载不变）
- TPS 影响：低负载可能略降（更多 flush = 更多锁获取），但 P99 改善

**风险**：
- 自适应参数需要实测标定
- 可能增加锁获取频率，影响 batch_flush

### 3.2 方案 B：batch_flush 锁竞争优化

**原理**：c=512 时 batch_flush P99=18.2ms，因 proposeBatchFlush 持写锁追加日志。
高并发下多个 batch 竞争写锁，P99 请求等待锁释放。

**设计**：
- 日志切片预分配：在 proposeBatchFlush 入口检查 cap(rn.logs) 是否足够，不足时先 grow
- 缩减锁范围：stats 更新移出写锁（用 atomic）
- 批量追加优化：使用 append(rn.logs, batchLogs...) 一次性追加

**预期效果**：
- c=512 batch_flush P99: 18.2ms → ~5ms
- c=128 batch_flush P99: 1.4ms → ~0.5ms
- TPS 影响：正面（锁持有时间缩短）

**风险**：
- 预分配可能浪费内存（但日志增长是单调的，预分配是安全的）
- stats 移分移出锁需要原子操作，确保不丢失计数

### 3.3 方案 C：quorum_wait 优化（如可行）

**原理**：quorum_wait 包含 replicateCh 投递 → heartbeatLoop 唤醒 → sendHeartbeats → RPC → ack → advanceCommit。
当前 heartbeatLoop 通过 select 监听 replicateCh，但 50ms ticker 兜底意味着最坏情况要等 50ms。
实际测试中 replicateCh 是非阻塞投递，heartbeatLoop 应立即唤醒。

**设计**：
- 在 proposeBatchFlush 中，不通过 replicateCh 间接触发，而是直接调用 sendHeartbeats
- 但需注意：sendHeartbeats 持读锁，不能在 proposeBatchFlush 写锁释放前调用
- 方案：在 proposeBatchFlush 写锁释放后，直接 go rn.sendHeartbeats()（异步）
- 风险：可能并发多个 sendHeartbeats，需确保线程安全

**预期效果**：
- 消除 replicateCh → heartbeatLoop 的调度延迟（~1-5ms）
- quorum_wait P99 可能从 63.5ms → ~55ms
- 但 quorum_wait 主体是 RPC RTT + follower fsync，优化空间有限

**红线检查**：
- 不跳 quorum（仍然等待多数派 ack）
- 不动 fsync（follower 端 fsync 不变）
- 不改 proto（AppendEntries 消息格式不变）

**固有下限测算**：
- quorum_wait = RPC RTT + follower append + follower fsync + advanceCommit
- localhost RPC RTT ≈ 1-2ms
- follower append ≈ 0.1ms（内存操作）
- follower fsync ≈ 不可测（红线内，不修改）
- advanceCommit ≈ 0.1ms（多数派计数）
- 固有下限 ≈ 2-3ms（不含 fsync）+ fsync
- 若 fsync 贡献 > 50ms，则 quorum_wait 固有下限 > 50ms，P99≤100ms 需要其他组件趋近于 0

### 3.4 方案 D：快照 I/O 隔离（已隔离确认）

**实测**：snapshot_io = 0%（c=128 和 c=512 均为 0）
**结论**：快照在独立 goroutine 运行，不阻塞复制路径，无需优化
**记录**：已隔离，若后续测试中出现非零值再处理

## 4. 优化优先级

| 优先级 | 方案 | 目标组件 | 预期 P99 改善 | 风险 |
|--------|------|----------|---------------|------|
| P1 | B: 锁竞争优化 | batch_flush | 18.2→5ms | 低 |
| P2 | A: 自适应 flush | batch_wait | 32.5→20ms | 中 |
| P3 | C: 直接复制 | quorum_wait | 63.5→55ms | 中 |
| P4 | D: 快照隔离 | snapshot_io | 已隔离 | 无 |

## 5. 验收标准

| 场景 | 指标 | 目标 | 当前 |
|------|------|------|------|
| c=128 | P99 | ≤100ms | 100ms（基线） |
| c=128 | TPS | ≥5200 | 6119（基线） |
| c=128 | 拒载率 | 0% | 0% |
| c=512 | 被服务 P99 | ≤100ms 或附证据 | 200ms |
| c=512 | 拒载率 | >0%（过载保护生效） | 36.45% |
| c=512 | fail | 0 | 0 |

## 6. 固有下限测算

若优化后 c=512 被服务 P99 仍 > 100ms，执行以下测算：

1. 测量 quorum_wait P99 在无负载时的值（c=1，单请求延迟）
2. 测量 follower fsync 延迟（通过结构化日志事件）
3. 计算：固有下限 = quorum_wait(c=1) + batch_wait(理论最小) + batch_flush(理论最小)
4. 若固有下限 > 100ms，落盘证据并申请调整验收线
5. 不得自行放宽验收线