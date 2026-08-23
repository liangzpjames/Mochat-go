# AI 设置知识库运行链路与默认会话分析助手 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 AI 知识库增加真实文档上传、解析、持久化和检索，并让唯一系统“会话分析助手”真实驱动 AI 洞察会话分析。

**Architecture:** 新增 tenant/corp 范围内的知识文档与分块表，文件保存到 `/static` 之外的私有同卷目录，Go 服务同步校验并解析 `.txt/.md/.pdf/.docx`。AI Insight runner 通过运行时设置接口读取固定助手、相关知识块和设置指纹，在保持消息证据边界的前提下注入有预算的业务上下文；Dashboard 只展示一个可编辑系统助手，并在会话分析页展示消费摘要。

**Tech Stack:** Go 1.26、`database/sql`、MariaDB/MySQL、纯 Go PDF 文本提取、标准库 ZIP/XML、React 19、TypeScript 5.9、React Query、Vitest、Docker Compose、Playwright。

## Global Constraints

- 文档格式仅支持 `.txt/.md/.pdf/.docx`，旧 `.doc` 明确拒绝。
- 单文件 20 MB、单文档 500,000 Unicode 字符、单知识库 100 个未删除文档。
- 用户不能新增、改名或删除智能体；每企业只有一个 `system_key=session-analysis` 的“会话分析助手”。
- 所有数据必须来自真实 API、私有文件存储和 MySQL；不得增加静态业务数组、伪成功或 Provider 占位成功。
- Phase 7 保持暂停；不修改 `docs/PROJECT_PROGRESS.zh-CN.md`、`.workbuddy/`、`tmp/`、调试脚本和旧 `web/saas-admin/`。
- Docker 只使用保留卷 Compose，禁止删除卷；所有行为按 TDD 先红后绿。

## 文件结构

- `deploy/standalone/migrations/0156_ai_settings_knowledge_runtime.*.sql`：文档、分块、系统智能体和权限资源。
- `internal/modules/ai-settings/ports/repository.go`：文档、系统助手和运行摘要领域合同。
- `internal/modules/ai-settings/document_parser.go`：格式识别、TXT/Markdown、DOCX、PDF 解析和分块。
- `internal/modules/ai-settings/private_storage.go`：私有根目录、临时文件、原子移动和安全删除。
- `internal/modules/ai-settings/adapters/mysql/document_repository.go`：文档/分块事务与真实计数。
- `internal/modules/ai-settings/adapters/mysql/runtime_repository.go`：默认助手幂等创建、运行时知识候选与设置指纹。
- `internal/modules/ai-settings/transport/http/document_handler.go`：嵌套文档 HTTP API。
- `internal/modules/ai-settings/transport/http/handler.go`：知识库真实计数、系统助手只读/更新合同。
- `internal/modules/ai-insight/conversation_runner.go`：助手停用、说明、知识检索和组合指纹。
- `internal/modules/ai-insight/workspace_handler.go`：AI 洞察消费摘要。
- `web/apps/dashboard/src/features/ai-settings/`：文档 API、知识库文档管理和单助手页面。
- `web/apps/dashboard/src/features/ai-insight/`：会话分析助手摘要。
- `docs/reviews/2026-08-23-ai-settings-knowledge-runtime-report.zh-CN.md`：最终实施与自测报告。

---

### Task 1：迁移、领域合同和权限资源

**Files:**
- Create: `deploy/standalone/migrations/0156_ai_settings_knowledge_runtime.up.sql`
- Create: `deploy/standalone/migrations/0156_ai_settings_knowledge_runtime.down.sql`
- Create: `internal/migration/ai_settings_knowledge_runtime_test.go`
- Modify: `internal/migration/migration_test.go`
- Modify: `internal/modules/ai-settings/ports/repository.go`
- Modify: `internal/dashboard/dashboard_page_catalog.json`

**Interfaces:**
- Produces: `ports.KnowledgeDocument`, `ports.KnowledgeChunk`, `ports.SessionAssistant`, `ports.RuntimeContext` 及文档仓储合同。

- [ ] **Step 1: Write the failing migration and contract tests**

```go
func TestAISettingsKnowledgeRuntimeMigration(t *testing.T) {
    up := readMigration(t, "0156_ai_settings_knowledge_runtime.up.sql")
    for _, fragment := range []string{"mochat_go_ai_knowledge_documents", "mochat_go_ai_knowledge_chunks", "system_key", "session-analysis"} {
        if !strings.Contains(up, fragment) { t.Fatalf("migration missing %q", fragment) }
    }
}
```

