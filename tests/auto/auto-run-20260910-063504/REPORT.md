# E08 自动化长跑报告

## 运行信息
| 项目 | 值 |
|------|-----|
| Run ID | 20260910-063504 |
| 开始时间 | 2026-09-10T06:35:04.3890417+08:00 |
| 结束时间 | 2026-09-10T06:51:19.9137109+08:00 |
| 当前阶段 | 2 |
| 阈值版本 | v2.5-batch5 |

## 阶段结果
| 阶段 | 状态 | 耗时(s) |
|------|------|---------|
| P0 | PASS | 10 |
| P1 | PASS | 36 |
| P4 | FAIL | 0 |

## 20min 对照压测
验收结果文件不存在: .\tests\auto\auto-run-20260910-063504\phase2_accept.json

## E08 30min 终审
验收结果文件不存在: .\tests\auto\auto-run-20260910-063504\phase4_accept.json

<details><summary>压测输出尾部</summary>

```
──────────────────────────────────────────────────────────────────────────────────

=== E04 最终结果 ===
总发送:     8831735 (写=441568 读=8390167)
成功:       8831612
失败:       123
成功率:     100.00%
平均TPS:    9813 req/s
重定向:     77
重试:       4136 (重试成功=3272)
探测:       13 (非NotLeader失败触发findLeader)
P50:        4.6706ms
P99:        61.4081ms
Max:        1.7773317s
耗时:       15m0s
```

</details>

## 节点存活检查
- raft-node-4	Up 16 minutes (healthy)
- raft-node-3	Up 16 minutes (healthy)
- raft-node-5	Up 16 minutes (healthy)
- raft-node-2	Up 16 minutes (healthy)
- raft-node-1	Up 16 minutes (healthy)

## 内存采样峰值
- phase4_mem.csv : 峰值 681.9MiB (node-1)

---
## 总结: **FAIL**

