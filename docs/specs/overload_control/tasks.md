# 过载保护设计重构编码任务清单（batch17）

> 批次：batch17
> 上游规格：docs/specs/overload_control/spec.md
> 上游设计：docs/specs/overload_control/design.md
> 基线参照：
> - 权威基线：8240.4 TPS（batch14 归零复测，3min c=128，成功率=100%，P99=50ms，5/5 存活，0 次 leader 切换）
> - batch16 c=128 mTLS 复测：TPS=10617.2，P99=50ms，P50=20ms，fail=0
> - batch16 c=512 过载：TPS=14919，成功率=99.30%，P99=200ms（**未达标**，根因：令牌桶限 TPS 不限并发）
> - batch16 阶梯：c=8 TPS=1501.9/P99=10ms，c=256 TPS=12726.3/P99=100ms，c=512 TPS=12927.2/P99=200ms
> - 令牌桶配置（继承不改）：maxTokens=1024，initialRate=10000
> - in-flight cap 标定：600（Little's Law 推导，λ=10617.2，W=P99=50ms，L_steady=531，headroom 13%）
> - 集群拓扑：5 节点，gRPC port 9500，HTTP port 9000，Docker 部署
> 红线约束：禁去 fsync / 禁跳 quorum / 禁缩选举超时 / 禁调参刷数 / pipeline 正确性三原则 / 禁止改 proto / 成功率双报制 / retry 不计入服务端成绩 / 可观测性代码不得进入写路径热区 / 三口径独立统计 / 测试密钥不得入 git
> 任务编号约定：T17-{模块}-{序号}，模块缩写 SELFCHK=自查 / IFL=并发限制器 / TRI=三口径 / GATE=双层协同 / CURVE=曲线 / UT=单测 / ACC=验收 / CERT=证书 / ART=产物

---

## 任务依赖关系 DAG

```
T17-SELFCHK-01 ~ T17-SELFCHK-03 (batch16 自查)  ─────── 独立前置，1 小时级
                                                     │
                                                     │
T17-IFL-01 ~ T17-IFL-03 (并发限制器)  ─┐            │
                                        ├──→ T17-UT-01 (信号量单测)
T17-TRI-01 ~ T17-TRI-02 (三口径统计)  ─┤            │
                                        ├──→ T17-UT-02 (三口径单测)
                                        │            │
                                        └──→ T17-GATE-01 ~ T17-GATE-04 (双层协同)
                                                  │
                                                  ├──→ T17-UT-03 (双层协同单测)
                                                  │
                                                  └──→ T17-CURVE-01 ~ T17-CURVE-03 (过载曲线)
                                                            │
                                                            └──→ T17-ACC-01 ~ T17-ACC-03 (验收)
                                                                      │
                                                                      └──→ T17-ART-01 ~ T17-ART-05 (产物)

T17-CERT-01 ~ T17-CERT-03 (证书工程化，附带)  ─── 独立旁支，时间富余才做
```

**并行机会**：
- **第一批并行**：T17-SELFCHK-*（任务零）/ T17-IFL-*（任务一）/ T17-TRI-*（任务二）/ T17-CERT-*（任务六附带）四组互不依赖，可同时启动
- **第二批**：T17-GATE-*（任务三）依赖 IFL + TRI 完成后方可启动
- **第三批并行**：T17-UT-01 / T17-UT-02 可在 IFL / TRI 实现完成后并行编写；T17-UT-03 须等 GATE 完成
- **第四批**：T17-CURVE-* 依赖 GATE-04（metrics 扩展）完成
- **第五批**：T17-ACC-* 依赖 UT 全 PASS + CURVE 完成
- **末批**：T17-ART-* 依赖 ACC 全部达成

---

## 0. batch16 交付物自查补齐（任务零，1 小时级前置）

> 逐项核对 batch16 已完成但未在报告中显式落盘的交付物，缺则补齐，均已存在仅报告漏列。
> 红线：自查表必须写入 tests/evidence/d3-batch17/decisions.md，五项逐项标注"已存在"或"已补齐"。

### 0.1 T17-SELFCHK-01：逐项核对 d3-batch16/ 交付物落盘
- [ ] **状态**：TODO
- **描述**：逐项核对 `tests/evidence/d3-batch16/` 是否落盘以下五项交付物，每项判定 status ∈ {exists, missing, incomplete}：
  1. 9232.5 对账表（batch16 任务零，未报）
  2. mTLS 开启后 3min c=128 复测数字（TPS/P99/双报）
  3. 结构化日志验证记录 + 无回归复测
  4. 验收线5 未达标的具体回归点定位（c=512 / P99=200ms / 目标 ≤100ms）
  5. tasks.md T15-* 状态回填为实际状态
  缺失项补齐至 `tests/evidence/d3-batch16/` 或 `tests/evidence/d3-batch17/decisions.md`，已存在项仅记录漏列原因。
- **涉及文件**：`tests/evidence/d3-batch16/`（核对 + 必要时补齐）、`tests/evidence/d3-batch17/decisions.md`（自查表写入）
- **验证方式**：
  1. 五项交付物逐项在 decisions.md 中有核对记录
  2. 每项标注"已存在"或"已补齐" + 漏列原因/补齐内容
  3. 缺失项补齐后文件实际存在
- **依赖**：无

### 0.2 T17-SELFCHK-02：tasks.md T15-* 状态回填
- [ ] **状态**：TODO
- **描述**：核对 `docs/specs/production_hardening/tasks.md` 中 T15-* 任务状态是否与实际一致：已完成标 `DONE`，未做标 `TODO`，取消标 `SKIP`。回填后状态须反映真实交付情况，不得有"TODO 但实际已完成"或"未标注但已实现"的不一致项。
- **涉及文件**：`docs/specs/production_hardening/tasks.md`（T15-* 状态字段回填）
- **验证方式**：
  1. T15-* 每项状态 ∈ {TODO, DONE, SKIP}
  2. 状态与实际交付一致（grep 代码确认）
  3. 不一致项在 decisions.md 中标注原因
- **依赖**：T17-SELFCHK-01

