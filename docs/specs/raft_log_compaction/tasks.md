# batch13 log compaction 编码任务清单

> 红线约束（全程遵守）：
> 1. 禁去 fsync 2. 禁跳 quorum 3. 禁缩选举超时（增大允许）4. 禁调参刷数 5. pipeline 正确性三原则 6. 禁止改 proto 7. 成功率双报制 8. 客户端 retry 不计入服务端稳定性成绩
>
> 关键参数：WAL_SNAPSHOT_THRESHOLD=50000，WAL_SNAPSHOT_MIN_INTERVAL_MS=10000，异步快照（B 级决策）
>
> 复用既有机制：CompactLogs / logStartIndex / ReloadFromSnapshot / getSnapshotData / installSnapshot / ErrCompacted，不另起炉灶

---

## 任务一：log compaction 主体手术

### 1.1 异步快照调度器核心结构定义
- [ ] **T1-01** 在 `raft_pipeline.go` 中定义 `snapshotRequest` 结构体（字段：lastIdx int64、lastTerm int64、term int64）与 `SnapshotScheduler` 结构体（字段：snapshotCh chan *snapshotRequest 容量1、storage *EncryptedStorage、onCompact func(int64)、node *RaftNode、wg sync.WaitGroup、stopCh chan struct{}、logger *log.Logger），完成字段注释说明各字段用途
  - 涉及文件：`raft_pipeline.go`
  - 验证方式：Go 编译通过；结构体字段与 design.md 2.2.2 节接口签名一致
  - 状态：TODO

### 1.2 异步快照调度器执行逻辑实现
- [ ] **T1-02** 在 `raft_pipeline.go` 中实现 `SnapshotScheduler.Start()`（启动消费 goroutine）、`Stop()`（关闭 stopCh 并 wg.Wait）、`Request(req *snapshotRequest)`（select+default 非阻塞投递 snapshotCh）；消费 goroutine 中校验 `rn.Term() == req.term && rn.IsLeader()`，通过后调用 `storage.Snapshot()`，成功则调用 `onCompact(req.lastIdx)` 并重置 totalCommitted，失败则记档不截断
  - 涉及文件：`raft_pipeline.go`
  - 验证方式：Go 编译通过；单测验证 Request 非阻塞（channel 满时跳过）；Start/Stop 生命周期无 goroutine 泄漏
  - 状态：TODO

### 1.3 OnCommit 异步快照触发改造
- [ ] **T1-03** 将 `raft_pipeline.go` 中 `OnCommit`（约 277-306 行）的同步快照逻辑替换为异步投递：达到阈值且满足节流条件时，构造 `snapshotRequest{lastIdx: log.Index, lastTerm: log.Term, term: p.node.Term()}` 并调用 `scheduler.Request()`，不再在 OnCommit 中同步调用 `storage.Snapshot()`；保留 `snapshotMu` 互斥与 `lastSnapshotTime` 节流双重保护
  - 涉及文件：`raft_pipeline.go`
  - 验证方式：OnCommit 不再阻塞等待 Snapshot 完成；快照期间 Propose 不超时；snapshotMu 节流逻辑保留
  - 状态：TODO

### 1.4 main.go 初始化接入与配置项调整
- [ ] **T1-04** 在 `main.go` 中创建 `SnapshotScheduler` 实例并 `Start()`，注入 `storage`、`onCompact: node.CompactLogs`、`node`；将 scheduler 注入 RaftPipeline；`defer scheduler.Stop()`；调整环境变量 `WAL_SNAPSHOT_THRESHOLD` 默认值为 50000、`WAL_SNAPSHOT_MIN_INTERVAL_MS` 默认值为 10000
  - 涉及文件：`main.go`、`raft_pipeline.go`（环境变量读取处）
  - 验证方式：进程启动日志输出 scheduler.Start；环境变量未设置时取默认值 50000/10000；进程退出时 scheduler.Stop 正常结束
  - 状态：TODO

