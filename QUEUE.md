# QUEUE.md — CHAIN-1 续链指令

> 创建: batch30 闭案
> 链: batch30→batch31→batch32
> 协议: 每批完成→自检门→读 QUEUE.md→下一批→自检门→...→三批合并晨报→停机等晨审
> 链间门: 任一批 FAIL 或触红线 → 断链停机

---

## batch31-S: 治理基建集成 + L-29-1 清偿

### 预算
- 单批上限: 18000K
- 链累计上限: 45000K（batch30 ~29K + batch31 ≤ 18000K + batch32 ≤ 18000K）
- 硬下限: 8000K

### 任务零（0.5h）
- 读 STATE_SNAPSHOT.md 快照启动
- LEDGER 检查: L-29-1 待清（本批任务三清偿）
- 首屏: gen_report.py 生成（如产物齐全）+ 手写机制级归因
- meter_diff.py 闭案必跑（首次自动化尝试）

### 任务一（1h）: gen_report.py 集成到闭案流程
- 闭案时自动调用 gen_report.py 生成首屏
- 首屏由脚本生成，禁止手写（除机制级归因）
- 变异自检每批必跑

### 任务二（0.5h）: meter_diff.py 每批闭案必跑
- 尝试自动化读取 IDE 计量
- 若不可读，继续手动双数字申报模式
- 结果入 verdict.json

### 任务三（0.5h）: L-29-1 LEDGER 清偿
- 晨审判决: 是否接受本地双轨模拟作为 quorumbench 替代？
- 若接受: LEDGER L-29-1 标记已清偿，cleared_at=batch31
- 若拒绝: 继续挂账，记录晨审意见

### 任务四（1h）: evidence schema 强制校验
- 新场景强制按 schema 留证
- judge 校验四段完整性（injection/observation/recovery/assertion）
- recovery.duration_s 必填

### 任务五（1h）: 战役III 闭案材料晨审预审
- 整理 docs/campaign3_closure_review.md
- 补齐缺口清单中 P1 项
- 产出战役III 闭案正式申请

### 闭案
- 红线 13+1/13+1 PASS
- 回归门 10/10 全绿
- meter_diff 入 verdict
- STATE_SNAPSHOT 更新
- commit + tag v2.4-post-batch31
- 读 QUEUE.md → batch32

---

## batch32-S: 链尾收束 + 三批合并晨报

### 预算
- 单批上限: 18000K
- 链总上限: 45000K
- 硬下限: 8000K

### 任务零（0.5h）
- 读 STATE_SNAPSHOT.md 快照启动
- LEDGER 检查
- gen_report.py 生成首屏
- meter_diff.py 闭案必跑

### 任务一（1h）: 文档整理（ROADMAP 方向五）
- 整理 batch14~batch31 全量文档
- 产出 docs/project_summary.md
- 更新 ROADMAP.md

### 任务二（1h）: 三批合并晨报
- 合并 batch30+batch31+batch32 报告
- 产出 docs/chain1_morning_report.md
- 含: 治理基建四件套状态+战役III 闭案状态+L-29-1 清偿状态+预算水位

### 任务三（0.5h）: 链终自检
- 红线 13+1/13+1 PASS
- 回归门 10/10 全绿
- meter_diff 三批对账
- 变异自检全 PASS
- STATE_SNAPSHOT 最终版

### 任务四（0.5h）: 开源准备预探（ROADMAP 方向六）
- 评估开源准备度
- 产出缺口清单
- 建议后续批次

### 闭案
- commit + tag v2.4-post-batch32
- tag v2.4-chain1-complete
- 三批合并晨报产出
- 停机等晨审

---

## 链间门

| 门 | 条件 | 动作 |
|----|------|------|
| 自检门 | 红线 13+1/13+1 PASS + 回归门 10/10 | FAIL → 断链停机 |
| 预算门 | 链累计 ≤ 45000K | 超 → 断链停机 |
| 硬下限 | 单批 ≥ 8000K | < → 禁开下一批 |
| L-29-1 | batch31 任务三清偿 | 未清 → 继续挂账 |
| gen_report | 每批闭案生成首屏 | 未生成 → 断链停机 |
| meter_diff | 每批闭案必跑 | 未跑 → 断链停机 |
---

## CHAIN-4: 发布收官与公网亮相 — ✅ COMPLETE

> 开链: 2026-09-15
> 闭链: 2026-09-15
> 前链: CHAIN-3 (COMPLETE, v0.5.0)
> HEAD: ceb1343 (含 vet 修复)
> 仓库: https://github.com/raftkv/raftkv (Public, CI 全绿)

### 任务1: 备份三级闭环 (铁律11核验) — ✅ COMPLETE
- 核验 v0.5.0 tag → f1f3842 完整性
- git bundle 产出 v0.5.0-release.bundle (74MB, MD5: 2e01b264...)
- 三物理位: <PROJECT_ROOT> + <HOME>/backup_chain4/ + <HOME>/raftkv_backup/
- 核验单: docs/chain4_backup_verify.md

### 任务2: 陌生环境发布验证 (OSS-EXPR-01 纪律) — ✅ COMPLETE
- 逐字走 Quickstart 完成，2 个文档缺陷已修复
- 走查留证: docs/chain4_walkthrough_evidence.md

### 任务3: 公网亮相准备 — ✅ COMPLETE
- 四道裁决全批复 (GitHub / org:raftkv / Public+零宣传 / OSS-EXPR-01)

### 任务4: 红线门 + 公网推送 — ✅ COMPLETE
- 泄露终扫零残留 + 回归门 10 线全绿
- 公网推送完成: https://github.com/raftkv/raftkv
- CI 全绿: Build + Vet + Unit Tests + Regression Gate
- 零宣传纪律生效 (无 topic/无公告/无引流徽章)
- 1-2 周观察期开始

### 后续待办 (v0.6.0)
- module path 变更: `raftkv` → `github.com/raftkv/raftkv`
- Gitee 镜像 (阶段二信创曝光前再建)