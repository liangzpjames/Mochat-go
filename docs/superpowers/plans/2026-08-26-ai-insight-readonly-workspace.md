# AI 洞察只读工作区修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 AI Provider Resolver 关闭时仍注册 AI 洞察只读工作区路由，使 5 个页面返回真实数据/空态和明确的 Provider 状态。

**Architecture:** AI 洞察 Module 在数据库存在时始终创建 WorkspaceHandler，并把可能为 `nil` 的租户 Provider Resolver 原样传入。WorkspaceHandler 继续负责区分只读查询、状态查询和模型运行；只有运行分析需要有效 Resolver。

**Tech Stack:** Go 1.22、`net/http`、内部 Module Router、`go test`、Docker Compose、Dashboard 浏览器冒烟。

## Global Constraints

- 不开启 `MOCHAT_GO_AI_INSIGHT_ENABLED`、日分析或启动即运行开关。
- 不创建或修改业务数据，不修改 AI Provider 密钥、企微配置和生产数据库。
- 只读接口必须保留租户、企业、员工范围和 RBAC 授权边界。
- 手动运行在 Resolver 为空时必须保持 `503 / AI_PROVIDER_UNAVAILABLE`，不得调用模型。
- 服务器部署只替换 app 镜像，不删除数据库、Redis、命名卷或现有配置。

---

### Task 1: 固化无 Resolver 时的模块路由合同

**Files:**
- Create: `internal/modules/ai-insight/module_test.go`
- Test: `internal/modules/ai-insight/module_test.go`

**Interfaces:**
- Consumes: `New(Dependencies) (*Module, error)`、`(*Module).RegisterRoutes(appmodules.RouteRegistrar) error`
- Produces: 无 Resolver 时 `workspace != nil`，且 Router 可匹配 records、status 和 run 路由的回归合同。

- [ ] **Step 1: 写失败测试**

```go
package aiinsight

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/DATA-DOG/go-sqlmock"
    appmodules "jiyi/mochat-go/internal/app/modules"
    transporthttp "jiyi/mochat-go/internal/modules/ai-insight/transport/http"
)

type modulePrincipalResolver struct{}

func (modulePrincipalResolver) Resolve(*http.Request) (transporthttp.Principal, error) {
    return transporthttp.Principal{UserID: 7, TenantID: 11, CorpID: 22}, nil
}

func TestModuleRegistersWorkspaceRoutesWithoutAIProviderResolver(t *testing.T) {
    db, _, err := sqlmock.New()
    if err != nil { t.Fatal(err) }
    t.Cleanup(func() { _ = db.Close() })

    module, err := New(Dependencies{PrincipalResolver: modulePrincipalResolver{}, DB: db})
    if err != nil { t.Fatal(err) }
    if module.workspace == nil { t.Fatal("workspace handler is nil without AI provider resolver") }

    router := appmodules.NewRouter()
    if err := module.RegisterRoutes(router); err != nil { t.Fatal(err) }
    for _, target := range []struct{ method, path string }{
        {http.MethodGet, "/dashboard/ai-insight/session-analysis/records"},
        {http.MethodGet, "/dashboard/ai-insight/session-analysis/status"},
        {http.MethodPost, "/dashboard/ai-insight/run"},
    } {
        if _, ok := router.Match(httptest.NewRequest(target.method, target.path, nil)); !ok {
            t.Fatalf("route not registered: %s %s", target.method, target.path)
        }
    }
}
```

- [ ] **Step 2: 运行测试确认红灯**

Run: `go test ./internal/modules/ai-insight -run TestModuleRegistersWorkspaceRoutesWithoutAIProviderResolver -count=1`

Expected: FAIL，错误包含 `workspace handler is nil without AI provider resolver`。

- [ ] **Step 3: 提交测试红灯证据后保留测试文件**

Run: `git diff --check && git diff -- internal/modules/ai-insight/module_test.go`

Expected: diff 格式正确，测试只覆盖模块装配和路由注册。

### Task 2: 最小化修复 WorkspaceHandler 装配

**Files:**
- Modify: `internal/modules/ai-insight/module.go:32-42`
- Test: `internal/modules/ai-insight/module_test.go`
- Test: `internal/modules/ai-insight/workspace_handler_test.go`

**Interfaces:**
- Consumes: `NewWorkspaceHandlerWithResolver(WorkspacePrincipalResolver, WorkspaceAuthorizer, Repository, providers.AIProviderResolver, ...any) *WorkspaceHandler`
- Produces: DB 非空时始终创建 WorkspaceHandler；Resolver 可为 `nil`。

- [ ] **Step 1: 写最小实现**

将条件创建逻辑改为：

```go
assistantRepo, _ := aisettingsmysql.NewAgentRepository(dependencies.DB)
workspace = NewWorkspaceHandlerWithResolver(
    workspacePrincipalAdapter{resolver: dependencies.PrincipalResolver},
    workspaceAuthorizer,
    NewSQLRepository(dependencies.DB),
    dependencies.AIProviderResolver,
    assistantRepo,
)
```

