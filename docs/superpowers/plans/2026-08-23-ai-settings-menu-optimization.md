# AI 设置菜单整体优化实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 将 Dashboard“AI 设置”下 AI 知识库与智能体管理两页改造成真实、可筛选、可持久化、可审计并可完成浏览器验收的配置工作台。

**架构：** 保留现有两张元数据表和八个 CRUD 路径，先加固 Go handler、ports 与 MySQL repository 的严格输入、状态、企业内引用、稳定错误和事务审计；再修补页面权限依赖。前端继续使用 React Query 和现有 Dashboard 组件，在真实列表结果上执行 URL 驱动的查询与分页，并明确未接入 Provider/文档索引的能力边界。

**技术栈：** Go 1.26、`database/sql`、MySQL/MariaDB、React 19、TypeScript 5.9、React Query、React Router、Ant Design、Vitest、Playwright、Docker Compose。

## 全局约束

- 实际范围只有 `/ai-setting/ai-knowledge-base` 和 `/ai-setting/agent`，不新增伪路由。
- 所有业务数据来自现有 API/MySQL；不增加前端业务假数据、伪成功或外部 Provider 占位成功。
- Phase 7 保持暂停，不扩展真实企微会话存档。
- 不修改 `docs/PROJECT_PROGRESS.zh-CN.md`、`.workbuddy/`、`tmp/`、调试脚本、旧 `web/saas-admin/` 及无关差异。
- Docker 仅使用保留卷的 `mochat-go-desktop` 项目，禁止删除 MySQL、Redis 或具名卷。
- 所有新行为先写失败测试并观察预期失败，再写最小实现。

## 文件结构

- `internal/modules/ai-settings/transport/http/handler.go`：HTTP 严格解码、验证、安全错误和跨仓储业务检查。
- `internal/modules/ai-settings/ports/repository.go`：知识库、智能体、引用与审计合同。
- `internal/modules/ai-settings/adapters/mysql/repository.go`：范围 CRUD、引用查询、事务审计和 JSON 损坏检测。
- `deploy/standalone/migrations/0155_ai_settings_integrity_audit.*.sql`：审计表和 Agent 页面 KB GET 权限映射。
- `internal/dashboard/dashboard_page_catalog.json` 与 RBAC 检查脚本：权限目录与迁移一致性。
- `web/apps/dashboard/src/features/ai-settings/`：API、URL 列表模型、两页实现和测试。
- `web/apps/dashboard/src/styles/index.css` 与 `ai-settings-layout.test.ts`：多视口布局合同。
- `web/e2e/tests/dashboard-page-actions.ts`：安全 evidence 动作。
- `docs/reviews/2026-08-23-ai-settings-menu-optimization-report.zh-CN.md`：中文实施与自测报告。

---

### Task 1：后端输入、状态和引用完整性

**文件：**

- 修改：`internal/modules/ai-settings/ports/repository.go`
- 修改：`internal/modules/ai-settings/transport/http/handler.go`
- 修改：`internal/modules/ai-settings/transport/http/handler_test.go`
- 修改：`internal/modules/ai-settings/module.go`

**接口：**

- 消费：登录 `Principal{UserID,TenantID,CorpID}` 和现有知识库/智能体仓储。
- 产出：`KnowledgeBaseRepository.GetByIDs`、`AgentRepository.ListReferencingKnowledgeBase`、稳定领域错误和只接受 `status ∈ {0,1}` 的 CRUD 合同。

- [ ] **步骤 1：写停用、非法状态、严格 JSON、引用校验和删除冲突测试**

```go
func TestKnowledgeBasePersistsDisabledStatus(t *testing.T) {
    response := requestKnowledgeBase(t, http.MethodPost, `{"name":"售后库","description":"","documentCount":0,"status":0}`)
    if response.Code != http.StatusOK || response.KnowledgeBase.Status != 0 {
        t.Fatalf("response = %#v, want persisted status 0", response)
    }
}

func TestAgentRejectsKnowledgeBaseOutsidePrincipalCorp(t *testing.T) {
    response := requestAgent(t, http.MethodPost, `{"name":"客服助手","knowledgeBaseIds":["other-corp"],"status":1}`)
    if response.Code != http.StatusBadRequest || response.MachineCode != "AI_SETTINGS_KNOWLEDGE_BASE_INVALID" {
        t.Fatalf("response = %#v", response)
    }
}
```

- [ ] **步骤 2：运行精确测试并确认因默认状态和无引用校验而失败**

