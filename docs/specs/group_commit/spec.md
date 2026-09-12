# 组提交（Group Commit）需求规格

## 背景

batch18 尾延迟分解证明：c=512 时 quorum_wait 占 P99 的 81.3%（63.5ms），是尾延迟主导分量。
根因为并发引起 quorum_wait 线性膨胀（c=1:5ms → c=512:120ms+），非 fsync 固有下限超标。
现有 pipeline 已有批量追加（batchSize=64）+ 批量 AppendEntries，但存在三个结构性瓶颈：

1. **commitNotify 容量 1**：advanceCommit 一次可提交 64+ 条目，但 notifyCommit 仅投递 1 个信号，
   512 个 waitForCommit goroutine 中仅 1 个被即时唤醒，其余靠 2ms 轮询 ticker 检测提交
2. **sendHeartbeats CAS 竞争**：高并发下多个 proposeBatchFlush 竞争 1 个 sendHBInFlight 槽位，
   落选者的 replicateCh 信号可能被 select+default 丢弃，导致复制延迟
3. **waitForCommit 逐请求轮询开销**：512 goroutine × 500Hz 轮询 = 256K polls/sec 额外 CPU 开销

## 目标

**验收线 2**：c=512 被服务 P99 ≤ 100ms（batch18 实测 200ms，需降至 100ms）

## 约束

- 禁止修改 fsync 语义（前缀完整性不变）
- 禁止跳过 quorum 确认（多数派确认后方可 commit）
- 禁止修改选举超时参数
- 禁止调参刷数（数字是什么就报什么）
- pipeline 正确性三原则不变
- 禁止修改 proto 定义
- 可观测性代码不进写路径热区

## EARS 格式需求

### REQ-GC-01: 提交广播通知

**WHEN** advanceCommit 推进 commitIdx **THE SYSTEM SHALL** 唤醒所有 index ≤ commitIdx 的 waitForCommit goroutine **WITHIN** 100μs

- 现状：commitNotify cap=1，仅唤醒 1 个 goroutine，其余 2ms 轮询
- 目标：广播唤醒所有已可提交的 goroutine，消除轮询延迟

### REQ-GC-02: 专用复制循环

**WHEN** leader 有未复制日志 **THE SYSTEM SHALL** 在 1ms 内触发 sendHeartbeats **WITHOUT** per-batch CAS 竞争

- 现状：每个 proposeBatchFlush 竞争 sendHBInFlight CAS，落选者信号可能丢弃
- 目标：专用复制 goroutine 定期触发，消除竞争和丢弃

### REQ-GC-03: 批量 quorum 确认

**WHEN** 多数 follower 确认同一 AppendEntries 批量 **THE SYSTEM SHALL** 一次性推进 commitIdx 至批量最高已确认 index **AND** 通知所有相关 waitForCommit

- 现状：advanceCommit 已扫描最高 N，但通知仅 1 个 goroutine
- 目标：批量确认 + 批量通知，一次推进通知所有

### REQ-GC-04: 前缀语义不变

**WHEN** group commit 合并 N 个 propose **THE SYSTEM SHALL** 保持日志前缀完整性 **SUCH THAT** 任意故障场景下已 commit 条目不丢失

- fsync 次数减少但已 commit 条目的持久化语义不变
- quorum 确认对象是批量最高 index，等价于逐条确认（前缀规则）

### REQ-GC-05: 故障中途批量回退

**WHEN** leader 在批量 quorum 确认期间失去 leadership **THE SYSTEM SHALL** 对未 commit 的请求返回 "lost leadership" 错误 **AND** 已 commit 的请求正常返回

- 现状：waitForCommit 已有 stillLeader 检查，需保持
- 目标：批量场景下不遗漏个别请求的状态检查

### REQ-GC-06: 性能不退化

**WHEN** c=128 正常负载 **THE SYSTEM SHALL** 保持 TPS ≥ 6119.2 × 95% = 5813 **AND** P99 ≤ 100ms

- 组提交改动不得损伤正常负载性能
- 基线：batch18-protocol-v1 中位数 TPS=6119.2, P99=100ms

## 预期收益模型

| 场景 | 现状 | 组提交预期 | 机理 |
|------|------|-----------|------|
| c=512 quorum_wait | 63.5ms (P99) | ~30-40ms | 广播通知消除 2ms 轮询 × N 次累积 |
| c=512 P99 | 200ms | ~100ms | quorum_wait 减半 + 消除轮询开销 |
| c=128 P99 | 100ms | ≤100ms | 不退化 |
| c=128 TPS | 7719 | ≥5813 | 不退化（可能因轮询减少而提升） |

## 与现有 pipeline 的关系

| 现有机制 | 覆盖范围 | 组提交增量改动点 |
|----------|----------|-----------------|
| proposeBatchLoop (batchSize=64, win=2ms) | 批量追加日志 | 不变 |
| proposeBatchFlush 锁外预构造 (T1) | 减少 batch_flush | 不变 |
| proposeBatchFlush 自适应 flush (T2) | 减少 batch_wait | 不变 |
| proposeBatchFlush CAS 触发 sendHeartbeats (T3) | 消除 replicateCh 调度延迟 | **改为专用复制循环** |
| sendHeartbeats 批量 AppendEntries | 批量复制 | 不变 |
| advanceCommit 扫描最高 N | 批量 commit 推进 | 不变 |
| notifyCommit cap=1 | 仅通知 1 个 goroutine | **改为广播通知** |
| waitForCommit 2ms 轮询 | 逐请求轮询 | **改为条件等待 + 广播唤醒** |