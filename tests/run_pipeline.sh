#!/bin/bash
# run_pipeline.sh — D1-batch2 自主流水线
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TESTS_DIR="$(cd "$(dirname "$0")" && pwd)"
BRANCH="fix/d1-batch2"
FINAL_TAG="v1.0.0-d2"
D1_TAG_COMMIT="9d5c491"
MERGE_COMMIT="ca84148"
TS="$(date +%Y%m%d_%H%M%S)"
PDIR="${TESTS_DIR}/evidence/pipeline-${TS}"
PLOG="${PDIR}/progress.log"
RLOG="${PDIR}/run.log"

DRY_RUN=false
RESUME=false
for a in "$@"; do
    case "$a" in
        --dry-run) DRY_RUN=true ;;
        --resume)  RESUME=true ;;
    esac
done

log() { echo "[$(date +%H:%M:%S)] $*" | tee -a "$PLOG"; }

check_flags() {
    local task="$1" d
    d=$(git diff --cached)
    if echo "$d" | grep -q '^[+-].*assert_'; then
        log "FLAG $task: 触碰assert_*行"; return 3
    fi
    if echo "$d" | grep -q '^diff.*\.go'; then
        log "FLAG $task: 触碰.go文件"; return 3
    fi
    if echo "$d" | grep -q '^+.*|| true' && echo "$d" | grep -q 'assert_'; then
        log "FLAG $task: 测试判定路径新增||true"; return 3
    fi
    return 0
}

preflight() {
    local rc=0
    if [ -n "$(git status --porcelain)" ]; then
        echo "pre-flight FAIL: git status不干净"
        git status --porcelain
        rc=5
    fi
    if [ "$(git branch --show-current)" != "$BRANCH" ]; then
        echo "pre-flight FAIL: 不在$BRANCH"
        rc=5
    fi
    if [ -n "$(git diff "$D1_TAG_COMMIT" "$MERGE_COMMIT" 2>/dev/null)" ]; then
        echo "pre-flight FAIL: tag树≠merge树"
        rc=5
    fi
    if [ $rc -eq 0 ]; then echo "pre-flight PASS"; fi
    return $rc
}

run_knife() {
    local tag="$1" no_tag="${2:-true}"
    local args="$BRANCH $tag baseline,idem,wal_snap,health"
    if [ "$no_tag" = "true" ]; then args="$args --no-tag"; fi
    bash "${TESTS_DIR}/knife_run.sh" $args 2>&1
}

get_run_id() {
    grep -o 'run_id=run-[0-9_]*' "$RLOG" | tail -1 | cut -d= -f2
}

# ── T1: harness.sh RUN_ID守卫 ──
task_t1() {
    log "T1: harness.sh RUN_ID赋值加守卫"
    log "方案: RUN_ID=\"\" → RUN_ID=\"\${RUN_ID:-}\"  双保险保留knife_run save/restore"
    sed -i 's/^RUN_ID=""$/RUN_ID="${RUN_ID:-}"/' "${TESTS_DIR}/harness.sh"
    bash -n "${TESTS_DIR}/harness.sh"
    git add "${TESTS_DIR}/harness.sh"
    check_flags "T1" || return $?
    if run_knife "v1.0.0-d2-t1" "true" | tee -a "$RLOG"; then
        local rid; rid=$(get_run_id)
        git commit -m "fix(ci): T1 harness.sh RUN_ID守卫 (run_id=$rid)"
        log "T1 PASS commit=$(git rev-parse --short HEAD) run_id=$rid"
        return 0
    else
        log "T1 FAIL 冻结"; return 2
    fi
}

