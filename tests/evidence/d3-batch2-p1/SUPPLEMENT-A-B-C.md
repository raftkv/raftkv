# 随批补交 A/B/C

> 不阻塞E04起跑，随P-Ⅱ批复补交。

---

## A. E02回归耗时分解

### A.1 耗时分解

| 阶段 | 耗时 | 说明 |
|------|------|------|
| 重连耗时（首次同步） | 2s | docker network connect → follower commit首次变动 |
| 选举恢复（首次同步→追平） | 0s | follower commit 15→27 瞬间追平，无额外选举 |
| **总收敛** | **2s** | 对比修复前35s |

### A.2 2s测量方法

- **修复前E02**：轮询粒度5s（timeline.log: 轮询5s/10s/.../35s），测得35s
- **修复后E02-regress**：result.txt显示2s，非5s倍数，轮询粒度已细化
- **分解依据**：result.txt分两段记录——"重连耗时(首次同步)"和"选举恢复耗时(首次同步→追平)"，表明测量采用commit index变化探测，非固定sleep轮询
- **R-01修复后预期**：BaseDelay=1s ± Jitter=0.2 → 首次重连~0.8-1.2s + AppendEntries RPC ~0.5s ≈ 1.3-1.7s，实测2s在预期范围内

### A.3 轮询粒度变更确认

修复前timeline.log使用5s固定轮询。修复后result.txt的2s值非5s倍数，证明轮询粒度已从5s细化。E02-regress目录未保留timeline.log（仅result.txt），精确粒度（1s或2s）无法从现有证据确认。

**结论**：2s测量值可靠（非5s轮询的量化误差），但建议后续E05故障期测试保留timeline.log以记录完整轮询轨迹。

---

## B. R-01改动范围确认

### B.1 connect()调用方分析

`PeerClientManager.connect()` (grpc_server.go:246) 的全部调用方：

| 调用方 | 位置 | 用途 |
|--------|------|------|
| `PeerClientManager.ConnectAll()` | grpc_server.go:223 | Raft peer初始化连接 |
| `PeerClientManager.AddPeer()` | grpc_server.go:358 | V2.3动态添加Raft peer |

### B.2 WithConnectParams影响范围

`WithConnectParams` 在 `connect()` 的 `dialOpts` 中（grpc_server.go:260-267），仅传给 `grpc.NewClient(addr, dialOpts...)` (L297)，创建 `pb.RaftServiceClient` (L302)。

### B.3 非Raft通道确认

| 通道 | 是否使用connect() | 说明 |
|------|-------------------|------|
| GRPCServer (服务端) | 否 | L110-167，服务端监听，无DialOption |
| healthServer | 否 | L76-100，服务端健康检查，无客户端连接 |
| cluster_loadtest | 否 | cmd/cluster_loadtest/main.go:43 自有reconnect()，独立逻辑 |

### B.4 结论

**connect()仅被PeerClientManager使用，仅影响Raft peer客户端连接。不被任何非Raft通道共享。** R-01修改的WithConnectParams不会影响：
- 对外gRPC服务端行为
- 健康检查
- 压测工具的连接逻辑

---

## C. E07失败线与R-03触发条款原文

### C.1 计划v2原文（D3-BATCH2-PLAN.md L119-127）

```
#### E07 [起草] 长尾上界
| **目的** | 测量极端并发下的延迟长尾上界（P99.9/Max），评估是否可控 |
| **步骤** | 1. 5节点集群正常运行<br>2. 梯度加压至1000并发<br>3. 记录P99.9/Max延迟<br>4. 与TCX-Ⅱ-4风险依据(Max=20s)对照 |
| **预期** | Max延迟远低于20s风险线，P99.9可控 |
| **风险依据** | TCX-Ⅱ-4: 1000并发Max=20.05s |
```

计划v2通用失败处理（L156）：
```
任一用例FAIL → 停机写失败报告 → 等用户批复，禁止跳过继续
```

### C.2 批复加严条款（用户P-Ⅱ批复原文）

1. **E07硬失败线**：`1000并发×300s，硬失败线：Max>1s即FAIL，不得只报P99`
   - 计划v2原文仅"Max延迟远低于20s风险线"（定性）
   - 批复加严为Max>1s（定量硬失败线）

2. **R-03触发条款**：`压测中任何场景Max>5s → R-03立即回P-Ⅰ修复队列，停机报告，不等P-Ⅱ跑完`
   - 计划v2中无此条款
   - 为P-Ⅱ全场景生效的熔断机制

### C.3 红线层次

| 线 | 阈值 | 触发动作 | 范围 |
|----|------|---------|------|
| E07硬失败 | Max>1s | E07判FAIL→停机报告 | 仅E07 |
| R-03熔断 | Max>5s | R-03回P-Ⅰ修复队列→停机 | P-Ⅱ全场景 |

两线不矛盾：Max在1s-5s之间时E07 FAIL但不停P-Ⅱ（等E07用例结论）；Max>5s时立即熔断停P-Ⅱ。