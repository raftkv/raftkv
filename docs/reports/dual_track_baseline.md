# 双轨对比基线 — Dual Track Baseline

> 版本: v2.4-batch26
> 日期: 2026-09-13
> 状态: 本平台基线已产出，quorumbench 侧待外部执行

---

## 1. 本平台（本仓库 HEAD = v2.4-post-batch25）

| 验收项 | 判定 | 实测值 | 阈值 | 证据 |
|--------|------|--------|------|------|
| F1 选举收敛 | PASS | 2.6853s | ≤5.0s | d3-batch25/verdict.json |
| F2 拒载率 | PASS | 20.00% | ≤30% | d3-batch25/verdict.json |
| F3 存活率 | PASS | 100.00% | =100% | d3-batch25/verdict.json |
| F4 无脑裂 | PASS | 1 | ≤1 | d3-batch25/verdict.json |
| F5 可回放 | PASS | true | true | d3-batch25/verdict.json |
| E1 稳态选举 | PASS | 1.8233s | ≤2.0s | d3-batch25/verdict.json |
| E4 级联选举 | FAIL | 3.2849s | ≤2.0s | d3-batch25/verdict.json |
| PV1 pre-vote | PASS | 85 | >0 | d3-batch25/verdict.json |
| PV2 term膨胀 | PASS | 5 | ≤5 | d3-batch25/verdict.json |
| LAN 历史数字 | PASS | 存在且非空 | — | d3-batch25/verdict.json |

**本平台 verdict: FAIL**（E4 未达标，待 D-24-1 晨批裁决）

## 2. quorumbench 平台（独立判定）

| 验收项 | 判定 | 实测值 | 备注 |
|--------|------|--------|------|
| — | — | — | **待外部执行** |

> quorumbench 平台不在本环境可用，需在外部 CI/CD 中跑标准验收套件。
> 本文件为双轨判定的首份基线模板，quorumbench 侧填充后完成双轨对照。

## 3. 双轨对比

| 验收项 | 本平台 | quorumbench | 一致? |
|--------|--------|-------------|-------|
| — | — | — | 待 quorumbench 侧填充 |

## 4. 后续操作

1. 在 quorumbench 平台对本仓库 HEAD (v2.4-post-batch25) 跑标准验收
2. 将结果填入本文件 §2/§3
3. 比对双轨判定一致性，不一致项入 MISBEHAVIOR 调查