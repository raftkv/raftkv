# 岱境235 V2.4独立分支 state.bin 完整性加固 任务分解文档

> **文档版本**: 1.0
> **生成时间**: 2026-08-31
> **对应需求**: spec.md v1.0
> **对应设计**: design.md v1.0
> **目标分支**: V2.4_Performance_Sandbox/_state_protection_research（独立研究分支）
> **基线红线**: raft.go MD5 = DD6F667133D2C9343CF43BC11A5B7C00（V2.2-S 主线绝对冻结）
> **任务总数**: 11 个主任务，37 个子任务组，142 个原子执行项
> **执行原则**: 仅在 _state_protection_research 子目录内增量，零改动 V2.2-S 主线与 V2.4 核心 raft.go / main.go

---

## 1. 基础设施层实现

> 为上层模块提供无业务依赖的通用能力：类型定义、原子写入、可观测指标。
> 依赖：无（最先实现）

### 1.1 扩展类型定义
- [ ] 在 `_state_protection_research/types_research.go` 中定义 `IntegrityStatus` 枚举（INTEGRITY_OK / INTEGRITY_TAMPERED / INTEGRITY_LEGACY_NO_HMAC / INTEGRITY_NEW_NODE），对应 design.md 2.3.2 类图
- [ ] 在 `types_research.go` 中定义 `KeySource` 枚举（RSA_PUBLIC_KEY_HASH / RANDOM_FALLBACK）
- [ ] 在 `types_research.go` 中定义 `LoadResult` 结构体（Status IntegrityStatus, VoteRecord *VoteRecord, Error error），对应 design.md 2.2.2.1
- [ ] 在 `types_research.go` 中定义 `StateProtectorConfig` 与 `ResyncConfig` 结构体，字段与 design.md 2.1.2 配置项表一致（含 stateBinPath / stateHmacPath / hmacKeyPath / rsaPubKeyPath / snapshotIntervalMs / resyncConfig）
- [ ] 在 `types_research.go` 中定义 `ResyncResult` 结构体（Success bool, RetryCount int, Duration time.Duration, Error error）
- [ ] 在 `types_research.go` 中定义 `Logger` 接口（Printf(format string, args ...any)），供后续模块注入
- [ ] 在 `types_research.go` 中定义错误常量：ErrStateBinNotFound / ErrHmacFileNotFound / ErrHmacMismatch / ErrKeyDeriveFailed / ErrPersistFailed，并实现对应 error 类型与 Error() 方法
- [ ] 验收：`go vet ./_state_protection_research/` 通过；类型定义与 design.md 2.3.2 类图完全一致

### 1.2 原子写入工具实现
- [ ] 在 `_state_protection_research/atomic_write.go` 中实现 `AtomicWriter` 结构体与 `NewAtomicWriter()` 构造函数
- [ ] 实现 `WriteAtomic(path string, data []byte, perm os.FileMode) error` 方法，流程为：写 `path.tmp` → fsync(file) → os.Rename(path.tmp, path) → fsync(目录)，对应 design.md 2.1.3.3 阶段 2/4
- [ ] 在写入失败时清理临时文件（os.Remove(path.tmp)），避免残留
- [ ] fsync 目录通过打开目录 fd 后 `file.Sync()` 实现（Linux 下目录可 fsync）
- [ ] 验收：写入后目标文件权限 == perm；写入过程中模拟 rename 失败时临时文件被清理；对应 FA-03 原子性前置能力