### 0.3 T17-SELFCHK-03：decisions.md 说明漏列项 + 验收线5 回归点定位
- [ ] **状态**：TODO
- **描述**：在 `tests/evidence/d3-batch17/decisions.md` 编写"batch16 交付物自查表"章节，含：
  - 五项交付物逐项核对结果（item / status / reason / remediation / verdict）
  - 验收线5 未达标回归点显式定位：并发档=c=512 / 指标=P99 / 实测=200ms / 目标=≤100ms / 根因=令牌桶限 TPS 不限并发
  - 漏列项的漏列原因说明（已存在但未在 report.md 显式落盘的原因）
  - 缺失项的补齐内容摘要
- **涉及文件**：`tests/evidence/d3-batch17/decisions.md`（新增章节）
- **验证方式**：
  1. decisions.md 含"batch16 交付物自查表"章节
  2. 五项逐项有核对结果
  3. 验收线5 回归点定位完整（并发档/指标/实测/目标/根因五字段齐全）
  4. 漏列项有原因说明，缺失项有补齐摘要
- **依赖**：T17-SELFCHK-01, T17-SELFCHK-02

---

## 1. 并发限制器实现（模块 A：in_flight_limiter.go）

> 信号量约束在途并发上限 L，TryAcquire 非阻塞 / Release 原子，O(1) 无锁。
> 红线：TryAcquire 耗时 <0.1ms，不得进入 Propose → WAL → quorum 写路径热区。

### 1.1 T17-IFL-01：新建 InFlightLimiter 结构与构造
- [ ] **状态**：TODO
- **描述**：新建 `in_flight_limiter.go`，定义 `InFlightLimiter` 结构：
  - `cap int64`：容量（在途并发上限，配置后不变）
  - `current atomic.Int64`：当前占用槽位数
  - `shedCount atomic.Int64`：信号量拒载计数（in_flight_cap 原因）
  - `enabled atomic.Int32`：启用标志（0=关闭，1=开启）
  提供 `NewInFlightLimiter(cap int64) *InFlightLimiter` 构造函数。启动时校验 `cap > 0`，违规 `log.Fatal` 拒绝启动。从环境变量 `IN_FLIGHT_CAP`（默认 600，Little's Law 标定）和 `IN_FLIGHT_CAP_ENABLED`（默认 true）读取配置。
- **涉及文件**：`in_flight_limiter.go`（新增）
- **验证方式**：
  1. 编译通过
  2. `cap <= 0` 时 `log.Fatal` 拒绝启动
  3. 环境变量 `IN_FLIGHT_CAP=800` 读取正确，`cap=800`
  4. `IN_FLIGHT_CAP_ENABLED=false` 时 `enabled=0`
- **依赖**：无

### 1.2 T17-IFL-02：实现 TryAcquire/Release 非阻塞语义
- [ ] **状态**：TODO
- **描述**：实现信号量核心方法：
  - `TryAcquire() bool`：非阻塞尝试占用一个槽位。`enabled=0` 时直接返回 true（限流关闭）。`atomic.Load(&current) >= cap` 时立即返回 false（零等待，不占用槽位，不消耗令牌桶令牌）。否则 `atomic.Add(&current, 1)` 后再次校验是否超 cap（CAS 失败回退），成功返回 true，失败返回 false + `atomic.Add(&shedCount, 1)`。Fail-Open：若 current 出现异常负数（状态损坏），放行请求并记录告警日志（structured_log）。
  - `Release()`：释放一个槽位。下界 0 保护：`if current.Load() > 0 { current.Add(-1) }`，防止 current 负数。
  全程原子操作，无 `mu.Lock`，无 I/O，O(1) 耗时 <0.1ms。
- **涉及文件**：`in_flight_limiter.go`
- **验证方式**：
  1. `current < cap` 时 `TryAcquire()` 返回 true，current 递增
  2. `current >= cap` 时 `TryAcquire()` 返回 false，current 不变，shedCount 递增
  3. `Release()` 后 current 递减，current=0 时 Release 不产生负数
  4. `enabled=0` 时 `TryAcquire()` 恒返回 true
  5. 基准：`TryAcquire()` 耗时 <0.1ms，`Release()` 耗时 <50ns
  6. 无 `mu.Lock`，无 I/O
- **依赖**：T17-IFL-01

### 1.3 T17-IFL-03：实现 Current/Cap/ShedCount 状态查询
- [ ] **状态**：TODO
- **描述**：实现三个原子读取方法供 metrics gauge / counter 导出：
  - `Current() int64`：`atomic.Load(&current)`，当前在途数
  - `Cap() int64`：返回 `cap`，容量配置
  - `ShedCount() int64`：`atomic.Load(&shedCount)`，信号量拒载计数
  全程 `atomic.Load`，无锁，可被 metrics 采集器并发安全调用。
- **涉及文件**：`in_flight_limiter.go`
- **验证方式**：
  1. `Current()` 返回值与实际占用数一致
  2. `Cap()` 返回构造时配置值
  3. `ShedCount()` 返回拒载累计数
  4. 并发调用无 race（`go test -race`）
- **依赖**：T17-IFL-01

---

## 2. 三口径统计实现（模块 B：tri_stats.go）

> 三口径独立原子计数：成功 / 拒载(shed) / 真失败，shed 按 reason 细分。
> 红线：IncShed/IncSuccess/IncFail 仅在 propose handler 入口/出口调用，不进入 Propose → WAL → quorum 同步路径。

### 2.1 T17-TRI-01：新建 TriStats 结构与构造
- [ ] **状态**：TODO
- **描述**：新建 `tri_stats.go`，定义 `TriStats` 结构：
  - `successCount atomic.Int64`：成功（200）
  - `shedInFlight atomic.Int64`：拒载-在途已满（503, reason=in_flight_cap）
  - `shedRateLimited atomic.Int64`：拒载-令牌桶耗尽（503, reason=rate_limited）
  - `failCount atomic.Int64`：真失败（非 200 非 503）
  提供 `NewTriStats() *TriStats` 构造函数。四个计数器均 `atomic.Int64`，无锁并发安全。
- **涉及文件**：`tri_stats.go`（新增）
- **验证方式**：
  1. 编译通过
  2. 构造后四个计数器均为 0
  3. 内存占用 <100 字节（4 个 int64）
