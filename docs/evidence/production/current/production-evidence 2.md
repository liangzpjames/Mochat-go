# MoChat Go 生产证据检查

- 生成时间：`2026-07-08 17:58:36 CST`
- 当前结论：**仍缺生产证据，不能标记最终完成**
- 严格模式：`True`
- 跳过快速本地门禁：`True`

## 当前迁移清单

- 路由：`224`
- 表：`70`
- crontab：`9`
- 事件处理器：`10`
- 异步队列注解：`15`

## 快速本地门禁

- 已按 `MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK=1` 跳过，仅校验证据文件。

## 生产证据文件

- 无效 `MySQL 5.7 amd64 真实容器门禁`：`MOCHAT_EVIDENCE_MYSQL57_AMD64=docs/evidence/production/current/mysql57-amd64.log` 指向的文件不存在。在 amd64/x86_64 CI 执行 `env -u GOROOT ./scripts/ci_mysql57_amd64.sh` 的日志或报告。
- 无效 `真实企业微信账号联调`：`MOCHAT_EVIDENCE_REAL_WECOM=docs/evidence/production/current/real-wecom.md` 指向的文件不存在。真实企业微信账号下授权、回调解密、通讯录、客户、客户群、标签、素材和企微应用链路验收记录。
- 无效 `真实微信开放平台联调`：`MOCHAT_EVIDENCE_REAL_WECHAT_OPEN=docs/evidence/production/current/real-wechat-open.md` 指向的文件不存在。真实微信开放平台第三方平台 ticket、预授权、授权回跳、公众号资料回填、取消授权和消息回调验收记录。
- 无效 `真实 SaaS 多租户数据回归`：`MOCHAT_EVIDENCE_REAL_SAAS_TENANTS=docs/evidence/production/current/real-saas-tenants.md` 指向的文件不存在。两个以上真实租户业务数据下菜单权限、企业归属、资源额度、上传账本、异步任务和告警隔离验收记录。
- 无效 `生产前端浏览器回归`：`MOCHAT_EVIDENCE_PROD_FRONTEND=docs/evidence/production/current/prod-frontend.md` 指向的文件不存在。dashboard/sidebar/operation 生产构建产物的真实浏览器业务路径和异常路径回归记录。
- 无效 `持续运行稳定性记录`：`MOCHAT_EVIDENCE_STABILITY=docs/evidence/production/current/stability.md` 指向的文件不存在。目标部署环境持续运行记录；可指向 `soak.ndjson` 或阶段报告。

## 使用方式

- 默认模式只报告缺口并返回 0，适合阶段交接。
- 上线前设置 `MOCHAT_PRODUCTION_EVIDENCE_STRICT=1`，缺任一生产证据或本地门禁失败都会返回非 0。
- 仅自检证据文件规则时可设置 `MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK=1`；生产候选门禁不要跳过快速本地门禁。
- 可设置 `MOCHAT_PRODUCTION_EVIDENCE_MIN_BYTES` 调整证据文件最小字节数，默认 `32`。
- 可设置 `MOCHAT_PRODUCTION_EVIDENCE_OUT=docs/production-evidence.md` 写入 Markdown 报告。
- 生产证据文件必须脱敏；严格检查会拒绝疑似原始 Authorization、Bearer、access_token、corpsecret、password 等敏感值。
