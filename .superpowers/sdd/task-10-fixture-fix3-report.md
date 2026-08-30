# Task 10 第三轮 fixture 合同修复报告

## 结论

本轮关闭最终复审提出的 fixture Important 项，不实施 Task 11，不修改业务运行语义：

- callback inbox 的当前态测试只应用最新生产 registry；0174 生命周期从生产 0173 前缀加 seed 开始，再通过生产 registry 的完整 0174 前缀 runner 执行和回滚。
- WeCom 0139 删除单元素 runner，改为 `DefaultMigrations` 到 0139 的完整前缀；原始 0139 probe 只通过精确 test-only 包装器执行，不形成宽泛生产 API。
- archive/contact 等迁移 fixture 统一使用生产 registry 前缀 helper，不再手拼 `[]Migration`。
- AI/0165 的每个破坏场景都从随机隔离库和生产 0164 前缀开始；只保留精确场景 mutation，不直接写标准 ledger。already-applied 场景由真实 controlled 0165 controller 产生。
- Dashboard/identity 的故障注入从父测试函数拆到精确 helper，allowlist 同时约束函数、对象和操作。
- 新的 AST audit 自动覆盖 store/migration 集成入口及调用闭包，并识别 `NewRunner`、migration slice、ledger SQL、`integrationtestdb.NewIsolated`、`sql.Open`、`testharness.ApplyThrough/ApplyLatest` 等结构化 sink/root。

## 防绕过合同

mutation 测试证明门禁能拒绝：

- callback/WeCom 的局部或单元素 runner；
- 已允许 prefix helper 被篡改为单元素 registry；
- 父函数新增与 helper 同名对象的 DDL；
- ledger SQL 经局部变量、文件常量、字符串拼接或 helper 返回值绕过；
- DSN 经常量和间接 helper 传递后调用结构化数据库 sink；
- 仅凭同 basename 排除嵌套测试文件。

允许的 runner helper 必须在 AST 上满足：读取 `DefaultMigrations`，在同一 registry 上定位目标 index，并把同一 registry 的 `[:index+1]` 传给 `NewRunner`。任何 migration slice composite 均拒绝。标准 ledger 的 `DELETE` 只允许 0130 recovery 的精确 version。

## 验证证据

### 无 DSN

- AST mutation、store/migration fixture 合同、0165 静态合同：退出码 0。
- 0165 真实集成测试因 `MOCHAT_GO_0165_MYSQL_INTEGRATION_DSN` 未设置而明确 SKIP；不将 SKIP 记为数据库 PASS。
- 带 `integration` tag 的 AI daily 测试因 `MOCHAT_GO_AI_INSIGHT_MYSQL_INTEGRATION_DSN` 未设置而明确 SKIP；退出码 0。
- `go test -tags integration ./internal/migration -run '^$' -count=1`：退出码 0，证明 integration build 可编译。

### MariaDB 10.6

- `go test ./internal/migration -run '^TestAIInsight0165' -count=1 -timeout 20m`：退出码 0，57.731s。
- `go test -tags integration ./internal/migration -run '^TestAIDailyInsightUnification0165RetriesEveryPartialStageOnRealMariaDB$' -count=1 -timeout 20m`：退出码 0，60.080s。
- callback/WeCom/archive/contact/Dashboard/identity 双库定向中的 MariaDB 运行：store 16.677s、migration 144.426s，退出码 0。

### MySQL 5.7

- `go test ./internal/migration -run '^TestAIInsight0165' -count=1 -timeout 20m`：退出码 0，94.617s；该组通过既有 MySQL57 compatibility plan 执行受控 0165。
- `TestAIDailyInsightUnification0165RetriesEveryPartialStageOnRealMariaDB` 是原始 MariaDB SQL 的专用合同。将其诊断性地指向 MySQL 5.7 时退出码 1（65.123s），根因是 MySQL 5.7 不支持原始 0165 的 `ADD COLUMN IF NOT EXISTS`；此项非 MySQL 5.7 适用验收，不修改生产 SQL 伪兼容，也不记为 PASS。
- callback/WeCom/archive/contact/Dashboard/identity 双库定向中的 MySQL 5.7 运行：store 39.260s、migration 272.833s，退出码 0。

### 清理与差异

- MariaDB `information_schema.schemata` 中 `mochat_it_*` 数量：0。
- MySQL 5.7 `information_schema.schemata` 中 `mochat_it_*` 数量：0。
- `git diff --check`：退出码 0。
- 两数据库 full 两包、vet、qualitygate 将在本轮原子提交与并行 SCRM 原子提交整合后统一重跑，避免把整合前结果冒充最终证据。

## 边界

- 未修改 `docs/PROJECT_PROGRESS.zh-CN.md`。
- 未 reset、clean、force push 或删除 Docker 命名卷。
- 未实施 Task 11。
- 本提交不包含并行 SCRM fixer 的三个 parity 测试、0109 down migration 和 `migration_test.go`；这些文件由其独立原子提交负责。
