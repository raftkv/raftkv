# D3-batch12 pipeline AppendEntries 编码任务规划

> 依据：`spec.md`（需求规格）+ `design.md`（实现方案）
> 验收线：TPS≥8000 / 成功率≥99.9% / P99≤50ms / 5/5 存活
> 红线：不许牺牲正确性 / 禁止去 fsync 跳 quorum 缩选举超时刷数 / 禁止调参刷数 / 禁止自行开始新优化
> 术式同源：etcd 3.x 同源方案（per-follower pipeline + in-flight + 乱序对账 + 选举打断）
> 任务顺序：开工前先打 tag v2.4-pre-batch12 → 任务一→二→三→四 → 完工后 commit D3-batch12-pipeline + tag v2.4-post-batch12

---

## 0. 回滚锚点与取证目录准备

**写作指导**：开工前必须先打回滚锚点，确保任一阶段出现 A 级停机项时可全量回滚至 `v2.4-pre-batch12`。同时创建产物落盘目录。

### 0.1 打回滚锚点 tag v2.4-pre-batch12
- [ ] 在当前工作树干净状态下执行 `git tag v2.4-pre-batch12`，作为全量回滚点（spec.md 6.7.1 / design.md 2.5.2）
- [ ] 验证 tag 已创建：`git tag -l v2.4-pre-batch12` 输出非空
- **验收条件**：tag v2.4-pre-batch12 存在且指向开工前提交；回滚命令 `git reset --hard v2.4-pre-batch12` 可用
- **对应红线**：spec.md 4.4.4 回滚锚点

### 0.2 创建产物落盘目录
- [ ] 创建目录 `tests/evidence/d3-batch12/`（spec.md 6.6.1）
- [ ] 将 spec.md 副本拷贝至 `tests/evidence/d3-batch12/spec.md`（spec.md 6.6.2）
- **验收条件**：`tests/evidence/d3-batch12/` 目录存在且含 spec.md 副本
- **对应产物**：spec.md 6.6 产物清单目录与副本

---

## 1. 任务一：RPC 往返解剖（动刀前取证）

**写作指导**：对 leader→follower AppendEntries 单次往返做耗时分解，核查连接复用，修便宜嫌疑。本任务仅取证与修连接复用，不动 fsync / quorum / 选举超时。

### 1.1 插入 AppendEntries 五段计时埋点
- [ ] 在 leader 端 AppendEntries 调用路径插入五段计时埋点：`t0` 连接获取 → `t1` 序列化完成 → `t2` 网络发出 → `t3` 对端处理完成（follower 侧回填）→ `t4` 应答到达 leader → `t5` 应答处理完成（design.md 2.1.3.1 埋点设计）
- [ ] 实现 `rpcDissectCollector` 结构体，提供 `CollectOneRoundTrip(target string) *RoundTripDissect` 方法采集单次往返五段耗时（design.md 2.2.2.7）
- [ ] 埋点开销控制：仅任务一取证阶段开启，常态关闭，避免影响任务四压测（design.md 2.1.3.1）
- **验收条件**：执行单次 AppendEntries 往返后，可获得连接建立/序列化/网络/对端处理/应答五段实测耗时数据
- **对应红线**：spec.md 5.1.1.1 往返分解规则

### 1.2 核查服务端连接复用状态
- [ ] 核查 `raft_pipeline.go` / `raft.go` 中 leader 发送 AppendEntries 的连接获取路径，判定是否每次 `grpc.Dial` / `http.Client` 新建还是从连接池取（design.md 2.1.3.1 连接复用核查）
- [ ] 明确判定结果写入 decisions.md（B 级自主，spec.md 5.1.1.2）
- **验收条件**：明确判定连接是否每次新建；若无法判定则按 B 级记录绕行选择保守路径（spec.md 5.1.3.2）
- **对应红线**：spec.md 5.1.1.2 连接复用核查规则

### 1.3 分支决策：修 keep-alive 或直接进任务二
- [ ] 若每次新建连接：修长连接池复用（keep-alive），修完复测阶梯 8→256，若 TPS 大幅提升则记录于 decisions.md 并继续任务二验证天花板，若 TPS 未提升则进入任务二（spec.md 5.1.1.3 分支决策）
- [ ] 若连接已复用：瓶颈坐实为往返本身，直接进入任务二（spec.md 5.1.1.3）
- [ ] 分支决策过程逐条落盘 decisions.md（spec.md 4.4.1）
- **验收条件**：当连接每次新建时先修 keep-alive 并复测阶梯；当连接已复用时直接进入任务二
- **对应红线**：spec.md 5.1.1.3 分支决策规则（B级自主）

