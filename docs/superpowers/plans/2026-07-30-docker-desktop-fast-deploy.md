# Docker Desktop 快速重复部署实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 提供一个可重复执行的 PowerShell 脚本，在 Docker Desktop 中重新构建并覆盖同一套 MoChat Go 容器，默认保留数据，并支持显式清空数据。

**Architecture:** 使用现有 `deploy/standalone/docker-compose.yml` 作为唯一编排来源。PowerShell 入口负责参数、前置检查、Compose 生命周期、迁移、初始化、健康等待和四前端 HTTP 验收；测试通过注入假的 Docker 命令验证安全边界，不依赖真实容器。

**Tech Stack:** PowerShell 5.1+、Docker Desktop、Docker Compose v2、现有 Go standalone 镜像。

## 全局约束

- 所有用户可见输出和审阅文档使用中文。
- 默认项目名固定为 `mochat-go-desktop`。
- 默认保留 `mysql-data`、`redis-data`、`app-storage` 和 `audit-anchor-storage`。
- 只有显式传入 `-ResetData` 时才允许执行带 `--volumes` 的 Compose down。
- 脚本只操作当前 Compose 项目，不执行全局镜像、容器或卷清理。
- 默认端口为 Dashboard `18080`、Sidebar `18081`、Operation `18082`；SaaS Admin 复用 Dashboard 端口。

---

### Task 1: 部署脚本安全命令生成

**Files:**
- Create: `scripts/deploy_docker_desktop.ps1`
- Create: `scripts/test_deploy_docker_desktop.ps1`

**Interfaces:**
- Consumes: `deploy/standalone/docker-compose.yml`
- Produces: `scripts/deploy_docker_desktop.ps1 [-ResetData] [-ProjectName <name>] [-DashboardPort <port>] [-SidebarPort <port>] [-OperationPort <port>] [-AdminPhone <phone>] [-AdminPassword <password>]`

- [ ] **Step 1: 编写失败测试**

测试建立临时 `docker.cmd`，记录参数并模拟 `docker info`、`docker compose version/config/down/up/ps/exec/logs`。分别执行默认部署和 `-ResetData` 部署，断言：

```powershell
if ($defaultLog -match 'down.*(--volumes|-v)') { throw '默认部署删除了数据卷' }
if ($resetLog -notmatch 'down.*--volumes') { throw 'ResetData 未删除数据卷' }
if ($defaultLog -notmatch '--project-name mochat-go-desktop') { throw '项目名不固定' }
if ($defaultLog -notmatch 'up -d --build --force-recreate --remove-orphans') { throw '缺少强制重建参数' }
```

- [ ] **Step 2: 运行测试并确认失败**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1`

Expected: FAIL，因为部署脚本尚不存在。

- [ ] **Step 3: 实现最小命令生成与 DryRun**

脚本参数包括：

```powershell
param(
  [switch]$ResetData,
  [switch]$DryRun,
  [string]$ProjectName = 'mochat-go-desktop',
  [ValidateRange(1, 65535)][int]$DashboardPort = 18080,
  [ValidateRange(1, 65535)][int]$SidebarPort = 18081,
  [ValidateRange(1, 65535)][int]$OperationPort = 18082,
  [string]$AdminPhone = '13800000000',
  [string]$AdminPassword = 'MochatLocal@123'
)
```

所有 Docker 调用经 `Invoke-Docker` 函数执行；`$ResetData` 为真时使用 `down --volumes --remove-orphans`，否则使用 `down --remove-orphans`；启动统一使用 `up -d --build --force-recreate --remove-orphans`。

- [ ] **Step 4: 运行测试并确认通过**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1`

Expected: PASS，并输出 `Docker Desktop 部署脚本测试通过`。

- [ ] **Step 5: 提交**

```powershell
git add scripts/deploy_docker_desktop.ps1 scripts/test_deploy_docker_desktop.ps1
git commit -m "feat: add reusable Docker Desktop deployment"
```

