# D3 盲审问题闭环回复

生成时间: 2026-09-07 20:10
批次: D3-audit-reply
盲审评级: B+ → 本批闭环4项

---

## 闭环0: batch1证据入库

### 处置
- `.gitignore`修改: `tests/evidence/` → `tests/evidence/*` + `!tests/evidence/d3-batch1/`（放行d3-batch1目录）
- 15个result.txt全部补入git
- 归档历史compose: `tests/evidence/d3-batch1/docker-compose-2node-historical.yml`（2节点裁剪版副本，标注"历史形态，仅作拓扑证明"）

### 入库清单
```
tests/evidence/d3-batch1/
├── docker-compose-2node-historical.yml  (历史compose副本，含标注注释)
├── F01/result.txt  ...  F16/result.txt  (15个测试结果)
```

---

## 闭环1: compose偏离说明

### 偏离分析: tests/deploy/docker-compose-5node.yml vs 根docker-compose.yml

| # | 维度 | 根compose | 5node compose | 偏离性质 | 说明 |
|---|---|---|---|---|---|
| 1 | 镜像来源 | `build: .` + `image: raftkv-gateway:latest` | `image: ${IMAGE_NAME}` (ci-knife) | **适配性改写** | ci-knife已验证含/health/live(strings确认)；build: .需Go编译环境且产物不含SM4_KEY/license逻辑 |
| 2 | 端口策略 | 每节点不同GRPC(9500-9502,9604-9605) + host映射(9001-9003,9104-9105) | 全节点相同GRPC(9500)+HTTP(9000)，无host暴露 | **适配性改写** | D2部署模式依赖容器网络隔离，各容器独立命名空间允许端口复用；无host暴露与D2一致(docker exec验证) |
| 3 | SM4_KEY | 无 | `SM4_KEY=${SM4_KEY}` | **适配性改写** | ci-knife为fail-closed模式，需SM4_KEY做数据加密；根compose的build产物为旧版本不需SM4_KEY |
| 4 | license挂载 | 无 | `${LICENSE_DIR}/node-N.key:/app/license.key:ro` | **适配性改写** | ci-knife需license.key做授权验证(fail-closed) |
| 5 | frontend | 有(nginx:alpine, 8096:80) | 无 | **适配性改写** | 冒烟测试聚焦Raft集群核心功能，frontend非必要 |
| 6 | PEERS格式 | 容器名+不同端口 `node-2=raft-node-2:9501` | 容器名+相同端口 `node-2=node-2:9500` | **适配性改写** | D2部署模式使用相同内部端口+容器名解析，各容器独立命名空间 |
| 7 | WAL卷 | 无 | `wal-node-N:/app/wal-data` | **适配性改写** | ci-knife需WAL持久化目录 |
| 8 | HTTP_BIND | 无 | `HTTP_BIND=0.0.0.0` | **适配性改写** | ci-knife需显式绑定0.0.0.0接受容器内通信 |
| 9 | healthcheck | 有(/health/live) | 有(/health/live) | **无偏离** | 一致 |
| 10 | LICENSE_FAIL_MODE | closed | closed | **无偏离** | 一致(格式差异:映射vs列表，语义等价) |

### 偏离性质总结

全部8处偏离均为**适配性改写**，无无意漂移。每处偏离的必要性：从`build: .`(旧版本镜像)迁移到`ci-knife`(fail-closed镜像)时，必须补充SM4_KEY/license/HTTP_BIND/WAL卷等运行时依赖。端口策略和PEERS格式遵循D2部署模式(已验证2x9/9可重复)。

### SM4_KEY与license值出处

| 配置项 | 值 | 出处 | 生成方式 |
|---|---|---|---|
| SM4_KEY | \<已轮换，见.sm4_key\> | tests/deploy/.sm4_key (gitignored) | `openssl rand -hex 16` (deploy_up.sh首次生成) |

### SM4_KEY密钥轮换整改说明

**泄露发现**：盲审#2发现本文件L50原明文记录SM4_KEY值（commit a4e51db），评级P1。

**处置方式**：密钥轮换制。生成新密钥替换.sm4_key内容，本文件明文改为`<已轮换，见.sm4_key>`。旧密钥自本commit起失效。

