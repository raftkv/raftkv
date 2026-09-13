#!/usr/bin/env python3
"""回归门检查脚本 — 先跑回归门再跑本批门

用法: python regression_gate.py --regression tests/contracts/regression.yaml --verdict tests/evidence/d3-batch25/verdict.json
"""

import argparse
import json
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML not installed", file=sys.stderr)
    sys.exit(2)


def check_regression_gate(regression_path, verdict_path):
    with open(regression_path, "r", encoding="utf-8") as f:
        reg = yaml.safe_load(f)
    with open(verdict_path, "r", encoding="utf-8") as f:
        verdict = json.load(f)

    criteria = reg.get("acceptance_criteria", {})
    results = {}

    mapping = {
        "REG-1": ("F1", "value", 5.0, "<="),
        "REG-2": ("E1", "value", 2.0, "<="),
        "REG-3": ("F4", "max_concurrent_leaders", 1, "<="),
        "REG-4": ("F2", "during_max_reject_rate", 30.0, "<="),
        "REG-5": ("F3", "min_survival_rate", 100.0, "=="),
    }

    for reg_id, (verdict_key, field, threshold, op) in mapping.items():
        v = verdict.get(verdict_key, {})
        actual = v.get(field)
        if actual is None:
            results[reg_id] = {"status": "INSUFFICIENT_EVIDENCE", "detail": f"{verdict_key}.{field} missing"}
            continue
        if op == "<=":
            passed = actual <= threshold
        elif op == "==":
            passed = actual == threshold
        else:
            passed = actual >= threshold
        results[reg_id] = {
            "status": "PASS" if passed else "FAIL",
            "detail": f"{verdict_key}.{field}={actual} {op} {threshold}: {passed}",
            "threshold": threshold,
            "actual": actual,
        }

    any_fail = any(r["status"] == "FAIL" for r in results.values())
    overall = "FAIL" if any_fail else "PASS"
    return results, overall


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--regression", required=True)
    parser.add_argument("--verdict", required=True)
    args = parser.parse_args()

    results, overall = check_regression_gate(args.regression, args.verdict)

    print(f"Regression Gate: {overall}")
    for k, v in results.items():
        print(f"  {k}: {v['status']}  {v['detail']}")

    if overall == "FAIL":
        print("回归门 FAIL → 冻结，不跑本批门")
        sys.exit(1)
    else:
        print("回归门 PASS → 继续跑本批门")
        sys.exit(0)


if __name__ == "__main__":
    main()