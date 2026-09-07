# R-03 写路径deadline传递评估报告

> **约束**: 只读诊断，未修改任何代码。

## 1. 写路径代码分析

### 1.1 Propose() (raft.go:918-965)

```go
func (rn *RaftNode) Propose(command []byte) (int64, error) {
    // ... 状态检查 ...
    index := int64(len(rn.logs)) + 1
    entry := RaftLog{Index: index, Term: rn.term, Command: command}
    rn.logs = append(rn.logs, entry)
    // ... 解锁 ...

    for attempt := 0; attempt < 50; attempt++ {
        rn.sendHeartbeats()           // 触发AppendEntries复制
        // 检查是否committed
        if committed { return index, nil }
        if !stillLeader { return 0, err }
        time.Sleep(20 * time.Millisecond)   // 固定等待
    }
    return 0, fmt.Errorf("commit timeout: index=%d not committed after 1s", index)
}
```

### 1.2 关键发现

| 项 | 现状 | 风险 |
|---|------|------|
| 超时机制 | 固定循环50次×20ms=**1s** | 高负载下1s可能不足 |
| context.Deadline | **无** | deadline不可传递到下游 |
| AppendEntries超时 | rpcTimeout=500ms (raft.go:816) | 独立于Propose超时 |
| WAL flush超时 | 需检查pipeline路径 | — |

## 2. WAL flush路径

### 2.1 Pipeline OnCommit (raft_pipeline.go)

Propose成功后，日志通过OnCommit回调进入WAL加密持久化。
WAL flush使用批量fsync，超时配置由`WAL_FLUSH_INTERVAL_MS`环境变量控制。

### 2.2 ProposeAndWait (raft_adapters.go:1359)

```go
func (p *RaftNodeProposer) ProposeAndWait(command []byte, timeout time.Duration) error
```

存在带超时的ProposeAndWait接口，但HTTP API的`/raft/propose`端点调用的是`Propose()`（无deadline）。

## 3. 评估结论

| 项 | 结论 |
|---|------|
| Propose固定1s超时 | 5节点低负载下**足够**（E03 PASS, 20条/3s） |
| 高负载风险 | 1s固定超时在高负载/大规模下可能不足 |
| deadline传递 | **缺失**，Propose不使用context.Deadline |
| 修复建议 | 建议改为context.WithTimeout，使deadline可传递（P-Ⅱ压测后评估是否需要） |
| 紧迫性 | **低**，当前5节点拓扑下1s足够，batch2压测验证后再决定 |

## 4. 与R-01/R-02的关联

写路径的1s超时与R-01的35s收敛无直接关联（Propose超时影响写入延迟，不影响分区恢复）。
R-01修复后（gRPC重连退避缩短），写路径在高负载下的AppendEntries延迟应改善。