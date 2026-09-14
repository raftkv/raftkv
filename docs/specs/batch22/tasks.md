# batch22 故障注入战役I修复 + 战役II预演 — 实施任务分解

> 版本: v2.4-batch22-election-f3
> 关联规格: spec.md（809 行，37 条 EARS 需求）
> 关联设计: design.md（1128 行，完整技术设计）
> 时间盒: 总计 ≤ 6 小时（任务零 1.5h + 任务一 2h + 任务二 1.5h + 任务三 1h + 闭案 50min + 收尾 15min），到点停手交数据（红线 RL-10）
> 命名约定: snake_case（函数/变量/脚本），kebab-case（目录），大驼峰（文档文件名）
> 前置批次: batch21（F1 FAIL 6.895s / F3 FAIL 0% / F2·F4·F5 PASS / chaos_injector 已实现 7 文件 864 行）
> 任务粒度: 每个任务 ≤ 30 分钟，完成后可独立 commit
> 集群编排: docker compose -p deploy5 -f tests/deploy/docker-compose-5node.yml -f tests/deploy/docker-compose-5node-ports.yml -f tests/deploy/docker-compose-5node-batch16.yml --env-file tests/deploy/deploy.env
> 核心红线: RL-01~RL-10 继承 + RL-11 新增（构建产物禁止入库）

---

## 并行优化总览

> **并行策略**：阶段 0 必须先行（销账 + 根治 + 性能贴齐为后续任务提供基础）；阶段 1（选举优化，改 raft.go）与阶段 2（观测端点，改 main.go）源码修改互不依赖，可并行；阶段 3（磁盘满方案设计，纯文档）可与阶段 1/2 并行；阶段 4 依赖阶段 1+2 全部完成；阶段 5 依赖阶段 4。

| 阶段 | 串行预估 | 并行预估 | 关键约束 |
|------|----------|----------|----------|
| 阶段 0 任务零前置 | 1.5h | 1.0h（T0.1/T0.2/T0.3 三任务并行） | 必须先行，为后续提供 LEDGER + .gitignore + 性能首屏 |
| 阶段 1 选举优化 | 2.0h | 2.0h（T1.1 先行 → T1.2/T1.3/T1.4 并行 → T1.5 收尾） | 与阶段 2/3 可并行 |
| 阶段 2 观测端点 | 1.5h | 1.5h（T2.1 先行 → T2.2/T2.3 并行） | 与阶段 1/3 可并行 |
| 阶段 3 战役II预演 | 1.0h | 0.5h（T3.1/T3.2 并行） | 与阶段 1/2 可并行 |
| 阶段 4 闭案 | 0.83h | 0.66h（T4.1 先行 → T4.2/T4.3 并行 → T4.4） | 依赖阶段 1+2 完成 |
| 阶段 5 收尾 | 0.25h | 0.25h（T5.1/T5.2 并行） | 依赖阶段 4 |
| **串行总计** | **7.08h** | | 超出 6h 时间盒 |
| **并行总计** | | **1.0 + max(2.0, 1.5, 0.5) + 0.66 + 0.25 = 3.91h** | **≤ 6h ✅** |

> **并行后总预估：3.91h ≤ 6h，满足时间盒约束（红线 RL-10）**。预留 2.09h 作为集群部署/编译/重测的缓冲时间。

---

## 阶段 0：任务零前置 — LEDGER 销账 + 构建产物根治 + 性能终审贴齐（预估 1.5h，并行 1.0h）

> **并行策略**：T0.1（LEDGER 销账）/ T0.2（构建产物根治）/ T0.3（性能终审贴齐）三任务互不依赖，可三任务并行。
> **阶段产物**：LEDGER.md 更新（L-21-1/L-21-2 入账清偿）+ .gitignore 补丁 + chaos_injector.exe 移出 + decisions.md 根因分析 + 报告.md 性能首屏数据

### T0.1 LEDGER 逐条销账 + L-21-1/L-21-2 入账
- **ID**: T0.1
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `LEDGER.md`（DEBT-0001~0003 已清偿，3 笔记录）
  - batch21 verdict.json（F1=FAIL 6.895s / F3=FAIL 0%）
  - spec.md §5.1.1 业务规则 1-3（REQ-T0-01/02/03）
- **依赖**: 无
- **并行**: 可与 T0.2/T0.3 并行
- **动作**:
  1. 逐条核对 LEDGER.md 中 DEBT-0001/0002/0003 的清偿状态，确认与实际证据一致（cleared_at: 453dff3 / 453dff3 / 947a1be）
  2. 在 LEDGER.md 表格追加两笔新入账：
     - `L-21-1 | batch21 | batch22 | 待清 | F1 选举 6.895s 归因取证：electionTimeoutMin=5000ms/electionTimeoutMax=7000ms 导致选举完成超时，需本批取证后清偿 | —`
     - `L-21-2 | batch21 | batch22 | 待清 | 观测端点 /raft/entry 缺失致 F3 无法验证（存活率 0%），需本批实现端点后清偿 | —`
  3. 在"已清偿记录详情"章节追加 L-21-1/L-21-2 的待清记录详情（来源/应清批次/清偿条件/数据来源）
  4. 待 T1.1 取证完成后回填 L-21-1 的 cleared_at；待 T2.1 端点实现后回填 L-21-2 的 cleared_at
  5. 格式与现有 DEBT-0001~0003 一致（六字段表格：debt_id/source_batch/target_batch/status/description/cleared_at）
- **输出**: `LEDGER.md`（更新后，含 5 笔记录：3 笔已清偿 + 2 笔待清）
- **验收**: LEDGER.md 存在 → DEBT-0001~0003 状态确认为已清偿 → L-21-1/L-21-2 两笔入账且格式合规 → 每笔有唯一 ID → 表格六字段齐全
- **commit 点**: `git add LEDGER.md && git commit -m "T0.1: LEDGER 销账 + L-21-1/L-21-2 入账"`
- **关联需求**: REQ-T0-01, REQ-T0-02, REQ-T0-03

### T0.2 构建产物入库根治（.gitignore 补丁 + chaos_injector.exe 移出 + 根因分析）
- **ID**: T0.2
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - 现有 `.gitignore`（已排除证据目录 + loadgen.exe，未排除 chaos_injector.exe / build/ 目录）
  - `cmd/chaos_injector/chaos_injector.exe`（batch21 编译产物误入库）
  - spec.md §5.1.1 业务规则 4（REQ-T0-04 / RL-11）
  - design.md §1.2.6 .gitignore 现状与根因分析
- **依赖**: 无
- **并行**: 可与 T0.1/T0.3 并行
- **动作**:
  1. 补丁 `.gitignore`，新增排除规则：
     - `*.exe`（通配符，覆盖所有 Go 编译产物：loadgen.exe / chaos_injector.exe / write_tester.exe 等）
     - `build/`（CI/CD 构建产物目录）
     - `cmd/*/*.exe`（cmd 子工具编译产物，双重保险）
  2. 将 `cmd/chaos_injector/chaos_injector.exe` 移出仓库：`git rm --cached cmd/chaos_injector/chaos_injector.exe`（保留本地文件，仅移出 git 索引）
  3. 若文件被其他进程占用无法删除，记录占用错误，先补丁 .gitignore（已生效），延迟重试移出
  4. 在 `decisions.md` 中落盘 batch20 修正为何未延续的根因分析：
     - 根因：build 目录规则未固化，缺乏通配符排除规则，每次新增 cmd 子工具时需手动补丁 .gitignore
     - batch20：loadgen.exe 误入库，已修正（逐个排除）
     - batch21：修正未延续——chaos_injector.exe 误入库，.gitignore 补丁不完整
     - 根治措施：`*.exe` 通配符 + `build/` 目录排除，一劳永逸
  5. 固化 build 目录规则文档（可在 decisions.md 中或独立文档说明：新增 cmd 子工具时自动被 .gitignore 覆盖）
  6. 确认 `git status` 干净（无 .exe / build/ 相关未跟踪条目）
- **输出**: `.gitignore`（补丁后）+ `decisions.md`（根因分析段落）+ chaos_injector.exe 移出 git 索引
- **验收**: 仓库中不存在 .exe / 二进制产物 → .gitignore 包含 `*.exe` + `build/` 排除规则 → build 目录规则已固化 → decisions.md 含 batch20 修正未延续根因分析 → `git status` 干净
- **commit 点**: `git add .gitignore decisions.md && git rm --cached cmd/chaos_injector/chaos_injector.exe && git commit -m "T0.2: 构建产物入库根治 + RL-11 红线升级"`
- **关联需求**: REQ-T0-04, RL-08, RL-11

