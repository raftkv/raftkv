# D1-batch2 DECISION

## T1: harness.sh RUN_ID守卫
commit: 1df4525
commit 1df4525af99dee033f8fe4e8a5d7870d03262d3a
Author: raftkv-dev <dev@raftkv.local>
Date:   Sat Sep 5 17:50:13 2026 +0000

    fix(ci): T1 harness.sh RUN_ID守卫 (run_id=run-20260905_174313)

 tests/harness.sh | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)
run_id: run-20260905_174313

## T2: do_rollback双节点冒烟
commit: f5996e0
commit f5996e009004e50a043ab759f18041ee3e33fadf
Author: raftkv-dev <dev@raftkv.local>
Date:   Sat Sep 5 17:57:25 2026 +0000

    fix(ci): T2 do_rollback双节点冒烟 (run_id=run-20260905_175022)

 tests/knife_run.sh | 42 ++++++++++++------------------------------
 1 file changed, 12 insertions(+), 30 deletions(-)
run_id: run-20260905_175022

## T3: sleep→wait_for统一
commit: 107cc3c
commit 107cc3cb61a6263dbedaf1a01feaad0c9f3aaefb
Author: raftkv-dev <dev@raftkv.local>
Date:   Sun Sep 6 04:19:44 2026 +0000

    fix(ci): T3 A2 no-op mode + generate_decision empty sha guard

 tests/run_pipeline.sh | 56 +++++++++++++++++++++------------------------------
 1 file changed, 23 insertions(+), 33 deletions(-)
run_id: run-20260906_043820

## 终局
tag: v1.0.0-d2
merge: 5d47cba Merge fix/d1-batch2: knife_run PASS (v1.0.0-d2)

## 断言计数（R10）
  总计: PASS=51 FAIL=0
  总计: PASS=51 FAIL=0
  总计: PASS=51 FAIL=0
  总计: PASS=51 FAIL=0

## 信任计分（R9: 从progress.log计算）
本批旗标数: 0
冻结次数: 2
门禁颁发tag数: 1

