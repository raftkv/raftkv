# 战役II：磁盘满故障注入 — 需求规格

> 版本：v2.4-batch22-diskfull-spec
> 状态：方案设计（实现留 batch23）
> 关联：batch22 任务三预演

## 1. 故障场景

### 1.1 注入方式
- **目标**：WAL 卷（Docker volume `wal-node-N`）填满至 100%
- **方式**：`docker exec` 向 WAL 卷写入大文件直至 `df` 显示 100%
- **范围**：单节点 WAL 卷填满（leader 或 follower）

### 1.2 预期行为
- WAL fsync 失败（磁盘满）
- 节点应优雅降级：标记自身不可用，停止接受新写入
- 集群应维持 quorum：如填满的是 follower，leader 继续服务；如填满的是 leader，触发选举
- 恢复后（清理磁盘空间），节点应重新加入集群并追平日志

## 2. 验收指标草案

| 指标 | 阈值 | 说明 |
|------|------|------|
| DF-1 | 磁盘满后集群可用性 | follower 磁盘满 → 集群继续服务；leader 磁盘满 → 选举完成 ≤2s |
| DF-2 | 磁盘满期间数据无损 | 已 commit 的 entry 不丢失 |
| DF-3 | 恢复后节点追平 | 清理空间后节点重新加入，log 追平 ≤30s |
| DF-4 | 无 panic/crash | 节点进程不 crash，优雅处理磁盘满错误 |

## 3. 场景矩阵

| 场景 | 目标节点 | 负载 | 恢复方式 |
|------|---------|------|---------|
| disk_full_follower | 随机 follower | c=128 持续 | 清理 WAL 卷空间 |
| disk_full_leader | 当前 leader | c=128 持续 | 清理 + 等待选举 |
| disk_full_recovery | 随机节点 | 无负载 | 清理后验证追平 |

## 4. 红线
- 磁盘满注入不得破坏 fsync 语义（fsync 失败应正确处理，不可忽略）
- 磁盘满不得导致数据丢失（已 commit entry 必须存活）
- 注入工具不得修改 Raft 协议语义