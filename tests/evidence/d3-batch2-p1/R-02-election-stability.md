# R-02 选举稳定性评估报告

> **约束**: 只读诊断，未修改任何代码。

## 1. 选举参数

| 参数 | 值 | 位置 |
|------|---|------|
| electionTimeoutMin | 1000ms | raft.go:32 |
| electionTimeoutMax | 2000ms | raft.go:33 |
| rpcTimeout | 500ms | raft.go:44 |

## 2. 选举保护机制（handleElectionTimeout, raft.go:490-560）

| 条件 | 行为 | 超时重置 |
|------|------|---------|
| 当前是Leader | 直接返回 | 不重置 |
| WAL重放未完成 | 拒绝选举 | ×2 (2-4s) |
| WAL门禁关闭 | 拒绝选举 | ×2 (2-4s) |
| 无peer | 不发起选举 | ×10 (10-20s) |
| 日志未追上Leader + 心跳<5s + 失败<3次 | 跳过选举 | ×3 (3-6s) |
| 连续3次选举失败 | 强制突破logCaughtUp | 正常 (1-2s) |

## 3. 5节点拓扑实测

- Leader崩溃→新Leader当选耗时: **<5s**（E01b实测 + R-01修复后回归PASS）
- 选举超时1-2s + RequestVote RPC 500ms + quorum收集 < 3s
- 5节点quorum=3，选举通常1轮完成

## 4. 结论（三段式）

### 4.1 5节点：无需修复

5节点拓扑下选举超时1-2s合理，quorum=3，1轮选举完成。
E01b实测<5s达标，R-01修复后回归PASS。
选举保护机制完善（WAL/日志追平/风暴自愈多重保护）。

### 4.2 50节点：风险未排除

TCX-Ⅱ-1鲲鹏实机压测（kunpeng-evidence/07_镜像压测矩阵）：
- 50节点单Raft组，100并发×30s×100%写入
- **写入成功率=0%**（无稳定Leader导致写入被拒）
- 根因：选举超时窗口太短，50节点的网络延迟超过超时窗口

当前V2.5选举超时1-2s未随节点数调整，50节点风险未排除。

### 4.3 裁决：待E06

batch2 E06规模阶梯（5→10→20）将实测选举稳定性随节点数的变化。
E06产出后裁决是否需引入动态选举超时（如 baseTimeout + log(peers) × jitter）。

## 5. 因果链（修正后）

R-01中node-2因收不到leader心跳而触发选举超时，这是**Raft协议的预期行为**（Follower收不到心跳→竞选Leader）。
病灶**仅**gRPC重连退避一处：退避MaxDelay=120s导致网络恢复后gRPC连接长时间不重建，node-2持续收不到心跳。
修复R-01（MaxDelay 120s→5s）后，node-2在2s内收到心跳，不触发选举，35s收敛降至2s。

## 6. SCOUT: V2.4→V2.5 Raft层diff盘点

### 6.1 diff范围

V2.4→V2.5 Raft层变更涉及以下文件（基于git log + 代码检查）：

| 文件 | 变更类型 | 关键差异 |
|------|---------|---------|
| raft.go | 参数调整+保护机制 | electionTimeout增大至>rpcTimeout(Fix#7); 选举风暴自愈(candidateFailCount>=3); logCaughtUp门禁 |
| raft_pipeline.go | 新增 | WAL加密持久化+SM4-CTR+loadSM4KeyFromEnv fail-closed |
| grpc_server.go | 新增 | PeerClientManager+gRPC重连退避(R-01修复) |
| batch_sync.go | 新增 | 批量AppendEntries同步 |

### 6.2 A/B/C结论

| 项 | 结论 | 说明 |
|---|------|------|
| A: 无影响 | 选举超时参数(1-2s) | V2.4→V2.5未变，5节点下稳定 |
| A: 无影响 | RequestVote/AppendEntries RPC语义 | Raft协议核心未变 |
| B: 有影响已处理 | gRPC重连退避 | V2.5新增PeerClientManager，R-01修复MaxDelay=5s |
| B: 有影响已处理 | 选举风暴自愈 | V2.5新增candidateFailCount>=3强制突破 |
| B: 有影响已处理 | logCaughtUp门禁 | V2.5新增日志追平检查，防止未同步节点竞选 |
| C: 有影响未处理 | 50节点选举超时 | V2.4 TCX-Ⅱ-1写入成功率0%，V2.5未引入动态超时，待E06裁决 |
| C: 有影响未处理 | Propose固定1s超时 | V2.5 Propose()无context.Deadline，高负载风险待P-Ⅱ压测 |

### 6.3 总结

- A类（无影响）：2项，Raft核心协议未变
- B类（已处理）：3项，gRPC重连+选举风暴自愈+logCaughtUp门禁
- C类（未处理）：2项，50节点选举超时+Propose deadline，待E06/P-Ⅱ裁决
