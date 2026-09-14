# batch23 pre-vote 防选票分裂 + 磁盘满故障注入实施 — 实施任务分解

> 版本: v2.4-batch23-prevote-diskfull
> 关联规格: spec.md（696 行，28 条 EARS 需求）
> 关联设计: design.md（1371 行，pre-vote 状态机 + 磁盘满注入架构 + 协同设计）
> 时间盒: 总计 ≤ 6 小时（任务一 pre-vote 3h + 任务二磁盘满 3h），到点停手交数据（红线 RL-10）
> 命名约定: snake_case（函数/变量/脚本），kebab-case（目录），大驼峰（文档文件名）
> 前置批次: batch22（verdict PASS, commit 29139aa, tag v2.4-post-batch22, E1=1.6003s / E2=1 / E3=20% / S1=100% / S2=20 / F1-F5 全 PASS）
> 蓝本依据: docs/specs/batch22/disk_full_spec.md + disk_full_design.md（磁盘满方案两件套，禁止推翻重设计）
> 任务粒度: 每个任务 ≤ 30 分钟，完成后可独立 commit
> 集群编排: docker compose -p deploy5 -f tests/deploy/docker-compose-5node.yml -f tests/deploy/docker-compose-5node-ports.yml -f tests/deploy/docker-compose-5node-batch16.yml --env-file tests/deploy/deploy.env
> 核心红线: RL-01~RL-11 继承（fsync/quorum/选举超时/proto/证据隔离/判定脚本/构建产物等不可破坏）
> 协同要点: pre-vote 使磁盘满 leader 降级后选举 1 轮成功 ≤2s（design.md §2.1.3.3）

---

## 并行优化总览

> **并行策略**：阶段 0 必须先行（git bundle + tag + LEDGER 入账为后续提供基线）；阶段 1（pre-vote，改 raft.go + raft_transport.go）与阶段 2（磁盘满，改 chaos_injector）源码修改互不依赖，可并行；T3.1（batch23.yaml 纯 YAML 定义）可与阶段 1/2 并行；阶段 3 其余任务依赖阶段 1+2 全部完成；阶段 4 依赖阶段 3。

| 阶段 | 串行预估 | 并行预估 | 关键约束 |
|------|----------|----------|----------|
| 阶段 0 任务零前置 | 0.50h | 0.50h（T0.1 单任务） | 必须先行，为后续提供 git bundle 备份 + LEDGER 入账 |
| 阶段 1 pre-vote 实现 | 2.50h | 2.00h（T1.1/T1.2 并行 → T1.3 → T1.4 → T1.5） | 与阶段 2/T3.1 可并行 |
| 阶段 2 磁盘满注入 | 2.00h | 1.50h（T2.1 → T2.2 → T2.3/T2.4 并行） | 与阶段 1/T3.1 可并行 |
| 阶段 3 闭案 | 1.75h | 1.25h（T3.1 先行 → T3.2 → T3.3 → T3.4） | T3.1 可与阶段 1/2 并行；T3.2+ 依赖阶段 1+2 完成 |
| 阶段 4 收尾 | 0.50h | 0.25h（T4.1/T4.2 并行） | 依赖阶段 3 |
| **串行总计** | **7.25h** | | 超出 6h 时间盒 |
| **并行总计** | | **0.50 + max(2.00, 1.50, 0.50) + 1.25 + 0.25 = 4.00h** | **≤ 6h ✅** |

> **并行后总预估：4.00h ≤ 6h，满足时间盒约束（红线 RL-10）**。预留 2.00h 作为集群部署/编译/重测/WAL 卷确认的缓冲时间。

---

## 阶段 0：任务零前置 — git bundle 备份 + tag 打点 + LEDGER 入账（预估 0.5h，并行 0.5h）

> **并行策略**：T0.1 单任务，必须先行。
> **阶段产物**：git bundle 备份 + tag v2.4-pre-batch23 + LEDGER.md 更新（L-22-1/L-22-2 入账）

### T0.1 git bundle 备份 + tag v2.4-pre-batch23 + LEDGER L-22-1/L-22-2 入账
- **ID**: T0.1
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `LEDGER.md`（当前 5 笔记录：DEBT-0001~0003 + L-21-1/L-21-2 全部已清偿）
  - batch22 verdict.json（overall=PASS, E1=1.6003s, commit 29139aa, tag v2.4-post-batch22）
  - spec.md §11 LEDGER 挂账计划（L-22-1 / L-22-2 两笔入账）
  - design.md §2.7 实现优先级与时间盒分配
- **依赖**: 无
- **并行**: 不可并行（后续任务的基础）
- **动作**:
  1. 创建 git bundle 备份：`git bundle create ../bundle/v2.4-pre-batch23.bundle --all`（在阶段 0 开始前打点，便于回滚）
  2. 打预备 tag：`git tag v2.4-pre-batch23`（标注 batch23 实施起点）
  3. 在 LEDGER.md 表格追加两笔新入账：
     - `L-22-1 | batch22 | batch23 | 待清 | W-1 pre-vote 未实现：batch22 选举优化绕行项，cascading_kill_02 选举 3.44s 为选票分裂导致多轮选举，需实现 pre-vote 防选票分裂 | —`
     - `L-22-2 | batch22 | batch23 | 待清 | D-8 磁盘满方案仅设计不实施：batch22 任务三预演完成 disk_full_spec.md + disk_full_design.md 两件套，实施 deferred 到 batch23 | —`
  4. 在 LEDGER.md "待清记录详情"章节追加 L-22-1/L-22-2 的待清记录详情（来源/应清批次/清偿条件/数据来源）
  5. 格式与现有 DEBT-0001~0003 + L-21-1/L-21-2 一致（六字段表格：debt_id/source_batch/target_batch/status/description/cleared_at）
  6. 待 T3.3 完成后回填 L-22-1/L-22-2 的 cleared_at（标注 batch23 commit SHA）
- **输出**: `../bundle/v2.4-pre-batch23.bundle` + tag `v2.4-pre-batch23` + `LEDGER.md`（更新后，含 7 笔记录：5 笔已清偿 + 2 笔待清）
- **验收**: git bundle 文件存在 → tag v2.4-pre-batch23 可见 → LEDGER.md 存在 → L-22-1/L-22-2 两笔入账且格式合规 → 每笔有唯一 ID → 表格六字段齐全
- **commit 点**: `git add LEDGER.md && git commit -m "T0.1: LEDGER L-22-1/L-22-2 入账 + git bundle + tag v2.4-pre-batch23"`
- **关联需求**: REQ-T0-01, REQ-T0-02

---

## 阶段 1：pre-vote 实现 — 状态机修改 + RPC 端点 + HandlePreVote（预估 2.5h，并行 2.0h）

> **并行策略**：T1.1（PreVote RPC 传输层，改 raft_transport.go）与 T1.2（HandlePreVote 服务端处理，改 raft.go Handle 方法）可两任务并行；T1.3（requestPreVotes）依赖 T1.1+T1.2；T1.4（handleElectionTimeout 注入）依赖 T1.3；T1.5（取证 + 场景执行）依赖 T1.4。
> **阶段产物**：raft_transport.go PreVote RPC 端点 + raft.go pre-vote 状态机注入 + collector.go 取证采集 + 14 场景证据 JSON
> **与阶段 2/T3.1 可并行**：pre-vote 改 raft.go/raft_transport.go，磁盘满改 chaos_injector，batch23.yaml 纯 YAML 定义，三者源码修改互不依赖

### T1.1 PreVote RPC 传输层扩展（raft_transport.go）
- **ID**: T1.1
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `pkg/raft-module/raft_transport.go:1-256`（HTTPTransport + HTTPServer，RequestVote/AppendEntries 两个端点）
  - design.md §1.2.5 HTTP RPC 传输层分析 + §2.2.2.1 POST /raft/pre_vote 接口签名 + §2.3.2 PreVoteRequest/Response 数据模型
  - spec.md §5.1.1 业务规则 1（REQ-PV-01：pre-vote 探测先行）
  - 约束：protoc 不可用，不修改 proto 定义（RL-07），PreVote RPC 通过路径区分
