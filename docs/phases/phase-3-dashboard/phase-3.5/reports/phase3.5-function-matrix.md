# Phase 3.5 功能矩阵

> 同步时间：2026-08-06 晚（依据 `feature/phase3.5-scrm-reporting` 最新工作区与当日重建部署 + 浏览器/视觉模型验收）

状态含义：`已验收` 表示实现、自动化测试、重建部署与浏览器截图证据已闭环；`待补证` 表示仍缺某项证据（如 Provider 数据、E2E 脚本、P35-ACCEPT 生命周期执行记录）。

| page | frontend | api | permission | persistence | tests | evidence | status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `/customer/friends` | `FriendsPage` 筛选/分页/详情/刷新 | `/workContact/index`、`/workContact/show` | 复用 Dashboard access/RBAC，corp 范围 | `mc_work_contact*` | `friends-page.test.tsx` | 浏览器截图 `01-friends.png` + 视觉核验通过；空态提示符合设计 | 已验收（Provider 数据待补证） |
| `/customer/group` | `GroupPage` 筛选/成员明细/刷新 | `/workRoom/index`、`/workRoom/roomIndex` | 复用 Dashboard access/RBAC，corp 范围 | `mc_work_room*`、`mc_work_contact_room` | `group-page.test.tsx` | 浏览器截图 `02-group.png` + 视觉核验通过；空态提示符合设计 | 已验收（Provider 数据待补证） |
| `/customer/order` | `OrderPage` 联系人搜索选择/快速建联系人/创建/详情/状态流转/审计时间线 | GET/POST `/scrm/orders`、PUT `/scrm/orders/{id}/transition`、GET `/scrm/orders/{id}?view=audit` | authorizer `/scrm/orders#get`、`@add#post`、`@edit#put` | `mochat_go_scrm_orders`、`mochat_go_scrm_order_audit`（0119+0121） | `order-page.test.tsx`、`order_handler_test.go`、`order_repository_integration_test.go` | 真实工作流跑通：创建订单 → 流转为已支付 → 转化漏斗订单 1、综合报表订单经营 1；截图 10–12 | 已验收 |
| `/customer/settings` | `SettingsPage` 分类定义/校验/版本/保存回读 | GET/PUT `/scrm/settings` | `/scrm/settings#get`、`@edit#put` | `mochat_go_scrm_settings`（0119） | `settings-page.test.tsx`、`settings_handler_test.go` | 浏览器截图 `04-settings.png` + 视觉核验通过（两行配置，版本 v1/v2） | 已验收 |
| `/data/customer` | `CustomerReportPage` KPI/趋势/明细（日期/负责人列，不直出内部 key） | GET `/dashboard/reports/customer` | reporting 合同 + tenant/corp 范围 | 复用 work contacts/SCRM 与订单表 | `customer-report-page.test.tsx`、reporting Go 测试 | 截图 `05-customer-report.png`：明细列“日期/负责人”显示真实姓名；默认日期已修正为当月 1 日 | 已验收 |
| `/data/employee` | `EmployeeReportPage` Provider 受限卡（文案已去重） | GET `/dashboard/reports/employee` | 同上 | 会话归档 Provider 缺失时返回 `limitations` | `remaining-report-pages.test.tsx` | 截图 `06-employee-report.png`：受限原因仅出现 1 次，恢复条件明确 | 已验收（Provider 数据待补证） |
| `/data/conversion` | `ConversionReportPage` 漏斗/阶段下钻 | GET `/dashboard/reports/conversion` | 同上 | 复用 leads/contacts/opportunities/orders | `conversion-report-page.test.tsx`、reporting Go 测试 | 截图 `07/13`：联系人 4、订单 1（流转后）；订单转化率与漏斗口径一致 | 已验收 |
| `/data/behavior` | `BehaviorReportPage` 中文行为字典/详情 | GET `/dashboard/reports/behavior` | 同上 | 订单/设置审计事件 | `remaining-report-pages.test.tsx` | 截图 `08-behavior-report.png`：6 条事件含订单创建/状态流转/设置更新 | 已验收 |
| `/data/report` | `ReportPage` 四段综合报表/跨页下钻 | GET `/dashboard/reports/report` | 同上 | 复用 reporting 服务，不复制 SQL 口径 | `remaining-report-pages.test.tsx` | 截图 `14-summary-after-order.png`：客户概览 8、订单经营 1、行为审计 8 | 已验收 |

## 阶段总状态

- 九页实现、自动化测试（Dashboard 476 测试、typecheck、生产构建、`pnpm check:phase3-5-dashboard` 9/9、`go test ./...`）、重建部署与浏览器/视觉模型验收于 2026-08-06 晚全部通过。
- 验收过程中发现并修复：
  1. 订单列表 JOIN 联系人 collation 混用（新增迁移 `0123_phase35_order_collation_align` 对齐 `utf8mb4_unicode_ci`）。
  2. 订单状态流转路由只注册 PATCH，前端 PUT 请求返回 501（`internal/modules/scrm/module.go` 兼容注册 PUT+PATCH）。
  3. 报表默认日期经 `toISOString` 偏移一天（`report-query.tsx` 改为本地日期）。
  4. 客户明细直出 UUID/负责人数字 ID（后端补 `ownerName`，前端明细列限为“日期/负责人”）。
  5. 会话分析受限文案重复（`employee-report-page.tsx` 去重）。
- 仍待补证（不阻塞本阶段验收，但未宣告产品完成）：
  1. ~~`P35-ACCEPT-*` 数据生命周期~~：已通过验收 API 在运行库执行 create/verify/cleanup 并留证（`D:\workspace\mochat-go\output\phase35-browser-20260806\p35-accept-lifecycle-evidence.txt`，审计表 create/verify/cleanup 各 3 条）。
  2. ~~Phase 3.4/3.5 Playwright E2E spec~~：已入库 `web/e2e/tests/phase35-live.spec.ts`（默认跳过，设置 `MOCHAT_E2E_LIVE_BASE` 运行；真实部署下 2 项通过）。
  3. 好友/客户群/会话分析依赖真实企微与会话存档 Provider，当前为空态/受限态。
