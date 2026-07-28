# Phase 2：前端逐页迁移

## 阶段状态

未开始，尚未建立可计算的任务完成比例。

## 实施前状态

- Dashboard manifest 共 61 条路由。
- React 业务页面只有 `/corp/index`。
- 60 条 Dashboard 路由仍使用 legacy。
- Sidebar 和 Operation 尚未迁移。
- React Shell 缺少完整导航渠道。
- SaaS Admin Docker 发布缺失必须先修复。
- 审计中还有 134 条页面未分配 owner、risk 和 batch。

## 阶段目标

- 分批迁移 Dashboard 基础 CRUD、资源密集页面和复杂业务流程。
- 建立 Sidebar React app，再复用其模式迁移 Operation。
- 每条路由独立测试、切换和回滚。
- 当 manifest 不再包含 legacy 后，独立移除旧运行引用和构建产物。

## 已完成

- 已建立逐页迁移 Runbook、六类审计矩阵和显式 manifest 机制。
- 已通过 `/corp/index` 证明单路由 React 切换闭环。

## 未完成与阻塞

- page metadata override 尚未实现，运行审计刷新可能覆盖人工批次元数据。
- 134 条页面尚未分配负责人、风险和批次。
- 部分 legacy 资产的许可证或来源未确认。
- 真实企微、微信开放平台及租户数据不足以验收复杂页面。

## 验收与证据

每批必须包含单元测试、API 契约、Playwright、构建门禁、路由 manifest 切换、视觉证据和独立回滚记录。Phase 2 的计划与证据将在本目录继续建立。

## 下一阶段入口

第一项实施是 page metadata override 与防覆盖测试；完成后生成 Dashboard batch 2 候选顺序。

