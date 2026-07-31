# MoChat 圆弧对标第一阶段验收

## 验收命令

在仓库根目录执行：

```bash
pnpm check:yuanhu-phase1
```

该命令依次验证菜单清单、Dashboard 类型检查、Dashboard 单元测试、Dashboard 生产构建、E2E 类型检查和第一阶段浏览器流程。

## 浏览器流程

- 访问 Dashboard 根路径并确认登录态可恢复。
- 确认页头显示 `MoChat AI`。
- 确认左侧八个菜单分组默认收缩。
- 展开“会话”分组，确认“全局消息”可访问。
- 刷新页面，确认品牌和菜单仍可渲染。
- 收集 console error；出现错误时 Playwright 输出截图和 trace 路径。

## 会话隔离

- Dashboard token：`mochat_dashboard_token`。
- SaaS Admin token：`mochat_go_saas_admin_token`。
- SaaS Admin 不再读取 Dashboard 的 `ACCESS_TOKEN`。
- 两端登出只清理自己的 token 和 cookie。

## 当前证据

最近一次单独浏览器流程：`yuanhu-phase1.spec.ts` 通过（1 passed）。完整门禁应以 `pnpm check:yuanhu-phase1` 的最终输出为准。