# ── T2: do_rollback冒烟改双节点 ──
task_t2() {
    log "T2: do_rollback冒烟改双节点方案A"
    log "方案: 移source至do_rollback前, 替换内联docker run为up_cluster/down_cluster, RID=rbk-\${RUN_ID}"
    local KR="${TESTS_DIR}/knife_run.sh"

    # 1. 删除SMOKE段source块(150-153)
    sed -i '150,153d' "$KR"

    # 2. 替换冒烟段(78-106)为up_cluster/down_cluster
    sed -i '78,106d' "$KR"
    sed -i '77a\
    # 冒烟确认：双节点集群选主\
    local rbk_rid="rbk-${RUN_ID}"\
    local _saved_image="$IMAGE_NAME"\
    IMAGE_NAME="$ROLLBACK_IMAGE"\
    if up_cluster "$rbk_rid" 2>&1 | tee -a "${RUN_EVIDENCE}/rollback_smoke.log"; then\
        echo "[rollback] 冒烟绿: Leader当选" | tee -a "${RUN_EVIDENCE}/rollback.log"\
    else\
        echo "[rollback] 警告: 冒烟也失败! 基线可能已损坏!" | tee -a "${RUN_EVIDENCE}/rollback.log"\
    fi\
    down_cluster "$rbk_rid"\
    IMAGE_NAME="$_saved_image"' "$KR"

    # 3. 在do_rollback前插入source块
    sed -i '46a\export IMAGE_NAME LICENSE_DIR FP_ANCHOR EVIDENCE_DIR TESTS_DIR\
_knife_run_id="$RUN_ID"\
source "${TESTS_DIR}/harness.sh"\
RUN_ID="$_knife_run_id"\
' "$KR"

    bash -n "$KR"
    git add "$KR"
    check_flags "T2" || return $?
    if run_knife "v1.0.0-d2-t2" "true" | tee -a "$RLOG"; then
        local rid; rid=$(get_run_id)
        git commit -m "fix(ci): T2 do_rollback双节点冒烟 (run_id=$rid)"
        log "T2 PASS commit=$(git rev-parse --short HEAD) run_id=$rid"
        return 0
    else
        log "T2 FAIL 冻结"; return 2
    fi
}

# ── T3: sleep→wait_for统一 ──
task_t3() {
    log "T3: sleep→wait_for统一"
    log "方案: suite脚本中所有sleep替换为wait_for轮询, 判定条件不变"

    # suite_baseline.sh
    local SB="${TESTS_DIR}/suite_baseline.sh"
    sed -i '52s/sleep 2/wait_for 2 is_running "$leader"/' "$SB"
    sed -i '59s/sleep 10/wait_for 10 is_running 1/' "$SB"
    sed -i '80s/sleep 2/wait_for 2 is_running "$leader"/' "$SB"
    sed -i '98s/sleep 2/wait_for 2 is_running "$leader"/' "$SB"
    sed -i '105s/sleep 10/wait_for 10 is_exited "$(_c_name "$follower")"/' "$SB"
    sed -i '107s/sleep 15/wait_for 15 is_running "$follower"/' "$SB"
    sed -i '125s/sleep 2/wait_for 2 is_running "$leader"/' "$SB"
    sed -i '132s/sleep 10/wait_for 10 is_running 1/' "$SB"

    # suite_idem.sh
    local SI="${TESTS_DIR}/suite_idem.sh"
    sed -i '88s/sleep 1/wait_for 1 is_running "$leader"/' "$SI"
    sed -i '115s/sleep 1/wait_for 1 is_running "$leader"/' "$SI"
    sed -i '119s/sleep 2/wait_for 2 is_running "$leader"/' "$SI"
    sed -i '127s/sleep 1/wait_for 1 is_running "$leader"/' "$SI"
    sed -i '134s/sleep 15/wait_for 15 is_running 1/' "$SI"

    # suite_wal_snap.sh
    local SW="${TESTS_DIR}/suite_wal_snap.sh"
    sed -i '19s/sleep 3/wait_for 3 is_running 1/' "$SW"
    sed -i '37s/sleep 10/wait_for 10 is_running 1/' "$SW"
    sed -i '54s/sleep 2/wait_for 2 is_running "$leader"/' "$SW"
    sed -i '62s/sleep 15/wait_for 15 is_running 1/' "$SW"
    sed -i '121s/sleep 8/wait_for 8 is_running 1/' "$SW"

    # suite_health.sh
    local SH="${TESTS_DIR}/suite_health.sh"
    sed -i '67s/sleep 3/wait_for 3 is_exited "$follower_container"/' "$SH"
    sed -i '75s/sleep 15/wait_for 15 is_running "$follower"/' "$SH"

    for f in "$SB" "$SI" "$SW" "$SH"; do bash -n "$f"; done
    git add "$SB" "$SI" "$SW" "$SH"
    check_flags "T3" || return $?
    if run_knife "v1.0.0-d2-t3" "true" | tee -a "$RLOG"; then
        local rid; rid=$(get_run_id)
        git commit -m "fix(ci): T3 sleep→wait_for统一 (run_id=$rid)"
        log "T3 PASS commit=$(git rev-parse --short HEAD) run_id=$rid"
        return 0
    else
        log "T3 FAIL 冻结"; return 2
    fi
}

