# 移动端视觉优化 Task 2 交付记录

## 范围

本任务仅升级 Sidebar 的员工工作台、客户资料和待迁移页面组合。生产改动严格限定在 brief 列出的 7 个 Sidebar 文件：新增 `SidebarPageShell` 及其测试，修改路由、客户页、对应测试和 `styles.css`。未修改共享包、Operation、认证会话合同、路由注册表或 manifest；未操作 Docker、数据库、服务器、数据卷或 `output`。

## TDD 证据

### RED

先新增页面壳、路由和客户资料断言后运行：

```powershell
corepack pnpm --filter @mochat/sidebar test -- src/ui/sidebar-page-shell.test.tsx src/app/sidebar-router.test.tsx src/features/contact/contact-page.test.tsx
```

结果为预期失败，退出码 `1`：

- `sidebar-page-shell.test.tsx` 无法解析 `./sidebar-page-shell`，原因是页面壳尚未实现。
- 员工工作台测试找不到可用入口，原因是首页仍为待迁移状态。
- 客户资料测试找不到“企业编号：3”，空头像也没有可访问 `img` 语义。

实现最小页面壳、资料卡和工作台后，`/codeAuth` 认证页仍显示员工导航；新增认证边界测试捕获后，改为无导航的共享 `MobileShell`、`MobileCard`、`MobileState` 组合。

内审发现业务链接原样携带 OAuth callback `state`。先补充底栏和工作台入口的回归断言并运行 RED：

```text
Expected: /contact?wxExternalUserid=external-1#profile
Received: /contact?wxExternalUserid=external-1&state=callback-state#profile
```

随后集中实现 `sidebarBusinessContextSuffix`，保留业务 query/hash 并剔除 `state`，由底栏和工作台入口共同使用。

### GREEN

修复后重跑指定测试命令：

```text
Test Files  6 passed (6)
Tests  60 passed (60)
```

## 实现清单

- `SidebarPageShell` 是唯一持有 `MobileBottomNavigation` 的 Sidebar 组合层；认证业务页显示“客户 / 会话 / 我的”。
- 底栏和工作台入口保留业务 query/hash，过滤 callback `state`；当前项正确提供 `aria-current="page"`。
- `/login`、`/auth`、`/codeAuth` 与未知路由均无员工导航；12 条 manifest 路由和未知 404 保持不变。
- 首页使用 CSS 渐变 hero 和由已注册认证业务路由派生的入口宫格，不展示虚构指标、成功状态或可写操作。
- `/contact` 仅展示真实 `ContactSummary` 的名称、客户编号、企业编号和头像；空头像是带可读名称的中性占位图，未推测手机号、标签或负责人。
- 待迁移模块使用共享卡片和状态组件，不含假按钮；样式保留 44px 触控下限、底栏内容预留和 320px 单列回退。

## 完整门禁

```powershell
corepack pnpm --filter @mochat/sidebar lint
# exit 0

corepack pnpm --filter @mochat/sidebar typecheck
# exit 0

corepack pnpm --filter @mochat/sidebar test
# 6 passed files, 60 passed tests, exit 0

corepack pnpm --filter @mochat/sidebar build
# exit 0；38 modules transformed

git diff --check
# exit 0
```

## 自审与复审

- 已核对改动范围仅为 brief 列出的 Sidebar 源码/测试文件；本交付记录单独提交。
- 已核对无直接 session/storage 读取新增到 feature 页面；会话、Cookie 和重认证逻辑未改变。
- 独立只读审查首轮发现 1 个 Important：callback `state` 泄漏到业务导航。已用集中 helper 与两处 RED→GREEN 回归测试整改；复审结论为 `READY`，无 Critical/Important/Minor 遗留。

## 提交

- 功能提交：`045be25 feat(sidebar): apply employee workbench visual system`

## Concerns

无 Task 2 范围内阻塞项。390×844/320px 的 CSS 约束与单元门禁已覆盖；全路由浏览器实测属于后续整合验收，不在本 Task 2 中宣称完成。
