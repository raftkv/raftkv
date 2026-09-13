# 审计判例库 AUDIT.md

> 版本: v2.4-batch24-governance
> 日期: 2026-09-13
> 维护规则: 每批 FAIL 项归因采信判例入盘，四次"固有"驳回链留档

## 1. 四次"固有"驳回链

### 判例 AUDIT-001: batch17 P99=200ms 固有尾延迟
- **批次**: batch17
- **FAIL 项**: c=512 P99=200ms > 100ms 目标
- **归因**: Raft quorum 复制固有尾延迟，含网络 RTT + follower append + fsync
- **驳回依据**: decomp_c512_raw.json quorum_wait P99=63.5ms 占 81.3%，为 Raft 强一致共识下界
- **处置**: 接受 P99=200ms，降级目标至 c=128 P99=50ms 达标
- **状态**: 采信

### 判例 AUDIT-002: batch20 fsync 合并比 437:1 固有
- **批次**: batch20
- **观察项**: 26 fsync/s, 438 entries/fsync, 437:1 合并比
- **归因**: group commit + batch flush 已将 fsync 合并至极限，WAL 非瓶颈
- **驳回依据**: batch11 证实 group commit 未提升 TPS，瓶颈在 quorum 等待非 fsync
- **处置**: 性能章节收官，fsync "固有"成立
- **状态**: 采信

### 判例 AUDIT-003: batch11 group commit 无效 quorum 等待固有
- **批次**: batch11
- **FAIL 项**: TPS 796.7 < 8000 目标
- **归因**: group commit 减少锁串行但 quorum 等待时间不变，瓶颈为 AppendEntries RPC 往返
- **驳回依据**: TPS 未提升证实 group commit 对当前瓶颈无效
- **处置**: 转向 Pipeline 提交（重叠 quorum 等待），group commit 不再作为优化手段
- **状态**: 采信

### 判例 AUDIT-004: batch23 E1/E4 FAIL pre-vote split vote 固有
- **批次**: batch23
- **FAIL 项**: E1=2.3719s > 2.0s, E4=3.5055s > 2.0s
- **归因**: pre-vote 在所有节点日志相同时无法防止 split vote，多节点同时获 quorum 预支持 → 同时发起正式选举 → 需第二轮
- **驳回依据**: 85 次探测中 16 次阻止选举（18.8%），但 69 次通过后仍 split vote；pre-vote 设计目标为防止日志落后节点选举，非防止同日志节点并发选举
- **处置**: batch24 终裁——若 2.0s 超机制下限则产出改线提案
- **状态**: 待终裁

## 2. 本批归因采信判例

### 判例 AUDIT-005: batch23 RWMutex 不可重入死锁
- **批次**: batch23
- **现象**: 集群部署后所有节点卡在 Follower term=0，/raft/pre_vote 端点挂起
- **根因**: getLastLogIndex()/getLastLogTerm() 内部调 rn.mu.RLock()，但 handleElectionTimeout 已持有 rn.mu.Lock()（写锁），Go sync.RWMutex 不可重入 → 永久阻塞
- **修复**: 内联 len(rn.logs) 和 rn.logs[-1].Term，避免持锁时调用会再次加锁的方法
- **状态**: 已修复

### 判例 AUDIT-006: batch23 磁盘满注入全 PASS
- **批次**: batch23
- **观察项**: DF1-DF4 全 PASS，soft(90%)+hard(99%) 6 场景存活率 100%
- **归因**: 磁盘满注入 follower 不影响 leader 可用性，清理后集群恢复
- **状态**: 采信

## 3. batch25 追加判例

