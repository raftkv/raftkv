# D3 环境基线 + 冒烟测试报告

生成时间: 2026-09-07 19:34
批次: D3-batch0 (2节点基线) → D3-batch0R (5节点基线修正)
状态: PASS

---

## 0. 批次演进说明

| 批次 | 时间 | 节点数 | 状态 | 说明 |
|---|---|---|---|---|
| D3-batch0 | 2026-09-07 18:45 | 2 | PASS(缺陷) | 环境基线+2节点冒烟，compose仅2节点(裁剪自原始5节点设计) |
| D3-batch0R | 2026-09-07 19:34 | 5 | PASS | 取证根目录compose→确认ci-knife含/health/live→创建5节点compose→冒烟2遍通过 |

D3-batch0发现的缺陷: tests/deploy/docker-compose.yml仅定义2节点，原始设计(docker-compose.yml根目录)为5节点。
D3-batch0R修正: 创建tests/deploy/docker-compose-5node.yml，基于D2部署模式(ci-knife镜像+SM4_KEY+license挂载)扩展至5节点，冒烟2遍11/11 PASS。

---

## A. 环境基线指纹

### A1. 版本
| 项 | 值 |
|---|---|
| docker Client | 29.7.2 |
| docker Server | 29.7.2 |
| docker compose | v5.4.0 |
| 存储驱动 | overlayfs |

### A2. 系统资源
| 项 | 值 |
|---|---|
| CPU逻辑核数 | 18 |
| 宿主机总内存 | 31.52 GB |
| Docker分配内存 | 16512516096 bytes (≈15.38 GB) |
| D盘剩余 | 530.7 GB |
| D盘已用 | 100.96 GB |

### A3. 镜像清单（82镜像含中间层，顶层31个，总1.9GB）
| 镜像 | 大小 |
|---|---|
| raftkv:latest-knife | 41.2MB (核心运行镜像) |
| raftkv:latest-rollback | 41.2MB |
| grpc-health-probe:ci | 40.2MB |
| raftkv:latest-health | 41.2MB |
| raftkv:latest-m1/m2 | 41.1MB |
| raftkv:latest-dev1~dev5b | 各41.1MB |
| raftkv:latest/fix8/fixc/fixd/fixf/fixg | 各41.1MB |
| raftkv:latest-audit | 41.1MB |
| raftkv-v25:amd64test/test | 41.1/40.8MB |
| raftkv-redteam:v22s-auth | 40.9MB |
| raftkv-gateway:v24test/v22s | 41.1/56.8MB |
| raftkv-state-protection-research:v2.4-research | 31.4MB |
| alpine/git:latest | 144MB |
| golang:1.25-alpine | 329MB |
| golang:1.24-alpine | 395MB |
| alpine:latest / 3.21 / 3.18 | 13/25.1/11.5MB |
| nginx:alpine | 49.3MB (D3-batch0R拉取，用于frontend) |

总计: 顶层镜像 ~2.0GB；docker info Images计数=83（含中间层，新增nginx:alpine）

### A4. compose 服务清单（5节点，D3-batch0R）
| 服务 | 镜像 | gRPC端口(容器内) | HTTP端口(容器内) | 健康检查 | License挂载 |
|---|---|---|---|---|---|
| node-1 | raftkv:latest-knife | 9500 | 9000 | /health/live | node-1.key |
| node-2 | raftkv:latest-knife | 9500 | 9000 | /health/live | node-2.key |
| node-3 | raftkv:latest-knife | 9500 | 9000 | /health/live | node-3.key |
| node-4 | raftkv:latest-knife | 9500 | 9000 | /health/live | node-4.key |
| node-5 | raftkv:latest-knife | 9500 | 9000 | /health/live | node-5.key |

compose文件: tests/deploy/docker-compose-5node.yml
项目名: deploy5 (docker compose -p deploy5)
网络: deploy5_raft-net (bridge)
WAL卷: deploy5_wal-node-{1..5}
SM4_KEY: 从tests/deploy/.sm4_key复用 (837e01cd...)
FP_ANCHOR: tcx4-v25-test
LICENSE_FAIL_MODE: closed (fail-closed授权防线)

---

## B. 取证三问（D3-batch0R）

### B1. ci-knife镜像内是否实现/health/live端点?
**结论: PASS**
- 源码main.go:199注册路由: `httpMux.HandleFunc("/health/live", ...)`
- 镜像内二进制`/app/gateway`的strings分析: `strings /app/gateway | grep -c 'health/live'` = 1
- 运行时验证: 5/5节点curl /health/live → HTTP 200

