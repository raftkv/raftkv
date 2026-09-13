# 项目总结 — V2.4 Performance Sandbox

> 生成: batch32（CHAIN-1 链尾）
> 日期: 2026-09-14
> 范围: batch14~batch31 全量文档整理

## 1. 项目目标

V2.4 Raft 共识引擎性能验证与治理基建：
- 吞吐: 796.7→9020 TPS (11.3x)
- 延迟: P99 50ms (c=128)
- 故障注入: 战役III 6 场景全 PASS + 复合场景 3/3 PASS
- 治理基建: gen_report + meter_diff + judge_schema + closure_integration

## 2. 批次历程

| 批次 | 主题 | verdict | 关键产物 |
|------|------|---------|----------|
| batch14~16 | 限流+mTLS | PASS | 限流器+双层准入 |
| batch17~20 | 延迟分解+fsync合并 | PASS | P99 200ms→50ms |
| batch21~23 | 故障注入+pre-vote | PASS | 23 场景全 PASS |
| batch24~26 | pre-vote优化+性能闭案 | PASS | E1 1.8233s, E4b 3.4696s |
| batch27~28 | 战役III T1~T3+NP验收 | PASS | 6 NP 场景全 PASS |
| batch29 | 推导机器催缴+复合场景 | PASS | N=3 CV=0.0% |
| batch30-S | 治理基建四件套 | PASS | gen_report+meter_diff+schema+SNAPSHOT |
| batch30-R | 深度补强 | PASS | 5缺口清零+AUDIT-019/020 |
| batch31 | L-29-1清偿+集成确认 | PASS | LEDGER 0待清 |

## 3. 性能数字冻结

| 指标 | 值 | 来源 |
|------|-----|------|
| TPS | 9020 (c=128) | batch26 闭案 |
| P99 | 50ms | batch26 闭案 |
| E1 | 1.8233s PASS | batch25 复验 |
| E4b | median=3.4696s PASS | batch28 N=5 扩测 |
| 成功率 | 100% | batch26 闭案 |
| 内存 | 17-29 MiB/节点 | batch9 终审 |

## 4. 治理文档状态

| 文档 | 条目数 | 状态 |
|------|--------|------|
| AUDIT.md | 20 判例 | 完备 |
| MISBEHAVIOR.md | 11 条 | 完备（MB-011 含深度归因） |
| LEDGER.md | 0 待清 | 全部清偿 |
| regression.yaml | 10 线 | 全绿 |

## 5. 战役III 闭案状态

| 阶段 | 状态 |
|------|------|
| T1~T5 | 全完成 |
| T6 quorumbench | L-29-1 已清偿（接受本地双轨模拟） |
| 闭案申请 | 准予闭案 |

## 6. CHAIN-1 产物清单

| 产物 | 路径 |
|------|------|
| gen_report.py | tests/contracts/gen_report.py |
| meter_diff.py | tests/contracts/meter_diff.py |
| state_snapshot.py | tests/contracts/state_snapshot.py |
| evidence_schema.yaml | tests/contracts/evidence_schema.yaml |
| judge_schema.py | tests/contracts/judge_schema.py |
| closure_integration.py | tests/contracts/closure_integration.py |
| variant_self_check_genreport.py | tests/contracts/variant_self_check_genreport.py |
| convert_legacy_evidence.py | tests/contracts/convert_legacy_evidence.py |
| STATE_SNAPSHOT.md | tests/evidence/d3-batch30/STATE_SNAPSHOT.md |
| campaign3_closure_review.md | docs/campaign3_closure_review.md |
| ROADMAP_review.md | docs/ROADMAP_review.md |