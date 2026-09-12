# 尾延迟分解优化 — 实施任务

## T1: batch_flush 锁竞争优化（P1）

**目标**：c=512 batch_flush P99 从 18.2ms 降至 ≤5ms

**改动**：
1. raft.go proposeBatchFlush：日志切片预分配
   - 在写锁内检查 cap(rn.logs) 是否足够容纳 batch
   - 不足时先 grow（避免 append 时 re-alloc）
2. raft.go proposeBatchFlush：stats 更新移出写锁
   - rn.stats.LogCount 改用 atomic.StoreInt32
   - 写锁范围仅覆盖 logs append
3. raft.go proposeBatchFlush：批量追加
   - 构造临时 batchLogs []RaftLog，一次 append(rn.logs, batchLogs...)

**验证**：
- 改动前后 batch_flush P99 对照数据落盘
- pipeline 正确性三原则不破坏
- 单测覆盖

**状态**：待执行

---

## T2: batch_wait 自适应 flush 超时（P2）

**目标**：c=128 batch_wait P99 从 14ms 降至 ≤8ms

**改动**：
1. raft.go proposeBatchLoop：动态调整 batchWin
   - 读取 inFlightLimiter.Utilization()
   - utilization < 0.5 → batchWin = 1ms
   - utilization >= 0.5 → batchWin = 2ms
2. raft.go RaftNode：添加 inFlightUtilization 函数
   - 返回当前在途利用率
   - 需要引用 inFlightLimiter（从 main.go 注入）

**验证**：
- 改动前后 batch_wait P99 对照数据落盘
- c=128 TPS 不降（≥5200）
- c=512 拒载率不变

**状态**：待执行

---

## T3: quorum_wait 直接复制触发（P3）

**目标**：消除 replicateCh → heartbeatLoop 调度延迟

**改动**：
1. raft.go proposeBatchFlush：写锁释放后直接异步触发 sendHeartbeats
   - 替换 replicateCh 投递为 go rn.sendHeartbeats()
   - 保留 replicateCh 作为 fallback（heartbeatLoop ticker 仍 50ms 兜底）
2. raft.go sendHeartbeats：确保并发安全
   - 检查是否已有 sendHeartbeats 在运行（用 atomic flag 或 channel）
   - 若已在运行，跳过（避免重复发送）

**验证**：
- 改动前后 quorum_wait P99 对照数据落盘
- 不跳 quorum（advanceCommit 仍等多数派）
- 不动 fsync
- 不改 proto

**状态**：待执行

---

## T4: 快照 I/O 隔离确认（P4）

**目标**：确认快照 I/O 不阻塞复制路径

**改动**：无代码改动（已隔离）

**验证**：
- 延迟分解测试中 snapshot_io = 0%
- 结构化日志中 snapshot 事件不在复制 goroutine 栈中
- 记入"已隔离"

**状态**：已完成（实测 snapshot_io=0%）

---

## T5: 延迟分解埋点开关化（已完成）

**目标**：埋点可通过环境变量开关，关闭时零开销

**改动**：
1. latency_decomp.go：LATENCY_DECOMP=true|1 启用
2. main.go：/latency/decomp 端点
3. raft.go：proposeRequest 时间戳字段

**验证**：
- 开启时 /latency/decomp 返回数据
- 关闭时 Record() 直接返回（atomic.Bool Load）

**状态**：已完成

---

## T6: 验收测试

**目标**：对照新基线（TPS=6119, P99=100ms@c=128）验收

**测试**：
1. c=128 正常负载 3min（新鲜集群）
   - P99 ≤ 100ms
   - TPS ≥ 5200（新基线 85%）
   - 拒载率 = 0
2. c=512 过载 3min（新鲜集群）
   - 被服务请求 P99 ≤ 100ms 或附证据
   - 三口径全报
   - 拒载率 > 0
   - fail = 0
   - 5/5 存活
3. 阶梯 8→256→512 三口径
4. cap 扫描含单调性验证（cap 递减时 shed 递增，排除病态区）

**状态**：待执行

---

## T7: 固有下限测算（条件执行）

**条件**：若 T6 中 c=512 被服务 P99 仍 > 100ms

**测算**：
1. c=1 单请求延迟（测 quorum_wait 固有下限）
2. follower fsync 延迟（结构化日志）
3. 计算：固有下限 = quorum_wait(c=1) + batch_wait_min + batch_flush_min
4. 落盘证据
5. 申请晨审批调整验收线

**状态**：待执行（条件触发）