# Phase 3.4 第一批渠道获客页面实施报告

日期：2026-08-03

## 实施范围

- `/acquisition/v2-channel-code` 渠道活码：接入现有 `/dashboard/channelCode/index` 真实查询，覆盖名称筛选、刷新、空数据、异常重试、表格和详情抽屉。
- `/acquisition/group-code` 群活码：接入现有 `/dashboard/workRoomAutoPull/index` 查询，展示群二维码配置与关联群聊；现有接口没有统一扫码统计口径的字段不伪造。
- `/acquisition/live-code-short-chain` 活码短链：完成圆弧对应的筛选区、列表字段和 Provider 状态界面；当前没有正式短链创建、跳转和访问统计 Provider，创建与查询保持禁用。

## 圆弧对照结果

三页均采用 Dashboard 统一侧栏与顶部栏，内容区覆盖圆弧已观察到的标题、筛选栏、数据连接状态、表格字段、空状态和主要操作层级。页面不要求像素完全一致，但在 `1440x1000` 下保持相同的信息层级、紧凑筛选结构和白底表格卡片。

截图：

- `evidence/channel-code-page.png`
- `evidence/group-code-page.png`
- `evidence/live-code-short-chain-page.png`

## 验证记录

| 验证项 | 结果 |
| --- | --- |
| Phase 3.4 页面测试与注册测试 | 通过，定向测试覆盖真实端点、筛选、详情、错误重试、权限兼容和 Provider 阻断 |
| TypeScript | 通过 |
| 本批修改文件 ESLint | 通过 |
| Dashboard 生产构建 | 通过 |
| standalone 镜像前端与 Go 构建 | 首次完整构建通过 |
| standalone `readyz` | `200`，使用替代端口 `18090` 完成页面浏览 |
| 浏览器检查 | 三个路由可达、菜单高亮、无白屏和 404，页面结构与目标一致 |
| 仓库全量 ESLint | 未通过；共有 175 个既存错误，集中在 Phase 3.3、SCRM 和敏感词等旧文件，本批新增文件单独检查通过 |
| 第二次 Docker 镜像导出 | Docker Desktop 在导出层时出现本地 `input/output error`；此前镜像与浏览环境仍可用，代码级生产构建不受影响 |

## Manifest 状态

- 渠道活码：`native / ready / unit-passed / phase 3.4`。
- 群活码：`native / partial / unit-passed / phase 3.4`；等待通用群活码与扫码统计合同后才能提升 backend。
- 活码短链：`native / missing / unit-passed / phase 3.4`；页面完成但 Provider 未完成，不标记集成或 E2E 通过。

## 遗留事项

1. 渠道活码的创建、编辑、分组、客户明细和统计仍需在后续第一批迭代继续接入新 React 交互。
2. 群活码需要补齐独立业务合同，避免长期用自动拉群语义承接通用群活码。
3. 活码短链需要正式领域模型、存储、跳转安全校验、访问统计和审计 Provider。
4. Docker Desktop 本地镜像导出故障需要在环境恢复后重新构建并补一轮最新动作按钮截图。
