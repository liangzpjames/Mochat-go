# docs/handle 文档入口

本目录是 MoChat Go 后续接手、架构、开发计划、台账和阶段验收文档的统一归档位置。

## 强制归档规则

以下新文档必须放在本目录：

- 项目接手、现状分析和阶段总结；
- 架构设计、技术决策和重构计划；
- 开发路线、实施计划、任务清单和验收记录；
- 需求、缺陷、风险、资源、密钥和生产证据台账；
- 本地环境、测试流程和部署准备说明。

以下文档应保留在所属代码附近，不强制移动：

- package 级 README、API 契约和数据库 migration 说明；
- 脚本专用说明；
- 依法必须位于仓库根目录的许可证与源码交付文件；
- 机器生成的生产证据原件。

禁止把真实密钥、token、密码或认证状态写入任何文档。台账只记录负责人、存储位置、轮换要求和状态。

## 当前文档

| 文档 | 用途 |
| --- | --- |
| `2026-07-23-phase0-architecture-design.md` | 已确认的 Phase 0 架构设计 |
| `2026-07-23-phase0-implementation-plan.md` | 可跟踪实施计划 |
| `2026-07-27-frontend-unification-foundation-implementation-plan.md` | Phase 1 前端统一底座实施计划 |
| `ARCHITECTURE.zh-CN.md` | 后续需求开发的架构规则 |
| `LOCAL_DEVELOPMENT.zh-CN.md` | Docker 本地开发与验证 |
| `PHASE0_CHECKLIST.zh-CN.md` | Phase 0 状态与退出条件 |
| `RESOURCE_AND_SECRET_INVENTORY.zh-CN.md` | 外部资源和密钥空白清单 |
| `RISK_REGISTER.zh-CN.md` | 风险台账 |
| `REQUIREMENT_REGISTER.zh-CN.md` | 需求台账 |
| `DEFECT_REGISTER.zh-CN.md` | 缺陷台账 |
| `PRODUCTION_EVIDENCE_REGISTER.zh-CN.md` | 生产证据缺口台账 |

## 命名约定

- 长期维护文档使用稳定名称，如 `ARCHITECTURE.zh-CN.md`。
- 阶段设计和执行计划使用 `YYYY-MM-DD-主题.md`。
- 验收原始输出不直接堆入本目录；在验收记录中写命令、时间、退出码和证据路径。
