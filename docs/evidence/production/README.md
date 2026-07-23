# MoChat Go 生产证据目录

该目录用于放置最终生产验收所需的外部证据文件。`scripts/production_evidence_check.sh` 会读取下列环境变量，并在严格模式下要求每个证据文件真实存在、非空且满足必要 marker / 核验项关键词。

可先生成模板：

```bash
./scripts/init_production_evidence_pack.sh
```

模板文件带有 `MOCHAT_EVIDENCE_TEMPLATE_DO_NOT_USE` 标记。真实联调完成后必须删除该标记并填写实际记录，否则严格门禁会拒绝。

真实生产证据必须脱敏后再落盘。`scripts/production_evidence_check.sh` 会拒绝疑似原始 `Authorization`、`Bearer`、JSON / query 中的 `access_token`、`component_access_token`、`authorizer_refresh_token`、`corpsecret`、`encodingaeskey`、`password` 等敏感值；也会拒绝证据正文中出现的 `example.com`、`localhost`、`127.0.0.1`、`.test`、`.invalid` 等示例域名或本机地址，避免把本地 fixture 或占位环境误当成真实生产证据。证据中应只保留脱敏摘要、必要业务字段、截图/日志/工单引用或 `<redacted>` 占位。生产证据采集脚本会对常见响应摘要和 URL 参数做自动脱敏，但真实证据仍应在导入前人工复核。

每份生产证据都必须记录当前源码/验收脚本指纹，格式为 `源码指纹：<scripts/source_fingerprint.py 输出的 fingerprint>`。采集脚本会自动写入该字段；手工整理或导入外部日志时也必须保留。严格门禁会拒绝缺少源码指纹或指纹不等于当前源码的过期证据。

## MySQL 5.7 amd64 真实容器门禁

- 环境变量：`MOCHAT_EVIDENCE_MYSQL57_AMD64`
- 推荐来源：`.github/workflows/mysql57-amd64.yml` 的 `mysql57-amd64-evidence` artifact。
- 本地 amd64 或 CI 命令：

```bash
env -u GOROOT ./scripts/ci_mysql57_amd64.sh 2>&1 | tee docs/evidence/production/mysql57-amd64.log
```

如果拿到的是 GitHub artifact zip 或下载后的日志，可先导入到标准证据位置：

```bash
./scripts/import_mysql57_amd64_evidence.sh path/to/mysql57-amd64-evidence.zip
# 或：
./scripts/import_mysql57_amd64_evidence.sh path/to/mysql57-amd64.log
```

有效证据必须包含 `mysql57 amd64 CI gate passed` 或 `mysql 5.7 schema migration smoke passed`、当前 `源码指纹`，且不能是 arm64 skip 日志。

## 真实企业微信账号联调

- 环境变量：`MOCHAT_EVIDENCE_REAL_WECOM`
- 建议文件：`docs/evidence/production/real-wecom.md`
- 至少记录授权、回调解密、通讯录、客户、客户群、标签、素材和企微应用消息链路；脚本会检查 `授权`、`回调`、`通讯录`、`客户`、`客户群`、`标签`、`素材`。

可用生产 API 和真实回调记录自动生成标准证据：

```bash
MOCHAT_REAL_WECOM_BASE_URL=https://mochat-prod.your-domain.cn \
MOCHAT_REAL_WECOM_TOKEN=dashboard-jwt \
MOCHAT_REAL_WECOM_CALLBACK_EVIDENCE="@docs/evidence/production/wecom-callback-run.txt" \
MOCHAT_REAL_WECOM_AGENT_MESSAGE_EVIDENCE="@docs/evidence/production/wecom-agent-message-run.txt" \
MOCHAT_REAL_WECOM_LOG_REF="工单/日志/截图引用" \
./scripts/capture_real_wecom_evidence.sh
```

