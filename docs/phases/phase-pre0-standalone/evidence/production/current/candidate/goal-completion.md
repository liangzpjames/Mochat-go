# MoChat Go 目标完成度审计

- 生成时间：`2026-07-19 07:41:28 CST`
- 当前结论：**目标未完成，继续保留生产证据缺口**
- 严格模式：`True`
- 是否要求本轮满 24 小时长稳证据：`False`
- 本地短证据包：`docs/evidence/latest`

## 目标拆解

- 通过 `能运行的独立 Go 项目`：standalone independence + independent package smoke + runtime route coverage
- 通过 `不依赖原 mochat/PHP 项目`：临时目录没有原 mochat/ 兄弟目录，MOCHAT_SOURCE_ROOT 和 MOCHAT_COMPAT_MANIFEST 指向不存在路径。
- 通过 `全部 manifest 功能已纳入 Go 本地验收`：PHP 源码清单对齐、manifest routes 224/224、smoke 直接覆盖、功能模块矩阵、docs/evidence/latest 全套短验收。
- 通过 `旧前端 dist API 全量承接`：扫描 dashboard/sidebar/operation 内置 dist 的静态 API 声明，并与 Go runtime dispatch/routes 对比。
- 通过 `SaaS、worker、cron、frontend 本地闭环`：docs/evidence/latest acceptance-saas/workers/cron/frontend + 前端 dist API 覆盖 + SaaS 指标和存储回收审计。
- 未完成 `生产外部证据`：MOCHAT_EVIDENCE_* 指向的真实证据文件。
  缺口：无效 `MySQL 5.7 amd64 真实容器门禁`：`MOCHAT_EVIDENCE_MYSQL57_AMD64=docs/evidence/production/current/mysql57-amd64.log` 指向的文件不存在。在 amd64/x86_64 CI 执行 `env -u GOROOT ./scripts/ci_mysql57_amd64.sh` 的日志或报告。；无效 `真实企业微信账号联调`：`MOCHAT_EVIDENCE_REAL_WECOM=docs/evidence/production/current/real-wecom.md` 指向的文件不存在。真实企业微信账号下授权、回调解密、通讯录、客户、客户群、标签、素材和企微应用链路验收记录。；无效 `真实微信开放平台联调`：`MOCHAT_EVIDENCE_REAL_WECHAT_OPEN=docs/evidence/production/current/real-wechat-open.md` 指向的文件不存在。真实微信开放平台第三方平台 ticket、预授权、授权回跳、公众号资料回填、取消授权和消息回调验收记录。；无效 `真实 SaaS 多租户数据回归`：`MOCHAT_EVIDENCE_REAL_SAAS_TENANTS=docs/evidence/production/current/real-saas-tenants.md` 指向的文件不存在。两个以上真实租户业务数据下菜单权限、企业归属、资源额度、上传账本、异步任务和告警隔离验收记录。；无效 `生产前端浏览器回归`：`MOCHAT_EVIDENCE_PROD_FRONTEND=docs/evidence/production/current/prod-frontend.md` 指向的文件不存在。dashboard/sidebar/operation 生产构建产物的真实浏览器业务路径和异常路径回归记录。；无效 `稳定性记录`：`MOCHAT_EVIDENCE_STABILITY=docs/evidence/production/current/stability.md` 指向的文件不存在。目标部署环境短稳回归、健康检查或外部监控记录；legacy 场景可指向 `soak.ndjson`。
- 通过 `短稳/外部监控稳定性证据（本轮不要求 24 小时）`：本轮不启动 24 小时持续运行；稳定性默认接受短稳回归、健康检查和外部监控记录。

## 当前门禁结果

- 通过 `standalone independence`：standalone independence audit passed
- 通过 `independent package smoke`：independent package smoke passed
- 通过 `PHP inventory parity`：standalone inventory parity passed
- 通过 `acceptance suite coverage`：acceptance suite coverage audit passed: smoke_scripts=107 referenced=107
- 通过 `manifest route smoke coverage`：manifest route smoke coverage audit passed: manifest_routes=224 directly_covered=224 smoke_scripts=107
- 通过 `functional module matrix`：functional module matrix audit passed: modules=29 manifest_unique_paths=213/213 manifest_route_entries=224 runtime_unique_paths=588/588 runtime_route_entries=730
- 通过 `frontend dist API coverage`：frontend dist API coverage audit passed: surfaces=3 endpoints=209 missing=0
- 通过 `queue annotation coverage`：queue annotation coverage source=embedded:internal/server/compat_manifest_embedded.json / queue annotation coverage passed
- 通过 `wework callback event coverage`：wework callback event coverage audit passed: process_events=20 smoke_events=20 unit_events=18
- 通过 `worker SaaS usage assertions`：worker SaaS usage assertion audit passed
- 通过 `SaaS metric coverage`：SaaS metric coverage audit passed: metrics=26
- 通过 `SaaS storage reclaim coverage`：SaaS storage reclaim coverage audit passed: functions=26 groups=14
- 通过 `runtime route coverage`：extra_route_total=209 / coverage_json=/var/folders/kb/j3ftznkj1sl14d0v300ymzzw0000gn/T/mochat-go-goal-audit.gs16ll93/route-coverage.json
- 失败 `production evidence check`