### T0.3 性能终审数据首屏直贴（decomp 四构成项 P50/P99/占比 + fsync 量纲）
- **ID**: T0.3
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `docs/specs/latency_decomp/decomp_c512_raw.json`（4037 bytes，四构成项已补全）
    - quorum_wait: P50=12436µs(74.4%) P99=63465µs(81.3%)
    - fsync_wait: P50=47µs(0.3%) P99=18165µs(23.3%)
    - rpc: P50≈5000µs(29.9%) P99≈25000µs(32.0%)
    - queue: P50=1563µs(9.3%) P99=32537µs(41.7%)
  - `tests/evidence/d3-batch20/fsync_forensics.json`（窗口 180s / fsync 总次数 4673 / 25.96 fsync/s / 合并比 437:1）
  - spec.md §5.1.1 业务规则 5-7（REQ-T0-05/06/07）+ §6.7 decomp 数据约束 + §6.8 fsync 量纲约束
- **依赖**: 无
- **并行**: 可与 T0.1/T0.2 并行
- **动作**:
  1. 从 `decomp_c512_raw.json` 提取四构成项 × {P50, P99, P50_ratio, P99_ratio} 全字段（16 个数字）
  2. 从 `fsync_forensics.json` 提取量纲数据：压测窗口时长=180s（test_config.duration）、窗口内 fsync 总次数=4673（leader.fsync_count）、每秒 fsync=25.96（leader.fsync_per_sec）、合并比=437:1（analysis.merge_ratio_leader）
  3. 在 `tests/evidence/d3-batch22/报告.md` 首屏创建"性能终审数据"章节，直接贴出：
     - decomp_c512 P99 分解表（四构成项 × {P50, P99, P50占比, P99占比}，16 个数字齐全）
     - fsync 量纲表（窗口时长 / 总次数 / 每秒 / 合并比 / 判定）
  4. 数据来源标注：decomp 来源=batch18 埋点实测，fsync 来源=batch20 取证
  5. 确认数字与 JSON 源文件逐字段一致（不得四舍五入或改写）
  6. 确认数据可直接决定性能章节是否正式闭案（附判定结论：INTRINSIC_CONFIRMED）
- **输出**: `tests/evidence/d3-batch22/报告.md`（首屏性能终审数据章节）
- **验收**: 报告.md 首屏 → 包含 decomp_c512 四构成项 P50/P99 及占比数字（16 个数字齐全且与 decomp_c512_raw.json 一致）→ 包含 fsync 窗口时长 180s + 总次数 4673 + 每秒 25.96 + 合并比 437:1 → 数据可直接决定性能章节是否正式闭案
- **commit 点**: `git add tests/evidence/d3-batch22/报告.md && git commit -m "T0.3: 性能终审数据首屏直贴（decomp 四构成项 + fsync 量纲）"`（注：证据目录被 .gitignore 排除，实际 commit 仅含代码文件，报告.md 落盘但不入库）
- **关联需求**: REQ-T0-05, REQ-T0-06, REQ-T0-07

---

## 阶段 1：选举优化 — 治 F1（预估 2h，并行 2h）

> **并行策略**：T1.1（选举取证）先行；T1.2（超时调参）/ T1.3（pre-vote 实现）/ T1.4（batch22.yaml 契约）均仅依赖 T1.1，可三任务并行；T1.5（验证）依赖 T1.2+T1.3+T1.4 全部完成。
> **阶段产物**：election_forensics.json + raft.go 调参 + pre-vote 实现 + batch22.yaml + E1-E3 验收证据
> **与阶段 2/3 可并行**：选举优化改 raft.go，观测端点改 main.go，磁盘满方案纯文档，三者源码修改互不依赖

### T1.1 选举取证（当前配置/随机区间/轮数/选票分布落盘）
- **ID**: T1.1
- **状态**: [ ] 未开始
- **预估**: 25 分钟
- **输入**:
  - 5 节点 Raft 集群（HTTP 9001-9005，leader/follower 可动态识别）
  - `/raft/stats` 端点（main.go:293-305，返回文本格式 `id=... state=... term=... leader=... voted=...`）
  - `cmd/chaos_injector/collector.go`（已有 CollectElectionTimeline，需扩展 CollectElectionForensics）
  - design.md §2.2.2.3 CollectElectionForensics 接口签名 + §2.3.2 ElectionForensics 数据模型
  - spec.md §5.2.1 业务规则 1（REQ-E-01）+ §6.2 选举取证 JSON 结构
- **依赖**: 阶段 0 完成（LEDGER L-21-1 已入账，待取证后清偿）
- **并行**: 不可并行（T1.2/T1.3/T1.4 的基础）
- **动作**:
  1. 在 `cmd/chaos_injector/collector.go` 中新增 `CollectElectionForensics(leaderID string, killTimeout time.Duration) (*ElectionForensics, error)` 方法
  2. 采集前通过 `/raft/stats` 读取当前 electionTimeout 配置（electionTimeoutMin=5000ms / electionTimeoutMax=7000ms）和随机区间
  3. kill 当前 leader（`docker kill --signal=9 raft-node-N`）
  4. 100ms 轮询各节点 `/raft/stats` 的 `state/voted/term` 字段，记录 6.895s 内的选举轮数与每轮选票分布
  5. 记录 kill_to_election_complete（wall-clock 秒）
  6. 落盘至 `tests/evidence/d3-batch22/election_forensics.json`，结构含：
     - `current_election_timeout`: "5000-7000ms"
     - `random_range`: [5000, 7000]
     - `round_count_within_6895ms`: int
     - `vote_distribution`: []VoteRound（每轮 term + 各节点得票数 + winner）
     - `kill_to_election_complete`: float64
     - `timestamp`: time.Time
  7. 恢复集群（重启被 kill 的节点，等待健康）
  8. 回填 LEDGER.md 中 L-21-1 的 cleared_at（标注本 commit SHA）
- **输出**: `tests/evidence/d3-batch22/election_forensics.json` + `cmd/chaos_injector/collector.go`（扩展）+ LEDGER.md L-21-1 清偿
- **验收**: election_forensics.json 落盘 → 含超时配置 / 随机区间 / 轮数 / 选票分布 / kill_to_election_complete → 数据可指导优化决策 → JSON ≤ 10KB
- **commit 点**: `git add cmd/chaos_injector/collector.go LEDGER.md && git commit -m "T1.1: 选举取证 + L-21-1 清偿"`
- **关联需求**: REQ-E-01, REQ-T0-02

### T1.2 选举超时调参（5000-7000ms → 800-1200ms）
- **ID**: T1.2
- **状态**: [ ] 未开始
- **预估**: 20 分钟
- **输入**:
  - T1.1 产出的 `election_forensics.json`（取证数据指导调参决策）
  - `raft.go:39-40`（electionTimeoutMin=5000 / electionTimeoutMax=7000 编译时常量）
  - `raft.go:2051-2053`（randomElectionTimeout 随机化函数）
  - design.md §1.2.1 选举超时配置 + §2.1.2 配置项取值策略 + §7.1 调参决策
  - spec.md §5.2.1 业务规则 2-3（REQ-E-02/03）+ §4.1 性能约束
- **依赖**: T1.1
- **并行**: 可与 T1.3/T1.4 并行
- **动作**:
  1. 在 `raft.go` 中将 `electionTimeoutMin` 从 5000 改为 800（毫秒）
  2. 将 `electionTimeoutMax` 从 7000 改为 1200（毫秒）
  3. 新增环境变量可配置支持（向后兼容）：
     - 启动时读取 `ELECTION_TIMEOUT_MIN` / `ELECTION_TIMEOUT_MAX` 环境变量
     - 未设置时使用编译时常量默认值（800 / 1200）
     - 校验：MIN > heartbeatIntervalMax(500ms)、MAX > MIN、区间宽度 ≥ 200ms、MIN > rpcTimeout
     - 校验失败 → log.Fatal 拒绝启动（fail-closed）
  4. `randomElectionTimeout()` 函数不变（仍返回 `[min, max)` 区间内的随机 Duration）
  5. 确认调参后区间宽度 400ms（1200-800）提供随机化，避免多节点同时超时
  6. 确认 800ms > heartbeatIntervalMax(500ms)，正常心跳不误触发选举
  7. 重新编译集群镜像并部署（5 节点 Docker 集群）
