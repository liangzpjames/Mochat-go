# MoChat Go 生产证据采集环境预检

- 生成时间：`2026-07-18 11:45:42 CST`
- readiness JSON：`docs/evidence/production/readiness.json`
- env 文件：`docs/evidence/production/readiness.env.local`
- env 文件存在：`False`
- 严格退出：`False`
- 允许非生产地址：`False`
- 访问生产：`未访问`
- 24 小时持续运行：`未启动`
- 采集环境就绪项：`0/6`
- 有效标准证据项：`0/6`
- 采集变量就绪项：`0/6`
- 当前结论：**采集环境变量未齐备，不能执行生产采集**

## env 文件提示

- env 文件不存在：`docs/evidence/production/readiness.env.local`

## 证据项

| 证据项 | 状态 | 缺失变量 | 格式/文件问题 | 下一步 |
| --- | --- | --- | --- | --- |
| MySQL 5.7 amd64 真实容器门禁 | 未就绪 | `MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE` | - | 设置 MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE 指向 amd64 CI artifact/log 后运行本脚本或 import_mysql57_amd64_evidence.sh。 |
| 真实企业微信账号联调 | 未就绪 | `MOCHAT_REAL_WECOM_BASE_URL`, `MOCHAT_REAL_WECOM_TOKEN`, `MOCHAT_REAL_WECOM_CALLBACK_EVIDENCE`, `MOCHAT_REAL_WECOM_AGENT_MESSAGE_EVIDENCE`, `MOCHAT_REAL_WECOM_LOG_REF` | - | 准备真实企微 dashboard token、回调解密记录、应用消息记录和日志/工单引用。 |
| 真实微信开放平台联调 | 未就绪 | `MOCHAT_REAL_WECHAT_OPEN_BASE_URL`, `MOCHAT_REAL_WECHAT_OPEN_TOKEN`, `MOCHAT_REAL_WECHAT_OPEN_TICKET_EVIDENCE`, `MOCHAT_REAL_WECHAT_OPEN_AUTH_REDIRECT_EVIDENCE`, `MOCHAT_REAL_WECHAT_OPEN_CANCEL_EVIDENCE`, `MOCHAT_REAL_WECHAT_OPEN_MESSAGE_CALLBACK_EVIDENCE`, `MOCHAT_REAL_WECHAT_OPEN_LOG_REF` | - | 准备真实开放平台 ticket、预授权/授权回跳、取消授权、消息回调和日志/工单引用。 |
| 真实 SaaS 多租户数据回归 | 未就绪 | `MOCHAT_REAL_SAAS_BASE_URL`, `MOCHAT_REAL_SAAS_TENANT_A_TOKEN`, `MOCHAT_REAL_SAAS_TENANT_B_TOKEN`, `MOCHAT_REAL_SAAS_TENANT_A_MARKER`, `MOCHAT_REAL_SAAS_TENANT_B_MARKER`, `MOCHAT_REAL_SAAS_TENANT_A_FORBIDDEN_PATHS`, `MOCHAT_REAL_SAAS_TENANT_B_FORBIDDEN_PATHS`, `MOCHAT_REAL_SAAS_QUOTA_EVIDENCE`, `MOCHAT_REAL_SAAS_UPLOAD_LEDGER_EVIDENCE`, `MOCHAT_REAL_SAAS_ASYNC_EVIDENCE`, `MOCHAT_REAL_SAAS_ALERT_EVIDENCE` | - | 准备两个真实租户 token、租户标识、互访禁止路径、额度/账本/异步/告警证据。 |
| 生产前端浏览器回归 | 未就绪 | `MOCHAT_PROD_FRONTEND_BASE_URL` | - | 设置生产 base URL；需要登录时先准备 MOCHAT_PROD_FRONTEND_AUTH_STATE。 |
| 稳定性记录 | 未就绪 | `MOCHAT_STABILITY_MONITOR_EVIDENCE`, `MOCHAT_STABILITY_HEALTH_EVIDENCE`, `MOCHAT_STABILITY_RESOURCE_EVIDENCE`, `MOCHAT_STABILITY_TIME_RANGE`, `MOCHAT_STABILITY_LOG_REF` | - | 提供短稳/外部监控、健康检查和资源记录；legacy soak.ndjson 仍可导入。本脚本不会启动新的 24 小时运行。 |

## 后续命令

环境变量预检通过后，在受控终端加载本地 env 文件并执行采集：
如果 6 类标准生产证据文件已存在且有效，可直接跳过 env 采集，执行 `./scripts/collect_production_evidence_pack.sh`。

```bash
set -a
. docs/evidence/production/readiness.env.local
set +a
MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1 ./scripts/production_evidence_doctor.sh
./scripts/collect_production_evidence_pack.sh
```
