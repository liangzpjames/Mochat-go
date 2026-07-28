# Phase 1：前端统一底座

## 阶段状态

统一底座 Task 1–10 已执行并验收，但仍处于发布纠偏状态。技术基础闭环不等于前端产品全量迁移完成。

## 实施前状态

Dashboard、Sidebar、Operation 和 SaaS Admin 没有统一 workspace、共享认证/API/路由契约或可控的逐页迁移机制，旧前端源码和许可证证据也未形成完整审计基线。

## 阶段目标

- 建立 pnpm workspace 和统一前端质量门禁。
- 将 SaaS Admin 纳入 workspace 并保持 `/saas-admin/`。
- 建立 React Dashboard Shell、共享认证/API/路由能力。
- 通过版本化 manifest 控制 React 与 legacy 路由。
- 迁移第一条 Dashboard 业务路由，证明切换和回滚闭环。

## 已完成

- Phase 1 Task 1–10，最终验收提交为 `78f4218`。
- React Dashboard 成为主入口。
- manifest 当前包含 61 条 Dashboard 路由：`/corp/index` 为 React，60 条为 legacy。
- 登录、企业上下文、权限、错误边界和路由决策已有自动化覆盖。
- 前端审计矩阵、迁移 Runbook 和 Windows/Linux 验收证据已形成。

## 未完成与阻塞

- Dockerfile 漏构建和复制 SaaS Admin dist，`/saas-admin/` 当前错误返回 Dashboard。
- Dashboard 菜单数据尚未形成完整的可点击导航。
- 登录页只实现功能骨架，视觉质量低于原 Vue 页面。
- Dashboard bundle 约 1.19 MB，后续迁移需拆包。
- 真实账号、真实 SaaS 租户和生产证据仍缺失。
- 前端审计测试有 2 项既有失败，原因是 `internal/dashboard/corp_admin_test.go` 不存在。

## 验收与证据

- `plans/`：统一底座实施计划、迁移 Runbook 和发布纠偏设计。
- `verification/`：Phase 1 最终验收。
- `audit/`：页面、路由、API、权限、资产和依赖审计矩阵。
- `evidence/`：Windows 前端门禁、Linux Go 门禁和已知限制。

## 下一阶段入口

先完成 SaaS Admin Docker 发布纠偏，再进入 [Phase 2：前端逐页迁移](../phase-2-frontend-migration/README.md)。

