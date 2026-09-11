# 生产化第一阶段编码任务清单（batch15）

> 批次：batch15
> 上游规格：docs/specs/production_hardening/spec.md
> 上游设计：docs/specs/production_hardening/design.md
> 基线参照：3min c=128 TPS=8240.4 / 成功率=100% / P99=50ms / 5/5 存活 / 0 次 leader 切换
> 红线约束：禁去 fsync / 禁跳 quorum / 禁缩选举超时 / 禁调参刷数 / pipeline 正确性三原则 / 禁止改 proto / 双报制 / retry 不计入服务端成绩 / 可观测性代码不得进入写路径热区
> 任务编号约定：T15-{模块}-{序号}，模块缩写 INFRA=基础埋点 / MET=metrics / RL=限流 / TLS=mTLS / AUTH=鉴权 / CURVE=曲线 / UT=单测 / ACC=验收 / DEPLOY=部署

---

## 任务依赖关系 DAG

```
T15-INFRA-01 (raft.go 心跳原子化)      ─┐
T15-INFRA-02 (raft.go 选举事件计数)    ─┼─→ T15-MET-01 ~ T15-MET-05 (metrics 模块)
T15-INFRA-03 (pipeline 埋点)           ─┘
                                          │
T15-RL-01 ~ T15-RL-06 (限流模块) ─────────┼─→ T15-MET-04 (限流触发计数导出)
                                          │
T15-TLS-01 ~ T15-TLS-06 (mTLS 模块) ──────┤
                                          │
T15-AUTH-01 ~ T15-AUTH-02 (鉴权模块) ─────┤
                                          │
T15-CURVE-01 ~ T15-CURVE-04 (曲线模块) ───┤   依赖 T15-MET-01
                                          │
                                          └─→ T15-DEPLOY-01 ~ T15-DEPLOY-03 (部署配置)
                                                  │
                                                  └─→ T15-UT-01 ~ T15-UT-03 (单测)
                                                          │
                                                          └─→ T15-ACC-01 ~ T15-ACC-04 (验收)
```

**并行机会**：
- T15-INFRA-* 完成后，T15-MET-* / T15-RL-* / T15-TLS-* / T15-AUTH-* 四组可并行开发
- T15-CURVE-* 依赖 T15-MET-01，可在 metrics 模块完成后启动
- T15-UT-* 三块单测可并行编写（对应三个新增模块）

---

## 1. 基础埋点设施改造（前置依赖）

> 为 metrics 采集提供原子化数据源，是 metrics 模块的前置依赖。
> 红线：埋点必须为单次 atomic 操作，不得进入 Propose → WAL → quorum 写路径热区。

### 1.1 T15-INFRA-01：心跳间隔原子化改造
- [ ] **状态**：TODO
- **描述**：将 `RaftNode.currentHeartbeatInterval` 从普通 `time.Duration` 字段改为 `atomic.Int64`（纳秒），统一所有写入点为 `atomic.StoreInt64`，读取点为 `atomic.LoadInt64`。grep 全部写入点确保无遗漏，避免并发数据竞争。
- **涉及文件**：`raft.go`（字段定义 + `adaptiveHeartbeatAdjust` + 心跳 goroutine 写入点）
- **验证方式**：
  1. `go vet` 无数据竞争告警
  2. `go test -race` 通过
  3. 单测：并发读写 `currentHeartbeatInterval` 无 race
- **依赖**：无

### 1.2 T15-INFRA-02：选举事件计数器埋点
- [ ] **状态**：TODO
- **描述**：在 `RaftNode` 新增 `electionEventCount atomic.Int64` 字段。在 `becomeLeader`/`becomeFollower`/`becomeCandidate` 三个状态转换方法以及 term 变更处（`advanceTerm` 或等价路径）调用 `atomic.AddInt64(&electionEventCount, 1)`。埋点位于状态转换方法入口，不在 Propose 同步路径。
- **涉及文件**：`raft.go`（结构体定义 + becomeLeader/becomeFollower/becomeCandidate + term 变更点）
- **验证方式**：
  1. 触发 leader 切换后 `atomic.Load` 读到递增值
  2. term 变化后计数递增
  3. 代码审查：埋点不在 Propose → WAL → quorum 路径
- **依赖**：无

### 1.3 T15-INFRA-03：compaction 计数与快照进度埋点
- [ ] **状态**：TODO
- **描述**：在 `SnapshotScheduler` 新增 `compactionCount atomic.Int64` 和 `snapshotProgress atomic.Int32`（0~1000 定点，采集时除 1000.0）。`Request()` 调用时 `atomic.StoreInt32(&snapshotProgress, 0)`；`executeSnapshot()` 成功后 `atomic.AddInt64(&compactionCount, 1)` + `atomic.StoreInt32(&snapshotProgress, 1000)`。快照在独立 goroutine 执行，不阻塞 OnCommit 写路径。
- **涉及文件**：`raft_pipeline.go`（SnapshotScheduler 结构 + Request + executeSnapshot）
- **验证方式**：
  1. 触发快照后 `compactionCount` 递增，值 = 实际 compaction 次数
  2. 快照执行中 `snapshotProgress` ∈ (0, 1000)
  3. 快照完成后 `snapshotProgress` = 1000
  4. 代码审查：埋点在独立 goroutine，不在 OnCommit 同步路径
- **依赖**：无

---

## 2. 统一 Prometheus /metrics 端点（模块 A：metrics_collector.go）