- **依赖**: 阶段 0 完成（LEDGER L-22-1 已入账）
- **并行**: 可与 T1.2 并行
- **动作**:
  1. 在 `pkg/raft-module/raft_transport.go` 中新增 `PreVoteRequest` 数据结构：
     - 字段：`Term int64` / `CandidateId string` / `LastLogIndex int64` / `LastLogTerm int64`
     - 与现有 RequestVoteRequest 字段一致，语义不同（预支持而非正式投票）
  2. 新增 `PreVoteResponse` 数据结构：
     - 字段：`Term int64` / `PreVoteGranted bool`
  3. 新增路径常量 `pathPreVote = "/raft/pre_vote"`
  4. 在 `HTTPTransport` 中新增 `PreVote(req *PreVoteRequest) (*PreVoteResponse, error)` 客户端方法：
     - 复用 `doRPC(pathPreVote, req)` 底层，JSON 编解码
     - 客户端超时 600ms（与 RequestVote 一致）
  5. 在 `NewHTTPServer` 中新增 `mux.HandleFunc(pathPreVote, srv.handlePreVote)` 路由注册
  6. 新增 `handlePreVote` HTTP handler：
     - 解析 JSON 请求体 → 调用 `node.HandlePreVote` → 编码 JSON 响应
     - 异常映射：非 POST → 405；JSON 解码失败 → 400；内部错误 → 500
  7. 确认不影响现有 RequestVote/AppendEntries 端点（纯增量扩展）
- **输出**: `pkg/raft-module/raft_transport.go`（新增 PreVoteRequest/Response + HTTPTransport.PreVote + pathPreVote 路由 + handlePreVote handler）
- **验收**: PreVoteRequest/Response 结构定义 → HTTPTransport.PreVote 方法可调用 → pathPreVote 路由注册 → handlePreVote handler 实现 → 不修改 proto 定义（RL-07）→ 编译通过
- **commit 点**: `git add pkg/raft-module/raft_transport.go && git commit -m "T1.1: PreVote RPC 传输层扩展（/raft/pre_vote 端点）"`
- **关联需求**: REQ-PV-01, RL-07

### T1.2 HandlePreVote 服务端处理（raft.go）
- **ID**: T1.2
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `raft.go:1719-1815+`（HandleRequestVote 五重投票检查逻辑）
  - design.md §1.2.3 HandleRequestVote 分析 + §2.2.2.3 HandlePreVote 接口签名 + §2.9.1 pre-vote 安全性论证
  - spec.md §5.1.1 业务规则 4-6（REQ-PV-04：pre-vote 不增加 term / REQ-PV-05：不阻止其他节点 / REQ-PV-06：不破坏 Raft 安全性）
  - T1.1 产出的 PreVoteRequest/PreVoteResponse 数据结构
- **依赖**: 阶段 0 完成
- **并行**: 可与 T1.1 并行
- **动作**:
  1. 在 `raft.go` 中新增 `HandlePreVote(req *PreVoteRequest) (*PreVoteResponse, error)` 方法
  2. 复用 HandleRequestVote 的五重检查**判断条件**，但**不执行状态变更**：
     - 检查 1：`req.Term < rn.term` → PreVoteGranted=false（候选者 term 过期）
     - 检查 2：候选者不在当前配置中 → false
     - 检查 3：WAL 重放未完成 / WAL 门禁关闭（walGateClosed）→ false
     - 检查 4：votedFor 已投他人 → false（注意：检查但不更新 votedFor）
     - 检查 5：自身日志未追上 / 候选者日志为空但集群已提交 / 候选者日志不如自己新 → false
  3. **关键差异**（与 HandleRequestVote 对比）：
     - 不更新 term（pre-vote 不增加 term，避免 term 膨胀）
     - 不更新 votedFor（pre-vote 不真正投票，不阻止后续正式选举投票）
     - 不降级 state（pre-vote 不改变节点角色）
     - 不重置 electionTimer（pre-vote 不干扰选举定时器）
  4. pre-vote 的 term 检查为"候选者 term+1 ≥ 我的 term"（预探测下一轮选举胜算），而非"候选者 term > 我的 term"
  5. 返回 `PreVoteResponse{Term: rn.term, PreVoteGranted: true/false}`
  6. 确认方法为纯只读判断，无副作用（不修改任何持久化状态）
- **输出**: `raft.go`（新增 HandlePreVote 方法）
- **验收**: HandlePreVote 方法实现 → 五重检查复用但不更新状态 → 不更新 votedFor/term/state/electionTimer → 纯只读判断 → 编译通过
- **commit 点**: `git add raft.go && git commit -m "T1.2: HandlePreVote 服务端处理（复用五重检查不更新状态）"`
- **关联需求**: REQ-PV-04, REQ-PV-05, REQ-PV-06

### T1.3 requestPreVotes 并发预投票方法（raft.go）
- **ID**: T1.3
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `raft.go:868-991`（requestVotes 并发正式选举流程，参考实现风格）
  - T1.1 产出的 `HTTPTransport.PreVote` 客户端方法
  - T1.2 产出的 `HandlePreVote` 服务端处理
  - design.md §2.2.2.2 requestPreVotes 接口签名 + §2.1.3.1 pre-vote 状态机设计
  - spec.md §5.1.1 业务规则 2-3（REQ-PV-02：获多数派转正式 / REQ-PV-03：未获保持 Follower）
- **依赖**: T1.1, T1.2
- **并行**: 不可并行（T1.4 的基础）
- **动作**:
  1. 在 `raft.go` 中新增 `requestPreVotes(preVoteTerm int64, peers []PeerInfo) int32` 方法
  2. 核心逻辑：
     - `votesGranted` 初始为 1（自己预投自己一票，不更新 votedFor，仅计数）
     - 并发发送 PreVote RPC 到所有 peer：`PreVoteRequest{Term: preVoteTerm, CandidateId: rn.id, LastLogIndex: lastLogIndex, LastLogTerm: lastLogTerm}`
     - 每个 peer 请求独立 goroutine + recover panic（连接断开不计入票数）
     - rpcTimeout(500ms) 内未响应的 peer 计为不支持
     - 收到 PreVoteGranted=true → votesGranted++
     - 收到更高 term 响应 → stepDown 降级（与 requestVotes 一致）
  3. 返回预支持票数（int32，含自己一票）
  4. 确认不修改任何持久化状态（votedFor/term/state 不变）
- **输出**: `raft.go`（新增 requestPreVotes 方法）
- **验收**: requestPreVotes 方法实现 → 并发发送 PreVote RPC → 统计预支持票数 → 不修改持久化状态 → recover panic → rpcTimeout 超时处理 → 编译通过
- **commit 点**: `git add raft.go && git commit -m "T1.3: requestPreVotes 并发预投票方法"`
- **关联需求**: REQ-PV-02, REQ-PV-03

### T1.4 handleElectionTimeout 注入 pre-vote 探测 + 编译部署（raft.go）
- **ID**: T1.4
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `raft.go:791-862`（handleElectionTimeout 选举状态机，pre-vote 注入点：第 836 行之后、第 838 行之前）
  - T1.3 产出的 `requestPreVotes` 方法
  - design.md §1.2.1 handleElectionTimeout 分析 + §2.1.3.1 pre-vote 状态机设计 + §2.8.1 与现有选举状态机关系
  - spec.md §5.1.1 业务规则 1-8（REQ-PV-01~08）+ §4.1 性能约束
  - 约束：pre-vote 不改 electionTimeout（RL-03），800ms > rpcTimeout(500ms) 约束维持
- **依赖**: T1.3
- **并行**: 不可并行（T1.5 的基础）
- **动作**:
  1. 在 `raft.go` `handleElectionTimeout()` 方法中，第 836 行（选举风暴自愈）之后、第 838 行（`rn.state = StateCandidate`）之前注入 pre-vote 探测逻辑：
     - 计算 `preVoteTerm = rn.term + 1`（预探测 term，不写入 rn.term）
     - 调用 `requestPreVotes(preVoteTerm, peers)` 统计预支持票数
     - **获 ≥ quorum(3/5) 预支持** → 执行第 838-844 行 Candidate 转换（现有流程：term+1 + 投自己 + requestVotes）
     - **未获 quorum 预支持** → 保持 Follower，重置选举定时器（`randomElectionTimeout() * 2` 回退），return
  2. pre-vote 探测超时（rpcTimeout 内未收到多数派响应）→ 保持 Follower，*2 回退
  3. pre-vote 失败不阻止其他节点发起选举（不引入活锁）
  4. 正式选举阶段仍遵循现有 RequestVote + quorum + term+1 逻辑（Raft 安全性保持）
  5. 新增结构化日志：pre-vote 探测发起/获支持/未获支持事件（含节点 ID/term/预支持票数/quorum）
  6. 重新编译集群镜像并部署（5 节点 Docker 集群）
  7. 确认 pre-vote 不改 electionTimeout（800-1200ms 不变，RL-03 不违反）