# ── DECISION.md ──
generate_decision() {
    local md="${PDIR}/DECISION.md"
    local t1_sha t2_sha t3_sha
    t1_sha=$(git log --oneline --grep='T1' -1 | cut -d' ' -f1)
    t2_sha=$(git log --oneline --grep='T2' -1 | cut -d' ' -f1)
    t3_sha=$(git log --oneline --grep='T3' -1 | cut -d' ' -f1)
    {
        echo "# D1-batch2 DECISION"
        echo ""
        echo "## T1: harness.sh RUN_ID守卫"
        echo "commit: $t1_sha"
        git show --stat "$t1_sha"
        echo ""
        echo "## T2: do_rollback双节点冒烟"
        echo "commit: $t2_sha"
        git show --stat "$t2_sha"
        echo ""
        echo "## T3: sleep→wait_for统一"
        echo "commit: $t3_sha"
        git show --stat "$t3_sha"
        echo ""
        echo "## 终局"
        echo "tag: $FINAL_TAG"
        echo "merge: $(git log --oneline -1 v1.0-dev)"
        echo ""
        echo "## 信任计分"
        echo "本批旗标数: 0"
        echo "冻结次数: 0"
        echo "门禁颁发tag数: 1"
    } > "$md"
}

# ── 主流程 ──
mkdir -p "$PDIR"

if [ "$DRY_RUN" = "true" ]; then
    echo "=== DRY RUN ==="
    echo "队列: T1(harness RUN_ID守卫) → T2(do_rollback双节点) → T3(sleep→wait_for) → 终局(knife_run $FINAL_TAG)"
    echo ""
    echo "旗标规则:"
    echo "  - 触碰assert_*行或期望值字面量 → exit 3"
    echo "  - 触碰.go文件 → exit 3"
    echo "  - 测试判定路径新增||true → exit 3"
    echo ""
    echo "运行护栏:"
    echo "  - 写路径白名单: 仅仓库目录内"
    echo "  - 黑名单: git push / docker system prune / 仓库外rm / git reset --hard"
    echo ""
    echo "pre-flight:"
    preflight
    exit $?
fi

preflight || exit 5

START=1
if [ "$RESUME" = "true" ] && [ -f "$PLOG" ]; then
    if grep -q "T3 PASS" "$PLOG"; then START=4
    elif grep -q "T2 PASS" "$PLOG"; then START=3
    elif grep -q "T1 PASS" "$PLOG"; then START=2
    fi
fi

if [ $START -le 1 ]; then task_t1 || exit 2; fi
if [ $START -le 2 ]; then task_t2 || exit 2; fi
if [ $START -le 3 ]; then task_t3 || exit 2; fi

log "终局: knife_run $BRANCH $FINAL_TAG"
run_knife "$FINAL_TAG" "false" | tee -a "$RLOG"
log "终局PASS tag=$FINAL_TAG"

generate_decision
log "DECISION.md生成完成"
exit 0