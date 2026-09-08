# term分裂根因调查（root-cause-degraded.md）

> **调查时间**：2026-09-08 22:50 CST
> **调查对象**：node-1 term=2644 vs node-5 term=7540（分裂差4896）
> **假设**：degraded机制切断了对被标记节点的心跳→node-5被隔离后独自反复选举term空转飙升

---

## 一、假设验证：degraded是否切断心跳？

### 1.1 代码路径审查

**degradedFollowers字段使用点**（grep `degradedFollowers` 全量）：

| 文件 | 行 | 用途 | 是否影响心跳/选举/quorum |
|------|---|------|--------------------------|
| raft.go:132 | 定义 | `degradedFollowers map[string]bool` | — |
| raft.go:425-426 | MarkFollowerDegraded | 设置标记+日志 | **否** |
| raft.go:435-436 | ClearFollowerDegraded | 清除标记+日志 | **否** |
| raft.go:460-461 | CheckGapAlerts | 告警日志中打印degraded状态 | **否** |
| raft.go:481-486 | DegradedFollowers | 返回列表供/raft/stats展示 | **否** |
| batch_sync.go:174 | MarkFollowerDegraded | 批量同步放弃时标记 | **否**（仅设置标记） |
| batch_sync.go:187 | ClearFollowerDegraded | 同步成功时清除 | **否**（仅清除标记） |
| main.go:236-237 | DegradedFollowers | /raft/stats端点展示 | **否** |

**sendHeartbeats()**（raft.go:858-957）：遍历`rn.peers`发送心跳，**不检查degradedFollowers**。

**advanceCommit()**（raft.go:961-998）：quorum计算遍历`rn.peers`计数matchIdx，**不检查degradedFollowers**。

**handleElectionTimeout()**（raft.go:588-649）：选举超时处理，**不检查degradedFollowers**。

### 1.2 结论

**假设不成立。** degraded机制**没有**切断心跳、没有排除quorum计数、没有影响选举逻辑。degradedFollowers纯粹是stats/logging层面的标记，不影响任何运行时行为。

---

## 二、真正根因：AppendEntries term检查引发正反馈循环

### 2.1 关键代码路径

**HandleAppendEntries**（raft.go:1224-1268）：

```go
// raft.go:1237-1240
if req.Term < rn.term {
    resp.Term = rn.term
    rn.mu.Unlock()
    return resp, nil  // ← 直接返回，不执行后续的electionTimer.Reset()
}
// ...
// raft.go:1268
rn.electionTimer.Reset(randomElectionTimeout())  // ← 只有term>=rn.term才到达此处
```

当follower的term高于leader的term时，AppendEntries被拒绝且**选举计时器不复位**。这是标准Raft行为，但在本场景下引发了正反馈循环。

### 2.2 node-5 term飙升轨迹（日志证据）

| 时间戳(UTC) | node-5 term | node-5 leader | node-1 term | 事件 |
|-------------|-------------|---------------|-------------|------|
| 12:39:30 | 2638 | node-1 | 2638 | node-1当选，node-5接受 |
| 12:39:40 | 2638 | node-1 | 2638 | 正常跟随 |
| 12:40:00 | 2638 | node-1 | 2638 | 正常跟随 |
| 12:40:10 | 2638 | node-1 | 2638 | 最后一次正常心跳 |
| **12:40:20** | **2641** | **(空)** | 2638 | **node-5选举超时→Candidate→term跳+3** |
| 12:40:30 | 2648 | (空) | 2644 | node-5 term已>node-1 term，正反馈开始 |
| 12:40:40 | 2652 | (空) | 2644 | 持续飙升 |
| 12:40:50 | 2660 | (空) | 2644 | 持续飙升 |
| 12:41:00 | 2667 | (空) | 2644 | 持续飙升 |
| 12:41:10 | 2674 | (空) | 2644 | 持续飙升 |
| 12:42:00 | 2708 | (空) | 2644 | 持续飙升 |
| 12:43:00 | 2753 | (空) | 2644 | 持续飙升 |
| 12:44:09 | 2801 | (空) | 2644 | 持续飙升 |
| ... | ... | (空) | 2644 | 持续飙升（每10s约+6~8） |
| 14:09+ | ~7540 | (空) | 2644 | 最终值 |

### 2.3 正反馈循环机制

```
node-5选举超时(12:40:20)
    ↓
node-5 term: 2638→2641, 成为Candidate
    ↓
node-1发送心跳(term=2638/2644) → node-5拒绝(req.Term < rn.term=2641)
    ↓
拒绝返回前不执行electionTimer.Reset() (raft.go:1237-1240直接return)
    ↓
node-5选举计时器不复位 → 再次超时 → term再+1 → 再次Candidate
    ↓
node-5发起RequestVote(term越来越高) → 但无法赢得quorum(其他节点degraded/gap大)
    ↓
选举失败 → candidateFailCount++ → 但term已升高 → 永远>node-1的term
    ↓
node-1的心跳永远被拒绝 → 正反馈循环 → term无限飙升
```

### 2.4 为什么node-5无法赢得选举？

node-5 term虽高但无法成为leader，因为：
1. 其他节点(node-2/3/4)的matchIdx=0（gap=58627），日志严重落后
2. node-5的日志可能也落后（它一直在选举而非接收日志复制）
3. RequestVote要求候选人的日志至少与多数节点的日志一样新（Raft up-to-date检查）
4. 5节点中node-1的日志最新，node-2/3/4落后，node-5可能也落后→无法获多数票

### 2.5 为什么node-1不stepDown？

node-1的sendHeartbeats在收到resp.Term > term时应调用stepDown（raft.go:934-936）。但node-1仍为Leader(term=2644)。可能原因：
1. **gRPC调用失败**：node-1到node-5的RPC超时/错误(err!=nil)，node-1从未收到node-5的高term响应
2. **stepDown后立即重选**：node-1收到2641后stepDown→选举→win(term=2642)→但node-5已在2648→再次stepDown→...→最终node-1在2644稳定（可能此时gRPC到node-5已完全失败，不再收到高term响应）

---

## 三、degraded的间接影响

虽然degraded不直接导致term分裂，但**间接加剧了问题**：

1. **batch_sync.go:174**：批量同步放弃时MarkFollowerDegraded→follower日志停止同步→gap增大
2. 日志落后→RequestVote时up-to-date检查失败→node-5无法赢得选举
3. node-5无法赢→继续选举→term继续飙升→正反馈循环无法打破

degraded本身不切断心跳，但**批量同步放弃导致日志落后**，间接阻止了node-5通过正常选举翻盘。

---

## 四、结论

| 项目 | 结论 |
|------|------|
| 假设(degraded切断心跳) | **不成立**。degradedFollowers不参与心跳/选举/quorum逻辑 |
| term分裂直接根因 | **AppendEntries term检查(raft.go:1237-1240)引发正反馈循环**：node-5选举超时→term>leader→心跳被拒→计时器不复位→持续选举→term飙升 |
| degraded间接作用 | 批量同步放弃导致日志落后→node-5无法赢得选举→正反馈无法打破 |
| 严重性 | term分裂比失败计数更严重：Raft一致性被破坏，node-5 term=7540>>node-1 term=2644 |

**修复G需同时解决两个问题**：
1. **degraded自清除**：恢复健康后清除标记，恢复quorum
2. **term分裂自愈**：follower收到更低term的AppendEntries时，应重置选举计时器（即使拒绝请求），防止term无限飙升。或：Candidate连续失败N次后回退term到集群已知水平。