- **输出**: `raft.go`（handleElectionTimeout 扩展 pre-vote 探测）+ 集群重新编译部署
- **验收**: pre-vote 注入位置正确（第 836 行后）→ 获 quorum 转 Candidate → 未获保持 Follower + *2 回退 → pre-vote 不增加 term → 选举超时 800-1200ms 不变（RL-03）→ 编译通过 → 集群部署成功
- **commit 点**: `git add raft.go && git commit -m "T1.4: handleElectionTimeout 注入 pre-vote 探测 + 编译部署"`
- **关联需求**: REQ-PV-01, REQ-PV-04, REQ-PV-07, REQ-PV-08, RL-03

### T1.5 pre-vote 取证采集 + 14 场景执行（collector.go + scheduler.go）
- **ID**: T1.5
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - T1.4 产出的 pre-vote 实现并部署的集群
  - `cmd/chaos_injector/collector.go`（已有 CollectElectionTimeline/CollectElectionForensics）
  - `cmd/chaos_injector/scheduler.go`（已有 BuildScenarioMatrix/ExecuteScenario）
  - `cmd/chaos_injector/node_ctl.go`（KillNode/StartNode/QueryLeader）
  - design.md §2.2.2.8 CollectPreVoteForensics 接口签名 + §2.4.1 pre-vote 验证场景（14 场景）+ §2.3.2 PreVoteForensics 数据模型
  - spec.md §5.1.1 业务规则 9（REQ-PV-09：取证落盘）+ §6.1 prevote_forensics.json 数据约束 + §9.1 pre-vote 验证场景
  - batch22 基线：`tests/evidence/d3-batch22/election_forensics.json`（pre_vote.status=not_implemented）+ `verdict.json`（E1 all_steady 含 2.78/3.19s 离群值）
- **依赖**: T1.4
- **并行**: 不可并行（阶段 1 收尾）
- **动作**:
  1. 在 `cmd/chaos_injector/collector.go` 中新增 `CollectPreVoteForensics(scenarioID string) (*PreVoteForensics, error)` 方法
  2. 采集逻辑：
     - 场景运行前采集 `term_before`（各节点 /raft/stats 的 term 最大值）
     - 运行 cascading 场景（kill L1 → L2 选举 → kill L2 → L3 选举）
     - 100ms 轮询各节点 /raft/stats 的 state/voted/term，记录选举轮数与每轮选票分布
     - 采集 `term_after` + `prevote_round_count` + `formal_election_round_count` + `vote_distribution`
     - 加载 batch22 基线（verdict.json E1 all_steady + election_forensics.json）
     - 对照 term 膨胀量（term_after - term_before）
  3. 落盘至 `tests/evidence/d3-batch23/prevote_forensics.json`，结构含：
     - `scenario_id` / `prevote_round_count` / `formal_election_round_count` / `vote_distribution` / `term_before` / `term_after` / `election_completion_s` / `batch22_baseline` / `timestamp`
  4. 在 `cmd/chaos_injector/scheduler.go` 中新增 pre-vote 场景调度：
     - `BuildScenarioMatrix` 新增 `case "prevote"` 分支生成 14 场景 ID（steady_prevote_01~10 + cascading_prevote_01~03 + prevote_forensics）
     - `ExecuteScenario` 新增 `case contains(scenarioID, "prevote")` 分发
  5. 执行 14 场景：
     - **稳态选举不劣化（steady_prevote_01~10）**：kill leader → 等待选举 → 验证 E1/E2/E3 → 重启节点
     - **cascading 选举收敛（cascading_prevote_01~03）**：kill L1 → L2 → L3，验证每次选举 ≤2s（E4）
     - **pre-vote 取证对照（prevote_forensics）**：采集取证数据与 batch22 基线对照
  6. 落盘 14 场景证据 JSON 至 `tests/evidence/d3-batch23/steady_prevote_*.json` + `cascading_prevote_*.json` + `prevote_forensics.json`
  7. 恢复集群（重启被 kill 的节点，等待健康）
  8. **异常处理**：
     - 若 E2 FAIL（检测到脑裂）：回退 pre-vote 实现（恢复 handleElectionTimeout 原逻辑），标注 E2=FAIL，触发红线
     - 若 E4 FAIL（cascading 选举 >2s）：如实记录，分析 pre-vote 探测命中率与选票分裂残余原因
- **输出**: `cmd/chaos_injector/collector.go`（扩展）+ `cmd/chaos_injector/scheduler.go`（扩展）+ `tests/evidence/d3-batch23/prevote_forensics.json` + 14 场景证据 JSON
- **验收**: prevote_forensics.json 落盘 → 含轮数/选票分布/term 膨胀/batch22 基线对照 → 14 场景证据 JSON 落盘 → E1 中位 ≤2s → E4 cascading 每次 ≤2s → E2 max(leaders) ≤1 → 时间线单调递增无缺口
- **commit 点**: `git add cmd/chaos_injector/collector.go cmd/chaos_injector/scheduler.go && git commit -m "T1.5: pre-vote 取证采集 + 14 场景执行 + 证据落盘"`
- **关联需求**: REQ-PV-07, REQ-PV-09, REQ-E1, REQ-E2, REQ-E4

---

## 阶段 2：磁盘满故障注入实施 — disk_ctl.go + chaos_injector 扩展 + 场景执行（预估 2.0h，并行 1.5h）

> **并行策略**：T2.1（disk_ctl.go）先行；T2.2（scheduler.go 扩展）依赖 T2.1；T2.3（软满 3 场景）/ T2.4（硬满 3 场景）均依赖 T2.2，可两任务并行。
> **阶段产物**：disk_ctl.go 注入/检测/清理三方法 + scheduler.go disk_full 场景调度 + 6 场景证据 JSON
> **与阶段 1/T3.1 可并行**：磁盘满改 chaos_injector，pre-vote 改 raft.go，batch23.yaml 纯 YAML 定义，三者互不依赖

### T2.1 disk_ctl.go 注入/检测/清理三方法（cmd/chaos_injector/disk_ctl.go）
- **ID**: T2.1
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `docs/specs/batch22/disk_full_design.md`（蓝本：disk_ctl.go 架构 + 注入/检测/清理方法 + 数据模型 + 风险约束）
  - `docs/specs/batch22/disk_full_spec.md`（蓝本：注入方式 + 预期行为 + DF-1~DF-4 验收指标）
  - `cmd/chaos_injector/node_ctl.go`（已有 Docker CLI 操作，参考实现风格）
  - design.md §1.2.7 batch22 磁盘满方案蓝本分析 + §2.2.2.4-2.2.2.6 InjectDiskFull/DetectDiskUsage/CleanupDiskFull 接口签名 + §2.3.2 DiskController/DiskFullEvidence 数据模型
  - spec.md §5.2.1 业务规则 1-2（REQ-DF-01：蓝本为依据 / REQ-DF-02：注入工具实现）+ §6.2 磁盘满证据数据约束
  - 约束：以蓝本为依据，不得推翻重设计（除非设计缺陷走等级二降级申报）
- **依赖**: 阶段 0 完成
- **并行**: 可与阶段 1/T3.1 并行
- **动作**:
  1. 创建 `cmd/chaos_injector/disk_ctl.go`，定义 `DiskController` 结构（含 dockerCLI 字段）
  2. 实现 `InjectDiskFull(container string, pressureLevel string) error`：
     - `docker exec <container> df -h /data/wal` 获取当前使用率
     - 计算需填充大小（目标 90% 或 100% - 当前使用率）
     - `docker exec <container> fallocate -l <size> /data/wal/fillfile` 填充
     - 验证 Use% 达到目标（DetectDiskUsage 复用）
     - 异常：fallocate 失败（权限/路径/空间不足）→ 返回 error，场景标记 BLOCKED
  3. 实现 `DetectDiskUsage(container string) (int, error)`：
     - `docker exec <container> df -h /data/wal` → 解析 Use% 列
     - 返回磁盘使用率（0-100，百分比）
  4. 实现 `CleanupDiskFull(container string) error`：
     - `docker exec <container> rm /data/wal/fillfile` → 验证 Use% 回落
     - 约束：仅删除 fillfile，不误删 WAL 日志文件（蓝本 §3 风险约束）
  5. 定义 `DiskFullEvidence` 数据结构（scenario_id/target_node/pressure_level/inject_timestamp/disk_usage_before/after/cluster_available/leader_changed/election_completion_s/survival_rate/node_crashed/recovery_timestamp/log_catchup_duration_s/status）
  6. 确认与蓝本 disk_full_design.md §1.1 架构一致（fidelity 约束）
  7. 预先确认 WAL 卷大小（蓝本 §3 风险约束：Docker volume 默认无限制，需确认或限制容器内 /data/wal 分区大小）
