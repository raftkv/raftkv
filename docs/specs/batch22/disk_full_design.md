# 战役II：磁盘满故障注入 — 技术设计

> 版本：v2.4-batch22-diskfull-design
> 状态：方案设计（实现留 batch23）
> 关联：disk_full_spec.md

## 1. 架构设计

### 1.1 注入工具
- **复用 chaos_injector 框架**：新增 `disk_ctl.go` 模块
- **注入方法**：`docker exec <container> fallocate -l <size> /data/wal/fillfile`
- **检测方法**：`docker exec <container> df -h /data/wal` → 解析 Use%
- **清理方法**：`docker exec <container> rm /data/wal/fillfile`

### 1.2 指标采集
- **DF-1**：注入期间轮询 `/raft/stats` 检查集群状态 + leader 可用性
- **DF-2**：注入前抽样 commit entry，注入后验证存活（复用 F3 方法）
- **DF-3**：清理后轮询 `/raft/stats` 检查 log 追平（gaps=0）
- **DF-4**：`docker inspect` 检查容器状态（不 Exit）

### 1.3 数据模型
```json
{
  "scenario_id": "disk_full_follower_01",
  "target_node": "node-3",
  "inject_timestamp": "...",
  "disk_usage_before": "45%",
  "disk_usage_after": "100%",
  "cluster_available": true,
  "leader_changed": false,
  "recovery_timestamp": "...",
  "log_catchup_duration_s": 12.5,
  "node_crashed": false,
  "status": "PASS"
}
```

## 2. 实现计划（batch23）

1. `cmd/chaos_injector/disk_ctl.go`：磁盘满注入/清理/检测
2. `cmd/chaos_injector/scheduler.go`：新增 disk_full 场景调度
3. `tests/contracts/batch23.yaml`：DF-1~DF-4 验收契约
4. `tests/contracts/judge_batch23.py`：判定脚本

## 3. 风险与约束
- Docker volume 大小需预先确认（默认 Docker volume 无限制，需限制容器内 /data/wal 分区大小）
- 磁盘满可能影响 Docker 引擎本身，需在独立 volume 上注入
- 恢复清理需确保 WAL 文件完整性（不可误删 WAL 日志文件）