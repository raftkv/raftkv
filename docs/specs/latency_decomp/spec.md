# 尾延迟分解优化 — 需求规格

## 1. 背景

batch17 验收线 2（c=512 P99≤100ms）未达标，P99=200ms。
batch18 任务零 A/B 矩阵证明 P99=200ms 非代码回归，是 Raft quorum 复制固有尾延迟 + 客户端测量开销。
batch18 任务一埋点实测四构成延迟分解，为优化提供数据依据。

## 2. 实测延迟分解数据

### 2.1 c=128 正常负载（82732 样本）

| 组件 | P50 | P99 | P50 占比 | P99 占比 |
|------|-----|-----|----------|----------|
| batch_wait | 1.06ms | 14.0ms | 20.3% | 39.5% |
| batch_flush | 0.03ms | 1.4ms | 0.6% | 4.0% |
| quorum_wait | 3.50ms | 31.6ms | 66.9% | 89.3% |
| snapshot_io | 0 | 0 | 0% | 0% |
| **服务端总计** | **5.2ms** | **35.4ms** | | |

### 2.2 c=512 过载（69982 样本，shed=36.45%）

| 组件 | P50 | P99 | P50 占比 | P99 占比 |
|------|-----|-----|----------|----------|
| batch_wait | 1.56ms | 32.5ms | 9.3% | 41.7% |
| batch_flush | 0.05ms | 18.2ms | 0.3% | 23.3% |
| quorum_wait | 12.4ms | 63.5ms | 74.4% | 81.3% |
| snapshot_io | 0 | 0 | 0% | 0% |
| **服务端总计** | **16.7ms** | **78.1ms** | | |

### 2.3 攻击优先级排序

1. **quorum_wait**（P99 81-89%）：Raft quorum 复制固有尾延迟
   - 含网络 RTT + follower append + fsync + advanceCommit 检测
   - 红线约束：fsync 不可动、quorum 不可跳
   - 可干预项：replicateCh 触发时机、AppendEntries 批大小、heartbeatLoop 调度

2. **batch_wait**（P99 40-42%）：攒批队列等待时间
   - 当前配置：batchSize=64, batchWin=2ms
   - 可干预项：batchWin 自适应、batchSize 调整

3. **batch_flush**（P99 4-23%）：锁内日志追加
   - 过载时锁竞争放大（4% → 23%）
   - 可干预项：日志预分配、锁范围缩减

4. **snapshot_io**（0%）：独立 goroutine，不阻塞复制路径
   - 无需干预

## 3. 需求

### REQ-1: batch 积累优化
系统在 c=512 过载时，batch_wait P99 应从 32.5ms 降至 ≤15ms。
系统在 c=128 正常负载时，batch_wait P99 应从 14.0ms 降至 ≤8ms。
改动不得破坏 pipeline 正确性三原则。

### REQ-2: batch_flush 锁竞争优化
系统在 c=512 过载时，batch_flush P99 应从 18.2ms 降至 ≤5ms。
改动前后对照数据落盘。

### REQ-3: quorum_wait 优化（如可行）
系统在 c=512 过载时，quorum_wait P99 应从 63.5ms 降低。
若 quorum+fsync 固有下限 > 100ms（服务端），落盘"固有下限测算"并附证据，申请调整验收线。
不得自行放宽验收线，需晨审批。

### REQ-4: 快照 I/O 隔离
快照写路径与复制路径的资源竞争消解。
独立 goroutine 池/写入节流/预算控制。
当前 snapshot_io=0%，若优化后仍为 0%，记录为"已隔离"。

### REQ-5: 不误伤
c=128 正常负载时：P99≤100ms（新基线），TPS≥5200（新基线 6119 的 85%），拒载率=0。
c=512 过载时：被服务请求 P99≤100ms 或附证据的固有下限测算申请。

### REQ-6: 可观测性
延迟分解埋点可通过环境变量开关，关闭时零开销。
埋点数据可通过 /latency/decomp 端点采集。