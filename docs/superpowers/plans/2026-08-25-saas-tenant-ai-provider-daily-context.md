# SaaS 租户级 AI Provider、日分析与历史上下文 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为每个 SaaS 客户租户提供加密、带有效期且失败关闭的 AI Provider 配置，并将 AI 分析统一为当日会话级结构化结果，同时在请求中加入上一自然日同会话评分和摘要。

**Architecture:** 新增 `aiproviderconfig` 领域包保存配置合同、AES-GCM 凭证管理器和运行时 Resolver；MySQLStore 实现租户配置读写；SaaS Admin Handler 与 React 租户详情提供脱敏管理界面。AI runner 在每个租户执行前解析 Provider，固定 Asia/Shanghai 当日窗口，并从统一结果表读取上一日快照；旧页面摘要表和调用链删除。

**Tech Stack:** Go 1.23、MariaDB 10.6、AES-GCM、React 19、TypeScript 5.8、TanStack Query、Vitest、Docker Compose。

## Global Constraints

- API Key 永不进入响应、日志、审计、URL、命令行、测试快照或 Git；数据库只保存认证加密密文和不可认证的末四位提示。
- 租户配置缺失、未生效、过期、停用、解密失败或字段无效均失败关闭，不回退全局 Provider。
- 自动日分析和启动即分析开关保持关闭；保存配置不触发模型调用。
- 分析事实来源严格限定为 Asia/Shanghai 当日 00:00 至执行时刻的归档消息。
- 上一次结果只能作为连续性参考，不能作为 `evidenceMessageIds` 或替代当日消息。
- 精确删除旧表 `mochat_go_ai_analysis`；不删除规则表、规则版本表、会话结果表或运行审计表。
- 不删除 Docker 数据卷，不修改已应用迁移字节，不 push、不合并，不输出任何真实凭证。

---

### Task 1: 租户 Provider 配置迁移与加密领域

**Files:**
- Create: `deploy/standalone/migrations/0164_saas_tenant_ai_provider.up.sql`
- Create: `deploy/standalone/migrations/0164_saas_tenant_ai_provider.down.sql`
- Create: `internal/aiproviderconfig/types.go`
- Create: `internal/aiproviderconfig/manager.go`
- Create: `internal/aiproviderconfig/manager_test.go`
- Modify: `internal/migration/migration_test.go`

**Interfaces:**
- Produces: `aiproviderconfig.StoredConfig`, `RuntimeConfig`, `Reader`, `Cipher`, `Manager.Encrypt`, `Manager.Decrypt`。
- Encryption AAD: `keyID + tenantID + "ai-provider"`；派生上下文为 `mochat-go/ai-provider-credentials/v1`。

- [ ] **Step 1: 写加密 RED 测试**

覆盖正确租户可解密、跨租户认证失败、密文不含 API Key、旧 key-ring 可解密、新活动 key 写入、无密钥禁止加密。

- [ ] **Step 2: 运行 RED**

Run: `go test ./internal/aiproviderconfig -count=1`

Expected: FAIL，包或类型尚不存在。

- [ ] **Step 3: 实现最小加密领域**

核心合同：

```go
type StoredConfig struct {
    TenantID int64
    ProviderCode, BaseURL, Model string
    APIKeyCiphertext, APIKeyKeyID, APIKeyHint string
    EffectiveAt, ExpiresAt time.Time
    Status string
    Version int64
}

type Cipher interface {
    Encrypt(tenantID int64, apiKey string) (ciphertext, keyID string, err error)
    Decrypt(tenantID int64, keyID, ciphertext string) (string, error)
}
```

- [ ] **Step 4: 增加 0164 迁移**

创建 `mochat_go_saas_tenant_ai_providers`，以 `tenant_id` 唯一，包含密文、key ID、hint、有效期、状态、版本及审计字段；down 只删除此新表。

- [ ] **Step 5: 运行 GREEN 与迁移合同**

