# Phase 3：Dashboard 与圆弧 AI 功能对标

## 阶段目标

Phase 3 以圆弧 AI 页面为业务和交互参考，将 Dashboard 清单中的页面逐批升级为具有真实数据查询、写入、权限、审计和异常处理能力的正式页面。

全阶段约束以 [`reference/PHASE3_CONSTRAINTS.zh-CN.md`](reference/PHASE3_CONSTRAINTS.zh-CN.md) 为准，页面范围和状态以 `web/apps/dashboard/src/benchmark/manifest.json` 为准。

## 阶段目录

- [`benchmark/`](benchmark/)：Phase 3 初始对标范围与功能矩阵。
- [`phase-3.1/`](phase-3.1/)：Dashboard 基础壳、路由、会话隔离与首批验证资料。
- [`phase-3.2/`](phase-3.2/)：数据概览、全局消息、敏感词及 SCRM 核心页面的设计、实施、验收和证据。
- [`phase-3.3/`](phase-3.3/)：会话与风险预警 15 个菜单的范围、设计、实施计划、验收与证据。
- [`phase-3-final/`](phase-3-final/)：Provider 接入（音频/AI/企微存档）与 53 页总验收收口。
- [`reference/`](reference/)：跨 Phase 3 子阶段持续有效的约束、数据流和状态说明。

## 文档维护规则

1. Phase 3 的统一入口只保留在本目录，不再新建 `docs/phase/` 或其他 Phase 3 根目录。
2. 跨子阶段规则进入 `reference/`；阶段特定资料进入对应 `phase-3.x/`。
3. 每个子阶段继续按 `design/`、`plans/`、`acceptance/`、`reports/` 和 `evidence/` 分类，缺少某类资料时不创建空目录。
4. 脚本、Manifest 和文档链接必须引用本目录中的新路径。
