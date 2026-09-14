# batch21 故障注入战役 I — 实施任务分解

> 版本: v2.4-batch21-chaos1
> 关联规格: spec.md / design.md
> 时间盒: 实现+执行 ≤ 6 小时（红线 RL-10）
> 命名约定: snake_case（函数/变量/脚本），kebab-case（目录）
> 前置批次: batch20（fsync 合并 437:1 已证实）
> 任务粒度: 每个任务 ≤ 30 分钟，完成后可独立 commit
> 集群编排: docker compose -p deploy5 -f tests/deploy/docker-compose-5node.yml -f tests/deploy/docker-compose-5node-ports.yml -f tests/deploy/docker-compose-5node-batch16.yml --env-file tests/deploy/deploy.env

---

## 阶段 0：任务零 — 前置清账（预估 1.5 小时）

> **并行策略**：T0.1 / T0.2 / T0.3 / T0.5 互不依赖，可四任务并行。T0.4 依赖 T0.3。
> **阶段产物**：LEDGER.md + fsync 量纲标注 + decomp_c512_raw.json + .gitignore 补丁

### T0.1 LEDGER.md 建账
- **ID**: T0.1
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**: batch18/batch19/batch20 已清记录（来自历史 commit 和 evidence 目录）
- **依赖**: 无
- **并行**: 可与 T0.2/T0.3/T0.5 并行
- **动作**:
  1. 在项目根目录创建 `LEDGER.md`
  2. 迁移三笔已清记录（DEBT-0001 延迟分解埋点 / DEBT-0002 quorum_wait P99 分解 / DEBT-0003 decomp 补全），标注 batch20 已清偿
  3. 新增 batch21 待清项（DEBT-0003 decomp_c512_raw.json 四构成项补全）
  4. 格式：`[欠账ID/来源批次/应清批次/状态]` 表格，含 debt_id/source_batch/target_batch/status/description/cleared_at 六字段
- **输出**: `LEDGER.md`
- **验收**: 格式合规，≥3 笔已清记录，batch20 相关记录状态为"已清偿"，每笔有唯一 ID
- **commit 点**: `git add LEDGER.md && git commit -m "T0.1: LEDGER.md 建账"`
- **关联需求**: REQ-T0-01

### T0.2 fsync 取证量纲澄清
- **ID**: T0.2
- **状态**: [ ] 未开始
- **预估**: 20 分钟
- **输入**: `tests/evidence/d3-batch20/fsync_forensics.json`（fsync_count=4673, fsync_per_sec=25.96, merge_ratio=437:1, test_config.duration=180s）
- **依赖**: 无
- **并行**: 可与 T0.1/T0.3/T0.5 并行
- **动作**:
  1. 从 `fsync_forensics.json` 提取：压测窗口时长=180s（`test_config.duration`）、窗口内 fsync 总次数=4673（`leader.fsync_count`）、每秒 fsync=25.96（`leader.fsync_per_sec`）
  2. 与"个位数~几十次/窗口"预设标准对齐：26 fsync/s 属"几十次/秒"范畴
  3. 在 `docs/specs/latency_decomp/` 下创建 `fsync_dimensional_clarification.md`，写入量纲标注段落
  4. 标注含 7 字段：窗口时长、fsync 总次数、每秒 fsync、预设标准、对齐结论、合并比、判定
- **输出**: `docs/specs/latency_decomp/fsync_dimensional_clarification.md`
- **验收**: 窗口时长、fsync 总次数、对齐结论三者齐全且数值可验证，7 字段完整
- **commit 点**: `git add docs/specs/latency_decomp/fsync_dimensional_clarification.md && git commit -m "T0.2: fsync 量纲澄清"`
- **关联需求**: REQ-T0-02

### T0.3 decomp_c512_raw.json 补全
- **ID**: T0.3
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**: batch18 埋点实测数据（c=512, 69982 样本）+ `/latency/decomp` 端点（main.go:526）+ 现有 `docs/specs/latency_decomp/decomp_c512_raw.json`（仅顶层聚合）
- **依赖**: 无（数据来源优先用 batch18 埋点实测；若不可查则需集群运行采集）
- **并行**: 可与 T0.1/T0.2/T0.5 并行
- **动作**:
  1. 检查 batch18 埋点原始数据是否可查（`tests/evidence/d3-batch18/` 下 decomp 相关文件）
  2. 若可查：整理 quorum_wait / fsync_wait / rpc / queue 四构成项的 P50 / P99 及占比
  3. 若不可查：启动集群，c=512 压测期间调用 `/latency/decomp` 端点采集分解数据
  4. 写入 `docs/specs/latency_decomp/decomp_c512_raw.json`，保留现有顶层聚合字段，新增 metadata + 四构成项 + total 字段
  5. 确认文件为 KB 级完整数据（≥1KB），四构成项 × {p50, p99, p50_ratio, p99_ratio} 全字段
  6. 数据来源标注：metadata.source 字段注明 "batch18埋点实测" 或 "/latency/decomp端点重新采集"
- **输出**: `docs/specs/latency_decomp/decomp_c512_raw.json`（补全后）
- **验收**: 四构成项 × {P50, P99, P50占比, P99占比} 全字段，≥1KB，数据可直接决定性能章节是否正式闭案
- **commit 点**: `git add docs/specs/latency_decomp/decomp_c512_raw.json && git commit -m "T0.3: decomp_c512_raw.json 四构成项补全"`
- **关联挂账**: DEBT-0003 → 清偿
- **关联需求**: REQ-T0-03

### T0.4 decomp 数据随晨报首屏上报
- **ID**: T0.4
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**: T0.3 产出的 `decomp_c512_raw.json`
- **依赖**: T0.3
- **并行**: 不可并行（依赖 T0.3 输出）
- **动作**:
  1. 定位晨报首屏模板（`docs/morning_report/` 或等效目录）
  2. 将 decomp 数据摘要加入晨报首屏模板（quorum_wait P99 占比最高项突出显示）
  3. 摘要含：四构成项 P99 值 + 占比 + 数据来源
