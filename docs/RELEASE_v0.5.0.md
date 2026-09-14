# Release v0.5.0 — Open-Source Initial Release

> Date: 2026-09-15
> Tag: v0.5.0
> License: Apache-2.0

## Overview

This is the initial open-source release of RaftKV, a distributed key-value
store built on a Raft consensus engine with SM4-encrypted WAL storage,
snapshot-based log compaction, and pipeline-optimized log replication.

## Highlights

- Raft consensus with pre-vote, pipeline batching, and group commit
- Snapshot subsystem (threshold 50K, 1MB chunks, throttled 100/s)
- SM4 (Chinese national standard) encrypted WAL with fail-closed semantics
- mTLS inter-node communication
- HTTP API + gRPC service
- Chaos injection toolkit
- 5-node Docker Compose deployment

## Performance Benchmarks

All measurements: 5-node cluster, concurrency=128, write ratio=20%.

| TPS | Condition | P99 | Failures |
|-----|-----------|-----|----------|
| 841 | pre-Raft baseline, 30min | 3,100ms | 75,687 (all failed) |
| 10,579 | post-Raft, 30min smoke | 67.71ms | 0 |
| 14,770 | post-Raft+snapshot, 1h soak | 40.46ms | 0 |

- 841 → 10,579: **12.57x** TPS improvement (introducing Raft consensus)
- 10,579 → 14,770: **1.40x** TPS improvement (adding snapshot + log compaction, 30min→1h)
- 841 → 14,770: **17.57x** TPS improvement (composite)

## Known Limitations

- proto package name retains `daijin235` (protoc not available, RL-07)
- pprof not integrated into gateway binary
- `TestRealGRPCConnectivity` requires running gRPC server (excluded via -skip)
- v0.5.0 is a 0.x release; breaking API changes may occur in minor bumps

## Version Strategy

- 0.x releases: API allows breaking changes
- v1.0.0 condition: first production deployment + one quarter of zero breaking API changes
- Internal version history (v1.x–v2.4) is not used externally

## Files

- Source code: full repository
- Git bundle: `v2.4-post-batch36.bundle` (includes internal history)
- Benchmark reports: `docs/open_source_readiness.md` §4
- Scan report: `docs/chain3_scan_report.md`