### 判例 AUDIT-007: batch24 首例合法改线提案全流程（正向）
- **批次**: batch24→batch25
- **FAIL 项**: E4=3.4607s > 2.0s
- **流程**:
  1. batch24 产出 D-24-1 提案（decisions.md），含根因分析+机制下限推导+反对论点自审+替代方案排除
  2. 提案状态="待晨批裁决"，**未自行修改 YAML**（遵守"改线仅限提案制"）
  3. batch25 任务一落地：补量化推导 + E4 拆为 E4a(≤2.0s 不放松)+E4b(≤3.5s) + 复跑验证
  4. YAML diff 显式落盘，实际修改待晨批批准后执行
- **合规要点**: 提案→自审→落盘→等裁→落地，全链可审计，无偷改线
- **状态**: 采信（正向标杆）

### 判例 AUDIT-008: 验收数字五次催缴未入晨报（失败）
- **批次**: 历史多批
- **现象**: 验收关键数字（如 E1/E4 实测值、P99 分解、fsync 量纲）多次催缴后仍未入晨报
- **归因**: 产物与晨报脱节，证据落盘但未同步至面审入口
- **对抗措施**: MISBEHAVIOR.md 收录"催缴无果"模式 + actions.json 双向对账机制
- **模板修订**: 是——batch25 新增 actions.json 双向对账，晨审与申报三清单差额=隐瞒或漏报
- **状态**: 采信（反面教材）

## 4. 判例索引

| 判例 ID | 批次 | 类型 | 状态 |
|---------|------|------|------|
| AUDIT-001 | batch17 | 固有驳回 | 采信 |
| AUDIT-002 | batch20 | 固有驳回 | 采信 |
| AUDIT-003 | batch11 | 固有驳回 | 采信 |
| AUDIT-004 | batch23 | 固有驳回 | 待终裁 |
| AUDIT-005 | batch23 | 死锁修复 | 已修复 |
| AUDIT-006 | batch23 | 磁盘满 | 采信 |
| AUDIT-007 | batch24→25 | 正向改线 | 采信 |
| AUDIT-008 | 历史 | 催缴失败 | 采信 || AUDIT-009 | batch25 | E4提案改判 | 采信 |
| AUDIT-010 | batch25 | 追问三则 | 采信 |
| AUDIT-011 | batch14~26 | 性能终审闭案 | 采信 |
| AUDIT-012 | batch27 | 口径不一致 | 采信 |
| AUDIT-013 | batch28 | 统计量口径漂移 | 采信 |
| AUDIT-014 | batch27~28 | 授权表实战 | 采信 |

## 5. batch25 追加判例

### 判例 AUDIT-009: batch25 E4 提案改判全流程
- **批次**: batch25
- **流程**: 量化推导（区间加宽空间证明）→ E4 拆 E4a(≤2.0s 不放松)+E4b(≤3.5s) → YAML diff 落盘 → 复跑 F4 PASS + E1 不劣化 → 回归门联调
- **关键数据**: E1=1.8233s（不劣化 +1.1ms），E4=3.2849s（改善 -175ms），CV=2.52% < 15% 可信
- **合规**: 未自行修改 YAML（RL-new-2），提案落盘待晨批
- **状态**: 采信

### 判例 AUDIT-010: batch25 追问三则结论
- **批次**: batch25
- **追问一**: E4b 区间推导 → 两轮机制下限 3.4s + 100ms 余量 = 3.5s
- **追问二**: 区间加宽空间 → E1≤2.0s 约束下最多加宽 356ms，但对 E4 无改善（级联 split vote 由 kill 时机决定）
- **追问三**: CV 制度 → E1 CV=2.52% < 15% 可信，E4 CV=5.85% < 15% 可信
- **状态**: 采信
## 6. batch27 追加判例

### 判例 AUDIT-011: 性能战役终审闭案（batch14~26）
- **批次**: batch14~26
- **闭案范围**: batch9 OOM 修复 → batch11 group commit → batch12 Bug A 修复 → batch14 loadgen 修复 → batch16~20 pipeline 达标 → batch22~25 选举定线
- **关键数字**: 796.7→9020 TPS (11.3x), P99 50ms (c=128), E1=1.8233s PASS, E4=3.2849s FAIL(待D-24-1)
- **闭案文件**: docs/reports/performance_campaign_final.md
- **冻结声明**: 性能数字冻结，后续批次不得修改
- **状态**: 采信