- **输出**: 晨报首屏模板更新
- **验收**: 晨报首屏可见 decomp 数据摘要，quorum_wait P99 占比突出
- **commit 点**: `git add docs/morning_report/ && git commit -m "T0.4: decomp 数据随晨报首屏上报"`
- **关联需求**: REQ-T0-04

### T0.5 loadgen.exe 积出仓库 + .gitignore 补丁
- **ID**: T0.5
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**: 现有 `tools/loadgen/loadgen.exe` + 现有 `.gitignore`
- **依赖**: 无
- **并行**: 可与 T0.1/T0.2/T0.3 并行
- **动作**:
  1. 将 `tools/loadgen/loadgen.exe` 移至仓库外目录（如 `../external/tools/loadgen.exe`）
  2. 补丁 `.gitignore`，添加排除规则：
     - `loadgen.exe` / `cluster_loadtest.exe` / `cluster_client.exe`
     - `chaos_injector_arm64`（后续 T2.7 编译产物）
     - `tests/evidence/d3-batch21/`（证据目录，红线 RL-08）
  3. 确认 `git status` 不显示这些文件为未跟踪
- **输出**: 仓库中无 loadgen.exe，`.gitignore` 已补丁
- **验收**: `git status` 干净（无 loadgen.exe / cluster_*.exe / chaos_injector_arm64 相关条目），证据目录被排除
- **commit 点**: `git add .gitignore && git commit -m "T0.5: loadgen.exe 移出 + .gitignore 补丁"`
- **关联需求**: REQ-T0-05, RL-08

---

## 阶段 1：验收契约与判定脚本落盘（预估 50 分钟）

> **并行策略**：T1.1 与阶段 0 互不依赖，可并行启动。T1.2 依赖 T1.1。
> **阶段产物**：batch21.yaml + judge_batch21.py

### T1.1 编写 batch21.yaml 验收契约
- **ID**: T1.1
- **状态**: [ ] 未开始
- **预估**: 20 分钟
- **输入**: spec.md §5.3 F1-F5 定义 + §6.1 YAML 数据约束 + §8 红线清单
- **依赖**: 无
- **并行**: 可与阶段 0 全部任务并行
- **动作**:
  1. 创建 `tests/contracts/batch21.yaml`
  2. 定义 F1-F5 五条验收线及阈值：
     - F1: election_completion_time_max = 5.0s（median_of_3，operator: <=）
     - F2: during_injection reject_rate ≤ 30%，after_recovery reject_rate == 0
     - F3: confirmed_write_survival_rate == 100%（sampling_comparison）
     - F4: max_concurrent_leaders ≤ 1（any_instant）
     - F5: log_complete = true, replayable = true
  3. 定义场景矩阵（稳态×10 + 压测中×10 + 连杀×3 = 23 场景）
  4. 定义红线清单（RL-01~RL-10）和时间盒（6h，on_expire: stop_and_hand_over）
  5. 格式与既有 batch 契约 YAML 一致（spec 4.5 兼容性）
- **输出**: `tests/contracts/batch21.yaml`
- **验收**: YAML 格式合法（`python -c "import yaml; yaml.safe_load(open('tests/contracts/batch21.yaml'))"` 通过），F1-F5 阈值齐全，场景矩阵 23 场景，红线 10 条
- **commit 点**: `git add tests/contracts/batch21.yaml && git commit -m "T1.1: batch21 验收契约 YAML"`
- **关联需求**: REQ-F1 ~ REQ-F5, REQ-YAML

### T1.2 编写判定脚本 judge_batch21.py
- **ID**: T1.2
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**: T1.1 产出的 `batch21.yaml` + spec.md §6.2 场景证据 JSON 结构 + design.md §2.2.2.6 判定伪代码
- **依赖**: T1.1
- **并行**: 不可并行（依赖 T1.1 输出）
- **动作**:
  1. 创建 `tests/contracts/judge_batch21.py`
  2. 实现 `judge_f1(evidence_list, threshold)`：取稳态场景 election completion_duration，3 次取中位，max(medians) ≤ 5s
  3. 实现 `judge_f2(evidence_list, during_max, after)`：注入期间 max(reject_rate) ≤ 30%，恢复后 all(post_recovery_reject_rate == 0)
  4. 实现 `judge_f3(evidence_list, expected_rate)`：min(survival_rate) == 100%，附 mismatched_entries 详情
  5. 实现 `judge_f4(evidence_list, max_leaders)`：max(max_concurrent_leaders) ≤ 1
  6. 实现 `judge_f5(evidence_list)`：all(timeline_complete) and all(replayable)
  7. 实现 `judge_overall(f1~f5)`：全 PASS → PASS；有 FAIL → FAIL；部分 PASS 无 FAIL → PARTIAL
  8. 逐字段对照证据 JSON vs YAML 阈值，输出 `verdict.json`（含 F1-F5 判定 + overall + timestamp）
  9. CLI：`--contract` / `--evidence-dir` / `--output`
  10. 异常处理：YAML 格式错误 → 退出码 2；证据缺字段 → 标记 INSUFFICIENT_EVIDENCE，不阻止其余项
- **输出**: `tests/contracts/judge_batch21.py`
- **验收**: 脚本可读 YAML + JSON，产出 verdict.json 含 F1-F5 判定 + overall，逐字段对照（红线 RL-04）
- **commit 点**: `git add tests/contracts/judge_batch21.py && git commit -m "T1.2: batch21 判定脚本"`
- **关联需求**: REQ-JUDGE, RL-04, RL-09

---

## 阶段 2：故障注入工具实现（预估 2.5 小时）

