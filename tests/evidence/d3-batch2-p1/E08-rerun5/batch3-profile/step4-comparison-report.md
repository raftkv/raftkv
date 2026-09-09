# D3-batch3 步骤4·修复后画像对照报告

## 对照基准
- 基线: profile-baseline-r2.csv (13轮, 20min, 100并发, write-ratio=20)
- 修复后: profile-postfix-loadtest-output.txt (20min, 同参数) + profile-postfix-short.csv (5min补充采样)

## 逐项对照

| 指标 | 基线 r2 | 修复后 | 变化 | 期望 | 达标 |
|------|---------|--------|------|------|------|
| 平均 TPS | 3,690 | 7,513 | ↑103.6% | 趋近锚7,877 | ✅ |
| P99 | 471ms | 241ms | ↓48.8% | 改善 | ✅ |
| P50 | 9.3ms | 13.3ms | ↑42.7% | - | ⚠️ 略升 |
| 总请求 | 4,428,587 | 9,015,992 | ↑103.6% | - | ✅ |
| 失败数 | 0 | 4,561 (0.05%) | - | 0 | ❌ |
| anon 峰值 | 3.01GB | 4.42GB | ↑46.8% | <500MB | ❌ |
| GC 骤降 | 2次 | 1-2次/5min窗 | - | 大幅减少 | ❌ |
| 5/5 存活 | 是(20min) | 否(node-4/5 OOM) | - | 5/5 | ❌ |

## 内存形态（只记录不下结论）

### 锯齿模式
- anon 呈周期性锯齿: 峰 2-5GB → GC 降至 300MB-1GB → 再峰
- 周期约 30-60s
- 峰值幅度与基线相似（未降）

### 采样数据 (profile-postfix-short.csv 节选, 单位: bytes)
```
20:58:30  node1=2.56GB  node2=4.85GB  node3=3.95GB  (峰)
20:59:05  node1=271MB   node2=2.01GB  node3=3.20GB  (部分GC)
21:02:17  node1=990MB   node2=302MB   node3=272MB   (GC骤降)
21:02:34  node1=2.63GB  node2=3.74GB  node3=4.42GB  (再峰)
```

### OOM 记录
- node-5: Exit 137 at ~20:45 (20min压测约13min处)
- node-4: Exit 137 at ~20:47 (20min压测约15min处)
- 3/5 存活: node-1, node-2, node-3

## OnCommit Snapshot 代码形态 (raft_pipeline.go:258-293)
- `snapshotThreshold` 触发 `p.storage.Snapshot()` 每 N 次 commit
- `totalCommitted` 快照后重置为 0
- Snapshot 只压缩 WAL 文件，**不截断 rn.logs**
- rn.logs 无限增长（缺陷 A 确认）

## TPS 时序形态
- 0-440s (7.3min): TPS ~10,000-12,000, 0 失败 (B1+B2 效果显著)
- 445s+: 失败开始出现 (4,561 总失败)
- 1000s+: 周期性 TPS 骤降至 ~400 (疑似 GC stop-the-world)
- 平均 TPS 7,513 (被后半段拖低，前半段接近锚 7,877)

## 结论
- B1+B2 **部分有效**: TPS 翻倍、P99 减半，前 7min 接近锚性能
- **未达内存期望**: anon 仍飙至 4.42GB，2 节点 OOM
- **根因残留**: 缺陷 A (rn.logs 无 compaction) 未修，持续增长致 OOM
- 建议步骤5 30min 终审后，无论通过与否，排期缺陷 A 修复