# 当前会话交接

> 更新时间：2026-07-28

## 当前状态

- 正式分支：`phase1/frontend-unification-foundation`
- worktree：`D:\workspace\mochat-go\mochat-go\.worktrees\frontend-unification-foundation`
- Phase 1 Task 1–9 已完成；Task 9 提交为 `ae9f0bd`。
- React 已成为 Dashboard 单入口；manifest 当前共 61 条：1 条 React（`/corp/index`），60 条 legacy。
- workspace 使用 Node `>=22.12 <25`、pnpm `11.17.0`、React 19.2、TypeScript strict、Vite、Vitest 和 Playwright。
- 本地 `.superpowers/`、`.workbuddy/` 等用户或会话文件不得提交或删除。

## 已验证能力

- 旧三端源码、哈希和六类审计矩阵已恢复。
- SaaS Admin 已进入 pnpm workspace 并保持 `/saas-admin/`。
- Dashboard 登录、企业选择/切换、菜单权限、401/403/404、通用加载失败边界和 React/legacy 决策已自动化覆盖。
- Windows quick/build/E2E 全绿；Playwright 9/9 通过真实 Go frontend wrapper。
- Windows `go test ./...` 的既有路径/权限失败未修复；Docker Linux `go test ./... && go vet ./...` 已通过，远端 CI 仍需确认。

## 当前阻塞与风险

- 真实企微、微信开放平台、真实 SaaS 租户和生产环境仍未提供，REQ-006/readiness 不可关闭。
- 部分 legacy 资产许可证或来源未确认，标记 blocked 的资产不得迁入 React。
- Task 9 的 `ae9f0bd` 已推送到远端 `main` 和 `phase1/frontend-unification-foundation`。
- Dashboard bundle 约 1.19 MB；后续批次需拆包。
- CI/Playwright 存在首次下载时长和外部网络 flaky 风险。

## 精确下一任务

领取 Dashboard batch 2 前，先给审计生成器增加受版本控制的 page metadata override 输入及防覆盖测试，避免 `--refresh` 重置迁移元数据。然后为当前 134 条 `unassigned` 页面补齐 owner、risk 和 batch，按“blocked 优先解阻；其余 risk 升序、共享 API 覆盖率降序、依赖升序”生成可独立发布的候选顺序，再选择第一条基础 CRUD/列表页执行 `FRONTEND_MIGRATION_RUNBOOK.zh-CN.md`。

## 必读入口

- `docs/handle/FRONTEND_PHASE1_VERIFICATION.zh-CN.md`
- `docs/handle/FRONTEND_MIGRATION_RUNBOOK.zh-CN.md`
- `docs/handle/2026-07-27-frontend-unification-foundation-implementation-plan.md`
- `docs/handle/frontend-audit/`
- `web/apps/dashboard/src/migration-routes.json`