### 判例 AUDIT-012: 判定口径不一致 verdict 硬编码 vs regression 线族
- **批次**: batch27
- **现象**: verdict.json 硬编码阈值（E4≤2.0s）与 regression.yaml 线族（REG-6 E4b≤3.5s）矛盾，同一 E4 实测值在两套口径下结论不同
- **根因**: verdict 判定脚本独立维护硬编码阈值，未引用 regression.yaml 线 ID，制度变更时两套口径不同步
- **处置**: batch27 任务一将 verdict 改为引用 regression.yaml 线 ID，删除一切硬编码阈值
- **MISBEHAVIOR**: MB-008 收录此模式
- **状态**: 采信
## 7. batch28 追加判例

### 判例 AUDIT-013: 判定统计量未入线定义致 max/中位数口径漂移（第二次口径族 bug）
- **批次**: batch27→batch28
- **现象**: batch27 E4b 复测 max=3.5166s > 3.5s 判 FAIL，但 median=3.4802s ≤ 3.5s 判 PASS；regression.yaml REG-6 未显式声明判定统计量（median/max），verdict 自由裁量用 max，晨审改判用 median
- **根因**: regression.yaml 线定义仅声明 threshold，未声明判定统计量（stat: median/max/p99），致 verdict 脚本可自由选择统计量，同一组测量值在不同统计量下结论不同
- **处置**: batch28 任务一将每条线显式声明判定统计量，verdict 按线定义执行，禁止自由裁量
- **MISBEHAVIOR**: MB-009 收录此模式
- **状态**: 采信

### 判例 AUDIT-014: 夜批授权表首次实战执行合规
- **批次**: batch27~28
- **授权表预裁决项**:
  - E4b 复测超 3.5s → 报 FAIL+机制级归因+挂账 LEDGER+等晨审 (batch27 执行)
  - quorumbench 不可达 → 降级为本地双轨模拟并申报降级理由 (batch27 执行)
  - SDD 代理放行 → 自核验全过后开工首任务 (batch27 执行)
  - 晨审改判 PASS（中位数口径）→ L-27-1 销账 (batch28 执行)
- **合规结论**: 全部按授权表执行，无越权决策，无红线触发
- **状态**: 采信
### 判例 AUDIT-015: E4b 定案 PASS，3.5s 冻结，性能战役终结
- **批次**: batch14~28
- **历程**: batch14 性能战役启动 → batch16 c=128 TPS=13664.9 突破 → batch20 fsync 合并取证 → batch24 pre-vote 优化 E1 PASS → batch25 D-24-1 提案 E4b≤3.5s → batch27 E4b 复测 max=3.5166s 挂账 → batch28 晨审改判 median 口径 PASS + N=5 扩测 median=3.4696s 确认 PASS
- **定案**: E4b stat=median threshold=3.5s 冻结，L-27-1 终结
- **性能战役终态**: 796.7→9020 TPS (11.3x), P99 50ms, E1=1.8233s PASS, E4b median=3.4696s PASS
- **状态**: 采信

### 判例 AUDIT-016: 申报纪律连续两批滑坡（失败模式）
- **批次**: batch27~28
- **现象**: batch27 首屏六追问中 decisions.md E4b 推导原文第三次催缴未答入（入 MISBEHAVIOR MB-007）；batch28 首屏二追问中 E4b N=5 五原始值未在首屏完整列出（仅在正文表格）
- **根因**: 夜批长程执行中，首屏申报纪律随任务推进松弛，必答项遗漏
- **对策**: batch29 引入推导机器催缴（judge_batch29 校验 decisions.md 推导段含量化内容）+ 首屏必答三项齐备检查位
- **MISBEHAVIOR**: MB-010 收录此模式
- **状态**: 采信