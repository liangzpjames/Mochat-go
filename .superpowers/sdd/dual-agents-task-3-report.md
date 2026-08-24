# Task 3 报告：洞察结果真实写库

## 根因与红灯证据

- `SaveInsight` 的 INSERT 列表有 27 列，其中 `created_at`、`updated_at` 由两个 `NOW()` 提供，因此业务参数必须对应 25 个 `?`。
- 修复前 SQL 有 26 个 `?`，而函数只传入 25 个业务实参。新增的精确 SQL 合同测试在修复前失败；错误中实际 SQL 显示 `VALUES` 有 26 个占位符，而测试期望 25 个。

## 改动

- `internal/modules/ai-insight/repository.go`：仅移除 `SaveInsight` VALUES 中多出的一个 `?`，保留两个 `NOW()` 和既有 `ON DUPLICATE KEY UPDATE` 语义。
- `internal/modules/ai-insight/repository_test.go`：增加 25 个业务参数与 25 个占位符的精确 SQL 合同测试。
- `internal/modules/ai-insight/repository_integration_test.go`：新增 `integration` 构建标签的 MariaDB 集成测试。
  - 仅读取 `MOCHAT_GO_AI_INSIGHT_MYSQL_INTEGRATION_DSN` 是否存在，不记录或输出其值。
  - 未设置时普通集成测试跳过；`MOCHAT_REQUIRE_MYSQL_INTEGRATION=1` 时缺失会失败。
  - 仅接受 `mochat_go_ai_insight_test` 或 `mochat_go_ai_insight_integration` 开头、且可选后缀只含小写字母/数字/下划线的专用测试 schema；并在单连接上创建/精确删除 `TEMPORARY TABLE`，不会操作业务表。
  - 验证首次成功写入的标识字段、结果 JSON、provider/model/prompt 版本与生成时间；验证唯一键重复写入更新失败状态及指定内容，并保留 `created_at`。

## 验证

- 红灯：`go test ./internal/modules/ai-insight -run '^TestSaveInsightUsesTwentyFiveBusinessPlaceholders$' -count=1` 在修复前失败，错误为实际 SQL 多一个占位符。
- 绿灯：`go test ./internal/modules/ai-insight -run 'SaveInsight' -count=1` 通过。
- 集成命令：`go test -v -tags=integration ./internal/modules/ai-insight -run 'SaveInsight' -count=1` 已执行且测试代码编译；因未配置专用 DSN，MariaDB 用例按设计跳过，未真实执行。
- `git diff --check` 通过。
- 审查后收紧了测试 schema 命名校验，并以红绿测试拒绝 `production-integration` 等非专用名称；临时表的 `created_at`、`updated_at` 已与迁移保持 `DATETIME` 一致。

## 集成测试交接

主代理在已设置专用、隔离测试 schema 的 `MOCHAT_GO_AI_INSIGHT_MYSQL_INTEGRATION_DSN` 环境运行：

```powershell
$env:MOCHAT_REQUIRE_MYSQL_INTEGRATION = '1'
go test -v -tags=integration ./internal/modules/ai-insight -run 'SaveInsight' -count=1
```