> 聚合 8 项指标，Prometheus 0.0.4 文本格式，纯原子读取零锁。
> 红线：采集代码不得进入写路径热区，单次采集 <100ms。

### 2.1 T15-MET-01：新建 MetricsCollector 结构与构造
- [ ] **状态**：TODO
- **描述**：新建 `metrics_collector.go`，定义 `MetricsCollector` 结构，持有 `node *RaftNode`、`scheduler *SnapshotScheduler`、`limiter *TokenBucketLimiter` 引用，以及 `proposeCount atomic.Int64`（TPS 滑动窗口计数）和 `windowStart atomic.Int64`（窗口起始纳秒）。提供 `NewMetricsCollector(node, scheduler, limiter) *MetricsCollector` 构造函数。
- **涉及文件**：`metrics_collector.go`（新增）
- **验证方式**：
  1. 编译通过
  2. 构造后字段非 nil
- **依赖**：T15-INFRA-01, T15-INFRA-02, T15-INFRA-03

### 2.2 T15-MET-02：实现 RenderPrometheus() 8 项指标渲染
- [ ] **状态**：TODO
- **描述**：实现 `RenderPrometheus() string`，输出 Prometheus 0.0.4 文本格式，包含全清单 8 项指标：
  1. `raft_tps`（gauge，滑动窗口差值 / 窗口秒数）
  2. `raft_request_duration_seconds`（histogram，复用 `globalLatency.prometheusText()`）
  3. `raft_heartbeat_interval_seconds`（gauge，`atomic.Load` 心跳纳秒 / 1e9）
  4. `raft_election_events_total`（counter，`atomic.Load`）
  5. `raft_compaction_count_total`（counter，`atomic.Load`）
  6. `raft_log_length`（gauge，`stats.RLock` + 读 LogCount + `RUnlock`）
  7. `raft_snapshot_progress`（gauge，`atomic.Load` / 1000.0）
  8. `raft_rate_limit_triggered_total`（counter，`limiter.TriggeredCount()`）
  每项指标含 `# HELP` 和 `# TYPE` 行。所有读取为 `atomic.Load` 或 `RLock`，无 `mu.Lock`，无 I/O。
- **涉及文件**：`metrics_collector.go`；复用 `latency_stats.go` 的 `prometheusText()`
- **验证方式**：
  1. `curl /metrics` 输出包含 8 项指标名，无遗漏
  2. 每指标含 `# HELP` 和 `# TYPE` 行
  3. Content-Type = `text/plain; version=0.0.4`
  4. Prometheus parser 可解析
  5. 单次渲染 <100ms
- **依赖**：T15-MET-01

### 2.3 T15-MET-03：实现 IncPropose() TPS 滑动窗口
- [ ] **状态**：TODO
- **描述**：实现 `IncPropose()`，仅执行 `atomic.AddInt64(&proposeCount, 1)`（单次原子加，<10ns）。TPS 计算在 `RenderPrometheus()` 中：读当前计数 C_now 和时间 T_now，与窗口起点 C_start/T_start 做差，`TPS = (C_now - C_start) / (T_now - T_start).Seconds()`，随后更新窗口起点。首次采集（无历史窗口）TPS=0 避免除零。`IncPropose()` 在 HTTP handler 返回 200 后调用，不在 `RaftNode.Propose` 内部。
- **涉及文件**：`metrics_collector.go`；`main.go`（propose handler 调用点）
- **验证方式**：
  1. `IncPropose()` 后 `raft_tps` 递增
  2. 基准对比：单次 `IncPropose()` 耗时 <10ns
  3. 代码审查：`IncPropose` 不在 Propose → WAL → quorum 同步路径
- **依赖**：T15-MET-01

### 2.4 T15-MET-04：main.go 注册 /metrics 端点
- [ ] **状态**：TODO
- **描述**：在 `main.go` 构造 `MetricsCollector` 实例，注册 `httpMux.HandleFunc("/metrics", ...)` handler，设置 `Content-Type: text/plain; version=0.0.4`，调用 `mc.RenderPrometheus()` 输出。现有 `/latency/metrics`、`/raft/stats` 等端点保持不变（兼容性约束）。
- **涉及文件**：`main.go`（HTTP mux 注册段）
- **验证方式**：
  1. `curl /metrics` 返回 200 + Prometheus 文本
  2. 现有 `/latency/metrics`、`/raft/stats` 响应不变
  3. `/metrics` 输出 ⊇ 现有端点全部指标
- **依赖**：T15-MET-02, T15-MET-03

### 2.5 T15-MET-05：现有指标聚合验证
- [ ] **状态**：TODO
- **描述**：验证 `/metrics` 聚合了现有分散端点的全部指标：`/latency/metrics` 的 `grpc_request_duration_seconds` histogram、`/raft/stats` 的 `consecutiveTimeouts`/`consecutiveSuccess`/`term`/`state`/`leaderID`、`/pipeline/stats` 的 compaction 相关指标。缺失项补入 `RenderPrometheus()`。
- **涉及文件**：`metrics_collector.go`（聚合逻辑）
- **验证方式**：
  1. `/metrics` 输出 ⊇ `/latency/metrics` + `/raft/stats` + `/pipeline/stats` 全部指标
  2. 指标值与原端点一致（同时刻采集）
- **依赖**：T15-MET-02

---

## 3. 自适应令牌桶限流器（模块 B：token_bucket_limiter.go）

