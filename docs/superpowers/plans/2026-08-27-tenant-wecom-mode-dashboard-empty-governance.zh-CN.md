# 租户企微模式、Dashboard 空态与超级管理员治理实施计划

> 对应设计：`docs/superpowers/specs/2026-08-27-tenant-wecom-mode-dashboard-empty-governance-design.zh-CN.md`

## 目标

将企微模式变成开户时必选、后续不可切换的租户企业策略；自建配置归属 Dashboard，第三方配置归属 SaaS；未配置企微时 Dashboard 仍可浏览并显示空数据；修复 SaaS Dashboard 超级管理员治理的无效下拉、必失败动作和状态残留。

## 任务 1：迁移与领域合同

涉及文件：

- 新增 `deploy/standalone/migrations/0167_*.up.sql`、`0167_*.down.sql`
- 新增或修改 `internal/migration/*_test.go`
- 修改 `internal/dashboardadmin/service.go`
- 修改 `internal/dashboardadmin/service_test.go`

步骤：

1. 先写迁移合同测试，要求绑定表新增合法枚举、旧数据回填和安全回滚。
2. 先写开户输入测试，要求 `wecomIntegrationMode` 必填且只接受两种模式。
3. 运行目标测试确认失败。
4. 实施迁移和领域字段、常量、校验。
5. 运行目标测试确认通过。

## 任务 2：开户事务写入不可变模式

涉及文件：

- 修改 `internal/store/dashboard_admin_provisioning.go`
- 修改 `internal/store/dashboard_admin_provisioning_test.go`
- 修改 `internal/store/dashboard_admin_provisioning_integration_test.go`
- 修改 `internal/dashboardadmin/http_test.go`

步骤：

1. 添加模式缺失、非法模式、幂等模式冲突、事务回滚和审计不含 Secret 的失败测试。
2. 将模式纳入开户指纹，在绑定插入语句中原子写入。
3. 第三方模式同时建立无凭据的当前配置槽；自建模式不复制 SaaS Secret。
4. 将模式加入安全的开户审计事实。
5. 运行 store、service、HTTP 目标测试。

## 任务 3：企微配置所有权与模式不可变

涉及文件：

- 修改 `internal/dashboardadmin/wecom_integration.go`
- 修改 `internal/dashboardadmin/wecom_integration_test.go`
- 修改 `internal/store/saas_wecom_integration.go`
- 修改 `internal/store/saas_wecom_integration_test.go`
- 修改 `internal/store/company_profile.go`
- 修改 `internal/store/company_profile_*_test.go`
- 修改 `internal/server/server.go`
- 修改 `internal/dashboard/saas_admin_access.go`

步骤：

1. 写测试证明模式来自绑定而不是请求，且任何跨模式保存、切换或回滚都会被拒绝。
2. 将 SaaS 写接口收敛为第三方当前配置的安全写入/轮换；读接口在未配置时返回模式和空状态，而不是 `TARGET_NOT_FOUND`。
3. 自建模式只允许 Dashboard 企业资料写凭据；第三方模式禁止 Dashboard 自建凭据写入口。
4. 企微运行凭据解析按绑定模式选择唯一来源。
5. 删除或禁用候选、验证、切换、回滚的公开路由与权限资源，并保留稳定兼容错误。
6. 运行企微集成、企业资料、权限与路由测试。

## 任务 4：SaaS 开户和企微详情界面

涉及文件：

- 修改 `web/apps/saas-admin/src/pages/TenantsPage.tsx`
- 修改 `web/apps/saas-admin/src/pages/TenantsPage.test.tsx`
- 修改 `web/apps/saas-admin/src/lib/api.ts`
- 修改 `web/apps/saas-admin/src/lib/api.test.ts`

步骤：