### 1.5 sendHeartbeats 压缩感知扩展
- [ ] **T1-05** 在 `raft.go` 的 `sendHeartbeats`（约 1028-1159 行）锁内构造 peerPlan 时，增加 `if start < rn.logStartIndex` 检查：标记 `pp.needsSnapshot = true` 并置 `pp.entries = nil`（不发送 Command=nil 的已压缩条目）；锁外发送阶段增加 `if plan.needsSnapshot` 分支，调用 `sendSnapshot(p.ID)` 走快照路径并 `continue`，跳过常规 AppendEntries
  - 涉及文件：`raft.go`
  - 验证方式：follower nextIdx < logStartIndex 时走 sendSnapshot 而非发送空 Command 条目；follower 收到 InstallSnapshot 后 ReloadFromSnapshot 并更新 nextIdx=lastIncludedIndex+1
  - 状态：TODO

### 1.6 HandleAppendEntries 压缩感知扩展
- [ ] **T1-06** 在 `raft.go` 的 `HandleAppendEntries`（约 1633-1768 行）PrevLogIndex 检查前，增加 `if req.PrevLogIndex > 0 && req.PrevLogIndex < rn.logStartIndex` 兼容处理：follower 已通过快照拥有该前缀，视为 prevLog 匹配成功，继续处理 Entries（从快照点之后开始追加），不因 Term 查询而拒绝
  - 涉及文件：`raft.go`
  - 验证方式：PrevLogIndex < logStartIndex 时返回 Success=true 并正确追加 Entries；日志匹配链不中断
  - 状态：TODO

### 1.7 落后 follower 快照+增量追赶路径验证
- [ ] **T1-07** 验证 `batch_sync.go` 中 `replicateToFollower` 的 ErrCompacted → sendSnapshot 路径与新增的 sendHeartbeats 压缩感知路径协同工作：follower nextIdx < logStartIndex 时统一走 InstallSnapshot，快照安装后从 lastIncludedIndex+1 开始 AppendEntries 增量追赶至 leader 当前位置；确认 nextIdx/matchIdx 更新正确
  - 涉及文件：`batch_sync.go`、`raft.go`（sendHeartbeats 扩展）
  - 验证方式：追赶完成后 follower commitIdx == leader commitIdx 且日志内容一致；不出现 AppendEntries 发送已压缩条目的情况
  - 状态：TODO

### 1.8 压缩正确性单测：日志匹配链完整
- [ ] **T1-08** 在新增 `raft_batch13_test.go` 中编写 `TestBatch13_LogMatchingChainAfterCompaction`：构造 10000 条日志 → 调用 `CompactLogs(5000)` → 遍历 logs[5000:] 验证每条 `logs[i].Index == i+1` 且 Term 保留 → 验证 prevLogIndex=5000 时 prevLogTerm=logs[4999].Term 正确衔接
  - 涉及文件：`raft_batch13_test.go`（新增）
  - 验证方式：`go test -run TestBatch13_LogMatchingChainAfterCompaction -v` 输出 PASS
  - 状态：TODO

### 1.9 压缩正确性单测：追赶跨越截断点
- [ ] **T1-09** 在 `raft_batch13_test.go` 中编写 `TestBatch13_CatchUpAcrossCompactionPoint`：leader CompactLogs(5000) → follower nextIdx=100（落后于 logStartIndex）→ 触发 InstallSnapshot → ReloadFromSnapshot → AppendEntries 增量追赶 → 验证 follower commitIdx == leader.commitIdx 且日志一致
  - 涉及文件：`raft_batch13_test.go`
  - 验证方式：`go test -run TestBatch13_CatchUpAcrossCompactionPoint -v` 输出 PASS
  - 状态：TODO