### 1.4 落盘 rpc-dissect.md
- [ ] 调用 `rpcDissectCollector.WriteMarkdown(path, dissects)` 将实测五段耗时分解数据落盘至 `tests/evidence/d3-batch12/rpc-dissect.md`（design.md 2.2.2.7 / spec.md 1.3.1）
- [ ] rpc-dissect.md 内容含实测分解数据（连接建立/序列化/网络/对端处理/应答各阶段耗时）
- **验收条件**：`tests/evidence/d3-batch12/rpc-dissect.md` 存在且含实测分解数据
- **对应产物**：spec.md 6.6.3 rpc-dissect.md
- **异常场景**：分解数据与模型对不上（瓶颈在已列嫌疑外）→ A 级停机项，落盘 decisions.md，停机等面审（spec.md 5.1.3.1）

---

## 2. 任务二：pipeline AppendEntries（主体手术）

**写作指导**：leader 对每个 follower 维护 in-flight 深度 8 的流水线，乱序应答按 term/index 对账，正确性不变。本任务为批次主体改动，涉及 proto 契约扩展、pipeline 状态机、监控采集扩展、单测覆盖。依赖任务一完成（往返解剖已取证）。

### 2.1 proto 契约扩展：AppendEntriesResponse 新增 match_index 字段
- [ ] 在 proto 定义中为 `AppendEntriesResponse` 新增 `match_index` 字段（int64，field 3，optional），确保 wire 兼容不破坏既有节点二进制（design.md 2.2.2.1 / 1.1.2 扩展方向）
- [ ] 重新生成 gRPC stub（`daijin235.pb.go` / `daijin235_grpc.pb.go`）
- [ ] follower 端 AppendEntries handler 处理完成后回填本地最后匹配 index 至 `MatchIndex` 字段（design.md 2.2.2.1 业务说明）
- [ ] leader 端兼容处理：旧节点不填该字段（zero value），leader 视为不回填按既有 Success 语义处理（design.md 2.2.2.1 后置条件）
- **验收条件**：proto wire 兼容；follower 回填 matchIndex；leader 可按 matchIndex 对账；旧节点 zero value 兼容
- **对应红线**：design.md 1.1.2 扩展方向共性约束 1（wire 兼容）

### 2.2 实现 inFlightQueue（per-follower 在途队列）
- [ ] 在 `raft_pipeline.go` 中实现 `inFlightQueue` 结构体：per-follower 一实例，FIFO 入队、按 BatchID 出队对账、深度上限 8（可配）（design.md 2.2.2.2）
- [ ] 实现 `Push(b *inFlightBatch) error`：入队，满则返回 `ErrQueueFull` 触发背压
- [ ] 实现 `Pop(batchID uint64) (*inFlightBatch, error)`：按 BatchID 出队对账
- [ ] 实现 `AbortAll() []*inFlightBatch`：选举/更高 term 打断时全部作废
- [ ] 实现 `Len() int` / `IsFull() bool`：暴露当前深度与满判定
- [ ] `inFlightBatch` 结构体含 `{BatchID, Term, PrevLogIndex, PrevLogTerm, Entries, SendTimestamp}`（design.md 2.2.2.2）
- [ ] 在 `DefaultPipelineConfig` 中追加 `InFlightDepth` 字段，默认 8（design.md 2.1.2 配置项）
- **验收条件**：Push 后 Len()+1；Pop 后 Len()-1；AbortAll 后 Len()==0；深度达 8 时 IsFull()==true 且 Push 返回 ErrQueueFull
- **对应红线**：spec.md 4.1.4 pipeline 深度 / 4.2.3 乱序应答正确性