- **输出**: `cmd/chaos_injector/disk_ctl.go`（全新文件，InjectDiskFull + DetectDiskUsage + CleanupDiskFull + DiskFullEvidence）
- **验收**: disk_ctl.go 存在 → 三方法可用 → 与蓝本 disk_full_design.md 架构一致 → fallocate/df/rm 三命令正确 → 仅删 fillfile不误删 WAL → 编译通过
- **commit 点**: `git add cmd/chaos_injector/disk_ctl.go && git commit -m "T2.1: disk_ctl.go 注入/检测/清理三方法（蓝本 fidelity）"`
- **关联需求**: REQ-DF-01, REQ-DF-02

### T2.2 scheduler.go disk_full 场景扩展（cmd/chaos_injector/scheduler.go）
- **ID**: T2.2
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - T2.1 产出的 `disk_ctl.go`（DiskController 三方法）
  - `cmd/chaos_injector/scheduler.go:21-61`（BuildScenarioMatrix + ExecuteScenario switch 分发）
  - `cmd/chaos_injector/node_ctl.go`（KillNode/StartNode/QueryLeader/SnapshotConfirmedEntries/VerifyEntrySurvival）
  - design.md §1.2.6 chaos_injector 框架分析 + §2.2.2.7 executeDiskFull 接口签名 + §2.4.2 磁盘满注入场景（6 场景）
  - spec.md §5.2.1 业务规则 10（REQ-DF-10：场景调度扩展）+ §9.2 磁盘满注入场景矩阵
- **依赖**: T2.1
- **并行**: 不可并行（T2.3/T2.4 的基础）
- **动作**:
  1. 在 `cmd/chaos_injector/scheduler.go` `BuildScenarioMatrix` 中新增 `case "disk_full"` 分支：
     - 生成 6 场景 ID：`disk_full_soft_follower_01` / `disk_full_soft_leader_01` / `disk_full_soft_recovery_01` / `disk_full_hard_follower_01` / `disk_full_hard_leader_01` / `disk_full_hard_recovery_01`
  2. 在 `ExecuteScenario` 中新增 `case contains(scenarioID, "disk_full")` 分发到 `executeDiskFull`
  3. 实现 `executeDiskFull(scenarioID string) (*ScenarioResult, error)`：
     - 解析 scenarioID → 确定目标节点（follower/leader）+ 压力等级（soft_90/hard_100）
     - 启动 c=128 负载（recovery 场景无负载）
     - SnapshotConfirmedEntries（采样 20 条已 commit entry）
     - InjectDiskFull（fallocate 填充 WAL 卷）
     - 轮询 /raft/stats 检查集群状态 + leader 可用性 + gaps
     - VerifyEntrySurvival（比对 entry 存活率）
     - CleanupDiskFull（rm fillfile）
     - 轮询 /raft/stats 检查 gaps=0（log 追平）
     - 落盘证据 JSON
  4. 确认不修改现有 scheduler.go 逻辑（仅扩展 switch 分支）
- **输出**: `cmd/chaos_injector/scheduler.go`（扩展 disk_full 场景调度 + executeDiskFull 方法）
- **验收**: BuildScenarioMatrix 新增 disk_full 分支 → ExecuteScenario 分发到 executeDiskFull → 6 场景 ID 生成 → executeDiskFull 执行流程完整 → 不修改现有逻辑 → 编译通过
- **commit 点**: `git add cmd/chaos_injector/scheduler.go && git commit -m "T2.2: scheduler.go disk_full 场景扩展（6 场景调度）"`
- **关联需求**: REQ-DF-10

### T2.3 软满 3 场景执行 + 证据落盘（soft_90%）
- **ID**: T2.3
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - T2.2 产出的 `executeDiskFull` 方法（已实现并编译）
  - `cmd/chaos_injector/disk_ctl.go`（InjectDiskFull/DetectDiskUsage/CleanupDiskFull）
  - `cmd/chaos_injector/node_ctl.go`（QueryLeader/SnapshotConfirmedEntries/VerifyEntrySurvival）
  - design.md §2.4.2.1-2.4.2.3 软满 follower/leader/recovery 场景执行步骤
  - spec.md §5.2.1 业务规则 3/5/9（REQ-DF-03：软满 90% / REQ-DF-05：follower 满集群继续 / REQ-DF-09：恢复自愈）+ §9.2 软满场景矩阵
- **依赖**: T2.2
- **并行**: 可与 T2.4 并行
- **动作**:
  1. **disk_full_soft_follower_01**（软满 follower）：
     - 查询当前 leader，随机选择一个 follower 作为目标节点
     - 启动 c=128 压测负载
     - SnapshotConfirmedEntries(leaderID, 20) 采样已 commit entry
     - InjectDiskFull(follower, "soft_90")（fallocate 填充至 90%）
     - DetectDiskUsage(follower) 验证 Use%=90%
     - 轮询 /raft/stats 验证集群继续服务（leader 不变，读请求正常）
     - VerifyEntrySurvival 比对存活率
     - CleanupDiskFull(follower)（rm fillfile）
     - 轮询 /raft/stats 验证 gaps=0（log 追平 ≤30s）
     - 落盘 `tests/evidence/d3-batch23/disk_full_soft_follower_01.json`
  2. **disk_full_soft_leader_01**（软满 leader）：
     - 查询当前 leader 作为目标节点
     - 启动 c=128 压测负载
     - SnapshotConfirmedEntries(leaderID, 20)
     - InjectDiskFull(leader, "soft_90")
     - 轮询 /raft/stats 验证集群继续服务（90% 软满未必触发 fsync 失败）
     - VerifyEntrySurvival 比对存活率
     - CleanupDiskFull(leader)
     - 落盘 `tests/evidence/d3-batch23/disk_full_soft_leader_01.json`
  3. **disk_full_soft_recovery_01**（软满恢复）：
     - 随机选择目标节点
     - InjectDiskFull(node, "soft_90")
     - CleanupDiskFull(node)
     - 轮询 /raft/stats 验证 gaps=0（log 追平 ≤30s）
     - 落盘 `tests/evidence/d3-batch23/disk_full_soft_recovery_01.json`
  4. 每场景证据 JSON 含完整 14 字段（spec §6.2）
  5. **异常处理**：fallocate 失败 → 场景标记 BLOCKED，跳过继续后续场景
- **输出**: `tests/evidence/d3-batch23/disk_full_soft_follower_01.json` + `disk_full_soft_leader_01.json` + `disk_full_soft_recovery_01.json`
- **验收**: 3 场景证据 JSON 落盘 → 每场景含 14 字段 → 软满 Use%=90% → 集群继续服务 → 存活率 100% → 自愈 log 追平 ≤30s → 无 panic/crash
- **commit 点**: 不独立 commit（证据目录被 .gitignore 排除，与 T2.4 合并 commit）
- **关联需求**: REQ-DF-03, REQ-DF-05, REQ-DF-09, REQ-S1, REQ-S3

### T2.4 硬满 3 场景执行 + 证据落盘（hard_100%）
- **ID**: T2.4
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - T2.2 产出的 `executeDiskFull` 方法（已实现并编译）
  - T2.3 产出的软满场景执行经验（复用流程）
  - `raft.go:388-413`（StepDownForWALFailure WAL 门禁机制，硬满 leader 降级依赖）
  - design.md §2.4.2.4-2.4.2.6 硬满 follower/leader/recovery 场景执行步骤 + §2.1.3.3 pre-vote 与磁盘满协同设计
  - spec.md §5.2.1 业务规则 4/6/7/8/9（REQ-DF-04：硬满 100% / REQ-DF-06：leader 满触发选举 / REQ-DF-07：数据无损 / REQ-DF-08：无 panic / REQ-DF-09：恢复自愈）+ §9.2 硬满场景矩阵
  - 协同依赖：T1.4 产出的 pre-vote 实现（硬满 leader 降级后选举 1 轮成功 ≤2s）
