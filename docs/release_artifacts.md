# Release Artifacts Manifest — v0.5.0

> Date: 2026-09-15

## Source Code

| Artifact | Path | Description |
|----------|------|-------------|
| Source tree | repository root | Full RaftKV source code |
| Git bundle | `v2.4-post-batch36.bundle` | Complete git history (includes internal v1.x–v2.4) |
| Go module | `go.mod` | Module: `raftkv`, Go 1.24.0+ |

## Documentation (OSS Six-Pack)

| Artifact | Path | Status |
|----------|------|--------|
| README | `README.md` | Project overview, TPS benchmark, quick start, API reference |
| LICENSE | `LICENSE` | Apache-2.0 full text |
| CHANGELOG | `CHANGELOG.md` | Version strategy + v0.5.0 changes + performance data |
| CONTRIBUTING | `CONTRIBUTING.md` | Development setup + contribution workflow + architecture |
| Quickstart | `docs/Quickstart.md` | Step-by-step guide (single node, 3-node, 5-node Docker) |
| API Reference | `docs/API_REFERENCE.md` | HTTP API + gRPC service documentation |

## Community Infrastructure

| Artifact | Path | Description |
|----------|------|-------------|
| Bug report template | `.github/ISSUE_TEMPLATE/bug_report.md` | GitHub issue template |
| Feature request template | `.github/ISSUE_TEMPLATE/feature_request.md` | GitHub issue template |
| PR template | `.github/PULL_REQUEST_TEMPLATE.md` | Pull request checklist |
| CI workflow | `.github/workflows/ci.yml` | Build + vet + test + regression gate check |

## Examples

| Artifact | Path | Description |
|----------|------|-------------|
| KV client | `examples/kv_client/main.go` | Go client demonstrating HTTP API operations |
| Docker Compose | `examples/docker-compose-quickstart.yml` | 5-node quickstart deployment |
| Examples README | `examples/README.md` | Example documentation |

## Benchmark Reports

| Artifact | Path | Description |
|----------|------|-------------|
| TPS benchmark | `docs/open_source_readiness.md` §4 | Three-tier TPS comparison (OSS-EXPR-01 compliant) |
| Scan report | `docs/chain3_scan_report.md` | Sensitive information scan results |
| Regression gate | `tests/contracts/regression.yaml` | 10-line regression gate config |
| Audit library | `docs/governance/AUDIT.md` | 26 audit cases |

## Verification

| Check | Command | Expected |
|-------|---------|----------|
| Build | `go build ./...` | PASS |
| Vet | `go vet ./...` | PASS |
| Unit tests | `go test . -skip TestRealGRPCConnectivity -count=1` | PASS |
| Example build | `go build ./examples/kv_client/` | PASS |
| Scan: no `235备份` | `git grep "235备份"` | 0 hits |
| Scan: no `<UID>` | `git grep "<UID>"` | 0 hits |
| Scan: no `DAIJIN235` | `git grep "DAIJIN235"` | 0 hits (excluding evidence/) |
| Scan: no `daijin235_012345` | `git grep "daijin235_012345"` | 0 hits |