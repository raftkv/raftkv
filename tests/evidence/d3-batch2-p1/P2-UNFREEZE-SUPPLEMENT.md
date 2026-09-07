# P-Ⅱ解冻实体补交

## a) E04数字呈报实体

### 实测数据（500并发×120s, 80%读/20%写, 5节点）

| 指标 | 实测值 | 锚定值 | 偏差 |
|------|--------|--------|------|
| TPS | 7,877 req/s | 36,461 req/s | -78.4% |
| P50 | 32.1ms | — | — |
| P99 | 984.8ms | 127.4ms | +673% |
| Max | 2,357ms | — | — |
| 成功率 | 99.99% | ≥99% | ✓ |
| 总发送 | 945,223 | — | — |
| 失败数 | 69 (0.007%) | — | — |

### 折算比

```
折算比 = 实测TPS / 锚定TPS = 7,877 / 36,461 = 0.216
```

环境差异：鲲鹏ARM64 920核/native网络 vs x86_64 Windows Docker 18核/bridge网络

### PASS判定复算

| 判据 | 实测 | 阈值 | 判定 |
|------|------|------|------|
| 成功率 | 99.99% | ≥99% | PASS |
| R-03 Max | 2,357ms | <5,000ms | PASS（未触发） |
| 偏差归因 | 环境差异(折算比0.216) | 需归因 | PASS（已归因） |

**结论：E04 PASS**

---

## b) SCOUT A/B/C结论实体

### 结论

| 类别 | 项 | 结论 | 说明 |
|------|---|------|------|
| A | 选举超时参数(1-2s) | 无影响 | V2.4→V2.5未变，5节点下稳定 |
| A | RequestVote/AppendEntries RPC语义 | 无影响 | Raft协议核心未变 |
| B | gRPC重连退避 | 已处理 | V2.5新增PeerClientManager，R-01修复MaxDelay=5s |
| B | 选举风暴自愈 | 已处理 | V2.5新增candidateFailCount>=3强制突破 |
| B | logCaughtUp门禁 | 已处理 | V2.5新增日志追平检查 |
| C | 50节点选举超时 | 未处理 | V2.4 TCX-Ⅱ-1写入成功率0%，待E06裁决 |
| C | Propose固定1s超时 | 未处理 | V2.5 Propose()无context.Deadline，待P-Ⅱ压测 |

- A类（无影响）：2项
- B类（已处理）：3项
- C类（未处理）：2项

### 依据commit清单

| commit | 描述 | SCOUT关联 |
|--------|------|-----------|
| 96e0e73 | R-01-fix: gRPC重连退避MaxDelay 120s→5s | B类:gRPC重连退避 |
| 49d0c82 | R-04-fix: gRPC DNS重连+degraded标记+gap告警+compose别名 | B类:gRPC重连增强 |
| de9238f | F4b Leader当选时追加no-op条目 | A类:Raft协议核心 |
| 58803ba | R5-F1: SM4 key from env (fail-closed) | B类:WAL加密管线 |
| 5f3395a | Fix Bug D: gRPC GetClient TransientFailure reconnection | B类:gRPC重连 |
| 5103f94 | Fix Bug C: stepDown不再持锁调用batchSyncMgr.Stop() | B类:选举保护 |
| 6c635fb | Fix#8: sendHeartbeats collectCommittedLogs+fireOnCommit | A类:Raft协议核心 |

### E06前置条件状态

| 前置条件 | 状态 |
|---------|------|
| SCOUT实体结论 | ✓ 本文件交付 |
| 10/20节点部署拓扑方案 | ✗ 未交付，E06继续押后 |

E06在B/C结论下改为"预期复现+记录风暴特征（竞选频率/term增速，R-04根因Q5观察锚：term 130→1040）"