- **依赖**: T2.2, T1.4（pre-vote 协同：硬满 leader 降级后选举收敛）
- **并行**: 可与 T2.3 并行
- **动作**:
  1. **disk_full_hard_follower_01**（硬满 follower）：
     - 查询当前 leader，随机选择一个 follower
     - 启动 c=128 压测负载
     - SnapshotConfirmedEntries(leaderID, 20)
     - InjectDiskFull(follower, "hard_100")
     - 轮询 /raft/stats 验证 follower 降级（walGateClosed=true）+ leader 继续服务
     - 验证集群 quorum 存活（4 节点 ≥ 3 quorum）
     - VerifyEntrySurvival 比对存活率（S1=100%）
     - 验证 follower 进程不 crash（DF-4）
     - CleanupDiskFull(follower)
     - 轮询 /raft/stats 验证 follower 自愈 + gaps=0（log 追平 ≤30s）
     - 落盘 `tests/evidence/d3-batch23/disk_full_hard_follower_01.json`
  2. **disk_full_hard_leader_01**（硬满 leader）：
     - 查询当前 leader 作为目标节点
     - 启动 c=128 压测负载
     - SnapshotConfirmedEntries(leaderID, 20)
     - InjectDiskFull(leader, "hard_100")
     - 验证 leader 降级（StepDownForWALFailure → walGateClosed=true → 停心跳）
     - 轮询 /raft/stats 验证触发选举 + 新 leader 选出 ≤2s（pre-vote 协同确保 1 轮成功）
     - VerifyEntrySurvival 比对存活率（S1=100%）
     - 验证原 leader 进程不 crash（DF-4）
     - CleanupDiskFull(原 leader)
     - 轮询 /raft/stats 验证原 leader 自愈 + gaps=0（log 追平 ≤30s）
     - 落盘 `tests/evidence/d3-batch23/disk_full_hard_leader_01.json`
  3. **disk_full_hard_recovery_01**（硬满恢复）：
     - 随机选择目标节点
     - InjectDiskFull(node, "hard_100")
     - 验证节点降级（walGateClosed=true）
     - CleanupDiskFull(node)
     - 轮询 /raft/stats 验证节点自愈 + gaps=0（log 追平 ≤30s）
     - 验证写入能力恢复
     - 落盘 `tests/evidence/d3-batch23/disk_full_hard_recovery_01.json`
  4. 每场景证据 JSON 含完整 14 字段（spec §6.2），硬满 leader 场景 election_completion_s ≤2s
  5. **异常处理**：
     - 磁盘满后集群脑裂 → 记录脑裂详情，标注 DF-1=FAIL，立即清理磁盘空间恢复集群
     - 磁盘满期间 entry 丢失 → 记录丢失 entry 详情，标注 DF-2=FAIL / S1=FAIL，触发红线
     - 磁盘满导致节点 panic/crash → 记录 crash 详情，标注 DF-4=FAIL，分析 fsync 错误处理路径
- **输出**: `tests/evidence/d3-batch23/disk_full_hard_follower_01.json` + `disk_full_hard_leader_01.json` + `disk_full_hard_recovery_01.json`
- **验收**: 3 场景证据 JSON 落盘 → 硬满 Use%=100% → follower 满集群继续 + leader 满选举 ≤2s → 存活率 100% → 无 panic/crash → 自愈 log 追平 ≤30s → walGateClosed 正确触发
- **commit 点**: `git add cmd/chaos_injector/ && git commit -m "T2.3+T2.4: 磁盘满 6 场景执行 + 证据落盘（软满+硬满）"`（注：证据 JSON 被排除，commit 仅含 chaos_injector 代码）
- **关联需求**: REQ-DF-04, REQ-DF-06, REQ-DF-07, REQ-DF-08, REQ-DF-09, REQ-S1, REQ-S3, REQ-DF-1, REQ-DF-2, REQ-DF-4

---

## 阶段 3：闭案 — 验收契约 + 判定脚本 + 报告 + 决策 + commit + tag（预估 1.75h，并行 1.25h）

> **并行策略**：T3.1（batch23.yaml）先行，可与阶段 1/2 并行；T3.2（judge_batch23.py）依赖 T3.1 + 阶段 1 + 阶段 2 全部完成；T3.3（报告 + decisions + LEDGER 销账）依赖 T3.2；T3.4（commit + tag + bundle）依赖 T3.3。
> **阶段产物**：batch23.yaml + judge_batch23.py + verdict.json + 报告.md + decisions.md + LEDGER 销账 + commit + tag v2.4-post-batch23 + bundle

### T3.1 batch23.yaml 验收契约（继承 batch22 + 新增 E4/S3/DF-1~DF-4）
- **ID**: T3.1
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - `tests/contracts/batch22.yaml`（F1-F5 + E1-E3 + S1-S2 验收线，继承）
  - design.md §2.2.2.9 batch23.yaml 接口签名 + §2.5.1 验收项判定逻辑 + §2.5.3 验收项与蓝本 DF 指标映射
  - spec.md §5.3 验收契约 E1-E4/S1-S3 + §7 验收契约草案 + §8 红线清单 + §9 场景矩阵（20 场景）+ §6.3 验收契约数据约束
- **依赖**: 阶段 0 完成
- **并行**: 可与阶段 1/2 并行（纯 YAML 定义，不依赖实现）
- **动作**:
  1. 创建 `tests/contracts/batch23.yaml`
  2. 定义契约元信息：`contract.name: batch23-prevote-diskfull` / `contract.version: "1.0"`
  3. 继承 batch22 全部验收线：F1-F5 + E1-E3 + S1-S2（阈值不变）
  4. 新增 E4 验收线：
     - `name: cascading_election_convergence` / `metric: max_cascading_election_time` / `threshold: 2.0` / `unit: seconds` / `operator: "<="` / `scope: cascading_scenarios`
     - `improvement: "batch22 cascading max=3.44s → batch23 ≤ 2.0s"`
  5. 新增 S3 验收线：
     - `name: disk_full_self_healing` / `metric: max_log_catchup_duration` / `threshold: 30` / `unit: seconds` / `operator: "<="` / `scope: disk_full_scenarios`
  6. 新增 DF-1~DF-4 磁盘满专项指标：
     - DF-1: `disk_full_cluster_available`（follower 满集群继续 + leader 满选举 ≤2s）
     - DF-2: `disk_full_data_preserved`（survival_rate == 100%）
     - DF-3: `disk_full_recovery_catchup`（log 追平 ≤ 30s）
     - DF-4: `disk_full_no_crash`（all(node_crashed == false)）
  7. 定义场景矩阵（20 场景：pre-vote 14 + 磁盘满 6，见 spec §9）
  8. 定义红线清单（RL-01~RL-11 继承）
  9. 定义时间盒（`timebox_hours: 6`，`on_expire: stop_and_hand_over`）
  10. 格式与既有 batch22.yaml 契约一致（spec §4.5 兼容性）
- **输出**: `tests/contracts/batch23.yaml`
- **验收**: YAML 格式合法（`python -c "import yaml; yaml.safe_load(open('tests/contracts/batch23.yaml'))"` 通过）→ E1-E4/S1-S3 阈值齐全 → DF-1~DF-4 定义 → F1-F5 继承 → 场景矩阵 20 场景 → 红线 11 条 → 时间盒 6h
- **commit 点**: `git add tests/contracts/batch23.yaml && git commit -m "T3.1: batch23 验收契约 YAML（E1-E4/S1-S3 + DF-1~DF-4 + 20 场景）"`
- **关联需求**: REQ-E1, REQ-E2, REQ-E3, REQ-E4, REQ-S1, REQ-S2, REQ-S3, REQ-DF-1, REQ-DF-2, REQ-DF-3, REQ-DF-4, REQ-YAML

### T3.2 judge_batch23.py 判定脚本（复用 batch22 + 新增 E4/S3/DF 判定）
- **ID**: T3.2
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - T3.1 产出的 `tests/contracts/batch23.yaml`（E1-E4/S1-S3/DF-1~DF-4 阈值）
  - T1.5 产出的 `tests/evidence/d3-batch23/prevote_forensics.json` + 14 场景证据 JSON
  - T2.3/T2.4 产出的 `tests/evidence/d3-batch23/disk_full_*.json`（6 场景证据）
  - `tests/contracts/judge_batch22.py`（复用 F1-F5 + E1-E3 + S1-S2 判定逻辑）
  - design.md §2.5.2 判定脚本执行流程 + §2.5.1 验收项判定逻辑
  - spec.md §5.3.1 业务规则 8-9（REQ-JUDGE-01：判定来源约束 / REQ-JUDGE-02：逐字段对照）+ §6 数据约束
