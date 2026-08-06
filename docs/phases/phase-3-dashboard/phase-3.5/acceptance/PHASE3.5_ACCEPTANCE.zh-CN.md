# Phase 3.5 非浏览器验收记录

## 已完成验证

- `go test ./...` 全量通过；Dashboard 476 测试、typecheck、生产构建通过；Phase 3.5 门禁 9/9。
- reporting 查询统一限制 tenant、corp、时间窗口和分页参数；客户明细返回负责人姓名，页面不直出内部 key。
- 2026-08-06 晚重建部署（保留数据卷），迁移应用至 0123；浏览器九页验收与订单跨页工作流证据闭合，见 [总验收报告](../reports/phase3.5-total-acceptance-report.md)。

## 未闭合项

- `P35-ACCEPT-*` 数据生命周期已执行留证（create/verify/cleanup，审计表 3 条）。
- Playwright E2E spec 已入库（`web/e2e/tests/phase35-live.spec.ts`，真实部署 2 项通过）。
- 真实企微/会话存档 Provider 数据待补；受限态已验证。
