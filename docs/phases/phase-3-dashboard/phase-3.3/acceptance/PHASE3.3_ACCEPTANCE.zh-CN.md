# Phase 3.3 会话与风险预警验收记录

## 验收范围

本阶段仅验收 `README.md` 列出的 15 个路由：会话 9 页、风险预警 6 页。阶段外页面即使已可访问，也不得计入 Phase 3.3 完成度。

## 前置阻塞条件

以下任一条件不满足时，阶段状态保持 `blocked` 或 `in-progress`，不得把页面标记为真实完成：

- 没有可用且合规的会话存档数据源和授权范围；
- 没有脱敏测试租户、双企业/多角色数据和隔离 provider；
- 导出、媒体下载、消息拦截等高风险操作没有防误用门禁；
- 页面仅有菜单、路由、静态 fixture 或旧截图，没有真实查询/命令证据。

## 验收主流程

```text
登录
→ 从菜单进入目标页面
→ 查询或执行一个真实业务命令
→ 验证页面反馈和服务端结果
→ 刷新页面
→ 再次查询验证持久化或任务状态
→ 更换企业、角色或数据范围验证隔离
```

## 非浏览器门禁

实施完成后至少执行：

```bash
node --test scripts/check_yuanhu_benchmark_manifest.test.mjs
node scripts/check_yuanhu_benchmark_manifest.mjs
go test ./...
pnpm --filter @mochat/dashboard typecheck
pnpm --filter @mochat/dashboard build
```

会话存档、导出、媒体、风险事件和消息拦截还必须补充对应的 MariaDB 集成、HTTP 合同、React 交互、幂等/并发和 provider 隔离测试。命令结果、提交号和证据路径统一写入 `reports/`，不得预先宣称通过。

## 浏览器验收记录

浏览器验收必须在真实本地部署环境中执行，固定桌面基线 `1440x1000`，并分别覆盖：菜单进入、直接 URL、刷新、前进后退、空数据、无权限、服务异常、写操作后重新查询和双企业/多角色隔离。最终截图和结果 JSON 归档到 `evidence/`。

当前状态：`未开始`。