- [ ] **Step 2: Run tests and observe the missing 0156 failure**

Run: `go test ./internal/migration -run "TestAISettingsKnowledgeRuntimeMigration|TestDefaultMigrationsLatest" -count=1`

Expected: FAIL because 0156 and the new schema are absent.

- [ ] **Step 3: Add the additive schema and scoped indexes**

```sql
ALTER TABLE `mochat_go_ai_agents` ADD COLUMN `system_key` varchar(64) NULL AFTER `corp_id`;
CREATE UNIQUE INDEX `uq_ai_agents_system_key` ON `mochat_go_ai_agents` (`tenant_id`,`corp_id`,`system_key`);
CREATE TABLE `mochat_go_ai_knowledge_documents` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint NOT NULL, `corp_id` bigint NOT NULL,
  `knowledge_base_id` varchar(36) NOT NULL, `filename` varchar(255) NOT NULL,
  `extension` varchar(16) NOT NULL, `mime_type` varchar(128) NOT NULL,
  `object_key` varchar(512) NOT NULL, `size_bytes` bigint NOT NULL, `sha256` char(64) NOT NULL,
  `status` varchar(16) NOT NULL, `error_summary` varchar(512) NOT NULL DEFAULT '',
  `character_count` int NOT NULL DEFAULT 0, `chunk_count` int NOT NULL DEFAULT 0,
  `created_by` bigint NOT NULL, `updated_by` bigint NOT NULL,
  `created_at` datetime(6) NOT NULL, `updated_at` datetime(6) NOT NULL, `deleted_at` datetime(6) NULL,
  PRIMARY KEY (`id`), KEY `idx_ai_documents_scope` (`tenant_id`,`corp_id`,`knowledge_base_id`,`deleted_at`)
);
CREATE TABLE `mochat_go_ai_knowledge_chunks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT, `tenant_id` bigint NOT NULL, `corp_id` bigint NOT NULL,
  `knowledge_base_id` varchar(36) NOT NULL, `document_id` varchar(36) NOT NULL,
  `ordinal` int NOT NULL, `content` text NOT NULL, `character_count` int NOT NULL,
  PRIMARY KEY (`id`), UNIQUE KEY `uq_ai_chunk_ordinal` (`document_id`,`ordinal`),
  KEY `idx_ai_chunks_scope` (`tenant_id`,`corp_id`,`knowledge_base_id`,`document_id`)
);
```

迁移为已有 tenant/corp binding 插入固定名称、`system_key='session-analysis'` 的助手，并给知识库页面权限加入三条嵌套文档资源。

- [ ] **Step 4: Run migration, RBAC catalog and package tests**

Run: `go test ./internal/migration ./internal/dashboard -run "AISettings|Migration|PageCatalog" -count=1`

Expected: PASS.

- [ ] **Step 5: Commit schema boundary**

```powershell
git add deploy/standalone/migrations/0156_ai_settings_knowledge_runtime.* internal/migration internal/modules/ai-settings/ports/repository.go internal/dashboard/dashboard_page_catalog.json
git commit -m "feat(ai-settings): add knowledge document schema"
```

### Task 2：安全文件解析与私有存储

**Files:**
- Create: `internal/modules/ai-settings/document_parser.go`
- Create: `internal/modules/ai-settings/document_parser_test.go`
- Create: `internal/modules/ai-settings/private_storage.go`
- Create: `internal/modules/ai-settings/private_storage_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces: `ParseDocument(path, filename string) (ParsedDocument, error)`、`ChunkText(text string) []ports.KnowledgeChunk`、`PrivateStorage.Stage/Commit/Delete`。

- [ ] **Step 1: Write failing parser and path-safety tests**

