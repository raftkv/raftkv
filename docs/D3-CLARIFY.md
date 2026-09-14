# D3-batch0R 存疑点补证

生成时间: 2026-09-07 19:50
批次: D3-clarify (batch0R盲审补证)
状态: PASS

---

## 存疑点1: 闸口报告称"D3-batch1 Part1已PASS"的实物证据

### 1a. 物理目录清单

`tests/evidence/d3-batch1/` 目录物理存在，内容如下：

```
tests/evidence/d3-batch1/
├── F01/result.txt  (162 bytes)
├── F02/result.txt  (233 bytes)
├── F03/result.txt  (140 bytes)
├── F04/result.txt  (254 bytes)
├── F05/result.txt  (158 bytes)
├── F06/result.txt  (216 bytes)
├── F07/result.txt  (213 bytes)
├── F08/result.txt  (84 bytes)
├── F09/result.txt  (195 bytes)
├── F10/result.txt  (87 bytes)
├── F11/result.txt  (92 bytes)
├── F12/result.txt  (97 bytes)
├── F13/result.txt  (109 bytes)
├── F14/result.txt  (109 bytes)
└── F16/result.txt  (103 bytes)
```

共15个子目录，每个含1个result.txt。F15缺失（WAL测试未执行）。

### 1b. result.txt内容摘要（逐条PASS确认）

| 用例 | 结果 | 原文摘要 |
|---|---|---|
| F01 Leader选举 | PASS | `Leader节点: node-1 after 3s` |
| F02 数据写入 | PASS | `5/5 success` |
| F03 数据读回 | PASS | `数据一致` |
| F04 数据同步 | PASS | `commit=6 一致` |
| F05 /raft/stats字段 | PASS | `全部字段存在` |
| F06 /raft/status JSON | PASS | `9字段完全匹配` |
| F07 幂等串行去重 | PASS | `增量1, duplicate=true` |
| F08 幂等并发去重 | PASS | `并发10次增量1` |
| F09 幂等LRU淘汰 | PASS | `idem功能可用, size=9` |
| F10 fail-closed SM4缺失 | PASS | `exit非0` |
| F11 fail-closed SM4非法 | PASS | `exit非0` |
| F12 fail-closed GRPC_PORT非法 | PASS | `拒绝启动` |
| F13 fail-closed WAL_FLUSH非法 | PASS | `拒绝启动` |
| F14 gRPC健康检查 | PASS | `两节点SERVING` |
| F16 权限校验(无license) | PASS | `Fail-Closed拒绝` |

### 1c. 版本控制状态

- `git ls-files tests/evidence/d3-batch1/` → **空**（未入git）
- `git check-ignore tests/evidence/d3-batch1/F01/result.txt` → 确认被`.gitignore`的`tests/evidence/`规则排除
- `git log --all --oneline -- tests/evidence/d3-batch1/` → **空**（无commit触及）
- `git tag -l '*batch1*'` → **空**（无batch1相关tag）

### 1d. 结论

**物理证据存在且内容全部PASS**，但**未入版本控制**（被.gitignore排除，无commit无tag）。

闸口报告称"D3-batch1 Part1已PASS"的说法：
- **有实物支撑**：tests/evidence/d3-batch1/目录下15个result.txt物理存在，内容均为PASS
- **缺版本控制证据**：无对应commit或tag，无法从git历史追溯
- **陈述准确性**：物理层面为真（文件存在且PASS），版本控制层面无记录

---

## 存疑点2: applied==commit为"11==11"，写入为10条，11的构成

### 2a. smoke日志原文佐证

**Leader当选时**（smoke-run1.log 第55-56行）：
```
[19:33:02] [PASS] Leader=raft-node-2 after 3s
[19:33:02]   stats: id=node-2 state=Leader term=1 leader=node-2 commit=1 applied=1 logs=1 peers=4 voted=node-2
```

此时 `commit=1 applied=1 logs=1` — 集群刚完成Leader选举，已有**1条初始日志条目**（Raft协议中Leader当选后写入的no-op/初始化条目，用于确立term权威性）。

**写入10条后**（smoke-run1.log 第86-88行）：
```
[19:33:10] --- 6. 读回stats ---
[19:33:11]   leader stats: id=node-2 state=Leader term=1 leader=node-2 commit=11 applied=11 logs=11 peers=4 voted=node-2
[19:33:11]   commit=11 applied=11
```

此时 `commit=11 applied=11 logs=11`。

**写入过程**（smoke-run1.log 第74-84行）：
```
write[1]: {"index":2,"success":true}
write[2]: {"index":3,"success":true}
...
write[10]: {"index":11,"success":true}
```

写入10条，index从2到11。

### 2b. 11的构成

```
11 = 1(初始条目, index=1) + 10(写入条目, index=2~11)
```

- **index=1**: Raft Leader当选后自动写入的初始条目（no-op），commit=1是集群就绪的基线
- **index=2~11**: 冒烟测试写入的10条数据（`d3b0r-run1-entry-1` ~ `d3b0r-run1-entry-10`）

这是Raft协议的标准行为，非异常。run2日志同样佐证：Leader当选时`commit=1`，写入10条后`commit=11`。

---

## 存疑点3: "3dc差异已完整记录"的文件位置

### 3a. 记录位置

文件：`docs/D3-BASELINE.md`
章节：**B3. docker-compose-3dc.yml与docker-compose.yml差异摘要**

### 3b. 原文摘要

```
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
```

该记录在D3-batch0R commit `c64bf46`中，文件`docs/D3-BASELINE.md`第B3节。

---

## 总结

| 存疑点 | 结论 |
|---|---|
| 1. D3-batch1 Part1 PASS证据 | 物理存在(15个result.txt全PASS)，但未入git(无commit/tag) |
| 2. 11==11构成 | 1(初始no-op) + 10(写入) = 11，Raft标准行为，日志原文佐证 |
| 3. 3dc差异记录位置 | docs/D3-BASELINE.md B3节，commit c64bf46 |