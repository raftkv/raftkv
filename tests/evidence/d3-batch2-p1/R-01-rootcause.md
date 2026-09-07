# R-01 根因报告：35s收敛追因

> **约束**: 只读诊断，未修改任何代码。

## 1. Raft参数现状

| 参数 | 值 | 位置 |
|------|---|------|
| electionTimeoutMin | 1000ms | raft.go:32 |
| electionTimeoutMax | 2000ms | raft.go:33 |
| heartbeatIntervalMin | 50ms | raft.go:36 |
| heartbeatIntervalMax | 500ms | raft.go:37 |
| rpcTimeout | 500ms | raft.go:44 |
| adaptiveTimeoutThreshold | 3 | raft.go:40 |
| adaptiveRecoverySuccess | 3 | raft.go:41 |

## 2. E02 timeline分析

### 2.1 commit增长曲线

| 时间点 | node-2 commit | leader commit | term | 事件 |
|--------|-------------|-------------|------|------|
| 22:40:46 | 15 | 17 | 25 | network connect |
| 22:40:51 (+5s) | 15 | 19 | — | node-2未动 |
| 22:40:56 (+10s) | 15 | 20 | — | node-2未动 |
| 22:41:02 (+15s) | 15 | 21 | — | node-2未动 |
| 22:41:07 (+20s) | 15 | 23 | — | node-2未动 |
| 22:41:12 (+25s) | 15 | 25 | — | node-2未动 |
| 22:41:17 (+30s) | 15 | 26 | — | node-2未动 |
| 22:41:23 (+35s) | **27** | 27 | **53** | 收敛 |

### 2.2 关键观察

1. **node-2 commit 35s内不变**：从15到27是瞬间跳变，非渐进式同步
2. **leader commit持续增长**：17→27，恢复网络后leader仍在接受写入/产生no-op
3. **term从25→53**：35s内增长28，说明发生了约28次选举

## 3. 根因定位

### 3.1 核心根因：gRPC重连退避

**代码路径**：
- `grpc_server.go:288`: `grpc.NewClient(addr, dialOpts...)` 创建gRPC连接
- 未设置`grpc.WithConnectParams`，使用gRPC默认重连退避

**gRPC默认退避参数**（vendor/google.golang.org/grpc）：
- BaseDelay: 1s
- Multiplier: 1.6
- Jitter: 0.2
- MaxDelay: **120s**

**退避计算**（断开10s后）：
- 1s → 1.6s → 2.56s → 4.1s → 6.5s → 10.5s → 16.8s → ...
- 断开10s后退避间隔已增长至约10-17s
- 恢复网络后需等待下一个重连周期（最长约17s）

### 3.2 选举风暴叠加

- node-2网络恢复后收不到leader心跳（gRPC连接未重建）
- 选举超时1-2s触发选举，但node-2无法获得quorum（其他4节点稳定在leader=node-1）
- 每次选举失败后term+1，重置选举超时
- 35s内约28次选举（term 25→53），与1-2s×28≈28-56s吻合

### 3.3 心跳无效

- leader的心跳通过gRPC发送到node-2（raft.go:828 `client.AppendEntries(ctx, req)`）
- gRPC连接未恢复时，每次心跳超时（rpcTimeout=500ms）
- 心跳失败后不重试（raft.go:829-831），等下一个心跳周期（50-500ms）
- 自适应心跳调整：连续3次超时后心跳间隔升至500ms（raft.go:1345-1346）

### 3.4 因果链

```
docker network disconnect node-2
  → leader到node-2的gRPC连接断开
  → gRPC自动重连开始指数退避(1s→1.6s→2.56s→...)
  → leader心跳到node-2全部超时(500ms)
  → node-2收不到心跳，触发选举超时(1-2s)
  → node-2选举失败(无法获quorum)，term+1
  → 反复选举(28次/35s)

docker network connect node-2
  → gRPC在下一个退避周期重连成功(约17s后)
  → leader心跳到达node-2
  → AppendEntries批量复制日志(15→27)
  → 收敛
```

## 4. 修复建议（仅评估，不修改代码）

### 方案A：缩短gRPC重连退避（推荐）

在`grpc_server.go:288`的`grpc.NewClient`调用中添加：
```go
grpc.WithConnectParams(grpc.ConnectParams{
    Backoff: backoff.Config{
        BaseDelay:  1 * time.Second,
        Multiplier: 1.6,
        Jitter:     0.2,
        MaxDelay:   5 * time.Second,  // 从默认120s降至5s
    },
}),
```

**预期效果**：重连退避上限5s，恢复后最多等5s即可重建连接，收敛耗时预计≤10s。

### 方案B：应用层主动重连

在`sendHeartbeats()`中检测到peer连续超时后，主动调用`PeerClientManager.RemovePeer`+`AddPeer`重建连接。

**风险**：可能引入连接管理复杂度。

### 方案C：缩短选举超时

将`electionTimeoutMin`从1000ms降至500ms。

**风险**：可能引入选举风暴（TCX-Ⅱ-1 50节点选举风暴风险依据）。

## 5. 结论

| 项 | 结论 |
|---|------|
| 根因 | gRPC默认重连退避MaxDelay=120s，断开10s后退避间隔增长至约17s |
| 叠加因素 | 选举风暴(28次/35s)延长了收敛时间 |
| 35s是否合理 | **否**，远超Raft理论收敛时间(选举超时2s + 日志同步<1s) |
| 是否为batch2瓶颈 | **是**，高负载下可能更严重 |
| 推荐修复 | 方案A（缩短gRPC重连退避MaxDelay至5s） |
| 预期修复后收敛 | ≤10s（重连5s + 选举2s + 同步<1s + 余量） |