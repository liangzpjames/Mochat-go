# Docker Desktop 快速重复部署设计

## 目标

为 Windows + Docker Desktop 开发环境提供一个可重复执行的 PowerShell 部署入口。每次执行都重新构建当前源码，并替换同一套 MoChat Go 容器，避免 Docker Desktop 中出现多个重复项目。

默认部署必须保留 MySQL、Redis、上传文件、备份和审计锚点数据。只有操作者显式传入 `-ResetData` 时，脚本才允许删除 Compose 项目的数据卷并进行全新初始化。

## 使用方式

主入口放在 `scripts/deploy_docker_desktop.ps1`，从仓库任意位置调用时都能自动定位项目根目录。

默认部署：

```powershell
.\scripts\deploy_docker_desktop.ps1
```

清空数据后重新部署：

```powershell
.\scripts\deploy_docker_desktop.ps1 -ResetData
```

脚本支持覆盖项目名、应用端口、Sidebar 端口、Operation 端口、管理员手机号和管理员密码。默认项目名保持固定，使重复执行始终操作同一套 Compose 资源。

## 部署流程

1. 检查 PowerShell、Docker CLI、Docker Compose 和 Docker Desktop 引擎是否可用。
2. 检查目标端口是否由当前 Compose 项目之外的进程或容器占用。
3. 准备本地部署环境变量，不向仓库写入真实密码。
4. 默认执行 `docker compose down --remove-orphans`，不附加 `--volumes`。
5. 使用 `docker compose up -d --build --force-recreate --remove-orphans` 重建并替换应用栈。
6. 等待 MySQL、Redis 和 App 健康检查通过。
7. 执行可重复的数据库迁移。
8. 首次部署或 `-ResetData` 部署时执行管理员初始化；重复部署时允许管理员已存在。
9. 检查 `/readyz` 以及四个前端入口。
10. 输出容器状态、管理员账号和访问地址。

## 数据安全

- 默认模式不执行 `docker compose down -v`，因此保留 `mysql-data`、`redis-data`、`app-storage` 和 `audit-anchor-storage`。
- `-ResetData` 是唯一允许删除卷的入口。执行前脚本用醒目的中文信息说明影响范围。
- 脚本只操作固定 Compose 项目名下的容器、网络和卷，不清理 Docker Desktop 中的其他项目、镜像或卷。
- 失败时不自动删除数据卷；保留现场并输出容器状态和最近日志。

## 健康检查与失败处理

健康等待采用有上限的轮询，不使用无限等待。任一关键服务进入 `unhealthy`、容器退出或等待超时，部署立即失败，并打印：

- `docker compose ps`
- App、MySQL、Redis 最近日志
- 建议重新执行的命令

HTTP 验收覆盖：

- Dashboard：`http://127.0.0.1:<应用端口>/`
- SaaS Admin：`http://127.0.0.1:<应用端口>/saas-admin/`
- Sidebar：`http://127.0.0.1:<Sidebar 端口>/`
- Operation：`http://127.0.0.1:<Operation 端口>/`

## 测试策略

PowerShell 测试通过注入一个假的 Docker 命令记录调用参数，验证：

- 默认部署不会出现 `--volumes` 或 `-v`。
- `-ResetData` 会对当前固定项目执行带卷删除的 `down`。
- 重复部署使用相同 Compose 项目名。
- 构建使用 `--build --force-recreate --remove-orphans`。
- Docker 不可用、健康检查失败或 HTTP 验收失败时返回非零退出码。

真实验收在 Docker Desktop 中连续运行两次默认部署。第二次部署后确认容器已替换、项目名未变化、数据库标记数据仍存在，并验证四个前端入口可访问。
