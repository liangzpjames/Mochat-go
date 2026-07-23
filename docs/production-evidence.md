# MoChat Go 生产证据检查

- 生成时间：`2026-07-15 10:15:39 CST`
- 当前结论：**仍缺生产证据，不能标记最终完成**
- 严格模式：`False`
- 跳过快速本地门禁：`False`
- 当前源码指纹：`026b3cef88d869a0e53ac6ce6c0c493d69d2102b981106808ec520ee1a084102`

## 当前迁移清单

- 路由：`224`
- 表：`70`
- crontab：`9`
- 事件处理器：`10`
- 异步队列注解：`15`

## 快速本地门禁

- 通过 `standalone independence`：standalone independence audit passed
- 通过 `independent package smoke`：independent package smoke passed
- 通过 `acceptance suite coverage`：acceptance suite coverage audit passed: smoke_scripts=96 referenced=96
- 通过 `manifest route smoke coverage`：manifest route smoke coverage audit passed: manifest_routes=224 directly_covered=224 smoke_scripts=96
- 通过 `functional module matrix`：functional module matrix audit passed: modules=29 manifest_unique_paths=213/213 manifest_route_entries=224 runtime_unique_paths=587/587 runtime_route_entries=729
- 通过 `queue annotation coverage`：queue annotation coverage source=embedded:internal/server/compat_manifest_embedded.json / queue annotation coverage passed
- 通过 `wework callback event coverage`：wework callback event coverage audit passed: process_events=20 smoke_events=20 unit_events=18
- 通过 `worker SaaS usage assertions`：worker SaaS usage assertion audit passed
- 通过 `SaaS metric coverage`：SaaS metric coverage audit passed: metrics=26
- 通过 `SaaS storage reclaim coverage`：SaaS storage reclaim coverage audit passed: functions=26 groups=14

## 生产证据文件

- 缺失 `MySQL 5.7 amd64 真实容器门禁`：设置 `MOCHAT_EVIDENCE_MYSQL57_AMD64` 指向证据文件。在 amd64/x86_64 CI 执行 `env -u GOROOT ./scripts/ci_mysql57_amd64.sh` 的日志或报告。
- 缺失 `真实企业微信账号联调`：设置 `MOCHAT_EVIDENCE_REAL_WECOM` 指向证据文件。真实企业微信账号下授权、回调解密、通讯录、客户、客户群、标签、素材和企微应用链路验收记录。
- 缺失 `真实微信开放平台联调`：设置 `MOCHAT_EVIDENCE_REAL_WECHAT_OPEN` 指向证据文件。真实微信开放平台第三方平台 ticket、预授权、授权回跳、公众号资料回填、取消授权和消息回调验收记录。
- 缺失 `真实 SaaS 多租户数据回归`：设置 `MOCHAT_EVIDENCE_REAL_SAAS_TENANTS` 指向证据文件。两个以上真实租户业务数据下菜单权限、企业归属、资源额度、上传账本、异步任务和告警隔离验收记录。
- 缺失 `生产前端浏览器回归`：设置 `MOCHAT_EVIDENCE_PROD_FRONTEND` 指向证据文件。dashboard/sidebar/operation 生产构建产物的真实浏览器业务路径和异常路径回归记录。
- 缺失 `稳定性记录`：设置 `MOCHAT_EVIDENCE_STABILITY` 指向证据文件。目标部署环境短稳回归、健康检查或外部监控记录；legacy 场景可指向 `soak.ndjson`。

## 使用方式

- 默认模式只报告缺口并返回 0，适合阶段交接。
- 上线前设置 `MOCHAT_PRODUCTION_EVIDENCE_STRICT=1`，缺任一生产证据或本地门禁失败都会返回非 0。
- 仅自检证据文件规则时可设置 `MOCHAT_PRODUCTION_EVIDENCE_SKIP_QUICK=1`；生产候选门禁不要跳过快速本地门禁。
- 可设置 `MOCHAT_PRODUCTION_EVIDENCE_MIN_BYTES` 调整证据文件最小字节数，默认 `32`。
- 可设置 `MOCHAT_PRODUCTION_EVIDENCE_OUT=docs/production-evidence.md` 写入 Markdown 报告。
- 可设置 `MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT=docs/production-evidence.json` 同步写入不含完整命令输出和密钥的机器可读 JSON。
- 生产证据文件必须脱敏；严格检查会拒绝疑似原始 Authorization、Bearer、access_token、corpsecret、password 等敏感值。
- 每个生产证据文件必须记录当前 `scripts/source_fingerprint.py` 生成的源码指纹；指纹不匹配会被判定为过期证据。