- **输出**: `raft.go`（调参后，electionTimeoutMin=800 / electionTimeoutMax=1200 + 环境变量支持）
- **验收**: 调参后选举超时为 800-1200ms 随机区间 → 随机化区间性质保持 → 800ms > heartbeatIntervalMax(500ms) → 环境变量未设置时使用默认值（向后兼容）→ 编译通过
- **commit 点**: `git add raft.go && git commit -m "T1.2: 选举超时调参 5000-7000ms → 800-1200ms + 环境变量支持"`
- **关联需求**: REQ-E-02, REQ-E-03, RL-03

### T1.3 pre-vote 实现（防选票分裂）
- **ID**: T1.3
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - T1.1 产出的 `election_forensics.json`（若轮数多/选票分裂则 pre-vote 必要）
  - `raft.go:790-860`（handleElectionTimeout 选举状态机，pre-vote 注入点）
  - `pkg/raft-module/raft_transport.go`（RequestVote RPC 传输层，PreVote RPC 复用）
  - design.md §1.2.2 handleElectionTimeout 分析 + §2.1.3.3 pre-vote 机制设计 + §3.2 选举超时语义论证 + §7.3 pre-vote 决策
  - spec.md §5.2.1 业务规则 4（REQ-E-04）
- **依赖**: T1.1
- **并行**: 可与 T1.2/T1.4 并行
- **动作**:
  1. 在 `raft.go` 中新增 PreVote RPC 消息类型（与现有 RequestVote 共存，通过消息类型区分）
  2. 在 `handleElectionTimeout()` 入口增加 pre-vote 探测逻辑：
     - 选举定时器到期 → 不直接转 Candidate，先发送 PreVote RPC（term+1, lastLogIndex, lastLogTerm）
     - **pre-vote 不增加 term**（避免无谓 term 膨胀，防止网络分区节点回归后触发不必要选举）
     - 统计预支持票 → ≥ quorum(3/5) 则转正式 Candidate（现有 RequestVote 流程：term+1 + 投自己 + 并发发送 RequestVote）
     - < quorum 则保持 Follower 并重置选举定时器
  3. pre-vote 失败不阻止其他节点发起选举（不引入活锁）
  4. 正式选举阶段仍遵循现有 RequestVote + quorum + term+1 逻辑（Raft 安全性保持）
  5. 新增 PreVote RPC handler（接收方处理：若候选人 log 足度 ≤ 自己 log 则预支持，否则拒绝）
  6. 重新编译部署集群
- **输出**: `raft.go`（handleElectionTimeout 扩展 pre-vote 探测）+ PreVote RPC handler
- **验收**: pre-vote 实现后 → 选举轮数减少（选票分裂减少）→ pre-vote 不增加 term → pre-vote 失败保持 Follower → 正式选举仍遵循 quorum 约束 → 编译通过
- **commit 点**: `git add raft.go && git commit -m "T1.3: pre-vote 防选票分裂实现"`
- **关联需求**: REQ-E-04, RL-03

### T1.4 batch22.yaml 验收契约（E1-E3 / S1-S2 / F2/F4/F5 继承）
- **ID**: T1.4
- **状态**: [ ] 未开始
- **预估**: 20 分钟
- **输入**:
  - T1.1 产出的取证数据（确认 E1-E3 验收线定义）
  - `tests/contracts/batch21.yaml`（F1-F5 验收线，复用 F2/F4/F5）
  - design.md §2.2.2.4 batch22.yaml 接口签名 + §2.1.2 配置项
  - spec.md §5.5 验收契约 E1-E3/S1-S2 + §6.1 YAML 数据约束 + §8 红线清单 + §9 场景矩阵
- **依赖**: T1.1
- **并行**: 可与 T1.2/T1.3 并行
- **动作**:
  1. 创建 `tests/contracts/batch22.yaml`
  2. 定义 E1-E3 验收阈值：
     - E1: `election_completion_time_max: 2.0`（秒，3 次取中位，c=128 持续下测量，operator: <=）
     - E2: `max_concurrent_leaders: 1`（复跑 F4 全绿，任一时刻）
     - E3: `reject_rate_during_injection_max: 30`（百分比，不劣化 batch21 基线 20%）
  3. 定义 S1-S2 验收阈值：
     - S1: `confirmed_write_survival_rate: 100`（百分比，通过 /raft/entry 端点抽样比对）
     - S2: `write_path_p99_fluctuation_max: 5`（百分比，端点运行期间写路径 P99 波动）
  4. 继承 batch21 验收线：F2（拒载率 ≤ 30%，恢复后 = 0）/ F4（无脑裂 ≤ 1）/ F5（日志完整可回放）
  5. 定义场景矩阵（election_opt_01~05 + entry_survival_01~03 + endpoint_latency_01 + inherited_f2/f4/f5 = 12 场景）
  6. 定义红线清单（RL-01~RL-11，含新增 RL-11 构建产物禁止入库）
  7. 定义时间盒（`timebox_hours: 6`，`on_expire: stop_and_hand_over`）
  8. 格式与既有 batch21.yaml 契约一致（spec 4.5 兼容性）
- **输出**: `tests/contracts/batch22.yaml`
- **验收**: YAML 格式合法（`python -c "import yaml; yaml.safe_load(open('tests/contracts/batch22.yaml'))"` 通过）→ E1-E3/S1-S2 阈值齐全 → F2/F4/F5 继承 → 场景矩阵 12 场景 → 红线 11 条 → 时间盒 6h
- **commit 点**: `git add tests/contracts/batch22.yaml && git commit -m "T1.4: batch22 验收契约 YAML（E1-E3/S1-S2 + F2/F4/F5 继承）"`
- **关联需求**: REQ-E1, REQ-E2, REQ-E3, REQ-S1, REQ-S2, REQ-YAML

### T1.5 选举优化验证（杀 leader ≤2s + F4 回归 + F2 不劣化）
- **ID**: T1.5
- **状态**: [ ] 未开始
- **预估**: 25 分钟
- **输入**:
  - T1.2 产出的调参后集群（electionTimeout 800-1200ms）
  - T1.3 产出的 pre-vote 实现
  - T1.4 产出的 batch22.yaml（E1-E3 阈值）
  - `cmd/chaos_injector/scheduler.go`（需新增选举优化场景调度）
  - loadgen 负载驱动（c=128 压测）
  - spec.md §5.2.1 业务规则 5（REQ-E-05）+ §5.5 验收契约 + §9.1 选举优化场景
- **依赖**: T1.2, T1.3, T1.4
- **并行**: 不可并行（依赖 T1.2+T1.3+T1.4 全部完成）
- **动作**:
  1. 在 `cmd/chaos_injector/scheduler.go` 中新增选举优化场景调度（election_opt_01~05）
  2. **E1 验证**（election_opt_01/04）：
     - 启动 c=128 压测负载持续运行
     - kill leader（第 1 次）→ 记录选举完成时间 T1
     - kill leader（第 2 次）→ 记录 T2
     - kill leader（第 3 次）→ 记录 T3
     - 取中位 median(T1, T2, T3)，验证 ≤ 2.0s
     - 稳态场景（无负载）重复上述流程
  3. **E2 验证**（election_opt_02/05）：
     - kill leader 后 50ms 轮询所有节点 leader 状态
     - 记录 max_concurrent_leaders，验证 ≤ 1（复跑 F4 全绿）
  4. **E3 验证**（election_opt_03）：
     - c=128 负载持续，kill leader，测量选举期间拒绝率
     - 验证 ≤ 30%（不劣化 batch21 基线 20%）
  5. 落盘 E1-E3 证据 JSON 至 `tests/evidence/d3-batch22/election_opt_*.json`
  6. **异常处理**：
     - 若 E1 FAIL（中位 > 2s）：如实记录，附实测中位值与取证数据路径，不自行放宽阈值（RL-05）
     - 若 E2 FAIL（检测到脑裂）：回退 pre-vote 实现（恢复 handleElectionTimeout 原逻辑），标注 E2=FAIL，触发红线
     - 若 E3 FAIL（拒绝率 > 30%）：如实记录，附实测拒绝率与基线对照
- **输出**: `tests/evidence/d3-batch22/election_opt_01~05.json`（E1-E3 验收证据）+ `cmd/chaos_injector/scheduler.go`（扩展）
- **验收**: E1 中位 ≤ 2s → E2 max(leaders) ≤ 1 → E3 拒绝率 ≤ 30% → 证据 JSON 落盘 → 时间线单调递增无缺口
- **commit 点**: `git add cmd/chaos_injector/scheduler.go && git commit -m "T1.5: 选举优化验证 E1-E3（杀 leader ≤2s + F4 回归 + F2 不劣化）"`
- **关联需求**: REQ-E-05, REQ-E1, REQ-E2, REQ-E3

---