```go
func TestParseDocumentSupportsUTF8Text(t *testing.T) {
    path := filepath.Join(t.TempDir(), "guide.txt")
    require.NoError(t, os.WriteFile(path, []byte("退款需要主管审批"), 0o600))
    parsed, err := ParseDocument(path, "guide.txt")
    require.NoError(t, err)
    require.Equal(t, "退款需要主管审批", parsed.Text)
    require.Len(t, parsed.SHA256, 64)
}

func TestParseDocumentRejectsLegacyDOC(t *testing.T) {
    _, err := ParseDocument(filepath.Join(t.TempDir(), "legacy.doc"), "legacy.doc")
    require.ErrorIs(t, err, ErrUnsupportedDocumentType)
}

func TestChunkTextHasBoundedOverlap(t *testing.T) {
    chunks := ChunkText(strings.Repeat("知识", 1400))
    require.Greater(t, len(chunks), 1)
    require.LessOrEqual(t, utf8.RuneCountInString(chunks[0].Content), 1200)
}

func TestPrivateStorageNeverEscapesPrivateRoot(t *testing.T) {
    storage, err := NewPrivateStorage(filepath.Join(t.TempDir(), "static"))
    require.NoError(t, err)
    _, err = storage.Resolve("../../outside.pdf")
    require.ErrorIs(t, err, ErrUnsafeObjectKey)
}
```

- [ ] **Step 2: Run the target tests and observe undefined types/functions**

Run: `go test ./internal/modules/ai-settings -run "TestParseDocument|TestChunkText|TestPrivateStorage" -count=1`

Expected: compile failure for missing parser/storage.

- [ ] **Step 3: Implement strict format parsing and limits**

```go
const maxUploadBytes int64 = 20 << 20
const maxDocumentRunes = 500_000

type ParsedDocument struct {
    Text string
    SHA256 string
    CharacterCount int
}
```

TXT/Markdown 验证 UTF-8；DOCX 只读取受解压大小限制的 `word/document.xml`；PDF 使用固定版本纯 Go 依赖提取文字层；空文字、加密、损坏和超限返回稳定领域错误。私有根为 `filepath.Join(filepath.Dir(fileStorageRoot), "ai-knowledge-private")`，所有最终路径做绝对路径前缀校验。

- [ ] **Step 4: Run parser tests and vulnerability cases**

Run: `go test ./internal/modules/ai-settings -count=1`

Expected: PASS, including path traversal and zip expansion limits.

- [ ] **Step 5: Commit parser/storage**

```powershell
git add go.mod go.sum internal/modules/ai-settings/document_parser* internal/modules/ai-settings/private_storage*
git commit -m "feat(ai-settings): parse private knowledge documents"
```

### Task 3：文档仓储与 HTTP 工作流

**Files:**
- Create: `internal/modules/ai-settings/adapters/mysql/document_repository.go`
- Create: `internal/modules/ai-settings/adapters/mysql/document_repository_test.go`
- Create: `internal/modules/ai-settings/transport/http/document_handler.go`
- Create: `internal/modules/ai-settings/transport/http/document_handler_test.go`
- Modify: `internal/modules/ai-settings/transport/http/routes.go`
- Modify: `internal/modules/ai-settings/transport/http/handler.go`
- Modify: `internal/modules/ai-settings/module.go`
- Modify: `internal/app/bootstrap/ai_settings.go`
- Modify: `cmd/mochat-go/ai_debt_clearance.go`

**Interfaces:**
- Consumes: parser、private storage、principal、authorizer。
- Produces: list/upload/delete document routes and derived knowledge-base counts.

- [ ] **Step 1: Write failing upload/list/delete and tenant isolation tests**

```go
func TestDocumentUploadPersistsReadyDocumentAndChunks(t *testing.T) {
    handler, repo := newDocumentTestHandler(t, Principal{UserID: 7, TenantID: 1, CorpID: 2})
    response := uploadDocument(t, handler, "/dashboard/ai-settings/knowledge-bases/kb-1/documents", "guide.txt", []byte("退款需要主管审批"))
    require.Equal(t, http.StatusCreated, response.Code)
    require.Equal(t, "ready", repo.documents[0].Status)
    require.NotEmpty(t, repo.chunks[repo.documents[0].ID])
}

func TestDocumentRoutesRejectCrossCorpParent(t *testing.T) {
    handler, _ := newDocumentTestHandler(t, Principal{UserID: 7, TenantID: 1, CorpID: 2})
    response := uploadDocument(t, handler, "/dashboard/ai-settings/knowledge-bases/other-corp/documents", "guide.txt", []byte("content"))
    require.Equal(t, http.StatusNotFound, response.Code)
}

func TestKnowledgeBaseCountIsDerivedFromDocuments(t *testing.T) {
    repo := newKnowledgeBaseRepoWithDocuments(3, 2)
    item := repo.list(t.Context(), 1, 2)[0]
    require.Equal(t, 3, item.DocumentCount)
    require.Equal(t, 2, item.ReadyDocumentCount)
}
```