### 2.3 实现 pipelineSender（链式衔接发送器）
- [ ] 在 `raft_pipeline.go` 中实现 `pipelineSender` 结构体：含 `followerID` / `queue` / `transport` / `nextBatchID`（design.md 2.2.2.3）
- [ ] 实现 `Send(entries, term, prevLogIndex, prevLogTerm) error`：链式衔接构造 AppendEntriesRequest，异步发出，入队 inFlightQueue，不阻塞等待应答（design.md 2.2.2.3 / 2.1.3.2 链式衔接设计）
- [ ] 链式衔接：批 i 的 `prevLogIndex` = 批 i-1 的 `prevLogIndex + len(entries_{i-1})`，`prevLogTerm` = 批 i-1 的最后 entry term（design.md 2.1.3.2 链式衔接设计）
- [ ] 队列满时背压等待（`ErrQueueFull` → 发送侧阻塞）
- **验收条件**：leader 向 follower 发送 AppendEntries 时允许 in-flight 多批（初始深度 8）而不阻塞等待上一批应答；prevLogIndex 链式衔接正确
- **对应红线**：spec.md 5.2.1.1 流水线允许规则 / 5.2.1.2 日志顺序保持规则

### 2.4 实现 responseReconciler（乱序应答对账器）
- [ ] 在 `raft_pipeline.go` 中实现 `responseReconciler` 结构体：含 per-follower `queues` / `matchIdx` / `nextIdx` / `commit`（design.md 2.2.2.4）
- [ ] 实现 `Reconcile(followerID, batchID, resp) error`：按 term/index 对账（design.md 2.2.2.4 / 2.1.3.2 乱序应答对账设计）
- [ ] 成功路径：校验 term 一致 → 滑动 matchIdx 窗口（`matchIdx[follower] = max(matchIdx[follower], resp.MatchIndex)`）→ 从队列 Pop 该批次
- [ ] 失败路径：按拒绝语义回退（`nextIdx[follower] = resp.MatchIndex + 1`）→ 清空该 follower 在途批次（spec.md 5.2.1.3）
- [ ] batchID 不在队列时忽略（已被 AbortAll 清空，design.md 2.2.2.4 异常映射）
- **验收条件**：应答乱序返回时按 index 对账，成功滑动 matchIdx（单调不减），失败回退并清空在途批次
- **对应红线**：spec.md 4.2.3 乱序应答正确性 / 5.2.1.3 乱序应答对账规则

### 2.5 实现 electionAbortHook（选举打断钩子）
- [ ] 在 `raft_pipeline.go` 中实现 `electionAbortHook` 结构体：含 per-follower `queues` / `nextIdx` / `matchIdx`（design.md 2.2.2.5）
- [ ] 实现 `OnElection() error`：调用所有 per-follower `inFlightQueue.AbortAll()`（在途 RPC 全部作废）+ `resetNextIdx`（`nextIdx = matchIdx + 1`）（design.md 2.1.3.2 选举打断处理）
- [ ] 在既有选举事件回调中追加 `electionAbortHook.OnElection()` 调用（design.md 1.1.2 扩展方向）
- [ ] 新 leader 选出后按 etcd 3.x 同源方案重建 pipeline（design.md 2.1.3.2 选举打断处理）
- **验收条件**：选举发生时在途 RPC 全部作废并重置 nextIdx；新 leader 选出后按持久化 raftLog 重建 pipeline
- **对应红线**：spec.md 4.2.5 选举打断处置 / 5.2.1.6 选举打断作废规则（红线）

### 2.6 实现 higherTermAbortHook（更高 term 停止钩子）
- [ ] 在 `raft_pipeline.go` 中实现 `higherTermAbortHook` 结构体：含 per-follower `queues`（design.md 2.2.2.5）
- [ ] 实现 `OnHigherTerm(higherTerm int64) error`：调用 `abortAllInFlight()` + `becomeFollower(higherTerm)`（design.md 2.1.3.2 更高 term 处理）
- [ ] 在 `Step` / `becomeFollower` 路径追加 `higherTermAbortHook.OnHigherTerm()` 调用，先停在途再转角色（design.md 1.1.2 扩展方向 / 2.1.3.2）
- **验收条件**：遇更高 term 时立即停止在途批次并转 follower
- **对应红线**：spec.md 4.2.4 更高 term 处置 / 5.2.1.5 更高 term 停止规则（红线）

