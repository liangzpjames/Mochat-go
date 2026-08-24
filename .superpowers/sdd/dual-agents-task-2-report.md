# Task 2 后端双助手配置合同实施报告

## 状态

DONE

## 实施范围

- 在 ports 中新增 `SystemAssistantRepository`、`SystemAssistantContext` 和 `SessionAnalysisRule`；保留 `SessionAssistantRepository` 与 `SessionAssistantContext` 兼容合同。
- `GET /dashboard/ai-settings/agents` 幂等确保并固定返回会话分析助手、智能分析助手两项，过滤旧自定义助手和重复用途，服务端覆盖固定名称。
- `PUT /dashboard/ai-settings/agents/{id}` 先按 tenant/corp/id 读取用途；MySQL 更新事务再次使用 `FOR UPDATE` 锁定并读取数据库真实 `system_key`，不信任请求用途。
- 会话助手只接收 `sessionAnalysisRule`，持久化到 `session-analysis`；智能助手只接收 `smartAnalysisRule`，持久化规则 key 继续使用 `default-smart-analysis`。
- 会话双提示词 trim 后按 Unicode 字符校验 2–4000；规则范围、回看天数、最少消息数沿用明确边界；智能 objective 保持 2–500。
- 助手、当前规则、必要的新版本和设置审计在同一事务完成。规则没有实质变化时不生成版本；`conversationTypes` 仅顺序变化视为无实质变化。
- POST/DELETE 固定返回 405，包括 POST 请求体不是合法 JSON 的情况；跨 tenant/corp/id 返回 404；客户端改名返回机器可读错误且不会生效。

## TDD 红灯证据

### HTTP 合同红灯

命令：

```text
go test ./internal/modules/ai-settings/transport/http -run 'TestAgentHandler(EnsuresExactlyTwoSystemAssistants|RoutesUpdateByPersistedSystemKey|RejectsRuleForWrongAssistantPurpose|ValidatesBothSessionPromptsByUnicodeLength|SystemAssistantsAreTenantScopedAndImmutable|RejectsSystemAssistantRename)$' -count=1
```

预期失败摘要：

```text
unknown field SessionAnalysisRule in struct literal of type ports.Agent
undefined: ports.SessionAnalysisRule
undefined: machineCodeSessionRuleInvalid
FAIL jiyi/mochat-go/internal/modules/ai-settings/transport/http [build failed]
```

### MySQL 双助手事务红灯

命令：

```text
go test ./internal/modules/ai-settings/adapters/mysql -run 'TestAgentRepository(EnsuresAndReturnsExactlyTwoSystemAssistants|RoutesSessionUpdateByLockedSystemKeyAndVersionsBothPromptsOnce|DoesNotVersionUnchangedSmartRuleOrTouchSessionRule|RollsBackSystemAssistantUpdateFailures|ReturnsNotFoundWhenSystemAssistantIDIsOutsideTenant)$' -count=1
```

预期失败摘要：

```text
repo.EnsureSystemAssistants undefined
repo.UpdateSystemAssistant undefined
FAIL jiyi/mochat-go/internal/modules/ai-settings/adapters/mysql [build failed]
```

### 无实质变化红灯

将智能规则当前会话类型设为 `["direct","group"]`、请求设为 `["group","direct"]` 后运行：

```text
go test ./internal/modules/ai-settings/adapters/mysql -run TestAgentRepositoryDoesNotVersionUnchangedSmartRuleOrTouchSessionRule -count=1
```

预期失败摘要：

```text
unexpected UPDATE mochat_go_ai_analysis_rules
```

证明原实现把集合顺序变化误判为规则变化。实现规范化排序后不再生成版本。

### 无变化助手更新红灯

让 sqlmock 返回助手 UPDATE `RowsAffected=0` 后运行同一测试，预期失败为：

```text
AI settings record not found
```

证明已锁定存在的助手在完全无变化时会被误判 404。实现改为以事务内锁定结果判断存在性，允许 no-op UPDATE 成功且不增加规则版本。

### 405 红灯

将系统助手 POST 请求体改为非法 JSON 后运行：

```text
go test ./internal/modules/ai-settings/transport/http -run TestAgentHandlerSystemAssistantsAreTenantScopedAndImmutable -count=1
```

预期失败摘要：

```text
create=400 delete=405
```

实现调整为系统助手 POST 不解析请求体，稳定返回 405。

## 绿灯与最终验证

```text
go test ./internal/modules/ai-settings/... -count=1
ok jiyi/mochat-go/internal/modules/ai-settings
ok jiyi/mochat-go/internal/modules/ai-settings/adapters/mysql
ok jiyi/mochat-go/internal/modules/ai-settings/transport/http
```

兼容验证：

```text
go test ./internal/modules/ai-insight/... -count=1
ok jiyi/mochat-go/internal/modules/ai-insight
ok jiyi/mochat-go/internal/modules/ai-insight/transport/http
```

格式验证：

```text
git diff --check
```

结果：退出码 0，无 whitespace error。

## 测试覆盖重点

- GET 恰好两项、固定 system key、固定名称和用途专属规则字段。
- handler 根据数据库读取的助手用途分流，不根据客户端字段猜测用途。
- 两助手说明、状态、知识库、规则互不串改。
- 会话双提示词的 trim、Unicode 最小/最大长度和规则范围边界。
- 会话双提示词变化只插入一个不可变新版本。
- 智能规则完全无变化或 conversationTypes 仅换序时不插入版本。
- 知识库锁定校验、版本插入、审计失败时事务回滚。
- tenant/corp/id 越界 404，POST/DELETE 405，改名被拒绝。
- 旧 ai-insight 会话 Ensure/Load 接口继续可用。

## 自审

- 仅修改 brief 允许的 ports、MySQL runtime repository、handler 及其测试，并新增 brief 指定报告；未修改 Runner、前端、迁移、其他模块或进度文件。
- 更新事务在任何写操作前锁定 scoped ID 并读取真实 system key；客户端传入的 `Agent.SystemKey` 在 repository 中被覆盖。
- 固定名称在 handler 公共响应和 repository 读写两层覆盖。
- SQL 错误统一映射为 `AI_SETTINGS_STORAGE_FAILURE`，不向 HTTP 响应泄露 SQL。
- 规则版本仅在规则实质变化时递增一次；当前规则更新和版本插入共享同一 `nextVersion`。
- 未发现需阻塞交付的问题。

## 关注点

- 兼容 wrapper 仍以 `session-analysis` 为默认用途；ai-insight 尚未改造为显式选择 `smart-analysis`，本任务按 brief 未修改 Runner。
- GET 确保事务提交后再读取两项；若提交成功后读取发生瞬时数据库错误，请求会返回存储失败，但已确保的数据仍保留。这与既有 Ensure 行为一致。
