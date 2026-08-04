# Phase 3.4 朋友圈 Provider 实施报告

日期：2026-08-04

## 实施结果

- 新增本地持久化 Provider，覆盖朋友圈任务草稿与素材的分页查询、筛选、创建、企业隔离和审计字段。
- 新增 `FriendsCirclePublisher` 可插拔发布合同；当前部署未配置企业微信发布适配器，发布接口返回 `503`，不会修改草稿状态或产生真实外发。
- 新增 `/dashboard/friendsCircle/taskIndex`、`materialIndex`、`taskStore`、`materialStore`、`publish` 五个受登录、企业选择和 RBAC 保护的路由。
- 新增 `0114_friends_circle_provider` 数据迁移，包含任务表、素材表和五项 RBAC 权限。
- 朋友圈页面接入真实 Provider，覆盖任务/素材双标签、关键字筛选、重置、刷新、空状态、失败重试、草稿创建和列表刷新；导出与正式发布继续阻断。

## 安全边界

- `corp_id`、创建用户和创建人由服务端登录上下文确定，不接受客户端覆盖。
- 发布前按企业归属以 `draft -> publishing` 条件更新原子抢占任务；跨企业或不存在资源返回 `404`，并发或非法状态返回 `422`。外部失败标记为 `failed`，外部成功但本地确认失败时保留 `publishing` 等待对账，避免重复外发。
- 自动化与浏览器验收只创建本地草稿和素材，没有调用真实企业微信发布。
- Manifest 如实标记 `backend=partial`、`acceptance=unit-passed`；外部发布、回调进度、失败明细和导出 Provider 完成前，不提升为 `backend=ready` 或 `e2e-passed`。

## 验证证据

- Dashboard 全量测试：67 个测试文件、429 项测试通过。
- TypeScript 类型检查、变更文件 Lint、生产构建、53 页 benchmark 清单和 `git diff --check` 通过。
- Go 定向包测试通过；全量 Go 回归包含迁移最新版本断言。
- Docker 原地重建 `mochat-go-desktop` 的 `app` 服务，MySQL、Redis 和四个既有数据卷未删除或重建；服务 `readyz` 与容器健康检查通过。
- 浏览器创建任务草稿和文字素材成功；页面刷新后任务字段仍完整展示，证明数据库持久化与前端 JSON 合同生效。
- 截图：`screenshots/friends-circle-provider.png`。

## 未完成边界

本次完成的是朋友圈本地领域 Provider 与页面闭环，不代表朋友圈产品能力全部完成。企业微信正式发布适配器、异步回调、进度统计、失败重试/明细、审批与导出仍属于后续工作。