### 2.7 保持 commit 推进不变式
- [ ] commit 推进逻辑完全沿用既有 `commitAdvancer`，pipeline 仅在上游插入 in-flight 队列与对账，commit 推进的输入（matchIdx 数组）来源不变、逻辑不变（design.md 2.1.3.2 commit 推进不变式）
- [ ] 不变式保持：`commitIndex = max{N | majority(matchIdx[i] ≥ N) ∧ log[N].term = currentTerm}`，与改造前一致（design.md 2.1.3.2）
- [ ] 崩溃恢复语义不变：pipeline 状态（in-flight 队列）为内存态，崩溃即丢失不写入 WAL，崩溃恢复完全沿用既有路径（raftLog + WAL + SM4）（design.md 2.1.3.2 崩溃恢复语义不变 / 2.3.2 持久化策略）
- [ ] SM4Key 单点（`raft_pipeline.go:53`）不动，避免触发安全 P1 回归（design.md 1.1.2 扩展方向共性约束 2）
- [ ] WAL 路径（`raft_wal.go` 的 `walMaxBatch`/`walFlushInterval`）不动，不引入新 fsync 路径（design.md 1.1.2 扩展方向共性约束 3）
- **验收条件**：pipeline 上线后 commit 推进逻辑与改造前一致；崩溃恢复后 commitIndex 与改造前一致；SM4Key/WAL 路径未改动
- **对应红线**：spec.md 4.2.6 commit 推进不变 / 4.2.7 崩溃恢复语义不变 / 4.3.2 禁止去 fsync

### 2.8 监控采集扩展：inFlightDepth / avgBatchEntries / fsyncPerSec + /pipeline/stats +3 字段
- [ ] 在 `raft_pipeline.go` 暴露 `InFlightDepth() map[string]int`：返回 per-follower in-flight 当前深度（design.md 2.2.2.6）
- [ ] 在 `raft_pipeline.go` 暴露 `AvgBatchEntries() float64`：返回批均条数（滑动窗口统计）（design.md 2.2.2.6）
- [ ] 在 `raft_wal.go` 暴露 `FsyncPerSec() float64`：返回 fsync 每秒次数（design.md 2.2.2.6）
- [ ] 扩展 `/pipeline/stats` 端点 JSON 响应新增 3 字段：`in_flight_depth` / `avg_batch_entries` / `fsync_per_sec`，不破坏既有字段名（design.md 2.2.2.6 端点响应扩展 / 1.2.4 约束）
- [ ] 扩展 `metric_extractor.py` 解析逻辑以识别新增 3 字段（design.md 1.2.4 扩展点）
- **验收条件**：`/pipeline/stats` 响应含 `in_flight_depth` / `avg_batch_entries` / `fsync_per_sec` 三字段；既有字段不受影响；harness 可解析新字段
- **对应产物**：spec.md 5.4.1.2 逐级必报 8 指标的数据源

### 2.9 单测覆盖：8 项不变式单测
- [ ] 编写 `test_log_match_chain`：多批 AppendEntries 按 prevLogIndex 链式衔接发送、乱序到达 follower，验证 follower 按 prevLogIndex/prevLogTerm 判定各批有效性（design.md 2.4 单测表）
- [ ] 编写 `test_out_of_order_response`：in-flight 深度 8、应答乱序返回（批 3 先于批 1），验证 leader 按 batchID+term 对账、matchIdx 单调不减（design.md 2.4）
- [ ] 编写 `test_partial_batch_success`：同一 follower 8 批 in-flight、批 2/4/6 失败其余成功，验证成功批次滑动 matchIdx、失败批次 nextIdx 回退+在途清空（design.md 2.4 / spec.md 5.2.3.1）
- [ ] 编写 `test_election_abort_inflight`：pipeline 在途 5 批、突发选举，验证 abortAllInFlight 清空全部在途、nextIdx=matchIdx+1、新 leader 重建 pipeline（design.md 2.4 / spec.md 5.2.3.2）
- [ ] 编写 `test_higher_term_abort`：pipeline 在途 5 批、发现更高 term，验证 abortAllInFlight+becomeFollower(higherTerm)（design.md 2.4 / spec.md 5.2.3.3）
- [ ] 编写 `test_crash_recovery_unchanged`：pipeline 在途 5 批时 leader 崩溃重启，验证持久化 raftLog+WAL+SM4 恢复、pipeline 按持久化状态重建、commitIndex 与改造前一致（design.md 2.4 / spec.md 5.2.3.4）
- [ ] 编写 `test_commit_advance_invariant`：pipeline 上线后多次 commit 推进，验证 commitIndex 不变式与改造前一致（design.md 2.4）
- [ ] 编写 `test_inflight_depth_bound`：持续发送深度达 8，验证 Push 返回 ErrQueueFull、发送侧背压、深度不超过 8（design.md 2.4）
- [ ] 单测文件位于服务端源码仓 `raft_pipeline_test.go`（与 `raft_pipeline.go` 同包），采用 Go 表驱动测试（design.md 2.4 单测组织）
- [ ] 全部 8 项单测通过；正确性相关单测失败 → A 级停机项；`inflight_depth_bound` 失败 → A 级停机项（内存无界风险）（design.md 2.4 单测通过条件）
- **验收条件**：8 项单测全部通过，覆盖日志匹配链/乱序应答/批次部分成功/选举打断/更高 term/崩溃恢复/commit 推进不变式/in-flight 深度有界
- **对应红线**：spec.md 5.2.1.8 单测覆盖规则 / design.md 2.4 单测方案
- **异常场景**：flaky 单测重跑按 B 级记录绕行（spec.md 6.5.2）