## manifest 与路由

- 路由：`224`
- 表：`70`
- crontab：`9`
- 事件处理器：`10`
- 异步队列注解：`15`
- 运行时 route_total：`224`
- 运行时 migrated_manifest_route_total：`224`
- 运行时 missing_route_total：`0`
- Go extra_route_total：`209`

## 本地短证据包：docs/evidence/latest

- 通过 `test`：`docs/evidence/latest/test.log`
- 通过 `independent-package`：`docs/evidence/latest/independent-package.log`
- 通过 `inventory-parity`：`docs/evidence/latest/inventory-parity.log`
- 通过 `acceptance-core`：`docs/evidence/latest/acceptance-core.log`
- 通过 `acceptance-saas`：`docs/evidence/latest/acceptance-saas.log`
- 通过 `acceptance-workers`：`docs/evidence/latest/acceptance-workers.log`
- 通过 `acceptance-cron`：`docs/evidence/latest/acceptance-cron.log`
- 通过 `acceptance-frontend`：`docs/evidence/latest/acceptance-frontend.log`
- 通过 `acceptance-mysql57`：`docs/evidence/latest/acceptance-mysql57.log`
- 通过 `route-coverage`：`docs/evidence/latest/route-coverage.log`
- 通过 `frontend-dist-api-coverage`：`docs/evidence/latest/frontend-dist-api-coverage.log`
- 通过 `stage-report`：`docs/evidence/latest/stage-report.log`
- 通过 `production-evidence`：`docs/evidence/latest/production-evidence.log`
- 证据包源码指纹：`ad95a7ba63dc2ec688355083c5cedbb9e81790ada25a4136ee01c01c06ae0e25`
- 当前源码指纹：`ad95a7ba63dc2ec688355083c5cedbb9e81790ada25a4136ee01c01c06ae0e25`
- 源码指纹匹配：`True`

## 持续运行与残留

- 未找到可解析的 soak.ndjson。
- 当前未发现 `standalone_soak_24h.sh` 进程。
- 发现 mochat-go 容器：`mochat-go-compose-app-check-app-1 Up About an hour (healthy) 0.0.0.0:18090->8080/tcp, [::]:18090->8080/tcp, 0.0.0.0:18091->8081/tcp, [::]:18091->8081/tcp, 0.0.0.0:18092->8082/tcp, [::]:18092->8082/tcp`
- 发现 mochat-go 容器：`mochat-go-compose-app-check-mysql-1 Up 11 hours (healthy) 0.0.0.0:13318->3306/tcp, [::]:13318->3306/tcp`
- 发现 mochat-go 容器：`mochat-go-compose-app-check-redis-1 Up 20 hours (healthy) 0.0.0.0:26391->6379/tcp, [::]:26391->6379/tcp`

## 生产证据缺口

- 无效 `MySQL 5.7 amd64 真实容器门禁`：`MOCHAT_EVIDENCE_MYSQL57_AMD64=docs/evidence/production/current/mysql57-amd64.log` 指向的文件不存在。在 amd64/x86_64 CI 执行 `env -u GOROOT ./scripts/ci_mysql57_amd64.sh` 的日志或报告。
- 无效 `真实企业微信账号联调`：`MOCHAT_EVIDENCE_REAL_WECOM=docs/evidence/production/current/real-wecom.md` 指向的文件不存在。真实企业微信账号下授权、回调解密、通讯录、客户、客户群、标签、素材和企微应用链路验收记录。
- 无效 `真实微信开放平台联调`：`MOCHAT_EVIDENCE_REAL_WECHAT_OPEN=docs/evidence/production/current/real-wechat-open.md` 指向的文件不存在。真实微信开放平台第三方平台 ticket、预授权、授权回跳、公众号资料回填、取消授权和消息回调验收记录。
- 无效 `真实 SaaS 多租户数据回归`：`MOCHAT_EVIDENCE_REAL_SAAS_TENANTS=docs/evidence/production/current/real-saas-tenants.md` 指向的文件不存在。两个以上真实租户业务数据下菜单权限、企业归属、资源额度、上传账本、异步任务和告警隔离验收记录。
- 无效 `生产前端浏览器回归`：`MOCHAT_EVIDENCE_PROD_FRONTEND=docs/evidence/production/current/prod-frontend.md` 指向的文件不存在。dashboard/sidebar/operation 生产构建产物的真实浏览器业务路径和异常路径回归记录。
- 无效 `稳定性记录`：`MOCHAT_EVIDENCE_STABILITY=docs/evidence/production/current/stability.md` 指向的文件不存在。目标部署环境短稳回归、健康检查或外部监控记录；legacy 场景可指向 `soak.ndjson`。
