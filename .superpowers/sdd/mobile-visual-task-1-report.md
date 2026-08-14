# 移动端视觉优化 Task 1 交付记录

## 范围

仅修改 `@mochat/mobile-foundation` 共享视觉基础：视觉令牌、卡片、图标入口、底部导航、页面壳可选 `eyebrow`/`hero` 与状态装饰图形。未改动 Sidebar、Operation、Docker、服务器、数据库、数据卷或 `output`。

## TDD 证据

### RED

先新增 `mobile-card.test.tsx`、`mobile-navigation.test.tsx`，并补充 `mobile-shell.test.tsx`；随后运行：

```powershell
corepack pnpm --filter @mochat/mobile-foundation test
```

结果为预期失败（退出码 1）：

- `mobile-card.test.tsx` 无法解析 `./mobile-card`；生产组件尚不存在。
- `mobile-navigation.test.tsx` 无法解析 `./mobile-navigation`；生产组件尚不存在。
- `MobileShell` 找不到 `客户运营`，说明尚未渲染 `eyebrow`/`hero`。
- `MobileState` 找不到 `mobile-state-graphic`，说明尚未渲染装饰图形。

以上失败均由待实现需求导致，而非测试拼写或环境错误。

### GREEN

实现最小生产代码后，测试环境报告 `Invalid Chai property`，根因是现有 Vitest 配置未注册 `jest-dom` 匹配器，而非组件行为失败。未新增依赖，已将新增断言改为现有项目可用的 DOM 原生属性检查。随后重新运行同一命令：

```text
Test Files  5 passed (5)
Tests  30 passed (30)
```

## 实现清单

- `MobileCard({ tone, padding, children })`：`surface`、`accent`、`muted` 三种视觉层级，以及 `none`、`compact`、`comfortable` 三种内边距。
- `MobileIconTile`：有 `href` 时为链接；禁用时渲染无链接的 `aria-disabled="true"` 容器，避免空链接。
- `MobileBottomNavigation`：具名 `nav`、项目链接和当前项 `aria-current="page"`，并包含安全区样式类。
- `MobileShell`：新增可选 `eyebrow` 与 `hero`，原有调用保持兼容。
- `MobileState`：新增 `aria-hidden="true"` 的内联 SVG 装饰图形。
- 样式令牌：浅蓝背景、白色表面、蓝色渐变、20px 卡片圆角、44px 触控下限、安全区和 reduced-motion 规则。

## 门禁结果

```powershell
corepack pnpm --filter @mochat/mobile-foundation lint
# exit 0

corepack pnpm --filter @mochat/mobile-foundation typecheck
# exit 0

corepack pnpm --filter @mochat/mobile-foundation test
# 5 passed files, 30 passed tests, exit 0

corepack pnpm --filter @mochat/mobile-foundation build
# exit 0
```

## 自审

- `git diff --check` 无空白错误。
- 变更仅位于 brief 指定的共享包文件及本交付记录。
- 新组件未读取 router、auth 或 storage，亦无直接 `fetch`、外部图片 URL 或参考品牌素材。
- 固定底部导航使用安全区，并由页面内容底部预留高度避免遮挡。

## 内审整改

只读代码审查发现并已在提交前整改：

- 新增显式 `bottomNavigation` 槽位；只有实际提供导航时才添加 `mobile-shell--has-bottom-navigation` 并预留底部空间，未使用导航的既有页面保持原布局。
- 保留设计指定的浅色主/次色 token，同时新增高对比的 `primary-text` 与 `muted-text` token 供 12–13px 文字使用。
- 底部导航标签和入口角标增加最大宽度、安全换行与收缩规则。
- 导航高度与内容预留共用固定 token，导航标签最多两行，避免长文本增长后遮挡内容。
- 禁用入口测试改为同时传入 `href`，确认不会产生可点击链接；卡片测试覆盖 `muted`/`comfortable`；新增显式导航槽位回归测试。

整改后的共享包测试结果：`5 passed files, 30 passed tests`。

## 提交

提交信息：`feat(mobile): add WeCom-inspired visual primitives`。

## Concerns

无 Task 1 范围内阻塞项。390×844 的真实浏览器全路由视觉验收属于后续 Sidebar/Operation 组合任务，未在本共享包任务中宣称完成。
