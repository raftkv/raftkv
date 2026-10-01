# 任务三：成功率回归定性（还 batch11 的账）

## batch11 成功率问题
batch11 成功率 99.40%~99.97%，首破 99.9% 线。

## 根因定位

### 根因 1：Bug A — timer 不 Reset 导致请求挂起超时
`proposeBatchLoop` 的 `flush()` 在 batch 为空时不 Reset timer → timer 触发后不再重置 → 请求进 batch 但永远不 flush（batch 不满 64，timer 不触发）→ Propose 永远挂在 `result := <-req.resultCh` → waitForCommit 1s 超时返回 error → 成功率下降。

**证据**：fresh cluster c=8 请求全挂起，pprof 显示 proposeBatchLoop 卡在 select 1 分钟。

### 根因 2：Bug B — loadgen 误计成功掩盖真实失败率
loadgen 仅检查 `StatusCode==200`，follower 返回 `{"success":false}` 被计为成功 → 成功率虚高。batch11 的 99.40%~99.97% 是修复前 loadgen 的测量值，**实际成功率更低**（follower 失败被误计为成功，挂起超时的请求被计为失败）。

## 修复后验证
- Bug A 修复（timer 总是 Reset）：请求不再挂起
- Bug B 修复（loadgen 只发 leader + 检查 body success）：正确计数
- **fresh cluster c=8~256：successRate=100.00%（全部级别）**

## 结论
- batch11 成功率问题是 Bug A（timer 不 Reset）导致的请求挂起超时
- Bug B（loadgen 误计）掩盖了真实失败率
- 修复后 100% 成功率，**无需回滚 group commit 攒批层**
- group commit 攒批层与 pipeline 优化解耦，无相互影响