- **依赖**：无

### 2.2 T17-TRI-02：实现 IncSuccess/IncShed/IncFail + Snapshot
- [ ] **状态**：TODO
- **描述**：实现三口径埋点方法 + 原子快照：
  - `IncSuccess()`：`atomic.Add(&successCount, 1)`，Propose 成功返回 200 时调用
  - `IncShed(reason string)`：拒载计数 +1，按 reason 分别 +1。`reason="in_flight_cap"` → `shedInFlight`；`reason="rate_limited"` → `shedRateLimited`；其他 reason → 归入 `shedInFlight` + 告警日志（structured_log）
  - `IncFail()`：`atomic.Add(&failCount, 1)`，Propose 失败返回非 200 非 503 时调用
  - `Snapshot() (success, shedInFlight, shedRateLimited, fail int64)`：原子快照四值，供 metrics 导出 + 曲线采集
  全程 `atomic.Add` / `atomic.Load`，单次 <50ns，无锁无 I/O。
- **涉及文件**：`tri_stats.go`
- **验证方式**：
  1. `IncSuccess()` 后 `Snapshot()` 的 success 递增
  2. `IncShed("in_flight_cap")` 后 shedInFlight 递增，shedRateLimited 不变
  3. `IncShed("rate_limited")` 后 shedRateLimited 递增，shedInFlight 不变
  4. `IncFail()` 后 fail 递增
  5. `success + shedInFlight + shedRateLimited + fail = 总调用次数`
  6. 基准：单次 Inc* 耗时 <50ns
  7. 并发调用无 race
- **依赖**：T17-TRI-01

---

## 3. 双层准入协同实现（模块 C：main.go 改造）

> 信号量先判（约束在途 L）→ 令牌桶后判（约束速率 λ），两层均通过方进入 Propose 路径。
> 红线：满载快速拒载 503 + shed 标识；defer Release 保证槽位释放；拒载不进 Propose。

### 3.1 T17-GATE-01：propose handler 双层准入顺序改造
- [ ] **状态**：TODO
- **描述**：改造 `main.go` 的 `/raft/propose` handler（约 line 348-401），在现有令牌桶 `Allow()` 之前叠加信号量 `TryAcquire()`，形成双层准入：
  1. 第一层：`inFlightLimiter.TryAcquire()` → false 时 `triStats.IncShed("in_flight_cap")` + 返回 503 + `{"error":"shed","reason":"in_flight_cap"}`，不进 Propose，不占用槽位
  2. `defer inFlightLimiter.Release()`：保证槽位在 Propose 成功/失败/超时/panic 均释放
  3. 第二层：`rateLimiter.Allow()` → false 时 `triStats.IncShed("rate_limited")` + 返回 503 + `{"error":"shed","reason":"rate_limited"}`，defer Release 释放槽位
  4. 双层均通过 → 进入 Propose 路径
  顺序固定：信号量先判 → 令牌桶后判，禁止颠倒（先判信号量可在在途已满时零开销拒载，不消耗令牌桶令牌）。
- **涉及文件**：`main.go`（propose handler 入口段，约 line 348-401）
- **验证方式**：
  1. 在途未满 + 令牌充足 → 进入 Propose，返回 200
  2. 在途已满 → 立即返回 503 + reason="in_flight_cap"，不调用 RaftNode.Propose
  3. 在途未满 + 令牌不足 → 返回 503 + reason="rate_limited"，信号量槽位已释放（current 不泄漏）
  4. 顺序固定：信号量先于令牌桶（代码审查）
  5. defer Release 在 Propose 成功/失败/panic 均执行
- **依赖**：T17-IFL-02, T17-TRI-02

### 3.2 T17-GATE-02：满载快速拒载 503 + shed 标识 + reason 区分
- [ ] **状态**：TODO
- **描述**：改造 propose handler 拒载分支，将现有 429 + `{"error":"rate limited"}` 改为 503 + shed 标识：
  - 信号量拒载：`w.WriteHeader(http.StatusServiceUnavailable)` + `w.Write([]byte('{"error":"shed","reason":"in_flight_cap"}'))`
  - 令牌桶拒载：`w.WriteHeader(http.StatusServiceUnavailable)` + `w.Write([]byte('{"error":"shed","reason":"rate_limited"}'))`
  - 拒载响应极轻量（仅写 HTTP 头 + 短 JSON 体），单次拒载开销 <0.1ms
  - 与限流 429 语义区分：503=shed（准入拒载），429=rate limited（保留给其他限流场景）
  - 拒载不调用 `RaftNode.Propose`，不追加 WAL，不触发 quorum 复制
- **涉及文件**：`main.go`（propose handler 拒载分支）
- **验证方式**：
  1. 信号量拒载 → 503 + 响应体含 `"error":"shed"` + `"reason":"in_flight_cap"`
  2. 令牌桶拒载 → 503 + 响应体含 `"error":"shed"` + `"reason":"rate_limited"`
  3. 拒载响应耗时 <1ms（无排队等待）
  4. 拒载时不调用 `RaftNode.Propose`（代码审查 + 日志确认）
  5. 拒载时不追加 WAL、不触发 quorum 复制
- **依赖**：T17-GATE-01

### 3.3 T17-GATE-03：三口径统计全路径埋点
- [ ] **状态**：TODO
- **描述**：在 propose handler 全路径埋点 TriStats：
  - 200 返回 → `triStats.IncSuccess()` + `metricsCollector.RecordPropose()`
  - 503 返回 → `triStats.IncShed(reason)`（reason 由 GATE-02 区分）
  - 非 200 非 503 返回 → `triStats.IncFail()`
  - Propose panic → defer Release + recover → `triStats.IncFail()` + 500
  埋点位于 handler 入口/出口，不在 `RaftNode.Propose` 内部，不进入 Propose → WAL → quorum 同步路径。
- **涉及文件**：`main.go`（propose handler 全路径）
- **验证方式**：
  1. 200 响应 → `triStats.Snapshot()` 的 success 递增
  2. 503 响应 → shed 递增（按 reason 细分）
  3. 非 200 非 503 响应 → fail 递增
  4. Propose panic → fail 递增 + 信号量槽位释放
  5. `success + shedInFlight + shedRateLimited + fail = 总请求数`
  6. 代码审查：埋点不在 Propose → WAL → quorum 路径
