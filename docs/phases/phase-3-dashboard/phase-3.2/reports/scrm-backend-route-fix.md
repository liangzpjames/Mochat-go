# SCRM 后端路由问题修复记录

## 问题现象

- `/customer/contact` 与 `/customer/clue/default` 页面进入后显示“加载失败”。
- 已登录请求 `/dashboard/scrm/contacts`、`/dashboard/scrm/leads` 均返回 HTTP 501，响应提示该路由尚未迁移。

## 根因

联系人与线索池的后端 handler 和路由注册代码已经存在，但独立 Docker 部署将
`MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT` 默认设为 `0`。因此 `mochat-go-desktop`
未注册 `/dashboard/scrm/*` 路由，请求落入服务器的 501 兼容占位响应。

## 修复

- 将 `deploy/standalone/docker-compose.yml` 中 SCRM 开关的独立部署默认值改为 `1`。
- 保留程序配置层的默认关闭行为，避免影响独立部署以外的运行模式。
- 增加部署配置回归测试，防止独立部署再次漏开 SCRM 路由。

## 验证

- 回归测试经过失败到通过的红绿验证。
- SCRM、应用启动、迁移相关 Go 测试通过。
- 使用 `docker compose ... up -d --build` 原地重建 app 容器，未删除 MySQL、Redis 或数据卷。
- 容器内环境变量确认为 `MOCHAT_GO_ENABLE_PHASE2_2_SCRM_PILOT=1`，app、MySQL、Redis 均为 healthy。
- 浏览器刷新联系人和线索池后，两个页面均能完成后端读取并显示正常的空数据状态，不再显示“加载失败”。

