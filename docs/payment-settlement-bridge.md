# 支付结算 Bridge 协议

支付结算 Bridge 负责连接微信支付、支付宝或其他支付渠道，持有渠道证书和密钥，并把渠道账单转换为 MoChat Go 的中立结构。Go SaaS 服务不保存渠道密钥，也不直接依赖渠道 SDK。

## 请求

```http
GET /v1/payment-settlements?provider=wechat_pay&cursor=opaque-cursor&limit=100
Accept: application/json
Authorization: Bearer <internal-token>
```

- `provider`：必填，必须属于 `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS`。
- `cursor`：可为空，由 Bridge 定义和解析；Go 只按不透明字符串保存，最大 512 字符。
- `limit`：每页最多返回的结算批次数，Go 默认传 `100`。
- `Authorization`：配置 `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_TOKEN` 后发送。

Bridge 应在成功时返回 `2xx` 和 JSON；非 `2xx`、超时、超过 32 MiB、非法 JSON 或字段校验失败都会使本次运行失败，渠道游标不会推进。

## 响应

```json
{
  "provider": "wechat_pay",
  "nextCursor": "opaque-cursor-2",
  "hasMore": false,
  "batches": [
    {
      "batchNo": "",
      "provider": "wechat_pay",
      "providerSettlementNo": "SETTLEMENT-20260710",
      "periodStart": "2026-07-10 00:00:00",
      "periodEnd": "2026-07-11 00:00:00",
      "currency": "CNY",
      "remark": "渠道日结单",
      "entries": [
        {
          "lineNo": 1,
          "providerTransactionNo": "CHANNEL-TXN-1",
          "transactionType": "payment",
          "orderNo": "PAY-20260710-1",
          "providerOrderNo": "CHANNEL-ORDER-1",
          "amountCents": 10000,
          "feeCents": -60,
          "netAmountCents": 9940,
          "currency": "CNY",
          "occurredAt": "2026-07-10 08:00:00",
          "raw": {
            "sourceLine": "optional"
          }
        }
      ]
    }
  ]
}
```

退款明细使用 `transactionType=refund`，`amountCents` 必须为负数，并提供 `refundNo` 或 `providerRefundNo`。所有明细都必须满足 `netAmountCents = amountCents + feeCents`。

`hasMore=true` 时，`nextCursor` 必须非空且不同于请求游标。单次 Go 运行最多读取 20 页。`hasMore=false` 时仍可返回新的 `nextCursor`，作为下次增量起点。

## 幂等与失败

- Bridge 应保证同一渠道结算批次的 `providerSettlementNo` 稳定。
- Go 会对规范化批次 JSON 计算 SHA-256，并按 `provider + providerSettlementNo` 与 `provider + source_sha256` 双重幂等。
- 多页运行中后续页失败时，已导入批次保留，渠道游标保持运行前值；下次重拉会命中幂等，不重复记账。
- dry-run 会完整拉取并校验，但不导入批次，也不推进游标。
- 同一渠道只能有一个活动运行；并发触发返回 `409`。活动运行超过 15 分钟后可由新运行接管。

## 安全边界

- Bridge 应部署在内网或受保护的服务网络，并校验 Bearer token。
- 渠道 API 密钥、私钥、证书、下载地址签名和原始银行卡信息不得出现在响应、日志或 `raw` 字段中。
- Bridge token 通过环境变量或密钥管理系统注入，不写入仓库、数据库或总后台表单。
- 生产环境应对 Bridge 的成功率、延迟、非 `2xx` 和游标停滞建立监控；Go 侧失败会生成 `payment_settlement_sync_failed` 通知 outbox。

## 人工差异处置

- Bridge 只负责提供渠道账单，不参与人工解决、忽略或重新打开差异。
- 启用总后台高风险审批后，直接调用 `paymentSettlementResolve` 返回 `428`；财务人员必须以 `payment.settlement.resolve` 发起申请，并由两名不同复核人批准后执行。
- 合法状态迁移只有 `open -> resolved/ignored` 和 `resolved/ignored -> open`。匹配明细不能进入人工处理链路，已关闭批次也不能再修改差异。
- 申请使用一致性只读事务冻结完整差异条目和所属批次账务；执行时按批次、条目的固定锁序重新比较。版本字段未变化但金额、匹配结果或处理审计发生漂移时同样返回 `409`。
- 条目状态、批次差异计数、平台操作审计和审批效果标记在同一事务提交，失败不会留下部分处理结果。