> **并行策略**：T2.1 先行；T2.2a/T2.2b/T2.4/T2.5 均仅依赖 T2.1，可四任务并行；T2.3a/T2.3b 依赖 T2.2a+T2.2b；T2.6 依赖 T2.2~T2.5 全部。
> **阶段产物**：chaos_injector_arm64 二进制

### T2.1 搭建 chaos_injector 骨架（Go）
- **ID**: T2.1
- **状态**: [ ] 未开始
- **预估**: 25 分钟
- **输入**: design.md §2.1.2 模块划分 + §2.2.2.1 CLI 接口 + §2.3.2 数据模型
- **依赖**: 无
- **并行**: 不可并行（后续任务的基础）
- **动作**:
  1. 创建 `cmd/chaos_injector/main.go`
  2. 实现 CLI 参数解析（`--contract` / `--evidence-dir` / `--cluster-config` / `--full-rerun` / `--scenario-type steady|under_load|cascading|all` / `--timebox 6h`）
  3. 搭建模块文件骨架：scheduler.go / node_ctl.go / collector.go / evidence.go / resume_mgr.go
  4. 定义核心数据结构（design §2.3.2）：
     - ScenarioResult（含 10 字段：scenario_id / scenario_type / timeline / election_metrics / reject_metrics / survival_metrics / split_brain_metrics / log_metrics / status / evidence_path）
     - TimelineEvent / ElectionMetrics / RejectMetrics / SurvivalMetrics / SplitBrainMetrics / LogMetrics / EntryMismatch / EntrySnapshot / ResumeState / Verdict / FVerdict
  5. main.go 中实现骨架流程：解析 CLI → 加载 RESUME.md → 调度场景 → 落盘证据
- **输出**: `cmd/chaos_injector/main.go` + 5 个模块文件骨架 + 结构体定义
- **验收**: `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /dev/null cmd/chaos_injector/main.go` 通过（空实现 + 结构定义）
- **commit 点**: `git add cmd/chaos_injector/ && git commit -m "T2.1: chaos_injector 骨架 + 数据结构"`
- **关联需求**: REQ-FI-06

### T2.2a 实现 node_ctl Docker 容器操作
- **ID**: T2.2a
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**: T2.1 骨架 + design.md §2.2.2.2 NodeController 接口
- **依赖**: T2.1
- **并行**: 可与 T2.2b/T2.4/T2.5 并行
- **动作**:
  1. 在 `cmd/chaos_injector/node_ctl.go` 中实现 `NodeController` 结构体（containerPrefix="raft-node-", httpPortBase=9001）
  2. 实现 `KillNode(nodeID)`：`docker kill --signal=9 raft-node-N`，确认进程在 ≤1s 内终止
  3. 实现 `RestartNode(nodeID)`：`docker start raft-node-N`，WAL 卷保留
  4. 实现 `WaitNodeHealthy(nodeID, timeout=30s)`：轮询 `http://localhost:900{N}/health/live` 返回 200，最长 30s
  5. Docker CLI 调用通过 `os/exec`，捕获 stderr 输出
- **输出**: `cmd/chaos_injector/node_ctl.go`（Docker 操作部分）
- **验收**: 可 kill/重启节点，WaitNodeHealthy 正确超时
- **commit 点**: 不独立 commit（与 T2.2b 合并 commit）
- **关联需求**: REQ-FI-01, REQ-FI-02

### T2.2b 实现 node_ctl HTTP 查询与解析
- **ID**: T2.2b
- **状态**: [ ] 未开始
- **预估**: 20 分钟
- **输入**: T2.1 骨架 + design.md §2.1.2 配置项 + §2.2.2.2 NodeController 接口
- **依赖**: T2.1
- **并行**: 可与 T2.2a/T2.4/T2.5 并行
- **动作**:
  1. 实现 `QueryLeader()`：遍历 5 节点 HTTP GET `http://localhost:900{N}/raft/status`（JSON），返回 state=Leader 的 nodeID（动态识别，非预设，REQ-FI-07）
  2. 实现 `GetFollowers()`：遍历 5 节点 `/raft/status`，返回 state=Follower 的节点列表
  3. 实现 `GetNodeStats(nodeID)`：HTTP GET `/raft/status`，返回结构化 RaftStats
  4. 实现 `parseRaftStatsText()`：fallback 文本解析器（解析 `/raft/stats` 文本格式 `id=... state=... leader=...`），按 `key=value` 空格分隔
  5. HTTP 请求超时 5s，调用方重试 3 次间隔 1s
- **输出**: `cmd/chaos_injector/node_ctl.go`（HTTP 查询部分）
- **验收**: 可查询 leader（动态识别），parseRaftStatsText 正确解析文本格式
- **commit 点**: `git add cmd/chaos_injector/node_ctl.go && git commit -m "T2.2: node_ctl 节点控制模块（Docker + HTTP）"`
- **关联需求**: REQ-FI-07

### T2.3a 实现 collector 选举时序与拒载率采集
- **ID**: T2.3a
- **状态**: [ ] 未开始
- **预估**: 20 分钟
- **输入**: T2.2a+T2.2b 产出的 node_ctl + design.md §2.2.2.3 Collector 接口
- **依赖**: T2.2a, T2.2b
- **并行**: 可与 T2.3b 并行
- **动作**:
  1. 在 `cmd/chaos_injector/collector.go` 中实现 `Collector` 结构体（持有 NodeController 引用）
  2. 实现 `CollectElectionTimeline(killTS, timeout=10s)`：100ms 轮询集群 leader，记录 T_election_complete，计算 completion_duration = T_election - T_kill
  3. 实现 `CollectRejectRate(loadgenStatsPath, duringStart, duringEnd)`：读 loadgen 统计文件，计算注入期间 reject_rate 和恢复后 post_recovery_reject_rate
  4. 选举超时处理：返回 timeout error，含已采集的部分时间线
