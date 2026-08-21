# AI 洞察会话分析与智能分析优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `/ai-insight/session-analysis` 与 `/ai-insight/smart-analysis` 从企业级总摘要空壳改造成基于真实归档会话、结构化 AI 结果、智能规则、权限范围和可追溯证据的完整工作台。

**Architecture:** 保留旧 `mochat_go_ai_analysis` 与另外三个旧 AI 洞察页，新增会话级结果、智能规则版本和任务批次表。后台每日任务跨消息分表按时间归并并以真实会话为单元调用 Provider，页面只读落库结果；前端使用独立 API、共享紧凑工作台、两套领域页和 URL 状态。

**Tech Stack:** Go 1.22、MariaDB 10.6/MySQL 兼容 SQL、React 18、TypeScript、TanStack Query、React Router、Vitest、Testing Library、Docker Compose、in-app Browser。

> 实施状态（2026-08-21）：已完成代码、迁移、Docker app-only 重建、自动化回归与浏览器点击验收。任务 1–10 已分别落地于 `f6b6550`、`680e2de`、`ab0a89e`、`f67803c`；存量权限资源回填补充迁移 `0151_ai_conversation_insight_api_resources`。最终事实以 `docs/reviews/2026-08-21-ai-insight-session-smart-acceptance.zh-CN.md` 为准。

## Global Constraints

- 本轮只修改 `/ai-insight/session-analysis` 与 `/ai-insight/smart-analysis`，不优化情绪识别、员工评分和沟通关键词。
- 保留侧边栏、全局顶部栏、菜单名称和路由；页面不增加“AI 洞察”眉标题和纯介绍大卡。
- 页面打开、查询和刷新不得调用模型；只有后台任务写分析结果。
- 所有结果必须对应真实归档会话、真实来源窗口和真实落库结果，不使用前端静态业务数据。
- 所有读写同时限制 `tenant_id`、`corp_id`；列表、详情和导出额外限制当前账号员工数据范围。
- 模型输出必须按固定 JSON Schema 校验；证据消息必须属于本次来源窗口。
- 固定每页 20 条；筛选、页码、标签和详情 ID 写入 URL。
- 智能规则版本不可变；历史结果不随规则编辑或对象改名漂移。
- 页面只显示紧凑任务故障状态，不使用大面积“能力未接入”卡片。
- 行为修改测试先行；每个任务保留 RED 与 GREEN 证据并单独提交。
- Docker 只重建必要的 `app` 服务，不删除 MySQL、Redis 或命名卷，继续使用 D 盘现有持久化路径。
- 实施完成后必须在 `2560×1440` 和常规桌面宽度自行点击两个页面。

---

### Task 1: 建立会话级 AI 洞察数据库模型

**Files:**

- Create: `deploy/standalone/migrations/0148_ai_conversation_insights.up.sql`
- Create: `deploy/standalone/migrations/0148_ai_conversation_insights.down.sql`
- Modify: `internal/migration/migration_test.go`
- Modify: `internal/migration/provider_schema_reconcile_test.go`
- Modify: `internal/dashboard/saas_admin_system_health.go`
- Modify: `internal/dashboard/saas_admin_system_health_test.go`

**Interfaces:**

- Produces: `mochat_go_ai_analysis_rules`、`mochat_go_ai_analysis_rule_versions`、`mochat_go_ai_conversation_insights`、`mochat_go_ai_insight_runs`。
- Produces: 迁移健康版本 `148`。

- [x] **Step 1: 写迁移 RED 测试**

在迁移测试中断言：

```go
func TestAIConversationInsightMigrationDiscovered(t *testing.T) {
    path := filepath.Join("..", "..", "deploy", "standalone", "migrations", "0148_ai_conversation_insights.up.sql")
    raw, err := os.ReadFile(path)
    if err != nil {
        t.Fatal(err)
    }
    up := string(raw)
    for _, table := range []string{
        "mochat_go_ai_analysis_rules",
        "mochat_go_ai_analysis_rule_versions",
        "mochat_go_ai_conversation_insights",
        "mochat_go_ai_insight_runs",
    } {
        if !strings.Contains(up, "CREATE TABLE IF NOT EXISTS `"+table+"`") {
            t.Fatalf("0148 migration does not create %s", table)
        }
    }
    for _, fragment := range []string{
        "uq_ai_rule_version",
        "uq_ai_conversation_source",
        "idx_ai_insight_scope_page",
        "idx_ai_insight_employee_page",
        "idx_ai_run_scope_type",
    } {
        if !strings.Contains(up, fragment) {
            t.Fatalf("0148 migration missing %s", fragment)
        }
    }
}
```

把 SaaS 健康测试期望迁移版本改为 `148`。

- [x] **Step 2: 运行 RED**

Run:

```powershell
go test ./internal/migration ./internal/dashboard -run 'Test.*(0148|ExpectedMigration|AIConversation)' -count=1
```

Expected: FAIL，原因是 0148 文件不存在或健康版本仍低于 148。

- [x] **Step 3: 编写完整 up/down SQL**

`mochat_go_ai_analysis_rules` 必须包含租户/企业、名称、目标、会话范围 JSON、对象范围、对象 ID JSON、回看天数、最少消息数、状态、当前版本、审计时间和软删除字段。

`mochat_go_ai_analysis_rule_versions` 必须保存不可变规则目标与范围快照，并定义：

```sql
UNIQUE KEY `uq_ai_rule_version` (`tenant_id`, `corp_id`, `rule_id`, `version`)
```

`mochat_go_ai_conversation_insights` 至少包含：