Run: `go test ./internal/aiproviderconfig ./internal/migration -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```text
feat(ai): 增加租户模型凭证加密存储
```

### Task 2: SaaS 租户 Provider 仓储、API 与权限

**Files:**
- Create: `internal/store/saas_tenant_ai_provider.go`
- Create: `internal/store/saas_tenant_ai_provider_test.go`
- Create: `internal/dashboard/saas_tenant_ai_provider.go`
- Create: `internal/dashboard/saas_tenant_ai_provider_test.go`
- Modify: `internal/dashboard/saas_admin_access.go`
- Modify: `internal/dashboard/saas_admin_access_test.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Modify: `cmd/mochat-go/main.go`
- Modify: `internal/store/mysql.go`

**Interfaces:**
- Produces GET `/dashboard/saasAdmin/tenantAIProvider?tenantId=` and PUT `/dashboard/saasAdmin/tenantAIProvider`。
- GET permission: `platform.integrations.read`；PUT permission: `platform.integrations.manage`。
- Store write uses optimistic `version`; API Key empty on update means preserve ciphertext。

- [ ] **Step 1: 写仓储 RED 测试**

断言 tenant scope、首次创建、留空保持 Key、替换 Key、版本冲突、加密失败、读取不暴露密文。

- [ ] **Step 2: 写 Handler/路由/权限 RED 测试**

断言 read/manage 权限边界、跨租户参数拒绝、HTTPS/厂商/模型/有效期校验、API Key 永不回显、审计 JSON 不含 Key。

- [ ] **Step 3: 运行 RED**

Run: `go test ./internal/store ./internal/dashboard ./internal/server -run 'TenantAIProvider|SaaSAdminAccess' -count=1`

Expected: FAIL，接口和实现尚不存在。

- [ ] **Step 4: 实现仓储和 Handler**

PUT 输入：

```go
type SaaSTenantAIProviderInput struct {
    TenantID int64 `json:"tenantId"`
    ProviderCode string `json:"providerCode"`
    BaseURL string `json:"baseUrl"`
    Model string `json:"model"`
    APIKey string `json:"apiKey"`
    EffectiveAt string `json:"effectiveAt"`
    ExpiresAt string `json:"expiresAt"`
    Status string `json:"status"`
    Version int64 `json:"version"`
}
```

响应只包含 `apiKeyConfigured`、`apiKeyHint`、`credentialProtection` 等脱敏字段。

- [ ] **Step 5: 接线服务器与主程序**

MySQLStore 注入 AI 凭证 Manager；注册 GET/PUT handler；访问守卫按 integrations read/manage 分类。

- [ ] **Step 6: 运行 GREEN**

Run: `go test ./internal/store ./internal/dashboard ./internal/server ./cmd/mochat-go -count=1`

Expected: PASS。

- [ ] **Step 7: 提交**

```text
feat(saas): 管理租户 AI Provider 配置
```

### Task 3: 租户 Provider Resolver 与运行时失败关闭

**Files:**
- Create: `internal/aiproviderconfig/resolver.go`
- Create: `internal/aiproviderconfig/resolver_test.go`
- Modify: `internal/modules/providers/providers.go`
- Modify: `internal/modules/ai-insight/conversation_runner.go`
- Modify: `internal/modules/ai-insight/conversation_runner_test.go`
- Modify: `internal/modules/ai-insight/daily.go`
- Modify: `internal/modules/ai-insight/daily_test.go`
- Modify: `internal/modules/ai-insight/module.go`
- Modify: `internal/app/bootstrap/ai_insight.go`
- Modify: `cmd/mochat-go/ai_debt_clearance.go`

**Interfaces:**
- Produces `providers.AIProviderResolver.Resolve(context.Context, tenantID, corpID) (AIProvider, error)`。
- Resolver validates active/effective/expiry before decryption and returns stable safe error codes。

- [ ] **Step 1: 写 Resolver RED 测试**

覆盖不同租户返回不同模型/Key、缺失/未生效/过期/停用/解密失败不创建客户端、无全局回退、错误文本不含密文或 Key。

- [ ] **Step 2: 写 Runner RED 测试**

断言每个企业只解析一次 Provider、租户 A/B 不串用、Resolver 失败生成不可用 run 且不 Chat。

- [ ] **Step 3: 运行 RED**

Run: `go test ./internal/aiproviderconfig ./internal/modules/ai-insight ./cmd/mochat-go -count=1`

Expected: FAIL。

- [ ] **Step 4: 实现 Resolver 和运行时接线**

保留静态 Resolver 仅供单元测试；生产使用数据库配置 Resolver。删除生产环境变量 Provider 回退路径；配置存在不代表调度开关开启。

