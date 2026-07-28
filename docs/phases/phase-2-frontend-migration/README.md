# Phase 2：前端逐页迁移

## 阶段状态

实施已开始；page metadata 基线已建立，尚未建立可计算的任务完成比例。

## 实施前状态

- Dashboard manifest 共 61 条路由。
- React 业务页面只有 `/corp/index`。
- 60 条 Dashboard 路由仍使用 legacy。
- Sidebar 和 Operation 尚未迁移。
- React Shell 已提供授权导航和 SaaS Admin 入口。
- SaaS Admin 已作为独立产物进入 Docker 发布链路。
- 单人开发模式不维护 owner；审计中的 135 条页面均已建立 risk 与 batch 基线。

## 阶段目标

- 分批迁移 Dashboard 基础 CRUD、资源密集页面和复杂业务流程。
- 建立 Sidebar React app，再复用其模式迁移 Operation。
- 每条路由独立测试、切换和回滚。
- 当 manifest 不再包含 legacy 后，独立移除旧运行引用和构建产物。

## 已完成

- 已建立逐页迁移 Runbook、六类审计矩阵和显式 manifest 机制。
- 已通过 `/corp/index` 证明单路由 React 切换闭环。
- 已完成 [Phase 2 迁移设计](plans/2026-07-28-phase2-frontend-migration-design.md)。
- page metadata override 与防覆盖测试已实现。
- 单人开发模式已删除 owner 元数据。
- 135 条页面已完成风险与批次基线，当前 [未基线化页面清单](audit/unassigned-pages.csv) 为空。
- Dashboard Batch 2 首批候选见 [候选顺序](audit/dashboard-batch2-candidates.csv)。

## 未完成与阻塞

- 部分 legacy 资产的许可证或来源未确认。
- 真实企微、微信开放平台及租户数据不足以验收复杂页面。

## 验收与证据

每批必须包含单元测试、API 契约、Playwright、构建门禁、路由 manifest 切换、视觉证据和独立回滚记录。Phase 2 的计划与证据将在本目录继续建立。

## 下一阶段入口

从 Dashboard Batch 2 候选报告第 1 条路由开始，逐路由通过交付门禁后切换 manifest。
