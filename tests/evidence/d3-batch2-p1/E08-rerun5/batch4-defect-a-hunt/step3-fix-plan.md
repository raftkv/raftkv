# D3-batch4 步骤3·缺陷A修复方案呈报

> **本批是查证批，本文件仅呈报方案，不实施任何代码改动。**
> 等待用户批复后，在后续修复批中实施。

## 1. 定罪回顾

| 维度 | 结论 |
|------|------|
| 真凶 | `EncryptedStorage.Snapshot()` → `ReplayAll()` → `json.Marshal()` O(N²) 分配风暴 |
| 机制 | 每 10K commit 触发 Snapshot，每次全量装载+序列化全部历史日志；39 次调用总处理 7.8M 条次 |
| 分配压力 | node-4: 63.9 GB / 15min (71 MB/s), node-5: 46.9 GB / 15min (52 MB/s) |
| 并发隐患 | `totalCommitted` 非原子，可并发触发 Snapshot |
| 缺陷A 关系 | rn.logs 无 compaction 是必要条件（日志线性增长 → Snapshot 处理量线性增长 → O(N²) 总分配） |

## 2. 方案矩阵

### 方案 A：Snapshot 增量化（治本·推荐）

**思路**：Snapshot 不再全量 ReplayAll + json.Marshal，改为只序列化 WAL 中的新日志条目，追加到现有快照文件。

**快照格式变更**：
```
旧格式: snapshot.gz = gzip(json([log1, log2, ..., logN]))
新格式: snapshot.gz = gzip(json(log1) + "\n" + json(log2) + "\n" + ... + json(logN))  // NDJSON
```

| 维度 | 内容 |
|------|------|
| 改动范围 | `raft_storage.go`: Snapshot() + ReplayAll()；`raft_wal.go`: 无 |
| 共识语义影响 | **无**。快照内容不变（仍是全部日志），仅序列化格式从 JSON Array 改为 NDJSON。Raft 协议层无感知。 |
| 回归要求 | (1) 快照恢复正确性：NDJSON 反序列化须得到与原 JSON Array 相同的 []RaftLog (2) 快照压缩率验证 (3) 跨快照边界读取验证 |
| 预期效果 | O(N²) → O(N)：每次 Snapshot 仅处理 10K 新日志，总处理 394K 条次（vs 7.8M），降低 19.8x |
| 风险等级 | 中（格式变更需验证兼容性，但语义不变） |
| 代码量 | ~80 行改动 |

**关键改动点**：
```go
// Snapshot — 增量追加
func (es *EncryptedStorage) Snapshot() (int, int64, error) {
    es.wal.Flush()
    entries, _ := es.wal.Replay()        // 仅读 WAL 新日志（不读旧快照）
    // 用 json.Encoder 逐条写入 gzip writer（流式，无大对象）
    for _, entry := range entries {
        decrypted, _ := sm4CTRDecrypt(es.key, entry.Data)
        var log RaftLog
        json.Unmarshal(decrypted, &log)
        enc.Encode(log)                  // 追加到现有快照
    }
    // 重置 WAL
}
```

---

### 方案 B：Snapshot 流式化（治本·降低峰值内存）

**思路**：保持全量 ReplayAll，但用 `json.Encoder` 流式写入 gzip writer，避免 `json.Marshal` 产生的大 []byte 临时对象。

| 维度 | 内容 |
|------|------|
| 改动范围 | `raft_storage.go`: Snapshot() |
| 共识语义影响 | **无**。快照格式不变，仅序列化方式从 `json.Marshal(logs)` 改为 `json.Encoder` 流式写入。 |
| 回归要求 | (1) 快照内容一致性验证 (2) 流式写入的 JSON 须与全量 Marshal 字节级一致（或至少语义一致） |
| 预期效果 | 峰值内存降低 ~40%（消除 json.Marshal 的 419-708 MB flat 分配），但 **O(N²) 仍存在**（ReplayAll 仍全量装载） |
| 风险等级 | 低（仅序列化方式变更） |
| 代码量 | ~30 行改动 |

**局限**：不消除 O(N²) 根因，仅降低峰值。需与方案 C 或 D 配合才能根治。

---

### 方案 C：rn.logs Compaction/Truncate（治本·Raft 标准做法）

**思路**：Snapshot 后截断 `rn.logs`，仅保留快照点之后的日志。引入 `snapshotIndex` / `snapshotTerm` 偏移。

| 维度 | 内容 |
|------|------|
| 改动范围 | `raft.go`: logs 访问全部需加偏移（~30 处）；`raft_storage.go`: Snapshot() 回调截断；新增 `snapshotIndex`/`snapshotTerm` 字段 |
| 共识语义影响 | **有**。Raft 日志变为 `[snapshotIndex+1, lastLogIndex]` 区间。需确保：(1) 选举时 lastLogIndex/Term 正确 (2) AppendEntries 的 PrevLogIndex 匹配 (3) 已截断的日志请求返回快照（InstallSnapshot RPC） |
| 回归要求 | (1) Raft safety：已提交日志不丢失（快照包含所有已提交日志） (2) 选举安全性：截断后选举仍正确 (3) AppendEntries 一致性：PrevLogIndex < snapshotIndex 时需 InstallSnapshot (4) 需实现 InstallSnapshot RPC handler（当前代码未实现） |
| 预期效果 | rn.logs 条数从 O(N) 降至 O(snapshotInterval)=O(10K)，Snapshot 处理量从 O(N) 降至 O(10K)，总分配 O(N) |
| 风险等级 | **高**（涉及 Raft 协议核心，需实现 InstallSnapshot RPC，回归面广） |
| 代码量 | ~200-300 行改动 + InstallSnapshot RPC 实现 |