---

## 3. 任务三：成功率回归定性（还 batch11 的账）

**写作指导**：定位 batch11 成功率 99.40%~99.97%（首破 99.9% 线）的失败形态，定位到则修复，定位不到则回滚 group commit 攒批层。pipeline 不强依赖攒批层。依赖任务二完成（pipeline 主体已上线）。

### 3.1 定位 batch11 失败形态
- [ ] 取 batch11 证据（成功率 99.40%~99.97% 区间及失败形态数据）（spec.md 5.3.1.1）
- [ ] 采集失败形态分布，按三维度定位：超时形态（`opLatency` 超阈值占比）/ 拒绝形态（Success=false 占比）/ 攒批窗口空等（group commit 无请求时窗口空转等待时长）（design.md 2.1.3.3 失败形态定位维度）
- **验收条件**：给出 batch11 成功率区间的失败形态定性结论
- **对应红线**：spec.md 5.3.1.1 失败形态定位规则

### 3.2 定位到原因则实施修复
- [ ] 若定位到失败原因：实施修复（spec.md 5.3.1.2）
- [ ] decisions.md 写明修复方案及依据（design.md 2.1.3.3 流程分支设计）
- [ ] 验证修复后回滚 group commit 攒批层是否影响 pipeline：若受影响则判定解耦破坏 A 级停机项（spec.md 5.3.3.2 / design.md 2.1.3.3）
- **验收条件**：定位到原因时修复已实施且 decisions.md 记录方案与依据；修复后 pipeline 主体功能不受影响
- **对应红线**：spec.md 5.3.1.2 定位到则修复规则 / 4.5.4 攒批层解耦

### 3.3 定位不到则回滚 group commit 攒批层
- [ ] 若定位不到原因且回归复现：回滚 group commit 攒批层（pipeline 不强依赖它），输入端直接进 pipeline 不经攒批（design.md 2.1.3.3 解耦设计 / spec.md 5.3.1.3）
- [ ] decisions.md 写明去留决策及依据（spec.md 5.3.1.3）
- [ ] 若既定位不到也无法回归复现：按 B 级记录绕行，回滚 group commit 攒批层，decisions.md 写明决策及依据（spec.md 5.3.3.1 / design.md 2.1.3.3）
- [ ] 验证回滚后 pipeline 主体功能不受影响；若受影响则判定解耦破坏 A 级停机项（spec.md 5.3.3.2）
- **验收条件**：定位不到且回归复现时回滚 group commit 攒批层且 decisions.md 记录去留决策及依据；回滚后 pipeline 不受影响
- **对应红线**：spec.md 5.3.1.3 定位不到则回滚规则 / 5.3.1.4 解耦规则 / 4.5.4 攒批层解耦
- **异常场景**：回滚后 pipeline 受影响 → 解耦破坏 A 级停机项（spec.md 5.3.3.2）

---

## 4. 任务四：阶梯复测 + 对照验收

**写作指导**：按 8→16→32→64→128→256 并发、每级 3min 阶梯复测，逐级报 8 指标，判定验收线，达标则稳态浸泡，未达标则败报+新瓶颈定位。依赖任务二（pipeline+监控采集）与任务三（成功率回归已定性）完成。

### 4.1 执行阶梯压测 8→256 并发
- [ ] 按 8→16→32→64→128→256 并发序列、每级 3min 执行阶梯复测，严格递增不许跳级（spec.md 5.4.1.1 / 6.1.1）
- [ ] c=256 级必须完整记录，不许省略（spec.md 5.4.1.3 / 6.1.3）
- [ ] 单级异常（非 OOM/节点死亡/正确性问题）按 B 级记录绕行降级续跑（spec.md 5.4.3.1）
- **验收条件**：6 级阶梯全部执行完毕；c=256 完整记录
- **对应红线**：spec.md 6.1 阶梯压测参数