- **依赖**：T17-GATE-02, T17-TRI-02

### 3.4 T17-GATE-04：/metrics 指标扩展（5 项新增）
- [ ] **状态**：TODO
- **描述**：扩展 `metrics_collector.go` 的 `RenderPrometheus()`，在现有 8 项指标基础上追加 5 项：
  1. `raft_shed_total{reason}`（counter，带 label：reason ∈ {in_flight_cap, rate_limited}，值 = shedInFlight / shedRateLimited）
  2. `raft_request_success_total`（counter，值 = TriStats.successCount）
  3. `raft_request_fail_total`（counter，值 = TriStats.failCount）
  4. `raft_in_flight_current`（gauge，值 = InFlightLimiter.Current()）
  5. `raft_in_flight_cap`（gauge，值 = InFlightLimiter.Cap()）
  每项含 `# HELP` 和 `# TYPE` 行。`NewMetricsCollector` 构造函数扩展入参，新增 `inFlight *InFlightLimiter` 和 `tri *TriStats` 引用。现有 8 项指标不得变更或删除（兼容性约束）。所有读取为 `atomic.Load`，无 `mu.Lock`，无 I/O。
- **涉及文件**：`metrics_collector.go`（RenderPrometheus 扩展 + 构造函数入参扩展）、`main.go`（构造 MetricsCollector 时传入 InFlightLimiter + TriStats）
- **验证方式**：
  1. `curl /metrics` 输出含 13 项指标（8 现有 + 5 新增），无遗漏
  2. `raft_shed_total{reason="in_flight_cap"}` 和 `raft_shed_total{reason="rate_limited"}` 分别可见
  3. `raft_in_flight_current` 值 = 当前信号量占用数
  4. `raft_in_flight_cap` 值 = 配置的 IN_FLIGHT_CAP
  5. 现有 8 项指标不变（值/类型/HELP 行均不变）
  6. Prometheus parser 可解析
  7. 单次渲染 <100ms
- **依赖**：T17-IFL-03, T17-TRI-02, T17-GATE-03

---

## 4. 过载曲线采集器（模块 D：overload_curve.go）

> 压测期间 1s 周期采样并落盘 overload_curve_c512.json，验证降级平滑性。
> 红线：采样 goroutine 独立运行，不阻塞 propose 路径，不进入写路径热区；相邻采样点拒载率变化 <15pp。

### 4.1 T17-CURVE-01：新建 OverloadCurveCollector 结构与周期采样
- [ ] **状态**：TODO
- **描述**：新建 `overload_curve.go`，定义：
  - `CurveSample` 结构：`timestamp string`（ISO 8601）、`concurrency int`、`tps float64`、`p50_ms float64`、`p99_ms float64`、`success int64`、`shed int64`、`fail int64`、`alive_nodes int`
  - `OverloadCurveCollector` 结构：`metrics *MetricsCollector`、`tri *TriStats`、`node *RaftNode`、`samples []CurveSample`、`mu sync.Mutex`（仅保护 samples 切片追加）、`stopCh chan struct{}`
  - `Start(interval time.Duration)`：启动后台采样 goroutine，每 interval（默认 1s）从 MetricsCollector 读取 TPS/P50/P99，从 TriStats 读取三口径快照，探测集群存活节点数（5 节点 HTTP 探活），构造 CurveSample 追加到 samples
  - `Stop(dir string) (string, error)`：关闭 stopCh 停止采样，将 samples 序列化为 JSON 写入 `dir/overload_curve_c512.json`，返回文件路径
  采样 goroutine 独立运行，panic 时已采样数据保留 + JSON 标注中断时间点。
- **涉及文件**：`overload_curve.go`（新增）
- **验证方式**：
  1. `Start(1s)` 后 samples 长度按 1s 递增
  2. 各字段值合理（TPS≥0, 0≤success_rate≤1, 1≤alive_nodes≤5）
  3. `Stop()` 后 goroutine 退出，返回 JSON 文件路径
  4. 采样 goroutine panic → 已采样数据保留 + JSON 含中断标注
  5. 采样不阻塞 propose 路径（基准对比：propose 耗时无增量）
- **依赖**：T17-GATE-04

### 4.2 T17-CURVE-02：JSON 落盘 + 字段完整性
- [ ] **状态**：TODO
- **描述**：实现 `Stop(dir string)` 的 JSON 落盘逻辑：
  - 文件路径：`dir/overload_curve_c512.json`（dir 默认 `tests/evidence/d3-batch17/`）
  - JSON 结构：`{"samples": [...], "meta": {"start_time": "...", "end_time": "...", "concurrency": 512, "interval_seconds": 1}}`
  - 每个 sample 含全部字段：timestamp（ISO 8601）/concurrency/tps/p50_ms/p99_ms/success/shed/fail/alive_nodes
  - 缺失字段的 JSON 判定不合格（验证时显式校验）
  - 文件权限 0644，目录不存在时自动创建
- **涉及文件**：`overload_curve.go`
- **验证方式**：
  1. `Stop()` 后文件 `overload_curve_c512.json` 存在
  2. JSON 可解析，含 samples 数组 + meta 对象
  3. 每个 sample 含全部 9 个字段（无缺失）
  4. timestamp 为 ISO 8601 格式
  5. 文件权限 ≤0644
- **依赖**：T17-CURVE-01

### 4.3 T17-CURVE-03：loadgen 集成曲线采集
- [ ] **状态**：TODO
- **描述**：在 `tools/loadgen/main.go` 集成 `OverloadCurveCollector`：
  - 压测启动时构造 `OverloadCurveCollector` + `Start(1s)` 周期采样
  - 压测结束时 `Stop("tests/evidence/d3-batch17/")` 落盘
  - c=512 过载压测自动产出 `overload_curve_c512.json`
  - 同时扩展 loadgen 三口径区分：按 HTTP 响应码分类 success(200)/shed(503)/fail(其他)，报告首行固定三口径栏位
  - 503 不重试（与 batch16 429 不重试语义一致），归入 shed 计数
