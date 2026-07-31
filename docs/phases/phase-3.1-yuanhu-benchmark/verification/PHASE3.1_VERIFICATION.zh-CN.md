# Phase 3.1 验收记录

## 自动化门禁

执行命令：

```bash
pnpm check:yuanhu-phase1
```

验收项：

| 项目 | 结果 |
| --- | --- |
| Yuanhu manifest 校验 | 通过 |
| Dashboard typecheck | 通过 |
| Dashboard 单元测试 | 284 tests passed |
| Dashboard production build | 通过 |
| E2E typecheck | 通过 |
| Phase 3.1 Playwright gate | 1 passed |

## 关键行为

1. Dashboard 显示 `MoChat AI`，不再显示圆弧品牌文字。
2. 左侧菜单初始收缩，点击菜单组后显示对应菜单项。
3. Dashboard 使用 `mochat_dashboard_*` 存储键；SaaS Admin 使用独立的 `mochat_go_saas_admin_token`。
4. Dashboard 与 SaaS Admin 退出登录不会互相删除对方的会话存储。

## 说明

旧版全量截图回归不属于本阶段门禁；Phase 3.1 使用独立 gate，避免历史截图资产阻塞新功能验收。
