# 批次四-A：企业微信客户精准群发 durable 闭环

## 范围

本批只收口 `contact_batch_send`，不实现 `room_batch_send`。旧客户群发页面继续保留读侧兼容，但新的创建与处理路径必须以 0139 operation/dispatch/result 为权威，不使用 `background_task`、旧 `send_status` 或旧 cron 的成功回写证明外部完成。

## 合同

1. HTTP 创建从认证 principal 和 `DashboardAccessContext` 取得 tenant/corp/actor/员工范围；请求体拒绝 tenant、actor 字段，拒绝跨 corp 员工和客户 external_userid，空或未证明归属的目标 fail closed。
2. Store 的 durable create 在一个 `sql.Tx` 中完成业务批次、contact operation、每个 chunk 的 dispatch、create audit/event；operation idempotency 与 chunk idempotency 重放只回读既有记录，不重复外发或审计。
3. Dispatch 使用 `DispatchKindContactBatch`，发送器只提交 contact payload；submitted/reconcile 只进入 poll，不再次 submit。发送侧 5xx/timeout/未明确未受理的 429 保守 reconcile，401/403/contract 终态失败。
4. 每次 claim 重新验证租户绑定、contact credential generation、SaaS/package/quota、页面 scope 与员工/客户归属；generation、租户、corp、lease/attempt 不匹配时零写。父状态是 dispatch/result 的读投影。
5. show/results/remind/delete 只读取同一 tenant/corp 与页面 scope；提醒/删除不覆盖 submitted/terminal durable 状态。API 使用稳定 machine code，前端消费 operation/status/results 并就地展示错误/空态。

## 验证批次

- RED：handler 拒绝 body 越权、空 scope、重复 idempotency；fake HTTP server 锁定请求字段和 msgid/错误分类。
- GREEN：事务 Store、DispatchRunner contact sender/poller adapter、结果幂等与多 chunk/partial/reconcile/fence 行为。
- 门禁：Go 定向测试、无 DSN 时明确 SKIP 的临时 schema 集成、Dashboard typecheck/lint/tests/build、provider/catalog gate 与 `git diff --check`。