## 阶段 2：观测端点 — 治 F3（预估 1.5h，并行 1.5h）

> **并行策略**：T2.1（端点实现）先行；T2.2（F3 全场景重测）/ T2.3（S2 验证）均仅依赖 T2.1，可两任务并行。
> **阶段产物**：/raft/entry handler + 端点三件套 + F3 重测证据 + S1-S2 验收证据
> **与阶段 1/3 可并行**：端点实现改 main.go，选举优化改 raft.go，磁盘满方案纯文档，三者互不依赖

### T2.1 实现 GET /raft/entry 端点
- **ID**: T2.1
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `main.go:288-305`（/raft/status 和 /raft/stats 端点，参考实现风格）
  - `node.Stats().Snapshot()`（获取 commit_index）
  - RaftNode log 只读访问方法（`node.LogEntries(start, end)` 或等效方法，若不存在需新增）
  - `cmd/chaos_injector/node_ctl.go:202,242`（SnapshotConfirmedEntries / VerifyEntrySurvival 已调用 /raft/entry，端点实现后自动生效）
  - design.md §2.2.2.1 GET /raft/entry 接口签名 + §3.3 端点不引入写路径延迟论证 + §7.2 端点设计决策
  - spec.md §5.3.1 业务规则 1-2/5（REQ-S-01/02/05）+ §6.3 F3 重测证据 JSON
- **依赖**: 阶段 0 完成（LEDGER L-21-2 已入账，待端点实现后清偿）
- **并行**: 不可并行（T2.2/T2.3 的基础）
- **动作**:
  1. 在 `main.go` 中新增 `/raft/entry` HTTP handler，独立注册（`httpMux.HandleFunc("/raft/entry", handleRaftEntry)`），不在 /raft/propose handler 内调用
  2. 实现 `handleRaftEntry(w, r)`：
     - 解析 query 参数 `index`（默认 commit_index）和 `count`（默认 10，上限 100）
     - **加 RLock 读快照**（不调用任何写操作，不 propose/commit/fsync）
     - 从 log 中读取 [index, index+count) 范围 entry
     - 过滤 index ≤ commit_index 的已确认 entry
     - JSON 编码返回：`{"entries": [{"index": N, "term": T, "value": "...", "committed": true}], "commit_index": N, "node_id": "..."}`
     - 释放 RLock
  3. 异常处理：
     - index < logStartIndex（已压缩）→ 400 `{"error":"log compacted", "log_start_index": N}`
     - index > commit_index（未确认）→ 200 `{"entries": [], "commit_index": N}`
     - 参数格式错误 → 400 `{"error":"invalid parameter"}`
  4. 若 `node.LogEntries(start, end)` 接口不存在，在 RaftNode 上新增只读访问方法（加 RLock，返回 log 副本）
  5. 补齐端点三件套：
     - 端点实现代码（main.go handler）✅
     - 端点文档（API 契约说明：请求参数 / 响应体 / 异常映射，可加注释或独立文档）
     - 端点测试（单元测试：mock log 数据验证 handler 输出 + 集成测试：集群运行时调用端点验证返回）
  6. 重新编译部署集群
  7. 回填 LEDGER.md 中 L-21-2 的 cleared_at（标注本 commit SHA）
- **输出**: `main.go`（新增 /raft/entry handler）+ 端点文档 + 端点测试 + LEDGER.md L-21-2 清偿
- **验收**: `GET /raft/entry` 可调用 → 返回 JSON 含 entry 列表 → 每个 entry 含 index/term/value/committed → 三件套齐全（代码+文档+测试）→ 端点为只读操作（RL-07）→ chaos_injector 的 SnapshotConfirmedEntries/VerifyEntrySurvival 自动生效
- **commit 点**: `git add main.go LEDGER.md && git commit -m "T2.1: GET /raft/entry 端点实现 + 三件套 + L-21-2 清偿"`
- **关联需求**: REQ-S-01, REQ-S-02, REQ-S-05, REQ-T0-03, RL-07

### T2.2 F3 全场景重测（杀 leader 前写入 → 新 leader 100% 存活）
- **ID**: T2.2
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - T2.1 产出的 /raft/entry 端点（已实现并部署）
  - `cmd/chaos_injector/node_ctl.go`（SnapshotConfirmedEntries / VerifyEntrySurvival 已就绪，端点实现后自动生效）
  - `cmd/chaos_injector/scheduler.go`（需新增端点验收场景调度）
  - design.md §2.1.3.2 /raft/entry 端点 + F3 重测流程 + §5.1 与 chaos_injector 关系
  - spec.md §5.3.1 业务规则 3-4（REQ-S-03/04）+ §6.3 F3 重测证据 JSON + §9.2 观测端点场景
- **依赖**: T2.1
- **并行**: 可与 T2.3 并行
- **动作**:
  1. 在 `cmd/chaos_injector/scheduler.go` 中新增端点验收场景调度（entry_survival_01~03）
  2. **entry_survival_01**（杀 leader 后比对）：
     - 通过 /raft/entry 采样 leader 已确认 entry（10-20 条，记录 index/term/value）
     - kill leader → 等待新 leader 选出
     - GET /raft/entry 查询新 leader 的 entry 列表
     - 逐条比对：采样 entry 的 index ≤ 新 leader commit_index 且内容一致
     - 计算存活率 = 存活 entry 数 / 采样 entry 数 × 100%
  3. **entry_survival_02**（重复场景 01，不同 entry 采样）：换一批 entry 采样重复比对
  4. **entry_survival_03**（连杀场景）：连杀 2 个 leader，采样比对存活率
  5. 落盘 F3 重测证据 JSON 至 `tests/evidence/d3-batch22/entry_survival_01~03.json`，结构含：
     - scenario_id / sampled_entries / new_leader_id / new_leader_commit_index / survived_entries_count / survival_rate / mismatched_entries / status
  6. **异常处理**：
     - 端点返回空列表：检查集群 commit_index > 0，若端点实现有误则修复
     - 抽样比对发现 entry 丢失：记录丢失 entry 详情（index/term/预期值/实际值），判定 S1=FAIL，触发红线（数据丢失）
- **输出**: `tests/evidence/d3-batch22/entry_survival_01~03.json`（S1 验收证据）+ `cmd/chaos_injector/scheduler.go`（扩展）
- **验收**: 3 场景存活率均 = 100% → S1=PASS → 证据 JSON 落盘 → mismatched_entries 为空 → 时间线单调递增无缺口
- **commit 点**: `git add cmd/chaos_injector/scheduler.go && git commit -m "T2.2: F3 全场景重测（杀 leader 前写入 → 新 leader 100% 存活）"`
- **关联需求**: REQ-S-03, REQ-S-04, REQ-S1

### T2.3 S2 验证（端点 P99 波动 ≤5%）
- **ID**: T2.3
- **状态**: [ ] 未开始
- **预估**: 25 分钟
- **输入**:
  - T2.1 产出的 /raft/entry 端点（已实现并部署）
  - loadgen 负载驱动（c=512 压测）
  - `docs/specs/latency_decomp/decomp_c512_raw.json`（写路径 P99 基线 ≈ 78ms）
  - design.md §3.3 /raft/entry 端点不引入写路径延迟论证 + §2.1.3.2 S2 验收
  - spec.md §5.3.1 业务规则 2（REQ-S-02）+ §5.5 验收契约 S2 + §9.2 endpoint_latency_01 场景
- **依赖**: T2.1
- **并行**: 可与 T2.2 并行
- **动作**:
  1. 在 `cmd/chaos_injector/scheduler.go` 中新增 endpoint_latency_01 场景调度
  2. **基线测量**：/raft/entry 端点未运行时，c=512 压测 60s，记录写路径 P99_baseline
  3. **端点运行测量**：/raft/entry 端点运行期间（每 1s 调用一次端点），c=512 压测 60s，记录写路径 P99_with_endpoint
  4. 计算 P99 波动 = |P99_with_endpoint - P99_baseline| / P99_baseline × 100%
  5. 落盘 S2 证据 JSON 至 `tests/evidence/d3-batch22/endpoint_latency_01.json`，结构含：
     - scenario_id / p99_baseline / p99_with_endpoint / p99_fluctuation / status
  6. **异常处理**：
     - P99 波动 > 5%：排查端点实现是否侵入写路径热区，修复后重测；若无法修复则标注 S2=FAIL
- **输出**: `tests/evidence/d3-batch22/endpoint_latency_01.json`（S2 验收证据）
- **验收**: P99 波动 ≤ 5% → S2=PASS → 端点为只读操作（RL-07）→ 端点代码不在写路径热区
- **commit 点**: 不独立 commit（与 T2.2 合并 commit，或单独 `git commit -m "T2.3: S2 验证（端点 P99 波动 ≤5%）"`）
- **关联需求**: REQ-S-02, REQ-S2, RL-07