- **输出**: `cmd/chaos_injector/collector.go`（选举 + 拒载部分）
- **验收**: 选举时序可采集（100ms 轮询），拒载率可从 loadgen 统计文件计算
- **commit 点**: 不独立 commit（与 T2.3b 合并 commit）
- **关联需求**: REQ-F1, REQ-F2

### T2.3b 实现 collector 存活率/脑裂/日志采集
- **ID**: T2.3b
- **状态**: [ ] 未开始
- **预估**: 20 分钟
- **输入**: T2.2a+T2.2b 产出的 node_ctl + design.md §2.1.3.4 F3 事务设计 + §2.2.2.3 Collector 接口
- **依赖**: T2.2a, T2.2b
- **并行**: 可与 T2.3a 并行
- **动作**:
  1. 实现 `SnapshotConfirmedEntries(leaderID, sampleCount=20)`：注入前查询 leader 的 commit_index，抽样记录 commit index 附近的 N 个 entry 内容（entry_index, entry_term, entry_value）
  2. 实现 `VerifyEntrySurvival(newLeaderID, snapshots)`：注入后查询新 leader 的 commit_index，比对抽样 entry 是否存活（entry_index ≤ post_kill_commit_idx 且内容一致），返回 SurvivalMetrics
  3. 实现 `DetectSplitBrain(duration=5s)`：50ms 轮询所有节点 leader 状态，记录 max_concurrent_leaders 和 detection_timestamps
  4. 实现 `CollectStructuredLog(startTS, endTS)`：采集集群日志流，验证时间线完整（单调递增无缺口）且可回放
- **输出**: `cmd/chaos_injector/collector.go`（存活 + 脑裂 + 日志部分）
- **验收**: 五类指标均可采集，时间线单调递增无缺口，F3 抽样比对正确
- **commit 点**: `git add cmd/chaos_injector/collector.go && git commit -m "T2.3: collector 指标采集模块（五类指标）"`
- **关联需求**: REQ-F3, REQ-F4, REQ-F5

### T2.4 实现 evidence 证据落盘模块
- **ID**: T2.4
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**: T2.1 骨架 + spec.md §6.2 场景证据 JSON 结构（10 字段）
- **依赖**: T2.1
- **并行**: 可与 T2.2a/T2.2b/T2.3a/T2.3b/T2.5 并行
- **动作**:
  1. 在 `cmd/chaos_injector/evidence.go` 中实现 `EvidenceManager` 结构体（evidenceDir 字段）
  2. 实现 `PersistScenario(result ScenarioResult)`：序列化 ScenarioResult 为 JSON，写入 `{evidenceDir}/scenario_{scenario_id}.json`
  3. 实现 `LoadScenario(scenarioID)`：读 JSON 反序列化（判定脚本验证用）
  4. 实现 `ListScenarios()`：列证据目录下所有场景 JSON
  5. JSON 结构含 spec 6.2 定义的全部 10 个字段，时间戳使用 UTC 毫秒精度（ISO 8601 with ms）
  6. 落盘延迟 ≤ 2s（spec 4.1 性能约束）
- **输出**: `cmd/chaos_injector/evidence.go`
- **验收**: JSON 证据格式合规（10 字段齐全），含完整时间线，落盘延迟 ≤ 2s
- **commit 点**: `git add cmd/chaos_injector/evidence.go && git commit -m "T2.4: evidence 证据落盘模块"`
- **关联需求**: REQ-FI-06

### T2.5 实现 resume_mgr 断点续跑管理器
- **ID**: T2.5
- **状态**: [ ] 未开始
- **预估**: 20 分钟
- **输入**: T2.1 骨架 + spec.md §5.4 断点续跑协议 + §6.5 RESUME.md 结构
- **依赖**: T2.1
- **并行**: 可与 T2.2a/T2.2b/T2.3a/T2.3b/T2.4 并行
- **动作**:
  1. 在 `cmd/chaos_injector/resume_mgr.go` 中实现 `ResumeManager` 结构体（resumePath, fullRerun 字段）
  2. 实现 `LoadResume()`：读取 RESUME.md，解析 completed_scenarios 列表（结构化格式对齐 spec 6.5：completed_scenarios / last_updated / total_scenarios / session_id）
  3. 实现 `MarkCompleted(scenarioID)`：追加场景 ID 到已完成列表，更新 last_updated
  4. 实现 `IsCompleted(scenarioID)` / `ShouldSkip(scenarioID)`
  5. 支持 `--full-rerun` 开关（ShouldSkip 恒返回 false）
  6. RESUME.md 损坏处理：备份为 `.bak`，返回空状态
  7. 证据 JSON 与 RESUME.md 不一致处理：标记场景为"需重跑"
- **输出**: `cmd/chaos_injector/resume_mgr.go`
- **验收**: 断点续跑逻辑正确，--full-rerun 可忽略 RESUME.md，损坏文件可回退
- **commit 点**: `git add cmd/chaos_injector/resume_mgr.go && git commit -m "T2.5: resume_mgr 断点续跑管理器"`
- **关联需求**: REQ-RESUME-01 ~ REQ-RESUME-04

### T2.6 实现场景调度器 scheduler
- **ID**: T2.6
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**: T2.2a+T2.2b（node_ctl）+ T2.3a+T2.3b（collector）+ T2.4（evidence）+ T2.5（resume_mgr）+ design.md §2.5 场景执行流程
- **依赖**: T2.2a, T2.2b, T2.3a, T2.3b, T2.4, T2.5
- **并行**: 不可并行（依赖前序全部模块）
- **动作**:
  1. 在 `cmd/chaos_injector/scheduler.go` 中实现 `Scheduler` 结构体（持有 nodeCtl, collector, evidence, resumeMgr 引用）
  2. 实现 `runSteadyKillLeader(count=10)`：稳态杀 leader 场景调度（design §2.5.1 流程 14 步）
  3. 实现 `runUnderLoadKillLeader(count=10)`：压测中杀 leader，loadgen 子进程管理（c=128, -duration=600s），负载全程不中断（design §2.5.2 流程 17 步）
  4. 实现 `runCascadingKill(count=3)`：连杀场景（kill leader → 新 leader → kill follower，design §2.5.3 流程 10 步）
  5. 实现 `executeScenario(scenario)`：单场景执行流程（状态机，见 design §2.1.3.1），含 ShouldSkip 检查
  6. 实现 `killLeader()` + `waitForElection()` + `autoRestart()` 辅助方法
  7. 单场景超时 60s 处理（标记 TIMEOUT，继续下一场景）
  8. leader 查询失败重试 3 次间隔 1s（标记 BLOCKED，跳过）
  9. loadgen 子进程通过 `os/exec` 管理，压测场景启动 1 次、最后一个场景后停止