```sql
`tenant_id` bigint unsigned NOT NULL,
`corp_id` bigint unsigned NOT NULL,
`analysis_type` varchar(16) NOT NULL,
`rule_id` bigint unsigned NOT NULL DEFAULT 0,
`rule_version_id` bigint unsigned NOT NULL DEFAULT 0,
`conversation_key` varchar(191) NOT NULL,
`employee_id` bigint unsigned NOT NULL,
`employee_name` varchar(128) NOT NULL,
`employee_avatar` varchar(512) NOT NULL DEFAULT '',
`target_type` varchar(16) NOT NULL,
`target_id` bigint unsigned NOT NULL,
`target_name` varchar(128) NOT NULL,
`target_avatar` varchar(512) NOT NULL DEFAULT '',
`source_started_at` datetime(6) NOT NULL,
`source_ended_at` datetime(6) NOT NULL,
`source_message_count` int unsigned NOT NULL,
`source_fingerprint` char(64) NOT NULL,
`status` varchar(16) NOT NULL,
`summary` text NULL,
`result_json` json NULL,
`error_summary` varchar(1024) NOT NULL DEFAULT '',
`provider` varchar(64) NOT NULL DEFAULT '',
`model` varchar(128) NOT NULL DEFAULT '',
`prompt_version` varchar(32) NOT NULL,
`generated_at` datetime(6) NULL
```

定义唯一键：

```sql
UNIQUE KEY `uq_ai_conversation_source`
(`tenant_id`, `corp_id`, `analysis_type`, `rule_version_id`, `conversation_key`, `source_fingerprint`)
```

`mochat_go_ai_insight_runs` 保存批次状态、候选/成功/失败/积压数量、时间和错误摘要。down.sql 只删除 0148 新建的四张表，按依赖逆序删除。

- [x] **Step 4: 运行迁移测试与 MariaDB 方言验证**

Run:

```powershell
go test ./internal/migration ./internal/dashboard -run 'Test.*(0148|ExpectedMigration|AIConversation)' -count=1
```

Expected: PASS。

若 `MOCHAT_GO_MYSQL_INTEGRATION_DSN` 未配置，MariaDB 集成测试必须明确输出 SKIP，不能写成已通过。

- [x] **Step 5: 提交**

```powershell
git add deploy/standalone/migrations/0148_ai_conversation_insights.up.sql deploy/standalone/migrations/0148_ai_conversation_insights.down.sql internal/migration/migration_test.go internal/migration/provider_schema_reconcile_test.go internal/dashboard/saas_admin_system_health.go internal/dashboard/saas_admin_system_health_test.go
git diff --cached --check
git commit -m "feat: add conversation insight schema"
```

---

### Task 2: 定义领域合同与严格结果解析

**Files:**

- Create: `internal/modules/ai-insight/contracts.go`
- Create: `internal/modules/ai-insight/result_parser.go`
- Create: `internal/modules/ai-insight/result_parser_test.go`

**Interfaces:**

- Produces: `AnalysisType`、`ConversationCandidate`、`SessionAnalysisResult`、`SmartAnalysisResult`、`ConversationInsight`、`AnalysisRule`、`AnalysisRuleVersion`、`InsightRun`。
- Produces: `ParseSessionAnalysisResult(raw string, allowedMessageIDs map[string]struct{})`。
- Produces: `ParseSmartAnalysisResult(raw string, allowedMessageIDs map[string]struct{})`。

- [x] **Step 1: 写解析 RED 测试**

覆盖：合法结果、代码围栏剥离、缺少 `schemaVersion`、未知枚举、分数越界、置信度越界、证据消息越界和空摘要。

```go
func TestParseSessionAnalysisRejectsEvidenceOutsideSourceWindow(t *testing.T) {
    raw := `{"schemaVersion":1,"summary":"摘要","customer":{"qualityLevel":"medium","qualityReason":"理由","purchaseIntent":{"level":"medium","score":70,"reason":"理由","evidenceMessageIds":["msg:outside"]},"churnRisk":{"level":"low","score":20,"reason":"理由","evidenceMessageIds":[]},"keywords":[],"explicitNeeds":[],"implicitNeeds":[],"emotion":{"label":"neutral","reason":"理由","evidenceMessageIds":[]},"recommendedReply":"回复","actions":[],"notes":[]},"employeeQa":{"score":80,"dimensions":[],"strengths":[],"issues":[],"suggestions":[]}}`
    _, err := ParseSessionAnalysisResult(raw, map[string]struct{}{"msg:inside": {}})
    if err == nil || !strings.Contains(err.Error(), "msg:outside") {
        t.Fatalf("ParseSessionAnalysisResult error = %v", err)
    }
}
```

- [x] **Step 2: 运行 RED**

Run:

```powershell
go test ./internal/modules/ai-insight -run 'TestParse(Session|Smart)Analysis' -count=1
```

Expected: FAIL，解析函数尚不存在。

- [x] **Step 3: 实现精确类型和校验**

核心类型必须使用明确枚举和可空分数：

```go
type AnalysisType string

const (
    AnalysisTypeSession AnalysisType = "session"
    AnalysisTypeSmart   AnalysisType = "smart"
)

type EvidenceAssessment struct {
    Level              string   `json:"level"`
    Score              *int     `json:"score"`
    Reason             string   `json:"reason"`
    EvidenceMessageIDs []string `json:"evidenceMessageIds"`
}

type SmartAnalysisResult struct {
    SchemaVersion      int      `json:"schemaVersion"`
    Conclusion         string   `json:"conclusion"`
    Matched            bool     `json:"matched"`
    Confidence         *float64 `json:"confidence"`
    EvidenceMessageIDs []string `json:"evidenceMessageIds"`
    Recommendations    []string `json:"recommendations"`
}
```

