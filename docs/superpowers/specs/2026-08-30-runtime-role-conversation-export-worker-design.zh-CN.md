# Runtime Role 会话导出 Worker 隔离修复设计

## 背景与根因

`MOCHAT_GO_RUNTIME_ROLE` 通过 `Config.applyRuntimeRole()` 把进程职责收敛为 API、worker 或 scheduler。现有实现会在非 worker 角色关闭 `EnableWeWorkCallbackWorker`、`EnableEmployeeApplyWorker` 等 worker 开关，但遗漏了同属 worker 的 `EnableConversationExportWorker`。

`cmd/mochat-go/main.go` 随后只依据 `cfg.EnableConversationExportWorker` 注册 `conversation-export-worker`。因此，当环境同时设置 `MOCHAT_GO_RUNTIME_ROLE=api`（或 `scheduler`）与 `MOCHAT_GO_ENABLE_CONVERSATION_EXPORT_WORKER=true` 时，本应不消费后台任务的进程仍会运行导出 worker。根因位于配置责任矩阵不完整，不在 task runner 或导出实现本身。

## 目标与非目标

目标：

- `all` 与 `worker` 角色保留显式启用的会话导出 worker。
- `api` 与 `scheduler` 角色强制关闭会话导出 worker。
- 通过配置层测试锁定全部四种角色，防止后续再次遗漏。

非目标：

- 不改变会话导出任务的处理逻辑、间隔、持久化或目录。
- 不重构全部 runtime flag 为新注册表。
- 不改变 durable archive 的独立 worker/scheduler 责任矩阵。

## 方案比较

### 方案 A：在 `applyRuntimeRole()` 的 worker 分支补齐开关（采用）

把 `EnableConversationExportWorker = false` 放入现有 `!RunsWorkers()` 分支，并扩展已有角色矩阵测试。该方案在配置进入主程序前统一裁剪职责，与其他 worker 的现有模式一致，改动最小且 fail closed。

### 方案 B：只在 `main.go` 注册处追加 `RunsWorkers()` 判断

可以阻止实际启动，但配置对象仍错误地声称 worker 已启用，`backgroundTasksEnabled`、校验逻辑和未来其他调用者仍可能产生漂移，因此不采用。

### 方案 C：建立所有后台职责的声明式注册表

长期可减少遗漏，但会同时改变大量现有配置字段和测试，超出本缺陷最小修复范围，也增加当前发布验收风险，因此不采用。

## 设计

数据流保持为：环境变量 → `FromEnv()` → `applyRuntimeRole()` → 配置校验 → `main()` 注册任务。唯一行为变化发生在 `applyRuntimeRole()`：若角色不运行 worker，则无条件把 `EnableConversationExportWorker` 置为 `false`。

测试在 `internal/config/config_test.go` 的 runtime role 责任矩阵中同时设置普通 worker、scheduler 和会话导出 worker 开关，并对 `all/api/worker/scheduler` 四种角色断言：

- `all`：普通 worker、会话导出 worker、scheduler 均为 `true`；
- `api`：三类后台职责均为 `false`；
- `worker`：普通 worker与会话导出 worker为 `true`，scheduler 为 `false`；
- `scheduler`：两类 worker 为 `false`，scheduler 为 `true`。

## 错误处理与兼容性

该修复不新增错误分支。非 worker 角色即使收到会话导出 worker 环境开关，也按其他 worker 的既有规则静默裁剪，保持 runtime role 对具体开关的最高约束。`all` 默认角色与 `worker` 角色行为不变。

## 验证

1. 先只修改测试，执行目标测试并确认在 `api` 或 `scheduler` 用例出现期望的 RED。
2. 补齐 `applyRuntimeRole()` 单行责任矩阵后重跑目标测试确认 GREEN。
3. 执行 `go test ./internal/config -count=1`、`go test ./... -count=1`、`go vet ./...` 和后续最终验收全量门禁。

## 自审结论

- 无 `TODO`、`TBD` 或未决行为。
- 目标、责任矩阵与测试断言一致。
- 修复严格限制在遗漏的 worker flag 与对应测试，不包含无关重构。
- 用户已在最终验收指令中明确授权发现缺陷后按中文设计、TDD、完整验证、普通合并与普通 push 闭环，因此本设计无需额外等待逐步确认。