### B2. 原始compose中frontend引用的镜像(nginx:alpine)本地是否存在?
**结论: 初始不存在，已拉取**
- 取证时: `docker image inspect nginx:alpine` → No such image
- 处置: `docker pull nginx:alpine` → 成功 (49.3MB)
- 当前: nginx:alpine已在本地，frontend服务可启动

### B3. docker-compose-3dc.yml与docker-compose.yml差异摘要
| 维度 | docker-compose.yml(根) | docker-compose-3dc.yml |
|---|---|---|
| 节点数 | 5 (node-1~5) | 9 (dc1-n1~3, dc2-n1~3, dc3-n1~3) |
| 拓扑 | 单子网daijin-net | 3子网(dc1-net/dc2-net/dc3-net) + inter-dc桥 |
| 子网IP | 自动分配 | 10.1.0.0/24, 10.2.0.0/24, 10.3.0.0/24 |
| frontend | 有(nginx:alpine, 8096:80) | 无 |
| build | build: . (从Dockerfile构建) | 无build,仅image引用 |
| LICENSE_FAIL_MODE | closed | 未设置 |
| gRPC端口 | 9500-9502, 9604-9605 | 9700-9702, 9710-9712, 9720-9722 |
| HTTP端口 | 9001-9003, 9104-9105 | 9201-9203, 9211-9213, 9221-9223 |
| 容灾能力 | 单机房 | 跨机房(停任一DC的3节点,剩6>majority(5)→集群可用) |

---

## C. 5节点冒烟测试（2遍重复，D3-batch0R）

### C1. 第1遍 (smoke-run1, 19:32:55)
| 项 | 结果 |
|---|---|
| docker compose up | PASS, 5容器启动 |
| Leader选举 | PASS, raft-node-2 after 3s |
| /health/live | 5/5 PASS (HTTP 200) |
| 容器Running | 5/5 PASS |
| 写入10条 | 10/10 success (index 2-11) |
| stats | commit=11 applied=11 (applied==commit!=0) |
| entry[index=2] | found=true |
| Follower同步 | 4/4 PASS (commit=11) |
| down -v | PASS |
| 无残留 | PASS (容器/卷/网络均无) |
| **汇总** | **PASS=11 FAIL=0** |

### C2. 第2遍 (smoke-run2, 19:33:51)
| 项 | 结果 |
|---|---|
| docker compose up | PASS, 5容器启动 |
| Leader选举 | PASS, raft-node-4 after 4s |
| /health/live | 5/5 PASS (HTTP 200) |
| 容器Running | 5/5 PASS |
| 写入10条 | 10/10 success (index 2-11) |
| stats | commit=11 applied=11 (applied==commit!=0) |
| entry[index=2] | found=true |
| Follower同步 | 4/4 PASS (commit=11) |
| down -v | PASS |
| 无残留 | PASS (容器/卷/网络均无) |
| **汇总** | **PASS=11 FAIL=0** |

### C3. 两次差异比对
功能结果差异: **0**（5/5 Up、5/5 health/live 200、10条写入、4/4同步、无残留 → 两次完全一致）

运行期动态值差异（非功能差异，属自然波动）:
- Leader节点: node-2 vs node-4（Raft选举自然选主差异）
- Leader选举耗时: 3s vs 4s（选举时间自然波动）
- entry[index=2].command: base64编码内容不同（两次写入数据不同，属正常）

---

## D. 结论

**D3-batch0R PASS**

- 环境基线指纹已完整采集（版本/资源/镜像/5节点服务清单）
- 取证三问全部解决:
  - 1a: ci-knife镜像内/health/live端点已确认（源码+strings+运行时三重验证）
  - 1b: nginx:alpine已拉取至本地
  - 1c: 3dc与根compose差异已完整记录
- 5节点compose(docker-compose-5node.yml)创建并验证:
  - 一键up → 5/5容器Running → 5/5 /health/live 200 → Leader当选 → 10条写入 → 4/4 Follower同步 → 一键down -v → 无残留
  - 2遍重复，功能结果差异为0，11/11 PASS
- 核心镜像 raftkv:latest-knife (41.2MB) 在本地，无拉取依赖
- D3-batch0的2节点裁剪缺陷已修正为5节点原始设计

---

## 证据文件
- tests/evidence/d3-batch0R/smoke-run1.log (第1遍完整日志)
- tests/evidence/d3-batch0R/smoke-run2.log (第2遍完整日志)
- tests/deploy/docker-compose-5node.yml (5节点compose定义)

---

## E. D3-batch1运行环境说明（D3-audit-reply闭环4补强）

### E1. batch1运行拓扑

