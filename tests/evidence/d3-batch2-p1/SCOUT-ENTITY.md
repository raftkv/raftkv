# SCOUT 实体（V2.4→V2.5 Raft层 diff 盘点 A/B/C 结论存证）

> 本文件为 SCOUT 专项的完整结论实体，独立存证，供盲审。

## 一、SCOUT 定义

SCOUT = V2.4 金融合规版 → V2.5 版本 Raft 层变更的 diff 盘点，按影响分级：

| 类别 | 定义 |
|------|------|
| A类 | Raft 协议核心未变，无影响 |
| B类 | 有影响但已处理（已修复/已加保护） |
| C类 | 有影响且未处理（待后续用例裁决） |

## 二、A/B/C 分类结论（完整清单）

| 序号 | 类别 | 项 | 结论 | 代码证据 |
|------|------|---|------|----------|
| A-1 | A(无影响) | 选举超时参数(1-2s) | V2.4→V2.5未变，5节点下稳定 | raft.go:32-33 electionTimeoutMin/Max=1000/2000ms |
| A-2 | A(无影响) | RequestVote/AppendEntries RPC语义 | Raft协议核心未变 | raft.go:45-52 HandleRequestVote/HandleAppendEntries |
| B-1 | B(已处理) | gRPC重连退避 | V2.5新增PeerClientManager，R-01修复MaxDelay 120s→5s | grpc_server.go:263-269 WithConnectParams MaxDelay=5s |
| B-2 | B(已处理) | 选举风暴自愈 | V2.5新增candidateFailCount>=3强制突破 | raft.go candidateFailCount 机制 |
| B-3 | B(已处理) | logCaughtUp门禁 | V2.5新增日志追平检查，防止未同步节点竞选 | raft.go:1183-1185, 1287-1291 |
| B-4 | B(已处理) | 批量同步放弃后follower掉队(fail-open) | R-04修复A/B/C/D：DNS重连+degraded标记+gap告警 | grpc_server.go/raft.go/main.go（commit 49d0c82） |
| C-1 | C(未处理) | 50节点选举超时 | V2.4 TCX-Ⅱ-1写入成功率0%，V2.5未引入动态超时，待E06裁决 | — |
| C-2 | C(未处理) | Propose固定1s超时 | V2.5 Propose()无context.Deadline，高负载风险 | raft.go:1016-1057 |

统计：A类 2 项，B类 4 项，C类 2 项。

## 三、依据 commit 清单

| commit | 描述 | SCOUT 关联 |
|--------|------|-----------|
| 96e0e73 | R-01-fix: gRPC重连退避MaxDelay 120s→5s | B-1 |
| 49d0c82 | R-04-fix: gRPC DNS重连+degraded标记+gap告警+compose别名固定 | B-4 |
| de9238f | F4b Leader当选时追加no-op条目 | A-2（协议核心） |
| 58803ba | R5-F1: SM4 key from env (fail-closed) | B-4（WAL加密管线） |
| 5f3395a | Fix Bug D: gRPC GetClient TransientFailure reconnection | B-1 |
| 5103f94 | Fix Bug C: stepDown不再持锁调用batchSyncMgr.Stop() | B-2（选举保护） |
| 6c635fb | Fix#8: sendHeartbeats collectCommittedLogs+fireOnCommit | A-2（协议核心） |

## 四、E08 实测验证后的更新结论（2026-09-08）

E08 2h 长稳（FAIL→根因定案）对 SCOUT A/B/C 的实证：

| SCOUT | 定案后结论 | 证据 |
|-------|-----------|------|
| A（选举稳定性） | **修正**：5节点 Docker bridge 下选举本身可完成，但 R-04 修复A重连循环误判 Idle 致每8s无条件重建，干扰心跳触发选举 | Q1(1次切换/2h) + Q7(重连日志) |
| B（批量同步） | 确认：leader 切换后批量同步正常，follower gap=0，degraded 未触发 | Q5(gap告警0次/degraded=0) |
| C（客户端韧性） | **新增确认**：客户端无 NotLeader 重定向/重试，leader 切换后写旧leader全部失败(3.4M) | Q3(main.go 代码证据) |

新增 B-5 / C-3 项（E08 根因定案后产生）：

| 序号 | 类别 | 项 | 结论 | 修复 |
|------|------|---|------|------|
| B-5 | B(已处理) | R-04修复A重连循环Idle误判 | 修复F：移除 connectivity.Idle 误判，仅 TransientFailure/Shutdown 才重连 | commit 待 tag d3-e08-fix-pass |
| C-3 | C(已处理) | 客户端NotLeader重定向+写失败重试 | 修复E：e04_loadtest 增 NotLeader 重定向 + 3次退避重试 + 指标 | commit 待 tag d3-e08-fix-pass |

## 五、E06 前置条件状态

| 前置条件 | 状态 |
|---------|------|
| SCOUT 实体结论 | ✓ 本文件交付 |
| 10/20 节点部署拓扑方案 | ✗ 未交付，E06 继续押后 |

E06 在 B/C 结论下改为"预期复现+记录风暴特征"（竞选频率/term增速）。

## 六、存证文件

| 文件 | 路径 |
|------|------|
| 选举稳定性评估 | `tests/evidence/d3-batch2-p1/R-02-election-stability.md` |
| R-01改动范围确认 | `tests/evidence/d3-batch2-p1/SUPPLEMENT-A-B-C.md` |
| P-Ⅱ解冻补交 | `tests/evidence/d3-batch2-p1/P2-UNFREEZE-SUPPLEMENT.md` |