> 令牌桶 O(1) 决策 + 自适应速率调整，仅接入客户端写请求。
> 红线：Allow() <0.1ms，不得拒绝 Raft 内部通信。

### 3.1 T15-RL-01：新建 TokenBucketLimiter 结构与构造
- [ ] **状态**：TODO
- **描述**：新建 `token_bucket_limiter.go`，定义 `TokenBucketLimiter` 结构：`capacity int64`（桶容量）、`rate atomic.Int64`（补充速率，令牌/秒）、`tokens atomic.Int64`（当前令牌数，定点小数 ×1000）、`triggeredCount atomic.Int64`（限流触发计数）、`stopCh chan struct{}`。提供 `NewTokenBucketLimiter(capacity, rate int64) *TokenBucketLimiter` 构造函数。启动时校验 capacity > 0 且 rate > 0，违规 `log.Fatal` 拒绝启动。从环境变量 `RATE_LIMIT_BURST`（默认 200）和 `RATE_LIMIT_RATE`（默认 10000）读取配置。
- **涉及文件**：`token_bucket_limiter.go`（新增）
- **验证方式**：
  1. 编译通过
  2. capacity=0 或 rate=0 时 `log.Fatal` 拒绝启动
  3. 环境变量读取正确
- **依赖**：无

### 3.2 T15-RL-02：实现 Allow() O(1) 原子决策
- [ ] **状态**：TODO
- **描述**：实现 `Allow() bool`：`atomic.Load` 令牌数，若 ≥1000（定点小数，1000=1.0 令牌）则 `atomic.Add(&tokens, -1000)` 返回 true；否则 `atomic.Add(&triggeredCount, 1)` 返回 false。全程原子操作，无 `mu.Lock`，无 I/O。Fail-Open：内部状态异常（令牌负数）时放行 + 告警日志。
- **涉及文件**：`token_bucket_limiter.go`
- **验证方式**：
  1. 桶满时 `Allow()` 返回 true
  2. 桶空时 `Allow()` 返回 false + `triggeredCount` 递增
  3. 基准：`Allow()` 耗时 <0.1ms
  4. 无 `mu.Lock`，无 I/O
- **依赖**：T15-RL-01

### 3.3 T15-RL-03：实现令牌补充 ticker goroutine
- [ ] **状态**：TODO
- **描述**：实现 `refillLoop()` 后台 goroutine，每 1ms tick：`atomic.Load(&rate)` 读当前补充速率，计算 `add = rate / 1000`（每 1ms 补充量），`atomic.Load(&tokens)` 读当前令牌，`newT = min(t + add, capacity * 1000)`，`atomic.Store(&tokens, newT)`。`stopCh` 关闭时退出 goroutine。
- **涉及文件**：`token_bucket_limiter.go`
- **验证方式**：
  1. 桶空后等待 refill，`Allow()` 恢复 true
  2. 令牌数不超过 `capacity * 1000`
  3. goroutine 随 `stopCh` 关闭退出
- **依赖**：T15-RL-01

### 3.4 T15-RL-04：实现 AdjustRate() 与 TriggeredCount()
- [ ] **状态**：TODO
- **描述**：实现 `AdjustRate(newRate int64)`：`atomic.Store(&rate, newRate)`，下限 1000 QPS。实现 `TriggeredCount() int64`：`atomic.Load(&triggeredCount)`，供 MetricsCollector 采集。
- **涉及文件**：`token_bucket_limiter.go`
- **验证方式**：
  1. `AdjustRate(5000)` 后速率降为 5000
  2. `AdjustRate(0)` 被钳位为 1000（下限）
  3. `TriggeredCount()` 返回当前触发计数
- **依赖**：T15-RL-01

### 3.5 T15-RL-05：实现 AdaptiveFeedback 自适应反馈 goroutine
- [ ] **状态**：TODO
- **描述**：在 `token_bucket_limiter.go` 新增 `AdaptiveFeedback` 结构：`limiter *TokenBucketLimiter`、`node *RaftNode`、`sampleInterval time.Duration`（默认 100ms，env `ADAPTIVE_SAMPLE_INTERVAL`）、`highLatencyMs int64`（默认 80，env `QUORUM_LATENCY_HIGH_MS`）、`lowLatencyMs int64`（默认 40，env `QUORUM_LATENCY_LOW_MS`）。`Start()` 启动采样 goroutine：每 `sampleInterval` 采样 quorum 提交延迟，延迟 > `highLatencyMs` → `AdjustRate(rate * 0.7)`（降载因子 env `ADAPTIVE_DECAY_FACTOR`）；延迟 < `lowLatencyMs` → `AdjustRate(rate * 1.1)`（恢复因子 env `ADAPTIVE_RECOVER_FACTOR`，上限初始速率）。滞回设计避免震荡。`Stop()` 关闭 goroutine。
- **涉及文件**：`token_bucket_limiter.go`
- **验证方式**：
  1. quorum 延迟 >80ms → 速率 ×0.7
  2. quorum 延迟 <40ms → 速率 ×1.1（上限初始值）
  3. 延迟在 40~80ms 之间速率不变（滞回）
  4. 速率不低于 1000 QPS 下限
- **依赖**：T15-RL-04