解析器只允许剥离单层 ```json 代码围栏，不允许从自然语言中猜测 JSON。会话分数范围为 0～100 或 `null`；智能置信度范围为 0～1 或 `null`；所有证据 ID 必须出现在 `allowedMessageIDs`。

- [x] **Step 4: 运行 GREEN 和包回归**

Run:

```powershell
go test ./internal/modules/ai-insight -count=1
```

Expected: PASS。

- [x] **Step 5: 提交**

```powershell
git add internal/modules/ai-insight/contracts.go internal/modules/ai-insight/result_parser.go internal/modules/ai-insight/result_parser_test.go
git diff --cached --check
git commit -m "feat: define structured insight contracts"
```

---

### Task 3: 实现跨分表会话候选与结果仓储

**Files:**

- Create: `internal/modules/ai-insight/repository.go`
- Create: `internal/modules/ai-insight/repository_test.go`
- Create: `internal/modules/ai-insight/repository_integration_test.go`
- Modify: `internal/modules/ai-insight/transport/http/analysis_store.go`

**Interfaces:**

- Produces: `Repository` 接口及 `SQLRepository`。
- Consumes: Task 2 的领域类型。
- Produces: 后台候选查询、来源消息读取、幂等结果写入、列表/详情/状态、规则 CRUD 与版本写入。

- [x] **Step 1: 写 repository RED 测试**

最少创建以下精确测试，并分别断言：

- `TestConversationCandidatesMergeShardsByGlobalMessageTime`：两个分表插入时间交错的四条消息，返回顺序必须为全局时间降序；
- `TestSaveInsightIsIdempotentForSourceFingerprint`：同一幂等键保存两次后表内只有一行；
- `TestInsightPageAppliesTenantCorpAndEmployeeScope`：混入另一租户、企业和未授权员工结果后只返回当前范围一行；
- `TestRuleUpdateCreatesImmutableVersion`：规则由 v1 编辑为 v2 后两个版本均存在且 v1 内容不变；
- `TestRuleDeleteKeepsHistoricalInsights`：规则软删除后规则列表不再返回，历史结果仍能读取规则名称与版本快照。

集成测试建立隔离 schema，在两个消息分表插入时间交错数据，证明候选窗口按 `msg_data_time DESC, seq DESC, table_index DESC, id DESC` 全局排序，而不是按表号排序。

- [x] **Step 2: 运行 RED**

Run:

```powershell
go test ./internal/modules/ai-insight -run 'Test(ConversationCandidates|SaveInsight|InsightPage|Rule)' -count=1
```

Expected: FAIL，`Repository` 和 `SQLRepository` 尚不存在。

- [x] **Step 3: 实现仓储接口**

接口精确为：

```go
type Repository interface {
    ConversationCandidates(context.Context, CandidateQuery) ([]ConversationCandidate, error)
    ConversationMessages(context.Context, ConversationWindowQuery) ([]SourceMessage, error)
    LatestSucceededFingerprint(context.Context, int64, int64, AnalysisType, int64, string) (string, error)
    SaveInsight(context.Context, ConversationInsight) error
    CreateRun(context.Context, InsightRun) (int64, error)
    FinishRun(context.Context, int64, InsightRunResult) error
    InsightPage(context.Context, InsightFilter) (InsightPage, error)
    InsightDetail(context.Context, InsightDetailFilter) (ConversationInsight, error)
    LatestRun(context.Context, int64, int64, AnalysisType) (*InsightRun, error)
    RulePage(context.Context, RuleFilter) (RulePage, error)
    RuleByID(context.Context, int64, int64, int64) (AnalysisRule, error)
    CreateRule(context.Context, RuleWrite) (AnalysisRule, error)
    UpdateRule(context.Context, RuleWrite) (AnalysisRule, error)
    SetRuleStatus(context.Context, RuleStatusWrite) error
    DeleteRule(context.Context, RuleDelete) error
    EnabledRuleVersions(context.Context, int64, int64) ([]AnalysisRuleVersion, error)
}
```

`ConversationCandidates` 从 10 张有效消息表生成统一子查询，第一层限制 `corp_id` 和 `deleted_at IS NULL`，再按会话键分组。来源消息保留稳定归档消息 ID、时间、方向、发送者和纯文本。对象资料查询失败必须返回错误，不能构造假名称。

会话分析列表按 `conversation_key` 选取最新一次尝试；智能分析列表按 `rule_version_id + conversation_key` 选取最新一次尝试。失败尝试不能更新或删除之前的成功行，历史成功结果继续保留供审计。

把旧 `AnalysisStore` 留在原文件供旧三页兼容，但新增代码不得从 `transport/http` 反向依赖。

- [x] **Step 4: 运行单元与 MariaDB 集成测试**

Run:

```powershell
go test ./internal/modules/ai-insight -count=1
```

Expected: PASS；无 DSN 时只有明确的集成测试 SKIP。

- [x] **Step 5: 提交**

```powershell
git add internal/modules/ai-insight/repository.go internal/modules/ai-insight/repository_test.go internal/modules/ai-insight/repository_integration_test.go internal/modules/ai-insight/transport/http/analysis_store.go
git diff --cached --check
git commit -m "feat: add conversation insight repository"
```

---

### Task 4: 重构每日任务为会话级增量分析

**Files:**

- Modify: `internal/modules/ai-insight/daily.go`
- Create: `internal/modules/ai-insight/conversation_runner.go`
- Create: `internal/modules/ai-insight/conversation_runner_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/mochat-go/ai_debt_clearance.go`
- Modify: `deploy/standalone/docker-compose.yml`
- Modify: `deploy/standalone/.env.example`

**Interfaces:**

- Consumes: Task 2 解析器、Task 3 Repository。
- Produces: `ConversationAnalysisRunner.RunCorp(ctx, tenantID, corpID)`。
- Produces: 配置 `MOCHAT_GO_AI_INSIGHT_BATCH_LIMIT` 和 `MOCHAT_GO_AI_INSIGHT_CONCURRENCY`。

- [x] **Step 1: 写 runner RED 测试**

使用 fake Repository 和 fake Provider 覆盖：

- 同一来源指纹跳过；
- 新消息产生新结果；
- 会话分析与每个启用规则分别执行；
- 单会话解析失败只增加失败数；
- 失败不覆盖旧成功结果；
- Provider 不可用时写 failed run，不写 insight；
- 单企业并发不超过配置；
- 超过批量上限时写 backlog 数；
- 提示词包含发送方向、消息时间、稳定消息 ID 和固定 JSON Schema；
- 不把两个会话拼进同一个请求。

```go
func TestConversationRunnerSkipsUnchangedFingerprint(t *testing.T) {
    repo := &fakeRepository{latestFingerprint: "same", candidates: []ConversationCandidate{{ConversationKey: "1001:1:2001", SourceFingerprint: "same"}}}
    provider := &fakeAIProvider{}
    runner := NewConversationAnalysisRunner(repo, provider, RunnerConfig{BatchLimit: 50, Concurrency: 2, PromptVersion: "session-v1"})
    if err := runner.RunCorp(context.Background(), 1, 1); err != nil {
        t.Fatal(err)
    }
    if provider.calls != 0 {
        t.Fatalf("provider calls = %d, want 0", provider.calls)
    }
}
```

- [x] **Step 2: 运行 RED**

Run:

```powershell
go test ./internal/modules/ai-insight -run 'TestConversationRunner' -count=1
```

Expected: FAIL，runner 尚不存在。

- [x] **Step 3: 实现 runner 与固定提示词**

`RunnerConfig`：

```go
type RunnerConfig struct {
    BatchLimit    int
    Concurrency   int
    PromptVersion string
    SessionDays   int
    SessionLimit  int
}
```

默认值固定为批量 200 个候选、并发 2、会话回看 30 天、每个会话最多 200 条消息。智能规则使用自己的 `lookback_days` 和 `minimum_messages`。

旧 `DailyAnalysisRunner` 继续为另外三个旧页面生成企业级结果；从 `dailyAnalysisPages` 移除 `session-analysis`、`smart-analysis`，并在同一个每日调度中调用新 runner。避免两个任务重复为这两个页面写入旧摘要。

- [x] **Step 4: 运行包测试和配置测试**

Run:

```powershell
go test ./internal/modules/ai-insight ./internal/config ./cmd/mochat-go -count=1
```

Expected: PASS。

- [x] **Step 5: 提交**

```powershell
git add internal/modules/ai-insight/daily.go internal/modules/ai-insight/conversation_runner.go internal/modules/ai-insight/conversation_runner_test.go internal/config/config.go internal/config/config_test.go cmd/mochat-go/ai_debt_clearance.go deploy/standalone/docker-compose.yml deploy/standalone/.env.example
git diff --cached --check
git commit -m "feat: run incremental conversation analysis"
```

---

### Task 5: 暴露工作台 HTTP、导出与 RBAC

**Files:**

- Create: `internal/modules/ai-insight/transport/http/workspace_handler.go`
- Create: `internal/modules/ai-insight/transport/http/workspace_handler_test.go`
- Modify: `internal/modules/ai-insight/transport/http/routes.go`
- Modify: `internal/modules/ai-insight/module.go`
- Modify: `internal/app/bootstrap/ai_insight.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`
- Modify: `internal/dashboard/dashboard_access_guard_test.go`
- Modify: `deploy/standalone/migrations/0148_ai_conversation_insights.up.sql`

**Interfaces:**

- Consumes: Task 3 Repository。
- Produces: 设计文档第 10 节的全部 HTTP 接口。
- Produces: `read`、`detail`、`export`、`rule.manage` 权限资源。

- [x] **Step 1: 写 handler/route/RBAC RED 测试**

覆盖：

- 列表默认页、固定 20、非法页码/日期/枚举；
- tenant/corp 来自 principal，不接受查询参数覆盖；
- 受限员工只能读取自己的结果；
- 越权详情返回 404，避免泄露记录存在；
- 状态接口返回最近 run 和 Provider 状态，不返回密钥；
- 导出当前筛选、UTF-8 BOM、中文表头、公式前缀防护；
- 规则新增/编辑/启停/删除、10 条启用上限、跨企业对象拒绝；
- 无 manage/export 权限返回 403；
- 旧两个 GET 接口仍可访问。

```go
func TestWorkspaceDetailHidesOutOfScopeInsight(t *testing.T) {
    request := httptest.NewRequest(http.MethodGet, "/dashboard/ai-insight/session-analysis/detail?id=99", nil)
    response := httptest.NewRecorder()
    handler.ServeHTTP(response, request)
    if response.Code != http.StatusNotFound {
        t.Fatalf("status = %d, want 404", response.Code)
    }
}
```

- [x] **Step 2: 运行 RED**

Run:

```powershell
go test ./internal/modules/ai-insight/transport/http ./internal/modules/ai-insight ./internal/dashboard -run 'Test.*(Workspace|InsightRule|AIInsightAccess)' -count=1
```

Expected: FAIL，新接口和资源不存在。

- [x] **Step 3: 实现 handler 和路由**

路由必须逐项注册：

```go
registrar.Handle(http.MethodGet, "/dashboard/ai-insight/session-analysis/records", workspaceHandler)
registrar.Handle(http.MethodGet, "/dashboard/ai-insight/session-analysis/detail", workspaceHandler)
registrar.Handle(http.MethodGet, "/dashboard/ai-insight/session-analysis/status", workspaceHandler)
registrar.Handle(http.MethodGet, "/dashboard/ai-insight/session-analysis/export", workspaceHandler)
registrar.Handle(http.MethodGet, "/dashboard/ai-insight/smart-analysis/records", workspaceHandler)
registrar.Handle(http.MethodGet, "/dashboard/ai-insight/smart-analysis/detail", workspaceHandler)
registrar.Handle(http.MethodGet, "/dashboard/ai-insight/smart-analysis/status", workspaceHandler)
registrar.Handle(http.MethodGet, "/dashboard/ai-insight/smart-analysis/rules", workspaceHandler)
registrar.Handle(http.MethodPost, "/dashboard/ai-insight/smart-analysis/rules", workspaceHandler)
registrar.Handle(http.MethodPut, "/dashboard/ai-insight/smart-analysis/rules", workspaceHandler)
registrar.Handle(http.MethodDelete, "/dashboard/ai-insight/smart-analysis/rules", workspaceHandler)
registrar.Handle(http.MethodPost, "/dashboard/ai-insight/smart-analysis/rules/status", workspaceHandler)
```

CSV 中对象名称、摘要、等级、来源时间、分析时间和规则名称使用结果快照；不查询当前资料改写历史。

`Module.New` 必须在 `DB != nil` 时始终创建只读 Repository 和 workspace handler，不得因为 Provider 当前不可用而丢失历史结果读取能力。Provider 只影响后台 runner 和状态接口的 `ready/degraded`，不影响已落库结果的列表与详情。

0148 同时插入新资源种子并保持 catalog 精确一致。

- [x] **Step 4: 运行 GREEN 与路由回归**

Run:

```powershell
go test ./internal/modules/ai-insight/transport/http ./internal/modules/ai-insight ./internal/dashboard ./internal/server ./cmd/mochat-go -count=1
```

Expected: PASS。

- [x] **Step 5: 提交**

```powershell
git add internal/modules/ai-insight/transport/http/workspace_handler.go internal/modules/ai-insight/transport/http/workspace_handler_test.go internal/modules/ai-insight/transport/http/routes.go internal/modules/ai-insight/module.go internal/app/bootstrap/ai_insight.go internal/dashboard/dashboard_page_catalog.json internal/dashboard/dashboard_access_guard_test.go deploy/standalone/migrations/0148_ai_conversation_insights.up.sql
git diff --cached --check
git commit -m "feat: expose AI insight workspace APIs"
```

---

### Task 6: 建立严格前端 API 与 URL 状态

**Files:**

- Create: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.ts`
- Create: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.test.ts`
- Create: `web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.ts`
- Create: `web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.test.ts`

**Interfaces:**

- Produces: `createAiInsightWorkspaceApi(client)`。
- Produces: `SessionInsightRow`、`SessionInsightDetail`、`SmartInsightRow`、`AnalysisRule`、`InsightRunStatus`。
- Produces: `readSessionFilters`、`writeSessionFilters`、`readSmartState`、`writeSmartState`。

- [x] **Step 1: 写 API 解析与 URL RED 测试**

覆盖：

- 合法列表、详情、规则和状态；
- 缺字段、未知状态、非法日期、非法对象类型和错误 result JSON 拒绝；
- 请求参数编码；
- 空筛选不写 URL；
- 查询变化重置页码；
- 关闭详情删除 ID；
- 智能分析两个标签保留各自筛选。

```ts
it('拒绝没有真实会话键的结果行', async () => {
  const client = { request: vi.fn().mockResolvedValue({ page: 1, pageSize: 20, total: 1, items: [{ id: 1 }] }) };
  const api = createAiInsightWorkspaceApi(client);
  await expect(api.sessionRecords({ page: 1 })).rejects.toThrow('会话分析列表接口返回了无效数据');
});
```

- [x] **Step 2: 运行 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/ai-insight-workspace-api.test.ts src/features/ai-insight/ai-insight-url-state.test.ts
```