### 4.2 逐级采集 8 指标
- [ ] 逐级采集并报 TPS / P50 / P95 / P99 / 成功率 / in-flight 深度 / 批均条数 / fsync 每秒次数（spec.md 5.4.1.2）
- [ ] 前 5 项来自 harness 既有采集 + 补 P95（design.md 1.2.3 缺 P95 / 2.1.3.4 逐级必报）
- [ ] 后 3 项来自 `/pipeline/stats` 扩展字段（任务 2.8 实现）（design.md 2.1.3.4）
- [ ] P99 / P50 / P95 每级必报；缺席视为 A 级停机项，连续两批缺失则纪律升格（spec.md 5.4.1.7 / 4.1.3）
- [ ] 调用 `stepJsonWriter.WriteLevel(path, concurrency, metrics)` 逐级落盘 JSON 至 `tests/evidence/d3-batch12/`（design.md 2.2.2.7）
- **验收条件**：每级阶梯完成时输出上述全部 8 指标；逐级 JSON 落盘；P99/P50/P95 无缺席
- **对应红线**：spec.md 5.4.1.2 逐级必报规则 / 5.4.1.7 分级延迟必报规则（红线）
- **异常场景**：P99/P50/P95 缺席 → A 级停机项（spec.md 5.4.3.6）

### 4.3 验收线判定
- [ ] 判定是否同时满足 TPS≥8000 且成功率≥99.9% 且 P99≤50ms 且 5/5 存活（spec.md 5.4.1.4 / 6.3）
- [ ] 生成 batch10 vs 11 vs 12 三列对照表，落盘至 `tests/evidence/d3-batch12/`（spec.md 1.3.2 / 6.6.4）
- **验收条件**：明确给出达标/未达标结论；三列对照表落盘
- **对应验收线**：spec.md 6.3 验收线（TPS≥8000 / 成功率≥99.9% / P99≤50ms / 5/5 存活）

### 4.4 达标则 30min 稳态浸泡 + 核对
- [ ] 达标则追加 30min 混合读写稳态浸泡（64 并发 7:3 读写比）（spec.md 5.4.1.5 / 6.4）
- [ ] 稳态浸泡期间持续核对 [SYNC] startIdx 恒>1，出现不大于 1 则 A 级停机项（spec.md 5.4.3.4 / 6.4.4）
- [ ] 内存有界核对：监控 RSS/heap，无界增长则 A 级停机项（spec.md 5.4.3.5 / 6.4.5）
- [ ] 若发现数据正确性问题 → A 级停机项（spec.md 5.4.3.3）
- **验收条件**：30min 稳态浸泡完成；[SYNC] startIdx 恒>1；内存有界
- **对应验收线**：spec.md 6.4 稳态浸泡参数 / 6.3.5 达标后稳态浸泡
- **异常场景**：startIdx 不恒>1 / 内存无界 / 数据正确性问题 → A 级停机项（spec.md 5.4.3.3/4/5）

### 4.5 未达标则交败报 + 新瓶颈定位
- [ ] 未达标照交败报 + 新瓶颈定位，禁止调参刷数（spec.md 5.4.1.6 / 红线 4.3.2）
- [ ] 败报遵循"先结论后细节，败报照交不追责"规范（spec.md 4.4.3 / 6.6.8）
- **验收条件**：未达标时交付败报与新瓶颈定位且不调参刷数
- **对应红线**：spec.md 5.4.1.6 未达标败报规则 / 4.3.2 禁止调参刷数

### 4.6 落盘全部产物
- [ ] 落盘 `tests/evidence/d3-batch12/heartbeat.log`：5 分钟一行全时段（spec.md 4.4.2 / 6.6.7）
- [ ] 落盘 `tests/evidence/d3-batch12/报告.md`：先结论后细节，败报照交不追责（spec.md 4.4.3 / 6.6.8）
- [ ] 落盘 `tests/evidence/d3-batch12/decisions.md`：A/B/C 分级决策全部逐条落盘（spec.md 4.4.1 / 6.6.6）
- [ ] 确认 `tests/evidence/d3-batch12/` 含 spec.md 6.6 产物清单全部产物：spec.md 副本 / rpc-dissect.md / 三列对照表 / 阶梯原始数据 JSON / decisions.md / heartbeat.log / 报告.md（spec.md 6.6）
- **验收条件**：`tests/evidence/d3-batch12/` 目录含产物清单全部 8 类产物
- **对应产物**：spec.md 6.6 产物清单