```powershell
go test ./internal/modules/ai-settings/transport/http -run "TestKnowledgeBasePersistsDisabledStatus|TestAgentRejectsKnowledgeBaseOutsidePrincipalCorp|TestKnowledgeBaseDeleteRejectsReferencedRecord|TestAISettingsRejectsUnknownOrTrailingJSON" -count=1
```

预期：`status=0` 被改为 1、未知字段被接受或跨企业 ID 未被拒绝。

- [ ] **步骤 3：实现严格解码、字段验证、稳定错误与跨仓储检查**

```go
func parseStatus(value *int) (int, error) {
    if value == nil || (*value != 0 && *value != 1) {
        return 0, errInvalidStatus
    }
    return *value, nil
}
```

handler 使用 `http.MaxBytesReader`、`DisallowUnknownFields` 并验证 EOF；Agent 写入前用当前 principal 范围查询知识库；KB 删除前查询引用智能体；原始存储错误不进入响应正文。

- [ ] **步骤 4：运行 `go test ./internal/modules/ai-settings/transport/http -count=1`，确认全部通过**

- [ ] **步骤 5：提交**

```powershell
git add internal/modules/ai-settings/ports/repository.go internal/modules/ai-settings/transport/http/handler.go internal/modules/ai-settings/transport/http/handler_test.go internal/modules/ai-settings/module.go
git commit -m "fix(ai-settings): enforce scoped configuration contracts"
```

### Task 2：事务审计与迁移

**文件：**

- 创建：`deploy/standalone/migrations/0155_ai_settings_integrity_audit.up.sql`
- 创建：`deploy/standalone/migrations/0155_ai_settings_integrity_audit.down.sql`
- 创建：`internal/migration/ai_settings_integrity_audit_test.go`
- 修改：`internal/migration/migration_test.go`
- 修改：`internal/modules/ai-settings/adapters/mysql/repository.go`
- 创建：`internal/modules/ai-settings/adapters/mysql/repository_test.go`

**接口：** CRUD 与 `mochat_go_ai_settings_audits` 同事务提交；审计只含字段名和实体标识。

- [ ] **步骤 1：写迁移和仓储事务失败测试**

```go
func TestAISettingsIntegrityAuditMigration(t *testing.T) {
    up := readMigration(t, "0155_ai_settings_integrity_audit.up.sql")
    for _, fragment := range []string{"mochat_go_ai_settings_audits", "changed_fields", "actor_user_id", "idx_ai_settings_audits_scope"} {
        if !strings.Contains(up, fragment) { t.Fatalf("migration missing %q", fragment) }
    }
}
```

仓储测试用 `sqlmock` 断言 `BEGIN → 业务写 → audit INSERT → COMMIT`；审计插入失败时必须 `ROLLBACK`。

- [ ] **步骤 2：运行并确认 0155、事务和审计缺失**

```powershell
go test ./internal/migration -run "TestAISettingsIntegrityAuditMigration|TestDefaultMigrationsLatest" -count=1
go test ./internal/modules/ai-settings/adapters/mysql -count=1
```

- [ ] **步骤 3：实现非破坏迁移与事务仓储**

```sql
CREATE TABLE IF NOT EXISTS `mochat_go_ai_settings_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int unsigned NOT NULL,
  `corp_id` int unsigned NOT NULL,
  `actor_user_id` int unsigned NOT NULL,
  `entity_type` varchar(32) NOT NULL,
  `entity_id` varchar(64) NOT NULL,
  `action` varchar(16) NOT NULL,
  `changed_fields` json NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ai_settings_audits_scope` (`tenant_id`,`corp_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

创建、更新、删除在同一 `sql.Tx` 内写审计；删除补写 `updated_by`；`scanAgent` 遇到损坏 JSON返回错误。

- [ ] **步骤 4：运行 `go test ./internal/migration ./internal/modules/ai-settings/... -count=1`**

- [ ] **步骤 5：提交**

```powershell
git add deploy/standalone/migrations/0155_ai_settings_integrity_audit.* internal/migration internal/modules/ai-settings/adapters/mysql
git commit -m "feat(ai-settings): add transactional configuration audit"
```

### Task 3：修复智能体页 RBAC 依赖

**文件：**

- 修改：`deploy/standalone/migrations/0155_ai_settings_integrity_audit.up.sql`
- 修改：`deploy/standalone/migrations/0155_ai_settings_integrity_audit.down.sql`
- 修改：`internal/dashboard/dashboard_page_catalog.json`
- 修改：`scripts/check_dashboard_page_rbac_catalog.mjs`
- 修改：`scripts/check_dashboard_page_rbac_catalog.test.mjs`

**接口：** `dashboard.ai_setting.agent` 除 Agent CRUD 外可读取 KB GET，但不能获得 KB 写权限。

