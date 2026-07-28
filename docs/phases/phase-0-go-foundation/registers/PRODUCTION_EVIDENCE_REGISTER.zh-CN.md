# 生产证据台账

> 来源：`docs/phases/phase-pre0-standalone/evidence/production/readiness.json`，2026-07-18 生成。当前 `ready=false`，本地 smoke 不得替代以下证据。

| ID | 证据 | 当前状态 | 缺少内容 | 标准产物 |
| --- | --- | --- | --- | --- |
| E-001 | MySQL 5.7 amd64 真实容器门禁 | 未完成 | amd64 CI artifact/log | `docs/phases/phase-pre0-standalone/evidence/production/mysql57-amd64.log` |
| E-002 | 真实企业微信账号联调 | 未完成 | token、回调、应用消息和日志引用 | `docs/phases/phase-pre0-standalone/evidence/production/real-wecom.md` |
| E-003 | 真实微信开放平台联调 | 未完成 | ticket、授权、取消授权、回调和日志 | `docs/phases/phase-pre0-standalone/evidence/production/real-wechat-open.md` |
| E-004 | 双真实 SaaS 租户回归 | 未完成 | 双 token、互访禁止、额度、账本、异步、告警 | `docs/phases/phase-pre0-standalone/evidence/production/real-saas-tenants.md` |
| E-005 | 生产前端浏览器回归 | 未完成 | 生产 base URL 与必要认证状态 | `docs/phases/phase-pre0-standalone/evidence/production/prod-frontend.md` |
| E-006 | 目标环境稳定性 | 未完成 | 外部监控、健康、资源、时间范围和日志 | `docs/phases/phase-pre0-standalone/evidence/production/stability.md` |

只有标准产物存在、校验脚本通过且证据来自真实目标环境时才能更新为完成。