- **输出**: `cmd/chaos_injector/scheduler.go`
- **验收**: 三类场景均可执行，leader 动态识别，单场景 ≤ 60s，loadgen 子进程正确启停
- **commit 点**: `git add cmd/chaos_injector/scheduler.go && git commit -m "T2.6: scheduler 场景调度器"`
- **关联需求**: REQ-FI-03, REQ-FI-04, REQ-FI-05, REQ-FI-06

### T2.7 交叉编译 chaos_injector
- **ID**: T2.7
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**: T2.1-T2.6 全部源码
- **依赖**: T2.6
- **并行**: 不可并行
- **动作**:
  1. 执行交叉编译：`GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o chaos_injector_arm64 cmd/chaos_injector/main.go`
  2. 确认二进制可在 ARM64 Linux 运行（`--help` 输出 CLI 用法）
  3. 确认 `chaos_injector_arm64` 被 `.gitignore` 排除（T0.5 已补丁）
- **输出**: `chaos_injector_arm64`（二进制，不进版本库）
- **验收**: 二进制编译成功，`--help` 输出正确，`git status` 不显示为未跟踪
- **commit 点**: 无（二进制不进版本库，由 .gitignore 排除）
- **关联需求**: 无（构建任务）

---

## 阶段 3：场景执行（预估 1.5 小时）

> **并行策略**：T3.1 先行；T3.2/T3.3/T3.4 串行（共享集群，避免互相干扰）；T3.5 依赖全部场景完成。
> **阶段产物**：23 个场景 JSON 证据 + verdict.json

### T3.1 启动 5 节点 Docker 集群
- **ID**: T3.1
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**: `tests/deploy/docker-compose-5node.yml` + ports overlay + batch16 overlay + deploy.env
- **依赖**: T2.7（需 chaos_injector 二进制可用）
- **并行**: 不可并行
- **动作**:
  1. 执行集群编排命令：`docker compose -p deploy5 -f tests/deploy/docker-compose-5node.yml -f tests/deploy/docker-compose-5node-ports.yml -f tests/deploy/docker-compose-5node-batch16.yml --env-file tests/deploy/deploy.env up -d`
  2. 确认集群健康（1 leader + 4 follower），轮询 5 节点 `/health/live` 和 `/raft/status`
  3. 确认 leader 角色可查询（`QueryLeader()` 返回有效 nodeID）
  4. 确认证据目录 `tests/evidence/d3-batch21/` 存在
- **输出**: 5 节点集群运行中
- **验收**: 集群稳态（1 leader + 4 follower），所有节点 `/health/live` 返回 200
- **commit 点**: 无（环境准备，不产生代码变更）
- **关联需求**: 无（环境准备）

### T3.2 执行稳态杀 leader 场景 ×10
- **ID**: T3.2
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**: T2.7 二进制 + T3.1 集群 + T1.1 验收契约
- **依赖**: T2.7, T3.1
- **并行**: 不可并行（与 T3.3/T3.4 共享集群）
- **动作**:
  1. 执行 `./chaos_injector_arm64 --contract tests/contracts/batch21.yaml --evidence-dir tests/evidence/d3-batch21/ --cluster-config config.toml --scenario-type steady`
  2. 10 次杀 leader，每次记录选举时序、存活率、脑裂检测、日志
  3. 落盘 10 个 JSON 证据（`scenario_steady_kill_leader_01.json` ~ `_10.json`）
  4. 更新 RESUME.md（MarkCompleted 每场景后调用）
- **输出**: 10 个 `scenario_steady_kill_leader_*.json` + RESUME.md 更新
- **验收**: 10 个证据文件完整，含完整时间线，每场景 ≤ 60s
- **commit 点**: 无（证据不进版本库，RL-08）
- **关联需求**: REQ-FI-03

### T3.3 执行压测中杀 leader 场景 ×10
- **ID**: T3.3
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**: T2.7 二进制 + T3.1 集群 + T1.1 验收契约 + loadgen 子进程
- **依赖**: T3.2（串行，避免集群状态干扰）
- **并行**: 不可并行
- **动作**:
  1. chaos_injector 自动启动 loadgen 子进程（c=128, -duration=600s, -output=loadgen_stats.json）
  2. 执行 `./chaos_injector_arm64 ... --scenario-type under_load`
  3. 10 次杀 leader，负载全程不中断（loadgen 子进程持续运行）
  4. 落盘 10 个 JSON 证据（含注入期间和恢复后拒载率统计）
  5. 停止 loadgen 子进程
  6. 更新 RESUME.md
- **输出**: 10 个 `scenario_under_load_kill_leader_*.json` + `loadgen_stats.json`
- **验收**: 10 个证据文件完整，负载全程未中断，拒载率数据齐全
- **commit 点**: 无（证据不进版本库，RL-08）
- **关联需求**: REQ-FI-04

### T3.4 执行连杀场景 ×3
- **ID**: T3.4
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**: T2.7 二进制 + T3.1 集群 + T1.1 验收契约
- **依赖**: T3.3（串行，避免集群状态干扰）
- **并行**: 不可并行
- **动作**:
  1. 执行 `./chaos_injector_arm64 ... --scenario-type cascading`
  2. 3 次 leader+follower 连杀（kill leader → 新 leader → kill follower）
  3. 落盘 3 个 JSON 证据
  4. 更新 RESUME.md