Expected: FAIL，文件和函数不存在。

- [x] **Step 3: 实现严格解析和 API 方法**

API 方法精确为：

```ts
type AiInsightWorkspaceApi = {
  sessionRecords(filters: SessionInsightFilters): Promise<SessionInsightPage>;
  sessionDetail(id: number): Promise<SessionInsightDetail>;
  sessionStatus(): Promise<InsightRunStatus>;
  sessionExportUrl(filters: SessionInsightFilters): string;
  smartRecords(filters: SmartInsightFilters): Promise<SmartInsightPage>;
  smartDetail(id: number): Promise<SmartInsightDetail>;
  smartStatus(): Promise<InsightRunStatus>;
  smartRules(filters: AnalysisRuleFilters): Promise<AnalysisRulePage>;
  createRule(input: AnalysisRuleInput): Promise<AnalysisRule>;
  updateRule(input: AnalysisRuleInput & { id: number }): Promise<AnalysisRule>;
  setRuleStatus(id: number, status: 'enabled' | 'disabled'): Promise<void>;
  deleteRule(id: number): Promise<void>;
};
```

所有业务枚举通过集合校验，不使用 `as` 强制通过不可信响应。

- [x] **Step 4: 运行 GREEN、lint 和 typecheck**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/ai-insight-workspace-api.test.ts src/features/ai-insight/ai-insight-url-state.test.ts
corepack pnpm --filter @mochat/dashboard typecheck
```

Expected: PASS。

- [x] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.ts web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.test.ts web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.ts web/apps/dashboard/src/features/ai-insight/ai-insight-url-state.test.ts
git diff --cached --check
git commit -m "feat: add typed AI insight workspace API"
```