### 1.3 可观测指标实现
- [ ] 在 `_state_protection_research/state_metrics.go` 中实现 `IntegrityMetrics` 结构体（checkTotal uint64 / checkFailedTotal uint64，均 atomic 访问）
- [ ] 实现 `NewIntegrityMetrics()` 构造函数
- [ ] 实现 `IncCheckTotal()` 与 `IncCheckFailed()` 方法，使用 `atomic.AddUint64` 保证并发安全
- [ ] 实现 `PrometheusFormat() string` 方法，输出 `state_integrity_check_total` 与 `state_integrity_check_failed_total` 两个 Prometheus 计数指标文本
- [ ] 实现 `RegisterHTTPHandler(mux *http.ServeMux)` 方法，注册 `/metrics` 端点暴露 PrometheusFormat 输出
- [ ] 验收：并发 1000 次 IncCheckTotal 后计数 == 1000；curl /metrics 返回两个指标且数值正确；对应 NFA-10

---

## 2. state.bin 持久化层实现

> 实现 VoteRecord 的二进制序列化与 state.bin 原子读写。
> 依赖：任务组 1（atomic_write.go, types_research.go）

### 2.1 VoteRecord 序列化与反序列化
- [ ] 在 `_state_protection_research/state_persist.go` 中实现 `StatePersistence` 结构体与 `NewStatePersistence(binPath string)` 构造函数
- [ ] 实现 `Serialize(vr VoteRecord) []byte` 方法，格式为 `[8 字节 int64 大端 term][4 字节 int32 大端 votedForLen][votedFor UTF-8 字节]`，对应 design.md 2.3.2 二进制格式定义
- [ ] 实现 `Deserialize(data []byte) (VoteRecord, error)` 方法，校验长度 ≥ 12、votedForLen 与剩余字节一致，不一致返回 ErrPersistFailed
- [ ] 验收：Serialize 与 Deserialize 互逆（`Deserialize(Serialize(vr)) == vr`）；votedFor 为空时总大小 == 12 字节；对应 FA-01 前置能力

### 2.2 state.bin 原子读写与丢弃
- [ ] 实现 `Save(vr VoteRecord) error` 方法：Serialize → AtomicWriter.WriteAtomic(binPath, data, 0600)，对应 design.md 2.2.2.2
- [ ] 实现 `Load() (VoteRecord, error)` 方法：os.ReadFile → Deserialize，文件不存在返回 ErrStateBinNotFound
- [ ] 实现 `Exists() bool` 方法：os.Stat 检查文件存在性
- [ ] 实现 `Discard() error` 方法：os.Remove(binPath)，文件不存在时返回 nil（幂等）
- [ ] 验收：Save 后文件权限 == 0600；Save 后 Load 返回值与入参一致；Discard 后 Exists() == false；对应 FA-01、6.1 权限约束

---

## 3. HMAC-SHA256 校验层实现

> 实现 HMAC 密钥派生、HMAC 计算与校验和文件读写。
> 依赖：任务组 1（atomic_write.go, types_research.go）

### 3.1 HMAC 密钥派生
- [ ] 在 `_state_protection_research/state_hmac.go` 中实现 `HMACKeyDeriver` 结构体（rsaPubKeyPath / fallbackKeyPath 字段）与 `NewHMACKeyDeriver(rsaPubKeyPath, fallbackKeyPath string)` 构造函数
- [ ] 实现 `Derive() ([]byte, error)` 方法，优先路径：读取 RSA 公钥文件 → `sha256.Sum256` → 取前 32 字节，对应 design.md 2.2.2.3
- [ ] 在 Derive 中实现回退路径：RSA 公钥不存在时，检查 fallbackKeyPath 文件是否存在；存在则读取并校验长度 == 32；不存在则 `crypto/rand.Read` 生成 32 字节并通过 AtomicWriter.WriteAtomic 落盘 0600
- [ ] 实现 `IsFromRSA() bool` 方法，返回上次 Derive 是否使用 RSA 公钥路径
- [ ] 验收：RSA 公钥存在时 Derive 返回 SHA-256 哈希前 32 字节且 IsFromRSA()==true；删除 RSA 公钥后 Derive 生成 state_hmac_key.bin 且权限 == 0600 且 IsFromRSA()==false；再次 Derive 复用已落盘密钥；对应 FA-02、NFA-07