- [ ] **Step 2: Run focused tests and observe missing route/repository failures**

Run: `go test ./internal/modules/ai-settings/... -run "Document|Derived" -count=1`

Expected: FAIL before the nested routes exist.

- [ ] **Step 3: Implement transactional metadata/chunks and safe file lifecycle**

```go
const DocumentsPath = KnowledgeBasesPath + "/{id}/documents"
const DocumentPath = DocumentsPath + "/{documentId}"
```

上传验证父知识库、100 文档限额、multipart 上限、文件结构和租户范围；ready 文档与 chunks 同事务写入。删除先把私有文件原子移动到隔离路径，数据库事务软删文档并删除 chunks；失败恢复文件，提交后清除隔离文件。知识库删除存在文档时返回 409。

- [ ] **Step 4: Run all AI settings and bootstrap tests**

Run: `go test ./internal/modules/ai-settings/... ./internal/app/bootstrap ./cmd/mochat-go -count=1`

Expected: PASS.

- [ ] **Step 5: Commit real document workflow**

```powershell
git add internal/modules/ai-settings internal/app/bootstrap/ai_settings.go cmd/mochat-go/ai_debt_clearance.go
git commit -m "feat(ai-settings): add document management workflow"
```

### Task 4：收敛为唯一系统助手

**Files:**
- Modify: `internal/modules/ai-settings/adapters/mysql/repository.go`
- Create: `internal/modules/ai-settings/adapters/mysql/runtime_repository.go`
- Create: `internal/modules/ai-settings/adapters/mysql/runtime_repository_test.go`
- Modify: `internal/modules/ai-settings/transport/http/handler.go`
- Modify: `internal/modules/ai-settings/transport/http/handler_test.go`
- Modify: `internal/modules/ai-settings/transport/http/routes.go`

**Interfaces:**
- Produces: `EnsureSessionAssistant(ctx, tenantID, corpID, actorID)` and a GET/PUT-only agent contract.

- [ ] **Step 1: Write failing uniqueness and method-contract tests**

```go
func TestAgentListEnsuresOnlySessionAnalysisAssistant(t *testing.T) {
    repo := &fakeAgentRepo{items: []ports.Agent{{ID: "legacy", Name: "旧自定义智能体"}}}
    response := perform(NewAgentHandler(repo, newKBRepo(), principalResolver(1, 2, 7), nil, fixedID("session")), http.MethodGet, AgentsPath, "")
    require.Equal(t, http.StatusOK, response.Code)
    require.Equal(t, []string{"会话分析助手"}, decodedAgentNames(t, response.Body.Bytes()))
}

func TestAgentCreateAndDeleteAreMethodNotAllowed(t *testing.T) {
    router := registeredAISettingsRouter(t)
    require.Equal(t, http.StatusMethodNotAllowed, request(router, http.MethodPost, AgentsPath).Code)
    require.Equal(t, http.StatusMethodNotAllowed, request(router, http.MethodDelete, AgentsPath+"/session").Code)
}

func TestAgentUpdateCannotRenameSystemAssistant(t *testing.T) {
    response := updateSessionAssistant(t, `{"name":"其他名称","description":"分析售后风险","knowledgeBaseIds":[],"status":1}`)
    require.Equal(t, http.StatusBadRequest, response.Code)
}
```

- [ ] **Step 2: Run agent tests and observe POST/DELETE still succeed**