---

### Task 7: 建立共享 AI 洞察工作台与视觉合同

**Files:**

- Create: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace.tsx`
- Create: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace.test.tsx`
- Create: `web/apps/dashboard/src/styles/ai-insight-workspace.css`
- Create: `web/apps/dashboard/src/styles/ai-insight-workspace-layout.test.ts`
- Modify: `web/apps/dashboard/src/main.tsx`

**Interfaces:**

- Produces: `AiInsightWorkspace`、`AiInsightPageHeader`、`AiInsightTabs`、`AiInsightQueryBar`、`AiInsightRunStrip`、`AiInsightResults`、`AiInsightDetailDrawer`。
- Consumes: 现有 `DashboardPagination`。

- [x] **Step 1: 写共享组件和 CSS RED 测试**

断言：

- 没有纯介绍大卡和通用三 KPI；
- 查询、重置、刷新只在查询栏出现一次；
- 状态正常时不显示状态条，降级/失败/积压时只显示一条；
- 详情支持关闭、遮罩、Escape 和焦点恢复；
- CSS 在宽屏使用完整内容区，查询字段最多五列，表格容器独立横向滚动；
- hover 不含 `transform`；
- `@media (max-width: 1100px)` 收敛查询列数。

- [x] **Step 2: 运行 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/ai-insight-workspace.test.tsx src/styles/ai-insight-workspace-layout.test.ts
```

Expected: FAIL，共享组件和样式文件不存在。

- [x] **Step 3: 实现共享组件和样式**

`AiInsightQueryBar` 使用标准表单提交，按钮文字固定为“查询 / 重置 / 刷新”；会话分析通过 `extraActions` 注入“导出”。状态条文案只使用运行元数据：最近失败、当前积压、Provider 暂不可用，不渲染 limitations 数组。

在 `main.tsx` 只新增：

```ts
import './styles/ai-insight-workspace.css';
```

- [x] **Step 4: 运行 GREEN、typecheck 和布局测试**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/ai-insight-workspace.test.tsx src/styles/ai-insight-workspace-layout.test.ts
corepack pnpm --filter @mochat/dashboard typecheck
```

Expected: PASS。