**前置条件**：需先实现 InstallSnapshot RPC handler（当前代码仅有 AppendEntries，无 InstallSnapshot）。这是 Raft log compaction 的标准配套，但工程量较大。

---

### 方案 D：totalCommitted 原子化 + Snapshot 互斥（安全修复·必做）

**思路**：将 `totalCommitted` 改为 `atomic.Int64`，加 `sync.Mutex` 确保 Snapshot 串行执行。

| 维度 | 内容 |
|------|------|
| 改动范围 | `raft_pipeline.go`: OnCommit() + RaftPipeline 结构体 |
| 共识语义影响 | **无**。仅并发安全修复，功能不变。 |
| 回归要求 | (1) Snapshot 不阻塞 Raft 主循环（OnCommit 须快速返回，Snapshot 可异步） (2) 原子计数器正确性 |
| 预期效果 | 消除并发 Snapshot 导致的内存翻倍（node-5 的 1.86GB 中部分来自并发 Snapshot） |
| 风险等级 | 低 |
| 代码量 | ~20 行改动 |

**注意**：若 Snapshot 仍同步执行，mutex 会导致 OnCommit 阻塞。建议 Snapshot 异步执行（放入独立 goroutine + channel 通知）。

---

### 方案 E：Snapshot 频率调整（治标·快速缓解）

**思路**：将 `snapshotThreshold` 从 10K 调至 50K 或 100K，减少 Snapshot 调用次数。

| 维度 | 内容 |
|------|------|
| 改动范围 | `raft_pipeline.go`: snapshotThreshold 默认值 |
| 共识语义影响 | **无**。仅频率调整。 |
| 回归要求 | (1) WAL 文件增大后的磁盘空间验证 (2) 恢复时 ReplayAll 时间增长 |
| 预期效果 | 39 次 → 8 次（50K）或 4 次（100K），总分配降低 5-10x，但 **O(N²) 仍存在** |
| 风险等级 | 极低 |
| 代码量 | 1 行改动 |

**局限**：不消除根因，仅延缓 OOM。WAL 文件增大可能引入其他问题（磁盘 IO、恢复时间）。

---

### 方案 F：Snapshot 后强制 GC（治标·辅助）

**思路**：Snapshot 完成后调用 `runtime.GC()` 强制回收临时对象。

| 维度 | 内容 |
|------|------|
| 改动范围 | `raft_pipeline.go`: OnCommit() Snapshot 后 |
| 共识语义影响 | **无**。 |
| 回归要求 | (1) GC 停顿对延迟的影响 (2) GC 频率验证 |
| 预期效果 | 降低死对象驻留时间，缓解 anon 膨胀，但 **不减少分配量** |
| 风险等级 | 低（但 `runtime.GC()` 是 STW，可能影响 P99） |
| 代码量 | 2 行改动 |

## 3. 推荐组合

| 组合 | 方案 | 效果 | 风险 | 工程量 |
|------|------|------|------|--------|
| **推荐1** | A + D | O(N²)→O(N) + 并发安全 | 中 | ~100 行 |
| 推荐2 | B + C + D | 全根治（流式 + compaction + 安全） | 高 | ~300 行 + InstallSnapshot RPC |
| 推荐3 | E + D + F | 缓解（降频 + 安全 + 强制GC） | 低 | ~25 行 |
| 最小 | D only | 仅修并发隐患，O(N²) 残留 | 低 | ~20 行 |

### 推荐1（A + D）理由
- **方案A** 消除 O(N²) 根因（增量快照），不涉及 Raft 协议层，风险可控
- **方案D** 修复并发隐患，必做
- 组合后预期：总分配从 46.9 GB 降至 ~2 GB（394K × 5KB = 1.97 GB），live heap 从 1.86 GB 降至 <200 MB
- 不需要实现 InstallSnapshot RPC（方案C 的前置条件），工程量可控

### 推荐2（B + C + D）理由
- **方案C** 是 Raft log compaction 的标准做法，根治 rn.logs 无限增长
- 但需实现 InstallSnapshot RPC（当前缺失），工程量大、回归面广
- 适合长期演进，不适合本批紧急修复

## 4. 不推荐单独实施的方案

| 方案 | 理由 |
|------|------|
| B 单独 | 不消除 O(N²)，仅降峰值 |
| E 单独 | 不消除根因，仅延缓 OOM |
| F 单独 | 不减少分配量，仅加速回收 |
| C 单独 | 无 D 则并发隐患仍存 |

## 5. 验收标准（无论选哪个组合）

| 指标 | 目标 |
|------|------|
| 20min 压测 OOM | 0/5 节点 OOM |
| anon 峰值 | < 1.5 GB / 节点 |
| TPS | ≥ 6,000 req/s（不低于 batch4 水平） |
| P99 | ≤ 300 ms |
| 成功率 | ≥ 99.9% |
| alloc_space / 15min | < 5 GB / 节点（vs 当前 47-64 GB） |

## 6. 呈报完毕

**本文件仅呈报方案，不实施任何代码改动。**

请用户批复选择哪个组合方案，将在后续修复批中实施。