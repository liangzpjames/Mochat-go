# SaaS 总后台独立部署路由修复

## 问题

独立部署访问 `/saas-admin/` 时显示 Dashboard 的“页面不存在”，而不是 SaaS 总后台。

## 原因

`deploy/standalone/docker-compose.yml` 将 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD` 默认设为 `0`。服务因此没有挂载 `/saas-admin/` SPA，访问请求落入 Dashboard 的兜底路由。SaaS Admin 构建产物本身已存在，未受到 phase3.2 前端改动破坏。

## 修复

- 独立部署默认启用 `MOCHAT_GO_ENABLE_SAAS_ADMIN_DASHBOARD`。
- 增加部署配置回归测试，防止默认开关再次关闭。
- 使用 `mochat-go-desktop` 原项目名重建应用容器，未删除或重建数据卷。

## 验证

- 配置、前端挂载、迁移及主程序相关 Go 测试通过。
- 容器健康检查为 `healthy`，运行环境中开关值为 `1`。
- `/saas-admin/` 返回 HTTP 200，页面标题为“MoChat SaaS 总后台”。
- 浏览器实际渲染进入总后台概览，可见客户租户、套餐、平台账号、系统运维和上线检查等导航，不再显示“页面不存在”。
