#!/usr/bin/env python3
"""state_snapshot.py — 状态快照生成

生成 ≤200 行快照：当前批次/未决挂账/下批待办/预算水位/回归门版本。
batch31 链节起，启动只读快照+按需点查，RESUME/LEDGER 降为详细档案。

用法:
  python state_snapshot.py --output tests/evidence/d3-batch30/STATE_SNAPSHOT.md
"""

import argparse
import time
from pathlib import Path


def generate_snapshot():
    lines = []
    lines.append("# STATE_SNAPSHOT — 状态快照")
    lines.append("")
    lines.append(f"> 生成时间: {time.strftime('%Y-%m-%dT%H:%M:%S')}")
    lines.append("> 维护规则: ≤200 行，启动只读快照+按需点查")
    lines.append("")
    lines.append("---")
    lines.append("")

    lines.append("## 1. 当前批次")
    lines.append("")
    lines.append("| 字段 | 值 |")
    lines.append("|------|-----|")
    lines.append("| 批次 | batch30 |")
    lines.append("| CHAIN | CHAIN-1（链首） |")
    lines.append("| 链节 | batch30→batch31→batch32 |")
    lines.append("| 规格 | 治理基建批 |")
    lines.append("| 状态 | 进行中 |")
    lines.append("| 上批 verdict | PASS (batch29, commit 789eb4c, tag v2.4-post-batch29) |")
    lines.append("")

    lines.append("## 2. 未决挂账")
    lines.append("")
    lines.append("| debt_id | source | target | status | description |")
    lines.append("|---------|--------|--------|--------|-------------|")
    lines.append("| L-29-1 | batch29 | batch31 | 待清 | quorumbench 真平台不可达，双轨复核挂账转晨审，本地双轨模拟=regression_gate 10线全绿 |")
    lines.append("")

    lines.append("## 3. 下批待办（batch31）")
    lines.append("")
    lines.append("| 任务 | 内容 |")
    lines.append("|------|------|")
    lines.append("| 任务一 | gen_report.py 集成到闭案流程 |")
    lines.append("| 任务二 | meter_diff.py 每批闭案必跑 |")
    lines.append("| 任务三 | L-29-1 LEDGER 清偿（接受本地双轨模拟作为替代） |")
    lines.append("| 任务四 | evidence schema 强制校验（新场景按 schema 留证） |")
    lines.append("| 任务五 | 战役III 闭案材料晨审预审 |")
    lines.append("")

    lines.append("## 4. 预算水位")
    lines.append("")
    lines.append("| 字段 | 值 |")
    lines.append("|------|-----|")
    lines.append("| 单批上限 | 18000K |")
    lines.append("| 链总上限 | 45000K |")
    lines.append("| 硬下限 | 8000K |")
    lines.append("| batch30 已消耗 | ~8K（任务零+一+二+三） |")
    lines.append("| 链已消耗 | ~8K |")
    lines.append("| 链剩余 | ~44992K |")
    lines.append("| 80% 预警线 | 14400K |")
    lines.append("| 越线? | 否 |")
    lines.append("")

    lines.append("## 5. 回归门")
    lines.append("")
    lines.append("| 字段 | 值 |")
    lines.append("|------|-----|")
    lines.append("| 版本 | 3.2 |")
    lines.append("| 线数 | 10 |")
    lines.append("| 状态 | 全绿 |")
    lines.append("| 线族 | REG-1~10 |")
    lines.append("| 最新入线 | REG-10 partition_recovery_timeliness (batch29) |")
    lines.append("")

    lines.append("## 6. 红线")
    lines.append("")
    lines.append("| 字段 | 值 |")
    lines.append("|------|-----|")
    lines.append("| 红线数 | 13 + 首屏由脚本生成检查位 |")
    lines.append("| 状态 | 13/13 PASS (batch29 闭案) |")
    lines.append("| 新增 | batch30: 首屏由 gen_report.py 生成（申报即产物） |")
    lines.append("")

    lines.append("## 7. 治理文档")
    lines.append("")
    lines.append("| 文档 | 路径 | 条目数 |")
    lines.append("|------|------|--------|")
    lines.append("| AUDIT.md | docs/governance/AUDIT.md | 18 判例 |")
    lines.append("| MISBEHAVIOR.md | docs/governance/MISBEHAVIOR.md | 11 条 |")
    lines.append("| LEDGER.md | LEDGER.md | 1 待清 (L-29-1) |")
    lines.append("| regression.yaml | tests/contracts/regression.yaml | 10 线 |")
    lines.append("")

    lines.append("## 8. 战役III 进度")
    lines.append("")
    lines.append("| 阶段 | 状态 | 批次 |")
    lines.append("|------|------|------|")
    lines.append("| T1 SDD 设计 | 完成 | batch27 |")
    lines.append("| T2 网络分区执行器 | 完成 | batch28 |")
    lines.append("| T3 NP 验收判定 | 完成 | batch28 |")
    lines.append("| T4 复合场景首探 | 完成 | batch29 |")
    lines.append("| T5 复合场景验收 | 完成 | batch29 |")
    lines.append("| T6 quorumbench 双轨 | 挂账 L-29-1 | batch29 |")
    lines.append("| 收尾核验 | 进行中 | batch30 |")
    lines.append("")

    lines.append("## 9. 性能数字冻结")
    lines.append("")
    lines.append("| 指标 | 值 |")
    lines.append("|------|-----|")
    lines.append("| TPS | 796.7→9020 (11.3x) |")
    lines.append("| P99 | 50ms (c=128) |")
    lines.append("| E1 | 1.8233s PASS |")
    lines.append("| E4b | median=3.4696s PASS |")
    lines.append("")

    lines.append("---")
    lines.append(f"> 行数: {len(lines)} / 200")

    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description="state_snapshot.py 状态快照生成")
    parser.add_argument("--output", required=True, help="输出路径")
    args = parser.parse_args()

    content = generate_snapshot()
    with open(args.output, "w", encoding="utf-8") as f:
        f.write(content)

    line_count = content.count("\n") + 1
    print(f"state_snapshot.py — 状态快照生成")
    print(f"output: {args.output}")
    print(f"lines: {line_count} / 200")
    if line_count > 200:
        print(f"WARNING: 超过 200 行限制!")


if __name__ == "__main__":
    main()