- [x] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/ai-insight/ai-insight-workspace.tsx web/apps/dashboard/src/features/ai-insight/ai-insight-workspace.test.tsx web/apps/dashboard/src/styles/ai-insight-workspace.css web/apps/dashboard/src/styles/ai-insight-workspace-layout.test.ts web/apps/dashboard/src/main.tsx
git diff --cached --check
git commit -m "feat: add AI insight workspace shell"
```

---

### Task 8: 实现会话分析筛选、列表、导出与详情

**Files:**

- Create: `web/apps/dashboard/src/features/ai-insight/session-analysis-page.tsx`
- Create: `web/apps/dashboard/src/features/ai-insight/session-analysis-page.test.tsx`
- Create: `web/apps/dashboard/src/features/ai-insight/session-analysis-detail.tsx`
- Create: `web/apps/dashboard/src/features/ai-insight/session-analysis-detail.test.tsx`

**Interfaces:**

- Consumes: Task 6 API/URL、Task 7 工作台。
- Produces: `SessionAnalysisPage({ api, navigate })`。

- [x] **Step 1: 写页面 RED 测试**

覆盖：

- 页面没有“AI 能力状态”三卡片和大块能力说明；
- 客户/群聊、员工、会话类型、成交意愿、流失风险、日期和关键词筛选；
- 输入不请求，查询/回车提交，重置清空，刷新沿用 applied filters；
- 固定中文列、20 条分页、URL 恢复；
- 头像真实值优先，fallback 为名称首字；
- 分数 `null` 显示“证据不足”，不显示 0；
- 长摘要截断；
- 导出 URL 包含当前筛选；
- 整行和“分析详情”打开详情，复选框不误开；
- 客户分析/员工质检切换；
- 证据消息、模型和来源窗口；
- 会话详情使用后端返回的 `conversationUrl`；
- 关闭、遮罩、Escape、焦点恢复；
- loading、empty、error、degraded 和 forbidden。

```tsx
it('证据不足时不把空分数展示成 0', async () => {
  renderSessionPage({ purchaseIntent: { level: 'insufficient', score: null, reason: '消息不足' } });
  expect(await screen.findByText('证据不足')).toBeTruthy();
  expect(screen.queryByText('0%')).toBeNull();
});
```

- [x] **Step 2: 运行 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/session-analysis-page.test.tsx src/features/ai-insight/session-analysis-detail.test.tsx
```

Expected: FAIL，页面组件不存在。

- [x] **Step 3: 实现页面和详情**

查询 key 只使用 `appliedFilters`。员工选择复用 `/workMessage/staffDirectory` 现有 API 数据，不增加员工 ID 手填框。详情证据列表只展示接口返回的短摘，不请求其他企业或其他会话消息。

导出通过受认证客户端下载 Blob；文件名格式：

```text
会话分析_YYYYMMDD_HHmmss.csv
```

- [x] **Step 4: 运行 GREEN、lint 和 typecheck**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/session-analysis-page.test.tsx src/features/ai-insight/session-analysis-detail.test.tsx
corepack pnpm --filter @mochat/dashboard typecheck
```

Expected: PASS。

- [x] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/ai-insight/session-analysis-page.tsx web/apps/dashboard/src/features/ai-insight/session-analysis-page.test.tsx web/apps/dashboard/src/features/ai-insight/session-analysis-detail.tsx web/apps/dashboard/src/features/ai-insight/session-analysis-detail.test.tsx
git diff --cached --check
git commit -m "feat: rebuild session analysis workspace"
```

---

### Task 9: 实现智能分析结果与规则工作台

**Files:**

- Create: `web/apps/dashboard/src/features/ai-insight/smart-analysis-page.tsx`
- Create: `web/apps/dashboard/src/features/ai-insight/smart-analysis-page.test.tsx`
- Create: `web/apps/dashboard/src/features/ai-insight/smart-analysis-results.tsx`
- Create: `web/apps/dashboard/src/features/ai-insight/smart-analysis-rules.tsx`
- Create: `web/apps/dashboard/src/features/ai-insight/smart-analysis-rule-drawer.tsx`
- Create: `web/apps/dashboard/src/features/ai-insight/smart-analysis-rule-drawer.test.tsx`

**Interfaces:**

- Consumes: Task 6 API/URL、Task 7 工作台。
- Produces: `SmartAnalysisPage({ api, navigate })`。

- [x] **Step 1: 写结果与规则 RED 测试**

结果覆盖：

- “分析结果 / 分析规则”标签和 URL 恢复；
- 客户/群聊、员工、会话类型、规则、状态、日期筛选；
- 固定列、分页、详情、证据和会话跳转；
- 规则版本显示，规则删除后历史结果仍显示快照名称；
- 空列表保持稳定高度。

规则覆盖：

- 名称、状态、会话范围筛选；
- 新增/编辑字段完整；
- 名称 1～80、目标 1～500、范围至少一项、回看 1～30、消息数 2～50；
- 对象范围为全部/部门/员工，使用真实选择器；
- 启用规则达到 10 条时阻止新增启用规则并展示服务端错误；
- 保存失败保留草稿；
- 保存生成的新版本显示；
- 启停与删除二次确认；
- 行内按钮不触发行详情；
- 遮罩和 Escape 关闭。

```tsx
it('规则保存失败时保留分析目标草稿', async () => {
  api.createRule.mockRejectedValue(new Error('启用规则最多 10 条'));
  renderSmartPage();
  await user.click(screen.getByRole('button', { name: '新增规则' }));
  await user.type(screen.getByLabelText('分析目标'), '识别价格异议并给出跟进建议');
  await user.click(screen.getByRole('button', { name: '保存规则' }));
  expect(await screen.findByText('启用规则最多 10 条')).toBeTruthy();
  expect(screen.getByLabelText('分析目标')).toHaveValue('识别价格异议并给出跟进建议');
});
```

- [x] **Step 2: 运行 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/smart-analysis-page.test.tsx src/features/ai-insight/smart-analysis-rule-drawer.test.tsx
```

Expected: FAIL，智能分析工作台组件不存在。

- [x] **Step 3: 实现结果、规则和抽屉**

规则输入合同：

```ts
type AnalysisRuleInput = {
  name: string;
  objective: string;
  conversationTypes: readonly ('direct' | 'group')[];
  targetScope: 'all' | 'department' | 'employee';
  targetIds: readonly number[];
  lookbackDays: number;
  minimumMessages: number;
  status: 'enabled' | 'disabled';
};
```

结果表中的 `matched=false` 显示“未命中”，不是失败；只有任务状态 `failed` 显示“分析失败”。

- [x] **Step 4: 运行 GREEN、lint 和 typecheck**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight/smart-analysis-page.test.tsx src/features/ai-insight/smart-analysis-rule-drawer.test.tsx
corepack pnpm --filter @mochat/dashboard typecheck
```

