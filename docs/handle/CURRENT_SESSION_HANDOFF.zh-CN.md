# 当前会话交接

> 更新时间：2026-07-27

## 当前状态

- 正式开发分支：`phase1/frontend-unification-foundation`
- 独立 worktree：`D:\workspace\mochat-go\mochat-go\.worktrees\frontend-unification-foundation`
- 分支基线提交：`e4289f4`
- 当前 worktree 尚未开始前端源码恢复或 React 工程改造。
- 不再继续 Phase 0；新会话应进入正式前端开发规划。
- 本地 `.workbuddy/` 属于用户文件，不得提交或删除。

## 前端源码位置

旧前端完整源码位于远程分支：

```text
origin/backup/pre-phase0-main-20260723
```

已确认包含：

- `dashboard/`：Vue 2，约 265 个源码文件、130 个 Vue 页面/组件；
- `sidebar/`：Vue 3，约 45 个源码文件、19 个 Vue 页面/组件；
- `operation/`：Vue 2，约 38 个源码文件、14 个 Vue 页面/组件；
- `api-server/`：旧 PHP 后端，仅作行为和接口参照，不并入正式运行代码。

当前 Go 分支中的 `web/dashboard/dist`、`web/sidebar/dist`、`web/operation/dist` 是旧构建产物；`web/saas-admin` 已有 React/TypeScript 源码。

## 已确认的正式开发方向

1. 统一前端技术栈为 React + TypeScript。
2. 先完成 Dashboard，再迁移 Sidebar，最后迁移 Operation。
3. 第一阶段优先保持原有业务行为、路由、字段、权限和基本视觉，不同时重做产品流程。
4. 采用 pnpm workspace：
   - `web/apps/`：dashboard、sidebar、operation、saas-admin；
   - `web/packages/`：api-client、auth、routing、ui、testing、config；
   - `web/legacy/`：恢复的旧三端源码，仅用于参照和临时兼容。
5. React 立即作为单一主入口；未迁移路由由受控兼容层承接。
6. 每个页面通过 API 契约、权限、租户、浏览器和基本视觉回归后，才从 legacy 切换到 React。
7. Go 继续作为唯一后端，不恢复旧 PHP 运行时。
8. 不引入微前端框架、SSR、RSC 或实验性路由能力。

## 暂定技术栈

- React 19.2
- TypeScript strict
- Vite
- React Router Data Mode
- TanStack Query
- Zustand（仅客户端状态）
- React Hook Form + Zod
- Ant Design
- Vitest + Testing Library
- Playwright
- MSW
- pnpm workspace

具体 patch 版本应在实施前重新核对并由 lockfile 固定。

## 新会话下一步

新会话不要立即修改前端代码，先完成一份详细实施计划，至少覆盖：

1. 从备份分支提取旧三端源码和许可证信息；
2. 建立页面、路由、API、权限、资源和依赖审计清单；
3. 建立 pnpm workspace 与共享包；
4. 将现有 SaaS Admin 纳入 workspace；
5. 建立 Dashboard React Shell；
6. 实现登录、企业上下文、菜单权限、错误处理和兼容路由；
7. 建立单元、契约、Playwright、构建和 CI 门禁；
8. 明确每个任务的文件、测试命令、验收条件、预计用时、难度和独立提交点。

所有后续规划、设计、验收和交接文档继续放在：

```text
docs/handle/
```