- [ ] **Step 5: 运行 GREEN**

Run: `go test ./internal/aiproviderconfig ./internal/modules/ai-insight ./internal/app/bootstrap ./cmd/mochat-go -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```text
feat(ai): 按租户解析有效模型配置
```

### Task 4: 统一日快照、上一日上下文并删除旧表

**Files:**
- Create: `deploy/standalone/migrations/0165_ai_daily_insight_unification.up.sql`
- Create: `deploy/standalone/migrations/0165_ai_daily_insight_unification.down.sql`
- Modify: `internal/migration/migration_test.go`
- Modify: `internal/modules/ai-insight/contracts.go`
- Modify: `internal/modules/ai-insight/repository.go`
- Modify: `internal/modules/ai-insight/repository_test.go`
- Modify: `internal/modules/ai-insight/repository_integration_test.go`
- Modify: `internal/modules/ai-insight/conversation_runner.go`
- Modify: `internal/modules/ai-insight/conversation_runner_test.go`
- Modify: `internal/modules/ai-insight/daily.go`
- Delete: `internal/modules/ai-insight/transport/http/analysis_store.go`
- Modify: `internal/modules/ai-insight/transport/http/handler.go`
- Modify/Delete: corresponding legacy analysis store and daily prompt tests
- Modify: `cmd/mochat-ai-insight-run/main.go`

**Interfaces:**
- Repository produces `PreviousInsightContext(ctx, tenantID, corpID, type, ruleVersionID, conversationKey, beforeDate)`。
- Saved insight carries `AnalysisDate`, `PreviousInsightID`, `PreviousScore`, `PreviousSummary`, `PreviousGeneratedAt`。

- [ ] **Step 1: 写 Asia/Shanghai 当日窗口 RED 测试**

固定 now 为 2026-08-25 10:30 Asia/Shanghai，断言窗口为 2026-08-25 00:00 至 10:30，规则 `lookback_days=30` 不能扩张窗口。

- [ ] **Step 2: 写上一日上下文 RED 测试**

断言同会话/类型/规则上一日成功记录被选中；同日、失败、其他租户、其他企业、其他规则均排除；无合法评分仍允许只传摘要。

- [ ] **Step 3: 写请求参数 RED 测试**

断言 prompt 含 previous generatedAt/score/summary 和不可作为证据约束；历史摘要中的伪消息 ID 不进入 allowed evidence；无历史时明确写无可用结果。

- [ ] **Step 4: 写迁移 RED 测试**

断言 0165 回填 `analysis_date`、按日去重、替换唯一键、增加历史快照字段、删除精确旧表，down 只重建空旧表并恢复旧唯一键合同。

- [ ] **Step 5: 运行 RED**

Run: `go test ./internal/modules/ai-insight ./internal/migration -count=1`

Expected: FAIL。

- [ ] **Step 6: 实现日窗口和历史上下文**

同日 Save 使用新唯一键更新；比较来源指纹时限定当前分析日期，避免昨日相同指纹阻止当日记录。历史上下文持久化实际发送快照。

- [ ] **Step 7: 删除旧链路**

删除 `mochat_go_ai_analysis` 的 store、读取、三页面逐页摘要调用。情绪、员工评分、沟通关键词只读 `mochat_go_ai_conversation_insights` 投影。

- [ ] **Step 8: 运行 GREEN 和真实 MariaDB integration**

Run: `go test ./internal/modules/ai-insight ./internal/migration -count=1`

Run: `MOCHAT_REQUIRE_MYSQL_INTEGRATION=1 go test -tags=integration ./internal/modules/ai-insight -count=1`

Expected: PASS；隔离 schema 中旧表不存在、同日 upsert 唯一、跨日历史可查。

- [ ] **Step 9: 提交**

```text
feat(ai): 统一当日洞察并引用上次结果
```

### Task 5: SaaS Admin React 租户配置界面

**Files:**
- Modify: `web/apps/saas-admin/src/lib/types.ts`
- Modify: `web/apps/saas-admin/src/pages/TenantsPage.tsx`
- Modify: `web/apps/saas-admin/src/pages/TenantsPage.test.tsx`
- Modify: `web/apps/saas-admin/src/index.css` only if existing utility classes cannot express responsive behavior

**Interfaces:**
- Query key: `['tenant-ai-provider', selectedTenantId]`。
- GET/PUT contracts match Task 2；API Key state is cleared on save, failure and dialog close。

- [ ] **Step 1: 写前端 RED 测试**

覆盖 read/manage 权限、加载状态、厂商预设、创建、编辑留空保留 Key、版本冲突、有效期客户端提示、成功后 Key 清空、Escape/关闭清空、390px 布局合同。

- [ ] **Step 2: 运行 RED**

Run: `corepack pnpm --filter @mochat/saas-admin test -- TenantsPage.test.tsx`

Expected: FAIL，界面和请求不存在。

- [ ] **Step 3: 实现最小界面**

在租户详情新增独立卡片；只读权限显示脱敏状态，manage 权限显示表单；保存不调用测试 Provider，不把 Key 放入 Query cache。

- [ ] **Step 4: 运行 GREEN、typecheck 和 build**

Run: `corepack pnpm --filter @mochat/saas-admin test`

Run: `corepack pnpm --filter @mochat/saas-admin typecheck`

Run: `corepack pnpm --filter @mochat/saas-admin build`

Expected: PASS。

- [ ] **Step 5: 提交**

```text
feat(saas): 增加租户 AI 模型配置界面
```

### Task 6: 完整验证、Docker、浏览器与报告

**Files:**
- Create: `docs/reviews/2026-08-25-saas-tenant-ai-provider-daily-context-report.zh-CN.md`

**Interfaces:**
- Produces final reproducible evidence and branch handoff only；不 push、不 merge。

- [ ] **Step 1: 验证迁移**

在隔离 MariaDB schema 验证 0164/0165 apply、重复执行、checksum、日去重、旧表删除和 down/up 边界；再在保留卷执行正常升级，不删除卷。

- [ ] **Step 2: 运行全量门禁**

```text
go test ./... -count=1
corepack pnpm --filter @mochat/saas-admin test
corepack pnpm --filter @mochat/saas-admin typecheck
corepack pnpm --filter @mochat/saas-admin build
corepack pnpm --filter @mochat/dashboard lint
corepack pnpm --filter @mochat/dashboard typecheck
corepack pnpm --filter @mochat/dashboard test
corepack pnpm --filter @mochat/dashboard build
corepack pnpm check:yuanhu-benchmark
corepack pnpm check:dashboard-all-pages-evidence
corepack pnpm check:phase4-dashboard-page-rbac
git diff --check
```

- [ ] **Step 3: 凭证扫描**

使用只输出文件名的通用 API Key 模式扫描；确认测试密钥均为显式 fixture，真实 Key 无命中。

- [ ] **Step 4: 保留卷 Docker 重建**

只重建 app；确认 app/MySQL/Redis healthy，healthz/readyz 200，日志无 migration/checksum/panic/fatal，AI 自动日分析和 run-on-start 仍关闭。

- [ ] **Step 5: 真实浏览器点击验收**

逐项点击租户详情、AI 配置加载、厂商切换、Key 输入、有效期、保存、编辑留空、启停、错误恢复、Escape、遮罩关闭；验证桌面、宽屏、390px 和小高度，无控制台 error/warning 与文档级横向溢出。使用测试 API Key 后立即替换/清除，不触发模型请求。

- [ ] **Step 6: 独立代码复审**

以设计文档、BASE SHA 和 HEAD SHA 请求独立 reviewer；修复所有 Critical/Important 后重跑相关门禁。

- [ ] **Step 7: 编写中文实施与自测报告并提交**

报告列出表删除边界、Provider 配置安全、当日窗口、历史上下文、接口、迁移、提交、自动化、Docker、浏览器证据、外部限制和分支状态，不含凭证。

```text
docs: 汇总租户 AI 配置与日分析验收
```

## 计划自审

- 设计中的配置、加密、失败关闭、日窗口、历史上下文、表统一、页面、权限、迁移、Docker 和浏览器验收均有对应任务。
- 所有接口、文件范围、关键类型、命令和预期结果已明确，没有未决占位项。
- `StoredConfig`、Resolver、SaaS API 和前端字段命名在各任务中一致；API Key 只存在于 PUT 请求瞬时内存和服务端加密入口。