---

## 阶段 3：战役II预演 — 磁盘满方案设计（预估 1h，并行 0.5h）

> **并行策略**：T3.1（spec 落盘）/ T3.2（design 落盘）互不依赖，可两任务并行。
> **阶段产物**：disk_full_spec.md + disk_full_design.md（实现留 batch23）
> **与阶段 1/2 可并行**：纯文档工作，不涉及源码修改

### T3.1 磁盘满方案 spec 落盘
- **ID**: T3.1
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - batch22 spec.md §5.4 任务三业务规则（REQ-D-01~05）+ §6.4 磁盘满方案两件套约束
  - 现有 fsync/WAL 持久化机制（`raft_wal.go`，磁盘满时 fsync 失败的预期行为）
  - 现有 Raft 降级机制（`/raft/stats` 的 degraded 字段）
  - batch21 chaos_injector 故障注入框架（磁盘满注入可复用 node_ctl Docker 操作）
- **依赖**: 无（纯方案设计，不依赖阶段 1/2）
- **并行**: 可与 T3.2 并行；可与阶段 1/2 并行
- **动作**:
  1. 创建 `tests/evidence/d3-batch22/disk_full_spec.md`
  2. 写入**注入方式**章节（至少一种具体可执行方法，附操作命令草案）：
     - 方式 A：填充 WAL 目录至 100%（`dd if=/dev/zero of=/wal/disk_full.fill bs=1M count=<剩余空间>`）
     - 方式 B：Docker 卷大小限制模拟磁盘满（`docker run --storage-opt size=1G`）
     - 方式 C：fallocate 快速占用空间（`fallocate -l <size> /wal/disk_full.fill`）
     - 推荐方式 C（快速且可精确控制占用大小）
  3. 写入**预期行为**章节（六维预期）：
     - fsync：fsync 调用返回 ENOSPC 错误，WAL 写入失败
     - entry：新 entry 拒写，/raft/propose 返回 503 或错误码
     - leader：leader 降级或步进（degraded=true，拒绝新写请求）
     - follower：follower 同步停滞（AppendEntries 失败）
     - client：客户端收到错误码（503 Service Unavailable 或 507 Insufficient Storage）
     - 可用性：集群可用性影响（单节点磁盘满 → 4/5 节点仍可用；多数派磁盘满 → 集群不可写）
  4. 写入**验收指标草案**章节（≥ 4 条可量化指标，标注为草案待 batch23 细化）：
     - fsync 失败后集群不脑裂（max_concurrent_leaders ≤ 1）
     - 已确认 entry 不丢失（存活率 = 100%）
     - 磁盘恢复后集群自愈（≤ 30s 恢复正常）
     - 降级期间拒绝率可控（≤ 50%）
  5. 明确标注"实现留 batch23"
- **输出**: `tests/evidence/d3-batch22/disk_full_spec.md`
- **验收**: disk_full_spec.md 存在 → 含注入方式 / 预期行为 / 验收指标草案三章 → 注入方式含至少一种具体可执行方法 + 操作命令草案 → 预期行为六维齐全 → 验收指标 ≥ 4 条 → 标注实现留 batch23
- **commit 点**: 不独立 commit（与 T3.2 合并 commit）
- **关联需求**: REQ-D-01, REQ-D-02, REQ-D-03, REQ-D-04, REQ-D-05

### T3.2 磁盘满方案 design 落盘
- **ID**: T3.2
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - T3.1 产出的 disk_full_spec.md（方案设计基础）
  - 现有 Raft 核心实现（raft.go / raft_wal.go / main.go，分析与现有机制关系）
  - batch21 chaos_injector 工具（7 文件 864 行，分析扩展点）
  - design.md §5.3 与 Raft 核心实现关系 + §5.1 与 chaos_injector 关系（参考分析风格）
- **依赖**: 无（可与 T3.1 并行；T3.1 的 spec 内容可从 spec.md 推导，不强依赖）
- **并行**: 可与 T3.1 并行；可与阶段 1/2 并行
- **动作**:
  1. 创建 `tests/evidence/d3-batch22/disk_full_design.md`
  2. 写入**技术设计**章节：
     - 注入实现方案（chaos_injector 扩展 disk_full 场景：node_ctl 新增 FillDisk/ReleaseDisk 方法）
     - fsync 失败检测机制（WAL 写入错误捕获 + degraded 状态切换）
     - entry 拒写机制（/raft/propose handler 检查 degraded 状态）
     - 集群自愈机制（磁盘空间释放后 fsync 恢复 → degraded=false → 集群恢复正常）
  3. 写入**实现计划**章节（标注留 batch23）：
     - batch23 任务分解草案（注入实现 + 验收场景 + 判定脚本扩展）
     - 预估时间盒（batch23 ≤ 6h）
     - 依赖项（本批 spec + design 两件套）
  4. 写入**与现有机制关系分析**章节：
     - 与 fsync 机制关系（磁盘满 → fsync ENOSPC，不破坏 fsync 语义 RL-01）
     - 与 quorum 机制关系（单节点磁盘满不影响 quorum，多数派磁盘满集群不可写）
     - 与 chaos_injector 关系（复用 node_ctl Docker 操作 + scheduler 场景调度 + collector 指标采集）
     - 与 WAL 持久化关系（磁盘满时 WAL 写入失败，已确认 entry 不丢失因已 fsync 刷盘）
  5. 写入**正确性论证**章节（参考 design.md §3 红线正确性论证风格）：
     - 磁盘满不破坏 fsync 语义（已确认 entry 已刷盘，不丢失）
     - 磁盘满不破坏 quorum 语义（quorum 计算不变）
     - 磁盘满不引入脑裂（leader 降级不产生新 leader）
  6. 明确标注"实现留 batch23"
- **输出**: `tests/evidence/d3-batch22/disk_full_design.md`
- **验收**: disk_full_design.md 存在 → 含技术设计 / 实现计划 / 与现有机制关系分析 / 正确性论证四章 → 实现计划标注留 batch23 → 与现有机制关系分析含 fsync/quorum/chaos_injector/WAL 四维
- **commit 点**: `git add tests/evidence/d3-batch22/disk_full_spec.md tests/evidence/d3-batch22/disk_full_design.md && git commit -m "T3.1+T3.2: 磁盘满方案 spec+design 两件套（实现留 batch23）"`（注：证据目录被 .gitignore 排除，实际 commit 仅含代码文件，方案文档落盘但不入库）
- **关联需求**: REQ-D-01, REQ-D-04, REQ-D-05

---

## 阶段 4：闭案 — 判定 + 报告 + 决策 + commit（预估 50min，并行 40min）

> **并行策略**：T4.1（判定脚本）先行；T4.2（报告.md）/ T4.3（decisions.md）均仅依赖 T4.1，可两任务并行；T4.4（commit + tag + bundle）依赖 T4.2+T4.3 全部完成。
> **阶段产物**：verdict.json + 报告.md（首屏三清单）+ decisions.md + commit D3-batch22-election-f3 + tag v2.4-post-batch22 + bundle

### T4.1 judge_batch22.py 判定脚本
- **ID**: T4.1
- **状态**: [ ] 未开始
- **预估**: 20 分钟
- **输入**:
  - T1.4 产出的 `tests/contracts/batch22.yaml`（E1-E3/S1-S2/F2/F4/F5 阈值）
  - T1.5 产出的 `tests/evidence/d3-batch22/election_opt_01~05.json`（E1-E3 证据）
  - T2.2 产出的 `tests/evidence/d3-batch22/entry_survival_01~03.json`（S1 证据）
  - T2.3 产出的 `tests/evidence/d3-batch22/endpoint_latency_01.json`（S2 证据）
  - `tests/contracts/judge_batch21.py`（复用 F2/F4/F5 判定逻辑）
  - design.md §2.2.2.5 judge_batch22.py 接口签名 + 判定逻辑表
  - spec.md §5.5 验收契约 + §6.1 YAML 数据约束
