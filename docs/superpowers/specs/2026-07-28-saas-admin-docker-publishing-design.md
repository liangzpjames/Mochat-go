# SaaS Admin Docker 发布链路修复设计

## 目标

保留 `web/apps/saas-admin` 现有目录结构，修复 Docker 镜像未包含 SaaS Admin 构建产物的问题，使 `/saas-admin/` 返回 SaaS Admin 自己的 HTML 和静态资源，而不是回退到 Dashboard。

## 范围

- 在 Docker 前端构建阶段定向构建 `@mochat/dashboard` 和 `@mochat/saas-admin`。
- 在最终镜像中复制 `web/apps/saas-admin/dist`。
- 同步更新前端产物同步脚本，避免该发布入口继续遗漏 SaaS Admin。
- 增加或强化回归验证，校验 SaaS Admin HTML 引用了 `/saas-admin/assets/`，并与 Dashboard HTML 区分。
- 重新构建并部署当前 Docker Compose 项目，提供本机地址供人工测试。

## 非目标

- 不调整 SaaS Admin 源码目录。
- 不改造登录页视觉样式。
- 不实现 Dashboard 菜单或其他 Phase 1 页面迁移。
- 不改动 SaaS Admin 业务功能。

## 实现方案

Dockerfile 继续使用现有 pnpm workspace，只增加 `@mochat/saas-admin` 的定向构建命令，并将其 `dist` 复制到运行镜像的 `/app/web/apps/saas-admin/dist`。同步脚本使用相同的应用集合，确保本地同步和镜像发布语义一致。

回归测试首先证明当前 Dockerfile/同步脚本缺少 SaaS Admin 构建或复制步骤，然后以最小改动使测试通过。部署验证同时检查：

1. `/` 返回 Dashboard 资源；
2. `/saas-admin/` 返回 `/saas-admin/assets/` 资源；
3. 两个入口的 HTML 不相同；
4. SaaS Admin 的关键静态资源可成功获取。

## 错误处理

构建阶段若 SaaS Admin 编译失败或 `dist` 不存在，Docker 构建应直接失败，不产生缺少管理后台的镜像。运行时不依赖 Dashboard fallback 掩盖缺失产物。

## 完成标准

- 发布链路回归测试通过。
- Dashboard 和 SaaS Admin 均成功完成生产构建。
- Docker 镜像重新构建并启动。
- `/saas-admin/` 可访问且确认返回 SaaS Admin 应用。