### 1.10 压缩正确性单测：崩溃恢复
- [ ] **T1-10** 在 `raft_batch13_test.go` 中编写 `TestBatch13_CrashRecoveryFromSnapshot`：写入 10000 条 → 快照落盘 → CompactLogs(5000) → 模拟崩溃（仅保留 snapshot.gz）→ 调用 ReloadFromSnapshot 重载 → 验证 commitIdx=5000、lastApplied=5000、logStartIndex=1、logs 长度=5000
  - 涉及文件：`raft_batch13_test.go`
  - 验证方式：`go test -run TestBatch13_CrashRecoveryFromSnapshot -v` 输出 PASS
  - 状态：TODO

### 1.11 压缩正确性单测：快照期间写入语义不变
- [ ] **T1-11** 在 `raft_batch13_test.go` 中编写 `TestBatch13_WriteSemanticsDuringSnapshot`：触发异步快照 → 快照执行期间并发 Propose 100 条 → 快照完成后验证 100 条日志 Index > lastIncludedIndex、Command 完整保留、全部 commit 成功
  - 涉及文件：`raft_batch13_test.go`
  - 验证方式：`go test -run TestBatch13_WriteSemanticsDuringSnapshot -v` 输出 PASS
  - 状态：TODO

### 1.12 任务一自检与 decisions.md 落档
- [ ] **T1-12** 运行 `go test -run TestBatch13 -v` 确认 4 项单测全部通过；运行 `go build` 确认编译通过；在 `tests/evidence/batch13/decisions.md` 中落档异步快照 vs 短暂停决策依据（B 级）、阈值 50000/10s 选择依据与内存上界计算、安全前提验证结论
  - 涉及文件：`tests/evidence/batch13/decisions.md`（新增）
  - 验证方式：4 项单测全 PASS；decisions.md 含决策时间、所选方案、安全前提验证结论、决策依据四要素
  - 状态：TODO

---

## 任务二：3min c=128 持续负载全项验收

### 2.1 内存曲线采样器实现
- [ ] **T2-01** 在 `main.go` 中新增 `MemCurveSampler` 结构体（字段：node *RaftNode、path string、interval time.Duration、stopCh chan struct{}、wg sync.WaitGroup）与 `MemSample` 结构体（ts、heapAlloc、heapInuse、heapObjects、numGC、logStartIndex、commitIdx）；实现 `Start()`（启动采样 goroutine，每 5s 调用 runtime.ReadMemStats + node.GetLogStartIndex 读取，JSON 序列化追加写入 mem_curve.jsonl）与 `Stop()`；在 main.go 中初始化并启动，`defer sampler.Stop()`
  - 涉及文件：`main.go`、`tests/evidence/batch13/mem_curve.jsonl`（运行时生成）
  - 验证方式：进程运行期间 mem_curve.jsonl 每 5s 追加一行合法 JSON；含 logStartIndex 字段可观测 compaction 发生
  - 状态：TODO

### 2.2 故障切换测试模块实现
- [ ] **T2-02** 在压测 harness（`tools/loadgen/` 或独立脚本）中新增故障注入模块 `FailoverInjector`：3min 压测第 90s 时 kill 1 个 follower 进程（SIGKILL）→ 记录 kill 时间戳 → 每 1s 轮询写入成功率 → 连续 3s >=99% 视为恢复 → 记录恢复写入秒数；kill 后 10s 重启 follower → 轮询 5/5 存活 → 记录 follower 恢复秒数
  - 涉及文件：`tools/loadgen/main.go` 或新增 `tools/loadgen/failover_injector.go`、`tests/evidence/batch13/failover.json`（运行时生成）
  - 验证方式：failover.json 含 kill 时间戳、恢复写入秒数、follower 重启恢复秒数、5/5 存活状态、leader 切换次数
  - 状态：TODO

