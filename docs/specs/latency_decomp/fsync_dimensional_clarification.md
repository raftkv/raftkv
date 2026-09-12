# fsync 取证量纲澄清

> 关联：DEBT-0001 / batch20 fsync 取证 / fsync_forensics.json
> 日期：2026-09-13

## 量纲标注

| 字段 | 值 | 来源 |
|------|-----|------|
| 1. 窗口时长 | 180s | fsync_forensics.json → test_config.duration |
| 2. fsync 总次数 | 4673 | fsync_forensics.json → leader.fsync_count |
| 3. 每秒 fsync | 25.96 (≈26) | fsync_forensics.json → leader.fsync_per_sec |
| 4. 预设标准 | 个位数~几十次/秒 | 性能章节验收标准："fsync count in single digits to tens per second with multi-entry coverage" |
| 5. 对齐结论 | 26 fsync/s 属"几十次/秒"范畴，对齐预设标准 | 26 ∈ [10, 99] → "几十次/秒" |
| 6. 合并比 | 437:1（avg 438 entries/fsync） | fsync_forensics.json → judgment.merge_ratio |
| 7. 判定 | INTRINSIC_CONFIRMED（fsync 合并固有生效） | fsync_forensics.json → judgment.verdict |

## 补充说明

- 85% 的 fsync 覆盖 >256 entries（batch_size_257_plus=3955 / fsync_count=4673 = 84.6%）
- follower 节点 fsync/s 与 leader 一致（26.31~26.38），合并比 432.7:1，佐证合并机制全节点生效
- fsync_avg_latency=32.0ms，fsync_max_latency=202ms，均在可接受范围
- task_two_needed=false：fsync 合并已证实生效，无需实现 true group commit