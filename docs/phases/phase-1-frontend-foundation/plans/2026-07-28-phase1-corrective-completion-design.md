# Phase 1 发布与可用性收尾设计

## 决策

Phase 1 剩余工作采用“定向收尾”方案：修复已经承诺但实际未闭环的发布和基础可用性，不扩大为全量业务页面迁移。

对比过的方案：

1. **只修 Docker 打包**：最快，但动态菜单和登录页基本视觉层级仍不满足 Phase 1 完成定义。
2. **发布 + 导航 + 登录页收尾（采用）**：补齐 Phase 1 已有承诺，同时保持业务页面迁移边界。
3. **直接重做所有旧管理页面**：会把 Phase 2 混入 Phase 1，无法保持逐页切换和独立回滚。

## 目标

- Docker 镜像同时构建并包含 Dashboard 与 SaaS Admin。
- `/saas-admin/` 返回 SaaS Admin 自己的 HTML 和 `/saas-admin/assets/`。
- Dashboard 根据后端权限菜单显示可点击导航，React 与 legacy 路由继续由 manifest 决定。
- 登录页使用现有 Ant Design 依赖形成可用的居中登录卡片，保留现有认证、安全返回和错误处理。
- 新增自动化回归，防止 HTTP 200 掩盖错误 SPA 回退。
- 更新 Phase 1 验收与总进度，重新部署供人工测试。

## 发布链路

Docker 前端构建阶段定向执行：

```text
pnpm --filter @mochat/dashboard build
pnpm --filter @mochat/saas-admin build
```

最终镜像分别从 `frontend-build` 复制两个应用的 `dist`。同步脚本使用同样的应用集合。发布静态测试必须同时断言：

- Dashboard HTML 引用 `/assets/`；
- SaaS Admin HTML 引用 `/saas-admin/assets/`；
- 两个入口 HTML 不相同；
- 两类静态资源都能请求成功。

## Dashboard 导航

`AccessContext` 保存后端返回的原始菜单树及计算后的权限集合。布局只渲染 `allowedRoutes` 中的内部页面链接，保留后端菜单名称和分组层级。点击任何页面仍先进入 React Router：

- manifest 为 `react`：由 React 页面渲染；
- manifest 为 `legacy`：受控 loader 跳转到 legacy mount；
- 未授权或未知：保持 403/404。

布局头部提供 Dashboard 首页和 SaaS Admin 的明确入口。没有菜单数据时显示空状态，不伪造权限。

## 登录页

继续使用 React Hook Form、Zod 和现有认证 API，只替换表现层：

- 全屏品牌背景；
- Ant Design `Card`、`Input`、`Button` 和排版组件；
- 明确的手机号、密码标签和内联错误；
- 提交中状态、服务端错误及浏览器自动填充保持不变；
- 移动端卡片自适应。

不复制许可证不明的旧图片，不改变登录字段、API 或 returnTo 安全规则。

## 测试

1. 静态发布链路测试先在现状失败，再驱动 Dockerfile 和同步脚本修复。
2. Access loader 测试菜单树进入上下文。
3. 布局测试授权菜单链接、空菜单和 SaaS Admin 入口。
4. 登录测试语义化表单及 UI 组件 class，原有行为测试继续通过。
5. Dashboard 与 SaaS Admin 分别生产构建。
6. Docker Compose 重建后进行 HTTP 身份校验和浏览器烟测。

## 完成标准

- 新增回归测试与现有 Dashboard 测试通过。
- 两个前端应用生产构建通过。
- 容器健康，`/` 与 `/saas-admin/` 返回不同应用。
- 登录页和授权导航可在浏览器中使用。
- Phase 1 状态改为完成，并建立 Phase 2 设计入口。