默认会访问授权、通讯录、客户、客户群、标签和素材读路径；生产路径或参数不一致时，可覆盖 `MOCHAT_REAL_WECOM_AUTH_PATHS`、`MOCHAT_REAL_WECOM_DIRECTORY_PATHS`、`MOCHAT_REAL_WECOM_CONTACT_PATHS`、`MOCHAT_REAL_WECOM_ROOM_PATHS`、`MOCHAT_REAL_WECOM_TAG_PATHS` 和 `MOCHAT_REAL_WECOM_MEDIUM_PATHS`。该脚本只有在 API 请求、回调证据和截图/日志/工单引用都满足时才会输出 `结论：通过`。

## 真实微信开放平台联调

- 环境变量：`MOCHAT_EVIDENCE_REAL_WECHAT_OPEN`
- 建议文件：`docs/evidence/production/real-wechat-open.md`
- 至少记录第三方平台 ticket、预授权、授权回跳、公众号资料回填、取消授权和消息回调；脚本会检查 `ticket`、`预授权`、`授权回跳`、`资料回填`、`取消授权`、`消息回调`。

可用生产 API 和真实回调记录自动生成标准证据：

```bash
MOCHAT_REAL_WECHAT_OPEN_BASE_URL=https://mochat-prod.your-domain.cn \
MOCHAT_REAL_WECHAT_OPEN_TOKEN=dashboard-jwt \
MOCHAT_REAL_WECHAT_OPEN_TICKET_EVIDENCE="@docs/evidence/production/wechat-open-ticket-run.txt" \
MOCHAT_REAL_WECHAT_OPEN_AUTH_REDIRECT_EVIDENCE="@docs/evidence/production/wechat-open-auth-redirect-run.txt" \
MOCHAT_REAL_WECHAT_OPEN_CANCEL_EVIDENCE="@docs/evidence/production/wechat-open-cancel-run.txt" \
MOCHAT_REAL_WECHAT_OPEN_MESSAGE_CALLBACK_EVIDENCE="@docs/evidence/production/wechat-open-message-callback-run.txt" \
MOCHAT_REAL_WECHAT_OPEN_LOG_REF="工单/日志/截图引用" \
./scripts/capture_real_wechat_open_evidence.sh
```

默认会访问 `getPreAuthUrl` 和 `officialAccount/index`；生产路径或参数不一致时，可覆盖 `MOCHAT_REAL_WECHAT_OPEN_PREAUTH_PATHS` 和 `MOCHAT_REAL_WECHAT_OPEN_PROFILE_PATHS`。该脚本只有在预授权、资料回填、ticket、授权回跳、取消授权、消息回调和截图/日志/工单引用都满足时才会输出 `结论：通过`。

## 真实 SaaS 多租户数据回归

- 环境变量：`MOCHAT_EVIDENCE_REAL_SAAS_TENANTS`
- 建议文件：`docs/evidence/production/real-saas-tenants.md`
- 至少使用两个以上真实租户，覆盖菜单权限、企业归属、资源额度、上传账本、异步任务、告警隔离和运维处置；脚本会检查 `租户`、`菜单权限`、`企业归属`、`资源额度`、`上传账本`、`异步任务`、`告警`。

可用生产 API 自动采集读路径和跨租户隔离证据：

```bash
MOCHAT_REAL_SAAS_BASE_URL=https://mochat-prod.your-domain.cn \
MOCHAT_REAL_SAAS_TENANT_A_TOKEN=tenant-a-dashboard-jwt \
MOCHAT_REAL_SAAS_TENANT_B_TOKEN=tenant-b-dashboard-jwt \
MOCHAT_REAL_SAAS_TENANT_A_NAME=tenant-a \
MOCHAT_REAL_SAAS_TENANT_B_NAME=tenant-b \
MOCHAT_REAL_SAAS_TENANT_A_MARKER=tenant-a-corp-or-user-marker \
MOCHAT_REAL_SAAS_TENANT_B_MARKER=tenant-b-corp-or-user-marker \
MOCHAT_REAL_SAAS_TENANT_A_FORBIDDEN_PATHS="/dashboard/someBResource/show?id=..." \
MOCHAT_REAL_SAAS_TENANT_B_FORBIDDEN_PATHS="/dashboard/someAResource/show?id=..." \
MOCHAT_REAL_SAAS_QUOTA_EVIDENCE="@docs/evidence/production/saas-quota-run.txt" \
MOCHAT_REAL_SAAS_UPLOAD_LEDGER_EVIDENCE="@docs/evidence/production/saas-upload-ledger-run.txt" \
MOCHAT_REAL_SAAS_ASYNC_EVIDENCE="@docs/evidence/production/saas-async-run.txt" \
MOCHAT_REAL_SAAS_ALERT_EVIDENCE="@docs/evidence/production/saas-alert-run.txt" \
./scripts/capture_real_saas_tenants_evidence.sh
```