- **依赖**: T1.5, T2.2, T2.3（全部验收证据落盘）
- **并行**: 不可并行（T4.2/T4.3 的基础）
- **动作**:
  1. 创建 `tests/contracts/judge_batch22.py`
  2. 实现 `judge_e1(evidence_list, threshold)`：取选举优化场景 election_complete_time，3 次取中位，median ≤ 2.0s
  3. 实现 `judge_e2(evidence_list, max_leaders)`：max(concurrent_leaders) ≤ 1
  4. 实现 `judge_e3(evidence_list, reject_max)`：max(reject_rate_during_injection) ≤ 30%
  5. 实现 `judge_s1(evidence_list, expected_rate)`：min(survival_rate) == 100%，附 mismatched_entries 详情
  6. 实现 `judge_s2(evidence_list, fluctuation_max)`：max(p99_fluctuation) ≤ 5%
  7. 复用 judge_batch21.py 的 `judge_f2` / `judge_f4` / `judge_f5` 判定逻辑（继承 batch21）
  8. 实现 `judge_overall(e1~e3, s1~s2, f2/f4/f5)`：全 PASS → PASS；有 FAIL → FAIL；部分 PASS 无 FAIL → PARTIAL
  9. 逐字段对照证据 JSON vs YAML 阈值，输出 `tests/evidence/d3-batch22/verdict.json`（含 E1-E3/S1-S2/F2/F4/F5 判定 + overall + timestamp）
  10. CLI：`--contract` / `--evidence-dir` / `--output`
  11. 异常处理：YAML 格式错误 → 退出码 2；证据缺字段 → 标记 INSUFFICIENT_EVIDENCE，不阻止其余项
  12. 执行脚本产出 verdict.json：`python tests/contracts/judge_batch22.py --contract=tests/contracts/batch22.yaml --evidence-dir=tests/evidence/d3-batch22/ --output=tests/evidence/d3-batch22/verdict.json`
- **输出**: `tests/contracts/judge_batch22.py` + `tests/evidence/d3-batch22/verdict.json`
- **验收**: 脚本可读 YAML + JSON → 产出 verdict.json 含 E1-E3/S1-S2/F2/F4/F5 判定 + overall → 逐字段对照（红线 RL-04）→ 助手只引用判定文件不得自写 PASS/FAIL（红线 RL-09）
- **commit 点**: `git add tests/contracts/judge_batch22.py && git commit -m "T4.1: batch22 判定脚本 + verdict.json 产出"`
- **关联需求**: REQ-JUDGE, RL-04, RL-09

### T4.2 报告.md（首屏三清单 + 性能终审数据 + 判定结果）
- **ID**: T4.2
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**:
  - T0.3 已贴齐的性能终审数据（decomp 四构成项 + fsync 量纲）
  - T4.1 产出的 `verdict.json`（E1-E3/S1-S2/F2/F4/F5 判定 + overall）
  - LEDGER.md（L-21-1/L-21-2 清偿状态）
  - spec.md §1.3 核心输出 5（报告三件套：首屏三清单）+ §5.7 三等级处置
- **依赖**: T4.1
- **并行**: 可与 T4.3 并行
- **动作**:
  1. 在 `tests/evidence/d3-batch22/报告.md` 中构建首屏三清单：
     - **绕行清单**：已绕过的问题及原因（如 batch21 F1 选举超时配置绕行 → 本批调参修复）
     - **降级清单**：降级运行的功能及影响（如无）
     - **销账清单**：LEDGER 挂账清偿状态（DEBT-0001~0003 已清偿 + L-21-1/L-21-2 本批清偿）
  2. 首屏贴齐性能终审数据（T0.3 已完成，确认在首屏可见）：
     - decomp_c512 P99 分解表（四构成项 × {P50, P99, P50占比, P99占比}）
     - fsync 量纲表（窗口 180s / 总次数 4673 / 每秒 25.96 / 合并比 437:1）
  3. 写入判定结果章节：引用 verdict.json 内容（E1-E3/S1-S2/F2/F4/F5 判定 + overall），不自行判定（RL-09）
  4. 写入证据索引章节：列出所有证据 JSON 文件路径（election_forensics.json / election_opt_*.json / entry_survival_*.json / endpoint_latency_01.json / verdict.json）
  5. 根据 overall 判定执行三等级处置（spec §5.7）：
     - PASS → 闭案处置（签发闭案决议，T4.4 执行）
     - PARTIAL → 挂账处置（未通过项挂账至 batch23，记录根因初判，不闭案）
     - FAIL → 红线处置（触发红线告警，记录失败详情，须晨批审议）
- **输出**: `tests/evidence/d3-batch22/报告.md`（首屏三清单 + 性能终审 + 判定结果 + 证据索引）
- **验收**: 报告.md 首屏 → 三清单齐全（绕行/降级/销账）→ 性能终审数据在首屏可见 → 判定结果引用 verdict.json → 证据索引完整
- **commit 点**: 不独立 commit（与 T4.3 合并 commit）
- **关联需求**: REQ-DISP-PASS, REQ-DISP-PARTIAL, REQ-DISP-FAIL

### T4.3 decisions.md（决策落盘）
- **ID**: T4.3
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**:
  - T0.2 已落盘的构建产物根因分析
  - T1.1 产出的选举取证数据（指导调参 + pre-vote 决策）
  - T1.2/T1.3 的调参 + pre-vote 实现决策
  - T3.1/T3.2 的磁盘满方案设计决策
  - design.md §7 决策记录（7.1 调参 / 7.2 端点 / 7.3 pre-vote / 7.4 构建产物根治）
  - spec.md §4.4 可维护性 4（决策落盘）
- **依赖**: T4.1
- **并行**: 可与 T4.2 并行
- **动作**:
  1. 创建或更新 `tests/evidence/d3-batch22/decisions.md`
  2. 落盘**选举超时调参决策**（design.md §7.1）：
     - 决策：5000-7000ms → 800-1200ms
     - 理由：使选举完成 ≤ 2s（E1），> heartbeatIntervalMax(500ms)，区间宽度 400ms 提供随机化
     - 备选方案分析（方案 B 仅 pre-vote 不调参 → 不满足 E1；方案 C 调至 300-500ms → 误触发风险）
  3. 落盘**pre-vote 实现决策**（design.md §7.3）：
     - 决策：handleElectionTimeout 入口增加 pre-vote 探测
     - 理由：减少选票分裂，不增加 term，不引入活锁
  4. 落盘**/raft/entry 端点设计决策**（design.md §7.2）：
     - 决策：main.go 新增 GET handler，只读 + RLock
     - 理由：chaos_injector 已调用该端点，实现后 F3 重测自动生效
  5. 落盘**构建产物根治决策**（design.md §7.4，T0.2 已完成，确认在 decisions.md 中）
  6. 落盘**磁盘满方案设计决策**：
     - 决策：本批仅 spec + design 两件套，实现留 batch23
     - 理由：时间盒约束，方案设计先行降低 batch23 实现风险
  7. 落盘**batch20 修正未延续根因分析**（T0.2 已完成，确认在 decisions.md 中）
- **输出**: `tests/evidence/d3-batch22/decisions.md`（或项目根目录 decisions.md，视项目约定）
- **验收**: decisions.md 存在 → 含调参 / pre-vote / 端点 / 构建产物根治 / 磁盘满方案 / batch20 根因 六项决策 → 每项决策有理由 + 备选方案分析
- **commit 点**: `git add tests/evidence/d3-batch22/报告.md tests/evidence/d3-batch22/decisions.md && git commit -m "T4.2+T4.3: 报告.md（首屏三清单）+ decisions.md（决策落盘）"`（注：证据目录被 .gitignore 排除，实际 commit 仅含代码文件）
- **关联需求**: REQ-T0-04, REQ-E-02, REQ-E-04, REQ-S-01, REQ-D-01

### T4.4 commit + tag v2.4-post-batch22 + bundle
- **ID**: T4.4
- **状态**: [ ] 未开始
- **预估**: 5 分钟
- **输入**:
  - T4.1 产出的 verdict.json（overall 判定）
  - T4.2 产出的报告.md
  - T4.3 产出的 decisions.md
  - 所有阶段产物（LEDGER.md / .gitignore / raft.go / main.go / chaos_injector 扩展 / batch22.yaml / judge_batch22.py）
  - spec.md §5.7.1 三等级处置协议
- **依赖**: T4.2, T4.3
- **并行**: 不可并行（闭案收尾）
- **动作**:
  1. 根据 verdict.json 的 overall 判定执行对应处置：
     - **overall=PASS** → 闭案处置：
       - 签发闭案决议（在报告.md 中标注"闭案决议已签发"）
       - `git add -A`（仅代码文件，证据目录被 .gitignore 排除 RL-08）
       - `git commit -m "D3-batch22-election-f3: 战役I修复 + 战役II预演闭案"`
       - `git tag v2.4-post-batch22`
       - bundle 备份：`git bundle create ../bundle/v2.4-post-batch22.bundle --all`
     - **overall=PARTIAL** → 挂账处置：
       - 将未通过项挂账至 batch23（在 LEDGER.md 中新增 L-22-X 待清记录）
       - 记录根因初判（在 decisions.md 中）
       - 不闭案，不打 tag
       - `git commit -m "D3-batch22-partial: 部分验收未通过，挂账 batch23"`
     - **overall=FAIL** → 红线处置：
       - 触发红线告警（在报告.md 中标注"红线告警"）
       - 记录失败详情（在 decisions.md 中）
       - 不闭案，不打 tag，须晨批审议
       - `git commit -m "D3-batch22-fail: 验收 FAIL，须晨批审议"`
  2. 更新 RESUME.md：标注所有任务已完成，session 结束
