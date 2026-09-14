# STATE_SNAPSHOT — 岱境235 · 战役V · CHAIN-3

> 日期: 2026-09-15
> 链: CHAIN-3 开源交付链
> 状态: **COMPLETE**
> 版本: v0.5.0
> License: Apache-2.0

## 链完成状态

| 任务 | 状态 | 提交 |
|------|------|------|
| 1. 敏感信息扫舱+脱敏 | ✅ COMPLETE | 63ec79f |
| 2. OSS 文档六件套 | ✅ COMPLETE | 5962848 |
| 3. 社区基建 | ✅ COMPLETE | 5962848 |
| 4. 示例应用 | ✅ COMPLETE | 5962848 |
| 5. 发布工程 | ✅ COMPLETE | 5962848 (tag v0.5.0) |
| 6. 红线门验收 | ✅ COMPLETE | 449bc9d |

## 关键裁决

- LICENSE: Apache-2.0
- 版本号: v0.5.0 (0.x 允许破坏性变更; v1.0.0 条件 = 首个生产部署 + API 连续一季度零破坏)
- Go module: raftkv
- 内部版本历史 (v1.x–v2.4) 不对外使用

## 红线门结果

| 检查项 | 结果 |
|--------|------|
| 扫舱零残留 | PASS |
| 六件套齐备 | PASS |
| 示例可编译 | PASS |
| 编译+单测 | PASS |

## 产出清单

### OSS 文档六件套
- README.md, LICENSE, CHANGELOG.md, CONTRIBUTING.md
- docs/Quickstart.md, docs/API_REFERENCE.md

### 社区基建
- .github/ISSUE_TEMPLATE/bug_report.md
- .github/ISSUE_TEMPLATE/feature_request.md
- .github/PULL_REQUEST_TEMPLATE.md
- .github/workflows/ci.yml

### 示例应用
- examples/kv_client/main.go
- examples/docker-compose-quickstart.yml
- examples/README.md

### 发布工程
- Git tag: v0.5.0 (annotated, on commit 449bc9d)
- docs/RELEASE_v0.5.0.md
- docs/release_artifacts.md

### 验收报告
- docs/chain3_scan_report.md (任务1)
- docs/chain3_redline_gate.md (任务6)

## 沿用规则

- 红线 13+4 条
- 回归门 10 线 (tests/contracts/regression.yaml)
- 判例库 26 条 (docs/governance/AUDIT.md)
- OSS-EXPR-01: TPS 宣传须标注对照基线条件

## RL-07 例外

proto 包名 `daijin235` 和 gRPC service path `/daijin235.RaftService/*` 保留，
因 protoc 不可用无法重新生成 .pb.go 文件。为 proto-level 标识符，非安全风险。

## 后续方向

- v0.6.0: CI 回归门在线化（需 verdict.json 证据自动生成）
- v0.7.0: protoc 可用后重命名 proto 包名（消除 RL-07 例外）
- v1.0.0: 首个生产部署 + API 连续一季度零破坏变更后升级