### 3.6 T15-RL-06：main.go 接入 /raft/propose 限流
- [ ] **状态**：TODO
- **描述**：在 `main.go` 构造 `TokenBucketLimiter` 和 `AdaptiveFeedback` 实例，`/raft/propose` handler 入口调用 `limiter.Allow()`：返回 false 时 `http.Error(w, '{"error":"rate limited"}', 429)` 并 return，不进入 Propose；返回 true 时正常处理。限流仅接入 `/raft/propose`，**不接入** Raft 内部 gRPC 通信（AppendEntries/RequestVote），确保 quorum 复制不受限流影响。
- **涉及文件**：`main.go`（propose handler 入口）
- **验证方式**：
  1. 限流触发时 `/raft/propose` 返回 429
  2. 限流未触发时正常处理
  3. 过载压测期间 quorum 复制正常，leader-follower 同步不受限流影响
  4. 节点间 gRPC 通信无限流拦截
- **依赖**：T15-RL-02, T15-RL-05

---

## 4. mTLS 证书体系（模块 C：tls_config.go）

> 统一证书加载，生产 Fail-Closed，开发跳过，TLS1.2 + ClientAuth=RequireAndVerifyClientCert。
> 红线：禁止明文降级（生产），禁止 InsecureSkipVerify。

### 4.1 T15-TLS-01：新建 LoadMTLSConfig 证书加载器
- [ ] **状态**：TODO
- **描述**：新建 `tls_config.go`，实现 `LoadMTLSConfig(role string) (*tls.Config, error)`。从环境变量 `TLS_CERT_FILE`/`TLS_KEY_FILE`/`TLS_CA_FILE`（客户端额外 `TLS_CLIENT_CERT_FILE`/`TLS_CLIENT_KEY_FILE`）读取证书路径。加载 cert/key/ca，构建 `tls.Config`：`MinVersion: tls.VersionTLS12`、`InsecureSkipVerify: false`。server 模式设置 `ClientAuth: tls.RequireAndVerifyClientCert` + `RootCAs`。客户端设置 `RootCAs` + `Certificates`。
- **涉及文件**：`tls_config.go`（新增）
- **验证方式**：
  1. 合法证书加载成功，返回 `tls.Config`
  2. `MinVersion` = TLS1.2
  3. `InsecureSkipVerify` = false
  4. server 模式 `ClientAuth` = `RequireAndVerifyClientCert`
- **依赖**：无

### 4.2 T15-TLS-02：实现生产 Fail-Closed / 开发模式跳过
- [ ] **状态**：TODO
- **描述**：在 `LoadMTLSConfig` 中增加模式判断：`DEV_MODE` 环境变量未设或 =0 → 生产模式，证书缺失/加载失败返回 error（调用方 `log.Fatal` 拒绝启动，不降级明文）；`DEV_MODE=1` → 开发模式，证书缺失返回 `nil, nil`（调用方跳过 TLS，使用 `insecure.NewCredentials()`）。启动时打印模式：`[mTLS] 生产模式：强制 mTLS` 或 `[mTLS] 开发模式：跳过 mTLS（仅限开发）`。
- **涉及文件**：`tls_config.go`
- **验证方式**：
  1. 生产模式 + 证书缺失 → 返回 error，调用方 `log.Fatal`
  2. 生产模式 + 证书损坏 → 返回 error
  3. `DEV_MODE=1` + 证书缺失 → 返回 nil, nil
  4. 不降级为明文（生产模式）
- **依赖**：T15-TLS-01

### 4.3 T15-TLS-03：实现 verifyPeerCert 证书校验回调
- [ ] **状态**：TODO
- **描述**：实现 `verifyPeerCert(rawCerts [][]byte) error` 自定义校验回调，校验对端证书：CA 签名验证（使用集群 CA）、有效期验证（NotBefore/NotAfter）、CN 匹配（可选，校验 CN=节点 ID）。设置到 `tls.Config.VerifyPeerCertificate`。
- **涉及文件**：`tls_config.go`
- **验证方式**：
  1. 合法证书 → 校验通过
  2. 过期证书 → 校验失败
  3. CA 不匹配 → 校验失败
  4. 自签名证书（非集群 CA）→ 被拒绝
- **依赖**：T15-TLS-01

### 4.4 T15-TLS-04：改造 grpc_server.go 服务端 mTLS
- [ ] **状态**：TODO
- **描述**：改造 `grpc_server.go` 服务端证书加载逻辑，替换内联 `tls.LoadX509KeyPair` 为 `LoadMTLSConfig("server")`。生产模式 error 时 `log.Fatal` 拒绝启动。服务端 `tls.Config` 增加 `ClientAuth: tls.RequireAndVerifyClientCert` + `RootCAs`（由 LoadMTLSConfig 提供）。删除原"证书不存在时降级为明文"逻辑（line 129 附近）。
- **涉及文件**：`grpc_server.go`（服务端证书加载段，约 line 119-138）
- **验证方式**：
  1. 生产模式 + 证书缺失 → 启动失败（不降级明文）
  2. 服务端要求客户端证书（`RequireAndVerifyClientCert`）
  3. 合法客户端证书 → 握手成功
  4. 无客户端证书 → 握手拒绝
- **依赖**：T15-TLS-02, T15-TLS-03

