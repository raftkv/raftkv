# E08 自动化长跑报告

## 运行信息
| 项目 | 值 |
|------|-----|
| Run ID | 20260910-123049 |
| 开始时间 | 2026-09-10T12:30:49.9101429+08:00 |
| 结束时间 | 2026-09-10T12:46:50.0153056+08:00 |
| 当前阶段 | 5 |
| 阈值版本 | v2.5-batch5 |

## 阶段结果
| 阶段 | 状态 | 耗时(s) |
|------|------|---------|
| P0 | PASS | 9 |
| P1 | PASS | 37 |
| P4 | PASS | 914 |

## 20min 对照压测
验收结果文件不存在: D:\235备份文件\V2.4_Performance_Sandbox\tests\auto\auto-run-20260910-123049\phase2_accept.json

## E08 30min 终审
| 指标 | 实测 | 阈值 | 判据 | 结果 |
|------|------|------|------|------|
| TPS | 9925 | 7000 | >= | PASS |
| P99_ms | 62.9987 | 150 | <= | PASS |
| 成功率% | 100 | 99.9 | >= | PASS |
| 内存峰值MiB | 610.8 | 1500 | <= | PASS |
| **Overall** | | | | **PASS** |

<details><summary>压测输出尾部</summary>

```
──────────────────────────────────────────────────────────────────────────────────

=== E04 最终结果 ===
总发送:     8932227 (写=446575 读=8485652)
成功:       8932130
失败:       97
成功率:     100.00%
平均TPS:    9925 req/s
重定向:     78
重试:       4457 (重试成功=3337)
探测:       19 (非NotLeader失败触发findLeader)
P50:        4.574ms
P99:        62.9987ms
Max:        169.4034ms
耗时:       15m0s
```

</details>

## 节点存活检查
- daijin235-node-1	Up 15 minutes (healthy)
- daijin235-node-4	Up 15 minutes (healthy)
- daijin235-node-5	Up 15 minutes (healthy)
- daijin235-node-2	Up 15 minutes (healthy)
- daijin235-node-3	Up 15 minutes (healthy)

## 内存采样峰值
- phase4_mem.csv : 峰值 610.8MiB (node-4)

---
## 总结: **PASS**