- **输出**: commit `D3-batch22-election-f3` + tag `v2.4-post-batch22` + bundle 备份（若 PASS）
- **验收**: overall=PASS → commit + tag + bundle 完成 → RESUME.md 标注 session 结束 → git log 可见 commit → git tag 可见 v2.4-post-batch22
- **commit 点**: 本任务即 commit 点
- **关联需求**: REQ-DISP-PASS, REQ-DISP-PARTIAL, REQ-DISP-FAIL, REQ-DISP-TIMEBOX, RL-08

---

## 阶段 5：收尾 — 红线自查 + 集群恢复（预估 15min，并行 15min）

> **并行策略**：T5.1（红线自查）/ T5.2（集群恢复）互不依赖，可两任务并行。
> **阶段产物**：红线自查结论 + 集群健康稳态

### T5.1 红线自查
- **ID**: T5.1
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**:
  - spec.md §8 红线清单（RL-01~RL-11）
  - 所有阶段产物
  - verdict.json
- **依赖**: T4.4
- **并行**: 可与 T5.2 并行
- **动作**:
  1. **RL-01 fsync 语义**：确认 raft.go 选举优化未触碰 WAL 写入路径 / fsync 调用 → ✅
  2. **RL-02 quorum 语义**：确认 quorum 计算（3/5）未修改 → ✅
  3. **RL-03 选举超时语义**：确认调参后仍为随机化区间（800-1200ms）、Raft 安全性保持、> heartbeatInterval → ✅
  4. **RL-04 PASS 判定逐字段对照**：确认 judge_batch22.py 逐字段比对证据 vs YAML → ✅
  5. **RL-05 改 YAML 须晨批**：确认 batch22.yaml 阈值未自行放宽 → ✅
  6. **RL-06 挂账到期未清**：确认 LEDGER.md 中 L-21-1/L-21-2 已清偿 → ✅
  7. **RL-07 可观测性代码不进写路径热区**：确认 /raft/entry handler 独立注册，不在 /raft/propose 内调用 → ✅
  8. **RL-08 证据目录被 .gitignore 排除**：确认 `git status` 不显示证据目录文件 → ✅
  9. **RL-09 助手不得自写 PASS/FAIL**：确认报告.md 中判定结果引用 verdict.json，未自写 → ✅
  10. **RL-10 时间盒 ≤ 6 小时**：确认实际耗时 ≤ 6h（并行后 3.91h）→ ✅
  11. **RL-11 构建产物禁止入库**：确认 `git status` 干净（无 .exe / build/ 相关条目）→ ✅
  12. 落盘红线自查结论至 `tests/evidence/d3-batch22/redline_check.json`
- **输出**: `tests/evidence/d3-batch22/redline_check.json`（11 条红线自查结论）
- **验收**: 11 条红线全部 ✅ → 自查结论落盘
- **commit 点**: 不独立 commit（自查结论落盘证据目录，被 .gitignore 排除）
- **关联需求**: RL-01~RL-11

### T5.2 集群恢复
- **ID**: T5.2
- **状态**: [ ] 未开始
- **预估**: 5 分钟
- **输入**:
  - 5 节点 Docker 集群（可能因测试期间 kill leader 处于非稳态）
  - `cmd/chaos_injector/node_ctl.go`（RestartNode / WaitNodeHealthy 方法）
- **依赖**: T4.4
- **并行**: 可与 T5.1 并行
- **动作**:
  1. 检查 5 节点集群状态：遍历 `/raft/stats` 确认每节点 state
  2. 若有节点被 kill 未恢复：`docker start raft-node-N` 重启
  3. 等待所有节点健康：轮询 `/health/live` 返回 200
  4. 确认集群恢复到健康稳态：1 leader + 4 follower
  5. 记录集群最终状态至 `tests/evidence/d3-batch22/cluster_final_state.json`
- **输出**: `tests/evidence/d3-batch22/cluster_final_state.json`（集群最终状态）
- **验收**: 5 节点全部健康 → 1 leader + 4 follower → 集群可正常读写
- **commit 点**: 不独立 commit（最终状态落盘证据目录）
- **关联需求**: spec §4.2 可靠性 3（集群可恢复）

---

## 需求追溯矩阵

### 任务 → 需求 ID 映射

