# Phase 1：前端统一底座

## 阶段状态

已完成。统一底座 Task 1–10 与发布纠偏均已实施并通过本地验收。技术基础闭环不等于前端产品全量迁移完成。

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
- Dashboard 与 SaaS Admin 已同时进入 Docker 镜像，`/` 与 `/saas-admin/` 返回不同应用及独立资源。
- Dashboard Shell 已按权限菜单生成可点击导航，并提供 SaaS 管理后台入口。
- 登录页已使用 Ant Design 完成产品化视觉与响应式布局。

## 转入后续阶段的风险

- Dashboard bundle 约 1.19 MB，后续迁移需拆包。
- 真实账号、真实 SaaS 租户和生产证据仍缺失。
- 前端审计测试已恢复为 30/30 通过；Windows Go 路径分隔符和 POSIX 权限断言仍作为平台限制记录。

## 验收与证据

- `plans/`：统一底座实施计划、迁移 Runbook 和发布纠偏设计。
- `verification/`：Phase 1 最终验收。
- `audit/`：页面、路由、API、权限、资产和依赖审计矩阵。
- `evidence/`：Windows 前端门禁、Linux Go 门禁和已知限制。

## 下一阶段入口

进入 [Phase 2：前端逐页迁移](../phase-2-frontend-migration/README.md)，先建立页面 metadata override 和批次基线。
