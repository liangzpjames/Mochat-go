# MoChat Go 生产证据采集诊断

- 生成时间：`2026-07-18 11:45:42 CST`
- 生产证据目录：`docs/evidence/production`
- 待填写环境变量清单：`docs/evidence/production/readiness.env.todo`
- 执行采集/导入：`False`
- 刷新已存在证据：`False`
- 严格退出：`False`
- 24 小时持续运行：`未启动`
- 标准证据文件齐备：`False`
- 标准证据文件有效：`False`
- 采集环境变量齐备或有效文件已存在：`False`
- 严格生产证据文件门禁：`未通过`

## 证据项

| 证据项 | 标准文件 | 文件状态 | 采集准备 | 本轮动作 | 下一步 |
| --- | --- | --- | --- | --- | --- |
| MySQL 5.7 amd64 真实容器门禁 | `docs/evidence/production/mysql57-amd64.log` | 缺失 | 缺少 `MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE` | 未执行 | 设置 MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE 指向 amd64 CI artifact/log 后运行本脚本或 import_mysql57_amd64_evidence.sh。 |
| 真实企业微信账号联调 | `docs/evidence/production/real-wecom.md` | 缺失 | 缺少 `MOCHAT_REAL_WECOM_BASE_URL`, `MOCHAT_REAL_WECOM_TOKEN`, `MOCHAT_REAL_WECOM_CALLBACK_EVIDENCE`, `MOCHAT_REAL_WECOM_AGENT_MESSAGE_EVIDENCE`, `MOCHAT_REAL_WECOM_LOG_REF` | 未执行 | 准备真实企微 dashboard token、回调解密记录、应用消息记录和日志/工单引用。 |
| 真实微信开放平台联调 | `docs/evidence/production/real-wechat-open.md` | 缺失 | 缺少 `MOCHAT_REAL_WECHAT_OPEN_BASE_URL`, `MOCHAT_REAL_WECHAT_OPEN_TOKEN`, `MOCHAT_REAL_WECHAT_OPEN_TICKET_EVIDENCE`, `MOCHAT_REAL_WECHAT_OPEN_AUTH_REDIRECT_EVIDENCE`, `MOCHAT_REAL_WECHAT_OPEN_CANCEL_EVIDENCE`, `MOCHAT_REAL_WECHAT_OPEN_MESSAGE_CALLBACK_EVIDENCE`, `MOCHAT_REAL_WECHAT_OPEN_LOG_REF` | 未执行 | 准备真实开放平台 ticket、预授权/授权回跳、取消授权、消息回调和日志/工单引用。 |
| 真实 SaaS 多租户数据回归 | `docs/evidence/production/real-saas-tenants.md` | 缺失 | 缺少 `MOCHAT_REAL_SAAS_BASE_URL`, `MOCHAT_REAL_SAAS_TENANT_A_TOKEN`, `MOCHAT_REAL_SAAS_TENANT_B_TOKEN`, `MOCHAT_REAL_SAAS_TENANT_A_MARKER`, `MOCHAT_REAL_SAAS_TENANT_B_MARKER`, `MOCHAT_REAL_SAAS_TENANT_A_FORBIDDEN_PATHS`, `MOCHAT_REAL_SAAS_TENANT_B_FORBIDDEN_PATHS`, `MOCHAT_REAL_SAAS_QUOTA_EVIDENCE`, `MOCHAT_REAL_SAAS_UPLOAD_LEDGER_EVIDENCE`, `MOCHAT_REAL_SAAS_ASYNC_EVIDENCE`, `MOCHAT_REAL_SAAS_ALERT_EVIDENCE` | 未执行 | 准备两个真实租户 token、租户标识、互访禁止路径、额度/账本/异步/告警证据。 |
| 生产前端浏览器回归 | `docs/evidence/production/prod-frontend.md` | 缺失 | 缺少 `MOCHAT_PROD_FRONTEND_BASE_URL` | 未执行 | 设置生产 base URL；需要登录时先准备 MOCHAT_PROD_FRONTEND_AUTH_STATE。 |
| 稳定性记录 | `docs/evidence/production/stability.md` | 缺失 | 缺少 `MOCHAT_STABILITY_MONITOR_EVIDENCE`, `MOCHAT_STABILITY_HEALTH_EVIDENCE`, `MOCHAT_STABILITY_RESOURCE_EVIDENCE`, `MOCHAT_STABILITY_TIME_RANGE`, `MOCHAT_STABILITY_LOG_REF` | 未执行 | 提供短稳/外部监控、健康检查和资源记录；legacy soak.ndjson 仍可导入。本脚本不会启动新的 24 小时运行。 |

## 严格证据门禁

- 报告：`docs/evidence/production/readiness-production-evidence.md`
- JSON：`docs/evidence/production/readiness-production-evidence.json`
- 日志：`docs/evidence/production/readiness-production-evidence.log`
- 返回码：`1`

## 采集命令

默认仅诊断：

```bash
./scripts/production_evidence_doctor.sh
```

环境变量齐备后执行采集/导入并复核：

先复制 `docs/evidence/production/readiness.env.todo` 为 `docs/evidence/production/readiness.env.local` 并填写，再执行预检：

```bash
MOCHAT_PRODUCTION_EVIDENCE_ENV_FILE=docs/evidence/production/readiness.env.local ./scripts/production_evidence_env_preflight.sh
```

预检通过后加载变量并采集：

```bash
set -a
. docs/evidence/production/readiness.env.local
set +a
MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1 ./scripts/production_evidence_doctor.sh
```

全部证据齐备后进入生产候选：

```bash
./scripts/collect_production_evidence_pack.sh
```