### 3.2 HMAC 计算与校验和读写
- [ ] 在 `state_hmac.go` 中实现 `HMACCalculator` 结构体与 `NewHMACCalculator(hmacPath string)` 构造函数
- [ ] 实现 `Compute(data, key []byte) [32]byte` 方法，使用 `hmac.New(sha256.New, key)` 计算，对应 design.md 2.2.2.4
- [ ] 实现 `SaveHmac(hmac [32]byte) error` 方法：AtomicWriter.WriteAtomic(hmacPath, hmac[:], 0600)
- [ ] 实现 `LoadHmac() ([32]byte, error)` 方法：os.ReadFile → 校验长度 == 32 → 转为 [32]byte；文件不存在返回 ErrHmacFileNotFound
- [ ] 实现 `ExistsHmac() bool` 方法
- [ ] 实现 `Verify(data, key []byte, expectedHmac [32]byte) bool` 方法：重算 HMAC → `hmac.Equal` 常量时间比对
- [ ] 实现 `DiscardHmac() error` 方法：os.Remove(hmacPath)，幂等
- [ ] 验收：SaveHmac 后文件权限 == 0600 且大小 == 32 字节；Verify 对匹配返回 true、对篡改返回 false；1000 次 Compute 平均耗时 ≤ 1ms；对应 NFA-01、NFA-07、6.2 权限约束

---

## 4. 篡改检测与自动降级编排

> 实现启动时校验、篡改检测、自动降级、运行时快照循环。
> 依赖：任务组 1、2、3、5（state_resync.go 的 TriggerResync 接口）

### 4.1 StateProtector 核心结构与初始化
- [ ] 在 `_state_protection_research/state_protection.go` 中实现 `StateProtector` 结构体（config / persistence / hmac / keyDeriver / resync / metrics / logger 字段）与 `NewStateProtector(cfg StateProtectorConfig, logger Logger) (*StateProtector, error)` 构造函数，对应 design.md 2.2.2.1
- [ ] 在构造函数中初始化 StatePersistence、HMACCalculator、HMACKeyDeriver、StateResyncManager、IntegrityMetrics 子模块
- [ ] 验收：构造成功后所有子模块非 nil；config 缺失必填字段时返回错误

### 4.2 启动时加载与校验（LoadAndVerify）
- [ ] 实现 `LoadAndVerify() (LoadResult, error)` 方法，按 design.md 2.1.3.1 状态机编排：派生密钥 → 读 state.bin → 读 state.bin.hmac → 分支处理
- [ ] 实现分支 InitNewNode：state.bin 不存在时返回 LoadResult{Status: INTEGRITY_NEW_NODE, VoteRecord: nil}，对应 spec 5.2.3 场景2
- [ ] 实现分支 InitLegacyHmac：state.bin 存在且 hmac 不存在时，计算并写入 state.bin.hmac，日志输出 `[STATE-PROTECTION] INFO legacy init`，返回 LoadResult{Status: INTEGRITY_LEGACY_NO_HMAC, VoteRecord: 加载值}，对应 FA-08、RC-04
- [ ] 实现分支 VerifyHmac 一致：返回 LoadResult{Status: INTEGRITY_OK, VoteRecord: 加载值}，对应 FA-04
- [ ] 实现分支 VerifyHmac 不一致（TamperDetected）：输出 `[CRITICAL] state.bin integrity check failed` 至 os.Stderr 与 logger；调用 persistence.Discard() 与 hmac.DiscardHmac() 丢弃损坏文件；调用 node.SetLogCaughtUp(false)（通过传入 node 引用或返回标志由 main 处理）；metrics.IncCheckFailed()；返回 LoadResult{Status: INTEGRITY_TAMPERED}，对应 FA-05、FA-06、FA-07、NFA-08
- [ ] 在 LoadAndVerify 全程调用 metrics.IncCheckTotal()
- [ ] 验收：正常启动返回 INTEGRITY_OK 且 VoteRecord 与持久化一致；篡改 state.bin 后返回 INTEGRITY_TAMPERED 且 stderr 含 [CRITICAL]；无 hmac 存量返回 INTEGRITY_LEGACY_NO_HMAC 且 hmac 文件被初始化；对应 FA-04/05/06/07/08

