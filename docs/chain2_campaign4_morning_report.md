# CHAIN-2 战役IV 全链合并晨报

> 日期: 2026-09-15
> 链: batch33-S → 34-S → 35-S → 36-S
> 墙钟总预算: ≥6h | token 总预算: ≤50000K

## 1. 四批概览

| 批次 | 类型 | 墙钟 | 核心产物 | 判定 |
|------|------|------|----------|------|
| batch33-S | 规格设计 | ~1.5h | SDD spec/design/tasks + 四大子系统接口骨架 + 基线锚定 | PASS |
| batch34-S | 核心实现一 | ~1.5h | 选举 + 日志复制核心实现 + 14 单测 + 30min smoke | PASS |
| batch35-S | 核心实现二 | ~3h | 快照 + 分区恢复 + 14 单测 + 1h soak | PASS |
| batch36-S | 集成收尾 | ~1h | 全模块集成 + 回归门 + 文档 + AUDIT | PASS |

**全链墙钟**: ~7h ≥ 6h ✓

## 2. 红线门汇总

| 红线 | batch33 | batch34 | batch35 | batch36 |
|------|---------|---------|---------|---------|
| 长时验证 ≥30min | 30min 基线锚定 ✓ | 30min smoke ✓ | 1h soak ✓ | 继承 ✓ |
| 性能基线不劣化 | 锚定 TPS=841 | TPS=10579 ✓ | TPS=14770 ✓ | TPS≥9020 ✓ |
| 回归门 REG-1~10 | N/A | 全绿 ✓ | 全绿 ✓ | 全绿 ✓ |
| 无带病闯门 | ✓ | ✓ | ✓ | ✓ |

## 3. 长时验证详情

### batch33-S 基线锚定 (30min)
- TPS=841, P99=3.10s, 成功率=95.00% (pre-Raft 基线)

### batch34-S smoke (30min)
- TPS=10579, P99=67.71ms, 成功率=100.00%, 0 失败

### batch35-S soak (1h)
- TPS=14770, P99=40.46ms, 成功率=100.00%, 0 失败, 53 次快照
- 注: 之前失败系 3 个并发 loadgen 进程污染，非代码 bug (AUDIT-026)

## 4. 单元测试汇总

| 批次 | 测试数 | 全 PASS |
|------|--------|---------|
| batch34-S | 14 | ✓ |
| batch35-S | 14 | ✓ |
| 全模块 (T048) | 全部 | ✓ (排除 TestRealGRPCConnectivity 预存失败) |

## 5. AUDIT 追认

| 判例 | 描述 | 状态 |
|------|------|------|
| AUDIT-021 | 设计批长时验证以基线锚定形式满足 | 采信 |
| AUDIT-022 | CompactLogs 切片截断物理删除 | 采信 |
| AUDIT-023 | WAL 并发保护 snapMu RWMutex | 采信 |
| AUDIT-024 | 快照限流器 SnapshotThrottle | 采信 |
| AUDIT-025 | InstallSnapshot HTTP 分片传输 | 采信 |
| AUDIT-026 | soak 失败根因: 并发 loadgen 污染 | 采信 |

## 6. 变更文件清单

| 文件 | 变更 |
|------|------|
| raft.go | T030-T035 快照子系统 + Index 口径修复 |
| raft_pipeline.go | T034 executeSnapshot 前置校验 |
| raft_storage.go | WAL 并发保护 snapMu |
| main.go | HTTP 端点 + 限流器初始化 |
| raft_batch35_test.go | NEW: 14 单元测试 |
| raft_batch13_test.go | 适配切片截断 |
| raft_knife2_test.go | logStartIndex |
| raft_knife3_test.go | 空快照行为 |

proto 文件未修改 (RL-07 满足) ✓

## 7. 红线活性演习报告

> 本节记录 CHAIN-2 链内四次红线真实触发，证明红线门非摆设而是活的拦截机制。

### 演习 ①: AUDIT-021 规则冲突停机（batch33-S）

- **红线**: 规则冲突时停机对齐，禁止绕过
- **触发场景**: batch33-S 规格设计阶段，长时验证要求 ≥30min 与设计批时长约束冲突
- **触发动作**: 停机，不强行在设计批内塞入 30min 验证，改为以"基线锚定"形式满足
- **处置结果**: 30min 基线锚定 TPS=841 作为 pre-Raft 基线，冲突消解
- **证明**: 红线触发后停机对齐而非绕过 → 红线活性确认 ✓

### 演习 ②: 长时验证不过拦截闭案（batch35-S，两次）

- **红线**: 长时验证不过不闭案，发现真 bug 修复后重跑，禁止带病闯门
- **第一次触发**: 1h soak 报告 ~33% 失败率，红线拦截闭案
- **第一次处置**: 排查根因 → 发现 3 个并发 e04_loadtest.exe 进程污染 → 杀残留 → 重跑
- **第二次触发**: 重跑后仍出现失败（另一组残留进程）→ 红线再次拦截
- **第二次处置**: 全面清理 + 单进程干净重跑 → 1h soak 100% 通过（TPS=14770, P99=40ms）
- **证明**: 两次拦截 + 两次排查 + 最终干净通过 → 红线活性确认 ✓

### 演习 ③: 美化倾向审计警告（batch35-S，附追认）

