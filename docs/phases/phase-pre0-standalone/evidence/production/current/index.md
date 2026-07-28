# MoChat Go 生产证据包

- 生成时间：`2026-07-19 07:41:29 CST`
- 当前结论：**未通过，不能标记最终完成**
- 来源目录：`docs/evidence/production`
- 输出目录：`docs/evidence/production/current`
- 当前源码指纹：`ad95a7ba63dc2ec688355083c5cedbb9e81790ada25a4136ee01c01c06ae0e25`
- 复制证据文件：`True`
- 运行生产候选门禁：`True`
- 24 小时持续运行：`未启动`

## 证据文件

| 证据项 | 环境变量 | 文件 | 状态 | 大小 | sha256 |
| --- | --- | --- | --- | ---: | --- |
| MySQL 5.7 amd64 真实容器门禁 | `MOCHAT_EVIDENCE_MYSQL57_AMD64` | `docs/evidence/production/current/mysql57-amd64.log` | 缺失 | 0 | `` |
| 真实企业微信账号联调 | `MOCHAT_EVIDENCE_REAL_WECOM` | `docs/evidence/production/current/real-wecom.md` | 缺失 | 0 | `` |
| 真实微信开放平台联调 | `MOCHAT_EVIDENCE_REAL_WECHAT_OPEN` | `docs/evidence/production/current/real-wechat-open.md` | 缺失 | 0 | `` |
| 真实 SaaS 多租户数据回归 | `MOCHAT_EVIDENCE_REAL_SAAS_TENANTS` | `docs/evidence/production/current/real-saas-tenants.md` | 缺失 | 0 | `` |
| 生产前端浏览器回归 | `MOCHAT_EVIDENCE_PROD_FRONTEND` | `docs/evidence/production/current/prod-frontend.md` | 缺失 | 0 | `` |
| 稳定性记录 | `MOCHAT_EVIDENCE_STABILITY` | `docs/evidence/production/current/stability.md` | 缺失 | 0 | `` |

## 命令结果

- 失败 `严格生产证据文件检查`：`docs/evidence/production/current/production-evidence.log`
- 失败 `skip-local 生产候选门禁`：`docs/evidence/production/current/production-candidate.log`

## 关键结论

- 严格生产证据文件检查：**仍缺生产证据，不能标记最终完成**
- 机器可读生产证据检查：`production-evidence.json`
- skip-local 生产候选门禁：**未通过，不能标记最终完成**

## 复跑方式

```bash
set -a
. docs/evidence/production/current/env.production-evidence
set +a
MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 ./scripts/production_candidate_gate.sh
```