- **依赖**: T3.1, T1.5, T2.3, T2.4（全部验收证据落盘）
- **并行**: 不可并行（T3.3 的基础）
- **动作**:
  1. 创建 `tests/contracts/judge_batch23.py`
  2. 复用 judge_batch22.py 的 `judge_f1`~`judge_f5` / `judge_e1`~`judge_e3` / `judge_s1` / `judge_s2` 判定逻辑（继承 batch22）
  3. 新增 `judge_e4(evidence_list, threshold)`：读 prevote_forensics.json cascading 选举完成时间，max(cascading_election_completion) ≤ 2.0s
  4. 新增 `judge_s3(evidence_list, threshold)`：读 disk_full_*.json log_catchup_duration_s，max(log_catchup_duration) ≤ 30s
  5. 新增 `judge_df1(evidence_list)`：联合判定（follower 满集群继续 + leader 满选举 ≤2s）
  6. 新增 `judge_df2(evidence_list, expected_rate)`：min(survival_rate) == 100%
  7. 新增 `judge_df3(evidence_list, threshold)`：max(log_catchup_duration) ≤ 30s
  8. 新增 `judge_df4(evidence_list)`：all(node_crashed == false)
  9. 实现 `judge_overall(e1~e4, s1~s3, f1~f5, df1~df4)`：全 PASS → PASS；有 FAIL → FAIL；部分 PASS 无 FAIL → PARTIAL
  10. 逐字段对照证据 JSON vs YAML 阈值，输出 `tests/evidence/d3-batch23/verdict.json`（含 E1-E4/S1-S3/F1-F5/DF-1~DF-4 判定 + overall + timestamp）
  11. CLI：`--contract` / `--evidence-dir` / `--output`
  12. 异常处理：YAML 格式错误 → 退出码 2；证据缺字段 → 标记 INSUFFICIENT_EVIDENCE，不阻止其余项
  13. 执行脚本产出 verdict.json：`python tests/contracts/judge_batch23.py --contract=tests/contracts/batch23.yaml --evidence-dir=tests/evidence/d3-batch23/ --output=tests/evidence/d3-batch23/verdict.json`
- **输出**: `tests/contracts/judge_batch23.py` + `tests/evidence/d3-batch23/verdict.json`
- **验收**: 脚本可读 YAML + JSON → 产出 verdict.json 含 E1-E4/S1-S3/F1-F5/DF-1~DF-4 判定 + overall → 逐字段对照（红线 RL-04）→ 助手只引用判定文件不得自写 PASS/FAIL（红线 RL-09）
- **commit 点**: `git add tests/contracts/judge_batch23.py && git commit -m "T3.2: batch23 判定脚本 + verdict.json 产出"`
- **关联需求**: REQ-JUDGE-01, REQ-JUDGE-02, RL-04, RL-09

### T3.3 报告.md + decisions.md + LEDGER 销账（首屏三清单 + batch22→batch23 对照）
- **ID**: T3.3
- **状态**: [ ] 未开始
- **预估**: 30 分钟
- **输入**:
  - T3.2 产出的 `verdict.json`（E1-E4/S1-S3/F1-F5/DF-1~DF-4 判定 + overall）
  - T1.5 产出的 `prevote_forensics.json`（pre-vote 取证：轮数/选票分布/term 膨胀）
  - T2.3/T2.4 产出的 `disk_full_*.json`（6 场景磁盘满证据）
  - `LEDGER.md`（L-22-1/L-22-2 待清记录）
  - batch22 基线：`tests/evidence/d3-batch22/verdict.json`（E1=1.6003s, cascading max=3.1928s）+ `election_forensics.json`（pre_vote.status=not_implemented）
  - design.md §2.9.3 预期收益模型 + §2.9.1 pre-vote 安全性论证 + §2.9.2 磁盘满数据完整性论证 + §2.8 与现有机制关系分析
  - spec.md §1.3 核心输出 8（报告三件套：首屏三清单）+ §5.3 验收契约
- **依赖**: T3.2
- **并行**: 不可并行（T3.4 的基础）
- **动作**:
  1. 创建 `tests/evidence/d3-batch23/报告.md`，构建首屏三清单：
     - **绕行清单**：已绕过的问题及原因（如 batch22 pre-vote 绕行 → 本批实现修复；batch22 磁盘满仅设计 → 本批实施落地）
     - **降级清单**：降级运行的功能及影响（如无）
     - **销账清单**：LEDGER 挂账清偿状态（DEBT-0001~0003 + L-21-1/L-21-2 已清偿 + L-22-1/L-22-2 本批清偿）
  2. 首屏贴齐 batch22→batch23 对照数据（必贴）：
     - E1 选举完成中位：batch22 1.6003s → batch23 实测值（不劣化）
     - E4 cascading 选举 max：batch22 3.44s（离群值 2.78/3.19s）→ batch23 ≤2s（-42%）
     - 选票分裂轮数：batch22 多轮 → batch23 1 轮（pre-vote 收敛）
     - term 膨胀：batch22 cascading term+多 → batch23 term+1~2（膨胀抑制）
     - DF-1 集群可用性：batch22 未测试 → batch23 follower 满集群继续 + leader 满选举 ≤2s
     - S3 磁盘满自愈：batch22 未测试 → batch23 log 追平 ≤30s
  3. 写入判定结果章节：引用 verdict.json 内容（E1-E4/S1-S3/F1-F5/DF-1~DF-4 判定 + overall），不自行判定（RL-09）
  4. 写入证据索引章节：列出所有证据 JSON 文件路径
  5. 根据 overall 判定执行三等级处置（spec §5.3）：
     - PASS → 闭案处置（签发闭案决议，T3.4 执行）
     - PARTIAL → 挂账处置（未通过项挂账至 batch24，记录根因初判，不闭案）
     - FAIL → 红线处置（触发红线告警，记录失败详情，须晨批审议）
  6. 创建 `tests/evidence/d3-batch23/decisions.md`，落盘决策：
     - **pre-vote 实现决策**：handleElectionTimeout 入口注入 pre-vote 探测，理由：减少选票分裂，不增加 term，不引入活锁
     - **pre-vote 安全性论证**：不增加 term / 不更新 votedFor / 不改变 state / 正式选举仍遵循 Raft 约束 / 失败不阻止其他节点（design.md §2.9.1）
     - **磁盘满蓝本 fidelity 决策**：以 batch22 disk_full_spec.md + disk_full_design.md 为蓝本，不推翻重设计
     - **磁盘满数据完整性论证**：已 commit entry 已持久化在 quorum 节点 / 磁盘满仅影响单节点 / quorum 存活 / 新 leader 有完整日志 / 清理后 log 追平（design.md §2.9.2）
     - **pre-vote 与磁盘满协同决策**：pre-vote 使磁盘满 leader 降级后选举 1 轮成功 ≤2s（design.md §2.1.3.3）
     - **预期收益模型**：E4 cascading 3.44s→≤2s（-42%）/ 选票分裂消除 / term 膨胀抑制（design.md §2.9.3）
  7. 回填 LEDGER.md 中 L-22-1/L-22-2 的 cleared_at（标注本批 commit SHA，状态改为"已清偿"）
  8. 在 LEDGER.md "已清偿记录详情"章节追加 L-22-1/L-22-2 的清偿记录详情
- **输出**: `tests/evidence/d3-batch23/报告.md`（首屏三清单 + 对照数据 + 判定结果）+ `tests/evidence/d3-batch23/decisions.md`（决策落盘）+ `LEDGER.md`（L-22-1/L-22-2 清偿）
- **验收**: 报告.md 首屏 → 三清单齐全（绕行/降级/销账）→ batch22→batch23 对照数据在首屏可见 → 判定结果引用 verdict.json → decisions.md 含 pre-vote/磁盘满/协同/收益 四项决策 → LEDGER.md L-22-1/L-22-2 已清偿
- **commit 点**: `git add LEDGER.md && git commit -m "T3.3: 报告.md + decisions.md + LEDGER L-22-1/L-22-2 销账"`（注：证据目录被 .gitignore 排除，实际 commit 仅含 LEDGER.md）
- **关联需求**: REQ-DISP-PASS, REQ-DISP-PARTIAL, REQ-DISP-FAIL, REQ-T0-01, REQ-T0-02

### T3.4 commit + tag v2.4-post-batch23 + bundle
- **ID**: T3.4
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**:
  - T3.2 产出的 `verdict.json`（overall 判定）
  - T3.3 产出的 `报告.md` + `decisions.md` + LEDGER.md（L-22-1/L-22-2 已清偿）
  - 所有阶段产物（raft.go / raft_transport.go / collector.go / scheduler.go / disk_ctl.go / batch23.yaml / judge_batch23.py / LEDGER.md）
  - spec.md §5.3 三等级处置协议