### 4.7 签发锚点：commit D3-batch12-pipeline + tag v2.4-post-batch12
- [ ] 达标且稳态浸泡通过后：执行 `git commit` 消息 `D3-batch12-pipeline`（spec.md 1.3.8 / 6.6.9 / 6.7.2）
- [ ] 执行 `git tag v2.4-post-batch12`（spec.md 6.6.10 / 6.7.2）
- [ ] 签发后停机等晨间面审，禁止自行开始任何新优化（spec.md 1.4.4 / 6.7.3 / design.md 2.5.2 终止约束）
- [ ] 签发后回滚需晨间面审人授权，本组件不自行回滚签发锚点（design.md 2.5.2）
- **验收条件**：commit D3-batch12-pipeline 与 tag v2.4-post-batch12 存在；签发后停机
- **对应红线**：spec.md 6.7.2 签发锚点 / 6.7.3 终止约束 / 1.4.4 禁止自行开始新优化

---

## 5. 全程决策落盘与心跳日志

**写作指导**：A/B/C 分级决策全部逐条落盘 decisions.md，heartbeat.log 5 分钟一行全时段。贯穿任务一至任务四。

### 5.1 decisions.md 逐条落盘
- [ ] 全程 A/B/C 分级决策通过 `decisionsWriter.Append(path, level, item, rationale)` 逐条落盘至 `tests/evidence/d3-batch12/decisions.md`（design.md 2.2.2.7 / spec.md 4.4.1 / 6.5）
- [ ] A 级停机项（OOM/节点死亡/数据正确性问题/分解数据与模型对不上/startIdx 不恒>1/内存无界/P99/P50/P95 缺席/解耦破坏）落盘后停机等面审（spec.md 6.5.1）
- [ ] B 级记录绕行（单级异常降级续跑/flaky 单测重跑/分支决策保守路径）落盘后继续（spec.md 6.5.2）
- [ ] C 级忽略记档（日志格式/统计小数）落盘后继续（spec.md 6.5.3）
- **验收条件**：decisions.md 含全程全部 A/B/C 决策记录，逐条可追溯
- **对应红线**：spec.md 4.4.1 决策落盘 / 6.5 分级授权

### 5.2 heartbeat.log 全时段心跳
- [ ] 全程通过 `heartbeatLogger` 每 5 分钟写一行至 `tests/evidence/d3-batch12/heartbeat.log`（spec.md 4.4.2 / 6.6.7）
- **验收条件**：heartbeat.log 5 分钟一行全时段，覆盖任务一至任务四全程
- **对应产物**：spec.md 6.6.7 heartbeat.log

---

## 6. 回滚锚点与红线对照

**写作指导**：任一阶段出现 A 级停机项或正确性破坏时，回滚至 v2.4-pre-batch12。

### 6.1 阶段回滚点
- [ ] 任务一后：若 keep-alive 修复引入回归，回滚至 `v2.4-pre-batch12` + 仅保留 rpc-dissect.md 取证（design.md 2.5.2）
- [ ] 任务二后：若单测任一正确性用例失败，回滚至 `v2.4-pre-batch12`，pipeline 改动全部撤销（design.md 2.5.2）
- [ ] 任务三后：若回滚 group commit 攒批层后 pipeline 受影响，回滚至 `v2.4-pre-batch12`，判定解耦破坏 A 级停机（design.md 2.5.2）
- [ ] 任务四后：若未达标，**不回滚**，照交败报 + 新瓶颈定位（红线 5.4.1.6 禁止调参刷数）（design.md 2.5.2）
- **验收条件**：各阶段回滚条件明确，回滚命令 `git reset --hard v2.4-pre-batch12` 可用
- **对应红线**：design.md 2.5.2 阶段回滚点 / 2.5.3 回滚锚点与红线对照