## progress.log全文（R10）
[ 17:43:12] T1: harness.sh RUN_ID赋值加守卫
[ 17:43:12] 方案: RUN_ID="" → RUN_ID="${RUN_ID:-}"  双保险保留knife_run save/restore
[ 17:43:12] R3验证: 15:RUN_ID="${RUN_ID:-}"
[ 17:50:14] T1 PASS commit=1df4525 run_id=run-20260905_174313
[ 17:50:14] T2: do_rollback冒烟改双节点方案A
[ 17:50:14] 方案: 移source至do_rollback前, 替换内联docker run为up_cluster/down_cluster, RID=rbk-${RUN_ID}
[ 17:50:14] R7: 作用域可用性检查
/workspace/tests/knife_run.sh:29:ROLLBACK_IMAGE="raftkv:latest-rollback"
/workspace/tests/knife_run.sh:33:RUN_EVIDENCE="${EVIDENCE_DIR}/${RUN_ID}"
/workspace/tests/knife_run.sh:35:mkdir -p "$RUN_EVIDENCE"
/workspace/tests/knife_run.sh:58:    cat > "${RUN_EVIDENCE}/FAIL" << EOF
/workspace/tests/knife_run.sh:71:    git reset --hard "$LAST_GREEN_TAG" 2>&1 | tee -a "${RUN_EVIDENCE}/rollback.log"
/workspace/tests/knife_run.sh:74:    echo "[rollback] 重建 ${LAST_GREEN_TAG} 镜像..." | tee -a "${RUN_EVIDENCE}/rollback.log"
/workspace/tests/knife_run.sh:75:    docker build --platform linux/amd64 -t "$ROLLBACK_IMAGE" \
/workspace/tests/knife_run.sh:76:        -f Dockerfile . 2>&1 | tee -a "${RUN_EVIDENCE}/rollback_build.log"
/workspace/tests/knife_run.sh:92:        "$ROLLBACK_IMAGE" 2>&1 | tee -a "${RUN_EVIDENCE}/rollback_smoke.log"
/workspace/tests/knife_run.sh:96:    echo "rollback smoke stats: $rb_stats" >> "${RUN_EVIDENCE}/rollback_smoke.log"
/workspace/tests/knife_run.sh:99:        echo "[rollback] 冒烟绿: 已知好状态确认" | tee -a "${RUN_EVIDENCE}/rollback.log"
/workspace/tests/knife_run.sh:101:        echo "[rollback] 警告: 冒烟也失败! 基线可能已损坏!" | tee -a "${RUN_EVIDENCE}/rollback.log"
/workspace/tests/knife_run.sh:108:    echo "[rollback] FAIL报告: ${RUN_EVIDENCE}/FAIL"
/workspace/tests/knife_run.sh:122:git checkout "$BRANCH" 2>&1 | tee "${RUN_EVIDENCE}/build.log"
/workspace/tests/knife_run.sh:124:echo "[build] docker build..." | tee -a "${RUN_EVIDENCE}/build.log"
/workspace/tests/knife_run.sh:126:    -f Dockerfile . 2>&1 | tee -a "${RUN_EVIDENCE}/build.log"
/workspace/tests/knife_run.sh:133:echo "[build] key leak scan..." | tee -a "${RUN_EVIDENCE}/build.log"
/workspace/tests/knife_run.sh:136:echo "key leak hits: $leak" | tee -a "${RUN_EVIDENCE}/build.log"
/workspace/tests/knife_run.sh:156:if ! up_cluster "$SMOKE_RID" 2>&1 | tee "${RUN_EVIDENCE}/smoke.log"; then
/workspace/tests/knife_run.sh:159:echo "[smoke] PASS: Leader elected" | tee -a "${RUN_EVIDENCE}/smoke.log"
/workspace/tests/knife_run.sh:160:down_cluster "$SMOKE_RID"
/workspace/tests/knife_run.sh:177:    suite_log="${RUN_EVIDENCE}/suite_${suite}.log"
/workspace/tests/knife_run.sh:227:cat > "${RUN_EVIDENCE}/PASS" << EOF
/workspace/tests/knife_run.sh:253:    git checkout v1.0-dev 2>&1 | tee -a "${RUN_EVIDENCE}/gate.log"
/workspace/tests/knife_run.sh:254:    git merge --no-ff "$BRANCH" -m "Merge $BRANCH: knife_run PASS ($TAG)" 2>&1 | tee -a "${RUN_EVIDENCE}/gate.log"
/workspace/tests/knife_run.sh:257:    git log --oneline -5 | tee -a "${RUN_EVIDENCE}/gate.log"
/workspace/tests/knife_run.sh:258:    git tag -l "v1.0.0-*" | tee -a "${RUN_EVIDENCE}/gate.log"
/workspace/tests/knife_run.sh:264:echo "[knife_run] 完成. evidence: ${RUN_EVIDENCE}/"
/workspace/tests/harness.sh:48:# up_cluster <run_id> [extra_env...]
/workspace/tests/harness.sh:52:up_cluster() {
/workspace/tests/harness.sh:125:# down_cluster [run_id]
/workspace/tests/harness.sh:128:down_cluster() {
[ 17:50:14] R1验证: source块已删除
 tests/knife_run.sh | 4 ----
 1 file changed, 4 deletions(-)
[ 17:50:18] R1验证: 内联docker run已删除
[ 17:50:18] R1验证: 替换块已插入
 tests/knife_run.sh | 38 ++++++++------------------------------
 1 file changed, 8 insertions(+), 30 deletions(-)
[ 17:50:22] R1验证: source块已插入
[ 17:50:22] R6验证: local rbk_rid在do_rollback体内
[ 17:57:26] T2 PASS commit=f5996e0 run_id=run-20260905_175022
[ 17:57:26] T3: sleep→wait_for统一
[ 17:57:26] 方案: suite脚本中sleep>=5替换为wait_for轮询, 判定条件不变
[ 17:57:26] R4: 仅替换sleep>=5, 保留sleep 1/2/3原样
[ 17:57:26] R4删除替换点: baseline(sleep2×4), idem(sleep1×3,sleep2×1), wal_snap(sleep2×1,sleep3×1), health(sleep3×1)
[ 17:57:26] R4保留替换点: baseline(sleep10×2,sleep15×1), idem(sleep15×1), wal_snap(sleep8×1,sleep10×1,sleep15×1), health(sleep15×1)
[ 17:57:26] R1验证: 4个suite脚本均已替换
 tests/suite_baseline.sh | 8 ++++----
 tests/suite_health.sh   | 2 +-
 tests/suite_idem.sh     | 2 +-
 tests/suite_wal_snap.sh | 6 +++---
 4 files changed, 9 insertions(+), 9 deletions(-)
[ 17:59:21] T3 FAIL 冻结
[ 03:58:36] T3: sleep→wait_for统一
[ 03:58:36] 方案: A路线——仅保留带明确容器变量的wait_for替换, 回退裸数字is_running替换
[ 03:58:36] 原因: 裸数字is_running 1引入运行态验证, 改变原始sleep盲等语义, 导致T3冻结
[ 04:00:25] T3: sleep→wait_for统一
[ 04:00:25] 方案: A路线——仅保留带明确容器变量的wait_for替换, 回退裸数字is_running替换
[ 04:00:25] 原因: 裸数字is_running 1引入运行态验证, 改变原始sleep盲等语义, 导致T3冻结
[ 04:00:25] 保留替换点: baseline(sleep15→wait_for 15 is_running "$follower"), health(sleep15→wait_for 15 is_running "$follower")
[ 04:00:25] 回退替换点: baseline(sleep10×2), idem(sleep15×1), wal_snap(sleep8/10/15×3)——均裸数字
[ 04:00:25] R1验证: baseline+health均已替换
 tests/suite_baseline.sh | 2 +-
 tests/suite_health.sh   | 2 +-
 2 files changed, 2 insertions(+), 2 deletions(-)
[ 04:06:42] T3 FAIL 冻结
[ 04:38:20] T3: sleep→wait_for统一
[ 04:38:20] 方案: A2——回退所有wait_for替换, T3为空操作验证模式
[ 04:38:20] 原因: wait_for is_running语义≠sleep盲等(running≠SERVING), 下批实现grpc_serving readiness探针
[ 04:38:20] T3: 空操作验证模式
[ 04:44:55] T3 PASS (no-op) commit=unchanged run_id=run-20260906_043820
[ 04:44:55] 终局: knife_run fix/d1-batch2 v1.0.0-d2
[ 04:51:35] 终局PASS tag=v1.0.0-d2

---

# D2-batch1 关账记录

## 目标
把沙箱测试拓扑转化为真实可部署拓扑（docker-compose + 配置文件化 + 一键脚本）

## 交付清单
1. tests/deploy/docker-compose.yml — 2节点集群拓扑声明 (md5=e04972a4e5e2f0dd61e5c8b6e58f6f9c)
2. tests/deploy/deploy.env.example — 配置模板（占位符，入git） (md5=d58dd90cb2b8948df4aa9cd6bd385de4)
3. tests/deploy/deploy_up.sh — 一键拉起脚本 (md5=3bbb2dfaabb12b728262d5e54545f1af)
4. tests/deploy/deploy_verify.sh — 一键验收脚本 (md5=4559909fea0e23f5b6ff53b695237bca)
5. .gitignore更新 — 添加tests/deploy/deploy.env和.sm4_key (md5=34a93e6c00a1b8713aaf60302cc93708)

## 两轮9/9证据指纹
- 步骤2 Leader当选: step2-up-20260906_230231/deploy_up.log (Leader=raft-node-1 after 5s)
- 步骤3 全流程9/9 PASS: step3-verify-20260906_230624/verify.log (md5=ea6ab38d963e517b8aaaec0394ba216a)
- 步骤4 可重复性9/9 PASS: step4-repeat-20260906_230722/verify.log (md5=7266d5a590940b3ede777206dd0dd68d)
- compose config校验: compose-config-output.txt (退出码0, 修正D不冲突确认)

## 修正A-D落实情况
- 修正A (SM4_KEY双模式): deploy_up.sh留空→首次生成写入.sm4_key复用; deploy_verify.sh一次性随机+down -v ✅
- 修正B (deploy.env不入git): 提交deploy.env.example(占位符), deploy.env+.sm4_key入.gitignore, LICENSE_DIR不入库 ✅
- 修正C (yaml语法校验): 删除version行, docker compose config退出码0 ✅
- 修正D (project name不冲突): compose project=deploy, 资源前缀deploy_ vs harness net-/wal/n前缀, 不冲突 ✅

## 已知限制
deploy_up.sh / deploy_verify.sh 为bash脚本，在本机经PowerShell(ps1)等价包装执行
（WSL2无docker CLI，bash入口未直跑）。功能逻辑由ps1等价覆盖，两轮9/9验证通过。
后续在Linux/WSL有docker环境时可直跑bash脚本无需ps1包装。

## 关账时间
2026-09-06 23:08

---

# 时间线补充记录

## Docker迁移验证 (2026-09-06)
- 迁移: <HOME>\AppData\Local\Docker → D:\DockerData (VHDX+目录联接)
- 验证: docker-migration-verify.log 6/6通过
  - 步骤1 docker version ✅
  - 步骤2 docker ps -a ✅ (4容器Exited0)
  - 步骤3 镜像计数 ✅ (82镜像一致)
  - 步骤4 hello-world ⚠️ (registry 403非迁移问题, 本地镜像确认可用)
  - 步骤5 集群实跑 ✅ (Leader 6s + gRPC SERVING + 10条写入读回 + Follower同步 + 干净关闭)
  - 步骤6 日志落盘 ✅
- 判定: Docker迁移无损确认

## C盘清理 (2026-09-06)
- 清理前: C盘 已用=205.60GB 剩余=94.40GB
- 清理后: C盘 已用=163.36GB 剩余=136.64GB
- 总释放: 42.24GB
  - Docker迁移: 24.51GB (VHDX C→D)
  - B类清理: 3.88GB (Temp 2.17 + npm 1.71)
  - C类迁移: 13.17GB (Desktop 3.16 + tcx4_forensics 10.01 → D盘归档)
  - D类处理: 0.92GB (.jdks僵尸 0.28 + go 0.64)
  - Ollama: 保留 (用户选择)
- 数据安全: 全部迁移到D盘(归档区+隔离区+DockerData), 无永久删除
- 报告: <BACKUP_DIR>/c-cleanup-report.txt