D3-batch1 Part1的15项测试(F01-F09,F10-F14,F16)运行在**2节点裁剪版compose**(tests/deploy/docker-compose.yml)上，而非5节点原始设计。

证据: F01 `peers=1`, F04 `leader=node-1/follower=node-2`, F14仅检查2节点SERVING。

**结论: Part1结果基于错误拓扑(2节点而非5节点)，全部作废待5节点重测。**

历史compose已归档: `tests/evidence/d3-batch1/docker-compose-2node-historical.yml`（标注"历史形态，仅作拓扑证明"）。

### E2. compose差异引用

5节点compose(docker-compose-5node.yml)相对根compose的8处适配性改写详见: `docs/D3-AUDIT-REPLY.md` 闭环1。

### E3. kunpeng-evidence归档状态

鲲鹏920实机证据已归档至 `docs/kunpeng-evidence/`（D3-clarify批次, commit da4b044）。

采集形态说明:
- 采集环境: 3台鲲鹏920裸金属服务器(DG-S920X20, 172.38.3.147/173/174)
- 采集窗口: 2026-08-08 ~ 2026-08-25
- **3台裸金属混部5节点**: V2.2S集群运行日志(05_V2.2S集群运行日志/)包含node-1~node-5的tail2000日志，说明在3台物理机上混部了5个节点进程
- 文件数: 28数据文件 + 1索引文档 = 29文件, 1.73MB
- MD5核对: 29/29逐字节保真
- 关联定位: 作为5节点拓扑的实机运行前例，供D3-batch2压测基线参照

---

## F. D3-batch1终态（2026-09-07 收官）

### F1. 用例PASS拓扑

| 批次 | 用例范围 | 节点数 | PASS/总计 | 证据目录 |
|------|---------|--------|----------|---------|
| batch1-r2 Part1 | F01-F16 | 5 | 16/16 | tests/evidence/d3-batch1-r2/ |
| batch1 Part2 | E01, E01b, E02, E03 | 5 | 3/3 (+E01b追加) | tests/evidence/d3-batch1-part2/ |
| **合计** | **F01-F16 + E01-E03** | **5** | **19/19** | — |

遗留缺陷数: **0**（F10/F11已修复并selfverify裁决为真实防线）

### F2. tag链

| tag | commit | 说明 |
|-----|--------|------|
| d3-batch0-pass | 277f30e | 环境基线+2节点冒烟 |
| d3-batch0R-pass | c64bf46 | 5节点基线修正+冒烟2x |
| d3-clarify-pass | da4b044 | 存疑点补证+鲲鹏证据归档 |
| d3-audit-reply-pass | a4e51db | 盲审问题闭环4项 |
| d3-sec-rotate-pass | 2490c22 | SM4密钥轮换+deploy脚本5节点化 |
| d3-batch1-r2-pass | e103ffa | F10-F13/F15/F16 5节点测试 |
| d3-batch1-r2-fix-pass | 13c23f6 | F10/F11修复+回归PASS |
| d3-selfverify-pass | fb34301 | F10/F11修复真伪判别(三实验) |
| d3-part2-pass | 928b386 | E01/E01b/E02/E03 四用例PASS |

### F3. 实测基线参数表

| 参数 | 实测值 | 场景 | batch2用途 |
|------|--------|------|-----------|
| 分区恢复日志追平耗时 | ~~35s~~ → **2s** | E02: R-01修复前35s, 修复后2s | 已根治, P-Ⅱ压测基线 |
| 分区恢复耗时分解 | 重连2s + 选举恢复0s | R-01修复后E02回归 | gRPC退避MaxDelay=5s生效 |
| Follower崩溃恢复追平耗时 | <10s | E01: node-1 kill→restart, commit追平 | 故障恢复基线 |
| Leader崩溃重新选举耗时 | <5s | E01b: R-01修复后回归PASS | 选举性能基线 |
| 并发写入确定性增量 | 11/20 | E03: 10带idem_token去重为1 + 10不带各1 | 幂等去重基线 |

> **R-01修复说明**: gRPC重连退避MaxDelay 120s→5s（保留指数退避曲线1s×1.6→上限5s）。
> 修复后E02收敛35s→2s，病灶仅gRPC退避一处，node-2竞选为Raft预期行为。

### F4. 挂账清单

| 项 | 状态 | 说明 |
|----|------|------|
| 外部盲审#4 | **挂账** | 待Credits恢复后补审 sec-rotate + selfverify + part2 全链 |
| batch2计划 | **起草中** | D3-BATCH2-PLAN.md，提交后停机等批复 |