- **红线**: 禁止"美化门失败"为"已知 trade-off"
- **触发场景**: batch35-S soak 失败后，存在将失败归因为"环境抖动"的美化倾向
- **触发动作**: 审计警告——要求查明真实根因，禁止用"环境抖动"定性
- **处置结果**: 查明真实根因为并发进程污染（非环境抖动），记入 AUDIT-026
- **追认**: 根因确认后，审计警告转为 AUDIT-026 判例，附进程卫生完整版（AUDIT-026a/b/c）
- **证明**: 美化倾向被拦截 → 真实根因被查明 → 红线活性确认 ✓

### 演习 ④: 水位门轮换 + 跨模型续链（batch34-S → batch35-S）

- **红线**: 水位门轮换——单模型 token/墙钟触水位时停机，跨模型续链不丢上下文
- **触发场景**: batch34-S 接近 token 水位，需轮换至新模型继续 batch35-S
- **触发动作**: 停机 → 保存链上下文（SDD spec/design/tasks + 已完成 T 编号 + 证据路径）→ 新模型加载续链
- **处置结果**: batch35-S 从 T030 无缝续接，上下文完整（14 单测 + 1h soak 全 PASS）
- **证明**: 跨模型续链后无上下文丢失、无任务遗漏 → 红线活性确认 ✓

### 红线活性演习总结

| 演习 | 红线 | 批次 | 触发次数 | 拦截结果 | 活性确认 |
|------|------|------|----------|----------|----------|
| ① | AUDIT-021 规则冲突停机 | batch33-S | 1 | 停机对齐 → 基线锚定 | ✓ |
| ② | 长时验证不过拦截闭案 | batch35-S | 2 | 两次拦截 → 干净重跑通过 | ✓ |
| ③ | 美化倾向审计警告 | batch35-S | 1 | 美化被拦截 → 真实根因查明 | ✓ |
| ④ | 水位门轮换+跨模型续链 | batch34→35 | 1 | 停机续链 → 上下文无丢失 | ✓ |

**结论**: 四次红线真实触发，四次拦截生效，零绕过。红线门活性确认。

## 8. 备份锚定报告

> 日期: 2026-09-15
> 目的: 确认 CHAIN-2 全链 bundle + tag 已推送远端，报告所有 bundle 第二物理位置

### 8.1 Tag 推送确认

| Tag | 本地 commit | V2.4_remote 推送 | 状态 |
|-----|-------------|------------------|------|
| v2.4-post-batch33 | c26bc6a | ✓ 已推送 | 锚定 |
| v2.4-post-batch34 | d927bdf | ✓ 已推送 | 锚定 |
| v2.4-post-batch35 | f9012a6 | ✓ 已推送 | 锚定 |
| v2.4-post-batch36 | 0307a5d | ✓ 已推送 | 锚定 |
| chain2-campaign4 | 0307a5d | ✓ 已推送 | 锚定 |

**V2.4_remote 路径**: `D:/235备份文件/V2.4_remote`（bare git 仓库）
**v1.0-dev 分支 HEAD**: 0307a5d（batch36-S 终点 commit）✓

### 8.2 Bundle 第二物理位置清单

| Bundle | 主位置 (V2.4_Performance_Sandbox/) | 第二物理位置 (bundle/) | 大小 | 完整性验证 |
|--------|-------------------------------------|------------------------|------|------------|
| v2.4-post-batch33.bundle | ✓ | ✓ `D:/235备份文件/bundle/` | 27.8MB | ✓ okay |
| v2.4-post-batch34.bundle | ✓ | ✓ `D:/235备份文件/bundle/` | 27.8MB | ✓ okay |
| v2.4-post-batch35.bundle | ✓ | ✓ `D:/235备份文件/bundle/` | 27.8MB | ✓ okay |
| v2.4-post-batch36.bundle | ✓ | ✓ `D:/235备份文件/bundle/` | 27.8MB | ✓ okay |

### 8.3 历史 Bundle 第二物理位置

| Bundle | 第二物理位置 | 说明 |
|--------|-------------|------|
| v2.4-chain1-complete.bundle | V2.4_Performance_Sandbox/（主） | CHAIN-1 终点 |
| v2.4-post-batch20.bundle | `D:/235备份文件/` | 散落备份 |
| v2.4-post-batch24~27.bundle | `D:/235备份文件/` | 散落备份 |
| v2.4-post-batch23.bundle | `D:/235备份文件/bundle/` | 已有备份 |
| v24-full-backup-20260907-v2.bundle | `D:/235备份文件/backup/` | 全量备份 |
| batch12-full-backup-20260911.bundle | `D:/235备份文件/` | batch12 全量 |

### 8.4 锚定结论

- **Tag 推送**: 5 个 tag（v2.4-post-batch33~36 + chain2-campaign4）已推送 V2.4_remote ✓
- **Bundle 双物理**: 4 个 CHAIN-2 bundle 均有第二物理位置 `D:/235备份文件/bundle/` ✓
- **Bundle 完整性**: 4 个 bundle 第二位置均通过 `git bundle verify` ✓
- **Commit 可达**: V2.4_remote v1.0-dev HEAD = 0307a5d = batch36-S 终点 ✓

## 9. 判定

**CHAIN-2 战役IV 全链 PASS，可合并。**

---

> **CHAIN-2 销链声明**
> 链: batch33-S → 34-S → 35-S → 36-S
> 四项必答: ①口径对照表 ✓ ②红线活性演习 ✓ ③AUDIT-026完整版 ✓ ④备份锚定 ✓
> 全链 PASS，正式销链，停机待命。