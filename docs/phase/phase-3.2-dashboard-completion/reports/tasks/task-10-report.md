# Phase 3.2 Task10 实施报告

基线：`0ad120f`

## 完成内容

- `/customer/tags` 已提供标签组浏览、搜索、新增和改名。
- 标签支持新增、改名、跨组移动、删除影响确认；删除响应返回 `affectedResourceCount`。
- 联系人标签关系支持同批绑定与解绑，并实时回读 `usageCount`。
- 所有新 mutation 使用 `version`、`Idempotency-Key` 和持久化请求指纹；同键异请求返回 409。
- tenant/corp 隔离及查看、新增、编辑、删除、联系人编辑 RBAC 已接入；跨企业资源按 404 隐藏。
- 同组标签或同企业标签组重名返回 422，版本冲突返回 409，不存在返回 404，越权返回 403。
- 新增可逆迁移 `0109_scrm_customer_tag_parity`，真实 MariaDB 已验证默认分组回填、索引、up/down 与原标签保留。
- 前端使用共享 Phase3.2 页面头、筛选条、统计卡、数据卡、表格和 `PageState`，筛选状态同步 URL。
- 审查修复已关闭旧 POST 绑定旁路；联系人页统一使用带 `version`、请求指纹幂等并返回 `version/usageCount` 的 PUT 合同，支持绑定与解绑。
- 标签 mutation 定向失效标签目录、联系人列表和联系人详情；初始分组与实际查询一致，浏览器前进/后退同步关键词和分组。
- 组名与标签名关键词均可检索；删除前调用服务端 preview 获取最新版本及影响数，删除响应再次回显准确清理数。
- `0109` 通过 generated active key 唯一索引保证活动标签组及组内活动标签并发唯一，MariaDB 1062 映射为 422。

## 验证

- `go test ./internal/modules/scrm/... ./internal/migration -count=1`：通过。
- `go test -tags=integration ./internal/modules/scrm/adapters/mysql ./internal/migration -run 'CustomerTag|OpportunityAndTag|ContactLifecycle|PublicPool|LeadParityMigration' -count=1`：通过，真实 MariaDB 未跳过。
- `pnpm --dir web/apps/dashboard exec vitest run src/features/scrm/tag-page.test.tsx src/features/scrm/scrm-api.test.ts`：2 files / 10 tests 通过。
- `pnpm --dir web/apps/dashboard run typecheck`：通过。
- `pnpm --dir web/apps/dashboard run build`：通过。
- `go vet ./internal/modules/scrm/... ./internal/migration/...`：通过。
- Dashboard 全量由控制器负责；本次收敛不重复运行长命令。
- 审查修复聚焦 Go application/HTTP：通过。
- 审查修复真实 MariaDB 并发标签组/标签测试：通过；`0109` up/down 隔离迁移测试：通过。
- 审查修复前端聚焦：4 files / 18 tests 通过；typecheck 通过；`git diff --check` 通过。
- 此前 Dashboard 全量基线：56 files / 366 tests 通过（本次按要求未重复运行）。

## 延后与风险

- 真实浏览器登录态证据按计划留到 Task11，不提前写入矩阵。
- 部署前必须执行 `0109_scrm_customer_tag_parity`。
- 两份用户文档修改保持未暂存、未提交。