Expected: PASS。

- [x] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/features/ai-insight/smart-analysis-page.tsx web/apps/dashboard/src/features/ai-insight/smart-analysis-page.test.tsx web/apps/dashboard/src/features/ai-insight/smart-analysis-results.tsx web/apps/dashboard/src/features/ai-insight/smart-analysis-rules.tsx web/apps/dashboard/src/features/ai-insight/smart-analysis-rule-drawer.tsx web/apps/dashboard/src/features/ai-insight/smart-analysis-rule-drawer.test.tsx
git diff --cached --check
git commit -m "feat: rebuild smart analysis workspace"
```

---

### Task 10: 切换路由并保留旧三页兼容

**Files:**

- Modify: `web/apps/dashboard/src/main.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.tsx`
- Modify: `web/apps/dashboard/src/benchmark/page-registry.test.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-pages.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-pages.test.tsx`

**Interfaces:**

- Consumes: Task 6 `createAiInsightWorkspaceApi`、Task 8/9 页面。
- Produces: 两个新路由；旧通用页只处理 `emotion`、`employee-score`、`communication-keyword`。

- [x] **Step 1: 写 registry RED 测试**

断言：

```tsx
expect(pages['/ai-insight/session-analysis']).toEqual(expect.objectContaining({ type: SessionAnalysisPage }));
expect(pages['/ai-insight/smart-analysis']).toEqual(expect.objectContaining({ type: SmartAnalysisPage }));
expect(pages['/ai-insight/emotion']).toEqual(expect.objectContaining({ type: AiInsightPage }));
```

并断言两个新页面需要 `aiInsightWorkspaceApi`，不会回落到 `DemoPage` 或通用 `AiInsightPage`。

- [x] **Step 2: 运行 RED**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/benchmark/page-registry.test.tsx src/features/ai-insight/ai-insight-pages.test.tsx
```

Expected: FAIL，registry 仍把两个路由交给通用页面。

- [x] **Step 3: 切换依赖和清理通用配置**

在 `main.tsx` 创建：

```ts
const aiInsightWorkspaceApi = createAiInsightWorkspaceApi(apiClient);
```

`aiInsightPageConfigs` 只保留：

```ts
export const aiInsightPageConfigs = {
  emotion: { title: '情绪识别', description: '按会话识别客户情绪倾向与异常波动', page: 'emotion' },
  'employee-score': { title: '员工评分', description: '按员工评估响应时效与沟通质量', page: 'employee-score' },
  'communication-keyword': { title: '沟通关键词', description: '会话高频词、敏感词命中与趋势', page: 'communication-keyword' },
} as const;
```

不删除旧 API 和旧表，以免影响另外三个页面。

