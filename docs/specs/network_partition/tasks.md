# 战役III：网络分区注入 — 任务拆解草案

> 版本: v2.4-batch26
> 日期: 2026-09-13
> 状态: SDD 起草完成，禁止直接开工
> 目标批次: batch27+

---

## 任务拆解

### T1: NodeController 网络隔离方法（batch27）

| 字段 | 内容 |
|------|------|
| **任务** | 实现 NetworkDisconnect/NetworkConnect 方法 |
| **仪表指认** | docker network disconnect/connect 命令本身为仪表，exit code=0 为成功 |
| **验收** | 手动断开 node-1，curl 不可达；接入后 curl 可达 |
| **依赖** | 无 |

### T2: 网络分区场景执行器（batch27）

| 字段 | 内容 |
|------|------|
| **任务** | 实现 executeNetworkPartition 方法 + 6 个场景 |
| **仪表指认** | /raft/stats 端点采集 state/term/commit_index |
| **验收** | 6 场景全执行，证据落盘 |
| **依赖** | T1 |

### T3: NP 验收判定脚本（batch28）

| 字段 | 内容 |
|------|------|
| **任务** | judge_batch28.py 实现 NP-1~5 判定逻辑 |
| **仪表指认** | 从 scenario JSON 提取 leader_count/tps/commit_index/term 序列 |
| **验收** | judge 输出 NP-1~5 逐项 PASS/FAIL |
| **依赖** | T2 |

### T4: 可观测性端点补全（batch27，与任务五合并）

| 字段 | 内容 |
|------|------|
| **任务** | 补齐选举轮次计数/分区期间心跳丢失计数端点 |
| **仪表指认** | 新端点本身为仪表，附测量用例 |
| **验收** | 端点返回非空数据 |
| **依赖** | 无（仪表先行） |

### T5: 回归线扩充（batch28）

| 字段 | 内容 |
|------|------|
| **任务** | regression.yaml 追加 NP-1~5 为 REG-9~13 |
| **仪表指认** | regression_gate.py 校验 NP-1~5 |
| **验收** | 全回归门 13 线全绿 |
| **依赖** | T3 |

### T6: 双轨对比（batch28）

| 字段 | 内容 |
|------|------|
| **任务** | quorumbench 平台跑网络分区套件，与本仓库对照 |
| **仪表指认** | quorumbench 独立判定 vs 本仓库 judge |
| **验收** | 双轨结论一致 |
| **依赖** | T3 |

---

## 执行顺序

```
T4（仪表先行）→ T1 → T2 → T3 → T5 → T6
batch27: T4 + T1 + T2
batch28: T3 + T5 + T6
```

## 仪表先行原则

**每个任务的验收线必须先指认测量仪表**（F3 判例的直接应用）：
- T1: docker 命令 exit code
- T2: /raft/stats 端点
- T3: scenario JSON 字段
- T4: 新端点返回值
- T5: regression_gate.py 输出
- T6: quorumbench 判定结果