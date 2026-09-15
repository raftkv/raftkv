# STATE_SNAPSHOT — RaftKV · CHAIN-4

> 日期: 2026-09-15
> 链: CHAIN-4 发布收官与公网亮相
> 前链: CHAIN-3 开源交付链 (COMPLETE, v0.5.0)
> 状态: **COMPLETE — 闭案，进入观察期**
> 版本: v0.5.0
> License: Apache-2.0
> HEAD: e6815fc (含 vet 修复 ceb1343)
> Tag: v0.5.0 → f1f3842
> 仓库: https://github.com/raftkv/raftkv (Public, CI 全绿)
> 验收: 机器裁决 9/13 + 4 项误伤补证 → 修正终裁 13/13 PASS

## CHAIN-4 链状态

| 任务 | 状态 | 产出 |
|------|------|------|
| 1. 备份三级闭环 | ✅ COMPLETE | bundle+MD5+核验单 |
| 2. 陌生环境发布验证 | ⏳ PENDING | |
| 3. 公网亮相准备 | ⏳ PENDING | |
| 4. 红线门 | ⏳ PENDING | |

## CHAIN-3 链完成状态 (前链)

| 任务 | 状态 | 提交 |
|------|------|------|
| 1. 敏感信息扫舱+脱敏 | ✅ COMPLETE | 63ec79f |
| 2. OSS 文档六件套 | ✅ COMPLETE | 5962848 |
| 3. 社区基建 | ✅ COMPLETE | 5962848 |
| 4. 示例应用 | ✅ COMPLETE | 5962848 |
| 5. 发布工程 | ✅ COMPLETE | 5962848 (tag v0.5.0) |
| 6. 红线门验收 | ✅ COMPLETE | 449bc9d |

## 备份三级闭环 (CHAIN-4 任务1)

| 凭证 | 值 |
|------|-----|
| Bundle 文件 | v0.5.0-release.bundle |
| Bundle 大小 | 74MB |
| MD5 | 2e01b2641482acf0f78838283f6d3ade |
| 物理位1 | <PROJECT_ROOT>/v0.5.0-release.bundle |
| 物理位2 | <HOME>/backup_chain4/v0.5.0-release.bundle (原目录，保留) |
| 物理位3 | <HOME>/raftkv_backup/v0.5.0-release.bundle (CHAIN-4 收尾新增，路径脱敏) |
| MD5 一致性 | PASS (三位一致) |
| Bundle 验证 | PASS (106 refs, 含 v0.5.0 tag → f1f3842) |
| 核验单 | docs/chain4_backup_verify.md |

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
- Git tag: v0.5.0 (annotated, on commit f1f3842)
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

- v0.6.0: module path 变更 `raftkv` → `github.com/raftkv/raftkv`
- v0.6.0: CI 回归门在线化（需 verdict.json 证据自动生成）
- 阶段二: Gitee 镜像（信创曝光前再建）

## 观察期

- 起始: 2026-09-15
- 周期: 1-2 周
- 监控: issue/安全类反馈
- 纪律: 零宣传（无 topic/无公告/无社区外链）
- v0.7.0: protoc 可用后重命名 proto 包名（消除 RL-07 例外）
- v1.0.0: 首个生产部署 + API 连续一季度零破坏变更后升级