- [ ] **Step 2: 格式化并运行定向测试确认绿灯**

Run: `gofmt -w internal/modules/ai-insight/module.go internal/modules/ai-insight/module_test.go && go test ./internal/modules/ai-insight -run 'TestModuleRegistersWorkspaceRoutesWithoutAIProviderResolver|TestWorkspace' -count=1`

Expected: PASS，且无模型调用测试回归。

- [ ] **Step 3: 运行模块与启动装配测试**

Run: `go test ./internal/modules/ai-insight/... ./internal/app/bootstrap/... ./cmd/mochat-go/... -count=1`

Expected: 全部 PASS。

- [ ] **Step 4: 提交最小代码修复**

Run: `git add internal/modules/ai-insight/module.go internal/modules/ai-insight/module_test.go && git commit -m "fix(ai): register readonly insight workspace without provider"`

Expected: 提交仅包含模块实现与回归测试。

### Task 3: 本地全量验证与镜像构建

**Files:**
- Inspect: `deploy/standalone/docker-compose.yml`
- Inspect: `Dockerfile`
- No source changes expected.

**Interfaces:**
- Consumes: 修复提交 SHA。
- Produces: 可追溯 Docker 镜像及本地测试证据。

- [ ] **Step 1: 运行 Go 全量门禁**

Run: `go test ./... -count=1`

Expected: exit code 0。

- [ ] **Step 2: 运行格式与静态检查**

Run: `gofmt -l internal/modules/ai-insight/module.go internal/modules/ai-insight/module_test.go`

Expected: 无输出。

Run: `go vet ./internal/modules/ai-insight/... ./cmd/mochat-go/...`

Expected: exit code 0。

- [ ] **Step 3: 按现有 Dockerfile 构建带 SHA 标签的镜像**

Run: `docker build -t mochat-go-deploy:<SHORT_SHA> .`

Expected: exit code 0；记录 `docker image inspect` 返回的镜像 ID。

- [ ] **Step 4: 验证工作树和提交可追溯性**

Run: `git status --short && git log -3 --oneline`

Expected: 工作树干净；设计提交和代码修复提交均存在。

### Task 4: 可回滚服务器部署与浏览器复验

**Files:**
- Server inspect: `/opt/mochat-go/deploy/standalone/docker-compose.yml`
- Server inspect: `/opt/mochat-go/deploy/standalone/.env.local`
- Create evidence under an existing deployment evidence directory; do not store credentials.

**Interfaces:**
- Consumes: `mochat-go-deploy:<SHORT_SHA>` 镜像归档和当前 Compose 部署。
- Produces: 新镜像摘要、部署前回滚标签、健康证据、5 页及 53 页浏览器复验结果。

- [ ] **Step 1: 建立部署前回滚点**

在服务器记录 `standalone-app-1` 当前镜像 ID、容器状态、`/healthz`、`/readyz`、Compose 文件校验和及功能开关；给当前镜像增加 `mochat-go-rollback:pre-ai-insight-readonly-20260826` 标签，并复制 Compose 与环境文件到带时间戳且权限受限的备份目录。

Expected: 标签指向部署前镜像；备份校验和与原文件一致。

- [ ] **Step 2: 上传并加载确定产物**

将本地镜像保存为压缩归档后上传到服务器临时目录，校验 SHA-256，再执行 `docker load` 并核对镜像 ID。

Expected: 本地与服务器镜像 ID 一致；归档校验和一致。

- [ ] **Step 3: 只替换 app 容器并检查重启状态**

使用现有 Compose 文件把 `standalone-app:latest` 指向新镜像，只执行 app 服务的重新创建；不操作 MySQL、Redis 和命名卷。

Expected: app 容器使用新镜像，健康检查通过；重启后仍为 healthy。

- [ ] **Step 4: 验证服务端接口与日志**

检查 `/healthz`、`/readyz`、Nginx 代理、容器日志；从已登录浏览器触发 5 个 AI 洞察页面请求，确认 records/status/filter-options 不再返回 501。

Expected: 健康端点成功；AI 洞察请求返回统一 `code/msg/data` 信封；日志无 panic、SQL 错误和跨租户异常。

- [ ] **Step 5: 浏览器复验**

逐页点击 5 个 AI 洞察页面，检查真实空态/Provider 状态、刷新、控制台和布局；再回归全部 53 个 Dashboard 页面。

Expected: 5 页无 Zod 原始错误；53 页无新增页面崩溃、资源损坏、控制台错误和横向溢出。

- [ ] **Step 6: 失败时回滚**

若任一关键验收失败，将 `standalone-app:latest` 恢复为 `mochat-go-rollback:pre-ai-insight-readonly-20260826` 并只重建 app 服务，再验证健康端点与关键页面。

Expected: 容器镜像恢复到部署前 ID，服务恢复部署前状态。
