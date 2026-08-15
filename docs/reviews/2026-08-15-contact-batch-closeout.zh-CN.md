# Contact Batch 批次收口证据报告

> 日期：2026-08-15（Asia/Shanghai）
> 分支：`phase6/provider-foundation-wecom-closeout`
> 本批范围：客户精准群发（`contact_batch_send`）durable 闭环收口；不含 `room_batch_send`。
> 提交：`72ed1976`（contact batch closeout 主提交）＋ 后续补丁（P1-9 请求构建器测试、本证据文档）
> 基线：`a406110`（HEAD 前身）／`214d95b`（main）；相对 main：0 behind / 67 ahead（含主提交）

## 1. 交付摘要（按交接文档 §18 模板）

- 批次：contact batch（P0-1 / P0-2 / P0-3 / P0-4 / P0-5 / P1 审阅 / 独立提交）
- 分支：`phase6/provider-foundation-wecom-closeout`
- 主提交 SHA：`72ed19760b1b516508a28651b7a84a98db92fa23`
- 与基线范围：`a406110..72ed1976`（24 文件，+5602/-69）
- MariaDB：真实临时 schema 集成测试 **11/11 PASS**，`mochat_contact_batch_%` leftovers=0（独立查询确认）
- Dashboard：typecheck PASS、lint PASS、content-reach-pages **15/15 PASS**（含 390px）；build 因环境写保护阻塞（见 §6）
- 门禁：provider completion PASS；phase4 RBAC catalog/completion PASS（脚本包装层仅因裸 `pnpm` 版本 11.7.0≠11.17.0 报错，组件检查直接运行全绿）；`git diff --check` PASS
- 外部 live：未调用（全部为 fake HTTP / httptest 合同）
- Docker/服务器：未操作
- 重叠文件：`cmd/mochat-go/main.go`（双 cron 接线）、`internal/dashboard/room_welcome_wecom.go`、`internal/store/company_profile.go`、`internal/store/dashboard_tenant_gate.go`、`internal/store/mysql.go`、`web/.../content-reach-pages.{tsx,test.tsx}`
- 工作树：clean（提交后仅 3 个待清理日志文件）

## 2. RED 证据

| 项 | 证据 |
|---|---|
| P0-1 | `contact_batch_dispatch_p01_red_test.go` 编译失败：`ContactMessageBatchSendWrite has no field or method BatchTitle`；集成 harness 中 medium_id 回读断言（修复前 `medium_id=0`） |
| P0-3 | `TestContactBatchHTTPChainSubmitErrorClassification` 失败：client 实际输出 `WECOM_HTTP_ERROR_429/500/401/403`，原分类器匹配 `HTTP_429/HTTP_5` 等子串 → 全部误判 contract（终态） |
| P0-5 | lint 2 错误（unbound-method / no-unsafe-assignment）；Vitest 5 失败（selectedOptions 代理 set、异步选项未等待、ResizeObserver 缺失） |

## 3. GREEN 证据（全部新鲜命令）

```powershell
# 定向 Go（dashboard+store+wecomcapability）
go test ./internal/dashboard ./internal/store ./internal/wecomcapability -run 'TestContactBatch|TestContactMessageBatch|TestWorkAgentMessageRequest|TestDispatch|TestCapability|TestWeComCapability|TestContactBatchReminder' -count=1   # ok ×3
# P0-3 fake HTTP 全链路 6/6 + P0-1 单元
go test ./internal/dashboard -run 'TestContactBatchHTTPChain|TestContactBatchDispatchInputPreservesBatchTitleAndMediumID' -count=1   # ok
# P0-4 store fencing 5/5 + 纯函数 4/4
go test ./internal/store -run 'TestContactBatchReminder|TestContactBatchDispatchKey|TestContactBatchDispatchTargetRoundTrip|TestContactBatchStringChunks|TestContactBatchRequestIDRoundTrip' -count=1   # ok
# P1-9 请求构建器兼容
go test ./internal/dashboard -run 'TestContactBatchRequestBuilderContentTypeCompatibility' -count=1   # ok
# 真实临时 MariaDB（root 管理 DSN，仅测试进程内使用）
go test ./internal/store -run 'TestContactBatch' -count=1 -v   # 11 个集成场景 PASS；leftovers=0
# 前端
corepack pnpm --filter @mochat/dashboard typecheck   # PASS
corepack pnpm --filter @mochat/dashboard lint        # PASS
corepack pnpm --filter @mochat/dashboard test src/features/phase34/content-reach-pages.test.tsx  # 15/15 PASS
# 门禁
corepack pnpm check:provider-completion             # PASS
node scripts/check_dashboard_page_rbac_catalog.mjs  # PASS（0 unmapped）
node scripts/check_dashboard_page_rbac_completion.mjs # PASS
git diff --check                                    # PASS
```