- **输出**: 3 个 `scenario_cascading_kill_*.json`
- **验收**: 3 个证据文件完整，连续故障下集群韧性数据齐全
- **commit 点**: 无（证据不进版本库，RL-08）
- **关联需求**: REQ-FI-05

### T3.5 运行判定脚本
- **ID**: T3.5
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**: T3.2+T3.3+T3.4 产出的 23 个场景 JSON + T1.2 判定脚本 + T1.1 验收契约
- **依赖**: T3.2, T3.3, T3.4, T1.2
- **并行**: 不可并行
- **动作**:
  1. 执行 `python tests/contracts/judge_batch21.py --contract tests/contracts/batch21.yaml --evidence-dir tests/evidence/d3-batch21/ --output tests/evidence/d3-batch21/verdict.json`
  2. 脚本读 23 个证据 JSON + batch21.yaml
  3. 逐字段对照产出 `verdict.json`（F1-F5 判定 + overall + timestamp）
  4. 确认判定由脚本产出（非助手自写，红线 RL-09）
- **输出**: `tests/evidence/d3-batch21/verdict.json`
- **验收**: verdict.json 包含 F1-F5 判定 + overall，判定由脚本产出（非助手自写）
- **commit 点**: 无（证据不进版本库，RL-08）
- **关联需求**: REQ-JUDGE, RL-09

---

## 阶段 4：报告与闭案（预估 50 分钟）

> **并行策略**：T4.1/T4.2/T4.3 均依赖 T3.5，可三任务并行。T4.4 依赖 T0.3+T3.5。T4.5 依赖全部。
> **阶段产物**：报告.md + decisions.md + commit + tag

### T4.1 编写报告.md
- **ID**: T4.1
- **状态**: [ ] 未开始
- **预估**: 25 分钟
- **输入**: T3.5 产出的 `verdict.json` + 23 个场景证据 JSON
- **依赖**: T3.5
- **并行**: 可与 T4.2/T4.3 并行
- **动作**:
  1. 先结论后细节（用户偏好 PREFERENCE_20）：overall 判定 → F1-F5 逐项 → 场景矩阵摘要
  2. 引用 `verdict.json` 判定（不自写 PASS/FAIL，红线 RL-09）
  3. 附关键时间线摘录（选举时序、拒载率曲线）
  4. 硬数字 + 证据路径（每个 F 项附证据 JSON 路径）
  5. 若有 BLOCKED/TIMEOUT 场景，附场景清单和原因
- **输出**: `tests/evidence/d3-batch21/报告.md`
- **验收**: 报告先结论后细节，判定引用 verdict.json，硬数字齐全，每个 F 项有证据路径
- **commit 点**: 无（报告在证据目录，不进版本库）
- **关联需求**: RL-09

### T4.2 编写 decisions.md
- **ID**: T4.2
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**: T3.5 产出的 `verdict.json` + 设计决策点
- **依赖**: T3.5
- **并行**: 可与 T4.1/T4.3 并行
- **动作**:
  1. 记录关键决策点（场景设计依据、阈值来源、异常处理策略）
  2. 记录三等级处置结论（PASS/PARTIAL/FAIL 对应后续动作）
  3. 记录挂账清偿状态（DEBT-0003 是否清偿）
  4. 记录时间盒实际耗时
- **输出**: `tests/evidence/d3-batch21/decisions.md`
- **验收**: 决策记录完整可追溯，含三等级处置结论
- **commit 点**: 无（在证据目录，不进版本库）
- **关联需求**: REQ-DISP-PASS ~ REQ-DISP-FAIL

### T4.3 三等级处置执行
- **ID**: T4.3
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**: T3.5 产出的 `verdict.json`（overall 判定）
- **依赖**: T3.5
- **并行**: 可与 T4.1/T4.2 并行
- **动作**:
  - 若 overall=PASS → 签发闭案决议，准备 tag `v2.4-post-batch21`
  - 若 overall=PARTIAL → 挂账未通过项至下一批次，记录根因初判，不闭案
  - 若 overall=FAIL → 触发红线告警，记录失败详情，须晨批审议，不得自行修复后重判（红线 RL-09）
- **输出**: 处置结论写入 decisions.md
- **验收**: 处置等级与 verdict.json overall 一致
- **commit 点**: 无（结论写入 decisions.md）
- **关联需求**: REQ-DISP-PASS, REQ-DISP-PARTIAL, REQ-DISP-FAIL

### T4.4 性能章节闭案判定
- **ID**: T4.4
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**: T0.3 产出的 `decomp_c512_raw.json` + T0.2 量纲标注 + T3.5 verdict.json
- **依赖**: T0.3, T0.2, T3.5
- **并行**: 可与 T4.1/T4.2/T4.3 并行
- **动作**:
  1. 确认 `decomp_c512_raw.json` 已补全（四构成项齐全，≥1KB）
  2. 确认 fsync 量纲已澄清（T0.2 输出 7 字段齐全）
  3. 判定性能章节是否正式闭案（decomp 数据 + fsync 取证 + F1-F5 判定综合）
  4. 更新 `LEDGER.md` 中 DEBT-0003 状态为"已清偿"，填写 cleared_at 时间戳
- **输出**: 性能章节闭案结论 + LEDGER.md 更新
- **验收**: 闭案结论有 decomp 数据支撑，LEDGER.md DEBT-0003 已清偿
- **commit 点**: `git add LEDGER.md && git commit -m "T4.4: 性能章节闭案 + DEBT-0003 清偿"`
- **关联需求**: REQ-T0-03