### Task 2: 健康等待、迁移和前端验收

**Files:**
- Modify: `scripts/deploy_docker_desktop.ps1`
- Modify: `scripts/test_deploy_docker_desktop.ps1`

**Interfaces:**
- Consumes: Compose 服务 `app`、`mysql`、`redis`；App `/readyz`
- Produces: 有上限的健康轮询、迁移、管理员初始化和四入口验收

- [ ] **Step 1: 编写失败测试**

扩展假 Docker，使 `compose ps --format json <service>` 返回健康状态；记录并断言存在：

```powershell
if ($log -notmatch 'exec -T app mochat-migrate -action up -project-root /app') { throw '未执行迁移' }
if ($log -notmatch 'exec -T app mochat-bootstrap') { throw '未执行管理员初始化' }
```

增加 `-SkipHttpCheck` 测试参数，使命令测试不访问网络；另模拟 `unhealthy`，断言脚本返回非零。

- [ ] **Step 2: 运行测试并确认失败**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1`

Expected: FAIL，提示缺少迁移、初始化或健康失败处理。

- [ ] **Step 3: 实现健康与验收流程**

实现：

```powershell
Wait-ComposeService -Service mysql -TimeoutSeconds 180
Wait-ComposeService -Service redis -TimeoutSeconds 180
Wait-ComposeService -Service app -TimeoutSeconds 300
Invoke-Compose exec -T app mochat-migrate -action up -project-root /app
Invoke-Compose exec -T app mochat-bootstrap -phone $AdminPhone -password $AdminPassword
```

Bootstrap 已存在时允许继续，其他非零状态失败。HTTP 检查对 `/readyz`、`/`、`/saas-admin/`、Sidebar 和 Operation 执行有限重试。

- [ ] **Step 4: 运行测试并确认通过**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1`

Expected: PASS。

- [ ] **Step 5: 提交**

```powershell
git add scripts/deploy_docker_desktop.ps1 scripts/test_deploy_docker_desktop.ps1
git commit -m "test: cover Docker Desktop deployment health flow"
```

### Task 3: Docker Desktop 双次部署验收

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: 完整部署脚本
- Produces: 可复制的中文使用说明和真实重复部署证据

- [ ] **Step 1: 检查 Docker Desktop**

Run: `docker info`

Expected: exit 0，Server 可用。

- [ ] **Step 2: 执行首次部署**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/deploy_docker_desktop.ps1
```

Expected: 三个服务健康，四个前端入口通过。

- [ ] **Step 3: 写入数据标记并记录容器 ID**

Run:

```powershell
docker compose --project-name mochat-go-desktop -f deploy/standalone/docker-compose.yml --profile app exec -T mysql mariadb -umochat -pmochat_pass mochat -e "CREATE TABLE IF NOT EXISTS mochat_deploy_probe (id INT PRIMARY KEY); INSERT IGNORE INTO mochat_deploy_probe VALUES (1);"
docker compose --project-name mochat-go-desktop -f deploy/standalone/docker-compose.yml --profile app ps -q app
```

Expected: 标记写入成功并输出 App 容器 ID。

- [ ] **Step 4: 执行第二次部署并验证覆盖与保留**

再次运行默认部署脚本，然后查询容器 ID 和：

```sql
SELECT COUNT(*) FROM mochat_deploy_probe WHERE id = 1;
```

Expected: App 容器 ID 改变，查询结果为 `1`，四个前端仍可访问。

- [ ] **Step 5: 补充中文 README 并完成验证**

README 只记录常用命令、默认保留数据和 `-ResetData` 风险。执行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test_deploy_docker_desktop.ps1
git diff --check
git status --short
```

Expected: 测试通过、无格式错误，只有计划内文件变化。

- [ ] **Step 6: 提交**

```powershell
git add README.md
git commit -m "docs: document Docker Desktop quick deployment"
```