该脚本只有在两租户正常读路径、租户标识、跨租户禁止访问、额度/账本/异步/告警证据全部满足时才会输出 `结论：通过`。

## 生产前端浏览器回归

- 环境变量：`MOCHAT_EVIDENCE_PROD_FRONTEND`
- 建议文件：`docs/evidence/production/prod-frontend.md`
- 至少覆盖 dashboard、sidebar、operation 的生产构建产物、主要业务路径和异常路径；脚本会检查 `dashboard`、`sidebar`、`operation`、`生产`、`异常`。

可用真实浏览器自动采集证据：

```bash
MOCHAT_PROD_FRONTEND_BASE_URL=https://mochat-prod.your-domain.cn \
MOCHAT_PROD_FRONTEND_BUILD_VERSION=prod-build-id \
MOCHAT_PROD_FRONTEND_DASHBOARD_PATHS="/login /dashboard" \
MOCHAT_PROD_FRONTEND_SIDEBAR_PATHS="/sidebar-app/contact?agentId=1" \
MOCHAT_PROD_FRONTEND_OPERATION_PATHS="/operation-app/workFission?id=1" \
MOCHAT_PROD_FRONTEND_AUTH_STATE=docs/evidence/production/playwright-auth-state.json \
./scripts/capture_prod_frontend_evidence.sh
```

`MOCHAT_PROD_FRONTEND_AUTH_STATE` 是可选的 Playwright 登录态文件；生产页面需要登录时，应先用受控测试账号生成该文件。脚本会输出 `prod-frontend.md` 和截图目录，并把 console error、同源请求失败和非预期 4xx/5xx 写入证据记录。

## 稳定性记录

- 环境变量：`MOCHAT_EVIDENCE_STABILITY`
- 建议文件：`docs/evidence/production/stability.md`。
- 用户当前已要求不再继续新的 24 小时 run；默认使用目标环境已有短稳回归、健康检查和外部监控记录。脚本会检查 `短稳回归` 和 `结论`。

推荐从外部监控摘要生成标准证据：

```bash
MOCHAT_STABILITY_MONITOR_EVIDENCE="@docs/evidence/production/stability-monitor-run.txt" \
MOCHAT_STABILITY_HEALTH_EVIDENCE="@docs/evidence/production/stability-health-run.txt" \
MOCHAT_STABILITY_RESOURCE_EVIDENCE="@docs/evidence/production/stability-resource-run.txt" \
MOCHAT_STABILITY_TIME_RANGE="2026-07-08 10:00:00 CST 至 2026-07-08 10:30:00 CST" \
MOCHAT_STABILITY_LOG_REF="监控面板/日志/工单引用" \
./scripts/capture_stability_evidence.sh
```

如已有目标环境探测输出，仍可用 legacy `soak.ndjson` 输入导入：

```bash
MOCHAT_STABILITY_SOAK_LOG=/path/to/soak.ndjson \
MOCHAT_STABILITY_LOG_REF="监控面板/日志/工单引用" \
MOCHAT_STABILITY_MIN_DURATION_SECONDS=300 \
./scripts/capture_stability_evidence.sh
```

外部监控摘要不能只写“monitor ok”这类结论短句；预检会要求监控摘要包含路由覆盖和 `224`，健康证据包含 `/readyz`、health、健康、探测或 `200` 等标记，资源证据包含 RSS、memory、CPU、连接、内存或资源等标记，并且 `MOCHAT_STABILITY_TIME_RANGE` 必须写明起止日期。该脚本只解析已有运行记录，不启动 24 小时持续运行。若上线另行要求满 24 小时长稳，可把 `MOCHAT_STABILITY_MIN_DURATION_SECONDS` 设为 `86400` 并导入已有记录。

