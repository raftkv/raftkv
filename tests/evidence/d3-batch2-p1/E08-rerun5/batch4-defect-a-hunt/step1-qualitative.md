# D3-batch5 步骤1·定性数据对照

## T+5min vs T+15min inuse_space 对照

| 节点 | T+5min (MB) | T+15min (MB) | 增长倍数 | T+5min 主导 | T+15min 主导 |
|------|------------|-------------|---------|------------|-------------|
| node-1 | 54.09 | 312.82 | 5.8x | AppendEntries(93%) | Snapshot(74%) |
| node-2 | 59.10 | 523.09 | 8.9x | AppendEntries(87%) | Snapshot(85%) |
| node-3 | 54.08 | 1,333.29 | 24.7x | AppendEntries(93%) | Snapshot(95%) |
| node-4 | 83.61 | 1,158.64 | 13.9x | io.ReadAll(77%)+Propose(11%) | Snapshot(87%) |
| node-5 | 49.60 | 1,856.94 | 37.4x | AppendEntries(92%) | Snapshot(97%) |

## alloc_space vs inuse_space（T+15min）

| 节点 | alloc_space (MB) | inuse_space (MB) | GC回收率 | 驻留率 |
|------|-----------------|-----------------|---------|--------|
| node-4 | 63,889.90 | 1,158.64 | 98.19% | 1.81% |
| node-5 | 46,852.97 | 1,856.94 | 96.03% | 3.97% |

## 判定

**瞬态缓冲+GC滞后**（非持续驻留）

依据：
1. GC回收率 96-98% → 绝大部分分配已被回收，非永久驻留
2. T+5min 仅 50-84MB → 早期Snapshot的临时对象已被GC回收
3. alloc_space/inuse_space = 25-55x → 高翻转率，对象在创建-回收循环中
4. live heap 增长是因为 Snapshot 临时对象随快照增大而增大，GC跟不上分配速率

## 步骤4验收指标指引

基于"瞬态缓冲+GC滞后"判定，修复后重点盯：
- **alloc_space**（总分配量）：从 ~50-64GB 降至 ~2-5GB（增量快照消除O(N²)放大）
- **inuse_space**（live heap）：从 ~1-2GB 降至 <200MB（每次Snapshot仅处理10K条）
- **Snapshot占live heap**：从 73-97% 降至 <20%