### 4.5 T15-TLS-05：改造 grpc_server.go 客户端 mTLS
- [ ] **状态**：TODO
- **描述**：改造 `grpc_server.go` 客户端证书加载逻辑，替换内联加载为 `LoadMTLSConfig("client")`。客户端 `tls.Config` 设置 `RootCAs`（CA 校验）+ `Certificates`（客户端证书）。删除原"三变量不全 → insecure.NewCredentials()"降级逻辑（line 297 附近），生产模式 Fail-Closed。
- **涉及文件**：`grpc_server.go`（客户端证书加载段，约 line 273-298）
- **验证方式**：
  1. 生产模式 + 客户端证书缺失 → 启动失败
  2. 合法客户端证书 → 与服务端握手成功
  3. `DEV_MODE=1` → 使用 `insecure.NewCredentials()`（仅开发）
- **依赖**：T15-TLS-02, T15-TLS-04

### 4.6 T15-TLS-06：gen_certs.sh 私钥权限与 SAN
- [ ] **状态**：TODO
- **描述**：修改 `gen_certs.sh`，在生成私钥后显式 `chmod 600 *_key.pem`（确保权限 ≤0600）。可选增加 SAN（Subject Alternative Name）支持 IP 地址校验，便于 mTLS 校验节点 IP。
- **涉及文件**：`gen_certs.sh`
- **验证方式**：
  1. `ls -l` 私钥文件权限 ≤0600
  2. 证书含 SAN（如启用）可校验节点 IP
- **依赖**：无

---

## 5. 客户端鉴权中间件（模块 D：auth_middleware.go）

> Bearer Token + subtle.ConstantTimeCompare，仅覆盖 /raft/propose 和 /raft/get。
> 红线：鉴权在 HTTP Handler 层完成，不进入 Propose 同步路径。

### 5.1 T15-AUTH-01：新建 AuthMiddleware 鉴权中间件
- [ ] **状态**：TODO
- **描述**：新建 `auth_middleware.go`，实现 `AuthMiddleware(next http.HandlerFunc) http.HandlerFunc`。启动时读取 `RAFT_API_TOKEN` 环境变量：未设则鉴权关闭 + 启动告警日志 `RAFT_API_TOKEN 未设置，鉴权关闭（仅限开发模式）`，直接放行；已设则包装 handler：校验 `Authorization` Header，缺失 → 401 `凭证缺失`；非 Bearer 格式 → 401 `凭证格式错误`；token 不匹配 → 401 `凭证无效`（使用 `subtle.ConstantTimeCompare` 恒定时间比较防时序攻击）。401 响应不带 `WWW-Authenticate` 头。
- **涉及文件**：`auth_middleware.go`（新增）
- **验证方式**：
  1. 无 Authorization Header → 401 `凭证缺失`
  2. 非 Bearer 格式 → 401 `凭证格式错误`
  3. token 不匹配 → 401 `凭证无效`
  4. token 匹配 → 正常处理
  5. `RAFT_API_TOKEN` 未设 → 鉴权关闭 + 告警日志
  6. 恒定时间比较（防时序侧信道）
- **依赖**：无

### 5.2 T15-AUTH-02：main.go 接入鉴权中间件
- [ ] **状态**：TODO
- **描述**：在 `main.go` 将 `/raft/propose` 和 `/raft/get` handler 用 `AuthMiddleware` 包装：`httpMux.HandleFunc("/raft/propose", AuthMiddleware(proposeHandler))`，`httpMux.HandleFunc("/raft/get", AuthMiddleware(getHandler))`。鉴权在 HTTP Handler 层完成，不进入 `RaftNode.Propose`。与幂等 token 正交：鉴权校验凭证 → 幂等 token 校验重复，两者独立。节点间 gRPC 通信无 Authorization Header（由 mTLS 保护）。
- **涉及文件**：`main.go`（handler 注册段）
- **验证方式**：
  1. 无凭证 POST `/raft/propose` → 401
  2. 无凭证 GET `/raft/get` → 401
  3. 带有效凭证 → 正常处理
  4. 带鉴权 + idem_token → 正常去重
  5. 节点间 gRPC 通信无鉴权 Header → 正常通信
- **依赖**：T15-AUTH-01

---

## 6. 过载行为曲线采集器（模块 E：overload_curve.go）

> 压测期间周期采样并落盘曲线报告，验证降级平滑性。
> 红线：相邻采样点成功率跌幅 <15%，曲线无垂直跌落。

### 6.1 T15-CURVE-01：新建 OverloadCurveCollector 结构
- [ ] **状态**：TODO
- **描述**：新建 `overload_curve.go`，定义 `OverloadCurveCollector` 结构：`samples []CurveSample`、`outputPath string`、`mc *MetricsCollector`、`stopCh chan struct{}`。定义 `CurveSample` 结构：`timestamp time.Time`、`concurrency int`、`tps float64`、`p50Ms float64`、`p99Ms float64`、`successRate float64`、`aliveNodes int`。提供构造函数。
- **涉及文件**：`overload_curve.go`（新增）
- **验证方式**：
  1. 编译通过
  2. 构造后字段非 nil
- **依赖**：T15-MET-01

### 6.2 T15-CURVE-02：实现 Sample() 周期采样
- [ ] **状态**：TODO
- **描述**：实现 `Sample()` 方法：从 `MetricsCollector` 读取当前 TPS/P50/P99，从压测工具读取当前并发数和成功率，探测集群存活节点数（5 节点 HTTP 探活），构造 `CurveSample` 追加到 `samples`。提供 `Start(interval time.Duration)` 启动周期采样 goroutine（建议 1s 或 5s 间隔），`Stop()` 关闭。
- **涉及文件**：`overload_curve.go`
- **验证方式**：
  1. 采样后 `samples` 长度递增
  2. 各字段值合理（TPS≥0, 0≤successRate≤1, 1≤aliveNodes≤5）
  3. goroutine 随 `Stop()` 退出