### T4.5 提交 + 打 tag
- **ID**: T4.5
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**: T4.1/T4.2/T4.3/T4.4 全部完成
- **依赖**: T4.1, T4.2, T4.3, T4.4
- **并行**: 不可并行
- **动作**:
  1. `git add` 代码文件：`cmd/chaos_injector/` + `tests/contracts/` + `LEDGER.md` + `docs/specs/fault_injection/` + `docs/specs/latency_decomp/` + `.gitignore`
  2. **不 add 证据目录**（`tests/evidence/d3-batch21/` 被 .gitignore 排除，红线 RL-08）
  3. `git commit -m "D3-batch21-chaos1"`
  4. 若 overall=PASS → `git tag v2.4-post-batch21`
- **输出**: commit `D3-batch21-chaos1` + tag `v2.4-post-batch21`（若 PASS）
- **验收**: 证据目录未进版本库（`git status` 确认），tag 已打（若 PASS）
- **commit 点**: `git commit -m "D3-batch21-chaos1"` + `git tag v2.4-post-batch21`
- **关联需求**: RL-08

---

## 阶段 5：收尾验证（预估 15 分钟）

> **并行策略**：T5.1/T5.2 可并行。
> **阶段产物**：红线自查清单 + 集群状态确认

### T5.1 红线自查
- **ID**: T5.1
- **状态**: [ ] 未开始
- **预估**: 10 分钟
- **输入**: 全部前序任务产出
- **依赖**: T4.5
- **并行**: 可与 T5.2 并行
- **动作**: 逐条核对 RL-01 ~ RL-10 红线：
  - RL-01: fsync 语义未破坏（chaos_injector 不修改集群代码）
  - RL-02: quorum 语义未破坏（不修改 quorum 配置）
  - RL-03: 选举超时语义未破坏（不修改选举超时配置）
  - RL-04: PASS 判定逐字段对照 YAML（judge_batch21.py 逐字段比对）
  - RL-05: 改 YAML 须晨批（YAML 未修改）
  - RL-06: 挂账到期未清 = 自动红线违反（LEDGER.md 检查）
  - RL-07: 可观测性代码不进写路径热区（chaos_injector 是外部工具）
  - RL-08: 证据目录被 .gitignore 排除（`git status` 确认）
  - RL-09: 助手不得自写 PASS/FAIL（报告引用 verdict.json）
  - RL-10: 时间盒 ≤ 6 小时（实际耗时统计）
- **输出**: 红线自查清单（写入 decisions.md 附录）
- **验收**: 10 条红线全部未违反
- **commit 点**: 无（自查结论写入 decisions.md，在证据目录）
- **关联需求**: RL-01 ~ RL-10

### T5.2 集群恢复确认
- **ID**: T5.2
- **状态**: [ ] 未开始
- **预估**: 5 分钟
- **输入**: T3.4 后的集群状态
- **依赖**: T4.5
- **并行**: 可与 T5.1 并行
- **动作**: 确认 5 节点集群恢复到健康稳态（1 leader + 4 follower），所有节点 `/health/live` 返回 200
- **输出**: 集群状态确认（写入 decisions.md 附录）
- **验收**: 集群健康（1 leader + 4 follower）
- **commit 点**: 无
- **关联需求**: spec 4.2 可靠性

---

## 时间盒总览

| 阶段 | 任务数 | 预估时长 | 累计 | 关键产物 |
|------|--------|----------|------|----------|
| 阶段 0：任务零 | 5 | 1.5 小时 | 1.5h | LEDGER.md + decomp_c512_raw.json + .gitignore |
| 阶段 1：验收契约 | 2 | 0.83 小时 | 2.33h | batch21.yaml + judge_batch21.py |
| 阶段 2：工具实现 | 8 | 2.58 小时 | 4.92h | chaos_injector_arm64 |
| 阶段 3：场景执行 | 5 | 1.33 小时 | 6.25h | 23 个场景 JSON + verdict.json |
| 阶段 4：报告闭案 | 5 | 0.83 小时 | 7.08h | 报告.md + decisions.md + commit + tag |
| 阶段 5：收尾验证 | 2 | 0.25 小时 | 7.33h | 红线自查清单 |

> **注意**：总预估 7.33h 超 6h 时间盒。优化策略：
> 1. **阶段 0 和阶段 1 可并行**（任务零与验收契约落盘无依赖，T0.1/T0.2/T0.3/T0.5 + T1.1 五任务并行）
> 2. **阶段 2 内部并行**：T2.2a/T2.2b/T2.4/T2.5 可四任务并行（均仅依赖 T2.1）；T2.3a/T2.3b 可两任务并行
> 3. **阶段 4 内部并行**：T4.1/T4.2/T4.3/T4.4 可四任务并行（均依赖 T3.5）
> 4. 实际执行以时间盒为准，到点停手交数据（红线 RL-10）
> 5. 若时间盒紧张，优先保证阶段 0-3（产物交付），阶段 4-5 可延后

---

## 任务依赖图

```
阶段 0（可并行）:
T0.1 ─┬─ T0.4
T0.2 ─┤
T0.3 ─┴─ T4.4
T0.5

阶段 1（T1.1 与阶段 0 并行）:
T1.1 ── T1.2 ──────────────────────┐
                                    │
阶段 2（内部并行）:
T2.1 ─┬─ T2.2a ─┐                   │
      ├─ T2.2b ─┤                   │
      ├─ T2.4 ──┤                   │
      ├─ T2.5 ──┤                   │
T2.3a ─┤        │                   │
T2.3b ─┘        │                   │
T2.6 ─── T2.7 ─┤                   │
                │                   │
阶段 3（串行）:
T3.1 ── T3.2 ── T3.3 ── T3.4 ── T3.5 ──┐
                                        │
阶段 4（T4.1-T4.4 可并行）:
T4.1 ──┤                                │
T4.2 ──┤                                │
T4.3 ──┤                                │
T4.4 ──┘                                │
T4.5 ───────────────────────────────────┘
                                        │
阶段 5（可并行）:
T5.1 ── T5.2 ───────────────────────────┘
```