## 生产证据采集诊断

在真正访问生产前，可先运行 doctor 检查标准证据文件、采集环境变量和严格证据门禁状态：

```bash
./scripts/production_evidence_doctor.sh
```

报告默认写入 `docs/evidence/production/readiness.md`，机器可读状态默认写入 `docs/evidence/production/readiness.json`，待填写环境变量清单默认写入 `docs/evidence/production/readiness.env.todo`；同时保留严格证据门禁的 `readiness-production-evidence.md` 与 `readiness-production-evidence.json`。JSON 会区分标准文件是否存在和是否通过严格生产证据规则；模板、待验收、未脱敏、非生产 URL、缺结构化记录项、缺源码指纹或源码指纹过期等文件即使存在，也会标记为无效。JSON 和 env 清单只记录变量名、标准文件路径、缺失项、采集命令和门禁返回码，不记录 token 或 secret 值。该脚本默认只诊断，不访问生产，不启动 24 小时持续运行。

不要直接把 token 或 secret 写入 `readiness.env.todo`。先复制成本地文件再填写；`readiness.env.local` 已加入 `.gitignore`：

```bash
cp docs/evidence/production/readiness.env.todo docs/evidence/production/readiness.env.local
```

填写本地 env 文件后，先做采集环境预检。预检会检查变量是否齐备、`@文件` 是否存在、URL 是否有效，并默认拒绝 `example.com`、`localhost`、`127.0.0.1` 等示例域名或本机地址作为真实生产采集地址；真实采集脚本本身也会做同样的 base URL 拦截，防止跳过 preflight 后误采占位地址。同时校验 `MOCHAT_MYSQL57_AMD64_EVIDENCE_SOURCE` 指向的日志或 artifact zip 是否包含 MySQL 5.7 amd64 通过标记、是否误用了 arm64 skip 日志；如果稳定性证据走外部监控摘要，也会检查路由覆盖、健康检查、资源使用和起止日期是否齐备。若标准证据文件已经存在但严格门禁判定无效，preflight 会标为“标准文件无效”；每一类证据只要有有效标准文件或采集变量齐备就算就绪，因此 6 类标准文件都有效时，不需要 env 文件也能通过 strict 预检并直接进入证据包收拢。它不访问生产、不打印密钥值、不启动 24 小时持续运行：

```bash
MOCHAT_PRODUCTION_EVIDENCE_ENV_FILE=docs/evidence/production/readiness.env.local \
./scripts/production_evidence_env_preflight.sh
```

预检通过后，可在受控终端加载并显式执行采集/导入：

```bash
set -a
. docs/evidence/production/readiness.env.local
set +a
MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_RUN=1 ./scripts/production_evidence_doctor.sh
```

随后收拢生产证据包：

```bash
./scripts/collect_production_evidence_pack.sh
```

也可以通过 `MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_ENV_OUT` 覆盖 env 清单输出路径，通过 `MOCHAT_PRODUCTION_EVIDENCE_PREFLIGHT_STRICT=1` 让预检未就绪时返回非 0。如需刷新已存在的标准证据文件，可再加 `MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_REFRESH=1`。如需让缺任一证据或采集失败时返回非 0，可设置 `MOCHAT_PRODUCTION_EVIDENCE_DOCTOR_STRICT=1`。

仅本地 fixture 或 smoke 需要使用示例域名、本机地址时，才允许显式设置 `MOCHAT_PRODUCTION_EVIDENCE_ALLOW_NON_PROD_URLS=1`；真实生产取证不要设置这个变量。

## 生产候选总门禁

收齐生产证据后，优先执行总门禁。它会默认先跑全套非 PHP 本地短验收，再跑严格生产证据检查，并输出 `docs/evidence/production/candidate/index.md`。

也可以先把外部证据收拢成一个可复核包：

