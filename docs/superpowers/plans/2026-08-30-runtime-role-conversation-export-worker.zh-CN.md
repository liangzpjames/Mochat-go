# Runtime Role 会话导出 Worker 隔离修复 Implementation Plan

> **面向执行代理：** 必须使用 `test-driven-development` 逐步执行，并在提交前使用 `verification-before-completion`。步骤使用勾选框追踪。

**目标：** 让会话导出 worker 严格服从 `MOCHAT_GO_RUNTIME_ROLE` 的 worker 责任边界。

**架构：** 在配置归一化层补齐一个遗漏的 worker flag；主程序继续只消费已经归一化的配置，不增加第二套角色判断。测试扩展现有四角色矩阵，直接证明配置裁剪行为。

**技术栈：** Go 1.26、标准库 `testing`、现有 `internal/config` 环境测试工具。

## 全局约束

- 不修改会话导出业务逻辑、调度周期或持久化。
- 不改变 durable archive 的 worker/scheduler 设计。
- 必须先确认新增断言在当前生产代码上失败，再修改生产代码。

---

### Task 1：用现有角色矩阵复现遗漏

**Files:**

- Modify: `internal/config/config_test.go`

**Interfaces:**

- Consumes: `FromEnv()` 与 `Config.EnableConversationExportWorker`
- Produces: 四种 runtime role 的会话导出 worker 责任合同

- [ ] **Step 1：重新读取目标测试最新内容**

Run: `Get-Content internal/config/config_test.go`（UTF-8）

- [ ] **Step 2：在现有测试设置导出 worker 并断言 worker 责任**

在 `TestFromEnvRuntimeRoleFiltersBackgroundResponsibilities` 中加入：

```go
t.Setenv("MOCHAT_GO_ENABLE_CONVERSATION_EXPORT_WORKER", "1")
```

并在普通 worker 断言后加入：

```go
if cfg.EnableConversationExportWorker != tt.wantWorker {
	t.Fatalf("EnableConversationExportWorker = %v, want %v", cfg.EnableConversationExportWorker, tt.wantWorker)
}
```

- [ ] **Step 3：执行 RED**

Run: `go test ./internal/config -run TestFromEnvRuntimeRoleFiltersBackgroundResponsibilities -count=1`

Expected: `api` 或 `scheduler` 子测试 FAIL，实际值 `true`、期望值 `false`。

### Task 2：在配置责任矩阵补齐 worker flag

**Files:**

- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**

- Consumes: `Role.RunsWorkers()`
- Produces: 非 worker 角色下 `EnableConversationExportWorker == false`

- [ ] **Step 1：重新读取 `applyRuntimeRole()` 最新内容**

Run: `Get-Content internal/config/config.go`（UTF-8）

- [ ] **Step 2：写最小实现**

在 `if !cfg.RuntimeRole.RunsWorkers()` 中加入：

```go
cfg.EnableConversationExportWorker = false
```

- [ ] **Step 3：执行 GREEN**

Run: `go test ./internal/config -run TestFromEnvRuntimeRoleFiltersBackgroundResponsibilities -count=1`

Expected: PASS。

- [ ] **Step 4：执行包级回归**

Run: `go test ./internal/config -count=1`

Expected: PASS。

- [ ] **Step 5：检查差异并提交**

Run: `gofmt -w internal/config/config.go internal/config/config_test.go`、`git diff --check`、`git diff --stat`

Commit: `fix(runtime): isolate conversation export worker by role`
