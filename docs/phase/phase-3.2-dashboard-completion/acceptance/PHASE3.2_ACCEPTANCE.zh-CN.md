# Phase 3.2 完成验收记录

## 验收范围

本次在分支 `phase3.2/dashboard-eight-real-pages` 完成 Phase 3.2：Overview、Global Conversation、敏感词、线索、公海、联系人、商机和标签八个目标路由；SCRM 后端按租户与企业隔离，写操作携带幂等键并使用版本冲突保护。

## 验收证据

- 八个页面的浏览器截图位于 `docs/phase/phase-3.2-dashboard-completion/evidence/`，统一使用 1440×1000 视口。
- 浏览器用例：`pnpm --filter @mochat/e2e exec playwright test tests/phase3-2-dashboard.spec.ts --workers=1`，4 个用例通过。
- Dashboard：typecheck、全量 Vitest、生产构建均通过。
- Go：`go test ./internal/modules/scrm/... ./internal/dashboard ./internal/store -count=1` 通过。
- 数据库：独立 MariaDB Compose 项目应用迁移后执行严格 MySQL 集成测试；脚本失败即退出并清理容器与卷。
- 查询性能：0103 为企业趋势查询增加 `(corp_id,date)` 索引，日期条件使用半开区间，避免对日期列做函数计算。

## 部署验证

最终包部署到 `mochat-go-desktop`，保留以下数据卷：应用存储、审计锚点、MySQL 数据和 Redis 数据。未保留的 standalone 测试卷已清理。部署后需确认 `/readyz` 返回 200，并确认 SCRM 路由在未认证请求下返回 401，证明路由已注册且认证边界仍生效。

## 已知边界

Phase 3.2 不扩展 Phase 3.1 之外的 45 个占位页面；既有全量迁移视觉用例仍有历史页面残余失败，不作为本次八路由门禁的替代。正式完成以本记录、八张截图、Phase 3.2 专项 Playwright 和最终门禁共同为准。
