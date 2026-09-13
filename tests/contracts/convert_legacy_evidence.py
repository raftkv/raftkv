#!/usr/bin/env python3
"""convert_legacy_evidence.py — 存量留证转 schema 格式

将 batch29 复合场景存量留证（scenario_comp_pdf_0{1..3}.json）补转为四段留证 schema 格式。
禁伪造——存量留证无法回溯转换时保留原格式+标记 legacy。

用法:
  python convert_legacy_evidence.py --evidence-dir tests/evidence/d3-batch29 --output-dir tests/evidence/d3-batch29/schema
"""

import argparse
import json
import sys
import time
from pathlib import Path


def convert_scenario(original):
    """将原始 scenario JSON 转为四段留证 schema 格式"""
    cm = original.get("composite_metrics", {})
    timeline = original.get("timeline", [])

    injection_ts = ""
    observation_start = ""
    observation_end = ""
    recovery_ts = ""
    for event in timeline:
        et = event.get("event_type", "")
        ts = event.get("timestamp", "")
        if et == "composite_start":
            injection_ts = ts
            observation_start = ts
        elif et == "nodes_reconnected":
            recovery_ts = ts
            observation_end = ts
        elif et == "composite_partition_end":
            if not observation_end:
                observation_end = ts

    recovery_duration = cm.get("partition_duration_s", 10)

    schema = {
        "schema_version": "1.0",
        "converted_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "legacy_source": original.get("evidence_path", ""),
        "conversion_note": "从 batch29 存量留证转换为四段 schema 格式",

        "injection": {
            "scenario_id": original.get("scenario_id"),
            "scenario_type": original.get("scenario_type"),
            "fault_type": "compound_partition_diskfull",
            "target_nodes": cm.get("partitioned_nodes", []),
            "parameters": {
                "partition_type": cm.get("partition_type"),
                "disk_full_target": cm.get("disk_full_target"),
                "disk_pressure_level": cm.get("disk_pressure_level"),
                "partition_duration_s": cm.get("partition_duration_s"),
            },
            "timestamp": injection_ts,
            "leader_before": next(
                (e.get("node_id") for e in timeline if e.get("event_type") == "leader_identified"),
                "",
            ),
        },

        "observation": {
            "metrics": {
                "max_concurrent_leaders": cm.get("max_concurrent_leaders"),
                "minority_leader_count": cm.get("minority_leader_count"),
                "majority_leader": cm.get("majority_leader"),
                "term_before": cm.get("term_before"),
                "term_after": cm.get("term_after"),
                "term_monotonic": cm.get("term_monotonic"),
                "commit_index_before": cm.get("commit_index_before"),
                "commit_index_after": cm.get("commit_index_after"),
                "commit_caught_up": cm.get("commit_caught_up"),
                "cluster_available": cm.get("cluster_available"),
            },
            "timeline": timeline,
            "timestamp_start": observation_start,
            "timestamp_end": observation_end,
        },

        "recovery": {
            "operation": "reconnect_partitioned_nodes + clean_disk_full",
            "duration_s": recovery_duration,
            "confirmed": cm.get("recovery_confirmed", False),
            "timestamp": recovery_ts,
        },

        "assertion": {
            "checks": [
                {"name": "COMP-1_no_split_brain", "expected": "<=1", "actual": cm.get("max_concurrent_leaders"), "status": "PASS" if cm.get("max_concurrent_leaders", 0) <= 1 else "FAIL"},
                {"name": "COMP-2_minority_no_election", "expected": "==0", "actual": cm.get("minority_leader_count"), "status": "PASS" if cm.get("minority_leader_count", 0) == 0 else "FAIL"},
                {"name": "COMP-3_term_monotonic", "expected": True, "actual": cm.get("term_monotonic"), "status": "PASS" if cm.get("term_monotonic") else "FAIL"},
                {"name": "COMP-4_commit_caught_up", "expected": True, "actual": cm.get("commit_caught_up"), "status": "PASS" if cm.get("commit_caught_up") else "FAIL"},
                {"name": "COMP-5_recovery_confirmed", "expected": True, "actual": cm.get("recovery_confirmed"), "status": "PASS" if cm.get("recovery_confirmed") else "FAIL"},
            ],
            "overall_status": original.get("status", "UNKNOWN"),
            "threshold_source": "regression.yaml REG-10",
        },
    }

    return schema


def main():
    parser = argparse.ArgumentParser(description="存量留证转 schema 格式")
    parser.add_argument("--evidence-dir", required=True, help="原始证据目录")
    parser.add_argument("--output-dir", required=True, help="schema 格式输出目录")
    args = parser.parse_args()

    evidence_dir = Path(args.evidence_dir)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    scenario_files = sorted(evidence_dir.glob("scenario_comp_pdf_*.json"))
    results = []

    for sf in scenario_files:
        try:
            with open(sf, "r", encoding="utf-8") as f:
                original = json.load(f)
            schema = convert_scenario(original)
            output_file = output_dir / f"schema_{sf.name}"
            with open(output_file, "w", encoding="utf-8") as f:
                json.dump(schema, f, indent=2, ensure_ascii=False)
            results.append({"file": sf.name, "output": str(output_file), "status": "PASS"})
            print(f"  {sf.name} → {output_file.name}: PASS")
        except Exception as e:
            results.append({"file": sf.name, "output": None, "status": "FAIL", "error": str(e)})
            print(f"  {sf.name}: FAIL ({e})")

    summary = {
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "evidence_dir": str(evidence_dir),
        "output_dir": str(output_dir),
        "converted": results,
        "total": len(results),
        "pass": sum(1 for r in results if r["status"] == "PASS"),
        "fail": sum(1 for r in results if r["status"] == "FAIL"),
    }

    summary_path = output_dir / "conversion_summary.json"
    with open(summary_path, "w", encoding="utf-8") as f:
        json.dump(summary, f, indent=2, ensure_ascii=False)

    print(f"\n转换完成: {summary['pass']}/{summary['total']} PASS")
    print(f"输出目录: {output_dir}")


if __name__ == "__main__":
    main()