**并行优化汇总**：
- 阶段 0：T0.1/T0.2/T0.3/T0.5 四任务并行（节省 ~45min）
- 阶段 0+1：T1.1 与阶段 0 并行（节省 ~20min）
- 阶段 2：T2.2a/T2.2b/T2.4/T2.5 四任务并行（节省 ~50min）；T2.3a/T2.3b 并行（节省 ~20min）
- 阶段 4：T4.1/T4.2/T4.3/T4.4 四任务并行（节省 ~40min）
- 阶段 5：T5.1/T5.2 并行（节省 ~5min）
- **理论并行后总耗时**：~4.5h（在 6h 时间盒内）

---

## 产物清单

| 产物 | 路径 | 阶段 | 进版本库 | 关联任务 |
|------|------|------|----------|----------|
| LEDGER.md | `LEDGER.md` | T0.1 | ✓ | T0.1, T4.4 |
| fsync 量纲标注 | `docs/specs/latency_decomp/fsync_dimensional_clarification.md` | T0.2 | ✓ | T0.2 |
| decomp_c512_raw.json | `docs/specs/latency_decomp/decomp_c512_raw.json` | T0.3 | ✓ | T0.3 |
| .gitignore 补丁 | `.gitignore` | T0.5 | ✓ | T0.5 |
| 验收契约 | `tests/contracts/batch21.yaml` | T1.1 | ✓ | T1.1 |
| 判定脚本 | `tests/contracts/judge_batch21.py` | T1.2 | ✓ | T1.2 |
| chaos_injector 源码 | `cmd/chaos_injector/*.go`（6 文件） | T2.1-T2.6 | ✓ | T2.1-T2.6 |
| chaos_injector 二进制 | `chaos_injector_arm64` | T2.7 | ✗（.gitignore） | T2.7 |
| 场景证据 JSON ×23 | `tests/evidence/d3-batch21/scenario_*.json` | T3.2-T3.4 | ✗（.gitignore） | T3.2-T3.4 |
| loadgen 统计 | `tests/evidence/d3-batch21/loadgen_stats.json` | T3.3 | ✗（.gitignore） | T3.3 |
| 判定输出 | `tests/evidence/d3-batch21/verdict.json` | T3.5 | ✗（.gitignore） | T3.5 |
| 报告 | `tests/evidence/d3-batch21/报告.md` | T4.1 | ✗（.gitignore） | T4.1 |
| 决策记录 | `tests/evidence/d3-batch21/decisions.md` | T4.2 | ✗（.gitignore） | T4.2 |
| RESUME.md | `tests/evidence/d3-batch21/RESUME.md` | T2.5 | ✗（.gitignore） | T2.5, T3.2-T3.4 |
| commit + tag | `D3-batch21-chaos1` + `v2.4-post-batch21` | T4.5 | ✓ | T4.5 |

---

## 需求追溯矩阵（任务 → 需求）

| 任务 ID | 对应需求 | 验收点 | 预估 |
|---------|----------|--------|------|
| T0.1 | REQ-T0-01 | LEDGER.md ≥3 笔已清记录 | 15min |
| T0.2 | REQ-T0-02 | 量纲标注 7 字段齐全 | 20min |
| T0.3 | REQ-T0-03 | decomp 四构成项全字段 ≥1KB | 30min |
| T0.4 | REQ-T0-04 | 晨报首屏含 decomp 摘要 | 10min |
| T0.5 | REQ-T0-05, RL-08 | git status 无 loadgen.exe | 15min |
| T1.1 | REQ-F1~F5, REQ-YAML | YAML 格式合法，23 场景 | 20min |
| T1.2 | REQ-JUDGE, RL-04, RL-09 | 脚本产出 verdict.json | 30min |
| T2.1 | REQ-FI-06 | go build 通过 | 25min |
| T2.2a | REQ-FI-01, REQ-FI-02 | kill/restart 可用 | 15min |
| T2.2b | REQ-FI-07 | QueryLeader 动态识别 | 20min |
| T2.3a | REQ-F1, REQ-F2 | 选举+拒载可采集 | 20min |
| T2.3b | REQ-F3, REQ-F4, REQ-F5 | 存活+脑裂+日志可采集 | 20min |
| T2.4 | REQ-FI-06 | JSON 证据 10 字段合规 | 15min |
| T2.5 | REQ-RESUME-01~04 | 断点续跑正确 | 20min |
| T2.6 | REQ-FI-03,04,05,06 | 三类场景可执行 | 30min |
| T2.7 | 无（构建） | 二进制可运行 | 10min |
| T3.1 | 无（环境） | 集群稳态 | 10min |
| T3.2 | REQ-FI-03 | 10 个稳态证据完整 | 15min |
| T3.3 | REQ-FI-04 | 10 个压测证据完整 | 30min |
| T3.4 | REQ-FI-05 | 3 个连杀证据完整 | 15min |
| T3.5 | REQ-JUDGE, RL-09 | verdict.json 由脚本产出 | 10min |
| T4.1 | RL-09 | 报告先结论后细节 | 25min |
| T4.2 | REQ-DISP-* | 决策记录完整 | 15min |
| T4.3 | REQ-DISP-PASS/PARTIAL/FAIL | 处置等级一致 | 10min |
| T4.4 | REQ-T0-03 | 性能章节闭案有数据支撑 | 10min |
| T4.5 | RL-08 | 证据未进版本库，tag 已打 | 10min |
| T5.1 | RL-01~RL-10 | 10 条红线未违反 | 10min |
| T5.2 | spec 4.2 | 集群健康 | 5min |

**需求覆盖统计**：
- 29 条 EARS 需求全部映射到任务
- 10 条红线全部有对应验证任务（T5.1）
- 27 个任务，总预估 7.33h（并行优化后 ~4.5h）
