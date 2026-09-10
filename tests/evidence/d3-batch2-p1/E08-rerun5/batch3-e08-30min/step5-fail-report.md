# D3-batch3 E08 30min 终审·失败报告

## 验收标准
- 5/5 存活 → **FAIL (3/5)**
- 无 OOM → **FAIL (2 OOM: node-1, node-4)**
- 失败率收敛 → **PARTIAL (99.89%, 11,803 失败)**

## E08 30min 结果
| 指标 | 值 |
|------|-----|
| 总发送 | 10,393,740 (写=519,687 读=9,874,053) |
| 成功 | 10,381,937 |
| 失败 | 11,803 (0.11%) |
| 平均 TPS | 5,774 req/s |
| P50 | 4.05ms |
| P99 | 5.12s |
| Max | 7.25s |
| 耗时 | 30m0s |

## 节点存活
| 节点 | 状态 | OOM 时间 |
|------|------|----------|
| node-1 | Exited (137) | ~17min 处 |
| node-2 | RUNNING | - |
| node-3 | RUNNING | - |
| node-4 | Exited (137) | ~20min 处 |
| node-5 | RUNNING | - |

## 时序形态
- 0-450s (7.5min): TPS ~11,000-12,000, 0 失败 (B1+B2 效果显著)
- 455s: 首次失败出现 (100 失败)
- 1015s: TPS 骤降至 ~400, P99 飙升 (疑似 GC stop-the-world)
- 1125s-1200s: 多次 TPS 骤降 + 失败攀升
- 1570s+: 持续低 TPS (~300-500), 高 P99 (5-8s)
- node-1 OOM @ ~17min, node-4 OOM @ ~20min

## 根因分析
- **B1+B2 部分有效**: 前 7.5min TPS ~11,500 (vs 基线 3,690, ↑211%)
- **缺陷 A 未修**: rn.logs 无 compaction，持续增长
  - sendHeartbeats B2 增量构造: 当 follower 落后时仍需拷贝大量 entries
  - rn.logs 本身不释放: Command+SM3Hash []byte 持续累积
- **OOM 机制**: anon 锯齿增长 (2-5GB 峰) → 5 节点合计超 WSL2 16GB → 内核杀容器

## 对比 20min 画像轮
| 指标 | 20min | 30min |
|------|-------|-------|
| TPS | 7,513 | 5,774 |
| 失败 | 4,561 (0.05%) | 11,803 (0.11%) |
| OOM 节点 | 2 (node-4,5) | 2 (node-1,4) |
| P99 | 241ms | 5.12s |

## 结论
- **缺陷 B 修复 (B1+B2) 部分有效**: TPS 显著提升，前 7.5min 接近锚性能
- **缺陷 A (rn.logs 无 compaction) 是 OOM 根因**: 必须修复才能通过 E08
- **建议**: 排期缺陷 A 修复 (rn.logs compaction/truncate)，作为 v2.5-batch4 优先项

## 证据文件
- e08-30min-output.txt: 30min 压测完整输出
- step4-comparison-report.md: 20min 画像对照报告
- profile-postfix-short.csv: 5min 内存采样
- batch3-B1B2-raft.diff: B1+B2 代码改动 diff