**历史处理**：旧密钥值已在git历史a4e51db中，视为已作废。按R2红线（禁止改写历史），不执行history清除。旧密钥作废验证见tests/evidence/d3-sec-rotate/。
| FP_ANCHOR | tcx4-v25-test | tests/deploy/deploy.env.example | 硬编码(指纹锚标识) |
| LICENSE_DIR | <HOME>/.raftkv/tcx4_test/licenses_v25 | tests/deploy/deploy.env | 本地license目录 |
| license.key | node-{1..5}.key (各817字节) | LICENSE_DIR下 | RSA-2048商业授权文件(预生成) |

### yml注释块

已在`tests/deploy/docker-compose-5node.yml`头部添加差异清单+改写原因注释块（见下方闭环1c执行）。

---

## 闭环2: Part1拓扑声明

### 声明

**D3-batch1 Part1的15项测试(F01-F09,F10-F14,F16)全部运行在2节点裁剪版compose(tests/deploy/docker-compose.yml)上，而非5节点原始设计。**

### 证据

| 用例 | 原文线索 | 拓扑推断 |
|---|---|---|
| F01 | `peers=1` | 集群仅2节点(自己+1peer) |
| F04 | `leader=node-1, follower=node-2` | 仅2节点参与 |
| F14 | `raft-node-1: SERVING, raft-node-2: SERVING` | 仅检查2节点 |
| 全部 | 无node-3/4/5出现 | 2节点拓扑 |

### 结论

**Part1结果基于错误拓扑(2节点而非5节点)，全部作废待5节点重测。**

历史compose已归档为`tests/evidence/d3-batch1/docker-compose-2node-historical.yml`，标注"历史形态，仅作拓扑证明"。

---

## 闭环3: F09/F15缺陷

### F09: 参数与标题不一致

| 项 | 值 |
|---|---|
| 标题 | `F09 幂等LRU淘汰 (CAPACITY=5)` |
| 实际运行 | `idem_stats: {"capacity":10000,"size":9}` |
| 期望(suite_idem.sh:81) | `IDEM_TOKEN_CAPACITY=5` |
| 根因 | 测试脚本未设置`IDEM_TOKEN_CAPACITY=5`环境变量，用了默认值10000 |
| 正确意图 | 验证LRU淘汰机制：CAPACITY=5时写7个token→size应=5(淘汰最早的2个) |
| 处置 | **标注DEFECT**：F09结果PASS但未验证LRU淘汰行为(capacity=10000时size=9不会触发淘汰)。需在5节点重测时设置`IDEM_TOKEN_CAPACITY=5`重跑 |

### F15: 缺失

| 项 | 值 |
|---|---|
| 编号 | F15 |
| 预期内容 | WAL持久化测试（D3-CLARIFY.md声明"WAL测试未执行"） |
| 目录状态 | `tests/evidence/d3-batch1/F15/` 不存在 |
| 原因 | **未执行**：Part1执行脚本(c-cleanup-d3b1-part1.ps1)已清理，无法确认具体跳过原因。从D3-CLARIFY.md记录推断为WAL测试未安排在Part1中 |
| 处置 | **标注BLOCKED**：F15 WAL持久化测试未执行，需在5节点重测时补跑 |

---

## 闭环4: D3-BASELINE.md补强

已在D3-BASELINE.md中补充：
1. batch1运行环境说明（2节点裁剪版compose，Part1结果作废）
2. compose差异引用（指向D3-AUDIT-REPLY.md闭环1）
3. kunpeng-evidence归档状态（含"3台裸金属混部5节点"采集形态说明）

---

## 总结

| 闭环项 | 状态 | 处置 |
|---|---|---|
| 0. batch1证据入库 | PASS | .gitignore放行+15文件入库+compose副本归档 |
| 1. compose偏离说明 | PASS | 8处偏离全为适配性改写，无无意漂移 |
| 2. Part1拓扑声明 | **作废** | 2节点拓扑，Part1全部作废待5节点重测 |
| 3. F09/F15缺陷 | DEFECT+BLOCKED | F09参数未生效(DEFECT)，F15未执行(BLOCKED) |
| 4. D3-BASELINE.md补强 | PASS | 已补充batch1环境+compose差异+kunpeng归档 |