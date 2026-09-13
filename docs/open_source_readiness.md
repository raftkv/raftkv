# 开源准备预探 — ROADMAP 方向六

> 生成: batch32
> 日期: 2026-09-14

## 1. 开源准备度评估

| 维度 | 准备度 | 缺口 |
|------|--------|------|
| 代码质量 | 高 | 无 pprof（已知限制） |
| 测试覆盖 | 高 | 10 线回归门全绿 |
| 文档 | 中 | 需 README + CONTRIBUTING + LICENSE |
| 治理 | 高 | AUDIT 20 判例 + MB 11 条 |
| CI/CD | 低 | 无自动化 CI 配置 |
| 安全 | 中 | mTLS 已落地，需安全审计 |

## 2. 缺口清单

| 缺口 | 优先级 | 建议批次 |
|------|--------|----------|
| README.md | P1 | batch33 |
| CONTRIBUTING.md | P2 | batch33 |
| LICENSE | P1 | batch33 |
| CI 配置（GitHub Actions） | P2 | batch34 |
| 安全审计 | P2 | batch34 |
| pprof 集成 | P3 | batch35（可选） |

## 3. 建议后续批次

- batch33: 开源文档三件套（README+CONTRIBUTING+LICENSE）
- batch34: CI/CD + 安全审计
- batch35（可选）: pprof 集成 + 性能分析工具链