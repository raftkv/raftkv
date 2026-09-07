# E05b 别名恢复场景 PASS 报告 — R-04修复后

## 测试时间
2026-09-08 01:30–01:33 (UTC+8)

## 测试目的
验证R-04修复A（gRPC DNS重连）自愈全链：别名丢失→degraded→别名恢复→自愈

## 测试步骤与结果

### 步骤1: 制造别名丢失
1. docker network disconnect node-1
2. 生成300条写入（10并发, 15s, 100%写）→ leader commit 7918→8218
3. docker network connect（**不带alias**）node-1
4. 验证DNS: `nslookup node-1` → **NXDOMAIN**（别名丢失确认）

### 步骤2: 确认degraded
- Leader stats: `gaps=map[node-1:300] degraded=[node-1]`
- **degraded标记正确触发**（修复B生效）
- gap=300持续不收敛（DNS无法解析，批量同步无法到达）

### 步骤3: 恢复别名
1. docker network disconnect node-1
2. docker network connect **--alias node-1** node-1
3. 验证DNS: `nslookup node-1` → **192.168.80.3**（别名恢复确认）

### 步骤4: 验证自愈全链
- Leader(node-2) stats: `gaps=map[node-1:0] degraded=[]`
- node-1自身: `commit=8241` = leader commit=8241
- **gap→0, degraded清除, 全集群一致**

## 自愈时序
| 时间点 | 事件 | 状态 |
|--------|------|------|
| 01:30:20 | disconnect node-1 | 开始隔离 |
| 01:31:10 | reconnect无alias | DNS NXDOMAIN |
| 01:31:20 | 确认degraded | degraded=[node-1], gap=300+ |
| 01:32:30 | reconnect带alias | DNS恢复 |
| <01:33:00 | 自愈完成 | gap=0, degraded=[] |

## 修复验证项
- 修复A: DNS恢复后重连循环检测到连接恢复，新DNS resolver成功解析→gRPC重连→批量同步到达
- 修复B: 别名丢失期间degraded=[node-1]正确标记；别名恢复后同步成功→degraded清除
- 修复C: /raft/stats全程正确输出gaps和degraded字段，gap>100持续10s告警触发
- 修复D: --alias node-1显式指定别名，DNS恢复

## 结论
**E05b PASS** — 别名丢失→degraded→别名恢复→自愈全链验证通过