- **依赖**：T15-CURVE-01

### 6.3 T15-CURVE-03：实现 Flush() JSON 落盘
- [ ] **状态**：TODO
- **描述**：实现 `Flush() error`：将 `samples` 序列化为 JSON 写入 `outputPath`（如 `overload_curve_c512.json`），含时间序列数据（时间戳/并发/TPS/P50/P99/成功率/存活节点数）。文件格式符合 spec §6.5 约束。
- **涉及文件**：`overload_curve.go`
- **验证方式**：
  1. `Flush()` 后文件存在
  2. JSON 可解析，含全部字段
  3. 时间戳为 ISO 8601 格式
- **依赖**：T15-CURVE-02

### 6.4 T15-CURVE-04：loadgen 集成曲线采集
- [ ] **状态**：TODO
- **描述**：在 `tools/loadgen/main.go` 集成 `OverloadCurveCollector`：压测启动时 `Start(1s)` 周期采样，压测结束时 `Stop()` + `Flush()` 落盘。c=512 过载压测自动产出 `overload_curve_c512.json`。
- **涉及文件**：`tools/loadgen/main.go`（修改）
- **验证方式**：
  1. c=512 压测结束后生成 `overload_curve_c512.json`
  2. 曲线含完整时间序列
  3. 相邻采样点成功率跌幅 <15%
- **依赖**：T15-CURVE-03

---

## 7. 单测护栏（三块全 PASS）

> 红线：metrics 埋点 / 限流器状态机 / 证书校验 三块单测全 PASS。

### 7.1 T15-UT-01：metrics 埋点单测
- [ ] **状态**：TODO
- **描述**：新建 `metrics_collector_test.go`，覆盖：
  - `RenderPrometheus()` 输出包含 8 项指标名
  - 每项指标含 `# HELP` 和 `# TYPE` 行
  - Prometheus 格式合规（Content-Type、parser 可解析）
  - `IncPropose()` 后 `raft_tps` 递增
  - `electionEventCount` 在 term 变化时递增
  - `compactionCount` 在快照完成后递增
  - `snapshotProgress` 在快照期间 ∈ (0, 1)
  - 采集开销基准对比：`IncPropose()` 耗时 <10ns
  - 覆盖场景：空状态 / 正常负载 / 高负载 / 指标溢出
- **涉及文件**：`metrics_collector_test.go`（新增）
- **验证方式**：
  1. `go test -run TestMetricsCollector` 全 PASS
  2. `go test -race` 无数据竞争
- **依赖**：T15-MET-05

### 7.2 T15-UT-02：限流器状态机单测
- [ ] **状态**：TODO
- **描述**：新建 `token_bucket_limiter_test.go`，覆盖：
  - 桶满时 `Allow()` 返回 true
  - 桶空时 `Allow()` 返回 false + `triggeredCount`++
  - 令牌补充：等待 refill 后 `Allow()` 恢复 true
  - 持续过载：连续 `Allow()` 部分拒绝部分通过
  - `AdjustRate()`：降速后拒绝率上升
  - 自适应反馈：quorum 延迟 >80ms → 速率 ×0.7
  - 自适应恢复：quorum 延迟 <40ms → 速率 ×1.1
  - 滞回：延迟在 40~80ms 之间速率不变
  - Fail-Open：内部状态异常时放行
  - `Allow()` 耗时 <0.1ms
  - 覆盖场景：桶满 / 桶空 / 恢复 / 持续过载 / 降载 / 恢复 / 震荡
- **涉及文件**：`token_bucket_limiter_test.go`（新增）
- **验证方式**：
  1. `go test -run TestTokenBucketLimiter` 全 PASS
  2. `Allow()` 耗时基准 <0.1ms
- **依赖**：T15-RL-06

### 7.3 T15-UT-03：证书校验单测
- [ ] **状态**：TODO
- **描述**：新建 `tls_config_test.go`，覆盖：
  - 合法证书加载成功
  - 无证书（生产模式）返回 error
  - 无证书（`DEV_MODE=1`）返回 nil
  - 过期证书握手失败
  - CA 不匹配握手失败
  - 自签名证书（非集群 CA）被拒绝
  - `InsecureSkipVerify` = false
  - 私钥权限 ≤0600
  - 服务端 `ClientAuth` = `RequireAndVerifyClientCert`
  - 覆盖场景：合法 / 无证书 / 过期 / CA不匹配 / 自签名 / 开发模式
- **涉及文件**：`tls_config_test.go`（新增）；依赖 `gen_certs.sh` 产出测试证书
- **验证方式**：
  1. `go test -run TestMTLSConfig` 全 PASS
  2. 正负路径均覆盖
- **依赖**：T15-TLS-06

---

## 8. 部署配置与文档

### 8.1 T15-DEPLOY-01：docker-compose 证书挂载与环境变量
- [ ] **状态**：TODO
- **描述**：修改 `tests/deploy/docker-compose-5node.yml`，为 5 个节点服务增加证书目录挂载（`./certs:/certs:ro`），设置环境变量 `TLS_CERT_FILE`/`TLS_KEY_FILE`/`TLS_CA_FILE`/`DEV_MODE`/`RAFT_API_TOKEN`/`RATE_LIMIT_BURST`/`RATE_LIMIT_RATE`。
- **涉及文件**：`tests/deploy/docker-compose-5node.yml`（修改）
- **验证方式**：
  1. `docker-compose up` 5 节点正常启动
  2. 证书文件正确挂载到各节点
  3. 环境变量正确传递
