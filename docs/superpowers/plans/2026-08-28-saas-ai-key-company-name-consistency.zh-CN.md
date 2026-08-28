# SaaS AI Key 保存与客户公司名称一致性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复租户 AI Key 保存失败后被误清理的问题，并让 SaaS 客户主名称始终读取 Dashboard 权威绑定公司名。

**Architecture:** AI Key 只留在当前弹窗内存，失败保留、成功或关闭时销毁；SaaS Overview 保留租户身份名并新增权威绑定公司名。新开户默认公司名与客户名相同，既有真实公司数据不做覆盖。

**Tech Stack:** Go、MySQL 5.7/10.6、React 19、TypeScript、TanStack Query、Vitest、Docker Compose、浏览器验收。

## Global Constraints

- 所有审阅、设计、实施与验收文档使用中文。
- 每项生产代码必须先有能按预期失败的回归测试，再做最小实现。
- API Key 不得进入 Query cache、Mutation variables、日志、截图或验收产物。
- `tenantName` 保留 SaaS 身份语义；`companyName` 来自 Dashboard 权威唯一企业绑定，缺失时回退。
- 不批量覆盖真实企业名，不删除或重建 Docker 数据卷。
- 部署时只构建并 `--no-deps --force-recreate app`。

---

### Task 1: 保存失败后保留 API Key 草稿

**Files:**
- Modify: `web/apps/saas-admin/src/pages/TenantsPage.test.tsx`
- Modify: `web/apps/saas-admin/src/pages/TenantsPage.tsx`

**Interfaces:**
- Consumes: `TenantAIProviderForm`、`ApiError`、现有 `aiProviderMutation`。
- Produces: 失败可重试的弹窗内 Key 生命周期；成功/关闭/切换 Provider 后清除。

- [ ] **Step 1: 写入失败保留与重试的 RED 测试**

把现有“失败后清空”测试改为明确断言：第一次 PUT 返回 500 后输入框仍为 `fixture-retry-secret`，第二次点击保存发出相同 Key 并成功；同时保留成功、关闭、Esc、切换 Provider 后清空的断言。

```tsx
setValue('API Key', 'fixture-retry-secret')
mocks.apiRequest.mockImplementationOnce(async () => {
  throw new mocks.ApiError('配置暂时不可用', 500, 'API_REQUEST_FAILED')
})
clickButton('保存 AI 配置')
await settle()
expect((document.querySelector('input[placeholder="留空则保留现有密钥"]') as HTMLInputElement).value).toBe('fixture-retry-secret')
clickButton('保存 AI 配置')
await settle()
const retry = mocks.apiRequest.mock.calls.filter(([path, init]) => path === '/dashboard/saasAdmin/tenantAIProvider' && init?.method === 'PUT').at(-1)
expect(JSON.parse(String(retry?.[1]?.body)).apiKey).toBe('fixture-retry-secret')
```

- [ ] **Step 2: 运行目标测试并确认 RED**

Run: `pnpm --filter @mochat/saas-admin exec vitest run --config vitest.config.ts src/pages/TenantsPage.test.tsx`

Expected: FAIL，实际输入框值为 `''`，证明旧 `onError` 清理逻辑被测试命中。

- [ ] **Step 3: 最小实现失败保留策略**

删除普通 `onError` 和前端校验失败路径中的 `apiKey: ''`；409 刷新公开配置时使用刷新结果更新版本/公开字段，但保留当前 `apiKey`。将错误文案改为不再声称已经清理 Key；400/409 返回可操作消息，5xx 使用通用文案。

```tsx
setAIProviderForm((form) => ({
  ...providerFormFromData(refreshed.data.provider),
  apiKey: form.apiKey,
}))
```

- [ ] **Step 4: 运行目标与全量 SaaS Admin 测试**

Run: `pnpm --filter @mochat/saas-admin test`

Expected: `53+` tests passed，0 failed；新增失败重试测试通过。

- [ ] **Step 5: 提交独立修复**

```bash
git add web/apps/saas-admin/src/pages/TenantsPage.tsx web/apps/saas-admin/src/pages/TenantsPage.test.tsx
git commit -m "fix(saas): preserve AI key draft after save errors"
```

### Task 2: SaaS 展示 Dashboard 权威公司名

**Files:**
- Modify: `internal/dashboard/saas_admin.go`
- Modify: `internal/dashboard/saas_admin_test.go`
- Modify: `internal/store/mysql.go`
- Modify: `internal/store/saas_tenant_default_corp_test.go`
- Modify: `web/apps/saas-admin/src/lib/types.ts`
- Modify: `web/apps/saas-admin/src/pages/TenantsPage.tsx`
- Modify: `web/apps/saas-admin/src/pages/TenantsPage.test.tsx`

**Interfaces:**
- Produces: `SaaSAdminTenantOverview.CompanyName string` 与 JSON `companyName`。
- Consumes: `mochat_go_tenant_corp_bindings(tenant_id,corp_id)` 和 `mc_corp(tenant_id,id,name,deleted_at)`。

- [ ] **Step 1: 写 API 与页面 RED 测试**

