# Phase 3.5 功能矩阵

状态含义：`未完成` 表示不能通过 Phase 3.5 完成门禁；必须以真实 API、权限、持久化、自动化测试与验收证据闭环后更新。

| page | frontend | api | permission | persistence | tests | evidence | status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `/customer/friends` | `FriendsPage` 筛选/详情/刷新 | `/workContact/index`、`/workContact/show` | 复用 Dashboard access/RBAC | `mc_work_contact*` | `friends-page.test.tsx` | 自动化通过，浏览器待父任务 | 代码完成 |
| `/customer/group` | `GroupPage` 筛选/成员详情/刷新 | `/workRoom/index`、`/workRoom/roomIndex` | 复用 Dashboard access/RBAC | `mc_work_room*`、`mc_work_contact_room` | `group-page.test.tsx` | 自动化通过，浏览器待父任务 | 代码完成 |
| `/customer/order` | 未完成 | 未完成 | 未完成 | 需最小 migration | 未完成 | 未完成 | 未完成 |
| `/customer/settings` | 未完成 | 未完成 | 未完成 | 需最小 migration | 未完成 | 未完成 | 未完成 |
| `/data/customer` | 未完成 | 未完成 | 未完成 | 复用现有表 | 未完成 | 未完成 | 未完成 |
| `/data/employee` | 未完成 | 未完成 | 未完成 | Provider 受限时声明 | 未完成 | 未完成 | 未完成 |
| `/data/conversion` | 未完成 | 未完成 | 未完成 | 复用现有表与订单 | 未完成 | 未完成 | 未完成 |
| `/data/behavior` | 未完成 | 未完成 | 未完成 | 复用可审计事件 | 未完成 | 未完成 | 未完成 |
| `/data/report` | 未完成 | 未完成 | 未完成 | 复用 reporting 服务 | 未完成 | 未完成 | 未完成 |
