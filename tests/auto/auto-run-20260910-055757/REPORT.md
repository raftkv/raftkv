# E08 自动化长跑报告

## 运行信息
| 项目 | 值 |
|------|-----|
| Run ID | 20260910-055757 |
| 开始时间 | 2026-09-10T05:57:57.9255813+08:00 |
| 结束时间 | 2026-09-10T05:59:49.1917842+08:00 |
| 当前阶段 | 2 |
| 阈值版本 | v2.5-batch5 |

## 阶段结果
| 阶段 | 状态 | 耗时(s) |
|------|------|---------|
| P0 | PASS | 10 |
| P1 | PASS | 36 |
| P4 | FAIL | 0 |

## 20min 对照压测
验收结果文件不存在: D:\235备份文件\V2.4_Performance_Sandbox\tests\auto\auto-run-20260910-055757\phase2_accept.json

## E08 30min 终审
| 指标 | 实测 | 阈值 | 判据 | 结果 |
|------|------|------|------|------|
| TPS | 0 | 7000 | >= | FAIL |
| P99_ms | 0 | 150 | <= | PASS |
| 成功率% | 0 | 99.9 | >= | FAIL |
| 内存峰值MiB | 38.77 | 1500 | <= | PASS |
| **Overall** | | | | **FAIL** |

<details><summary>压测输出尾部</summary>

```
D:\235备份文件\V2.4_Performance_Sandbox\e04_loadtest.exe : invalid value "$dur" for flag -duration: parse error
    + CategoryInfo          : NotSpecified: (invalid value "...on: parse error:String) [], RemoteException
    + FullyQualifiedErrorId : NativeCommandError
 
Usage of D:\235备份文件\V2.4_Performance_Sandbox\e04_loadtest.exe:
  -concurrency int
    	并发goroutine数 (default 500)
  -duration duration
    	压测持续时间 (default 2m0s)
  -nodes string
    	节点HTTP端口列表 (default "9001,9002,9003,9004,9005")
  -write-ratio int
    	写入百分比(0-100) (default 20)
```

</details>

## 节点存活检查
- daijin235-node-2	Up About a minute (healthy)
- daijin235-node-5	Up About a minute (healthy)
- daijin235-node-1	Up About a minute (healthy)
- daijin235-node-4	Up About a minute (healthy)
- daijin235-node-3	Up About a minute (healthy)

## 内存采样峰值
- phase4_mem.csv : 峰值 38.77MiB (node-1)

---
## 总结: **FAIL**