```bash
MOCHAT_PRODUCTION_EVIDENCE_SOURCE_DIR=docs/evidence/production \
MOCHAT_PRODUCTION_EVIDENCE_PACK_DIR=docs/evidence/production/current \
./scripts/collect_production_evidence_pack.sh
```

该脚本会复制标准证据文件、生成 `env.production-evidence`、记录 `manifest.json` 中的大小、sha256 和当前源码指纹、执行严格生产证据文件检查，并生成 `production-evidence.md` 与 `production-evidence.json`；默认继续运行 skip-local 生产候选门禁；它不会启动 24 小时持续运行。只想先检查证据文件本身时，可设置 `MOCHAT_PRODUCTION_EVIDENCE_RUN_CANDIDATE=0`。复制前会确认每份源证据包含当前源码指纹，防止旧生产证据被复用到新构建。

证据包收拢会在复制前扫描源证据。若源文件疑似包含未脱敏 token、secret 或 password，脚本会在 `index.md` 标记为 `敏感值拒绝，未复制`；若源文件包含示例域名或本机地址，会标记为 `非生产地址拒绝，未复制`；若源文件缺少当前源码指纹或源码指纹过期，会标记为 `源码指纹拒绝，未复制`。这三类问题都会让严格门禁失败，避免把问题证据落入 `docs/evidence/production/current/`。

```bash
MOCHAT_EVIDENCE_MYSQL57_AMD64=docs/evidence/production/mysql57-amd64.log \
MOCHAT_EVIDENCE_REAL_WECOM=docs/evidence/production/real-wecom.md \
MOCHAT_EVIDENCE_REAL_WECHAT_OPEN=docs/evidence/production/real-wechat-open.md \
MOCHAT_EVIDENCE_REAL_SAAS_TENANTS=docs/evidence/production/real-saas-tenants.md \
MOCHAT_EVIDENCE_PROD_FRONTEND=docs/evidence/production/prod-frontend.md \
MOCHAT_EVIDENCE_STABILITY=docs/evidence/production/stability.md \
./scripts/production_candidate_gate.sh
```

如果本地短验收已经由 `docs/evidence/latest/` 或 CI artifact 单独完成，可只复核外部生产证据：

```bash
MOCHAT_PRODUCTION_CANDIDATE_SKIP_LOCAL=1 \
MOCHAT_EVIDENCE_MYSQL57_AMD64=docs/evidence/production/mysql57-amd64.log \
MOCHAT_EVIDENCE_REAL_WECOM=docs/evidence/production/real-wecom.md \
MOCHAT_EVIDENCE_REAL_WECHAT_OPEN=docs/evidence/production/real-wechat-open.md \
MOCHAT_EVIDENCE_REAL_SAAS_TENANTS=docs/evidence/production/real-saas-tenants.md \
MOCHAT_EVIDENCE_PROD_FRONTEND=docs/evidence/production/prod-frontend.md \
MOCHAT_EVIDENCE_STABILITY=docs/evidence/production/stability.md \
./scripts/production_candidate_gate.sh
```

## 严格生产证据检查

```bash
MOCHAT_PRODUCTION_EVIDENCE_STRICT=1 \
MOCHAT_PRODUCTION_EVIDENCE_OUT=docs/evidence/production/current/production-evidence.md \
MOCHAT_PRODUCTION_EVIDENCE_JSON_OUT=docs/evidence/production/current/production-evidence.json \
MOCHAT_EVIDENCE_MYSQL57_AMD64=docs/evidence/production/mysql57-amd64.log \
MOCHAT_EVIDENCE_REAL_WECOM=docs/evidence/production/real-wecom.md \
MOCHAT_EVIDENCE_REAL_WECHAT_OPEN=docs/evidence/production/real-wechat-open.md \
MOCHAT_EVIDENCE_REAL_SAAS_TENANTS=docs/evidence/production/real-saas-tenants.md \
MOCHAT_EVIDENCE_PROD_FRONTEND=docs/evidence/production/prod-frontend.md \
MOCHAT_EVIDENCE_STABILITY=docs/evidence/production/stability.md \
./scripts/production_evidence_check.sh
```
