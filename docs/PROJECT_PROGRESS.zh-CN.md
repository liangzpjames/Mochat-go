# MoChat Go 开发总进度

> 更新时间：2026-07-28  
> 当前分支：`main`  
> 当前阶段：Phase 1 发布纠偏，完成后进入 Phase 2

## 当前结论

后端独立版和 Go 工程底座已经完成，前端统一底座的技术链路也已建立。但前端产品迁移尚未完成：当前只有 `/corp/index` 是 React 业务页面，60 条 Dashboard 路由仍由 legacy 承接，Sidebar 和 Operation 尚未迁移。

当前优先事项不是继续迁移新页面，而是修复 SaaS Admin 在 Docker 镜像中漏打包的问题。现有容器访问 `/saas-admin/` 会回退并返回 Dashboard HTML，因此尚不能进行 SaaS Admin 界面测试。

## 阶段总览

| 阶段 | 状态 | 进度口径 | 主要结果 | 入口 |
| --- | --- | --- | --- | --- |
| Phase Pre-0：独立版与 SaaS 能力收口 | 已完成 | 以独立部署、SaaS MVP 和候选证据为准 | 建立独立运行、SaaS 总后台和生产证据链 | [阶段详情](phases/phase-pre0-standalone/README.md) |
| Phase 0：Go 工程底座 | 已完成 | 计划、检查表和验收项已闭合 | 建立 Go 单体运行边界、登记表和验证基线 | [阶段详情](phases/phase-0-go-foundation/README.md) |
| Phase 1：前端统一底座 | 技术任务完成，存在发布纠偏 | Task 1–10 已执行；产品可用性不按全量迁移计算 | pnpm workspace、React Shell、共享契约、manifest 和首条 React 路由 | [阶段详情](phases/phase-1-frontend-foundation/README.md) |
| Phase 2：前端逐页迁移 | 未开始 | 尚未完成页面元数据基线，不虚构百分比 | 计划迁移 Dashboard、Sidebar 和 Operation，最终移除 legacy | [阶段详情](phases/phase-2-frontend-migration/README.md) |

## 当前可测试范围

| 范围 | 状态 | 说明 |
| --- | --- | --- |
| `/login` | 可访问 | React 功能骨架，尚未恢复原视觉质量 |
| `/` | 可访问 | React Dashboard Shell，占位首页 |
| `/corp/index` | 可测试 | 当前唯一注册的 React 业务页面，需要有效账号与权限 |
| 60 条 Dashboard legacy 路由 | 部分可直接访问 | manifest 可承接，但当前 React 壳没有完整导航渠道 |
| `/saas-admin/` | 不可正确测试 | Docker 镜像未包含 SaaS Admin dist，请求回退到 Dashboard |
| Sidebar | 未迁移 | 保留旧前端和独立运行入口 |
| Operation | 未迁移 | 保留旧前端和独立运行入口 |

## 当前阻塞与风险

1. **P0：SaaS Admin Docker 发布缺失。** Dockerfile 只构建和复制 Dashboard，需要补齐 `@mochat/saas-admin` 构建、镜像复制和身份验证。
2. **P0：Dashboard 导航未形成完整测试渠道。** 菜单权限数据已加载，但 React Shell 尚未渲染可遍历全部页面的导航。
3. **P1：登录页视觉回退。** 当前登录页是未使用 Ant Design 组件的原生表单骨架。
4. **迁移基线未完成。** 开始 Dashboard batch 2 前，必须给 134 条未分配页面补齐 owner、risk 和 batch，并保护元数据不被 `--refresh` 覆盖。
5. **外部证据缺失。** 真实企微、微信开放平台、SaaS 租户及生产环境证据仍未提供。
6. **既有测试限制。** Windows 环境存在既有 Go 路径/权限问题；前端审计测试还有 2 项因 `internal/dashboard/corp_admin_test.go` 不存在而失败。

## 精确下一任务

1. 修复 Dockerfile 和同步脚本，使 SaaS Admin 与 Dashboard 同时进入镜像。
2. 重新部署，并验证 `/` 与 `/saas-admin/` 返回不同应用及各自静态资源。
3. 建立 Phase 2 页面 metadata override 和防覆盖测试。
4. 为未分配页面建立迁移批次，再领取 Dashboard batch 2 第一条基础 CRUD/列表页。

## 最近交付

- `78f4218`：Phase 1 验收与交接。
- `2869ed0`：SaaS Admin Docker 发布修复设计。
- `c7bae35`：阶段化文档重组设计。
- `79a5e27`：阶段化文档重组实施计划。

## 更新规则

- 开始任何开发实施前，更新当前阶段 README 的“实施前状态”。
- 每个任务提交后更新“已完成、未完成与阻塞、验收与证据”。
- 只有任务清单和验收定义已基线化时才给出完成比例。
- 证据生成文件不得直接覆盖人工维护的总进度。