- [x] **Step 4: 运行前端目标回归**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/benchmark/page-registry.test.tsx src/features/ai-insight
corepack pnpm --filter @mochat/dashboard typecheck
```

Expected: PASS。

- [x] **Step 5: 提交**

```powershell
git add web/apps/dashboard/src/main.tsx web/apps/dashboard/src/benchmark/page-registry.tsx web/apps/dashboard/src/benchmark/page-registry.test.tsx web/apps/dashboard/src/features/ai-insight/ai-insight-pages.tsx web/apps/dashboard/src/features/ai-insight/ai-insight-pages.test.tsx
git diff --cached --check
git commit -m "feat: route AI insight workspaces"
```

---

### Task 11: 更新总进度与文档证据

**Files:**

- Modify: `docs/PROJECT_PROGRESS.zh-CN.md`
- Modify: `web/apps/dashboard/src/benchmark/manifest.json`
- Create: `docs/reviews/2026-08-21-ai-insight-session-smart-acceptance.zh-CN.md`

**Interfaces:**

- Produces: 两页实施状态、数据来源、验收命令和正式环境专项记录。

- [x] **Step 1: 写文档门禁 RED 检查**

运行搜索并记录当前 manifest 仍指向不存在的旧 spec：

```powershell
rg -n "pages-ai-insight-(session-analysis|smart-analysis)/spec.md" web/apps/dashboard/src/benchmark/manifest.json
```

Expected: 命中两个失效路径。

- [x] **Step 2: 更新 manifest 证据路径**

两个页面的 `evidence.spec` 都指向：

```text
docs/superpowers/specs/2026-08-21-ai-insight-session-smart-yuanhu-design.md
```

验收完成后 `evidence.acceptance` 指向本任务中文验收报告。

- [x] **Step 3: 更新总进度文档**

只记录真实完成项与正式环境专项：

- 会话级结果、结构化证据、智能规则、权限、分页、导出和浏览器验收；
- 正式企微归档 Provider、正式 AI Key、数据处理协议、保留期限和成本告警仍需生产专项；
- 情绪识别、员工评分、沟通关键词未纳入本轮。

页面不重复展示上述生产专项的大段文案。

- [x] **Step 4: 创建验收报告骨架并在最终验收后填写事实**

报告固定章节：环境与提交、自动化命令、数据库迁移、数据来源、浏览器点击矩阵、分辨率、控制台/网络、容器/卷、发现与修复、正式环境专项。未运行的命令明确写“未运行”，不得预填 PASS。

- [x] **Step 5: 提交文档更新**

```powershell
git add docs/PROJECT_PROGRESS.zh-CN.md web/apps/dashboard/src/benchmark/manifest.json docs/reviews/2026-08-21-ai-insight-session-smart-acceptance.zh-CN.md
git diff --cached --check
git commit -m "docs: record AI insight workspace delivery"
```

---

### Task 12: 全栈验证、Docker 交付和浏览器点击验收

**Files:**

- Modify only if acceptance finds a scoped defect: Tasks 1–11 已列文件。
- Modify: `docs/reviews/2026-08-21-ai-insight-session-smart-acceptance.zh-CN.md`

- [x] **Step 1: 检查工作树与任务差异**

Run:

```powershell
git status --short
git diff --check
git log --oneline --decorate -15
```

Expected: 本任务提交只包含两页及必要依赖；现有用户改动未被 reset、clean、覆盖或误提交。

- [x] **Step 2: 运行后端完整相关回归**

Run:

```powershell
go test ./internal/modules/ai-insight ./internal/modules/ai-insight/transport/http ./internal/app/bootstrap ./internal/dashboard ./internal/server ./internal/config ./internal/migration ./cmd/mochat-go -count=1
```

Expected: PASS；无隔离 DSN 时 MariaDB 集成测试明确 SKIP。

- [x] **Step 3: 运行前端目标与完整回归**

Run:

```powershell
corepack pnpm --filter @mochat/dashboard test -- src/features/ai-insight src/benchmark/page-registry.test.tsx src/styles/ai-insight-workspace-layout.test.ts
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard build
```

Expected: 全部 PASS。若完整回归存在与本任务无关的基线失败，必须先给出失败文件、测试名和复现命令，不能把目标测试通过写成全量通过。

- [x] **Step 4: 应用 0148 并准备真实分析数据**

使用现有迁移机制应用 0148。验证四张新表和最新迁移版本。

浏览器验收数据必须来自当前企业真实归档消息。若当前 Provider Key 不可用，先通过受保护环境文件配置测试环境 Key，再以：

```text
MOCHAT_GO_AI_INSIGHT_DAILY_ANALYSIS_ENABLED=1
MOCHAT_GO_AI_INSIGHT_ANALYSIS_RUN_ON_START=1
```

启动一次任务；不得把 Key 写入仓库、命令输出、验收报告或聊天记录。任务完成后恢复 `RUN_ON_START=0`，防止每次重启重复产生费用。

Expected: 至少生成 1 条 `session` 结果、1 条真实智能规则版本和 1 条 `smart` 结果；结果来源窗口对应当前企业真实归档消息。如果无法取得可用 Key，必须把“新模型生成”标记为验收阻塞，不能用静态前端数据代替。

- [x] **Step 5: 只重建 app 并确认健康**

先读取当前 Compose 所需的保护变量和 `deploy/standalone/.env.local`，再运行现有项目的 app-only 构建。禁止使用 `down -v` 或删除卷。

Run:

```powershell
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" --filter "name=mochat-go-desktop"
```

Expected: app、MySQL、Redis healthy，四个现有命名卷未删除或重建。

- [x] **Step 6: 在 2560×1440 自行点击会话分析**

打开：

```text
http://127.0.0.1:18080/ai-insight/session-analysis
```

逐项验证：

- 页面无三张“能力未接入”卡和大块能力说明；
- 查询输入期间不请求，点击查询/回车后请求一次；
- 重置、刷新、分页和 URL 恢复；
- 会话类型、员工、成交意愿、流失风险、日期和关键词真实改变结果；
- 导出当前筛选并检查 CSV；
- 整行详情、复选框隔离、客户分析/员工质检、证据、会话跳转；
- 关闭按钮、遮罩、Escape 和焦点返回；
- 头像和文字样式正常，表格填满可用高度。

- [x] **Step 7: 在 2560×1440 自行点击智能分析**

打开：

```text
http://127.0.0.1:18080/ai-insight/smart-analysis
```

逐项验证：

- 分析结果/分析规则标签与 URL；
- 查询、重置、刷新、分页；
- 新增规则校验、保存、版本显示；
- 编辑生成新版本，旧结果仍显示旧版本；
- 启停和删除确认；
- 结果详情、证据和会话跳转；
- 写失败时草稿保留；
- 遮罩、Escape 和焦点返回。

- [x] **Step 8: 回归常规桌面、控制台和网络**

在当前常规桌面宽度重复打开两个页面，确认无页面整体横向滚动、按钮挤压、文本重叠和异常大留白。检查控制台 error/warn 与关键网络响应；任何未解释错误均需修复或在验收报告中标记未通过。

- [x] **Step 9: 填写中文验收报告并最终复核**

把实际命令、结果、截图路径、浏览器尺寸、生成结果 ID、规则版本、容器状态和发现/修复写入报告。不得记录消息全文、Provider Key 或模型完整原始响应。

Run:

```powershell
git diff --check
git status --short
```

- [x] **Step 10: 提交验收报告和验收期修复**

```powershell
git add docs/reviews/2026-08-21-ai-insight-session-smart-acceptance.zh-CN.md
git add web/apps/dashboard/src/features/ai-insight web/apps/dashboard/src/styles/ai-insight-workspace.css internal/modules/ai-insight
git diff --cached --check
git commit -m "test: verify AI insight workspaces"
```

只暂存本任务验收期实际修改过的文件；命令中的目录暂存前必须先用 `git diff --name-only` 核对，避免纳入并行改动。

---

## Final Verification Checklist

- [x] 只改会话分析和智能分析两个菜单；
- [x] 两页不再渲染通用能力三卡片和大块介绍；
- [x] 会话分析结果按真实会话、来源窗口和来源指纹落库；
- [x] 智能分析结果关联真实不可变规则版本；
- [x] 模型 JSON、枚举、分数、置信度和证据消息均严格校验；
- [x] 页面打开、查询和刷新不调用模型；
- [x] tenant/corp/员工范围覆盖列表、详情、导出和规则对象范围；
- [x] 规则管理、导出和详情使用独立 action RBAC；
- [x] 固定每页 20 条，筛选、标签、页码和详情 ID 可由 URL 恢复；
- [x] 头像 fallback 使用名称首字，不出现固定“请”字；
- [x] 证据不足显示“证据不足”，不伪造 0 分；
- [x] 旧三个 AI 洞察页和旧 `mochat_go_ai_analysis` 兼容行为未被破坏；
- [x] 目标测试、相关全量测试、typecheck、build 与 Go 回归通过；
- [x] 0148 up/down、0151 权限回填、健康版本、索引和 MariaDB 方言验证完成；
- [x] app-only Docker 交付后 MySQL、Redis 和命名卷保持健康；
- [x] 两个分辨率的真实浏览器点击验收完成；
- [x] 正式企微归档、正式 AI Key、数据协议和成本治理只记录在总进度，不在页面重复铺陈；
- [x] 未覆盖、删除或误提交工作树中其他人的改动。
