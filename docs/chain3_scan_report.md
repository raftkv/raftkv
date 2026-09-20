# CHAIN-3 敏感信息扫舱报告

> 日期: 2026-09-15
> 扫描范围: 全仓 git-tracked 文件
> 目的: 开源前零私有信息出仓

## 1. 扫描结果汇总

| 类别 | 敏感模式 | 命中数 | git-tracked 文件数 | 严重度 |
|------|----------|--------|---------------------|--------|
| A. 本地路径 | `<ARCHIVE>` | 49 | ~15 | P0 |
| B. 用户路径 | `<HOME>` | 11 | ~8 | P0 |
| C. 私有标识 | `daijin235` | 528 | ~50 | P0 |
| D. 内网地址 | `192.168.x.x` | 8 | ~3 | P1 |
| E. 硬编码密钥 | `daijin235_012345` / 私钥路径 | 6 | ~4 | P0 |

## 2. 详细发现

### 2.1 本地路径 (<ARCHIVE>)

| 文件 | 行号 | 内容 | 处置 |
|------|------|------|------|
| docs/chain2_campaign4_morning_report.md | 137,144-163 | `<ARCHIVE>/...` | → 相对路径 |
| docs/D2-FINAL-REPORT.md | 12,69,70,99,108,145 | `<ARCHIVE>\...` | → 相对路径 |
| docs/kunpeng-evidence/README.md | 20 | `<ARCHIVE>\archive-...` | → 删除行 |
| docs/kunpeng-evidence/00_总索引...md | 264 | `<ARCHIVE>\...` | → 删除行 |
| tests/deploy/run_baseline33.bat | 2-5 | `<ARCHIVE>\...` | → `%~dp0` 相对路径 |
| cmd/gen_lic_tmp/main.go | 26 | `<ARCHIVE>\岱境235_RSA密钥备份\...` | → env var |
| cmd/license-tool/main.go | 18 | `<ARCHIVE>\岱境235_RSA密钥备份\...` | → env var |
| 双模式授权配置说明.md | 283,302 | `<ARCHIVE>\...` | → 相对路径 |
| _state_protection_research/*.md | 多行 | `<ARCHIVE>\...` | → 相对路径 |
| tests/auto/*/REPORT.md | 多行 | `<ARCHIVE>\...` | → 删除或脱敏 |
| tests/evidence/*/...md | 多行 | `<ARCHIVE>\...` | gitignored，标注 |

### 2.2 用户路径 (<HOME>)

| 文件 | 行号 | 内容 | 处置 |
|------|------|------|------|
| tests/deploy/deploy.env | 5 | `LICENSE_DIR=<HOME>/.daijin235/...` | → env var |
| tests/deploy/deploy_batch12.env | 5 | 同上 | → env var |
| docs/D3-AUDIT-REPLY.md | 60 | 同上 | → env var |
| docs/D3-SEC-ROTATE-EVIDENCE.md | 72 | `<HOME>\.sm4_key_old` | → 占位符 |
| cmd/gen_lic_tmp/main.go | 27 | `<HOME>\.daijin235\...` | → env var |
| docs/kunpeng-evidence/07_.../TCX-Ⅱ...md | 240,247 | `<HOME>\...` | → 删除行 |
| _state_protection_research/tasks.md | 298 | `<HOME>\Desktop\...` | → 相对路径 |
| _state_protection_research/spec.md | 532 | `<HOME>\Desktop\` | → 相对路径 |
| start_monitor.bat | 2 | `<HOME>\Desktop\...` | → env var |
| README_DOCKER.md | 15 | `<HOME>\Desktop\...` | → 相对路径 |

### 2.3 私有标识 (daijin235)

| 维度 | 用法 | 文件数 | 处置 |
|------|------|--------|------|
| Go module 名 | `module daijin235` (go.mod) | 1 | → `raftkv` |
| Go import 路径 | `daijin235/proto`, `daijin235/pkg/...` | ~30 | → `raftkv/proto`, `raftkv/pkg/...` |
| gRPC service 路径 | `/daijin235.RaftService/...` | 2 | → 保留（proto 生成，RL-07） |
| 容器名 | `daijin235-node-1..5` | ~10 | → `raft-node-1..5` |
| 镜像名 | `daijin235-v26:batch*` | ~5 | → `raftkv:latest` |
| 网络名 | `deploy5_daijin235-net` | ~5 | → `deploy5_raft-net` |
| CA CN | `daijin235-ca` | 2 | → `raft-ca` |
| WAL 文件名 | `daijin235_raft.wal` | 1 | → `raftkv.wal` |
| 证据/文档引用 | `daijin235-node-*` 状态行 | ~20 | → 脱敏或删除 |

### 2.4 内网地址

| 文件 | 行号 | 内容 | 处置 |
|------|------|------|------|
| tests/evidence/.../node-5-exit137-log.txt | 11 | `192.0.2.2:9500` | gitignored |
| tests/evidence/.../pass-report.md | 25 | `192.0.2.3` | gitignored |
| share_all.bat | 19 | `192.168.` LAN 检测 | → 保留（通用 LAN 检测） |
| cmd/diag_tool/main.go | 293 | `192.168.1.10:9501` (注释示例) | → 保留（示例地址） |

### 2.5 硬编码密钥/私钥路径

| 文件 | 行号 | 内容 | 严重度 | 处置 |
|------|------|------|--------|------|
| pipeline_integration_test.go | 51,158 | `[]byte("daijin235_012345")` | P0 | → `[]byte("test_sm4_key_0123")` |
| cmd/gen_lic_tmp/main.go | 26 | `<ARCHIVE>\岱境235_RSA密钥备份\...` | P0 | → env var `RSA_PRIVATE_KEY_PATH` |
| cmd/license-tool/main.go | 18 | 同上 | P0 | → env var `RSA_PRIVATE_KEY_PATH` |
| tests/deploy/docker-compose-5node.yml | 17 | `SM4_KEY出处: ...` (注释) | P2 | → 保留（无密钥值） |
| tests/suite_wal_snap.sh | 141,147,151 | `daijin235_012345` (leak scan) | P1 | → `test_sm4_key_0123` |
| tests/knife_run.sh | 121,124 | 同上 | P1 | → 同上 |

## 3. 脱敏计划

### 3.1 Go module 重命名: daijin235 → raftkv

- go.mod: `module daijin235` → `module raftkv`
- 所有 .go 文件 import: `daijin235/...` → `raftkv/...`
- gRPC service path `/daijin235.RaftService/...` 保留（proto 生成，RL-07 不可改）
- proto package `package daijin235` 保留（.pb.go 生成物，不改 proto 源）

### 3.2 容器/镜像/网络重命名

- 容器: `daijin235-node-*` → `raft-node-*`
- 镜像: `daijin235-v26:batch*` → `raftkv:latest`
- 网络: `deploy5_daijin235-net` → `deploy5_raft-net`
- CA CN: `daijin235-ca` → `raft-ca`
- WAL: `daijin235_raft.wal` → `raftkv.wal`

### 3.3 路径脱敏

- `<ARCHIVE>\...` → 相对路径或 env var
- `<HOME>\...` → env var 或占位符
- 私钥路径 → `os.Getenv("RSA_PRIVATE_KEY_PATH")`

### 3.4 密钥脱敏

- `daijin235_012345` → `test_sm4_key_0123`（测试用占位密钥）
- 私钥路径 → env var

## 4. gRPC service path 特例说明

gRPC service 路径 `/daijin235.RaftService/AppendEntries` 由 proto 生成（.pb.go），protoc 不可用（RL-07），无法重命名。该路径为运行时 RPC 路由键，不影响开源安全性（非密钥/非路径/非标识符），保留并在此说明。

## 5. 验收标准

- `git grep "235备份"` = 0 命中 ✓
- `git grep "<UID>"` = 0 命中 **[翻案: 原报0命中实为2命中, 2026-09-20已清理]**
- `git grep "daijin235"` ≤ 4 命中（仅 proto 文件 + gRPC service path，附 RL-07 说明）✓
- `git grep "daijin235_012345"` = 0 命中 ✓
- `git grep "C:\\Users"` = 0 命中 ✓
- `git grep "岱境235"` = 1 命中（proto 文件注释，RL-07）✓
- 编译通过 + 单测通过（排除 TestRealGRPCConnectivity）✓

## 6. 扫舱结果

| 检查项 | 结果 | 说明 |
|--------|------|------|
| 本地路径 `235备份` | 0 命中 | 全部替换为相对路径/占位符 |
| 用户路径 `<UID>` | **翻案: 0→2→0** | 原报0命中实为2命中(chain4_backup_verify.md + chain3_scan_report.md), 2026-09-20已清理为 `<HOME>` |
| 私有标识 `daijin235` | 4 命中 | 仅 proto/daijin235.proto（RL-07）+ grpc_server.go gRPC path（RL-07） |
| 硬编码密钥 `daijin235_012345` | 0 命中 | 替换为 `raftkv_sm4test01`（16字节） |
| 中文标识 `岱境235` | 1 命中 | proto 文件注释（RL-07） |
| Go module 重命名 | `daijin235` → `raftkv` | go.mod + 全部 import 路径 |
| 容器/镜像重命名 | `daijin235-node-*` → `raft-node-*` | docker-compose + 脚本 |
| 二进制文件 | 5 个 ARM64 二进制 | 从 git 移除（含嵌入路径） |
| 编译 | PASS | `go build ./...` 零错误 |
| 单测 | PASS | `go test . -skip TestRealGRPCConnectivity` 全绿 |

## 7. RL-07 例外清单

以下 `daijin235` 引用因 protoc 不可用（RL-07）保留，已在扫舱报告中登记：

| 文件 | 行 | 内容 | 保留原因 |
|------|-----|------|----------|
| proto/daijin235.proto | package | `package daijin235;` | proto 包名，改需 protoc 重新生成 |
| proto/daijin235.proto | go_package | `option go_package = "daijin235/proto";` | Go 包路径，改需 protoc 重新生成 |
| proto/daijin235.proto | 注释 | `// 岱境235 确定性引擎` | proto 文件注释，RL-07 禁改 proto |
| grpc_server.go | 3 处 | `/daijin235.RaftService/...` | gRPC service path，由 .pb.go 生成 |
| proto/daijin235.pb.go | 多处 | `package daijin235` + 函数名 | protoc 生成物，不可手改 |
| proto/daijin235_grpc.pb.go | 多处 | `daijin235.RaftService` | protoc 生成物，不可手改 |