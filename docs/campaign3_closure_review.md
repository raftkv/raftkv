# 战役III 闭案预审材料 — Campaign3 Closure Review

> 版本: v2.4-batch30-campaign3-closure
> 日期: 2026-09-14
> 用途: 核验 T1~T5 全量完成状态+缺口清单，供晨审批阅

---

## 1. 战役III 全量核验

### T1: SDD 设计（batch27）

| 字段 | 值 |
|------|-----|
| 状态 | ✅ 完成 |
| 批次 | batch27 |
| 产物 | docs/specs/network_partition/ SDD 文档 |
| schema 留证 | N/A（设计阶段，非测试场景） |
| 缺口 | 无 |

### T2: 网络分区执行器（batch28）

| 字段 | 值 |
|------|-----|
| 状态 | ✅ 完成 |
| 批次 | batch28 |
| 产物 | cmd/chaos_injector/scheduler.go executePartition |
| 场景数 | 6（NP-1~5 对称/非对称分区） |
| schema 留证 | legacy（batch28 存量留证，可按需补转） |
| 缺口 | 无 |

### T3: NP 验收判定（batch28）

| 字段 | 值 |
|------|-----|
| 状态 | ✅ 完成 |
| 批次 | batch28 |
| 产物 | tests/contracts/judge_batch23.py NP-1~5 判定 |
| 验收项 | NP-1(无脑裂) NP-2(多数派可用) NP-3(少数派不选举) NP-4(commit追平) NP-5(term单调) |
| schema 留证 | legacy |
| 缺口 | 无 |

### T4: 复合场景首探（batch29）

| 字段 | 值 |
|------|-----|
| 状态 | ✅ 完成 |
| 批次 | batch29 |
| 产物 | cmd/chaos_injector/scheduler.go executeCompositePartitionDiskFull |
| 场景数 | 3（comp_pdf_01~03: 分区+磁盘满复合） |
| schema 留证 | ✅ 已补转 schema 格式（batch30 任务四） |
| schema 路径 | tests/evidence/d3-batch29/schema/schema_scenario_comp_pdf_0{1..3}.json |
| 缺口 | 无 |

### T5: 复合场景验收（batch29）

| 字段 | 值 |
|------|-----|
| 状态 | ✅ 完成 |
| 批次 | batch29 |
| 产物 | tests/evidence/d3-batch29/composite_verdict.json |
| 验收项 | COMP-1~5（无脑裂/少数派不选举/term单调/commit追平/恢复确认） |
| N | 3 |
| CV | 0.0% < 15% |
| 结果 | 3/3 PASS |
| schema 留证 | ✅ 已补转 |
| 缺口 | 无 |

### T6: quorumbench 双轨对照

| 字段 | 值 |
|------|-----|
| 状态 | ❌ 挂账 L-29-1 |
| 批次 | batch29（挂账）→ batch31（清偿） |
| 原因 | quorumbench 真平台不可达 |
| 处置 | 三次重试+挂账转晨审，本地双轨模拟=regression_gate 10线全绿 |
| 缺口 | 待晨审裁决：是否接受本地双轨模拟作为持续替代 |

---

## 2. 全部场景 schema 留证状态

| 场景 | 批次 | schema 留证 | 状态 |
|------|------|------------|------|
| NP-1~5 (6场景) | batch28 | legacy | 保留原格式 |
| comp_pdf_01~03 (3场景) | batch29 | ✅ 已补转 | schema 格式 |
| 总计 | — | 3/9 schema, 6/9 legacy | — |

**说明**: batch28 NP 场景为存量留证，可按需补转 schema 格式。batch29 复合场景已补转。batch31 起新场景强制按 schema 留证。

---

## 3. 缺口清单

| 序号 | 缺口 | 严重度 | 处置 | 目标批次 |
|------|------|--------|------|---------|
| 1 | T6 quorumbench 双轨对照阻塞 | P2 | L-29-1 挂账转晨审，待裁决 | batch31 |
| 2 | batch28 NP 场景未转 schema 格式 | P3 | 可按需补转，非阻塞 | batch31+ |
| 3 | gen_report.py 未集成到闭案流程 | P1 | batch31 任务一集成 | batch31 |
| 4 | meter_diff.py 首次运行为手动模式 | P1 | batch31 任务二建立自动化 | batch31 |
| 5 | evidence schema judge 校验未实现 | P1 | batch31 任务四实现 judge 校验 | batch31 |

---

## 4. 闭案预审结论

### 可闭案项
- T1~T5 全量完成，无缺口
- T4/T5 复合场景已按 schema 格式留证
- 回归门 10 线全绿（REG-10 入线）

### 待晨审裁决项
1. **L-29-1**: quorumbench 真平台不可达——是否接受本地双轨模拟作为持续替代？
2. **T6 标记**: 战役III T1~T5 完成、T6 挂账——是否标记战役III 闭案（T6 转入持续跟踪）？
3. **缺口 3~5**: batch31 继承清单——gen_report 集成/meter_diff 自动化/schema judge 校验

### 建议闭案状态
- 战役III T1~T5: **闭案**
- 战役III T6: **挂账转持续跟踪**（L-29-1）
- batch30 产出: 治理基建四件套（gen_report.py/meter_diff.py/state_snapshot.py/evidence_schema.yaml）+ 战役III 闭案预审材料