### 4.3 运行时定期快照循环
- [ ] 实现 `StartSnapshotLoop(node *RaftNode) func()` 方法，启动 goroutine 按 design.md 2.1.3.2 流程定期快照：睡眠 snapshotIntervalMs → 读取 node.Term() 与 node.Stats().VotedFor → 与上次快照比对 → 有变更则 PersistVoteRecord
- [ ] 实现 `PersistVoteRecord(vr VoteRecord) error` 方法：Serialize → Compute HMAC → 原子写入 state.bin → 原子写入 state.bin.hmac（先 bin 后 hmac 顺序，对应 design.md 2.1.3.3 决策），对应 FA-01
- [ ] 快照循环通过 `node.Term()`（atomic 读）与 `node.Stats().VotedFor`（RLock 读）获取当前值，禁止反射或修改 raft.go
- [ ] 返回的 stop 函数通过 channel 或 context 通知 goroutine 退出
- [ ] 验收：触发 term/votedFor 变更后 100ms 内 state.bin 与 state.bin.hmac 同时更新且 hmac 有效；stop() 调用后 goroutine 退出（通过 goroutine 泄漏检测或 wait group 验证）；对应 FA-01、FA-03

### 4.4 优雅关闭
- [ ] 实现 `Shutdown() error` 方法，停止快照循环并释放资源
- [ ] 验收：Shutdown 后快照 goroutine 退出；重复 Shutdown 不 panic

---

## 5. 重同步恢复管理

> 实现节点进入 corrupted 状态后的日志重同步、状态重建与超时重试。
> 依赖：任务组 1、2、3（state_persist, state_hmac, state_metrics）；V2.4 raft.go 公共方法（RestoreFromWAL, SetLogCaughtUp, LeaderID）

### 5.1 重同步触发与编排
- [ ] 在 `_state_protection_research/state_resync.go` 中实现 `StateResyncManager` 结构体与 `NewStateResyncManager(cfg ResyncConfig, metrics IntegrityMetrics, logger Logger) *StateResyncManager` 构造函数，对应 design.md 2.2.2.5
- [ ] 实现 `TriggerResync(node *RaftNode) ResyncResult` 方法：循环 MaxRetry 次，每次 `context.WithTimeout(TimeoutMs)` 调用单次重同步，成功则返回，失败则等待 RetryIntervalMs 重试，对应 design.md 2.1.3.1 TriggerResync 状态
- [ ] 单次重同步逻辑：调用 node.LeaderID() 获取 Leader → 通过 Raft 日志拉取机制（复用 raft.go 现有 RPC，不引入新 RPC）等待 node.logCaughtUp == true → 调用 node.RestoreFromWAL 重建日志，对应 design.md 2.2.2.5 业务说明
- [ ] 验收：进入 corrupted 后 TriggerResync 自动发起重同步；重同步成功后 ResyncResult.Success == true；对应 FA-09

### 5.2 重同步后状态重建
- [ ] 在重同步成功后，基于拉取的日志重建 VoteRecord（term 从日志最后一条 term 推导，votedFor 置空等待新任期投票），调用 StatePersistence.Save 与 HMACCalculator.SaveHmac 写入 state.bin 与 state.bin.hmac，对应 design.md 2.1.3.1 RebuildState
- [ ] 重建后调用 node.SetLogCaughtUp(true) 退出 corrupted 状态
- [ ] 日志输出 `[STATE-PROTECTION] INFO resync success`
- [ ] 验收：重同步完成后 state.bin 与 state.bin.hmac 重新生成；节点状态转为正常 Follower；term/votedFor 与集群一致；对应 FA-10