### 2.3 3min c=128 持续压测执行与数据采集
- [ ] **T2-03** 启动 5 节点 Raft 集群，执行 3min c=128 持续写入压测，每 30s 窗口采集 TPS / 首试成功率 / 重试后成功率 / P99 / P50 / P95 / 存活数 / leader 切换次数，逐窗口落盘至 `tests/evidence/batch13/`
  - 涉及文件：`tests/evidence/batch13/tps_curve.jsonl`、`tests/evidence/batch13/success_rate.json`、`tests/evidence/batch13/latency.json`、`tests/evidence/batch13/alive.jsonl`
  - 验证方式：180s 压测完成，6 个 30s 窗口数据全部落盘，无空窗
  - 状态：TODO

### 2.4 TPS 达标与不退化验收
- [ ] **T2-04** 计算 3min 全程平均 TPS，验证 ≥ 8000；计算首 30s 窗口平均 TPS vs 末 30s 窗口平均 TPS 衰减率 = (首30s - 末30s) / 首30s，验证 < 5%；将计算结果与原始数据落盘 `tests/evidence/batch13/tps_degradation.json`
  - 涉及文件：`tests/evidence/batch13/tps_degradation.json`
  - 验证方式：TPS ≥ 8000 且衰减率 < 5%；未达标则照交败报 + 新瓶颈定位，不调参刷数
  - 状态：TODO

### 2.5 双报成功率验收
- [ ] **T2-05** 统计 3min c=128 首试成功率与重试后成功率，验证首试 ≥ 99%、重试后 ≥ 99.9%；确认报告同时含两个指标（双报制）；落盘 `tests/evidence/batch13/success_rate.json`
  - 涉及文件：`tests/evidence/batch13/success_rate.json`
  - 验证方式：首试成功率 ≥ 99% 且重试后成功率 ≥ 99.9%；报告含双报字段
  - 状态：TODO

### 2.6 分级延迟（P99/P50/P95）验收
- [ ] **T2-06** 统计 3min c=128 全程 P99 / P50 / P95，验证 P99 ≤ 50ms；确认三项分级延迟全部报告（缺席视为 A 级停机项）；落盘 `tests/evidence/batch13/latency.json`
  - 涉及文件：`tests/evidence/batch13/latency.json`
  - 验证方式：P99 ≤ 50ms；P50/P95/P99 三项均非空
  - 状态：TODO

### 2.7 存活数与 leader 切换次数验收
- [ ] **T2-07** 验证 3min 全程 5/5 存活（每 30s 检查）；统计并报告 leader 切换次数实数；落盘 `tests/evidence/batch13/alive.jsonl`
  - 涉及文件：`tests/evidence/batch13/alive.jsonl`
  - 验证方式：全程 5/5 存活；leader 切换次数为实数且已报告
  - 状态：TODO

### 2.8 内存曲线有界验证
- [ ] **T2-08** 分析 `tests/evidence/batch13/mem_curve.jsonl` 全程内存曲线，验证 compaction 后堆内存占用不随日志条目数线性增长（存在上界）；确认 logStartIndex 在压测期间发生过递增（证明 compaction 确实触发）
  - 涉及文件：`tests/evidence/batch13/mem_curve.jsonl`
  - 验证方式：内存曲线末段不持续上升；logStartIndex 出现 > 0 的采样点；无界则照交败报 + compaction 未生效根因定位
  - 状态：TODO

### 2.9 故障切换实测
- [ ] **T2-09** 执行故障切换测试（T2-02 模块）：3min 第 90s kill 1 follower → 计时恢复写入秒数 → kill 后 10s 重启 follower → 验证 5/5 恢复存活；落盘 `tests/evidence/batch13/failover.json`，含 kill 时间戳、恢复写入秒数、follower 恢复秒数、leader 切换次数
  - 涉及文件：`tests/evidence/batch13/failover.json`
  - 验证方式：恢复写入秒数已记录；follower 重启后 5/5 存活；未恢复则照交败报
  - 状态：TODO