- **涉及文件**：`tools/loadgen/main.go`（修改）
- **验证方式**：
  1. c=512 压测结束后生成 `overload_curve_c512.json`
  2. 曲线含完整时间序列（采样点数 ≈ 压测秒数）
  3. loadgen 输出含 success/shed/fail 三类计数
  4. 503 归入 shed，非 200 非 503 归入 fail
  5. 报告首行固定三口径栏位（成功/拒载/真失败）
  6. 503 不重试
- **依赖**：T17-CURVE-02

---

## 5. 单测护栏（三块全 PASS）

> 红线：并发限制器状态机 / 满载拒载路径 / 双层协同三块单测全 PASS，go test -race 无数据竞争。

### 5.1 T17-UT-01：并发限制器状态机单测
- [ ] **状态**：TODO
- **描述**：新建 `in_flight_limiter_test.go`，覆盖：
  - **状态机覆盖**：Idle → Normal → Full → Shed → Full → Normal → Idle 全状态转移
  - **满载拒载**：`current = cap` 时 `TryAcquire()` 返回 false，current 不变，shedCount 递增
  - **槽位释放**：`Release()` 后 current 递减；current=0 时 Release 不产生负数（下界保护）
  - **并发安全**：N 个 goroutine 并发 TryAcquire/Release，current 始终 ∈ [0, cap]，无 race
  - **Fail-Open**：current 负数（状态损坏）时 TryAcquire 放行 + 告警日志
  - **enabled=0**：限流关闭时 TryAcquire 恒返回 true
  - **基准**：`TryAcquire()` 耗时 <0.1ms，`Release()` 耗时 <50ns
  - 覆盖场景：空状态 / 正常占用 / 满载 / 拒载 / 释放 / 并发过载 / 状态损坏 / 限流关闭
- **涉及文件**：`in_flight_limiter_test.go`（新增）
- **验证方式**：
  1. `go test -run TestInFlightLimiter` 全 PASS
  2. `go test -race` 无数据竞争
  3. 状态机全状态转移覆盖
  4. 基准：TryAcquire <0.1ms / Release <50ns
- **依赖**：T17-IFL-03

### 5.2 T17-UT-02：三口径统计独立性单测
- [ ] **状态**：TODO
- **描述**：新建 `tri_stats_test.go`，覆盖：
  - **独立性**：`IncSuccess()` 不影响 shed/fail；`IncShed("in_flight_cap")` 不影响 success/fail/shedRateLimited；`IncShed("rate_limited")` 不影响 success/fail/shedInFlight；`IncFail()` 不影响 success/shed
  - **reason 细分**：`IncShed("in_flight_cap")` → shedInFlight 递增；`IncShed("rate_limited")` → shedRateLimited 递增；其他 reason → 归入 shedInFlight + 告警
  - **守恒律**：`success + shedInFlight + shedRateLimited + fail = 总调用次数`
  - **Snapshot 原子性**：并发 Inc* + Snapshot 无 race，Snapshot 返回值一致
  - **Prometheus 格式**：RenderPrometheus 输出含 `raft_shed_total{reason="in_flight_cap"}` 和 `raft_shed_total{reason="rate_limited"}` 分别可见，含 `# HELP` 和 `# TYPE` 行
  - **基准**：单次 Inc* 耗时 <50ns
  - 覆盖场景：空状态 / 仅成功 / 仅拒载 / 仅失败 / 混合 / 并发 / reason 异常
- **涉及文件**：`tri_stats_test.go`（新增）
- **验证方式**：
  1. `go test -run TestTriStats` 全 PASS
  2. `go test -race` 无数据竞争
  3. 三口径独立 + 守恒律验证通过
  4. Prometheus 格式合规
- **依赖**：T17-TRI-02, T17-GATE-04

### 5.3 T17-UT-03：双层协同单测
- [ ] **状态**：TODO
- **描述**：新建 `main_test.go` 或 `gate_test.go`，覆盖双层准入协同：
  - **双层顺序**：信号量先判 → 令牌桶后判（mock 令牌桶验证调用顺序）
  - **满载快速拒载**：信号量满载时立即返回 503 + reason="in_flight_cap"，不调用令牌桶 Allow、不调用 RaftNode.Propose
  - **令牌桶拒载槽位释放**：信号量通过 + 令牌桶拒载 → 返回 503 + reason="rate_limited"，信号量槽位已释放（current 不泄漏）
  - **Propose 成功**：双层均通过 → 调用 RaftNode.Propose → 200 + IncSuccess
  - **Propose 失败**：双层均通过 + Propose 返回 error → 非 200 + IncFail + 槽位释放
  - **Propose panic**：双层均通过 + Propose panic → defer Release + recover → IncFail + 500
  - **无死锁**：N 个 goroutine 并发调用双层准入，无死锁、无 goroutine 泄漏、无 panic
  - **拒载不进 Propose**：任一层拒绝时不调用 RaftNode.Propose、不追加 WAL、不触发 quorum
  - 覆盖场景：双层通过 / 信号量拒载 / 令牌桶拒载 / Propose 成功 / Propose 失败 / Propose panic / 并发过载
- **涉及文件**：`main_test.go` 或 `gate_test.go`（新增）
- **验证方式**：
  1. `go test -run TestDualGate` 全 PASS
  2. `go test -race` 无数据竞争
  3. 双层顺序固定（信号量先于令牌桶）
  4. 槽位无泄漏（current 始终 ∈ [0, cap]）
  5. 拒载不进 Propose（mock 验证 Propose 未被调用）
- **依赖**：T17-GATE-03

---

## 6. 验收测试（全部硬数字）

> 全部为用户签发的验收线原文，必须逐条达成。
> 红线：双报制 / retry 不计入服务端成绩 / 不调参刷数 / 不跳 quorum / 三口径独立统计。

