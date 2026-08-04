# Phase 3.4 第四批素材底座与九页浏览器验收报告

日期：2026-08-04

## 结论

第四批已将素材管理从历史 `placeholder` 状态升级为真实本地 Provider，并完成跨页面素材选择、引用持久化、删除保护和营销工具九页浏览器回归。文本素材、分组和引用保护在本机 Docker 数据库中真实执行；企业微信外部发布、授权、客服同步以及依赖企业成员数据的触达动作仍按失败状态或阻塞状态呈现，未伪造成功。

本批次不宣称 Phase 3.4 产品级全线完成，Manifest 只提升有真实证据支持的页面。

## 实施范围

- `mc_medium` 作用域：`public`、`department`、`personal`，并增加 `sidebar_visible`、`status` 和当前企业/操作者可见性过滤。
- 素材分组、文本素材创建与持久化、分组移动、批量删除；删除前检查 `mc_greeting`、朋友圈任务、群发任务和可信素材引用。
- `GET /dashboard/materialSelector/index`：按企业、场景、可见作用域和 `available` 状态返回统一 `{id,name,type,preview}` 选择项。
- 朋友圈、客户群发、群聊群发和一键加群创建流程保存 `mediumId`，服务端再次校验企业和权限边界。
- 朋友圈任务草稿、发布状态、进度抽屉、失败原因和失败明细导出入口均走真实 Provider；未配置发布适配器时返回 `503` 并持久化 `failed`。

## TDD 与缺陷修复证据

1. 先写失败测试，再实现素材作用域、批量操作、引用校验和选择器 Provider；`go test ./internal/dashboard ./internal/store` 及 Dashboard 定向测试通过。
2. Docker 首次加载选择器时发现真实 SQL 语法错误。新增 `TestMediumWhereSelectorVisibleScopeConditionIsBalanced`，先观察到括号不平衡导致 MySQL `1064`，再修复 `internal/store/mysql.go`，提交 `7edbfcd`。
3. 短链真实跳转首次被前端 fallback 截获。新增 `TestWrapDashboardPassesShortLinkRedirectsToNext`，先观察到 `/r/<token>` 返回 SPA，再修复保留路径并提交 `d31b31d`。Docker 重建后真实跳转、访问计数和停用 `410` 均已验证。
4. 既有 Provider 失败状态刷新缺陷分别通过 `52699bb`、`b0861fe`、`cee987f` 修复，浏览器刷新后仍显示失败状态。

## 九页浏览器证据

浏览器截图目录：`D:\workspace\mochat-go\output\phase34-browser-20260804`

| 页面 | 本机真实 Provider / 结果 | 关键证据 |
| --- | --- | --- |
| `/acquisition/v2-channel-code` 渠道活码 | 真实查询 Provider；企业隔离空列表可见 | `01-channel-code-list.png` |
| `/acquisition/group-code` 群活码 | 复用自动拉群查询 Provider；当前企业无群配置，保持 `partial` | `02-group-code-list.png` |
| `/acquisition/redirect-link` 获客链接 | 创建、持久化、授权失败状态和刷新均真实执行；外部授权适配器未配置 | `04-redirect-link-create-drawer.png`、`05-redirect-link-saved.png`、`08-redirect-link-failed-persisted.png` |
| `/acquisition/wechat-customer-service` 微信客服 | 创建、持久化、同步失败原因和刷新均真实执行；外部同步适配器未配置 | `10-wechat-customer-service-create-drawer.png`、`11-wechat-customer-service-saved.png`、`21-wechat-customer-service-failed-refreshed.png` |
| `/acquisition/live-code-short-chain` 活码短链 | 创建、真实 `/r/<token>` 跳转、访问计数和停用 `410` 已验证 | `15-live-code-short-chain-saved.png`、`17-live-code-short-chain-redirect-fixed.png`、`18-live-code-short-chain-visit-count.png`、`19-live-code-short-chain-disabled.png` |
| `/acquisition/group-template` 一键加群 | 素材选择器可用；提交时服务端真实校验到当前企业没有员工，显示“使用者信息错误”，没有写入伪造模板 | `32-group-template-material-selector-fixed.png`、`33-group-template-material-selected.png`、`37-group-template-no-employee-blocker-final.png` |
| `/acquisition/precise-group-send` 精准群发 | 客户群发真实保存为 `执行中 0/0`；详情抽屉可查；群聊群发因无群主返回“群主参数有误” | `42-precise-group-send-state-after-refresh.png`、`43-precise-group-send-detail.png`、`45-group-chat-send-create-drawer.png`、`47-group-chat-send-provider-blocker.png` |
| `/acquisition/friends-circle` 朋友圈 | 草稿和素材引用真实持久化；正式发布返回 `503`，任务变为 `失败`，进度抽屉显示 `friends circle publisher not configured` | `51-friends-circle-draft-saved.png`、`53-friends-circle-progress-drawer.png`、`55-friends-circle-failure-details.png` |
| `/acquisition/material-management` 素材管理 | 文本创建、分组、移动、引用保护和未引用批量删除均真实执行 | `28-material-saved.png`、`29-material-group-created.png`、`30-material-moved.png`、`57-material-reference-protected.png`、`59-material-batch-delete-complete.png` |

每个已打开的抽屉、侧栏或进度面板均保存了浏览器可见截图，验收同时检查遮罩、布局、控件状态和错误反馈，不以 DOM 存在代替操作证据。

## 门禁与部署

- Go：定向测试、全量 `go test ./...` 通过；第四批迁移最新版本断言已更新为 `0118_material_selector_references`。
- Dashboard：`typecheck`、Vitest（69 个文件、449 个测试）、生产 `build` 和 `pnpm check:yuanhu-benchmark` 通过。
- Dashboard 全仓库 `lint` 仍失败：175 个既有错误、0 个 warning，集中在历史 Phase 3.3/SCRM/旧测试文件；没有通过修改 lint 配置掩盖该债务。
- Docker：仅执行 `docker compose -f deploy/standalone/docker-compose.yml -p mochat-go-desktop up -d --build --no-deps app`；app、MySQL、Redis 均 healthy，`/healthz` 和 `/readyz` 返回 `200`。端口为 app `18080`、sidebar `18081`、operation `18082`、MySQL `13316`、Redis `26389`。
- Docker 数据卷未删除或清理。迁移前逻辑备份位于 `D:\workspace\mochat-go\output\docker-backups\mochat-go-desktop-20260804-phase34-final\mochat-before-phase34-migrations.sql`。

## 精确阻塞

1. 本机没有企业微信授权/客服同步/朋友圈发布凭据。正式适配器存在，但 `Phase34UnavailableExternalProvider`/`FriendsCirclePublisher` 明确返回未配置错误；这不是成功外发证据。
2. 测试企业 `corp_id=1` 没有 `mc_work_employee` 和 `mc_work_employee_department` 数据。一键加群和群聊群发因此被服务端拒绝；没有为通过截图而注入假员工、假群主或假回调。
3. 客户群发本地任务保持 `执行中 0/0`，本机没有企业微信结果回调。结果同步 Cron/Provider 合同已保留，但在没有外部回调时不能把任务提升为成功。
4. 正式迁移命令仍被历史 `0001_initial_schema` checksum 漂移阻断：应用记录为 `b7db...`，当前文件为 `03425c...`。为保持数据卷不变，已先备份，再在容器内应用并登记本批 `0116`、`0117`、`0118`；未改写旧账、未删除数据卷。该迁移账本问题需要单独治理。