| 任务 ID | 关联需求 ID | 需求描述 | EARS 类型 | 验收线 |
|---------|------------|----------|-----------|--------|
| T0.1 | REQ-T0-01 | LEDGER 逐条销账 | Ubiquitous | DEBT-0001~0003 确认 + L-21-1/L-21-2 入账 |
| T0.1 | REQ-T0-02 | L-21-1 入账与清偿（F1 取证） | Event-driven | L-21-1 入账 → 取证后清偿 |
| T0.1 | REQ-T0-03 | L-21-2 入账与清偿（F3 端点） | Event-driven | L-21-2 入账 → 端点实现后清偿 |
| T0.2 | REQ-T0-04 | 构建产物入库根治 | Ubiquitous | .gitignore 补丁 + chaos_injector.exe 移出 + 根因分析 |
| T0.2 | RL-08 | 证据目录被 .gitignore 排除 | 红线 | git status 干净 |
| T0.2 | RL-11 | 构建产物禁止入库 | 红线（新增） | *.exe 通配符 + build/ 排除 |
| T0.3 | REQ-T0-05 | 性能终审数据贴齐 | Ubiquitous | 报告.md 首屏贴 decomp + fsync |
| T0.3 | REQ-T0-06 | decomp_c512 P99 分解数字直贴 | Ubiquitous | 四构成项 × {P50, P99, 占比} 齐全 |
| T0.3 | REQ-T0-07 | fsync 量纲数字直贴 | Ubiquitous | 窗口 180s / 总次数 4673 / 每秒 25.96 / 合并比 437:1 |
| T1.1 | REQ-E-01 | 选举取证先行 | Event-driven | election_forensics.json 落盘 |
| T1.1 | REQ-T0-02 | L-21-1 清偿（取证完成） | Event-driven | L-21-1 cleared_at 回填 |
| T1.2 | REQ-E-02 | 依据取证数据优化 | State-driven | 调参后选举完成 ≤ 2s |
| T1.2 | REQ-E-03 | 调参不破坏选举超时语义 | State-driven | 随机化区间 + Raft 安全性保持 |
| T1.2 | RL-03 | 选举超时语义不可破坏 | 红线 | 调参允许但语义不可破坏 |
| T1.3 | REQ-E-04 | pre-vote 实现 | Optional | pre-vote 探测 → 选票分裂减少 |
| T1.3 | RL-03 | 选举超时语义不可破坏 | 红线 | pre-vote 不破坏 Raft 安全性 |
| T1.4 | REQ-E1 | 选举完成时间 ≤ 2s | Event-driven | E1 阈值 2.0s |
| T1.4 | REQ-E2 | 优化不引入脑裂风险 | State-driven | E2 阈值 max_leaders ≤ 1 |
| T1.4 | REQ-E3 | 选举期间拒绝率不劣化 | State-driven | E3 阈值 ≤ 30% |
| T1.4 | REQ-S1 | 已确认写入存活率 = 100% | Event-driven | S1 阈值 100% |
| T1.4 | REQ-S2 | 端点不引入写路径延迟劣化 | State-driven | S2 阈值 P99 波动 ≤ 5% |
| T1.4 | REQ-YAML | YAML 变更须晨批 | Unwanted | batch22.yaml 阈值未自行放宽 |
| T1.5 | REQ-E-05 | 压测负载持续下测量 | State-driven | c=128 持续 + 3 次取中位 |
| T1.5 | REQ-E1 | E1 验收 | Event-driven | 选举完成中位 ≤ 2s |
| T1.5 | REQ-E2 | E2 验收 | State-driven | 复跑 F4 全绿 |
| T1.5 | REQ-E3 | E3 验收 | State-driven | 拒绝率 ≤ 30% |
| T2.1 | REQ-S-01 | 实现 GET /raft/entry 端点 | Ubiquitous | 端点可调用 + 返回 entry 列表 |
| T2.1 | REQ-S-02 | 端点只读不侵入写路径 | State-driven | 只读 + RLock + 零侵入 |
| T2.1 | REQ-S-05 | 端点三件套完整 | Ubiquitous | 代码 + 文档 + 测试 |
| T2.1 | REQ-T0-03 | L-21-2 清偿（端点实现） | Event-driven | L-21-2 cleared_at 回填 |
| T2.1 | RL-07 | 可观测性代码不进写路径热区 | 红线 | handler 独立注册 |
| T2.2 | REQ-S-03 | 重跑 F3 全场景 | Event-driven | 杀 leader 前采样 → 新 leader 比对 |
| T2.2 | REQ-S-04 | 抽样比对验证 | Event-driven | index ≤ commit_index + 内容一致 |
| T2.2 | REQ-S1 | S1 验收 | Event-driven | 存活率 100% |
| T2.3 | REQ-S-02 | 端点只读不侵入写路径 | State-driven | P99 波动 ≤ 5% |
| T2.3 | REQ-S2 | S2 验收 | State-driven | P99 波动 ≤ 5% |
| T2.3 | RL-07 | 可观测性代码不进写路径热区 | 红线 | 端点零侵入 |
| T3.1 | REQ-D-01 | 磁盘满方案设计 | Ubiquitous | spec + design 两件套 |
| T3.1 | REQ-D-02 | 注入方式设计 | Ubiquitous | ≥ 1 种可执行方法 |
| T3.1 | REQ-D-03 | 预期行为设计 | Ubiquitous | 六维预期齐全 |
| T3.1 | REQ-D-04 | 验收指标草案 | Ubiquitous | ≥ 4 条可量化指标 |
| T3.1 | REQ-D-05 | 实现留 batch23 | Optional | 明确标注留 batch23 |
| T3.2 | REQ-D-01 | 磁盘满方案设计 | Ubiquitous | 技术设计 + 实现计划 |
| T3.2 | REQ-D-04 | 验收指标草案 | Ubiquitous | 草案待 batch23 细化 |
| T3.2 | REQ-D-05 | 实现留 batch23 | Optional | 实现计划标注留 batch23 |
| T4.1 | REQ-JUDGE | 判定由脚本产出 | Ubiquitous | judge_batch22.py 产出 verdict.json |
| T4.1 | RL-04 | PASS 判定逐字段对照 | 红线 | 逐字段比对证据 vs YAML |
| T4.1 | RL-09 | 助手不得自写 PASS/FAIL | 红线 | 助手只引用 verdict.json |
| T4.2 | REQ-DISP-PASS | PASS 等级处置 | Event-driven | 闭案 + tag + commit + bundle |
| T4.2 | REQ-DISP-PARTIAL | PARTIAL 等级处置 | Event-driven | 挂账 batch23 + 不闭案 |
| T4.2 | REQ-DISP-FAIL | FAIL 等级处置 | Event-driven | 红线告警 + 须晨批审议 |
| T4.3 | REQ-T0-04 | 构建产物根治决策落盘 | Ubiquitous | decisions.md 根因分析 |
| T4.3 | REQ-E-02 | 调参决策落盘 | State-driven | decisions.md 调参理由 |
| T4.3 | REQ-E-04 | pre-vote 决策落盘 | Optional | decisions.md pre-vote 理由 |
| T4.3 | REQ-S-01 | 端点设计决策落盘 | Ubiquitous | decisions.md 端点理由 |
| T4.3 | REQ-D-01 | 磁盘满方案决策落盘 | Ubiquitous | decisions.md 方案理由 |
| T4.4 | REQ-DISP-PASS | PASS 闭案处置 | Event-driven | commit + tag + bundle |
| T4.4 | REQ-DISP-PARTIAL | PARTIAL 挂账处置 | Event-driven | 挂账 batch23 |
| T4.4 | REQ-DISP-FAIL | FAIL 红线处置 | Event-driven | 红线告警 |
| T4.4 | REQ-DISP-TIMEBOX | 时间盒到期处置 | Event-driven | 到点停手交数据 |
| T4.4 | RL-08 | 证据目录被 .gitignore 排除 | 红线 | git add 仅代码文件 |
| T5.1 | RL-01~RL-11 | 红线自查 | 红线 | 11 条红线全 ✅ |
| T5.2 | spec §4.2.3 | 集群可恢复 | 可靠性 | 1 leader + 4 follower |

### 需求覆盖统计

| 需求类别 | 需求数 | 覆盖任务 | 覆盖率 |
|---------|--------|---------|--------|
| 任务零（REQ-T0-01~07） | 7 | T0.1/T0.2/T0.3/T1.1/T2.1 | 100% |
| 选举优化（REQ-E-01~05） | 5 | T1.1/T1.2/T1.3/T1.5 | 100% |
| 观测端点（REQ-S-01~05） | 5 | T2.1/T2.2/T2.3 | 100% |
| 磁盘满方案（REQ-D-01~05） | 5 | T3.1/T3.2 | 100% |
| 验收契约（REQ-E1~S2） | 5 | T1.4/T1.5/T2.2/T2.3 | 100% |
| 判定约束（REQ-JUDGE/YAML） | 2 | T1.4/T4.1 | 100% |
| 断点续跑（REQ-RESUME-01~03） | 3 | T4.4（RESUME.md 更新） | 100% |
| 三等级处置（REQ-DISP-*） | 5 | T4.2/T4.4 | 100% |
| 红线（RL-01~RL-11） | 11 | T0.2/T1.2/T1.3/T2.1/T2.3/T4.1/T4.4/T5.1 | 100% |
| **合计** | **48** | **19 任务** | **100%** |

---

## 任务依赖图

```
阶段 0（并行）          阶段 1（并行）           阶段 2（并行）         阶段 3（并行）
┌─ T0.1 ─┐            ┌─ T1.1 ─┐              ┌─ T2.1 ─┐            ┌─ T3.1 ─┐
├─ T0.2 ─┤            ├─ T1.2 ─┤              ├─ T2.2 ─┤            └─ T3.2 ─┘
└─ T0.3 ─┘            ├─ T1.3 ─┤              └─ T2.3 ─┘
                       ├─ T1.4 ─┤
                       └─ T1.5 ─┘
                           │
                           ▼
                    ┌─ T4.1 ─┐
                    ├─ T4.2 ─┤
                    ├─ T4.3 ─┤
                    └─ T4.4 ─┘
                           │
                           ▼
                    ┌─ T5.1 ─┐
                    └─ T5.2 ─┘
```

**依赖关系**：
- T0.1/T0.2/T0.3 → 无依赖，可三任务并行
- T1.1 → 依赖阶段 0 完成
- T1.2/T1.3/T1.4 → 依赖 T1.1，可三任务并行
- T1.5 → 依赖 T1.2+T1.3+T1.4
- T2.1 → 依赖阶段 0 完成（与 T1.x 可并行）
- T2.2/T2.3 → 依赖 T2.1，可两任务并行
- T3.1/T3.2 → 无依赖（与 T1.x/T2.x 可并行）
- T4.1 → 依赖 T1.5+T2.2+T2.3（全部验收证据落盘）
- T4.2/T4.3 → 依赖 T4.1，可两任务并行
- T4.4 → 依赖 T4.2+T4.3
- T5.1/T5.2 → 依赖 T4.4，可两任务并行

---

## 时间盒汇总

| 阶段 | 任务数 | 串行预估 | 并行预估 | 关键产物 |
|------|--------|----------|----------|----------|
| 阶段 0 任务零前置 | 3 | 1.5h | 1.0h | LEDGER + .gitignore + 性能首屏 |
| 阶段 1 选举优化 | 5 | 2.0h | 2.0h | election_forensics.json + 调参 + pre-vote + E1-E3 |
| 阶段 2 观测端点 | 3 | 1.5h | 1.5h | /raft/entry + F3 重测 + S1-S2 |
| 阶段 3 战役II预演 | 2 | 1.0h | 0.5h | disk_full spec + design |
| 阶段 4 闭案 | 4 | 0.83h | 0.66h | verdict.json + 报告.md + decisions.md + commit/tag |
| 阶段 5 收尾 | 2 | 0.25h | 0.25h | 红线自查 + 集群恢复 |
| **合计** | **19** | **7.08h** | **3.91h** | **≤ 6h ✅** |

> **并行后总预估：3.91h ≤ 6h，满足时间盒约束（红线 RL-10）**。预留 2.09h 作为集群部署/编译/重测的缓冲时间。