### 6.1 T17-ACC-01：c=512 过载终审（验收线 2）
- [ ] **状态**：TODO
- **描述**：启动 5 节点集群，双层准入控制开启（IN_FLIGHT_CAP=600），c=512 过载压测 3min，验证：
  - **被服务请求 P99 ≤100ms**：通过双层准入的请求 P99 延迟 ≤100ms（核心指标，闭合 batch16 P99=200ms 未达标）
  - **三口径上报**：报告首行固定含成功/拒载/真失败三栏位，缺栏位判定不合格
  - **拒载率曲线平滑**：相邻采样点拒载率变化 <15 个百分点（非跳崖式跳变）
  - **5/5 存活**：过载压测期间 5 节点全部存活，无节点崩溃
  - **0 次 leader 异常切换**：过载降级而非崩溃，选举超时不缩（红线约束 3）
  - **无级联失败**：单节点拒载不触发其他节点崩溃/quorum 不可用
  - **过载曲线 JSON 落盘**：`tests/evidence/d3-batch17/overload_curve_c512.json` 生成，含时间序列数据
  - **quorum 复制正常**：限流不拒绝 Raft 内部通信，leader-follower 同步不受准入控制影响
  - **双报制**：服务端成绩 + 客户端成绩分别记录，客户端 retry 不计入服务端稳定性成绩
- **涉及文件**：全集群 + `tools/loadgen/main.go`
- **验证方式**：
  1. c=512 过载压测 3min → 被服务请求 P99 ≤100ms（硬数字）
  2. 报告首行含三口径栏位（成功/拒载/真失败），数值独立可辨
  3. 拒载率曲线相邻采样变化 <15pp
  4. 5/5 存活 / 0 次 leader 异常切换 / 无节点崩溃
  5. `overload_curve_c512.json` 生成，含完整时间序列
  6. quorum 复制正常（leader-follower 日志一致）
  7. 双报制记录服务端 + 客户端成绩
- **依赖**：T17-UT-01, T17-UT-02, T17-UT-03, T17-CURVE-03

### 6.2 T17-ACC-02：3min c=128 正常负载不误伤（验收线 3）
- [ ] **状态**：TODO
- **描述**：双层准入控制开启后，3min c=128 正常负载压测，验证不误伤：
  - **拒载率 =0**：shed=0，双层准入全通过
  - **TPS ≥7828.4**：权威基线 8240.4 的 95%
  - **P99 ≤50ms**：不劣化
  - **三口径全报**：成功=N，拒载=0，真失败=0，三栏位均有值
  - **5/5 存活**：正常负载无节点异常
  - **cap 不误伤验证**：c=128 稳态在途数 L_steady=531 < cap=600，信号量无误拒
  - **双报制**：服务端 + 客户端成绩分别记录
- **涉及文件**：全集群 + `tools/loadgen/main.go`
- **验证方式**：
  1. 3min c=128 复测 → 拒载率=0（shed=0）
  2. TPS ≥7828.4（硬数字）
  3. P99 ≤50ms（硬数字）
  4. 报告首行三口径全报（成功/拒载=0/真失败=0）
  5. 5/5 存活
  6. cap=600 不误伤 c=128（L_steady=531 < 600）
- **依赖**：T17-ACC-01

### 6.3 T17-ACC-03：阶梯复测 8→256→512 三口径全报（验收线 4）
- [ ] **状态**：TODO
- **描述**：阶梯复测 c=8 → c=256 → c=512 三档，每档三口径全报，对照权威基线无回归：
  - **c=8 档**：TPS ≥1501.9（batch16 基线），P99 ≤10ms，三口径全报
  - **c=256 档**：TPS ≥12726.3（batch16 基线），P99 ≤100ms，三口径全报
  - **c=512 档**：被服务请求 P99 ≤100ms（验收线5 闭合），三口径全报
  - **档间状态清理**：每档压测前确认信号量槽位全部释放、在途并发归零，无档间状态污染
  - **阶梯三口径栏位固定**：每档报告首行固定含三口径栏位，缺栏位判定不合格
  - **验收线5 闭合标注**：c=512 P99≤100ms → 验收线5 标注"闭合"，batch15 vs 16 vs 17 三列对照表显式标注
  - **双报制**：每档服务端 + 客户端成绩分别记录
- **涉及文件**：全集群 + `tools/loadgen/main.go`
- **验证方式**：
  1. c=8：TPS ≥1501.9 / P99 ≤10ms / 三口径全报
  2. c=256：TPS ≥12726.3 / P99 ≤100ms / 三口径全报
  3. c=512：被服务请求 P99 ≤100ms / 三口径全报 / 验收线5 标注"闭合"
  4. 每档档前状态干净（信号量槽位归零）
  5. 每档报告首行含三口径栏位
  6. 三列对照表显式标注验收线5 闭合
- **依赖**：T17-ACC-01, T17-ACC-02

---

## 7. 证书工程化（附带，时间富余才做）

> 红线：测试密钥不得入 git；生产部署前证书体系整体更换的风险写入 production_readiness.md。

### 7.1 T17-CERT-01：gen_certs.sh 脚本化 + 密钥出库
- [ ] **状态**：TODO
- **描述**：完善 `gen_certs.sh` 脚本化：
  - 一键产出 CA + 5 节点证书（无手动步骤）
  - 私钥权限 `chmod 600 *_key.pem`
  - 可选增加 SAN（Subject Alternative Name）支持 IP 地址校验
  - 测试密钥加入 `.gitignore`，git 历史中已误入的标注需 `git filter-branch` 或 BFG 清理
  - `git status` 无密钥文件
- **涉及文件**：`gen_certs.sh`（修改）、`.gitignore`（新增密钥路径）
- **验证方式**：
  1. 执行 `gen_certs.sh` → 产出 CA + 5 节点证书
  2. `ls -l` 私钥文件权限 ≤0600
  3. `.gitignore` 含密钥路径
  4. `git status` 无密钥文件
- **依赖**：无

### 7.2 T17-CERT-02：轮换演练文档
- [ ] **状态**：TODO
- **描述**：编写证书轮换演练文档，含：
  - 双证书并行窗口（T0~T4 阶段）
  - 轮换步骤：gen_certs → 分发 → 滚动重启 → 清理
  - 回滚方案：改回旧证书路径 + 重启
  - 验证步骤：mTLS 握手成功 + quorum 复制正常
