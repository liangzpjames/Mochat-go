# 企业微信生产同步修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复客户同步错误纳入初始化占位员工的问题，并在服务器完成真实企微全量同步及 Dashboard 验收。

**Architecture:** 复用成员同步已持久化的 `contact_auth` 授权事实，在 MySQL 查询边界过滤客户同步成员。部署只替换 `app`，通过 API、数据库和浏览器三层证据验证结果。

**Tech Stack:** Go、MariaDB 10.6、Redis 7、Docker Compose、React/Vite、企业微信 API、Codex Browser。

## Global Constraints

- 不创建工作分支，保持 `main`。
- 不删除或重建 Docker 命名卷。
- 不输出 JWT、企微 Secret、Token、EncodingAESKey 或服务器密码。
- 代码从本地同步到服务器并在服务器编译。
- 用户审阅文档使用中文。

---

### Task 1: 客户同步成员查询契约

**Files:**
- Modify: `internal/store/mysql.go`
- Create: `internal/store/work_contact_sync_query_test.go`

**Interfaces:**
- Consumes: `MySQLStore.WorkContactSyncEmployees(ctx context.Context, corpID int)`
- Produces: `workContactSyncEmployeesQuery() string`

- [ ] **Step 1: 写失败测试**

```go
func TestWorkContactSyncEmployeesQueryOnlySelectsAuthorizedCustomerContactEmployees(t *testing.T) {
	query := strings.Join(strings.Fields(workContactSyncEmployeesQuery()), " ")
	for _, fragment := range []string{
		"corp_id = ?", "contact_auth = 1", "wx_user_id <> ''", "deleted_at IS NULL",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("query missing %q: %s", fragment, query)
		}
	}
}
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `go test ./internal/store -run '^TestWorkContactSyncEmployeesQueryOnlySelectsAuthorizedCustomerContactEmployees$' -count=1`

Expected: FAIL，提示 `undefined: workContactSyncEmployeesQuery`。

- [ ] **Step 3: 写最小实现并接入真实查询**

```go
func workContactSyncEmployeesQuery() string {
	return `
		SELECT id, wx_user_id
		FROM mc_work_employee
		WHERE corp_id = ? AND contact_auth = 1 AND wx_user_id <> '' AND deleted_at IS NULL
		ORDER BY id ASC
	`
}
```

`WorkContactSyncEmployees` 使用 `s.db.QueryContext(ctx, workContactSyncEmployeesQuery(), corpID)`。

- [ ] **Step 4: 运行目标测试并确认 GREEN**

Run: `go test ./internal/store -run '^TestWorkContactSyncEmployeesQueryOnlySelectsAuthorizedCustomerContactEmployees$' -count=1`

Expected: PASS。

- [ ] **Step 5: 运行相关回归测试**

Run: `go test ./internal/dashboard ./internal/store -count=1`

Expected: PASS。

- [ ] **Step 6: 提交独立修复**

```bash
git add internal/store/mysql.go internal/store/work_contact_sync_query_test.go
git commit -m "fix(wecom): filter contact sync employees"
```

### Task 2: 本地发布门禁

**Files:**
- Verify: `Dockerfile`
- Verify: `deploy/standalone/docker-compose.yml`
- Verify: `internal/dashboard/corp_admin_wecom.go`
- Verify: `web/apps/dashboard`

**Interfaces:**
- Consumes: Task 1 的客户同步查询修复及现有企微联调改动
- Produces: 可部署的本地源码状态和完整门禁证据

- [ ] **Step 1: 运行 Go 全量测试**

Run: `go test ./... -count=1`

Expected: PASS。

- [ ] **Step 2: 运行 Dashboard 测试**

Run: `pnpm --filter @mochat/dashboard test`

Expected: 所有测试 PASS。

- [ ] **Step 3: 运行 Dashboard 类型和构建检查**

Run: `pnpm --filter @mochat/dashboard typecheck && pnpm --filter @mochat/dashboard build`

Expected: 两个命令退出码均为 0。

- [ ] **Step 4: 检查补丁格式**

Run: `git diff --check`

Expected: 无错误。

### Task 3: 服务器部署与真实同步

**Files:**
- Sync to: `/opt/mochat-go`
- Preserve: `/opt/mochat-go/output/wecom-server-sync-20260809/pre-sync.sql.gz`

**Interfaces:**
- Consumes: Task 2 已验证源码
- Produces: 新 `standalone-app` 镜像、健康运行容器及企微同步结果

- [ ] **Step 1: 记录容器、镜像、卷和源文件哈希**

Run: `docker compose ls && docker ps && docker volume ls && sha256sum Dockerfile deploy/standalone/docker-compose.yml internal/dashboard/corp_admin_wecom.go internal/store/mysql.go`

Expected: 记录完整且四个命名卷仍存在。

- [ ] **Step 2: 从本地同步源码到服务器**

使用归档排除 `.git`、`node_modules`、缓存和 `output`，覆盖 `/opt/mochat-go` 中对应源码；不删除服务器 `output` 和数据卷。

- [ ] **Step 3: 在服务器编译并只重建 app**

Run: `docker compose -f deploy/standalone/docker-compose.yml build app && docker compose -f deploy/standalone/docker-compose.yml up -d --no-deps --force-recreate app`

Expected: 构建退出码 0，MySQL/Redis 容器 ID 不变，app 健康。

- [ ] **Step 4: 运行健康检查**

Run: `curl -f http://127.0.0.1/healthz && curl -f http://127.0.0.1/readyz`

Expected: HTTP 200。

- [ ] **Step 5: 连续两轮执行企微同步**

依次调用：

```text
PUT /dashboard/workEmployee/synEmployee
PUT /dashboard/workContactTag/synContactTag
PUT /dashboard/workContact/synContact
PUT /dashboard/workRoom/syn
```

Expected: 两轮全部 HTTP 200；第二轮数据库实体计数不重复增长。

- [ ] **Step 6: 核对数据库**

核对 `mc_work_department`、`mc_work_employee`、`mc_work_contact_tag_group`、`mc_work_contact_tag`、`mc_work_contact`、`mc_work_contact_employee`、`mc_work_room`、`mc_work_contact_room` 的有效记录数、`corp_id=1` 归属和更新时间。

Expected: 同步数据均归属于有效企业，客户同步结果非错误状态。

### Task 4: Dashboard 浏览器验收

**Files:**
- Evidence: `output/wecom-server-sync-20260809/`

**Interfaces:**
- Consumes: Task 3 的已同步服务器
- Produces: 页面可见数据、接口状态、控制台和截图证据

- [ ] **Step 1: 建立短时验收会话**

使用服务器容器内 JWT 配置生成短时令牌，不修改管理员密码，并将令牌注入浏览器 `ACCESS_TOKEN`；令牌不写入验收文档。

- [ ] **Step 2: 验收核心页面**

访问 `/department/index`、`/workEmployee/index`、`/workContactTag/index`、`/workContact/index`、`/workRoom/index`、`/index`。

Expected: 页面加载成功，显示与数据库相符的真实数据或真实空状态。

- [ ] **Step 3: 验收同步按钮和刷新**

在具备权限的页面执行安全同步/刷新，确认成功反馈与数据回读。

Expected: 无本地接口 4xx/5xx，无控制台错误。

- [ ] **Step 4: 清理临时凭据并整理证据**

删除服务器 `/tmp/mochat-dashboard-token` 与 `/tmp/mochat-token-helper` 及容器内对应临时程序，保留数据库备份和无敏感信息的验收证据。
