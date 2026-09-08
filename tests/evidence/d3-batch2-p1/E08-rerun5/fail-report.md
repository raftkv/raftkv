# E08第5轮 FAIL报告 — Docker OOM三次复发(8GB+16GB均不足)

## 执行信息
- 第5轮第1次(8GB): 2026-09-09 01:59-02:28, 29.6min, node-1/2/4 Exited(137)
- 第5轮第2次(16GB): 2026-09-09 02:34-03:04, 29.6min, node-1/2/3 Exited(137)
- 参数: 100并发 × 7200s × 20%写

## Docker配额整改记录
1. 探测后端: WSL2确认
2. 写.wslconfig memory=8GB → wsl --shutdown → 重启Docker → VM=7.75GiB ✓
3. 8GB仍OOM → 改.wslconfig memory=16GB → 重启 → VM=15.62GiB ✓
4. 16GB仍OOM

## 根因分析（更新）
**这不是Docker配额问题，而是容器内存持续增长（疑似内存泄漏）。**

证据：
- 空载时5节点总计~925MiB（每节点120-210MiB）
- 压测~30分钟后3个节点OOM
- 8GB VM: 3节点OOM, 2节点存活(~144MiB)
- 16GB VM: 3节点OOM, 2节点存活(~144MiB)
- OOM总是发生在~30分钟、总是3个节点

推断：
- 每个节点在压测期间内存持续增长（~200MiB → ~5GB+/30min）
- 16GB/3≈5.3GB，3个节点内存达到此阈值被OOM
- 存活节点（Leader+1 follower）内存稳定~144MiB
- 疑似WAL写入/日志累积/gRPC缓冲导致内存增长

## 16GB轮压测数据（被中断前，不作验收依据）
- 运行: ~1765s (29.4min)
- 总发送: 3,362,937
- 成功: 3,218,940 (95.7%)
- 失败: 143,997
- TPS: 1300-2800
- P99: 1320-1380ms
- Max: 3946ms

## 修复G/E增强成效
本轮因OOM中断，无法评估修复G/E增强成效。
不记为代码回归——环境事故非代码失败。

## 建议
1. **缩短压测时长**: 先用20min(duration=1200s)验证修复G/E增强成效（在OOM前完成）
2. **调查内存泄漏**: 排查WAL写入/日志累积/gRPC缓冲的内存增长
3. **限制容器内存**: docker compose加mem_limit让OOM更可控

## 证据文件
- raw-output.txt: 16GB轮压测输出
- docker-snapshot-30min.txt: 8GB轮30min快照
- docker-snapshot-30min-16gb.txt: 16GB轮30min快照
- node-{1,2,3}-exit137-16gb-log.txt: OOM容器日志