1. 写组件测试：开户模式必选；确认摘要包含模式；模式成功提交；详情不可切换；自建无 Secret 表单；第三方允许空配置并可安全写入。
2. 修改开户表单和请求类型。
3. 将企微详情改成不可变模式卡片：自建显示 Dashboard 配置入口，第三方显示 SaaS 安全配置入口。
4. 移除“尚未绑定可用企业”文案和候选/验证/切换/回滚控件。
5. 保存失败时清理敏感字段但保留非敏感内容；响应不回显凭据。
6. 运行 SaaS Admin test、lint、typecheck、build。

## 任务 5：Dashboard 待配置访问与空数据

涉及文件：

- 修改 `internal/dashboard/dashboard_access_guard.go`
- 修改 `internal/dashboard/dashboard_access_guard_test.go`
- 修改 `web/apps/dashboard/src/app/access-loader.ts`
- 修改 `web/apps/dashboard/src/app/access-loader.test.ts`
- 修改相关查询 handler、module transport 和空态测试
- 修改 `web/apps/dashboard/src/features/company-settings/website-page.tsx`
- 修改 `web/apps/dashboard/src/features/company-settings/company-profile-api.ts`
- 修改对应组件/API 测试

步骤：

1. 写失败测试：pending 绑定的超级管理员可加载访问档案和授权 GET 页面；未绑定员工仍获得租户级查询范围；写操作按配置能力阻塞。
2. 删除前端 pending 强制跳转及“只允许企业资料”的菜单裁剪。
3. 调整后端守卫，让 pending 绑定继续执行正式 RBAC；保留租户停用和越界拒绝。
4. 对路由清单运行无数据请求合同，修复 403、501 和错误信封；查询返回类型稳定的空数组、零统计、空分页。
5. Dashboard 企业资料按模式显示自建编辑区或第三方只读状态。
6. 运行 Dashboard API、组件、访问控制和页面清单测试。

## 任务 6：超级管理员治理动作模型与界面

涉及文件：

- 修改 `internal/dashboardadmin/service.go`
- 修改 `internal/store/dashboard_admin_provisioning.go`
- 修改相关 Go 单元/集成测试
- 修改 `web/apps/saas-admin/src/pages/TenantsPage.tsx`
- 修改 `web/apps/saas-admin/src/pages/TenantsPage.test.tsx`

步骤：

1. 写动作矩阵失败测试：待激活、已激活、唯一超管、多超管、停用、恢复、替换候选和跨租户。
2. 服务端治理读模型计算 `availableActions` 与 `blockedReasons`，不返回敏感身份信息。
3. 前端按动作展示可执行对象和空态；唯一超管不出现可点击停用动作；无替换候选不显示无效选择器。
4. 将请求键改为租户、动作和目标维度；切换租户、关闭详情、成功和刷新后清理不再有效的状态。
5. 版本冲突刷新后保留仍合法选择，其他错误显示在对应动作区域。
6. 运行治理 Go 与 SaaS 组件测试。

## 任务 7：Docker、全页面与回归验收

涉及文件：

- 修改独立验收 compose 或启动配置
- 修改必要的本地验收 fixture/脚本及中文报告

步骤：

1. 运行 `git diff --check`、相关 Go 测试和 `go test ./... -count=1`。
2. 运行四前端 lint、typecheck、test、build。
3. 运行 Phase 4 RBAC、Provider completion、Dashboard 页面证据/圆弧 benchmark 和迁移合同；MariaDB integration DSN 缺失时明确记录 SKIP。
4. 使用独立 compose project 构建并迁移，验证健康检查和重启恢复。
5. 创建幂等、可清理、带“本地验收”标记的自建与第三方未配置租户，通过正式开户和激活流程生成状态。
6. 浏览器实际验收 SaaS 开户、第三方配置、治理动作矩阵，以及 Dashboard 全路由空数据、企业资料、刷新、控制台和网络；员工端页面补 390×844。
7. 自行代码审查并修复问题，更新中文验证报告，提交最终代码，保持验收 Docker 可访问。