- **涉及文件**：`docs/specs/overload_control/cert_rotation.md`（新增）或 design.md 补充章节
- **验证方式**：
  1. 文档含轮换窗口（双证书并行）
  2. 含轮换步骤（gen_certs → 分发 → 滚动重启 → 清理）
  3. 含回滚方案（改回旧证书路径 + 重启）
  4. 含验证步骤（mTLS 握手 + quorum 复制）
- **依赖**：T17-CERT-01

### 7.3 T17-CERT-03：production_readiness.md 清单更新
- [ ] **状态**：TODO
- **描述**：更新 `production_readiness.md`，新增证书整体更换风险声明：
  - "生产部署前必须整体更换证书体系"风险声明
  - 测试证书与生产证书的隔离要求
  - 证书过期监控 + 轮换触发条件
  - 引用轮换演练文档路径
- **涉及文件**：`production_readiness.md`（修改）
- **验证方式**：
  1. `production_readiness.md` 含"生产部署前必须整体更换证书体系"风险声明
  2. 含测试/生产证书隔离要求
  3. 含证书过期监控 + 轮换触发条件
  4. 引用轮换演练文档路径
- **依赖**：T17-CERT-02

---

## 8. 产物与交付

> 红线：报告首行固定三口径栏位；decisions.md 含 batch16 缺项自查表；commit + tag 打点。

### 8.1 T17-ART-01：report.md（三口径栏位固定首行）
- [ ] **状态**：TODO
- **描述**：编写 `tests/evidence/d3-batch17/report.md`，含：
  - **首行固定三口径栏位**：成功数/拒载数/真失败数（缺栏位判定不合格）
  - c=512 过载终审结果：被服务请求 P99 / 三口径 / 拒载率曲线 / 5/5 存活 / 0 次 leader 切换
  - 3min c=128 正常负载结果：拒载率=0 / TPS / P99
  - 阶梯复测结果：c=8/c=256/c=512 三档三口径全报
  - 过载曲线 JSON 路径引用
  - 双报制：服务端成绩 + 客户端成绩分别记录
- **涉及文件**：`tests/evidence/d3-batch17/report.md`（新增）
- **验证方式**：
  1. 报告首行含三口径栏位（成功/拒载/真失败）
  2. c=512 / c=128 / 阶梯复测结果齐全
  3. 双报制记录完整
  4. 引用 overload_curve_c512.json 路径
- **依赖**：T17-ACC-03

### 8.2 T17-ART-02：decisions.md（含 batch16 缺项自查表）
- [ ] **状态**：TODO
- **描述**：编写 `tests/evidence/d3-batch17/decisions.md`，含：
  - **batch16 交付物自查表**（五项逐项核对结果，T17-SELFCHK-03 已完成）
  - **in-flight cap 标定推导记录**：Little's Law 推导链路（λ=10617.2, W=50ms, L_steady=531, headroom 13%, cap=600）
  - **验收线5 闭合记录**：batch16 P99=200ms → batch17 P99≤100ms，根因/修复/验证三段
  - **设计决策记录**：双层准入顺序（信号量先判 → 令牌桶后判）的依据
  - **红线遵守记录**：11 条红线逐条核对结果
- **涉及文件**：`tests/evidence/d3-batch17/decisions.md`（新增/扩展）
- **验证方式**：
  1. 含 batch16 交付物自查表（五项逐项）
  2. 含 in-flight cap 标定推导记录
  3. 含验收线5 闭合记录（根因/修复/验证）
  4. 含设计决策记录
  5. 含 11 条红线逐条核对结果
- **依赖**：T17-SELFCHK-03, T17-ACC-03

### 8.3 T17-ART-03：过载曲线 JSON 落盘
- [ ] **状态**：TODO
- **描述**：确认 `tests/evidence/d3-batch17/overload_curve_c512.json` 已落盘（T17-CURVE-03 产出），含完整时间序列数据：
  - 时间戳 / 并发数 / TPS / P50 / P99 / 三口径（成功/拒载/真失败）/ 存活节点数
  - 相邻采样点 TPS/成功率/拒载率/P99 变化 <30%（无垂直跌落）
  - 相邻采样点拒载率变化 <15pp（平滑上升）
- **涉及文件**：`tests/evidence/d3-batch17/overload_curve_c512.json`（T17-CURVE-03 产出，此处确认落盘）
- **验证方式**：
  1. 文件存在 + JSON 可解析
  2. 含全部 9 个字段时间序列
  3. 相邻采样点变化 <30%（无垂直跌落）
  4. 拒载率相邻变化 <15pp（平滑上升）
- **依赖**：T17-ACC-01

### 8.4 T17-ART-04：batch15 vs 16 vs 17 三列对照表
- [ ] **状态**：TODO
- **描述**：在 `tests/evidence/d3-batch17/report.md` 中编写 batch15 vs 16 vs 17 三列对照表，逐指标对照：
  - TPS（c=128 / c=512 各档）
  - P99（c=128 / c=512 各档）
  - 成功率（三口径口径修正前后）
  - 拒载率（batch15/16 未统计 → batch17 新增）
  - 5/5 存活 / leader 切换次数
  - 验收线5 状态（batch16 未达标 → batch17 闭合）
  - 未达标项显式标注（红色/加粗）
- **涉及文件**：`tests/evidence/d3-batch17/report.md`（扩展章节）
- **验证方式**：
  1. 三列对照表存在（batch15 / batch16 / batch17）
  2. 每指标三批均有数值
  3. 未达标项显式标注
  4. 验收线5 状态：batch16 "未达标" → batch17 "闭合"
- **依赖**：T17-ART-01

### 8.5 T17-ART-05：commit + tag 打点
- [ ] **状态**：TODO
- **描述**：完成 batch17 全部任务后，执行 git 打点：
  - `git add -A` + `git commit -m "D3-batch17-overload: 双层准入控制 + 三口径统计 + batch16 自查补齐"`
  - `git tag v2.4-post-batch17`
  - commit message 含：本批新增文件清单 / 修改文件清单 / 验收线达成情况 / 红线遵守情况
- **涉及文件**：git 仓库
- **验证方式**：
  1. `git log -1` 含 commit message "D3-batch17-overload"
  2. `git tag -l v2.4-post-batch17` 存在
  3. commit message 含新增/修改文件清单 + 验收线达成 + 红线遵守
