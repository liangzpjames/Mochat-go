# MoChat Go 开发总进度

> 更新时间：2026-07-28  
> 当前分支：`main`  
> 当前阶段：Phase 2 设计完成，等待逐页迁移实施

## 当前结论

后端独立版、Go 工程底座和 Phase 1 前端统一底座均已完成。Docker 已分别发布 Dashboard 与 SaaS Admin，授权导航和产品化登录页也已补齐。前端产品迁移尚未完成：当前只有 `/corp/index` 是 React 业务页面，60 条 Dashboard 路由仍由 legacy 承接，Sidebar 和 Operation 尚未迁移。

## 阶段总览

| 阶段 | 状态 | 进度口径 | 主要结果 | 入口 |
| --- | --- | --- | --- | --- |
| Phase Pre-0：独立版与 SaaS 能力收口 | 已完成 | 以独立部署、SaaS MVP 和候选证据为准 | 建立独立运行、SaaS 总后台和生产证据链 | [阶段详情](phases/phase-pre0-standalone/README.md) |
| Phase 0：Go 工程底座 | 已完成 | 计划、检查表和验收项已闭合 | 建立 Go 单体运行边界、登记表和验证基线 | [阶段详情](phases/phase-0-go-foundation/README.md) |
| Phase 1：前端统一底座 | 已完成 | Task 1–10 与发布纠偏均验收 | pnpm workspace、React Shell、共享契约、manifest、首条 React 路由、双应用发布与授权导航 | [阶段详情](phases/phase-1-frontend-foundation/README.md) |
| Phase 2：前端逐页迁移 | 设计完成，实施未开始 | 尚未完成页面元数据基线，不虚构百分比 | 已确定元数据基线、分批迁移、视觉门禁和 legacy 退出策略 | [阶段详情](phases/phase-2-frontend-migration/README.md) |

## 当前可测试范围

| 范围 | 状态 | 说明 |
| --- | --- | --- |
| `/login` | 可测试 | React + Ant Design 产品化登录页 |
| `/` | 可测试 | React Dashboard Shell，按权限渲染导航 |
| `/corp/index` | 可测试 | 当前唯一注册的 React 业务页面，需要有效账号与权限 |
| 60 条 Dashboard legacy 路由 | 可按权限导航或直接访问 | manifest 承接，逐页迁移尚未开始 |
| `/saas-admin/` | 可测试 | Docker 独立产物与资源前缀已验证 |
| Sidebar | 未迁移 | 保留旧前端和独立运行入口 |
| Operation | 未迁移 | 保留旧前端和独立运行入口 |

## 当前阻塞与风险

1. **迁移基线未完成。** 开始 Dashboard batch 2 前，必须给 134 条未分配页面补齐 owner、risk 和 batch，并保护元数据不被 `--refresh` 覆盖。
2. **外部证据缺失。** 真实企微、微信开放平台、SaaS 租户及生产环境证据仍未提供。
3. **既有测试限制。** Windows 环境仍存在 Go 路径分隔符和 POSIX 权限断言问题；前端审计测试已恢复为 30/30 通过。
4. **性能风险。** Dashboard 首包约 1.19 MB，Phase 2 必须按页面切分。

## 精确下一任务

1. 建立 Phase 2 页面 metadata override 和防覆盖测试。
2. 为未分配页面建立迁移批次。
3. 按风险和依赖领取 Dashboard batch 2 第一条基础 CRUD/列表页。

## 最近交付

- `78f4218`：Phase 1 验收与交接。
- `2869ed0`：SaaS Admin Docker 发布修复设计。
- `c7bae35`：阶段化文档重组设计。
- `79a5e27`：阶段化文档重组实施计划。
- `9f93cd9`：Dashboard 与 SaaS Admin 双应用发布。
- `aa42a52`：Dashboard 授权导航。
- `9a7eb1c`：Ant Design 登录页。

## 更新规则

- 开始任何开发实施前，更新当前阶段 README 的“实施前状态”。
- 每个任务提交后更新“已完成、未完成与阻塞、验收与证据”。
- 只有任务清单和验收定义已基线化时才给出完成比例。
- 证据生成文件不得直接覆盖人工维护的总进度。
