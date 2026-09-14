# 战役III：网络分区注入 — 实现方案

> 版本: v2.4-batch26
> 日期: 2026-09-13
> 状态: SDD 起草完成，禁止直接开工

---

## 1. 注入点选择

### 方案: Docker 网络隔离

```bash
# 分区: 将 node-1 从集群网络断开
docker network disconnect deploy5_raft-net raft-node-1

# 恢复: 将 node-1 重新接入
docker network connect deploy5_raft-net raft-node-1
```

**选择理由**:
- Docker 网络隔离是物理层分区，不依赖 iptables（Windows Docker 无 iptables）
- disconnect/connect 是幂等操作，可精确控制 timing
- 不需要修改容器内代码，纯外部注入

### 备选方案（不采用）

| 方案 | 不采用理由 |
|------|-----------|
| iptables 规则 | Windows Docker 无 iptables |
| 容器内 netem | 需修改容器，违反"不修改运行时代码"原则 |
| gRPC 拦截 | 需修改 proto，protoc 不可用 (RL-07) |

## 2. Timing 控制

```
T0: 集群正常，leader 已选出
T1: docker network disconnect（分区开始）
T2: 等待 partition_duration（默认 10s）
T3: 采集分区期间指标（NP-1~3）
T4: docker network connect（分区恢复）
T5: 等待 recovery_timeout（默认 30s）
T6: 采集恢复后指标（NP-4~5）
```

### 场景矩阵

| 场景 | 分区方式 | partition_duration | 预期 |
|------|---------|-------------------|------|
| np_symmetric_01 | {1,2} vs {3,4,5} | 10s | 多数派 {3,4,5} 可用 |
| np_symmetric_02 | {1,2,3} vs {4,5} | 10s | 多数派 {1,2,3} 可用 |
| np_asymmetric_01 | {1} vs {2,3,4,5} | 10s | {2,3,4,5} 正常 |
| np_bridge_01 | {1,2} - 3 - {4,5} | 10s | 无脑裂 |
| np_recovery_01 | {1,2} vs {3,4,5} → 恢复 | 10s + 30s | 30s 内追平 |
| np_cascading_01 | 分区→恢复→再分区 | 10s×3 | term 单调 |

## 3. 与现有 chaos_injector 的关系

### 扩展点

```go
// scheduler.go: BuildScenarioMatrix
case "network_partition":
    scenarios = append(scenarios, "np_symmetric_01")
    scenarios = append(scenarios, "np_symmetric_02")
    scenarios = append(scenarios, "np_asymmetric_01")
    scenarios = append(scenarios, "np_bridge_01")
    scenarios = append(scenarios, "np_recovery_01")
    scenarios = append(scenarios, "np_cascading_01")

// scheduler.go: ExecuteScenario
case contains(scenarioID, "np_"):
    return s.executeNetworkPartition(scenarioID)
```

### 新增方法

```go
func (s *Scheduler) executeNetworkPartition(scenarioID string) (*ScenarioResult, error) {
    // 1. 识别当前 leader
    // 2. docker network disconnect 隔离目标节点
    // 3. 等待 partition_duration，采集指标
    // 4. docker network connect 恢复
    // 5. 等待 recovery_timeout，采集指标
    // 6. 返回 ScenarioResult
}
```

### NodeController 扩展

```go
func (nc *NodeController) NetworkDisconnect(nodeID string) error
func (nc *NodeController) NetworkConnect(nodeID string) error
```

## 4. 仪表指认（F3 判例应用）

每个验收项必须先指认测量仪表：

| 验收项 | 仪表 | 端点 | 采集方式 |
|--------|------|------|---------|
| NP-1 | leader 计数 | /raft/stats | 分区期间每 1s 轮询 5 节点 state |
| NP-2 | TPS | loadgen | 分区期间持续写入 |
| NP-3 | 少数派 state | /raft/stats | 分区期间轮询少数派节点 |
| NP-4 | commit index | /raft/stats | 恢复后每 1s 轮询 commit index |
| NP-5 | term 序列 | /raft/stats | 全程每 1s 记录 term |

**仪表先行原则**: 先验证仪表可采集，再执行注入。