- **依赖**：T17-ART-04, T17-ART-02, T17-ART-03

---

## 9. 代码审查与红线核对

### 9.1 T17-REV-01：写路径热区隔离审查
- [ ] **状态**：TODO
- **描述**：代码审查确认所有 batch17 新增埋点（`InFlightLimiter.TryAcquire/Release` / `TriStats.IncSuccess/IncShed/IncFail`）和准入决策均不在 Propose → WAL → quorum 写路径同步路径中：
  - `TryAcquire()` / `Release()` 在 propose handler 入口/出口调用，单次 atomic 操作
  - `IncSuccess/IncShed/IncFail` 在 handler 返回前调用，不在 `RaftNode.Propose` 内部
  - `OverloadCurveCollector` 采样 goroutine 独立运行，不阻塞 propose
  - 无 `mu.Lock`（除 OverloadCurveCollector.samples 切片保护，仅采样 goroutine单写），无 I/O
- **涉及文件**：`in_flight_limiter.go`, `tri_stats.go`, `main.go`, `overload_curve.go`, `metrics_collector.go`
- **验证方式**：
  1. 代码审查：采集点仅用 `atomic.Load`/`atomic.Add`，无 `mu.Lock`（除曲线 samples），无 I/O
  2. 基准对比：单次 Propose 耗时增量 <1ms
  3. `TryAcquire()` 耗时 <0.1ms
  4. 准入决策不在 Propose → WAL → quorum 路径
- **依赖**：T17-ACC-01

### 9.2 T17-REV-02：红线约束逐条核对
- [ ] **状态**：TODO
- **描述**：逐条核对 11 条红线约束：
  1. 禁去 fsync：WAL/fsync 逻辑未变更
  2. 禁跳 quorum：双层准入不拒绝 Raft 内部通信（仅接入 /raft/propose）
  3. 禁缩选举超时：选举超时未变更
  4. 禁调参刷数：in-flight cap=600 一经标定不复测中调整，令牌桶配置继承不改
  5. pipeline 正确性三原则：pipeline 逻辑未变更
  6. 禁止改 proto：proto 定义未变更
  7. 成功率双报制：所有成功率按服务端 + 客户端双报
  8. retry 不计入服务端成绩：客户端 retry 标记排除
  9. 可观测性代码不进热区：单测验证 + 代码审查
  10. 三口径独立统计：shed 不计入 success/fail，双报栏位固定首行
  11. 测试密钥不得入 git：.gitignore 含密钥路径 + git status 无密钥
- **涉及文件**：全仓库
- **验证方式**：
  1. 11 条红线逐条核对通过
  2. 无红线违反记录
  3. 核对结果写入 decisions.md
- **依赖**：T17-ACC-03, T17-CERT-01

---

## 任务统计

| 模块 | 任务数 | 新增文件 | 修改文件 |
|------|--------|---------|---------|
| 0. batch16 自查 | 3 | 0 | tests/evidence/d3-batch17/decisions.md, docs/specs/production_hardening/tasks.md |
| 1. 并发限制器 | 3 | in_flight_limiter.go | 0 |
| 2. 三口径统计 | 2 | tri_stats.go | 0 |
| 3. 双层协同 | 4 | 0 | main.go, metrics_collector.go |
| 4. 过载曲线 | 3 | overload_curve.go | tools/loadgen/main.go |
| 5. 单测护栏 | 3 | in_flight_limiter_test.go, tri_stats_test.go, gate_test.go | 0 |
| 6. 验收测试 | 3 | 0 | 0 |
| 7. 证书工程化（附带） | 3 | cert_rotation.md | gen_certs.sh, .gitignore, production_readiness.md |
| 8. 产物交付 | 5 | report.md, decisions.md, overload_curve_c512.json | 0 |
| 9. 代码审查 | 2 | 0 | 0 |
| **合计** | **31** | **8 新增 + 3 测试** | **7 修改** |

**新增文件清单**：
1. `in_flight_limiter.go`（并发限制器/信号量）
2. `tri_stats.go`（三口径统计器）
3. `overload_curve.go`（过载曲线采集器）
4. `in_flight_limiter_test.go`（信号量单测）
5. `tri_stats_test.go`（三口径单测）
6. `gate_test.go` 或 `main_test.go`（双层协同单测）
7. `tests/evidence/d3-batch17/report.md`（验收报告）
8. `tests/evidence/d3-batch17/decisions.md`（决策记录 + 自查表）
9. `tests/evidence/d3-batch17/overload_curve_c512.json`（过载曲线 JSON）
10. `docs/specs/overload_control/cert_rotation.md`（证书轮换演练，附带）

**修改文件清单**：
1. `main.go`（propose handler 双层准入改造 + 三口径埋点 + MetricsCollector 构造入参扩展）
2. `metrics_collector.go`（RenderPrometheus 追加 5 项指标 + 构造函数入参扩展）
3. `tools/loadgen/main.go`（曲线采集集成 + 三口径区分 + 503 不重试）
4. `docs/specs/production_hardening/tasks.md`（T15-* 状态回填）
5. `gen_certs.sh`（私钥权限 + SAN，附带）
6. `.gitignore`（密钥路径，附带）
7. `production_readiness.md`（证书更换风险声明，附带）

**验收线映射**：
- 验收线 1（单测全 PASS）→ T17-UT-01 + T17-UT-02 + T17-UT-03
- 验收线 2（c=512 过载终审）→ T17-ACC-01
- 验收线 3（正常负载不误伤）→ T17-ACC-02
- 验收线 4（阶梯复测三口径全报）→ T17-ACC-03
- 验收线 5（batch16 交付物自查补齐）→ T17-SELFCHK-01 + T17-SELFCHK-02 + T17-SELFCHK-03

**关键路径**：T17-IFL-01 → T17-IFL-02 → T17-TRI-02 → T17-GATE-01 → T17-GATE-04 → T17-CURVE-01 → T17-CURVE-03 → T17-ACC-01 → T17-ACC-03 → T17-ART-05（最长依赖链 10 步）