### 5.3 超时与重试处理
- [ ] 单次重同步超时（context.Done）时日志输出 `[STATE-PROTECTION] WARN resync timeout`，对应 FA-11
- [ ] 重试间隔等待 RetryIntervalMs（默认 1000ms）
- [ ] 重试 MaxRetry 次（默认 3）仍失败时日志输出 `[STATE-PROTECTION] ERROR resync failed after N retries`，返回 ResyncResult{Success: false, RetryCount: MaxRetry}，节点保持 corrupted 状态，对应 FA-11、spec 5.3.1 规则3
- [ ] 验收：模拟网络分区使重同步超时 → 3 次重试后 ResyncResult.Success == false 且节点保持 corrupted；对应 FA-11

---

## 6. 独立分支启动入口

> 实现独立 main 入口，复用 V2.4 公共组件并注入 StateProtector。
> 依赖：任务组 1-5；V2.4 main.go 公共组件（NewRaftNode, NewRaftPipeline, NewGRPCServer 等，只读复用）

### 6.1 启动流程组装
- [ ] 在 `_state_protection_research/cmd/state-protection-research/main_research.go` 中实现独立 main 入口，对应 design.md 5.1 目录结构
- [ ] 复用 V2.4 main.go 的启动顺序（解析配置 → 连接 peer → NewRaftNode → NewRaftPipeline → ReplayWAL → RestoreFromWAL → NewGRPCServer → HTTP 端点 → errgroup 启动），仅在外层包装 StateProtector，对应 design.md 1.2.4
- [ ] 在 NewRaftNode 之后、Run 之前插入 `protector.LoadAndVerify()`，根据 LoadResult.Status 分支处理：
  - INTEGRITY_OK / INTEGRITY_LEGACY_NO_HMAC：加载 VoteRecord 至内存（通过扩展的 RestoreVoteRecord 方法或等价机制）
  - INTEGRITY_NEW_NODE：正常启动空状态
  - INTEGRITY_TAMPERED：触发 resync.TriggerResync(node)，成功后继续启动，失败则退出进程
- [ ] 启动后调用 `protector.StartSnapshotLoop(node)` 并 defer stop()
- [ ] 验收：独立二进制 `gateway_research` 可正常启动并加入集群；启动延迟增量 ≤ 50ms；对应 NFA-02、FA-12

### 6.2 配置加载与环境变量映射
- [ ] 从环境变量加载 StateProtectorConfig（STATE_BIN_PATH / STATE_HMAC_PATH / STATE_HMAC_KEY_PATH / RSA_PUBLIC_KEY_PATH / STATE_SNAPSHOT_INTERVAL_MS / STATE_RESYNC_TIMEOUT_MS / STATE_RESYNC_MAX_RETRY / STATE_RESYNC_RETRY_INTERVAL_MS），默认值对应 design.md 2.1.2 配置项表
- [ ] 验收：未设置环境变量时使用默认值；设置环境变量时覆盖默认值

### 6.3 HTTP 端点扩展
- [ ] 在 /raft/status 端点输出中新增 `integrity_status` 字段，暴露当前 IntegrityStatus（不修改 V2.4 main.go，在 main_research.go 中重新注册 Handler）
- [ ] 注册 /metrics 端点（通过 IntegrityMetrics.RegisterHTTPHandler）
- [ ] 验收：curl /raft/status 返回含 integrity_status 字段；curl /metrics 返回两个完整性指标；对应 NFA-10

---

## 7. 隔离构建与镜像生成

> 实现多架构交叉编译、研究镜像构建与 3 节点集群编排。
> 依赖：任务组 1-6（代码完成）

