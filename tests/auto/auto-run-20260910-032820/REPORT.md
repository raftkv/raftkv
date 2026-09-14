# E08 自动化长跑报告

## 运行信息
| 项目 | 值 |
|------|-----|
| Run ID | 20260910-032820 |
| 开始时间 | 2026-09-10T03:28:20.3426915+08:00 |
| 结束时间 | 2026-09-10T03:29:48.8466442+08:00 |
| 当前阶段 | 5 |
| 阈值版本 | v2.5-batch5 |

## 阶段结果
| 阶段 | 状态 | 耗时(s) |
|------|------|---------|
| P0 | PASS | 10 |
| P1 | PASS | 16 |
| P2 | PASS | 31 |
| P3 | PASS | 0 |
| P4 | PASS | 31 |

## 20min 对照压测
| 指标 | 实测 | 阈值 | 判据 | 结果 |
|------|------|------|------|------|
| TPS | 11110 | 7000 | >= | PASS |
| P99_ms | 61.7522 | 150 | <= | PASS |
| 成功率% | 100 | 99.9 | >= | PASS |
| 内存峰值MiB | 41.37 | 1500 | <= | PASS |
| **Overall** | | | | **PASS** |

<details><summary>压测输出尾部</summary>

```

=== E04 最终结果 ===
总发送:     333312 (写=16641 读=316671)
成功:       333312
失败:       0
成功率:     100.00%
平均TPS:    11110 req/s
重定向:     0
重试:       0 (重试成功=0)
探测:       0 (非NotLeader失败触发findLeader)
P50:        4.6979ms
P99:        61.7522ms
Max:        510.9606ms
耗时:       30s

```

</details>

## E08 30min 终审
| 指标 | 实测 | 阈值 | 判据 | 结果 |
|------|------|------|------|------|
| TPS | 10606 | 7000 | >= | PASS |
| P99_ms | 60.0109 | 150 | <= | PASS |
| 成功率% | 100 | 99.9 | >= | PASS |
| 内存峰值MiB | 59.81 | 1500 | <= | PASS |
| **Overall** | | | | **PASS** |

<details><summary>压测输出尾部</summary>

```

=== E04 最终结果 ===
总发送:     318176 (写=15878 读=302298)
成功:       318176
失败:       0
成功率:     100.00%
平均TPS:    10606 req/s
重定向:     0
重试:       0 (重试成功=0)
探测:       0 (非NotLeader失败触发findLeader)
P50:        4.3131ms
P99:        60.0109ms
Max:        131.5045ms
耗时:       30s

```

</details>

## 节点存活检查
- raft-node-5	Up About a minute (healthy)
- raft-node-2	Up About a minute (healthy)
- raft-node-4	Up About a minute (healthy)
- raft-node-3	Up About a minute (healthy)
- raft-node-1	Up About a minute (healthy)

## 内存采样峰值
- phase2_mem.csv : 峰值 41.37MiB (node-1)
- phase4_mem.csv : 峰值 59.81MiB (node-1)

---
## 总结: **PASS**

