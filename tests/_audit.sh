#!/bin/bash
# _audit.sh — D1-batch 机械验收
# 用法: bash tests/_audit.sh [seal_md5]
set -euo pipefail

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TESTS_DIR="$(cd "$(dirname "$0")" && pwd)"

# 封印值：参数 > 文件 > 默认
SEAL_MD5="${1:-}"
if [ -z "$SEAL_MD5" ] && [ -f "${TESTS_DIR}/.seal_md5" ]; then
    SEAL_MD5=$(cat "${TESTS_DIR}/.seal_md5" | tr -d '[:space:]')
fi
if [ -z "$SEAL_MD5" ]; then
    SEAL_MD5="d4731dd87cb197b4e8f56af98ffd0a05"
fi

# 找最新pipeline目录
PDIR=$(ls -dt "${TESTS_DIR}"/evidence/pipeline-* 2>/dev/null | head -1)
if [ -z "${PDIR:-}" ] || [ ! -d "$PDIR" ]; then
    echo "AUDIT FAIL: 找不到pipeline目录"
    exit 1
fi
PLOG="${PDIR}/progress.log"
DECISION="${PDIR}/DECISION.md"
PIPELINE_SH="${TESTS_DIR}/run_pipeline.sh"

PASS_CNT=0
FAIL_CNT=0

audit_pass() { echo "[PASS] $1"; PASS_CNT=$((PASS_CNT + 1)); }
audit_fail() { echo "[FAIL] $1"; FAIL_CNT=$((FAIL_CNT + 1)); }

# ── 1. md5封印核验 ──
actual_md5=$(md5sum "$PIPELINE_SH" | cut -d' ' -f1)
if [ "$actual_md5" = "$SEAL_MD5" ]; then
    audit_pass "1. md5封印核验 ($actual_md5)"
else
    audit_fail "1. md5封印核验 (期望=$SEAL_MD5 实际=$actual_md5)"
fi

# ── 2. bash -n语法检查 ──
if bash -n "$PIPELINE_SH" 2>/dev/null; then
    audit_pass "2. bash -n语法检查"
else
    audit_fail "2. bash -n语法检查"
fi

# ── 3. grep '[$' = 0 ──
dollar_bracket=$(grep -cF '[$' "$PIPELINE_SH" 2>/dev/null || true)
if [ "$dollar_bracket" = "0" ]; then
    audit_pass "3. grep '[\$' = 0"
else
    audit_fail "3. grep '[\$' = $dollar_bracket"
fi

# ── 4. 断言计数对账 ──
if [ -f "$DECISION" ]; then
    last_total=$(grep '总计: PASS=' "$DECISION" | tail -1)
    dec_pass=$(echo "$last_total" | grep -oE 'PASS=[0-9]+' | cut -d= -f2)
    dec_fail=$(echo "$last_total" | grep -oE 'FAIL=[0-9]+' | cut -d= -f2)
    task_pass=$(grep -c 'PASS' "$PLOG" 2>/dev/null || true)
    task_fail=$(grep -c 'FAIL 冻结' "$PLOG" 2>/dev/null || true)
    if [ "${dec_fail:-0}" = "0" ] && [ -n "${dec_pass:-}" ]; then
        audit_pass "4. 断言计数对账 (DECISION PASS=$dec_pass FAIL=$dec_fail, progress PASS=$task_pass FAIL=$task_fail)"
    else
        audit_fail "4. 断言计数对账 (DECISION PASS=$dec_pass FAIL=$dec_fail)"
    fi
else
    audit_fail "4. 断言计数对账 (DECISION.md不存在)"
fi

# ── 5. SHA核对 ──
sha_ok=true
sha_detail=""
for task in T1 T2 T3; do
    plog_sha=$(grep "${task} PASS" "$PLOG" | grep -oE 'commit=[a-f0-9]{7}' | tail -1 | cut -d= -f2 2>/dev/null || true)
    if [ -n "$plog_sha" ] && [ "$plog_sha" != "unchanged" ]; then
        if git -C "$REPO_DIR" cat-file -e "${plog_sha}^{commit}" 2>/dev/null; then
            sha_detail="${sha_detail}${task}=${plog_sha}✓ "
        else
            sha_ok=false
            sha_detail="${sha_detail}${task}=${plog_sha}✗ "
        fi
    fi
done
if $sha_ok; then
    audit_pass "5. SHA核对 ($sha_detail)"
else
    audit_fail "5. SHA核对 ($sha_detail)"
fi

# ── 6. tag计数 ──
tag_count=$(git -C "$REPO_DIR" tag -l 'v1.0.0-*' | wc -l | tr -d ' ')
if [ "$tag_count" -ge 2 ]; then
    audit_pass "6. tag计数 ($tag_count)"
else
    audit_fail "6. tag计数 ($tag_count < 2)"
fi

# ── 总判定 ──
echo ""
echo "=============================="
echo "  AUDIT: PASS=$PASS_CNT  FAIL=$FAIL_CNT"
echo "=============================="
if [ "$FAIL_CNT" = "0" ]; then
    echo "  ALL PASS"
    exit 0
else
    echo "  HAS FAIL"
    exit 1
fi