- [ ] **步骤 1：写 Agent 页面 KB GET 资源测试**

```js
assert.deepEqual(
  agentPage.resources.filter((resource) => resource.pathPattern.includes('knowledge-bases')),
  [{ method: 'GET', pathPattern: '/dashboard/ai-settings/knowledge-bases', scopeRequired: false }],
);
```

- [ ] **步骤 2：运行 `node --test scripts/check_dashboard_page_rbac_catalog.test.mjs` 并确认缺少映射**

- [ ] **步骤 3：更新目录、0155 overlay 与校验脚本**

0155 为 `dashboard.ai_setting.agent` 插入 KB GET 资源；down 只删除该 permission-resource 组合。目录声明共享 GET 所有者，脚本把 0155 overlay 纳入最终 seeded mappings。

- [ ] **步骤 4：运行 RBAC 与 benchmark 门禁**

```powershell
corepack pnpm check:yuanhu-benchmark
corepack pnpm check:phase4-dashboard-page-rbac
```

预期：53 页、48 普通页、5 超管页、0 个未映射 API。

- [ ] **步骤 5：提交 `fix(rbac): authorize agent knowledge base dependency`**

### Task 4：前端 API 与 URL 列表模型

**文件：**

- 修改：`web/apps/dashboard/src/features/ai-settings/ai-settings-api.ts`
- 创建：`web/apps/dashboard/src/features/ai-settings/ai-settings-api.test.ts`
- 创建：`web/apps/dashboard/src/features/ai-settings/list-state.ts`
- 创建：`web/apps/dashboard/src/features/ai-settings/list-state.test.ts`

**接口：** 产出 `parseAISettingsListState`、`filterAndPageAISettings`、`formatAISettingsTime` 和 `resolveKnowledgeBaseNames`。

- [ ] **步骤 1：写八个 API 请求和 URL 状态失败测试**

```ts
it('persists an explicit disabled status', async () => {
  await api.updateAgent(9, 'agent-1', { name: '客服助手', description: '', knowledgeBaseIds: [], status: 0 });
  expect(request).toHaveBeenCalledWith('/ai-settings/agents/agent-1?corpId=9', expect.objectContaining({ method: 'PUT', body: expect.stringContaining('"status":0') }));
});

it('clamps page and filters real records', () => {
  const result = filterAndPageAISettings(items, { q: '客服', status: 'disabled', page: 9, pageSize: 10 });
  expect(result.page).toBe(1);
  expect(result.items.map((item) => item.id)).toEqual(['disabled-agent']);
});
```

- [ ] **步骤 2：运行目标测试并确认模块/覆盖不存在**

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/ai-settings/ai-settings-api.test.ts src/features/ai-settings/list-state.test.ts
```

- [ ] **步骤 3：实现类型化输入、URL 解析、过滤分页和关联显示**

```ts
export type AISettingsListState = { q: string; status: 'all' | 'enabled' | 'disabled'; page: number; pageSize: 10 | 20 | 50 };

export function filterAndPageAISettings<T extends { name: string; description: string; status: number }>(items: readonly T[], state: AISettingsListState) {
  const keyword = state.q.trim().toLocaleLowerCase();
  const filtered = items.filter((item) => (!keyword || `${item.name} ${item.description}`.toLocaleLowerCase().includes(keyword))
    && (state.status === 'all' || item.status === (state.status === 'enabled' ? 1 : 0)));
  const pages = Math.max(1, Math.ceil(filtered.length / state.pageSize));
  const page = Math.min(state.page, pages);
  return { items: filtered.slice((page - 1) * state.pageSize, page * state.pageSize), total: filtered.length, page };
}
```

- [ ] **步骤 4：重跑目标测试并确认全部通过**

- [ ] **步骤 5：提交 `test(ai-settings): define api and list state contracts`**

### Task 5：优化两页界面与交互

**文件：**

- 修改：`web/apps/dashboard/src/features/ai-settings/knowledge-base-page.tsx`
- 修改：`web/apps/dashboard/src/features/ai-settings/agent-page.tsx`
- 修改：`web/apps/dashboard/src/features/ai-settings/ai-settings-pages.test.tsx`
- 修改：`web/apps/dashboard/src/styles/index.css`
- 创建：`web/apps/dashboard/src/styles/ai-settings-layout.test.ts`
- 修改：`web/e2e/tests/dashboard-page-actions.ts`

**接口：** 使用任务 4 helpers 和共享 Dashboard 组件，产出查询/重置/刷新、分页、URL 恢复、CRUD/停用/冲突反馈与能力边界。

- [ ] **步骤 1：写页面和布局失败测试**

```tsx
it('filters agents through URL state and renders real knowledge base names', async () => {
  renderAgentPage('/ai-setting/agent?q=客服&status=enabled&page=1&pageSize=10');
  expect(await screen.findByText('售后话术库')).toBeInTheDocument();
  expect(screen.queryByText('停用助手')).not.toBeInTheDocument();
});