- **依赖**: T3.3
- **并行**: 不可并行（闭案收尾）
- **动作**:
  1. 根据 verdict.json 的 overall 判定执行对应处置：
     - **overall=PASS** → 闭案处置：
       - 签发闭案决议（在报告.md 中标注"闭案决议已签发"）
       - `git add -A`（仅代码文件，证据目录被 .gitignore 排除 RL-08，构建产物排除 RL-11）
       - `git commit -m "D3-batch23-prevote-diskfull: pre-vote 防选票分裂 + 磁盘满故障注入实施闭案"`
       - `git tag v2.4-post-batch23`
       - bundle 备份：`git bundle create ../bundle/v2.4-post-batch23.bundle --all`
     - **overall=PARTIAL** → 挂账处置：
       - 将未通过项挂账至 batch24（在 LEDGER.md 中新增 L-23-X 待清记录）
       - 记录根因初判（在 decisions.md 中）
       - 不闭案，不打 tag
       - `git commit -m "D3-batch23-partial: 部分验收未通过，挂账 batch24"`
     - **overall=FAIL** → 红线处置：
       - 触发红线告警（在报告.md 中标注"红线告警"）
       - 记录失败详情（在 decisions.md 中）
       - 不闭案，不打 tag，须晨批审议
       - `git commit -m "D3-batch23-fail: 验收 FAIL，须晨批审议"`
  2. 更新 RESUME.md：标注所有任务已完成，session 结束
- **输出**: commit `D3-batch23-prevote-diskfull` + tag `v2.4-post-batch23` + bundle 备份（若 PASS）
- **验收**: overall=PASS → commit + tag + bundle 完成 → RESUME.md 标注 session 结束 → git log 可见 commit → git tag 可见 v2.4-post-batch23 → git status 干净（无证据/构建产物）
- **commit 点**: 本任务即 commit 点
- **关联需求**: REQ-DISP-PASS, REQ-DISP-PARTIAL, REQ-DISP-FAIL, REQ-DISP-TIMEBOX, RL-08, RL-11

---

## 阶段 4：收尾 — 红线自查 + 集群恢复（预估 0.5h，并行 0.25h）

> **并行策略**：T4.1（红线自查）/ T4.2（集群恢复）互不依赖，可两任务并行。
> **阶段产物**：红线自查结论 + 集群健康稳态 + WAL 卷清理

### T4.1 红线自查
- **ID**: T4.1
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**:
  - spec.md §8 红线清单（RL-01~RL-11）
  - 所有阶段产物
  - verdict.json
- **依赖**: T3.4
- **并行**: 可与 T4.2 并行
- **动作**:
  1. **RL-01 fsync 语义**：确认 pre-vote/磁盘满实施未触碰 WAL 写入路径 / fsync 调用 → ✅
  2. **RL-02 quorum 语义**：确认 quorum 计算（3/5）未修改 → ✅
  3. **RL-03 选举超时语义**：确认 pre-vote 不改 electionTimeout（800-1200ms 不变）、800ms > rpcTimeout(500ms) → ✅
  4. **RL-04 PASS 判定逐字段对照**：确认 judge_batch23.py 逐字段比对证据 vs YAML → ✅
  5. **RL-05 禁调参刷数**：确认 batch23.yaml 阈值未自行放宽，实测数据如实记录 → ✅
  6. **RL-06 pipeline 正确性**：确认 pre-vote/磁盘满不侵入写路径热区 → ✅
  7. **RL-07 禁改 proto**：确认 PreVote RPC 通过路径区分，不修改 proto 定义 → ✅
  8. **RL-08 证据目录被 .gitignore 排除**：确认 `git status` 不显示证据目录文件 → ✅
  9. **RL-09 助手不得自写 PASS/FAIL**：确认报告.md 中判定结果引用 verdict.json，未自写 → ✅
  10. **RL-10 挂账到期未清**：确认 LEDGER.md 中 L-22-1/L-22-2 已清偿 → ✅
  11. **RL-11 构建产物禁止入库**：确认 `git status` 干净（无 .exe / build/ 相关条目，disk_ctl.go 编译产物不入库）→ ✅
  12. 落盘红线自查结论至 `tests/evidence/d3-batch23/redline_check.json`
- **输出**: `tests/evidence/d3-batch23/redline_check.json`（11 条红线自查结论）
- **验收**: 11 条红线全部 ✅ → 自查结论落盘
- **commit 点**: 不独立 commit（自查结论落盘证据目录，被 .gitignore 排除）
- **关联需求**: RL-01~RL-11

### T4.2 集群恢复 + WAL 卷清理
- **ID**: T4.2
- **状态**: [ ] 未开始
- **预估**: 15 分钟
- **输入**:
  - 5 节点 Docker 集群（可能因测试期间 kill leader / 磁盘满注入处于非稳态）
  - `cmd/chaos_injector/node_ctl.go`（RestartNode / WaitNodeHealthy 方法）
  - `cmd/chaos_injector/disk_ctl.go`（CleanupDiskFull 方法，清理残留 fillfile）
- **依赖**: T3.4
- **并行**: 可与 T4.1 并行
- **动作**:
  1. 检查 5 节点集群状态：遍历 `/raft/stats` 确认每节点 state
  2. 若有节点被 kill 未恢复：`docker start raft-node-N` 重启
  3. 清理所有节点 WAL 卷残留 fillfile：遍历 5 节点执行 `CleanupDiskFull(node)`（确保磁盘满注入后空间已清理）
  4. 等待所有节点健康：轮询 `/health/live` 返回 200
  5. 确认集群恢复到健康稳态：1 leader + 4 follower
  6. 确认所有 WAL 卷空间已清理干净（df -h /data/wal 使用率回落至正常水平）
  7. 记录集群最终状态至 `tests/evidence/d3-batch23/cluster_final_state.json`
- **输出**: `tests/evidence/d3-batch23/cluster_final_state.json`（集群最终状态）
- **验收**: 5 节点全部健康 → 1 leader + 4 follower → 集群可正常读写 → WAL 卷空间清理干净 → 无残留 fillfile
- **commit 点**: 不独立 commit（最终状态落盘证据目录）
- **关联需求**: spec §4.2 可靠性 3（集群可恢复）

---

## 需求追溯矩阵

### 任务 → 需求 ID 映射

