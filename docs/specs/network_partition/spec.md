# 战役III：网络分区注入 — 需求规格

> 版本: v2.4-batch26
> 日期: 2026-09-13
> 状态: SDD 起草完成，禁止直接开工

---

## 1. 目标

验证 Raft 共识引擎在网络分区下的安全性（无脑裂）与可用性（少数派存活/多数派恢复）。

## 2. 故障形态

| 分区类型 | 描述 | 预期行为 |
|---------|------|---------|
| 对称双分区 | {A,B} vs {C,D,E} | 多数派 {C,D,E} 选出 leader，少数派 {A,B} 不选举 |
| 非对称分区 | {A} vs {B,C,D,E} | A 孤立不选举，{B,C,D,E} 正常 |
| 桥接节点分区 | {A,B} - C - {D,E} | C 可达双方但不同时，C 不成桥接脑裂 |
| 分区恢复 | 分区愈合 | 日志追平，term 单调，无回退 |

"分区恢复" 语义：分区愈合后，旧少数派节点重新加入多数派，日志追平，leader 不变（除非旧 leader 在少数派侧）。

## 3. 隔离粒度

- **节点级**: docker network disconnect/connect 隔离整个节点
- **方向级**: iptables 规则隔离单向流量（A→B 通但 B→A 不通）
- **本战役范围**: 节点级（方向级留后续）

## 4. 恢复语义

| 阶段 | 语义 |
|------|------|
| 分区前 | 集群正常，leader 已选出 |
| 分区中 | 多数派继续可用，少数派不选举（pre-vote 阻止） |
| 分区恢复 | docker network connect 恢复网络 |
| 恢复后 | 30s 内日志追平，term 单调，无脑裂 |

## 5. 验收草案

| 验收项 | 阈值 | 测量仪表 |
|--------|------|---------|
| NP-1: 分区期间无脑裂 | max_concurrent_leaders ≤ 1 | /raft/stats (state=Leader 计数) |
| NP-2: 多数派可用 | 多数派侧 TPS > 0 | loadgen 写入成功率 |
| NP-3: 少数派不选举 | 少数派侧 0 个 Leader | /raft/stats |
| NP-4: 恢复后追平 | 30s 内 commit_index 追平 | /raft/stats (commit index) |
| NP-5: term 单调 | term 不回退 | /raft/stats (term 序列) |

## 6. 与现有系统的关系

- **chaos_inject%injector**: 扩展 scenario_type="network_partition"，复用 NodeController + Collector + EvidenceManager
- **pre-vote**: 网络分区下 pre-vote 应阻止少数派发起选举（日志落后者预支持不通过）
- **regression.yaml**: NP-1~5 将作为新回归线加入 REG-9~13