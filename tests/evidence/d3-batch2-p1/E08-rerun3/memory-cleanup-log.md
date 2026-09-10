# 内存清理日志 — E08第3轮OOM事故整改

## 事故背景
E08第3轮2h压测期间，node-1/3/5分别于开测后约3/5/12分钟被Docker OOM强杀(Exited 137)。
根因：宿主机31.5GB内存长期94%占用，5节点集群+压测客户端超出Docker配额。

## 清理前水位
- 总内存: 31.5GB
- 可用内存: 15.8GB
- 占用率: 50.0%
- 注: 用户已手动停止压测客户端，OOM杀掉的容器已释放内存

## 清理操作
1. Exited(137)容器日志取证: node-1/3/5各50行 → tests/evidence/d3-batch2-p1/E08-rerun3/node-{1,3,5}-exit137-log.txt
2. 删除Exited(0)的4个smoke容器: n1/n2-smoke-run-20260906_044455, n1/n2-smoke-run-20260906_043820
3. 删除Exited(137)的3个容器: daijin235-node-1/3/5 (日志已取证)
4. 清理悬空镜像: 20个 → 回收5.404GB
5. 清理历史卷: 33个(wal1/wal2-smoke-*, wal1/wal2-base-*, wal1/wal2-health-*, wal1/wal2-probe-test-*, 匿名卷)

## 清理后水位
- 总内存: 31.5GB
- 可用内存: 15.6GB
- 占用率: 50.6%
- Docker磁盘: Images=1.506GB, BuildCache=39.02GB, Volumes=6.106GB

## 保留资源
- deploy5_wal-node-{1..5}: 当前集群WAL卷
- 235__grafana-data/mysql-data/prometheus-data: 监控数据
- desktop_grafana_data/prometheus_data: Docker Desktop监控
- daijin235-v26:ci-knife: 当前镜像

## Docker Desktop配额建议
当前Docker Desktop内存配额可能过高（导致OOM时未及时限制）。建议用户手动将配额压至8GB:
Settings → Resources → Memory → 8GB

## 复测前置条件
1. 5个node容器全部Up且healthy
2. 宿主机内存占用<80% (当前50.6% ✓)
3. docker stats确认集群内存占用平稳无泄漏
4. 压测客户端启动后记录当时宿主机可用内存