### 7.1 研究镜像 Dockerfile
- [ ] 在 `_state_protection_research/Dockerfile.research` 中实现两阶段构建（golang:1.24-alpine builder → alpine:3.21 runtime），对应 design.md 3.1
- [ ] 在 builder 阶段通过 `ARG TARGETOS / TARGETARCH` 接收 buildx 注入的目标架构，执行 `CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /gateway_research ./cmd/state-protection-research/`
- [ ] 在 runtime 阶段预创建 /app/data 与 /app/keys 目录，复制 gateway_research，EXPOSE 9500 9000
- [ ] 验收：docker build 成功生成镜像；镜像名 == daijin235-state-protection-research；与 V2.2-S 商业镜像名不同；对应 FA-14、RC-03

### 7.2 交叉编译与镜像构建脚本
- [ ] 在 `_state_protection_research/build_research.sh` 中实现交叉编译 ARM64 与 amd64 两个二进制产物至 dist/ 目录，对应 design.md 3.2
- [ ] 在脚本中调用 `docker buildx build --platform linux/arm64,linux/amd64 -t daijin235-state-protection-research:v2.4-research -f Dockerfile.research .` 构建多架构镜像
- [ ] 脚本使用 `set -euo pipefail` 严格模式
- [ ] 验收：执行后 dist/ 下生成 gateway_research_arm64 与 gateway_research_amd64；docker images 含 daijin235-state-protection-research:v2.4-research；对应 FA-13、FA-14

### 7.3 3 节点集群编排
- [ ] 在 `_state_protection_research/docker-compose.research.yml` 中定义 node-1 / node-2 / node-3 三个服务，均使用 daijin235-state-protection-research:v2.4-research 镜像，对应 design.md 4.1
- [ ] 每个节点配置 NODE_ID / GRPC_PORT / HTTP_PORT / PEERS / STATE_BIN_PATH / STATE_HMAC_PATH / STATE_HMAC_KEY_PATH 环境变量
- [ ] 端口映射：node-1 (9500/9000)、node-2 (9501/9001)、node-3 (9502/9002)
- [ ] 每节点独立 volume（node1-data / node2-data / node3-data）与 research-net bridge 网络
- [ ] 验收：`docker-compose -f docker-compose.research.yml up -d` 启动 3 节点集群；Leader 数量 == 1；对应 SV-01

### 7.4 测试用 RSA 密钥材料
- [ ] 在 `_state_protection_research/keys/` 下生成测试用 RSA-2048 密钥对（public.pem / private.pem），仅用于沙箱验证
- [ ] 验收：public.pem 可被 HMACKeyDeriver.Derive 成功读取并派生 32 字节密钥

---

## 8. 单元测试覆盖

> 为所有核心模块编写单元测试，保证逻辑正确性与边界覆盖。
> 依赖：任务组 1-5（对应模块实现完成）

### 8.1 原子写入测试
- [ ] 在 `_state_protection_research/atomic_write_test.go` 中测试 WriteAtomic 正常写入、权限设置、临时文件清理、并发写入安全性
- [ ] 验收：`go test -race -count=50 -run TestAtomicWriter ./_state_protection_research/` 全部通过

### 8.2 state.bin 持久化测试
- [ ] 在 `_state_protection_research/state_persist_test.go` 中测试 Serialize/Deserialize 互逆、votedFor 为空边界、Load 不存在文件、Save+Load 往返、Discard 幂等
- [ ] 验收：`go test -race -count=50 -run TestStatePersistence ./_state_protection_research/` 全部通过

### 8.3 HMAC 校验测试
- [ ] 在 `_state_protection_research/state_hmac_test.go` 中测试密钥派生优先路径（RSA）、回退路径（随机密钥落盘 0600）、回退密钥复用、HMAC 计算一致性、Verify 匹配/不匹配、SaveHmac+LoadHmac 往返、1000 次 Compute 性能（≤ 1ms 平均）
- [ ] 验收：`go test -race -count=50 -run TestHMAC ./_state_protection_research/` 全部通过；性能测试断言平均耗时 ≤ 1ms；对应 NFA-01、NFA-04