### 6.2 红线遵守全程核查
- [ ] 全程不去 fsync：WAL 路径（`raft_wal.go` 的 `walMaxBatch`/`walFlushInterval`）不动（spec.md 4.3.2 / design.md 1.1.2 共性约束 3）
- [ ] 全程不跳 quorum：commit 推进仍需 majority matchIdx（spec.md 4.3.2 / design.md 2.1.3.2）
- [ ] 全程不缩选举超时：选举超时配置不动（spec.md 4.3.2 / design.md 2.1.3.2）
- [ ] 全程不调参刷数：未达标照交败报（spec.md 4.3.2 / 5.4.1.6）
- [ ] SM4Key 单点（`raft_pipeline.go:53`）不动（design.md 1.1.2 共性约束 2）
- [ ] `pkg/pipeline-module` 等独立 Go module 不触碰（design.md 1.2.2 约束）
- **验收条件**：全程未触碰 fsync/quorum/选举超时/SM4Key/pkg/ 路径
- **对应红线**：spec.md 4.3 安全性 / design.md 2.5.3 红线对照表

---

## 任务依赖关系

```
0.1 打 tag v2.4-pre-batch12 ─┐
0.2 创建产物目录 ────────────┤
                             ├─→ 1.1 五段计时埋点 ─→ 1.2 核查连接复用 ─→ 1.3 分支决策 ─→ 1.4 落盘 rpc-dissect.md
                             │
                             └─→ 2.1 proto 扩展 match_index ─→ 2.2 inFlightQueue ─→ 2.3 pipelineSender ─→ 2.4 responseReconciler
                                       │                                                                    │
                                       ├─→ 2.5 electionAbortHook ─→ 2.6 higherTermAbortHook ─→ 2.7 commit 推进不变式
                                       │
                                       └─→ 2.8 监控采集扩展 ─→ 2.9 单测 8 项
                                                 │
                                                 └─→ 3.1 定位失败形态 ─→ 3.2 修复 或 3.3 回滚攒批层
                                                           │
                                                           └─→ 4.1 阶梯压测 ─→ 4.2 逐级 8 指标 ─→ 4.3 验收线判定
                                                                     │
                                                                     ├─→ 4.4 达标则稳态浸泡
                                                                     ├─→ 4.5 未达标则败报
                                                                     └─→ 4.6 落盘产物 ─→ 4.7 commit + tag v2.4-post-batch12

5.1 decisions.md 贯穿全程
5.2 heartbeat.log 贯穿全程
6.1 阶段回滚点 贯穿全程
6.2 红线核查 贯穿全程
```

---

## 验收线与红线汇总

| 类别 | 条目 | 阈值/要求 | 来源 |
|------|------|----------|------|
| 验收线 | TPS | ≥8000 | spec.md 6.3.1 |
| 验收线 | 成功率 | ≥99.9% | spec.md 6.3.2 |
| 验收线 | P99 | ≤50ms | spec.md 6.3.3 |
| 验收线 | 存活 | 5/5 | spec.md 6.3.4 |
| 验收线 | 稳态浸泡 | 30min/64并发/7:3 读写 + startIdx 恒>1 + 内存有界 | spec.md 6.3.5 / 6.4 |
| 红线 | 正确性 | pipeline 不许牺牲正确性 | spec.md 4.3.1 |
| 红线 | 禁止刷数 | 禁止去 fsync / 跳 quorum / 缩选举超时 / 调参刷数 | spec.md 4.3.2 |
| 红线 | commit 推进 | 逻辑不变 | spec.md 4.2.6 |
| 红线 | 崩溃恢复 | 语义不变 | spec.md 4.2.7 |
| 红线 | 更高 term | 立即停止在途转 follower | spec.md 4.2.4 |
| 红线 | 选举打断 | 在途 RPC 全部作废 + 重置 nextIdx | spec.md 4.2.5 |
| 红线 | 分级延迟必报 | P99/P50/P95 每级必报 | spec.md 4.1.3 |
| 红线 | 回滚锚点 | 开工前 tag v2.4-pre-batch12 | spec.md 4.4.4 / 6.7.1 |
| 红线 | 签发锚点 | commit D3-batch12-pipeline + tag v2.4-post-batch12 | spec.md 6.7.2 |
| 红线 | 终止约束 | 签发后禁止自行开始新优化 | spec.md 1.4.4 / 6.7.3 |
| 约束 | 术式同源 | etcd 3.x 同源方案 | spec.md 4.5.1 |
| 约束 | 攒批层解耦 | pipeline 不强依赖 group commit | spec.md 4.5.4 |
| 约束 | in-flight 深度 | 可配，初始 8 | spec.md 6.2.1 |
| 约束 | 阶梯协议 | 8→16→32→64→128→256，每级 3min，不许跳级 | spec.md 6.1 |