Run: `go test ./internal/modules/ai-settings/... -run "SessionAssistant|MethodNotAllowed|CannotRename" -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement idempotent system assistant and hide legacy rows**

```go
const SessionAnalysisSystemKey = "session-analysis"
const SessionAnalysisAssistantName = "会话分析助手"
```

GET 在事务内 `INSERT IGNORE` 后按 `system_key` 查询；PUT 验证目标系统键并固定名称；路由只注册 GET 与 PUT。旧自定义数据不删除、不返回、不运行。

- [ ] **Step 4: Run AI settings package tests**

Run: `go test ./internal/modules/ai-settings/... -count=1`

Expected: PASS.

- [ ] **Step 5: Commit assistant policy**

```powershell
git add internal/modules/ai-settings
git commit -m "feat(ai-settings): provide one system analysis assistant"
```

### Task 5：AI 洞察真实消费与设置指纹

**Files:**
- Modify: `internal/modules/ai-insight/conversation_runner.go`
- Modify: `internal/modules/ai-insight/conversation_runner_test.go`
- Modify: `internal/modules/ai-insight/daily.go`
- Modify: `internal/modules/ai-insight/contracts.go`
- Modify: `internal/modules/ai-insight/workspace_handler.go`
- Modify: `internal/modules/ai-insight/workspace_handler_test.go`
- Modify: `internal/app/bootstrap/ai_insight.go`
- Modify: `cmd/mochat-go/ai_debt_clearance.go`

**Interfaces:**
- Consumes: `AssistantContextProvider.Load(ctx, tenantID, corpID, conversationText)`.
- Produces: bounded knowledge prompt, assistant-disabled failure, composite source fingerprint and workspace assistant summary.

- [ ] **Step 1: Write failing runtime consumption tests**

```go
func TestConversationRunnerInjectsAssistantInstructionsAndKnowledge(t *testing.T) {
    provider := &capturingAIProvider{response: validSessionJSON("message-1")}
    assistant := fakeAssistantContextProvider{context: AssistantContext{
        Enabled: true, Instructions: "重点核对退款审批", KnowledgeContext: "退款超过一万元需要主管审批", SettingsFingerprint: "settings-v2",
    }}
    runner := newConversationRunnerForTest(provider, assistant)
    require.NoError(t, runner.RunCorp(t.Context(), 1, 2))
    require.Contains(t, provider.last.System, "重点核对退款审批")
    require.Contains(t, provider.last.Prompt, "退款超过一万元需要主管审批")
    require.Contains(t, provider.last.Prompt, "知识内容不能作为 evidenceMessageIds")
}