### 2.10 30min 混合读写浸泡执行
- [ ] **T2-10** 前置门禁：确认 T2-04 至 T2-09 全项达标后方可执行；启动 30min c=64 7:3 读写比浸泡压测，全程采集 TPS / 成功率 / P99 / 存活数并落盘；验证重试后成功率 ≥ 99.9%、5/5 存活
  - 涉及文件：`tests/evidence/batch13/soak_alive.jsonl`、`tests/evidence/batch13/soak_success.json`
  - 验证方式：30min 完成；重试后成功率 ≥ 99.9%；5/5 存活；3min 未达标则不启动浸泡
  - 状态：TODO

### 2.11 [SYNC] startIdx 核对
- [ ] **T2-11** 30min 浸泡结束后，核对所有 follower 的 [SYNC] startIdx 恒 > 1，证明 compaction 确实发生且快照同步生效；落盘 `tests/evidence/batch13/sync_startidx.json`
  - 涉及文件：`tests/evidence/batch13/sync_startidx.json`
  - 验证方式：所有 follower startIdx > 1；存在 startIdx <= 1 则照交败报 + 快照未同步根因定位
  - 状态：TODO

### 2.12 heartbeat.log 与 报告.md 落盘
- [ ] **T2-12** 全程维护 `tests/evidence/batch13/heartbeat.log`（5 分钟一行全时段）；任务二结束后生成 `tests/evidence/batch13/报告.md`，先结论后细节，含 TPS/双报成功率/P99/存活/leader切换/内存曲线/故障切换/浸泡/[SYNC] startIdx 全项验收结论，败报照交不追责
  - 涉及文件：`tests/evidence/batch13/heartbeat.log`、`tests/evidence/batch13/报告.md`
  - 验证方式：heartbeat.log 覆盖全时段；报告.md 先结论后细节，全项指标齐全
  - 状态：TODO

---

## 任务三：c=256 饱和边界复测（定性不修）

### 3.1 c=256 饱和压测执行
- [ ] **T3-01** 启动 c=256 并发压测，采集 TPS / 首试成功率 / 重试后成功率 / P99 / 存活数；如实记录，不进行任何优化操作
  - 涉及文件：`tests/evidence/batch13/c256_result.json`（运行时生成）
  - 验证方式：压测完成，数据采集无空项
  - 状态：TODO

### 3.2 c=256 定性记录落盘与不优化确认
- [ ] **T3-02** 将 c=256 实测结果（成功率、P99、TPS、存活数）落盘 `tests/evidence/batch13/c256_result.json`，定性标注为"饱和边界行为"；确认未对 c=256 执行任何优化改动（代码 diff 无 c=256 专项优化）；在 `tests/evidence/batch13/报告.md` 中追加 c=256 定性复测结论
  - 涉及文件：`tests/evidence/batch13/c256_result.json`、`tests/evidence/batch13/报告.md`
  - 验证方式：c256_result.json 含四项实测值；报告.md 含 c=256 定性结论；无优化代码改动
  - 状态：TODO

---

## 任务依赖关系

```
T1-01 → T1-02 → T1-03 → T1-04（异步快照主线）
T1-05（sendHeartbeats 压缩感知，独立于 T1-01~T1-04）
T1-06（HandleAppendEntries 压缩感知，独立于 T1-01~T1-04）
T1-05 + T1-06 → T1-07（落后 follower 追赶验证）
T1-04 → T1-08 ~ T1-11（4 项单测依赖异步快照实现）
T1-08 + T1-09 + T1-10 + T1-11 → T1-12（任务一自检）
T1-12 → T2-01 ~ T2-09（任务二主体验收依赖任务一完成）
T2-01 → T2-08（内存曲线验证依赖采样器）
T2-02 → T2-09（故障切换实测依赖注入模块）
T2-04 + T2-05 + T2-06 + T2-07 + T2-08 + T2-09 → T2-10（浸泡前置门禁）
T2-10 → T2-11（[SYNC] startIdx 核对依赖浸泡完成）
T2-11 → T2-12（报告落盘）
T2-12 → T3-01 → T3-02（任务三在任务二完成后执行）
```