| 任务 ID | 关联需求 ID | 需求描述 | EARS 类型 | 验收线 |
|---------|------------|----------|-----------|--------|
| T0.1 | REQ-T0-01 | LEDGER L-22-1/L-22-2 入账 | Ubiquitous | 两笔入账 + 格式合规 |
| T0.1 | REQ-T0-02 | git bundle + tag 打点 | Ubiquitous | v2.4-pre-batch23 可见 |
| T1.1 | REQ-PV-01 | PreVote RPC 传输层扩展 | Event-driven | /raft/pre_vote 端点可用 |
| T1.1 | RL-07 | 禁改 proto 定义 | 红线 | 通过路径区分不修改 proto |
| T1.2 | REQ-PV-04 | pre-vote 不增加 term | Ubiquitous | HandlePreVote 不更新 term |
| T1.2 | REQ-PV-05 | pre-vote 失败不阻止其他节点 | Ubiquitous | 不引入活锁 |
| T1.2 | REQ-PV-06 | pre-vote 不破坏 Raft 安全性 | Ubiquitous | 不更新 votedFor/state |
| T1.3 | REQ-PV-02 | 获多数派转正式选举 | State-driven | ≥quorum 转 Candidate |
| T1.3 | REQ-PV-03 | 未获多数派保持 Follower | State-driven | <quorum 保持 Follower |
| T1.4 | REQ-PV-01 | pre-vote 探测先行 | Event-driven | handleElectionTimeout 注入 |
| T1.4 | REQ-PV-04 | pre-vote 不增加 term | Ubiquitous | 探测阶段 term 不变 |
| T1.4 | REQ-PV-07 | cascading 选举时间收敛 | Event-driven | cascading 每次 ≤2s |
| T1.4 | REQ-PV-08 | 选举完成不劣化 | State-driven | E1 ≤2s 不劣化 |
| T1.4 | RL-03 | 选举超时语义不可破坏 | 红线 | 800-1200ms 不变 |
| T1.5 | REQ-PV-07 | cascading 选举收敛验证 | Event-driven | E4 验收 |
| T1.5 | REQ-PV-09 | pre-vote 取证数据落盘 | Event-driven | prevote_forensics.json |
| T1.5 | REQ-E1 | E1 选举完成 ≤2s | Event-driven | E1 验收 |
| T1.5 | REQ-E2 | E2 无脑裂 | State-driven | E2 验收 |
| T1.5 | REQ-E4 | E4 cascading ≤2s | Event-driven | E4 验收 |
| T2.1 | REQ-DF-01 | 以蓝本为实施依据 | Ubiquitous | fidelity 约束 |
| T2.1 | REQ-DF-02 | 磁盘满注入工具实现 | Ubiquitous | 三方法可用 |
| T2.2 | REQ-DF-10 | 磁盘满场景调度扩展 | Ubiquitous | 6 场景调度 |
| T2.3 | REQ-DF-03 | 软满 90% 压力等级 | Event-driven | Use%=90% |
| T2.3 | REQ-DF-05 | follower 满集群继续服务 | State-driven | leader 继续服务 |
| T2.3 | REQ-DF-09 | 磁盘空间恢复后自愈 | Event-driven | log 追平 ≤30s |
| T2.3 | REQ-S1 | S1 存活率 100% | Event-driven | S1 验收 |
| T2.3 | REQ-S3 | S3 磁盘满自愈 | Event-driven | S3 验收 |
| T2.4 | REQ-DF-04 | 硬满 100% 压力等级 | Event-driven | Use%=100% |
| T2.4 | REQ-DF-06 | leader 满触发选举 | State-driven | 选举 ≤2s |
| T2.4 | REQ-DF-07 | 磁盘满期间数据无损 | Ubiquitous | survival_rate=100% |
| T2.4 | REQ-DF-08 | 磁盘满无 panic/crash | Ubiquitous | node_crashed=false |
| T2.4 | REQ-DF-09 | 磁盘空间恢复后自愈 | Event-driven | log 追平 ≤30s |
| T2.4 | REQ-DF-1 | DF-1 集群可用性 | 联合判定 | follower 满+leader 满 |
| T2.4 | REQ-DF-2 | DF-2 数据无损 | Ubiquitous | survival=100% |
| T2.4 | REQ-DF-4 | DF-4 无 crash | Ubiquitous | node_crashed=false |
| T3.1 | REQ-E1~E4 | E1-E4 验收线定义 | Ubiquitous | YAML 阈值齐全 |
| T3.1 | REQ-S1~S3 | S1-S3 验收线定义 | Ubiquitous | YAML 阈值齐全 |
| T3.1 | REQ-DF-1~4 | DF-1~DF-4 指标定义 | Ubiquitous | YAML 定义 |
| T3.1 | REQ-YAML | YAML 变更须晨批 | Unwanted | 阈值未自行放宽 |
| T3.2 | REQ-JUDGE-01 | 判定由脚本产出 | Ubiquitous | judge_batch23.py 产出 verdict.json |
| T3.2 | REQ-JUDGE-02 | 逐字段对照 YAML | Ubiquitous | 无跳过 |
| T3.2 | RL-04 | PASS 判定逐字段对照 | 红线 | 逐字段比对 |
| T3.2 | RL-09 | 助手不得自写 PASS/FAIL | 红线 | 助手只引用 verdict.json |
| T3.3 | REQ-DISP-PASS | PASS 等级处置 | Event-driven | 闭案 + tag + commit |
| T3.3 | REQ-DISP-PARTIAL | PARTIAL 等级处置 | Event-driven | 挂账 batch24 |
| T3.3 | REQ-DISP-FAIL | FAIL 等级处置 | Event-driven | 红线告警 |
| T3.3 | REQ-T0-01 | L-22-1 清偿 | Event-driven | cleared_at 回填 |
| T3.3 | REQ-T0-02 | L-22-2 清偿 | Event-driven | cleared_at 回填 |
| T3.4 | REQ-DISP-PASS | PASS 闭案处置 | Event-driven | commit + tag + bundle |
| T3.4 | REQ-DISP-TIMEBOX | 时间盒到期处置 | Event-driven | 到点停手交数据 |
| T3.4 | RL-08 | 证据目录被 .gitignore 排除 | 红线 | git add 仅代码文件 |
| T3.4 | RL-11 | 构建产物禁止入库 | 红线 | git status 干净 |
| T4.1 | RL-01~RL-11 | 红线自查 | 红线 | 11 条红线全 ✅ |
| T4.2 | spec §4.2.3 | 集群可恢复 | 可靠性 | 1 leader + 4 follower |

### 需求覆盖统计

| 需求类别 | 需求数 | 覆盖任务 | 覆盖率 |
|---------|--------|---------|--------|
| 任务零前置（REQ-T0-01~02） | 2 | T0.1/T3.3 | 100% |
| pre-vote 探测（REQ-PV-01~09） | 9 | T1.1/T1.2/T1.3/T1.4/T1.5 | 100% |
| 磁盘满注入（REQ-DF-01~10） | 10 | T2.1/T2.2/T2.3/T2.4 | 100% |
| 验收契约（REQ-E1~E4/S1~S3/DF-1~4） | 11 | T3.1/T3.2/T1.5/T2.3/T2.4 | 100% |
| 判定约束（REQ-JUDGE-01~02/YAML） | 3 | T3.1/T3.2 | 100% |
| 三等级处置（REQ-DISP-*） | 5 | T3.3/T3.4 | 100% |
| 红线（RL-01~RL-11） | 11 | T1.1/T1.4/T3.2/T3.4/T4.1 | 100% |
| **合计** | **51** | **16 任务** | **100%** |

---

## 任务依赖图

```
阶段 0                    阶段 1（pre-vote，并行）         阶段 2（磁盘满，并行）       阶段 3（闭案）
┌─ T0.1 ─┐               ┌─ T1.1 ─┐                     ┌─ T2.1 ─┐                 ┌─ T3.1 ─┐
└────────┘               ├─ T1.2 ─┤                     ├─ T2.2 ─┤                 └────────┘
                          └───┬────┘                     └───┬────┘                     │
                              ▼                              ▼                          ▼
                          ┌─ T1.3 ─┐                     ┌─ T2.3 ─┐                 ┌─ T3.2 ─┐
                          └────────┘                     ├─ T2.4 ─┤                 └────────┘
                              ▼                              └───┬────┘                     │
                          ┌─ T1.4 ─┐                           │                          ▼
                          └────────┘                           │                      ┌─ T3.3 ─┐
                              ▼                                │                      └────────┘
                          ┌─ T1.5 ─┐                           │                          ▼
                          └────────┘                           │                      ┌─ T3.4 ─┐
                              │                                │                      └────────┘
                              └──────────────┬────────────────┘                          │
                                             ▼                                          ▼
                                     ┌─ T3.2 依赖 ─┐                              阶段 4（收尾）
                                     └──────────────┘                             ┌─ T4.1 ─┐
                                                                                   ├─ T4.2 ─┤
                                                                                   └────────┘
```

**依赖关系**：
- T0.1 → 无依赖，必须先行
- T1.1/T1.2 → 依赖阶段 0 完成，可两任务并行
- T1.3 → 依赖 T1.1 + T1.2
- T1.4 → 依赖 T1.3
- T1.5 → 依赖 T1.4
- T2.1 → 依赖阶段 0 完成（与 T1.x 可并行）
- T2.2 → 依赖 T2.1
- T2.3/T2.4 → 依赖 T2.2（T2.4 还依赖 T1.4 pre-vote 协同），可两任务并行
- T3.1 → 依赖阶段 0 完成（与 T1.x/T2.x 可并行）
- T3.2 → 依赖 T3.1 + T1.5 + T2.3 + T2.4（全部验收证据落盘）
- T3.3 → 依赖 T3.2
- T3.4 → 依赖 T3.3
- T4.1/T4.2 → 依赖 T3.4，可两任务并行

---

## 时间盒汇总

| 阶段 | 任务数 | 串行预估 | 并行预估 | 关键产物 |
|------|--------|----------|----------|----------|
| 阶段 0 任务零前置 | 1 | 0.50h | 0.50h | git bundle + tag v2.4-pre-batch23 + LEDGER L-22-1/L-22-2 入账 |
| 阶段 1 pre-vote 实现 | 5 | 2.50h | 2.00h | raft_transport.go PreVote RPC + raft.go pre-vote 状态机 + prevote_forensics.json + 14 场景证据 |
| 阶段 2 磁盘满注入 | 4 | 2.00h | 1.50h | disk_ctl.go + scheduler.go 扩展 + 6 场景证据 JSON |
| 阶段 3 闭案 | 4 | 1.75h | 1.25h | batch23.yaml + judge_batch23.py + verdict.json + 报告.md + decisions.md + commit/tag |
| 阶段 4 收尾 | 2 | 0.50h | 0.25h | 红线自查 + 集群恢复 + WAL 卷清理 |
| **合计** | **16** | **7.25h** | **4.00h** | **≤ 6h ✅** |

> **并行后总预估：4.00h ≤ 6h，满足时间盒约束（红线 RL-10）**。预留 2.00h 作为集群部署/编译/重测/WAL 卷确认的缓冲时间。
> **关键路径**：T0.1 → T1.1/T1.2 → T1.3 → T1.4 → T1.5 → T3.2 → T3.3 → T3.4 → T4.1/T4.2
> **并行机会**：阶段 1（pre-vote）与阶段 2（磁盘满）与 T3.1（batch23.yaml）无依赖，可三路并行开发。磁盘满蓝本已就绪，disk_ctl.go 可立即开始。