- **依赖**：T15-TLS-05, T15-AUTH-02, T15-RL-06

### 8.2 T15-DEPLOY-02：deploy.env 配置项补全
- [ ] **状态**：TODO
- **描述**：修改 `tests/deploy/deploy.env`，补全 batch15 新增配置项及默认值：
  - `RATE_LIMIT_BURST=200`
  - `RATE_LIMIT_RATE=10000`
  - `ADAPTIVE_SAMPLE_INTERVAL=100ms`
  - `QUORUM_LATENCY_HIGH_MS=80`
  - `QUORUM_LATENCY_LOW_MS=40`
  - `ADAPTIVE_DECAY_FACTOR=0.7`
  - `ADAPTIVE_RECOVER_FACTOR=1.1`
  - `DEV_MODE=0`
  - `RAFT_API_TOKEN=`（生产必须设置）
  - `TLS_CERT_FILE`/`TLS_KEY_FILE`/`TLS_CA_FILE` 路径
- **涉及文件**：`tests/deploy/deploy.env`（修改）
- **验证方式**：
  1. 配置项齐全，默认值符合 design §2.1.2 配置表
  2. 生产模式 `DEV_MODE=0` 强制 mTLS
- **依赖**：无

### 8.3 T15-DEPLOY-03：密钥轮换方案文档
- [ ] **状态**：TODO
- **描述**：在 design.md 或独立文档写明密钥轮换方案（含双证书并行窗口、轮换步骤、回滚方案），内容参照 design §2.4.3 的轮换窗口 T0~T4 和回滚方案。
- **涉及文件**：`docs/specs/production_hardening/design.md`（补充章节）或独立轮换文档
- **验证方式**：
  1. 文档含轮换窗口（双证书并行）
  2. 含轮换步骤（gen_certs → 分发 → 滚动重启 → 清理）
  3. 含回滚方案（改回旧证书路径 + 重启）
- **依赖**：无

---

## 9. 集成验收（5 条硬数字）

> 全部为用户签发的验收线原文，必须逐条达成。
> 红线：双报制 / retry 不计入服务端成绩 / 不调参刷数 / 不跳 quorum。

### 9.1 T15-ACC-01：/metrics 端点 + 采集开销复测（验收线 1）
- [ ] **状态**：TODO
- **描述**：启动 5 节点集群，`curl /metrics` 验证 8 项指标齐全且 Prometheus 格式合规。开启全清单 metrics 采集后，3min c=128 基准复测：TPS ≥8100（基线 8240.4 的 98%）/ 双报成功率 ≥99.9% / P99 ≤50ms / 5/5 存活。采集开启前后基准对比 → TPS 衰减 <2%，P99 无劣化。
- **涉及文件**：全集群
- **验证方式**：
  1. `/metrics` 输出 8 项指标，格式合规
  2. 3min c=128 复测：TPS ≥8100 / 成功率 ≥99.9% / P99 ≤50ms / 5/5 存活
  3. 采集开销：TPS 衰减 <2%，P99 增量 <1ms
  4. 双报制：服务端成绩 + 客户端成绩
- **依赖**：T15-MET-04, T15-MET-05, T15-DEPLOY-01

### 9.2 T15-ACC-02：限流过载压测 + 曲线落盘（验收线 2）
- [ ] **状态**：TODO
- **描述**：限流开启后 c=512 过载压测 3min：系统降级而非崩溃——P99 ≤100ms 且成功率曲线平滑（相邻采样点跌幅 <15%）、无级联失败（0 次 leader 切换 / 5/5 存活 / 无节点崩溃）、过载行为曲线落盘（`overload_curve_c512.json` 含时间序列数据）。验证限流不拒绝 Raft 内部通信（quorum 复制正常）。
- **涉及文件**：全集群 + `tools/loadgen/main.go`
- **验证方式**：
  1. c=512 过载压测 3min：P99 ≤100ms
  2. 成功率曲线平滑，相邻采样点跌幅 <15%
  3. 0 次 leader 切换 / 5/5 存活 / 无节点崩溃
  4. `overload_curve_c512.json` 生成，含时间序列
  5. quorum 复制正常，leader-follower 同步不受限流影响
- **依赖**：T15-RL-06, T15-CURVE-04, T15-DEPLOY-01

### 9.3 T15-ACC-03：mTLS 正负路径 + 开销复测（验收线 3）
- [ ] **状态**：TODO
- **描述**：
  - **正路径**：mTLS 开启后 5/5 节点互通正常，leader 选举正常，读写正常，0 次 leader 异常切换。mTLS 开销复测 3min c=128：TPS ≥7800（基线 ~95%）。
  - **负路径**：启动 4 个带证书节点形成正常集群，启动 1 个无证书节点尝试 ConnectAll 入簇，预期被拒绝（`RequireAndVerifyClientCert`），记录拒绝事件，现有 4 节点集群不受影响（4/4 存活，quorum=3 维持）。
- **涉及文件**：全集群 + `gen_certs.sh`
- **验证方式**：
  1. 正路径：5/5 节点 mTLS 互通 / leader 选举正常 / 读写正常
  2. 正路径：3min c=128 TPS ≥7800
  3. 负路径：无证书节点入簇被拒绝，日志记录拒绝事件
  4. 负路径：现有 4 节点集群不受影响，4/4 存活
  5. 抓包可见加密流量（非明文）