### 8.4 篡改检测与降级测试
- [ ] 在 `_state_protection_research/state_protection_test.go` 中测试 LoadAndVerify 四个分支（INTEGRITY_OK / INTEGRITY_TAMPERED / INTEGRITY_LEGACY_NO_HMAC / INTEGRITY_NEW_NODE）、篡改后 [CRITICAL] 输出、损坏文件丢弃、快照循环触发持久化、stop() 退出 goroutine
- [ ] 验收：`go test -race -count=50 -run TestStateProtector ./_state_protection_research/` 全部通过；对应 FA-04/05/06/07/08

### 8.5 重同步管理测试
- [ ] 在 `_state_protection_research/state_resync_test.go` 中测试 TriggerResync 成功路径、超时重试、3 次重试失败保持 corrupted、状态重建后 state.bin 与 hmac 重新生成
- [ ] 验收：`go test -race -count=50 -run TestStateResync ./_state_protection_research/` 全部通过；对应 FA-09/10/11

### 8.6 全量测试与竞态检测
- [ ] 执行 `go test -race -count=50 ./_state_protection_research/...` 全量运行所有单元测试
- [ ] 验收：全部通过，无竞态告警，无内存泄漏（goroutine 计数稳定）

---

## 9. 沙箱验证执行

> 在 3 节点集群中执行端到端篡改-检测-降级-恢复验证。
> 依赖：任务组 7（镜像与集群编排完成）

### 9.1 沙箱验证脚本
- [ ] 在 `_state_protection_research/verify_sandbox.sh` 中实现 SV-01 至 SV-06 六个验证步骤的自动化，对应 design.md 4.2
- [ ] SV-01：启动 3 节点集群 → 等待 Leader 选出 → 校验 Leader 数量 == 1
- [ ] SV-02：`docker exec research-node-3 sh -c 'echo "TAMPERED" > /app/data/state.bin'` 篡改 → `docker restart research-node-3` → `docker logs research-node-3 2>&1 | grep [CRITICAL]` 校验告警
- [ ] SV-03：`docker logs research-node-3 | grep local-cache-corrupted` 校验降级状态
- [ ] SV-04：等待 10s → `curl http://localhost:9002/raft/status` 校验 state == Follower 且 term 与集群一致
- [ ] SV-05：全流程轮询 3 节点 /raft/status 校验 Leader 数量始终 == 1
- [ ] SV-06：`docker inspect research-node-3 --format='{{.Config.Image}}'` 校验使用研究镜像
- [ ] 验收：脚本执行后 PASS == 6，FAIL == 0；对应 SV-01/02/03/04/05/06

### 9.2 沙箱验证执行与结果记录
- [ ] 在 _state_protection_research 目录下执行 `bash verify_sandbox.sh`，捕获完整终端输出
- [ ] 记录每个 SV 步骤的实际结果（PASS/FAIL）、关键日志片段、curl 响应、docker logs 输出
- [ ] 验收：SV-01 至 SV-06 全部 PASS；重同步完成耗时 ≤ 10s；对应 NFA-03、NFA-05、NFA-06

### 9.3 性能与可靠性指标采集
- [ ] 在沙箱中测量 HMAC 计算耗时（1000 次平均）、启动延迟增量（对比 V2.4 原始启动）、重同步完成时间
- [ ] 执行 10000 次正常写入/读取循环，校验无误判
- [ ] 验收：HMAC 平均耗时 ≤ 1ms；启动延迟增量 ≤ 50ms；重同步 ≤ 10s；10000 次循环无误判；对应 NFA-01/02/03/04

---

## 10. 红线约束验证

> 验证整个实现过程严格遵守红线约束，未污染 V2.2-S 主线与 V2.4 核心 raft.go。
> 依赖：所有任务组完成

