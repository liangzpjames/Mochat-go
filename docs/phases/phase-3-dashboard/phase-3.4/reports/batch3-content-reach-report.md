# Phase 3.4 第三批内容触达页面实施报告

## 实施范围

- `/acquisition/precise-group-send`：客户群发/群聊群发双标签、任务名称与日期筛选、真实任务列表、执行结果、执行数据、异常重试和详情。
- `/acquisition/friends-circle`：朋友圈/朋友圈素材双标签、任务筛选、发送方式、状态、完成情况、添加与导出入口。

## 真实能力与安全边界

- 精准群发查询分别接入 `/contactMessageBatchSend/index` 和 `/roomMessageBatchSend/index`，任务名称使用 `batchTitle`。页面不注入 fixture。
- 自动化环境禁止真实外发，新建群发入口保持禁用；未调用 `store`、`remind` 或 `destroy` 写接口。
- 当前仓库没有正式朋友圈任务、素材、发布和导出 Provider。页面覆盖圆弧信息架构并明确显示阻断状态，所有依赖 Provider 的操作保持禁用。
- 清单如实记录精准群发 `backend=partial`、朋友圈 `backend=missing`，均未标记 `e2e-passed`。

## 验证与证据

- 定向页面与路由测试：23 项通过。
- Dashboard 全量测试：67 个测试文件、429 项测试通过。
- TypeScript 类型检查、变更文件 Lint、生产构建、53 页 benchmark 清单检查和 `git diff --check` 通过。
- Docker 仅原地重建 `mochat-go-desktop` 的 `app` 服务；MySQL、Redis 未重建，四个既有数据卷继续存在并挂载。
- 本地浏览器完成精准群发双标签切换、任务名称查询/重置，以及朋友圈任务/素材标签和阻断按钮检查。
- 浏览器截图：`screenshots/precise-group-send.png`、`screenshots/friends-circle.png`。