- **依赖**：T15-TLS-05, T15-TLS-06, T15-DEPLOY-01

### 9.4 T15-ACC-04：阶梯复测无回归（验收线 5）
- [ ] **状态**：TODO
- **描述**：阶梯复测 8/16/32/64/128/256 各档，对照 batch14 基线确认无性能回归：各档 TPS/P99/成功率不劣于 batch14 基线。双报制记录服务端成绩 + 客户端成绩，客户端 retry 不计入服务端稳定性成绩。
- **涉及文件**：全集群 + `tools/loadgen/main.go`
- **验证方式**：
  1. 8/16/32/64/128/256 各档 TPS 不劣于 batch14 基线
  2. 各档 P99 不劣于 batch14 基线
  3. 各档成功率不劣于 batch14 基线
  4. 双报制：服务端 + 客户端成绩分别记录
  5. retry 不计入服务端成绩
- **依赖**：T15-ACC-01, T15-ACC-02, T15-ACC-03

---

## 10. 代码审查与红线核对

### 10.1 T15-REV-01：写路径热区隔离审查
- [ ] **状态**：TODO
- **描述**：代码审查确认所有 metrics 埋点（`IncPropose`/`electionEventCount`/`compactionCount`/`snapshotProgress`）和限流决策（`Allow()`）均不在 Propose → WAL → quorum 写路径同步路径中。`IncPropose` 在 HTTP handler 返回后调用，`Allow()` 在 handler 入口调用，埋点均为单次 atomic 操作。
- **涉及文件**：`metrics_collector.go`, `token_bucket_limiter.go`, `main.go`, `raft.go`, `raft_pipeline.go`
- **验证方式**：
  1. 代码审查：采集点仅用 `atomic.Load`/`atomic.Add`，无 `mu.Lock`，无 I/O
  2. 基准对比：单次 Propose 耗时增量 <1ms
  3. `Allow()` 耗时 <0.1ms
- **依赖**：T15-ACC-01

### 10.2 T15-REV-02：红线约束逐条核对
- [ ] **状态**：TODO
- **描述**：逐条核对 9 条红线约束：
  1. 禁去 fsync：WAL/fsync 逻辑未变更
  2. 禁跳 quorum：限流不拒绝 Raft 内部通信
  3. 禁缩选举超时：选举超时未变更
  4. 禁调参刷数：限流阈值/mTLS 配置一经确定不复测中调整
  5. pipeline 正确性三原则：pipeline 逻辑未变更，仅新增埋点
  6. 禁止改 proto：proto 定义未变更
  7. 双报制：所有成功率按服务端 + 客户端双报
  8. retry 不计入服务端成绩：客户端 retry 标记排除
  9. 可观测性代码不进热区：单测验证 + 代码审查
- **涉及文件**：全仓库
- **验证方式**：
  1. 9 条红线逐条核对通过
  2. 无红线违反记录
- **依赖**：T15-ACC-04

---

## 任务统计

| 模块 | 任务数 | 新增文件 | 修改文件 |
|------|--------|---------|---------|
| 1. 基础埋点设施 | 3 | 0 | raft.go, raft_pipeline.go |
| 2. metrics 端点 | 5 | metrics_collector.go | main.go, latency_stats.go(复用) |
| 3. 令牌桶限流 | 6 | token_bucket_limiter.go | main.go |
| 4. mTLS 证书 | 6 | tls_config.go | grpc_server.go, gen_certs.sh |
| 5. 客户端鉴权 | 2 | auth_middleware.go | main.go |
| 6. 过载曲线 | 4 | overload_curve.go | tools/loadgen/main.go |
| 7. 单测护栏 | 3 | 3 个 _test.go | 0 |
| 8. 部署配置 | 3 | 0 | docker-compose-5node.yml, deploy.env, design.md |
| 9. 集成验收 | 4 | 0 | 0 |
| 10. 代码审查 | 2 | 0 | 0 |
| **合计** | **38** | **6 新增 + 3 测试** | **7 修改** |

**新增文件清单**：
1. `metrics_collector.go`
2. `token_bucket_limiter.go`
3. `tls_config.go`
4. `auth_middleware.go`
5. `overload_curve.go`
6. `metrics_collector_test.go`
7. `token_bucket_limiter_test.go`
8. `tls_config_test.go`

**修改文件清单**：
1. `main.go`（注册 /metrics + 限流 + 鉴权接入）
2. `raft.go`（心跳原子化 + 选举事件计数）
3. `raft_pipeline.go`（compaction 计数 + 快照进度）
4. `grpc_server.go`（mTLS Fail-Closed + ClientAuth）
5. `gen_certs.sh`（私钥权限 + SAN）
6. `tools/loadgen/main.go`（曲线采集集成）
7. `tests/deploy/docker-compose-5node.yml`（证书挂载 + 环境变量）
8. `tests/deploy/deploy.env`（配置项补全）

**验收线映射**：
- 验收线 1（/metrics + 采集开销）→ T15-ACC-01
- 验收线 2（限流过载 + 曲线）→ T15-ACC-02
- 验收线 3（mTLS 正负路径）→ T15-ACC-03
- 验收线 4（单测护栏）→ T15-UT-01 + T15-UT-02 + T15-UT-03
- 验收线 5（阶梯复测）→ T15-ACC-04