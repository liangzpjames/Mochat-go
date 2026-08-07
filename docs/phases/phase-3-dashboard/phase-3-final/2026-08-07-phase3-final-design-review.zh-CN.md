# Phase 3 Final 设计审阅记录（主任务，2026-08-07）

> 审阅对象：`2026-08-07-phase3-final-provider-onboarding-design.zh-CN.md`
> 结论：设计可执行，关键假设已逐项核验，无需变更设计正文；以下为核验事实与边界记录。

## 1. 已核验事实

- `mc_work_message_1..10` 全部存在 `corp_id`、`content_text`、`deleted_at`、`msg_data_time`、`work_employee_id` 字段（`0014_auto_tag.up.sql`），AI 洞察的“按 corp 取归档文本、按时间倒序限 20 条”查询契约可直接落地。
- 迁移常量引用点已定位：`internal/dashboard/saas_admin_system_health.go`（当前 `0125_bootstrap_role_remark_cn`/125）、`internal/dashboard/saas_admin_system_health_test.go`、`internal/migration/migration_test.go`（两处版本断言）。新增 `0126` 后需全部同步为 `0126_phase3_final_providers`/126。
- `internal/config` 目前没有任何 AI Provider / 企微存档相关环境变量；需在 `config.go` 新增解析与默认值（属于 Go 后端范围），并在 compose 与 README 同步。
- AI 洞察模块现状：`Dependencies{PrincipalResolver, Authorizer}`，5 条 GET 路由，`InsightHandler` 保持 `InsightPage` 响应形态；扩展 DB + AIProvider 后路由与响应结构不变，注册入口为 `cmd/mochat-go/ai_debt_clearance.go` 的 `registerAIDebtClearanceModules`。
- 前端 `/chat/file-audio` 现状：由 `phase33-operations-page.tsx` 的 `phase33OperationConfigs['/chat/file-audio']`（`providerState:'unavailable'`）兜底；manifest 为 `placeholder/missing/not-started`；`scripts/check_debt_clearance.mjs` 的 `allowedIncomplete` 含该页（第 12 行），输出文案为“坏账清理”。

## 2. 边界与记录（本阶段不修）

- 运行库 `mc_rbac_menu` 中不存在任何 Go 原生路由（含 `/chat/file-audio`、SCRM 原生页）的菜单行，RBAC 对未注册菜单返回 `ErrPermissionDenied`。这与既有 SCRM/Phase33 原生页一致：浏览器验收账号为超管，可绕过 RBAC 菜单校验；非超管角色的 Go 原生路由 RBAC 菜单补种属于既有系统债，本阶段只记录、不新增菜单与角色授权（避免无依据的角色授权变更）。
- 企微真实解码依赖用户凭据与线上协议，本阶段只交付适配层与契约测试，不宣称线上兼容。
- AI 结果质量以“真实调用成功 + 落库回读 + 页面展示”为验收，不评审分析内容本身。
- 音频时长不做解析，`duration_seconds` 记 0，前端显示 `--`。

## 3. 开发代理硬性约束（复述）

- 代理 A（后端）：只改 Go 后端、`deploy/standalone/docker-compose.yml`（仅新增 env 映射）、README（仅新增 env 表）、迁移与迁移相关常量、新建测试；禁止 `git add/commit/push`、禁止启动/重建 Docker。
- 代理 B（前端）：只改 `web/apps/dashboard/src`、`scripts/check_debt_clearance.mjs` 及对应测试；禁止改 Go；禁止 `git add/commit/push`、禁止启动/重建 Docker。
- 契约冲突时在各自报告中记录，由主任务仲裁，不得擅自跨范围修改。