it('reports a referenced knowledge base deletion conflict', async () => {
  await confirmDelete('售后话术库');
  expect(await screen.findByRole('alert')).toHaveTextContent('仍被智能体引用');
});
```

样式测试断言工作台宽屏 `width: 100%`、表格局部滚动、移动筛选换行、小高度对话框内部滚动且 hover 无 transform/filter。

- [ ] **步骤 2：运行页面与样式测试并确认筛选、分页、关联名称和反馈缺失**

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/ai-settings/ai-settings-pages.test.tsx src/styles/ai-settings-layout.test.ts
```

- [ ] **步骤 3：实现两页工作台**

两页使用 `DashboardFilterPanel` 与 `DashboardPagination`；查询通过 `updateSearch` 写 `q/status/page/pageSize`。知识库页将 `documentCount` 明确为登记值并展示未接入提示；智能体页联结真实知识库名称，区分 loading/error/empty/invalid reference，保存/删除失败保留上下文。

- [ ] **步骤 4：实现弹层脏数据保护、pending 锁定和响应式样式**

关闭按钮、遮罩和 Escape 使用同一处理；无改动直接关闭，有改动先确认；保存或删除 pending 时禁用重复操作。宽屏填满右栏，窄屏表格局部滚动且按钮可达。

- [ ] **步骤 5：运行目标、共享交互和 E2E 静态测试**

```powershell
corepack pnpm --filter @mochat/dashboard exec vitest run src/features/ai-settings src/styles/ai-settings-layout.test.ts src/components/dashboard-interactions.test.tsx src/styles/dashboard-scroll-layout.test.ts
corepack pnpm --filter @mochat/e2e lint
corepack pnpm --filter @mochat/e2e typecheck
```

- [ ] **步骤 6：提交 `feat(ai-settings): optimize configuration workspaces`**

### Task 6：全量验证、Docker、浏览器 evidence 与报告

**文件：**

- 创建：`docs/reviews/2026-08-23-ai-settings-menu-optimization-report.zh-CN.md`
- 创建：`web/e2e/artifacts/ai-settings-menu-optimization/` 下截图与 `evidence.json`

**接口：** 消费任务 1–5 和保留卷运行环境，产出可复核证据与中文报告。

- [ ] **步骤 1：执行新鲜自动化门禁**

```powershell
corepack pnpm --filter @mochat/dashboard lint
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard build
go test ./internal/modules/ai-settings/... ./internal/dashboard ./internal/store ./internal/server ./internal/migration -count=1
corepack pnpm check:yuanhu-benchmark
corepack pnpm check:phase4-dashboard-page-rbac
corepack pnpm check:dashboard-all-pages-evidence
git diff --check
```

- [ ] **步骤 2：记录卷和容器，执行迁移并仅重建 app**

```powershell
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps
docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop
docker compose -p mochat-go-desktop --env-file deploy/standalone/.env.local -f deploy/standalone/docker-compose.yml up -d --build --no-deps app
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml ps
```

- [ ] **步骤 3：真实浏览器执行两页核心工作流**

在 2560×1440、1366×900、390×844 和 1366×620 覆盖查询、重置、刷新、分页、新增、编辑、停用、删除确认/取消、被引用删除失败、刷新后持久化、网络失败反馈、关闭按钮、遮罩、Escape、横向溢出、console error 与关键请求状态。测试数据使用唯一 `Codex AI 设置验收-*` 前缀并经 UI 清理；不执行 Provider、文档上传或索引写操作。

- [ ] **步骤 4：生成 `evidence.json` 与多视口截图并运行 evidence 门禁**

- [ ] **步骤 5：编写中文实施与自测报告，列出提交、路由、接口、0155、验证结果、浏览器证据、容器卷、已知 RBAC 边界和未执行外部项**

- [ ] **步骤 6：请求独立代码审查，修复 Critical/Important 问题并重跑受影响测试**

- [ ] **步骤 7：提交**

```powershell
git add docs/reviews/2026-08-23-ai-settings-menu-optimization-report.zh-CN.md web/e2e/artifacts/ai-settings-menu-optimization
git commit -m "docs(ai-settings): report implementation verification"
```