在 handler fake store 中返回 `TenantName: "MoChat Test Enterprise", CompanyName: "蓝鲸数字科技（上海）有限公司"`，断言 Overview JSON 同时包含两个字段。前端 mock 返回同样数据，断言列表主名称和详情标题使用公司名，同时辅助文字保留 SaaS 租户别名。

- [ ] **Step 2: 写新开户默认公司名 RED 测试**

把 `TestSaaSTenantDefaultCorpValues` 的期望改成：

```go
if name != "示例客户" {
    t.Fatalf("name = %q, want 示例客户", name)
}
```

- [ ] **Step 3: 运行 RED 测试**

Run: `go test ./internal/dashboard ./internal/store -run 'TestSaaS(AdminOverview|TenantDefaultCorpValues)' -count=1`

Run: `pnpm --filter @mochat/saas-admin exec vitest run --config vitest.config.ts src/pages/TenantsPage.test.tsx`

Expected: FAIL，原因分别为缺少 `companyName`、页面仍显示 `tenantName`、默认公司仍追加“演示企业”。

- [ ] **Step 4: 实现后端公司名契约与查询**

为 `SaaSAdminTenantOverview` 增加 `CompanyName string`，payload 输出 `companyName`。`saasAdminTenants` 使用权威绑定左连接有效 `mc_corp`，扫描公司名并在空值时回退租户名；关键字同时匹配 `t.name` 与 `c.name`。把 `saasTenantDefaultCorpValues` 改为返回清理后的租户名和原 `fake_tenant_<id>`。

- [ ] **Step 5: 实现前端主名称选择**

为 `TenantSummary` 增加 `companyName: string`，使用以下纯选择规则贯穿过滤、列表、详情、治理摘要和 AI/企微弹窗：

```tsx
const tenantCompanyName = (tenant?: TenantSummary | null) => tenant?.companyName?.trim() || tenant?.tenantName?.trim() || ''
```

当 `companyName !== tenantName` 时，列表辅助信息展示 `SaaS 租户：<tenantName> · 租户 ID <id>`，避免丢失身份定位。

- [ ] **Step 6: 运行 GREEN 与静态门禁**

Run: `go test ./internal/dashboard ./internal/store -count=1`

Run: `pnpm --filter @mochat/saas-admin test`

Run: `pnpm --filter @mochat/saas-admin typecheck`

Expected: 全部 exit 0，0 failed。

- [ ] **Step 7: 提交独立修复**

```bash
git add internal/dashboard/saas_admin.go internal/dashboard/saas_admin_test.go internal/store/mysql.go internal/store/saas_tenant_default_corp_test.go web/apps/saas-admin/src/lib/types.ts web/apps/saas-admin/src/pages/TenantsPage.tsx web/apps/saas-admin/src/pages/TenantsPage.test.tsx
git commit -m "fix(saas): display bound dashboard company names"
```

### Task 3: 全量验证、Docker 交付与浏览器验收

**Files:**
- Create: `docs/reviews/2026-08-28-saas-ai-key-company-name-acceptance.zh-CN.md`

**Interfaces:**
- Consumes: Task 1/2 的提交与本地 `mochat-go-desktop` Compose 项目。
- Produces: 代码门禁、运行态 API、浏览器、数据卷与提交 SHA 证据。

- [ ] **Step 1: 执行完整代码门禁**

Run: `go test ./... -count=1`

Run: `pnpm --filter @mochat/saas-admin lint`

Run: `pnpm --filter @mochat/saas-admin build`

Run: `git diff --check`

Expected: 全部 exit 0，0 failed，0 whitespace errors。

- [ ] **Step 2: 记录部署前容器与卷**

记录 `docker ps`、`docker volume ls --filter label=com.docker.compose.project=mochat-go-desktop` 和 `/readyz`，不得输出任何 Secret 值。

- [ ] **Step 3: 仅重建并重建 app**

使用当前项目受控环境文件执行：

```powershell
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml build app
docker compose -p mochat-go-desktop -f deploy/standalone/docker-compose.yml up -d --no-deps --force-recreate app
```

- [ ] **Step 4: 运行态 API 与浏览器验收**

验证 `/readyz`、登录态 Overview API、SaaS 客户列表/详情和 Dashboard 唯一企业资料。租户 1 的 SaaS 客户主名称、Overview `companyName` 和 Dashboard 公司名必须都为 `蓝鲸数字科技（上海）有限公司`，同时 `tenantName` 仍为 `MoChat Test Enterprise`。

使用测试专用假 Key 触发可恢复失败，证明失败后输入仍保留；随后关闭弹窗，证明 DOM 中不再存在测试 Key。不得提交或记录真实 Key。

- [ ] **Step 5: 记录验收并提交**

把命令、计数、HTTP 状态、页面断言、容器 ID、卷清单和最终提交 SHA 写入中文验收文档，提交：

```bash
git add docs/reviews/2026-08-28-saas-ai-key-company-name-acceptance.zh-CN.md
git commit -m "docs(saas): record AI key and company name acceptance"
```