## 4. 修复与缺陷记录

1. **P0-1**：durable（及 legacy）创建持久化 `batch_title`/`medium_id`，list/show 回读。五层：`ContactMessageBatchSendWrite`/`Item` 加字段、`contactBatchDispatchInputFromBody` 传递、`insertContactBatchBusinessRowTx` INSERT 补列、5 处 SELECT+scan 补列、`batchListPayload`/`batchShowPayload` 输出；legacy `CreateContactMessageBatchSend` INSERT 与 `batchWriteFromParams` 同步补齐。
2. **P0-3 真实 bug**：`classifyContactBatchProviderError` 与 client 错误格式（`WECOM_HTTP_ERROR_<code>`/`WECOM_API_ERROR_<code>`）不匹配，429/5xx/401/403 被误分类为 contract（终态）——违反"模糊 5xx/429 进 reconcile 不重发"合同。已改为匹配真实格式。
3. **P0-5 测试技术**：`fireEvent.change(select,{target:{value}})` 在 jsdom 中绕过原生 setter 导致 `selectedOptions` 不变 → 改为直接设置 option.selected 后派发 change；补异步选项等待与 `ResizeObserver` polyfill。
4. **harness 修正**：非超管子测试 access identity 匹配；failed 终态过渡需 `LastErrorCode`；reminder 场景输入 2 名员工。

## 5. 集成测试覆盖（P0-2 十场景）

原子提交 / audit+event 故障整笔回滚（触发器）/ 幂等重放不重复写 / 零越权（跨租户、停用 actor、撤销权限、收窄 scope、目标归属变化）×5 / scheduled 到期前后 due / legacy cron 排除 durable / 多 dispatch 成功+失败聚合为 partial_failed / cancel 与 worker claim 竞争 fencing / cancel 安全态成功 / reminder 崩溃窗口→reconcile、完成幂等、部分失败→partial_failed 且再准备→reconcile。全部通过生产 `MySQLStore` + 真实 0139 迁移，leftovers=0。

## 6. 未完成 / 受限（如实标注）

1. **Dashboard build**：`dist/index.html` unlink 被环境写保护拒绝（EPERM）；typecheck/lint/test 已绿，build 属纯环境阻塞，非代码问题。
2. **gofmt -w 5 个文件**（cosmetic）：`contact_batch_dispatch.go`、`contact_batch_dispatch_integration_test.go`、`contact_batch_dispatch_reminder_test.go`、`contact_batch_dispatch_helpers_test.go`、`contact_batch_dispatch_http_chain_test.go` 未格式化；不影响编译/测试。
3. **既有基线失败（非本批引入，文件未被修改）**：
   - `web/apps/dashboard/src/features/phase35/report-query.test.ts`：UTC 序列化 `Z` vs `+00:00`（Node 环境差异）。
   - `internal/dashboard/saas_compliance_export_deletion_test.go`：fixture `ExpiresAt=2026-08-14` 与会话日期 2026-08-15 的日期边界（自 phase 0 基线 `8dde40e` 未修改）。
4. **独立只读审阅**：待主模型（用户）执行；Critical/Important 关闭后才进入 room batch（阶段 B）。
5. **Docker/服务器部署**：未操作（未授权）。

## 7. 提交与恢复记录

`git commit` 实际成功（对象与 reflog 存在），但分支 ref 文件一度丢失（`refs/heads/phase6/provider-foundation-wecom-closeout` 消失，git 报 "No commits yet"）；已通过手动重建 ref 文件恢复至 `72ed1976`。若再次发生，按 reflog（`.git/logs/refs/heads/phase6/provider-foundation-wecom-closeout`）与 commit 对象恢复即可，工作内容无丢失。
