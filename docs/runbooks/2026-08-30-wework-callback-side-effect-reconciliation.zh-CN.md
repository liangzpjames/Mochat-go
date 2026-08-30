# 企业微信 callback 副作用 unknown 恢复运维说明

## 适用范围

本说明只处理 callback durable worker 已把外发动作标记为 `unknown`、但无法证明企业微信是否收到请求的场景。恢复接口不会调用企业微信，也不会自动猜测请求结果。列表、详情和恢复命令均使用当前 Dashboard principal 的 tenant/corp，要求 `dashboard.company_setting.website` 权限；请求体不能指定租户、企业或操作人。

本地 fake、MariaDB/MySQL 契约测试只能证明状态机、事务、幂等和租户隔离，不能证明真实企业微信已发送、未发送或支持 exactly-once。

## 排查和证据要求

1. 在 `GET /dashboard/company/callback-side-effects?status=unknown` 找到目标事件和动作，并用详情接口复核同事件另一动作的状态、action version、inbox lease fence 与隔离截止时间。
2. 在企业微信管理后台、受控网关日志或企业微信工单中核查结果。禁止把 MoChat 超时、连接断开或本地日志缺失当作“未发送”证据。
3. 记录不含 token、secret、callback 正文或客户消息正文的证据引用。`reason` 应写明判断依据；`confirm_sent` 的 `evidenceKind` 只能为 `provider_message_id`、`provider_delivery_query_sent`、`provider_support_confirmed_sent`，`confirm_not_sent_and_retry` 只能为 `provider_delivery_query_absent`、`provider_request_rejected`、`provider_support_confirmed_not_sent`；`evidenceRef` 写工单号、回执号或受控日志索引。
4. 每次命令生成 16–128 字符的 `Idempotency-Key`。网络超时后必须用同一 key 和完全相同的请求重试；不得换 key 猜测首次请求是否提交。

## 两种决议

- `confirm_sent`：已有证据证明企业微信收到并完成外发。系统只把目标 action 标记为 `sent`，绝不调用 Provider。其他 `unknown` action 保持不变。
- `confirm_not_sent_and_retry`：已有证据证明企业微信未收到或明确未执行。系统只把目标 action 重置为 `pending`，随后由 durable worker 领取 inbox 并外发。此操作存在不可逆的重复发送风险：如果人工判断错误且 Provider 不支持幂等或回执查询，重试仍可能重复外发。

恢复命令要求提交详情页读到的 `expectedVersion` 与 `expectedInboxLeaseFence`。隔离期未结束、worker lease 仍有效、版本/fence 已变化、action 不再是 `unknown` 或存在未知 action 时，接口返回稳定的 409 错误，不得绕过后重试。跨 tenant/corp 的事件和当前 scope 不存在的事件统一返回 404。

## 审计、双 action 和故障恢复

action 更新、幂等 receipt、Dashboard 审计及最后一个 `unknown` 解决后的 inbox 复活在同一 MySQL 事务提交。任一步失败都会整体回滚。同一 key/同一 fingerprint 重放首次响应，不重复增加 version、审计或 inbox fence；同一 key/不同 fingerprint 返回 `IDEMPOTENCY_CONFLICT`。

同一 callback 的 `fission.employee_reminder` 与 `fission.customer_push` 独立推进。只要任一 action 仍为 `unknown`，inbox 不复活；最后一个 `unknown` 解决后，inbox 才回到 `pending`，attempt 清零并递增 lease fence，使旧 worker 无法写回。唤醒加速失败不影响正确性，worker 的数据库轮询仍可恢复处理。

## 回滚 0176 前置条件

回滚会删除命令 receipt 和恢复元数据。必须先停止 callback worker 与管理写入，确认没有 `status='unknown'` 或未完成恢复，导出 `mochat_go_wework_callback_side_effect_commands` 及对应 `mochat_go_dashboard_permission_audits`，完成数据库备份与校验，再执行 down。回滚后核验 0174 side-effect 表、复合主键、外键和 `pending/unknown/sent` 状态仍在。
