# 运维 — OPERATIONS

> 摘编自既有 evidence，禁止新编造

## 集群部署

5 节点 Docker Compose 部署:
- 镜像: daijin235-v26:batch27
- 端口: 9001-9005 (HTTP), 9501-9505 (gRPC)
- 网络: deploy5_daijin235-net

## 健康检查

```bash
curl.exe http://localhost:9001/health/live
curl.exe http://localhost:9001/raft/stats
```

## 故障注入

```bash
external/tools/chaos_injector.exe \
  -cluster-config config.toml \
  -contract tests/contracts/batch21.yaml \
  -evidence-dir tests/evidence/d3-batchXX \
  -scenario-type cascading
```

场景类型: steady | under_load | cascading | all

## 回归门

```bash
python tests/contracts/regression_gate.py
```

8 线全绿方可推进本批门。

## 判定

```bash
python tests/contracts/judge_batch23.py \
  --contract tests/contracts/batch23.yaml \
  --evidence-dir tests/evidence/d3-batchXX \
  --output tests/evidence/d3-batchXX/verdict.json
```

verdict 引用 regression.yaml 线 ID (batch27 改造)。

## 红线

- RL-01: protoc 不可用
- RL-02: 证据目录 gitignore
- RL-07: proto 定义不可修改
- RL-11: 构建产物禁入库
- RL-new-1: N=3 中位数 + CV>15% 重测
- RL-new-2: 改线仅限提案制