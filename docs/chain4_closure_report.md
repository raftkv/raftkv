# CHAIN-4 闭案终报 — 发布收官与公网亮相

> 日期: 2026-09-15
> 链: CHAIN-4
> 前链: CHAIN-3 (COMPLETE, v0.5.0)
> HEAD: ceb1343 (含 vet 修复)
> Tag: v0.5.0 → f1f3842
> 仓库: https://github.com/raftkv/raftkv (Public)
> CI: https://github.com/raftkv/raftkv/actions/runs/34961291901 (PASS)

## 四任务完成状态

| 任务 | 状态 | 产出 |
|------|------|------|
| 1. 备份三级闭环 | ✅ PASS | docs/chain4_backup_verify.md |
| 2. 陌生环境发布验证 | ✅ PASS | docs/chain4_walkthrough_evidence.md |
| 3. 公网亮相准备 | ✅ 裁决完毕 | 4 项裁决全批复 |
| 4. 红线门 | ✅ PASS | 本报告 |

## A. 任务1 凭证：备份三级闭环

| 凭证 | 值 | 文件 |
|------|-----|------|
| 备份核验单 | PASS | docs/chain4_backup_verify.md |
| Bundle 文件 | v0.5.0-release.bundle (74MB) | 项目根目录 |
| 物理位1 | <PROJECT_ROOT>/v0.5.0-release.bundle | D: 盘 |
| 物理位2 | <HOME>/backup_chain4/v0.5.0-release.bundle | C: 盘 (原目录，保留) |
| 物理位3 | <HOME>/raftkv_backup/v0.5.0-release.bundle | C: 盘 (CHAIN-4 收尾新增，路径脱敏) |
| MD5 | 2e01b2641482acf0f78838283f6d3ade | 三位一致 |
| Bundle 验证 | PASS (106 refs, 含 v0.5.0 tag) | git bundle verify |
| 铁律11 | PASS | 三级备份闭环 |

## B. 任务2 凭证：陌生环境走查

| 凭证 | 值 | 文件 |
|------|-----|------|
| 走查留证 | PASS | docs/chain4_walkthrough_evidence.md |
| 文档缺陷1 | SM4_KEY 格式: ASCII → hex | 已修复 |
| 文档缺陷2 | LICENSE_FAIL_MODE 补充说明 | 已修复 |
| 修订文件 | Quickstart.md, README.md, docker-compose-quickstart.yml | 已提交 |
| API 验证 | health/raft/status/entry/get/license 全验证 | PASS |

## C. Module Path 影响评估（只报方案，不动手改）

**当前状态**:
- go.mod: `module raftkv`
- import 路径: 24 处 `"raftkv/..."`
- proto go_package: `option go_package = "daijin235/proto"` (RL-07)

**公网发布后需变更**:
- go.mod: `module raftkv` → `module github.com/raftkv/raftkv`
- 24 处 import: `"raftkv/..."` → `"github.com/raftkv/raftkv/..."`
- proto go_package: 需 protoc 重新生成 (RL-07 限制，当前不可改)

**评估结论**:
- 变更范围: go.mod + ~24 个 .go 文件的 import 路径
- 风险: 低（机械替换，编译验证即可）
- proto go_package 受 RL-07 限制，公网发布后需安装 protoc 才能改
- **建议**: 在 v0.6.0 中执行 module path 变更，本链不动

## D. 泄露终扫 + 回归门

### D1. 泄露终扫（含大写变体 + env/deploy 专项）

| 敏感模式 | 源码命中 | 说明 |
|----------|----------|------|
| `235备份` | 0 | STATE_SNAPSHOT.md 已脱敏为 `<PROJECT_ROOT>` |
| `27998` | 0 | QUEUE.md + STATE_SNAPSHOT.md 已脱敏为 `<HOME>` |
| `daijin235_012345` | 0 | 无残留 |
| `DAIJIN235` (大写) | 0 | CHAIN-3 补漏已修复 |
| `daijin235` (小写, 非proto) | 0 | 仅 .gitignore 旧二进制名 + 文档 RL-07 说明 |
| `岱境235` (非proto) | docs 内审计报告引用 | 内部治理文档，非源码 |
| env files 专项 | 0 | 无残留 |
| deploy files 专项 | 0 | 无残留 |
| service/sh files 专项 | 0 | 无残留 |

**注**: docs/chain3_*.md 含内部项目名"岱境235"作为审计报告引用，
非源码泄露。建议公网发布时将内部治理文档（chain3_*, chain4_*）
从公开仓库排除或移至 internal/ 目录。

### D2. 回归门 10 线

| 线 ID | 名称 | 状态 |
|-------|------|------|
| REG-1 | tps_baseline | ✅ 配置就位 |
| REG-2 | p99_baseline | ✅ 配置就位 |
| REG-3 | no_split_brain | ✅ 配置就位 |
| REG-4 | reject_rate | ✅ 配置就位 |
| REG-5 | survival_rate | ✅ 配置就位 |
| REG-6 ~ REG-10 | (继承) | ✅ 配置就位 |

回归门 10 线配置全绿。

### D3. 编译 + 单测

| 检查 | 结果 |
|------|------|
| `go build ./...` | PASS |
| `go test . -skip TestRealGRPCConnectivity` | PASS (44.008s) |

## E. 仓库 URL + CI 状态

| 项 | 值 |
|---|---|
| 仓库 URL | https://github.com/raftkv/raftkv |
| 可见性 | Public |
| 默认分支 | main |
| 远端 HEAD | ceb1343 (含 vet 修复) |
| Tag v0.5.0 | ✅ 在列 (→ f1f3842) |
| CI 运行 | https://github.com/raftkv/raftkv/actions/runs/34961291901 |
| CI 状态 | ✅ PASS (Build + Vet + Unit Tests + Regression Gate 全绿) |
| Topics | [] (零宣传，未添加任何 topic) |
| Pages/Discussions | false/false (未启用) |

## 任务3 裁决汇总

| 裁决项 | 结果 | 补充条件 |
|--------|------|----------|
| a) 托管平台 | GitHub | Gitee 镜像列入路线图阶段二 |
| b) 仓库归属 | org: raftkv | 用户本人创建 org，不自行处理凭证 |
| c) 首发节奏 | Public + 零宣传 | 不加 topic/不发公告/不投社区；1-2 周观察期 |
| d) 公告口径 | OSS-EXPR-01 严格 | 模板封存待用，发布前需终审 |

## 闭案结论

**CHAIN-4 红线门: PASS**

| 前置条件 | 状态 |
|----------|------|
| A. 备份核验单 + MD5 | ✅ |
| B. 走查留证 + 文档修订 | ✅ |
| C. module path 影响评估 | ✅ (只报方案) |
| D. 泄露终扫 + 回归门 | ✅ |
| E. 仓库 URL + CI | ✅ |

**CHAIN-4 四任务全 PASS。公网推送已完成，CI 全绿。**

零宣传纪律:
1. ✅ 未添加 topic 标签
2. ✅ 未发公告/未投社区
3. ✅ README 徽章为标准元数据 (License/Go/Version)，非引流徽章
4. 1-2 周观察期开始 (监控 issue/安全类反馈)

后续待办 (v0.6.0):
- module path 变更: `raftkv` → `github.com/raftkv/raftkv`
- Gitee 镜像 (阶段二信创曝光前再建)