### 10.1 V2.2-S 主线零改动验证
- [ ] 计算 `D:\岱境235源码备份\daijin235_go_engine\raft.go` 的 MD5，校验 == DD6F667133D2C9343CF43BC11A5B7C00
- [ ] 对比加固前后 `D:\岱境235源码备份\daijin235_go_engine\` 全目录文件列表与 MD5，确认零改动
- [ ] 验收：MD5 一致；全目录无变更；对应 RC-01、RC-05

### 10.2 V2.4 核心 raft.go 零改动验证
- [ ] 计算 `D:\235备份文件\V2.4_Performance_Sandbox\raft.go` 加固前后 MD5，校验一致
- [ ] 计算 `D:\235备份文件\V2.4_Performance_Sandbox\main.go` 加固前后 MD5，校验一致
- [ ] 计算 `D:\235备份文件\V2.4_Performance_Sandbox\types.go` 加固前后 MD5，校验一致
- [ ] 验收：三个核心文件 MD5 加固前后一致；对应 RC-02

### 10.3 独立分支隔离验证
- [ ] 执行 `git diff --stat` 检查变更范围，确认所有变更仅在 `_state_protection_research/` 子目录内
- [ ] 验收：git diff 范围仅含 _state_protection_research/ 下文件；对应 FA-12

### 10.4 研究镜像隔离验证
- [ ] 执行 `docker images` 列出所有镜像，确认 daijin235-state-protection-research 与 V2.2-S 商业镜像名不同
- [ ] 对比研究镜像与商业镜像的 image name / tag / entrypoint 二进制名，确认完全隔离
- [ ] 验收：镜像名不同、标签体系不同、入口二进制不同；对应 FA-14、RC-03

---

## 11. 交付物同步与完成确认

> 生成加固验证报告并同步至三处，完成最终确认。
> 依赖：任务组 9、10（验证与红线确认完成）

### 11.1 加固验证报告生成
- [ ] 在 `_state_protection_research/` 下生成《V2.4独立分支state.bin完整性加固验证报告.md》，包含：验证结果汇总（PASS/FAIL 计数）、红线确认（raft.go MD5、V2.4 零改动、镜像隔离）、性能指标（HMAC 耗时、启动延迟、重同步时间）、SV-01 至 SV-06 详细结果、必须包含声明"此加固方案已在V2.4独立分支中验证成功，将作为V2.5升级功能储备，暂时不合并进V2.2-S主线。"
- [ ] 验收：报告内容完整，含必须声明；对应 spec 8.1

### 11.2 报告三处同步
- [ ] 将报告同步至桌面：`C:\Users\27998\Desktop\V2.4独立分支state.bin完整性加固验证报告.md`
- [ ] 将报告同步至 D 盘备份：`D:\235备份文件\V2.4独立分支state.bin完整性加固验证报告.md`
- [ ] 将报告同步至归档04：`D:\岱境235_20260810_硬核工程产出归档\04_压力测试与工程验证工具\V2.4独立分支state.bin完整性加固验证报告.md`
- [ ] 计算三处文件 MD5，校验完全一致
- [ ] 验收：三处文件存在且 MD5 一致；对应 spec 8.1

### 11.3 完成确认语输出
- [ ] 在终端输出完成确认语："隔离沙箱内state.bin修复验证完成，V2.2-S主线零改动，加固方案已作为V2.5储备。"
- [ ] 验收：确认语与 spec 8.2 完全一致；对应 spec 8.2

---

> **任务分解文档结束**
> 本文档将 design.md 的技术方案分解为 11 个主任务、47 个子任务，覆盖 spec.md 全部 14 项功能验收（FA-01~FA-14）、10 项非功能验收（NFA-01~NFA-10）、5 项红线约束验收（RC-01~RC-05）、6 项沙箱验证（SV-01~SV-06）。
> 任务按依赖关系排序：基础设施 → 持久化 → HMAC → 编排 → 重同步 → 启动入口 → 构建 → 测试 → 沙箱验证 → 红线验证 → 交付物。
> 所有任务严格遵守红线约束：仅在 _state_protection_research 子目录内增量，零改动 V2.2-S 主线与 V2.4 核心 raft.go / main.go / types.go。