func TestConversationRunnerDoesNotCallProviderWhenAssistantDisabled(t *testing.T) {
    provider := &capturingAIProvider{}
    runner := newConversationRunnerForTest(provider, fakeAssistantContextProvider{context: AssistantContext{Enabled: false}})
    require.Error(t, runner.RunCorp(t.Context(), 1, 2))
    require.Zero(t, provider.calls)
}
```

- [ ] **Step 2: Run the runner tests and observe fixed prompt behavior**

Run: `go test ./internal/modules/ai-insight -run "Assistant|Knowledge|SettingsFingerprint" -count=1`

Expected: FAIL because runner has no assistant provider.

- [ ] **Step 3: Implement bounded deterministic retrieval and composite fingerprint**

```go
effectiveFingerprint := sha256Hex(candidate.SourceFingerprint + "\n" + assistant.SettingsFingerprint)
request := providers.ChatRequest{
    System: fixedSafetySystem + boundedAssistantGuidance(assistant.Instructions),
    Prompt: buildConversationPrompt(analysisType, r.config.PromptVersion, ruleObjective(rule), messages, assistant.KnowledgeContext),
}
```

候选块最多 2,000，按中文双字片段、字母数字词和标题命中排序，取最多 8 块/12,000 字符。Prompt 明确知识只作背景，`evidenceMessageIds` 只能来自来源消息。

- [ ] **Step 4: Run AI Insight and integration tests**

Run: `go test ./internal/modules/ai-insight/... ./internal/app/bootstrap ./cmd/mochat-go -count=1`

Expected: PASS.

- [ ] **Step 5: Commit runtime linkage**

```powershell
git add internal/modules/ai-insight internal/app/bootstrap/ai_insight.go cmd/mochat-go/ai_debt_clearance.go
git commit -m "feat(ai-insight): consume session assistant knowledge"
```

### Task 6：Dashboard 知识库、单助手与消费摘要

**Files:**
- Modify: `web/apps/dashboard/src/features/ai-settings/ai-settings-api.ts`
- Modify: `web/apps/dashboard/src/features/ai-settings/ai-settings-api.test.ts`
- Modify: `web/apps/dashboard/src/features/ai-settings/knowledge-base-page.tsx`
- Modify: `web/apps/dashboard/src/features/ai-settings/agent-page.tsx`
- Modify: `web/apps/dashboard/src/features/ai-settings/ai-settings-pages.test.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-api.ts`
- Modify: `web/apps/dashboard/src/features/ai-insight/session-analysis-page.tsx`
- Modify: `web/apps/dashboard/src/features/ai-insight/ai-insight-workspace-pages.test.tsx`
- Modify: `web/apps/dashboard/src/styles/index.css`

**Interfaces:**
- Produces: typed document API, document manager dialog, one system assistant card, AI Insight assistant summary.

- [ ] **Step 1: Write failing UI/API tests**

```tsx
it('uploads a selected document and refreshes derived counts', async () => {})
it('shows parse failures without claiming the document is usable', async () => {})
it('renders one fixed session assistant without create or delete actions', async () => {})
it('shows the consuming assistant on the session analysis page', async () => {})
```

- [ ] **Step 2: Run focused Vitest files and observe missing controls**

Run: `corepack pnpm --filter @mochat/dashboard exec vitest run src/features/ai-settings/ai-settings-api.test.ts src/features/ai-settings/ai-settings-pages.test.tsx src/features/ai-insight/ai-insight-workspace-pages.test.tsx`

Expected: FAIL.

- [ ] **Step 3: Implement real document manager and single assistant UI**

```ts
export type KnowledgeDocumentItem = {
  id: string; knowledgeBaseId: string; filename: string; status: 'ready' | 'failed';
  sizeBytes: number; characterCount: number; chunkCount: number; errorSummary: string; createdAt: string;
};
```

知识库表单移除人工文档数；“管理文档”弹层支持筛选、上传、失败状态和删除确认。助手页只保留固定卡片及编辑/启停，AI 洞察会话分析头部展示消费摘要。复用 Dashboard 组件并保持 URL、Escape、遮罩和焦点合同。

- [ ] **Step 4: Run focused and full Dashboard tests**

Run: `corepack pnpm --filter @mochat/dashboard test`

Expected: all Dashboard tests PASS.

- [ ] **Step 5: Commit UI workflow**

```powershell
git add web/apps/dashboard/src/features/ai-settings web/apps/dashboard/src/features/ai-insight web/apps/dashboard/src/styles/index.css
git commit -m "feat(dashboard): manage analysis knowledge documents"
```

### Task 7：迁移、Docker、浏览器与最终报告

**Files:**
- Modify: `web/e2e/tests/dashboard-page-actions.ts`
- Create: `docs/reviews/2026-08-23-ai-settings-knowledge-runtime-report.zh-CN.md`

**Interfaces:**
- Produces: fresh verification evidence and final atomic commits.

- [ ] **Step 1: Add safe page evidence actions and run focused browser automation tests**

文档页面 evidence 只打开管理弹层、验证文件选择控件和关闭行为，不自动上传外部数据；真实上传使用本地无敏感测试文件，在已登录 Docker 环境执行并清理。

- [ ] **Step 2: Run complete fresh verification**

```powershell
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard lint
corepack pnpm --filter @mochat/dashboard build
go test ./internal/modules/ai-settings/... ./internal/modules/ai-insight/... ./internal/migration ./internal/dashboard ./internal/server ./cmd/mochat-go -count=1
corepack pnpm check:yuanhu-benchmark
corepack pnpm check:phase4-dashboard-page-rbac
corepack pnpm check:dashboard-all-pages-evidence
git diff --check
```

Expected: every command exits 0.

- [ ] **Step 3: Apply 0156 on preserved-volume Docker and verify health**

使用 `deploy/standalone/docker-compose.yml` 的现有项目只重建 app；运行 migration status/up、HTTP health、数据库文档/分块/系统助手查询。禁止 `down -v`。

- [ ] **Step 4: Complete real browser acceptance**

覆盖两个 AI 设置路由和 AI 洞察会话分析：知识库新增/编辑/启停、四格式上传、失败格式、文档删除、刷新持久化；助手编辑/启停/关联与无新增入口；消费摘要；关闭按钮/遮罩/Escape；2560×1440、1366×900、390×844 和小高度。外部 Provider 未就绪时只确认明确受限，不宣称模型生成成功。

- [ ] **Step 5: Write Chinese report and commit evidence**

报告列出提交、路由、迁移、接口、文件格式、验证命令结果、浏览器截图/网络证据、已知限制和外部未执行项。

```powershell
git add web/e2e/tests/dashboard-page-actions.ts docs/reviews/2026-08-23-ai-settings-knowledge-runtime-report.zh-CN.md
git commit -m "docs(ai-settings): record knowledge runtime acceptance"
```
