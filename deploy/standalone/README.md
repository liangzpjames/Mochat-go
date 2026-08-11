# MoChat Go 独立部署栈

这个 compose 可启动 Go 版运行所需的 MySQL、Redis 和可选 Go app 容器，不启动 PHP Hyperf，也不挂载原 `../mochat` 源码目录。

standalone Compose 默认通过 `MOCHAT_TIMEZONE=Asia/Shanghai` 统一应用与 MariaDB 时区；部署到其他地区时应在环境文件中显式改为目标 IANA 时区，不要让应用、数据库和运维 CLI 使用不同日期边界。

完整交付候选流程见 `docs/phases/phase-pre0-standalone/plans/release-candidate.md`。正式部署前先复制 `deploy/standalone/.env.example` 为已忽略的 `deploy/standalone/.env.local`，替换所有 `CHANGE_ME` 值，再通过 `--env-file deploy/standalone/.env.local` 启动。

Docker 构建会用 `scripts/source_fingerprint.py` 计算源码与验收配置指纹，并通过 Go linker 写入四个交付二进制。SaaS 发布准备中心只接受当前运行二进制内置的权威指纹，前端自动回填且不可修改；未内置指纹时发布候选门禁保持禁用。本地直接 `go run` 可用 `MOCHAT_GO_RELEASE_SOURCE_FINGERPRINT` 注入当前指纹，正式镜像的内置值始终优先，环境变量不能覆盖。

发布证据状态为 `passed` 时，应用会下载 HTTPS 工件并比对实际 SHA-256 与字节数；创建发布候选时会再次下载六项工件，只有复核全部通过且源码指纹一致才生成 `ready`。准备状态还会把候选不可变快照与当前六项证据逐项比较；任何证据的状态、版本、URL、摘要、大小或复核信息变化，旧候选都会立即降为 `stale`，即使证据随后恢复也必须重新运行门禁生成新候选。下载默认超时 30 秒、单工件最大 64 MiB，并启用私网/云元数据阻断、DNS 绑定、禁用环境代理和同源重定向限制。内部证据库只能通过 `MOCHAT_GO_SAAS_RELEASE_EVIDENCE_ALLOWED_CIDRS` 放行最小网段；若使用自有 CA，将证书以只读方式挂载到 app 容器，再把容器内路径写入 `MOCHAT_GO_SAAS_RELEASE_EVIDENCE_CA_FILE`。

## 启动

只启动依赖栈，供本机 `go run ./cmd/mochat-go` 或 smoke 脚本使用：

```bash
cd /Users/lv/Documents/企业微信/mochat-go
docker compose -f deploy/standalone/docker-compose.yml up -d mysql redis
```

启动完整 Go standalone 容器栈：

```bash
cd /Users/lv/Documents/企业微信/mochat-go
cp deploy/standalone/.env.example deploy/standalone/.env.local
# 编辑 deploy/standalone/.env.local 后启动
docker compose \
  --env-file deploy/standalone/.env.local \
  -f deploy/standalone/docker-compose.yml \
  --profile app up -d --build
```

正常 app 启动不挂载、也不依赖 bootstrap 密码文件。首次使用容器栈时，先记录已由 MySQL init SQL 建好的 schema baseline；随后只在现有 app 容器内临时放置一次性密码文件，初始化完成或失败都会删除容器内副本：

```bash
docker compose \
  --env-file deploy/standalone/.env.local \
  -f deploy/standalone/docker-compose.yml \
  --profile app exec app \
  mochat-migrate -action baseline -project-root /app

BOOTSTRAP_HOST_FILE="/secure/path/from-secret-manager"
BOOTSTRAP_REQUEST_KEY="initial-saas-admin-v1"
BOOTSTRAP_LOGIN="platform-admin"
BOOTSTRAP_PHONE="13800000000"
BOOTSTRAP_NAME="Platform Admin"

BOOTSTRAP_CONTAINER_FILE="$(
  docker compose \
    --env-file deploy/standalone/.env.local \
    -f deploy/standalone/docker-compose.yml \
    --profile app exec -T -u 0 app \
    sh -c 'umask 077; mktemp /tmp/mochat-bootstrap-saas-admin.XXXXXX'
)"

cleanup_bootstrap_file() {
  docker compose \
    --env-file deploy/standalone/.env.local \
    -f deploy/standalone/docker-compose.yml \
    --profile app exec -T -u 0 app rm -f "$BOOTSTRAP_CONTAINER_FILE" >/dev/null 2>&1 || true
}
trap cleanup_bootstrap_file EXIT

docker compose \
  --env-file deploy/standalone/.env.local \
  -f deploy/standalone/docker-compose.yml \
  --profile app cp "$BOOTSTRAP_HOST_FILE" "app:$BOOTSTRAP_CONTAINER_FILE"

docker compose \
  --env-file deploy/standalone/.env.local \
  -f deploy/standalone/docker-compose.yml \
  --profile app exec -T -u 0 app chmod 0400 "$BOOTSTRAP_CONTAINER_FILE"
docker compose \
  --env-file deploy/standalone/.env.local \
  -f deploy/standalone/docker-compose.yml \
  --profile app exec -T -u 0 \
  -e "MOCHAT_BOOTSTRAP_SAAS_ADMIN_PASSWORD_FILE=$BOOTSTRAP_CONTAINER_FILE" \
  app mochat-bootstrap \
    -request-key "$BOOTSTRAP_REQUEST_KEY" \
    -login-name "$BOOTSTRAP_LOGIN" \
    -phone "$BOOTSTRAP_PHONE" \
    -name "$BOOTSTRAP_NAME"
```

`BOOTSTRAP_HOST_FILE` 由 Secret Manager 提供，文件内容不进入命令行、环境变量或日志；命令只把路径作为 `*_FILE` 传给一次性 bootstrap 进程。`docker compose cp` 复用已经运行的 app 容器，不创建新服务或容器；退出 trap 会删除容器内临时文件，宿主机原始文件仍由 Secret Manager 管理。Unix 文件权限只能允许 owner，Windows 必须使用仅 owner/系统管理员可读的 ACL。

默认端口：

- MySQL：`127.0.0.1:13316`
- Redis：`127.0.0.1:26389`
- Go dashboard/API：容器栈默认映射到 `127.0.0.1:18080`，可用 `MOCHAT_GO_PORT` 覆盖
- Go sidebar 前端：容器栈默认映射到 `127.0.0.1:18081`，可用 `MOCHAT_SIDEBAR_PORT` 覆盖
- Go operation 前端：容器栈默认映射到 `127.0.0.1:18082`，可用 `MOCHAT_OPERATION_PORT` 覆盖
- 上传静态资源：Go dashboard/API 会把容器内 `MOCHAT_FILE_STORAGE_ROOT` 以只读方式托管到 `/static/*`

数据库初始化使用本项目内置 SQL：

```text
deploy/standalone/schema/mochat.sql
deploy/standalone/migrations/0002_seed_core_data.up.sql
deploy/standalone/migrations/0003_saas_provisioning.up.sql
deploy/standalone/migrations/0004_saas_storage_objects.up.sql
deploy/standalone/migrations/0005_background_tasks.up.sql
deploy/standalone/migrations/0006_saas_alerts.up.sql
deploy/standalone/migrations/0007_wechat_component_tickets.up.sql
deploy/standalone/migrations/0008_saas_alert_notifications.up.sql
deploy/standalone/migrations/0009_contact_batch_add_rbac.up.sql
deploy/standalone/migrations/0010_sensitive_words.up.sql
deploy/standalone/migrations/0011_sop_rbac.up.sql
deploy/standalone/migrations/0012_shop_code.up.sql
deploy/standalone/migrations/0013_radar.up.sql
deploy/standalone/migrations/0014_auto_tag.up.sql
deploy/standalone/migrations/0015_lottery.up.sql
deploy/standalone/migrations/0016_room_fission.up.sql
deploy/standalone/migrations/0017_room_clock_in.up.sql
deploy/standalone/migrations/0018_room_quality.up.sql
deploy/standalone/migrations/0019_room_calendar.up.sql
deploy/standalone/migrations/0020_room_remind.up.sql
deploy/standalone/migrations/0021_room_infinite_pull.up.sql
deploy/standalone/migrations/0022_work_message_archive_sync.up.sql
deploy/standalone/migrations/0023_saas_alert_settings.up.sql
deploy/standalone/migrations/0024_saas_package_extended_limits.up.sql
deploy/standalone/migrations/0025_saas_radar_limit.up.sql
deploy/standalone/migrations/0026_saas_lottery_limit.up.sql
deploy/standalone/migrations/0027_saas_room_infinite_pull_limit.up.sql
deploy/standalone/migrations/0028_saas_room_fission_limit.up.sql
deploy/standalone/migrations/0029_saas_room_clock_in_limit.up.sql
deploy/standalone/migrations/0030_saas_room_operation_limits.up.sql
deploy/standalone/migrations/0031_saas_sop_limits.up.sql
deploy/standalone/migrations/0032_saas_sensitive_word_limit.up.sql
deploy/standalone/migrations/0033_saas_admin_operation_logs.up.sql
deploy/standalone/migrations/0034_saas_billing_events.up.sql
deploy/standalone/migrations/0035_saas_admin_tasks.up.sql
deploy/standalone/migrations/0036_saas_notification_policy_controls.up.sql
deploy/standalone/migrations/0037_saas_notification_health_index.up.sql
deploy/standalone/migrations/0038_saas_notification_slo_index.up.sql
deploy/standalone/migrations/0039_saas_subscription_lifecycle.up.sql
deploy/standalone/migrations/0040_saas_payment_collection.up.sql
deploy/standalone/migrations/0041_saas_payment_refunds.up.sql
deploy/standalone/migrations/0042_saas_billing_invoices.up.sql
deploy/standalone/migrations/0043_saas_payment_settlements.up.sql
deploy/standalone/migrations/0044_saas_payment_settlement_sync.up.sql
deploy/standalone/migrations/0045_saas_admin_rbac.up.sql
deploy/standalone/migrations/0046_saas_admin_approvals.up.sql
deploy/standalone/migrations/0047_saas_admin_approval_governance.up.sql
deploy/standalone/migrations/0048_saas_admin_system_health.up.sql
deploy/standalone/migrations/0049_saas_service_accounts.up.sql
deploy/standalone/migrations/0050_saas_backup_recovery.up.sql
deploy/standalone/migrations/0051_saas_backup_resilience.up.sql
deploy/standalone/migrations/0052_saas_compliance_lifecycle.up.sql
deploy/standalone/migrations/0053_saas_identity_security.up.sql
deploy/standalone/migrations/0054_saas_branding_profiles.up.sql
deploy/standalone/migrations/0055_saas_tenant_domains.up.sql
deploy/standalone/migrations/0056_saas_domain_delivery.up.sql
deploy/standalone/migrations/0057_saas_release_readiness.up.sql
deploy/standalone/migrations/0058_saas_release_evidence_integrity.up.sql
deploy/standalone/migrations/0059_saas_service_account_rate_limits.up.sql
deploy/standalone/migrations/0060_saas_service_account_usage_retention.up.sql
```

生产或升级场景建议使用 Go migration 工具显式建库和记录版本：

```bash
env -u GOROOT \
  MOCHAT_MYSQL_DSN='mochat:mochat_pass@tcp(127.0.0.1:13316)/mochat?parseTime=true&loc=Local' \
  go run ./cmd/mochat-migrate -action apply -project-root .
```

如果数据库已经通过 `deploy/standalone/schema/mochat.sql` 初始化，可以先记录基线版本：

```bash
env -u GOROOT \
  MOCHAT_MYSQL_DSN='mochat:mochat_pass@tcp(127.0.0.1:13316)/mochat?parseTime=true&loc=Local' \
  go run ./cmd/mochat-migrate -action baseline -project-root .
```

当前 `0001_initial_schema` 只负责原 MoChat 业务建表，`0002_seed_core_data` 负责核心 seed，`0003` 至 `0035` 负责 SaaS 元数据、业务能力、额度、审计、账单和运营任务，`0036` 至 `0044` 负责通知治理、订阅生命周期、收款、退款、发票、渠道结算和自动同步，`0045_saas_admin_rbac` 负责平台角色、权限和成员授权，`0046_saas_admin_approvals` 与 `0047_saas_admin_approval_governance` 负责高风险审批、策略、会签、委托和 SLA 提醒，`0048_saas_admin_system_health` 负责平台健康扫描、系统事故和健康治理权限，`0049_saas_service_accounts` 负责租户服务账号、API Key 和集成治理权限，`0050_saas_backup_recovery` 与 `0051_saas_backup_resilience` 负责加密备份、异地副本和隔离恢复，`0052_saas_compliance_lifecycle` 负责租户数据策略、法律保留、加密导出、双人审批和可恢复擦除证据，`0053_saas_identity_security` 负责登录策略、账号锁定、TOTP/恢复码、持久会话、登录事件与安全事故，`0054_saas_branding_profiles` 负责平台与租户品牌白标档案，`0055_saas_tenant_domains` 负责自定义登录域名、DNS 所有权校验、主域名和 Host 租户隔离，`0056_saas_domain_delivery` 负责域名路由/TLS 交付状态、持久化任务、租约重试和签名回调事件，`0057_saas_release_readiness` 负责六类生产证据、源码指纹一致性与不可变发布候选门禁，`0058_saas_release_evidence_integrity` 固化证据工件 SHA-256 与字节大小，`0059_saas_service_account_rate_limits` 负责服务账号 OpenAPI 限流与日路由用量治理，`0060_saas_service_account_usage_retention` 负责历史用量保留策略与清理治理，`0061_saas_service_account_usage_alerts` 负责账号级用量/拒绝预警、通知冷却和自动恢复，`0062_saas_audit_integrity` 负责操作日志 legacy 锚点、按租户 SHA-256 结构摘要链、篡改校验记录和审计治理权限，`0063_saas_audit_anchor_signatures` 负责数据库外密钥签名、独立证据文件和回退检测，`0064_saas_audit_anchor_remote_immutability` 负责 S3 Object Lock 异地不可变副本、对象版本与留存元数据，`0065_saas_service_account_key_pepper_ring` 负责 API Key 独立 pepper 密钥环、历史密钥选择与旧 JWT pepper 迁移，`0066_saas_alert_credential_encryption` 负责租户通知 webhook URL 与 Secret 加密、历史密钥读取和批量轮换，`0067_wecom_credential_encryption` 负责企业微信企业/应用凭据加密、历史密钥读取和平台治理轮换，`0068_wechat_open_credential_encryption` 负责组件 Ticket 与公众号授权凭据加密、历史密钥读取和平台治理轮换，`0069_saas_release_candidate_approval` 负责发布候选双人会签、短时审批有效期和审批副作用原子关联，`0070_saas_approval_policy_change_guard` 负责审批策略变更双人会签并禁止治理策略自降级，`0071_saas_backup_policy_change_guard` 负责备份策略变更双人会签和原子生效，`0072_saas_backup_cleanup_saga` 负责审批后持久化、可续跑的备份保留清理，`0073_saas_compliance_export_deletion_saga` 负责合规导出提前删除的双人会签、冻结快照、断点续跑和失败恢复，`0074_saas_compliance_legal_hold_release_guard` 负责法律保留解除的双人会签、冻结快照与事务副作用标记，`0075_saas_compliance_policy_change_guard` 负责合规生命周期策略变更的双人会签、版本冻结和原子生效，`0076_saas_identity_policy_change_guard` 负责租户身份安全策略变更的双人会签、标准化载荷冻结和原子生效，`0077_saas_tenant_disable_approval_guard` 负责 critical 级租户停用强制双人会签并禁止策略关闭或降级，`0078_saas_critical_approval_policy_guard` 负责全部 critical 策略强制启用、零绕过阈值和至少双人会签并把退款升级为全额双人审批，`0079_saas_service_account_key_revoke_guard` 负责 API Key 吊销强制双人会签、版本冻结与审批副作用原子关联，`0080_saas_service_account_update_guard` 负责服务账号配置变更强制双人会签、完整配置与版本冻结，以及账号更新、操作审计和审批副作用原子关联，`0081_saas_service_account_key_rotate_guard` 负责 API Key 轮换强制双人会签、延迟生成新 Key、一次性明文交付和审批副作用原子关联，`0082_saas_service_account_create_guard` 负责服务账号创建强制双人会签、延迟生成首个 Key、一次性明文交付和审批副作用原子关联，`0083_saas_identity_mfa_reset_guard` 负责平台重置用户 MFA 的双人会签、冻结租户和凭据版本，并原子禁用 MFA、撤销活动会话、写安全事件与审批副作用，`0084_saas_package_definition_guard` 负责平台套餐定义创建和变更的 critical 双人会签、完整载荷与租户影响冻结、乐观锁版本校验，以及套餐写入、操作审计和审批副作用原子关联，`0085_saas_tenant_package_assignment_guard` 负责租户套餐分配的 critical 双人会签、租户与套餐定义快照冻结、绑定版本校验，以及套餐绑定、订阅同步、操作审计和审批副作用原子关联，`0086_saas_tenant_provision_approval_guard` 负责平台开户双人会签、凭据脱敏、任务版本与请求摘要冻结，以及完整租户资源、任务、审计和审批效果原子提交，`0087_saas_tenant_renewal_approval_guard` 负责租户续费双人会签、租户/套餐/订阅/任务快照冻结，以及套餐权益、账单、订阅、任务、审计和审批效果原子提交，`0088_saas_subscription_transition_approval_guard` 负责总后台人工订阅状态迁移的 critical 双人会签、完整订阅快照与版本冻结，以及订阅、事件、操作审计和审批效果原子提交，`0089_saas_invoice_issue_approval_guard` 负责蓝票和红票正式开具的 critical 双人会签、单据与支付订单账务快照冻结，以及开票/红冲金额、操作审计和审批效果原子提交。上述迁移都会写入 `mochat_go_schema_migrations` 并记录 SQL checksum；各版本明细见 `deploy/standalone/migrations/README.md`。后续增量迁移继续放在：

```text
deploy/standalone/migrations
```

`0086_saas_tenant_provision_approval_guard` 把平台开户纳入 critical 双人会签，并为运营任务增加版本；申请冻结密码哈希、任务请求摘要、套餐定义与开户预览，执行时原子提交完整租户资源、任务状态、操作审计和审批效果。

当前最新增量 `0089_saas_invoice_issue_approval_guard` 把蓝票和红票正式开具纳入 critical 双人会签。审批申请冻结单据身份、版本、票面信息及支付订单版本与金额台账；执行时重新校验全部冻结引用，并原子提交单据状态、开票或红冲金额、操作审计与审批效果。进入处理、失败和取消仍保留财务直接纠错链路。

命名规则为 `0020_xxx.up.sql` 和 `0020_xxx.down.sql`。`apply/status/baseline` 会自动发现 `.up.sql`；`rollback` 只回滚最后一条已应用且具备 `.down.sql` 的增量迁移。`0001_initial_schema` 没有 down 脚本，仍禁止 rollback，避免误删生产库。

如果旧环境已经记录过拆分前的一体化 `0001_initial_schema` checksum，迁移器会识别 legacy checksum；升级后再次执行 `apply` 会接受旧 checksum，并用 `0002_seed_core_data` 补齐 seed 迁移记录。

首次独立部署完成 migration 后，只使用 `cmd/mochat-bootstrap` 创建一个 SaaS 平台管理员。该命令只写入 `mochat_go_saas_admin_users`，不会创建或修改 tenant、corp、`mc_user`、Dashboard identity、套餐或 RBAC 业务数据。

密码文件必须由 Secret Manager 提供并限制读取权限；密码不接受命令行参数、环境变量明文或 `MOCHAT_SIMPLE_JWT_SECRET`，也不会写入日志、返回值或审计正文。上面的现有 app 容器一次性流程是唯一初始化入口。

重复提交相同 request key/login 时只返回已有 SaaS 管理员，不重置密码；不同的企业、租户和 Dashboard 身份必须通过后续受保护的 SaaS API 流程创建和授权。bootstrap 失败时只返回不含密码内容的通用错误。

回滚最后一条增量迁移：

```bash
env -u GOROOT \
  MOCHAT_MYSQL_DSN='mochat:mochat_pass@tcp(127.0.0.1:13316)/mochat?parseTime=true&loc=Local' \
  go run ./cmd/mochat-migrate -action rollback -project-root .
```

## Go standalone 启动示例

```bash
env -u GOROOT \
  MOCHAT_GO_STANDALONE=1 \
  MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1 \
  MOCHAT_GO_ADDR=127.0.0.1:18082 \
  MOCHAT_MYSQL_DSN='mochat:mochat_pass@tcp(127.0.0.1:13316)/mochat?parseTime=true&loc=Local' \
  MOCHAT_REDIS_ADDR=127.0.0.1:26389 \
  MOCHAT_SIMPLE_JWT_SECRET='请替换成生产密钥' \
  go run ./cmd/mochat-go
```

standalone 模式下同时配置 `MOCHAT_MYSQL_DSN` 和 `MOCHAT_SIMPLE_JWT_SECRET` 时，会默认启用当前所有已迁移业务路由；如需只启用少量路由做调试，可设置 `MOCHAT_GO_ENABLE_ALL_MIGRATED_ROUTES=0` 后再单独打开对应 `MOCHAT_GO_MIGRATE_*`。

## 自动检查

> 注意：下列历史业务 smoke 清单不等于本阶段身份验收入口。凡仍依赖旧 tenant bootstrap、Dashboard 旧登录或企业选择的脚本，在 Task12 完成 SaaS/Dashboard 身份切换前均不得执行或宣称可用。

```bash
./scripts/standalone_stack_check.sh
./scripts/standalone_inventory_parity.sh
./scripts/smoke_schema_migrate.sh
./scripts/smoke_saas_provisioning.sh
./scripts/smoke_saas_quota_enforcement.sh
./scripts/smoke_saas_storage_reconcile.sh
./scripts/smoke_saas_storage_reclaim.sh
./scripts/smoke_saas_admin_access_rbac.sh
./scripts/smoke_saas_admin_approvals.sh
./scripts/smoke_saas_tenant_provision_approval.sh
./scripts/smoke_saas_tenant_renewal_approval.sh
./scripts/smoke_saas_usage_refresh.sh
./scripts/lint_mysql57_schema.sh
./scripts/smoke_mysql57_schema_migrate.sh
./scripts/smoke_queue_idempotency.sh
./scripts/smoke_async_file_upload_worker.sh
./scripts/smoke_mark_tags_worker.sh
./scripts/smoke_auto_tag_keyword_task.sh
./scripts/smoke_sidebar_frontend_contact.sh
./scripts/smoke_operation_frontend_work_fission.sh
```

这些脚本会验证：

- MySQL/Redis 容器可健康启动。
- `mc_user` 表存在。
- `mc_rbac_menu` 初始化数据数量为 `458`。
- `scripts/audit_login_corp_validation.sh` 会阻止 dashboard 业务 handler 绕过统一登录企业校验直接信任 `mc:user.{userId}` 缓存。
- `scripts/standalone_inventory_parity.sh` 会在迁移工作区对比 PHP 原项目重新扫描结果和 Go 内置 manifest，确认路由、表、定时任务、事件处理器和队列注解没有清单漏项；生产独立部署不需要保留 PHP 原项目。
- `scripts/standalone_route_coverage.sh` 会验证 standalone + MySQL + Redis + JWT secret 下默认挂载的已迁移业务路由仍覆盖全部 manifest 路由。
- `cmd/mochat-migrate` 可在空数据库上顺序执行当前 89 个版本，从 `0001_initial_schema` 到 `0089_saas_invoice_issue_approval_guard`；其中 `0011_sop_rbac` 兼容旧初始化 SQL 缺少 SOP 主表/触达日志表的情况。迁移器可对已有 schema + seed + SaaS 表执行 `baseline`，拒绝对空库做 baseline，可对带 `.down.sql` 的增量迁移执行 rollback，并可接受拆分前的一体化 `0001_initial_schema` legacy checksum 后补记 seed 迁移。
- `cmd/mochat-bootstrap` 只创建一个 SaaS 平台管理员并只写入 `mochat_go_saas_admin_users`；它不会创建或修改 tenant、corp、`mc_user`、Dashboard identity、套餐或 RBAC 业务数据。密码只通过一次性现有 app 容器流程的受限 `PasswordFile` 读取，`RequestKey` 持久化并用于幂等判定。
- 旧的 tenant bootstrap smoke 已删除；`scripts/smoke_standalone_compose_app.sh` 等历史业务 smoke 尚未切换到 SaaS-only bootstrap，本阶段不作为身份验收，必须由 Task12 更新后才能恢复使用。
- `scripts/smoke_saas_provisioning.sh` 会用 CSV 一次开通两个租户，并验证两个租户管理员都能通过 Go standalone 登录。
- `scripts/smoke_saas_tenant_isolation.sh` 会用真实 MySQL/Redis/Go standalone 验证两个租户的企业绑定、Redis 企业选择缓存、首页统计、基础通讯录、角色列表、权限菜单和 dashboard 上传账本互相隔离，并主动污染 `mc:user.{userId}` 为其他租户企业来回归读路径不会跨租户取数；两个租户分别上传文件后，脚本会断言 `mochat_go_saas_storage_objects` 和 `storage_mb` 用量只归属各自租户。
- `scripts/smoke_saas_quota_enforcement.sh` 会用真实 Go standalone 写接口验证企业数、子账号数、应用数、渠道活码数、门店活码数、互动雷达数、抽奖活动数、无限拉群数、群裂变数、群打卡数、群质检规则数、群日历数、客户群提醒数、个人 SOP 规则数、群 SOP 规则数、敏感词词库数、客户数、客户群数、客户群发任务数、客户群群发任务数、标签建群任务数、自动拉群活码数、裂变活动数、公众号授权数和素材存储套餐上限拦截，并确认超额请求不会写入对应业务表、`mochat_go_saas_storage_objects` 或上传目录。
- 渠道活码创建或 `cron-channel-code` 定时刷新 contact_way 失败触发软删回滚时，会同步刷新 `channel_codes` 用量；其中创建失败回滚还会解析 `welcome_message.messageDetail` 里的本地欢迎语素材路径，回收 `mochat_go_saas_storage_objects` 并刷新 `storage_mb`。门店活码创建和删除会同步刷新 `shop_codes` 用量，更新、删除门店活码或通过 `pageSet` 替换页面设置时，也会回收 `employee_qrcode`、`qw_code` 活码 JSON 和 `mc_shop_code_page.default` / `poster` 中的本地二维码/海报账本并刷新 `storage_mb`；互动雷达创建和删除会同步刷新 `radars` 用量；抽奖活动创建和删除会同步刷新 `lotteries` 用量，更新或删除抽奖活动也会回收奖品设置和中奖记录客服二维码里的本地素材账本并刷新 `storage_mb`；无限拉群创建和删除会同步刷新 `room_infinite_pulls` 用量，更新或删除无限拉群也会回收 `avatar`、`logo`、`qw_code` 活码 JSON 二维码字段对应的本地上传文件账本并刷新 `storage_mb`；群裂变创建和删除会同步刷新 `room_fissions` 用量；群打卡创建和删除会同步刷新 `room_clock_ins` 用量；群质检、群日历、客户群提醒、个人 SOP、群 SOP 和敏感词创建和删除会同步刷新 `room_qualities`、`room_calendars`、`room_reminds`、`contact_sops`、`room_sops` 和 `sensitive_words` 用量；`scripts/smoke_channel_code_cron.sh` 会验证渠道活码成功刷新和失败回滚两条链路，`scripts/smoke_saas_storage_reclaim.sh` 会验证渠道活码创建失败回滚时的欢迎语素材账本回收。
- 设置 `MOCHAT_GO_ENABLE_WORK_MESSAGE_ARCHIVE_SYNC_CRON=1`、`MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_BASE_URL=<url>` 后，Go standalone 会启动 `cron-work-message-archive-sync`，从内部会话存档 SDK bridge 拉取已解密消息，按企业微信 `seq` 写入 `mc_work_message_1` 至 `mc_work_message_10`，并使用 `mc_work_message_id.type=40` 维护拉取游标；可用 `MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_INTERVAL_SECONDS`、`MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_CRON_RUN_ON_START`、`MOCHAT_GO_WORK_MESSAGE_ARCHIVE_SYNC_LIMIT` 和 `MOCHAT_GO_WORK_MESSAGE_ARCHIVE_BRIDGE_TOKEN` 控制周期、启动即跑、每批条数和 bridge 鉴权。`scripts/smoke_work_message_archive_sync_cron.sh` 会用 fake SDK bridge + 真实 MySQL 验证入库、游标推进、重复 tick 幂等，并验证入库消息可被敏感词监控继续消费。
- 设置 `MOCHAT_GO_ENABLE_SENSITIVE_WORD_MONITOR_CRON=1` 后，Go standalone 会启动 `cron-sensitive-word-monitor`，扫描 `mc_work_message_1` 至 `mc_work_message_10` 的新增会话存档消息，按启用敏感词写入 `mc_sensitive_words_monitor`，并使用 `mc_work_message_id.type=21..30` 分表维护游标；`scripts/smoke_sensitive_word_monitor_cron.sh` 会用真实 MySQL 验证命中写入、游标推进和重复 tick 幂等。
- 客户群发、客户群群发、标签建群、自动拉群和裂变活动删除或失败回滚后，会刷新 `contact_message_batches`、`room_message_batches`、`room_tag_pulls`、`work_room_auto_pulls` 和 `work_fissions` 用量；`scripts/smoke_saas_storage_reclaim.sh` 会同时断言文件账本和业务资源计数下降。
- 公众号 `unauthorized` 取消授权和已有授权更新后会刷新 `official_accounts` 用量；取消授权账号会从公众号列表隐藏，且不再占用套餐名额。
- `cmd/mochat-saas-maintenance -action reconcile-storage` 可用当前上传目录校准 `mochat_go_saas_storage_objects`，把丢失文件和危险路径标记为软删除，修正文件大小，并刷新 `storage_mb` 用量；也可以在 Go standalone 中设置 `MOCHAT_GO_ENABLE_SAAS_STORAGE_RECONCILE_CRON=1` 启动 `cron-saas-storage-reconcile` 定时任务，默认每天执行一次，`MOCHAT_GO_SAAS_STORAGE_RECONCILE_CRON_RUN_ON_START=1` 会在进程启动后立即校准一次。`scripts/smoke_saas_storage_reconcile.sh` 会用真实 MySQL 验证手动命令和内置定时任务。
- `PUT /dashboard/medium/update` 替换素材文件路径时会回收旧路径账本，`DELETE /dashboard/medium/destroy` 软删素材时会回收当前路径账本，路径字段包括 `imagePath`、`voicePath`、`videoPath` 和 `filePath`；`POST /dashboard/channelCode/store` 在企业微信 contact_way 创建失败回滚时会解析 `welcome_message.messageDetail`，回收 `pic_url`、`imagePath`、`linkPic`、`coverPath` 等本地欢迎语素材账本；`PUT /dashboard/shopCode/update` 和 `POST /dashboard/shopCode/updateQrcode` 替换或移除门店活码 `employee_qrcode`、`qw_code` 本地二维码时会回收旧路径账本，`POST /dashboard/shopCode/pageSet` 替换或移除页面设置 `default` / `poster` 中的本地二维码或海报时会回收旧路径账本，`DELETE /dashboard/shopCode/destroy` 软删门店活码时会回收当前二维码账本；`PUT /dashboard/roomWelcome/update` 和 `DELETE /dashboard/roomWelcome/destroy` 会回收入群欢迎语 `msg_complex.pic` 本地图片账本；`DELETE /dashboard/contactMessageBatchSend/destroy` 和 `DELETE /dashboard/roomMessageBatchSend/destroy` 会回收群发内容里的本地 `pic_url` 图片账本；`DELETE /dashboard/roomTagPull/destroy` 会回收标签建群 `rooms.image` 本地图片账本；`PUT /dashboard/workRoomAutoPull/update` 替换或移除自动拉群 `rooms.roomQrcodeUrl` 本地群二维码时会回收旧路径账本，`POST /dashboard/workRoomAutoPull/store` 在企业微信 contact_way 创建失败回滚时也会回收已写入记录里的本地群二维码；`DELETE /dashboard/workFission/destroy` 会在级联软删裂变活动时回收 poster、welcome、push、invite 里的本地图片账本；`scripts/smoke_saas_storage_reclaim.sh` 会用真实 Go standalone 验证这些触发式回收链路。
- `POST /dashboard/roomFission/invite` 替换群裂变邀请封面时会回收旧邀请图账本，`DELETE /dashboard/roomFission/destroy` 会回收群裂变 poster、room、welcome、invite 对应的本地图片账本；`scripts/audit_saas_storage_reclaim_coverage.sh` 已接入 `scripts/test.sh`，会静态校验 26 个 store 回收函数仍由 `scripts/smoke_saas_storage_reclaim.sh` 覆盖。
- `cmd/mochat-saas-maintenance -action refresh-usage` 可在套餐配置变更、计数器缺失或历史数据导入后重刷 26 个 lifetime 用量指标和额度，包括门店活码数、互动雷达数、抽奖活动数、无限拉群数、群裂变数、群打卡数、群质检规则数、群日历数、客户群提醒数、个人 SOP 规则数、群 SOP 规则数和敏感词词库数；`scripts/smoke_saas_usage_refresh.sh` 会用真实 MySQL 验证脏计数器、缺失指标和新套餐额度都能被修正。
- `cmd/mochat-saas-maintenance -action list-alerts` 可按租户、指标、状态列出 `mochat_go_saas_alerts` 告警；`-action resolve-alert -tenant-id <id> -metric <metric>` 可把当前打开告警标记为 resolved；`-action dispatch-alert-notifications` 会读取 `mochat_go_saas_alert_notifications` 到期通知，优先按 `mochat_go_saas_alert_settings` 的租户级 webhook URL、签名、模板、HTTP retry、outbox retry、事件订阅、最低严重级别、免打扰和每小时限流策略处理，未配置租户项时再用 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL` 等全局环境变量兜底。0066 起租户 URL 与 Secret 可使用独立 AES-256-GCM 密钥环加密，`-action rotate-alert-credentials -tenant-id <id>` 可将旧明文或历史 Key 凭据批量改写到活动 Key。全局与租户级地址统一受出站安全策略约束：默认只允许 HTTPS 公网目标，DNS 结果与实际拨号绑定，环境代理关闭，重定向必须同源，私网/回环/链路本地/保留网段默认阻断；确需内网接收端时用 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_ALLOWED_CIDRS` 最小化放行，并仅在明确需要 HTTP 时将 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_REQUIRE_HTTPS=0`。云元数据地址不能放行。策略拒绝的通知写为 `suppressed`，免打扰或限流保持 `pending` 并推迟 `next_retry_at`，两者都不消耗发送尝试次数；dispatcher 会回写 delivered、deferred、suppressed、failed 或 dead 统计。设置 `MOCHAT_GO_ENABLE_SAAS_ALERT_NOTIFICATION_DISPATCH_CRON=1` 后，Go standalone 会启动 `cron-saas-alert-notification-dispatch`，按间隔自动扫描并重发 outbox 到期通知。设置 `MOCHAT_GO_ENABLE_SAAS_ALERT_DASHBOARD=1` 后，租户超管也可以访问 Go 自带的 `GET /dashboard/saasAlert/page` 管理页面，或用真实 dashboard JWT 调用 `GET /dashboard/saasAlert/index` 查看本租户告警，用 `PUT/POST /dashboard/saasAlert/resolve` 解决打开告警，用 `GET/PUT/POST /dashboard/saasAlert/setting` 查看和保存本租户通知策略。设置 `MOCHAT_GO_SAAS_ALERT_WEBHOOK_URL` 后，后台 worker 在租户未配置 webhook 时仍可用全局兜底发送 `saas.quota_alert` JSON webhook。
- 0067 起 `mc_corp` 与 `mc_work_agent` 的企微 Secret、回调 Token/AES Key 和会话存档 Secret 可使用独立 AES-256-GCM 密钥环加密。`cmd/mochat-saas-maintenance -action rotate-wecom-credentials -tenant-id <id>` 会将旧明文或历史 Key 凭据改写到活动 Key；总后台“企微凭据保护”展示企业、应用、旧明文、待轮换和不可用 Key 数量，不返回任何 Secret。
- `GET /dashboard/saasAdmin/notificationHealth` 仅平台租户超级管理员可用，按 1 至 720 小时窗口聚合跨租户通知送达健康度，区分健康、预警、严重和无数据租户，并返回送达成功率、待投递、延期、积压、失败、耗尽、关闭、抑制、延迟和 Top 失败原因；`GET /dashboard/saasAdmin/export?type=notificationHealth` 可按同一筛选导出租户健康 CSV。严重/预警租户同时进入统一运营待办的 `notification_health` 来源，可按负责人和复查时间认领；`POST/PUT /dashboard/saasAdmin/notificationHealthRecovery` 只对当前窗口明确为健康且至少存在一次成功送达的认领自动写入 `resolved` 关闭审计，无数据、仍异常或没有成功送达证据的租户保持打开，重复执行幂等。
- `GET /dashboard/saasAdmin/notificationSlo` 仅平台租户超级管理员可用，支持 1 至 90 天窗口、租户/套餐关键字、送达成功率目标、目标送达秒数和时延达标率目标；按通知 `created_at` cohort 补齐每日趋势并返回租户排行。成功率分母包含 `delivered/failed/dead/closed`，`pending/suppressed` 单列；`GET /dashboard/saasAdmin/export?type=notificationSlo` 导出同口径 summary/day/tenant CSV，普通租户访问返回 `403`。
- SaaS 总后台的订阅生命周期中心仅平台租户超级管理员可用：`GET /dashboard/saasAdmin/subscriptions` 查看 `trialing/active/grace/past_due/suspended/canceled` 存储状态与有效访问权，`GET /dashboard/saasAdmin/subscriptionEvents` 追溯状态事件，`POST/PUT /dashboard/saasAdmin/subscriptionTransition` 执行带乐观锁和幂等键的受控迁移，`POST/PUT /dashboard/saasAdmin/subscriptionReconcile` 支持 dry-run 与应用校准，`GET /dashboard/saasAdmin/export?type=subscriptions` 导出同口径 CSV。`trialing/active/grace` 允许访问，`past_due/suspended/canceled` 会同时拦截新登录和旧 token；无订阅记录的存量租户继续按旧套餐到期口径兼容。设置 `MOCHAT_GO_ENABLE_SAAS_SUBSCRIPTION_RECONCILE_CRON=1` 后启动 `cron-saas-subscription-reconcile`，默认每 5 分钟校准 500 条；`scripts/smoke_saas_subscription_lifecycle.sh` 会验证六态、权限、并发/幂等、开停联动、续费账单、校准 cron、CSV 和页面。
- SaaS 总后台的支付收款中心仅平台租户超级管理员可管理：`paymentOrders` 查订单与汇总，`paymentOrder` 幂等创单，`paymentOrderCancel` 按版本取消，`paymentDunning` 预演或应用催缴，`paymentWebhookEvents` 追溯回调，`export?type=paymentOrders` 导出 CSV。`POST /webhooks/saas/payment` 是与支付提供方解耦的中立入口，强制时间戳和 HMAC-SHA256 验签；成功回调在同一事务内结算订单、续费账单、套餐和订阅，失败回调进入 `payment_failed_reminder` 催缴 outbox。设置 `MOCHAT_GO_ENABLE_SAAS_PAYMENT_DUNNING_CRON=1` 后启动 `cron-saas-payment-dunning`；`scripts/smoke_saas_payment_collection.sh` 会验证签名、幂等、乐观锁、催缴、原子结算、访问恢复、CSV、页面和 cron。
- 退款中心同样仅平台租户超级管理员可管理：`paymentRefunds` 查询退款与汇总，`paymentRefund` 创建带幂等键的部分或全额退款申请，`paymentRefundCancel` 按版本取消尚未处理的申请，`export?type=paymentRefunds` 导出 CSV。`refund.processing/failed/canceled/succeeded` 继续复用签名回调入口；退款金额先在订单内原子预占，失败或取消释放预占，成功后同一事务写入 `refund` 账单事件、冲减净收入并更新订单退款累计。全额退款可显式选择保留、暂停或取消订阅，默认保留权益，避免部分退款静默停服；`scripts/smoke_saas_payment_refunds.sh` 覆盖并发超额、回调幂等、净收入和订阅访问控制。
- 发票中心仅平台租户超级管理员可受理和开具：`invoiceProfile` 代管租户资料，`invoiceDocuments` 查询蓝票/红票，`invoice`、`creditNote` 和 `invoiceTransition` 负责申请、红冲、处理、开具、失败与取消，`export?type=invoiceDocuments` 导出 CSV。设置 `MOCHAT_GO_ENABLE_SAAS_BILLING_PORTAL=1` 后，租户可从 `/dashboard/saasBilling/page` 查看本租户净收款、退款、可开票额与单据，自助维护开票资料、申请蓝票和取消待处理申请；所有租户 API 强制使用 JWT 租户归属。`scripts/smoke_saas_billing_invoices.sh` 覆盖金额预占、蓝红票净额、并发防超开、提供方单号唯一、事务回滚、租户隔离、页面与路由。
- 渠道结算中心仅平台租户超级管理员可管理：`paymentSettlementBatches` 和 `paymentSettlementEntries` 查询批次、差异与处理汇总，`paymentSettlementImport` 导入 JSON/CSV，`paymentSettlementReconcile` 预演或正式重对账，`paymentSettlementResolve` 解决、忽略或重开差异，`paymentSettlementTransition` 关账或重开批次，`export?type=paymentSettlementBatches/paymentSettlementEntries` 导出 CSV。收款和退款使用带符号金额，平台单号与渠道单号必须指向同一内部账本；待处理差异未清零时不能关账，关闭后必须重开才能修改。`scripts/smoke_saas_payment_settlements.sh` 覆盖导入幂等、差异分类、人工处理、乐观锁、事务回滚、CSV、页面与路由。
- 结算自动同步通过内部 Bridge 隔离支付渠道密钥。Go 固定请求 `GET {MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_BRIDGE_BASE_URL}/v1/payment-settlements?provider=...&cursor=...&limit=...`，可附带 Bearer token；Bridge 返回 `provider`、`nextCursor`、`hasMore` 和与 JSON 手工导入同结构的 `batches`。配置 `MOCHAT_GO_SAAS_PAYMENT_SETTLEMENT_PROVIDERS` 后，总后台可通过 `paymentSettlementSyncRuns` 查看渠道游标/运行历史，并通过 `paymentSettlementSync` 预演或立即同步；启用 `MOCHAT_GO_ENABLE_SAAS_PAYMENT_SETTLEMENT_SYNC_CRON=1` 后由 `cron-saas-payment-settlement-sync` 周期拉取。游标仅在整次运行成功后推进，部分导入靠来源摘要重试幂等；同渠道并发返回 `409`，超过 15 分钟的僵死运行可由新任务接管，失败和待处理差异写入通知 outbox。`scripts/smoke_saas_payment_settlement_sync.sh` 覆盖真实 MySQL、fake Bridge、游标、幂等、dry-run、并发、失败告警、task runner、权限、页面与路由。
- 平台总后台 RBAC 由 `0045_saas_admin_rbac` 至 `0055_saas_tenant_domains` 提供。平台超级管理员始终拥有隐式全权；其他平台成员通过六个内置角色或自定义角色获得 29 项租户、运营、通知、财务、审计、访问治理、审批、系统健康、集成、灾备、合规、身份安全、品牌和域名治理权限。`accessProfile` 返回当前授权快照，`accessRoles/accessRole` 管理角色，`accessAssignments/accessAssignment` 管理平台成员授权；内置角色不可修改，自定义角色和成员授权均使用乐观锁。合规管理自动依赖合规查看、审计查看和审批查看，身份安全、品牌与域名管理均自动依赖对应查看权限和审计查看。`scripts/smoke_saas_admin_access_rbac.sh` 覆盖即时授权、职责隔离、越权拒绝、版本冲突和审计日志。
- 服务账号 API Key 使用 `mch_live_<12 hex>_<43 base64url>` 格式，数据库只保存以独立 32 字节 pepper 计算的 HMAC-SHA256 摘要、`hash_key_id`、非敏感前缀和后四位。创建响应与批准后的轮换执行响应只显示一次明文，审批申请和持久化执行结果均不保存明文；账号配置变更、Key 轮换和吊销必须通过不可绕过的 critical 双人审批，直接调用返回 `428`。`platform.integrations.read/manage` 分离查看和管理，Scope、账号/密钥状态与过期时间、IP/CIDR 白名单和租户归属由服务端统一校验。总后台可查看 pepper 保护状态、待轮换旧 Key 和缺失密钥 ID，以及 7/30/90 天成功/拒绝趋势、账号排行和路由排行；合规策略统一控制保留天数，每个账号还可配置日用量百分比、拒绝数阈值和通知冷却。生产必须配置 `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPER` 或密钥环 `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_KEY_PEPPERS`，并启用 `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_REQUIRE_DEDICATED_PEPPER=1`。升级期可暂时保留 `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_ALLOW_LEGACY_JWT_PEPPER=1`，轮换完全部旧 Key 后设为 `0`；Dashboard JWT 轮换不影响已使用独立 pepper 的 Key。`scripts/smoke_saas_service_accounts.sh` 覆盖创建、独立/旧 pepper 鉴权切换、健康门禁、配置变更审批、Key 轮换与吊销审批、一次性明文交付、RBAC、限流、历史聚合、维护清理、用量预警、冷却去重、自动恢复、cron 账本和法律保留保护。
- 反向代理后的客户端 IP 必须使用受信边界：在 `MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS` 列出可信代理网段后，才可分别开启 `MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS` 和 `MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS`。只有 TCP 直连对端命中可信网段时才会从右向左解析 `X-Forwarded-For`，否则忽略所有转发头；总后台的身份安全与服务账号区域会显示当前解析模式和可信网段数量。
- 灾备中心使用 `platform.backups.read/manage` 分离查看和写操作。备份流程为 `mysqldump -> gzip -> AES-256-GCM -> 原子改名 -> S3 全流 SHA-256 校验`，本地工件权限为 `0600`，数据库只保存密钥 ID、摘要和非敏感对象位置。历史密钥环允许轮换后继续校验旧备份；本地工件缺失或损坏时可从已校验异地副本原子回源。恢复演练可使用预配置隔离空库，也可由管理 DSN 自动创建 `mochat_restore_` 前缀临时库并在校验后销毁；始终拒绝源库与非空固定库。`scripts/smoke_saas_backup_recovery.sh` 覆盖真实 MinIO、副本篡改检测与修复、跨密钥校验、回源、两类恢复、清理、RBAC、审计和健康检查。
- 租户数据合规中心使用 `platform.compliance.read/manage` 分离查看和管理。导出包含静态数据清单覆盖的 JSONL、manifest 和租户文件，再使用分块 AES-256-GCM 加密、SHA-256 校验和 `0600` 原子发布。擦除必须先停用租户、完整输入租户名、存在最近成功导出、没有活动法律保留、通过两人复核且财务保留期已满。执行按数据集持久化步骤恢复，删除业务数据和物理文件，脱敏必须保留的结算/审计证据，最终生成不可逆墓碑和验证 SHA-256。`scripts/smoke_saas_compliance_lifecycle.sh` 覆盖篡改拒绝、法律保留、审批、保留期阻断、恢复执行和物理擦除。
- 品牌与白标中心使用 `platform.branding.read/manage` 分离查看和管理。`brandingProfiles` 提供跨租户筛选，`brandingProfile` 提供带乐观锁的单租户查询和保存；平台档案同步安全登录页与总后台标题，租户档案通过同源 `/dashboard/external/tenantIndex` 回显到业务前端。logo、favicon、登录背景和支持二维码只允许 `/img/`、`/static/` 或 `/favicon.ico` 本地路径，不接受第三方远程资产；对外网站、客服和文档入口必须使用 HTTPS。
- 身份安全中心使用 `platform.identity.read/manage` 分离查看和处置。租户策略统一控制失败锁定、IP/CIDR、强制 MFA、会话 TTL/空闲超时/并发数和保留期；TOTP 密钥以 AES-256-GCM 密钥环加密，挑战、会话 token 和恢复码只保存不可逆摘要。总后台可解锁账号、撤销会话、重置 MFA、确认/分派/解决安全事故，所有写操作使用乐观锁并进入脱敏操作审计。`scripts/smoke_saas_identity_security.sh` 覆盖完整登录、MFA、持久会话、风险事件与清理闭环。

备份 cron 默认关闭。生产环境应从密钥管理系统注入 32 字节 base64 或 64 位 hex 密钥，不要把密钥写入仓库、数据库或命令行参数：

```bash
export MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY='<active-secret-manager-value>'
export MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEYS='{"primary-2026-q2":"<old-secret>","primary-2026-q3":"<active-secret>"}'
export MOCHAT_GO_SAAS_BACKUP_ENCRYPTION_KEY_ID='primary-2026-q3'
export MOCHAT_GO_SAAS_BACKUP_ROOT='/app/storage/backups'
export MOCHAT_GO_SAAS_BACKUP_RESTORE_ADMIN_DSN='restore_admin:<password>@tcp(mysql:3306)/?parseTime=true&loc=Local'
export MOCHAT_GO_SAAS_BACKUP_RESTORE_AUTO_PROVISION=1
export MOCHAT_GO_SAAS_BACKUP_RESTORE_DATABASE_PREFIX='mochat_restore_'
export MOCHAT_GO_SAAS_BACKUP_S3_ENDPOINT='https://s3.example.com'
export MOCHAT_GO_SAAS_BACKUP_S3_BUCKET='mochat-production-backups'
export MOCHAT_GO_SAAS_BACKUP_S3_ACCESS_KEY_ID='<secret-manager-access-key>'
export MOCHAT_GO_SAAS_BACKUP_S3_SECRET_ACCESS_KEY='<secret-manager-secret-key>'
export MOCHAT_GO_SAAS_BACKUP_S3_USE_SSL=1
export MOCHAT_GO_ENABLE_SAAS_BACKUP_CRON=1
export MOCHAT_GO_SAAS_BACKUP_CRON_INTERVAL_SECONDS=300
export MOCHAT_GO_SAAS_BACKUP_CRON_RUN_ON_START=0
```

cron 的扫描间隔不是备份周期；是否到期以数据库策略中的 `interval_minutes` 为准。总后台灾备中心会显示自动调度开关、扫描间隔和启动检查状态；持久化策略已启用但调度器未启用、扫描间隔无效或策略停用时，系统健康中心的 `backup_automation` 探针会按关键故障提示，不能只凭手工备份就判定自动灾备就绪。自动恢复管理账号只需要连接实例及创建、写入、销毁临时数据库所需权限，不应复用应用日常账号；若使用固定 `MOCHAT_GO_SAAS_BACKUP_RESTORE_DSN`，目标必须预先创建并在每次演练前保持空库。手工维护命令复用同一管理器和审计台账：

合规 cron 默认关闭，但只要启用 SaaS 总后台，合规查看和手动处理 API 就会注册。生产建议使用与备份分离的 32 字节密钥环；未配置独立密钥时会回退到备份密钥：

```bash
export MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEYS='{"compliance-2026-q3":"<active-secret>"}'
export MOCHAT_GO_SAAS_COMPLIANCE_ENCRYPTION_KEY_ID='compliance-2026-q3'
export MOCHAT_GO_SAAS_COMPLIANCE_ARTIFACT_ROOT='/app/storage/compliance'
export MOCHAT_GO_ENABLE_SAAS_COMPLIANCE_CRON=1
export MOCHAT_GO_SAAS_COMPLIANCE_CRON_INTERVAL_SECONDS=300
export MOCHAT_GO_SAAS_COMPLIANCE_CRON_RUN_ON_START=0
```

开启定时处理前必须确认 `MOCHAT_FILE_STORAGE_ROOT` 指向真实租户文件根目录，并将合规工件目录纳入主机权限、容量和密钥轮换运维。

身份安全默认关闭。生产应使用独立且稳定的密钥环；若未配置独立密钥会按合规、备份密钥顺序回退。打开持久会话强制检查前，应明确通知当前用户重新登录；只有受信入口代理会覆盖外部转发头时才允许信任代理来源 IP：

```bash
export MOCHAT_GO_ENABLE_SAAS_IDENTITY_SECURITY=1
export MOCHAT_GO_SAAS_IDENTITY_ENFORCE_SESSIONS=1
export MOCHAT_GO_SAAS_TRUSTED_PROXY_CIDRS='10.20.0.0/16,127.0.0.1/32'
export MOCHAT_GO_SAAS_IDENTITY_TRUST_PROXY_HEADERS=1
export MOCHAT_GO_SAAS_SERVICE_ACCOUNT_TRUST_PROXY_HEADERS=1
export MOCHAT_GO_SAAS_IDENTITY_ISSUER='MoChat Go'
export MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEYS='{"identity-2026-q3":"<active-secret>"}'
export MOCHAT_GO_SAAS_IDENTITY_ENCRYPTION_KEY_ID='identity-2026-q3'
export MOCHAT_GO_ENABLE_SAAS_IDENTITY_CLEANUP_CRON=1
export MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_INTERVAL_SECONDS=3600
export MOCHAT_GO_SAAS_IDENTITY_CLEANUP_CRON_RUN_ON_START=0
export MOCHAT_GO_SAAS_IDENTITY_CLEANUP_LIMIT=1000
```

租户通知凭据应使用独立密钥环。升级时先同时保留旧 Key 和活动 Key，执行轮换并确认总后台“旧明文、待轮换、不可用”均为 0，再开启强制加密并移除不再被引用的历史 Key：

```bash
export MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEYS='{"alert-2026-q2":"<old-secret>","alert-2026-q3":"<active-secret>"}'
export MOCHAT_GO_SAAS_ALERT_CREDENTIAL_ENCRYPTION_KEY_ID='alert-2026-q3'
export MOCHAT_GO_SAAS_ALERT_CREDENTIAL_REQUIRE_ENCRYPTION=1
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action rotate-alert-credentials -tenant-id 0 -alert-credential-rotation-limit 100
```

企业微信凭据使用另一套独立密钥环。保留仍被历史密文引用的 Key，轮换后确认总后台旧明文、待轮换、不可用均为 0，再开启强制加密：

```bash
export MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEYS='{"wecom-2026-q2":"<old-secret>","wecom-2026-q3":"<active-secret>"}'
export MOCHAT_GO_WECOM_CREDENTIAL_ENCRYPTION_KEY_ID='wecom-2026-q3'
export MOCHAT_GO_WECOM_CREDENTIAL_REQUIRE_ENCRYPTION=1
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action rotate-wecom-credentials -tenant-id 0 -wecom-credential-rotation-limit 100
```

微信开放平台 Ticket 与公众号授权凭据使用第三套独立密钥环。轮换会清空旧明文；移除历史 Key 前必须确认总后台旧明文、待轮换、不可用均为 0：

```bash
export MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEYS='{"wechat-open-2026-q2":"<old-secret>","wechat-open-2026-q3":"<active-secret>"}'
export MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_ENCRYPTION_KEY_ID='wechat-open-2026-q3'
export MOCHAT_GO_WECHAT_OPEN_CREDENTIAL_REQUIRE_ENCRYPTION=1
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action rotate-wechat-open-credentials -tenant-id 0 -wechat-open-credential-rotation-limit 100
```

```bash
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action backup-status
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action backup-create
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action backup-verify -backup-run-id <id>
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action backup-replicate -backup-run-id <id>
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action backup-restore-drill -backup-run-id <id>
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action backup-cleanup
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action compliance-status
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action compliance-process -compliance-limit 10
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action compliance-export-process -compliance-id <export-id>
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action compliance-erasure-process -compliance-id <request-id>
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action identity-cleanup -identity-cleanup-limit 1000
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action rotate-alert-credentials -tenant-id 0 -alert-credential-rotation-limit 100
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action rotate-wecom-credentials -tenant-id 0 -wecom-credential-rotation-limit 100
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action cleanup-service-account-usage -service-account-usage-cleanup-limit 10000
env -u GOROOT go run ./cmd/mochat-saas-maintenance -action verify-audit-integrity -audit-integrity-limit 100
```
- 获客前独立部署样例默认 `MOCHAT_GO_SAAS_ADMIN_APPROVAL_REQUIRED=0`，用于只有一名平台管理员的单人运营阶段，关键动作直接执行但仍写入平台操作审计。增加第二位平台管理员后必须改为 `1`；此时由数据库策略决定哪些动作进入双人审批，审批申请保存策略快照，达到法定票数后才允许执行，业务动作在同一事务写入副作用标记，租约恢复不会重放已提交动作。生产团队模式不应长期使用 `0`。
- 设置 `MOCHAT_GO_ENABLE_SAAS_APPROVAL_REMINDER_CRON=1` 后，Go standalone 会启动 `cron-saas-admin-approval-reminder`；默认每 5 分钟扫描 100 条到期待复核审批，按申请时的策略快照生成 `approval_sla_reminder` 通知 outbox。`MOCHAT_GO_SAAS_APPROVAL_REMINDER_CRON_RUN_ON_START=1` 可在启动时立即执行，手动 `approvalReminders` 与自动任务复用同一事务和幂等键，任务运行状态写入后台任务账本。
- 设置 `MOCHAT_GO_ENABLE_SAAS_SYSTEM_HEALTH_CRON=1` 后，Go standalone 会启动 `cron-saas-admin-system-health`；默认每 5 分钟扫描 MySQL/Redis、迁移版本、后台任务、通知积压、审批 SLA、运营任务和结算同步。新增、重开或升级事故写入 `system_health_incident` 通知 outbox，恢复后自动结案；手动和自动扫描复用同一事务状态机。`scripts/smoke_saas_admin_system_health.sh` 覆盖真实故障注入、去重、处置、重开、恢复、RBAC 和 task runner 持久化。
- `0062_saas_audit_integrity` 会在每个租户首次新增操作日志或执行校验时，为既有 legacy 日志计算固定锚点；后续操作日志按租户写入 SHA-256 结构摘要链。总后台 `platform.audit.read/manage` 分离查看和校验，保留期只做清理预估，不会自动删除审计数据。操作日志目标名称、前后快照和备注脱敏后，受保护结构字段的摘要仍保持可验证。设置 `MOCHAT_GO_ENABLE_SAAS_AUDIT_INTEGRITY_CRON=1` 后启动 `cron-saas-admin-audit-integrity`，默认每小时最多校验 100 个租户链。
- `0063_saas_audit_anchor_signatures` 使用独立环境密钥为健康链头生成 HMAC-SHA256 检查点。证据目录默认是 `/app/audit-anchors`，Compose 使用独立 `audit-anchor-storage` 卷；新检查点、历史密钥 ID、证据摘要和校验状态可在总后台审计区查看。当前合规清单为 158 项、可恢复擦除为 161 步。生产应把该卷映射到数据库管理员不可写的独立或 WORM 存储，并妥善保留历史密钥环；仅使用同机普通卷不能防同时删除数据库与证据文件。
- `0064_saas_audit_anchor_remote_immutability` 可把每个规范化签名锚点写入启用 Object Lock 的 S3 兼容桶。建议生产设置 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_REQUIRE_REMOTE=1`、使用 `compliance` 模式，并按审计政策设置留存天数；启动和每日任务会分批回填 0063 历史检查点。总后台、维护命令和系统健康会核对对象版本、SHA-256、大小、留存模式、到期时间、远端缺失与孤儿对象。访问密钥只从环境读取，不写入数据库或审计日志。
- 从数据库备份恢复且原 Object Lock 前缀仍保留更晚对象时，不得覆盖、删除或复用内容不一致的同名对象。应保留旧前缀作为回退证据，将 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_S3_PREFIX` 切换到新的恢复代际后重跑锚点回填，并分别留存旧、新前缀清单；同名前缀内容不一致会按设计阻断回填。
- 设置 `MOCHAT_GO_ENABLE_SAAS_AUDIT_ANCHOR_CRON=1` 后启动 `cron-saas-admin-audit-anchor`。必须配置 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY` 或 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEYS`；轮换时用 `MOCHAT_GO_SAAS_AUDIT_ANCHOR_HMAC_KEY_ID` 指向新密钥。维护命令 `create-audit-anchor` 和 `verify-audit-anchor` 可用于变更前后即时留证。
- 设置 `MOCHAT_GO_ENABLE_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON=1` 后，Go standalone 会启动 `cron-saas-operation-queue-assignment-reminder`；默认每小时按最新认领分别扫描逾期和 7 天内到期事项，生成 `operation_queue_assignment_reminder` 通知 outbox，并写入系统操作者审计。`MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_CRON_RUN_ON_START=1` 可在启动时立即执行，`MOCHAT_GO_SAAS_OPERATION_QUEUE_ASSIGNMENT_REMINDER_LIMIT` 默认每个到期状态扫描 500 条。该 cron 只生成 outbox，webhook 投递继续由通知 dispatch cron 执行；`scripts/smoke_saas_operation_queue_assignment_reminder_cron.sh` 会验证最新状态、关闭状态、重启幂等和 task runner 持久化。
- 设置 `MOCHAT_GO_ENABLE_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON=1` 后，Go standalone 会启动 `cron-saas-notification-health-recovery`；默认每 15 分钟按 24 小时健康窗口和 15 分钟积压阈值复查通知健康认领，只对存在成功送达证据的 `healthy` 租户写入系统操作者关闭审计。仍异常、无数据或没有成功送达证据的认领保持打开；可用 `MOCHAT_GO_SAAS_NOTIFICATION_HEALTH_RECOVERY_CRON_RUN_ON_START=1` 启动即跑，并通过窗口和积压阈值变量调整判定口径。`scripts/smoke_saas_notification_health_recovery_cron.sh` 会在空库完整迁移后验证四类租户、重启幂等和 task runner 持久化。
- `scripts/lint_mysql57_schema.sh` 会静态扫描独立 schema 和增量迁移，拦截 MySQL 8 专属语法和 MySQL 5.7 不兼容 JSON 默认值。
- `scripts/smoke_mysql57_schema_migrate.sh` 会使用 `mysql:5.7` 真实执行初始 schema、增量迁移和 rollback，作为生产 MySQL 5.7 兼容门禁；在 arm64 本机默认跳过真实容器 smoke，避免 `mysql:5.7` amd64 镜像在 qemu 下初始化段错误，amd64 CI 或 `MOCHAT_FORCE_MYSQL57=1` 会执行真实容器验收。
- `EmployeeApply`、`wework-callback`、`contact-welcome`、`async-file-upload`、`mark-tags`、`message-remind` 和 `work-room-sync` Redis 队列 envelope 和幂等 key 可在真实 Redis 中正常入队、去重、出队和 ack；`contact-welcome` 会在启用 `MOCHAT_GO_ENABLE_WEWORK_CALLBACK_WORKER=1` 时随企微回调 worker 一起启动，用于接管 PHP 欢迎语异步发送队列，新增客户回调已覆盖渠道码专属欢迎语优先、自动拉群 `State=workRoomAutoPullId-{id}` 欢迎语和通用欢迎语兜底；`async-file-upload` 可通过 `MOCHAT_GO_ENABLE_ASYNC_FILE_UPLOAD_WORKER=1` 接管 PHP `AsyncFileUpload` 文件异步上传队列，支持 URL 和本地临时文件写入 `MOCHAT_FILE_STORAGE_ROOT` 下目标相对路径，并已用双租户 smoke 验证 `tenantId/corpId` payload 只刷新当前租户的执行量、存储账本和超额告警；`mark-tags` 可通过 `MOCHAT_GO_ENABLE_MARK_TAGS_WORKER=1` 接管 PHP `MarkTags` 客户打标签队列，写入本地 pivot/轨迹并调用企业微信 `externalcontact/mark_tag`；自动标签关键词任务在启用 `MOCHAT_GO_ENABLE_MARK_TAGS_WORKER=1` 时会把命中的 `mc_auto_tag_record` 投递到同一 `mark-tags` 队列，并支持 pending 记录重投递；`message-remind` 可通过 `MOCHAT_GO_ENABLE_MESSAGE_REMIND_WORKER=1` 接管 PHP `MessageRemind` 消息提醒队列，读取 `mc_work_agent` 提醒应用并调用企业微信 `message/send`；`work-room-sync` 可通过 `MOCHAT_GO_ENABLE_WORK_ROOM_SYNC_WORKER=1` 接管 PHP `UpdateApply/UpdateCallback` 客户群同步队列，拉取企业微信 `groupchat/list/get` 并写入客户群和群成员；渠道码、自动拉群和企微裂变新增客户打标签已覆盖本地 pivot、互动轨迹和企业微信 `externalcontact/mark_tag` 同步。
- `mochat_go_background_tasks`、`mochat_go_background_task_runs` 和 `mochat_go_background_task_executions` 已纳入 `0005_background_tasks` 迁移；`mochat_go_saas_alerts` 已纳入 `0006_saas_alerts` 迁移；`mochat_go_saas_alert_notifications` 已纳入 `0008_saas_alert_notifications` 迁移；`mochat_go_saas_alert_settings` 已纳入 `0023_saas_alert_settings` 迁移。启用后台任务时 recorder 也会兼容旧库自动补表、补列和补索引，并记录任务当前状态、执行实例、cron tick 历史和带租户归属的 Redis worker queue item 历史；queue item 结束后会刷新 `async_executions` 运行时计数器，超额时写入日志和 SaaS 告警账本，可选发送 webhook，不阻断企微回调处理。`scripts/smoke_employee_apply_worker.sh` 已覆盖 fake webhook、模板化标题正文、真实 JWT 查询 dashboard 告警列表、维护命令解决告警和 outbox 到期通知重发；`scripts/smoke_saas_alert_notification_cron.sh` 已覆盖内置 cron 自动重发 outbox 到期通知和后台任务执行历史；`scripts/smoke_saas_alert_setting_dispatch.sh` 已覆盖无全局 webhook URL 时按租户配置表投递签名 webhook。
- Redis `PING` 正常。
- Go standalone `/readyz` 返回 200。
- `/compat/routes` 使用内置清单。
- 未迁移路由返回 501，而不是 PHP fallback。
- dashboard、sidebar、operation 三个前端入口都由 Go 项目内置 `web/*/dist` 产物托管；主监听地址根路径继续托管 dashboard，同时提供 `/sidebar-app/` 和 `/operation-app/` 前缀入口来加载 sidebar/operation，静态托管层会重写旧 dist 的根路径 JS/CSS、webpack publicPath 和 Vue Router base，避免与 dashboard 根资源冲突。`scripts/smoke_frontend_static_browser.sh` 会用真实浏览器验证主端口前缀入口、独立前端监听入口和同源 JS/CSS 资源均可从独立 Go 服务加载。
- dashboard 登录和业务页浏览器验收由 `scripts/smoke_dashboard_frontend_login.sh` 覆盖，脚本会访问 `/corpData/index`、`/corp/index`、`/user/index`、`/passwordUpdate/index`、`/role/index`、`/role/permissionShow?roleId=910001`、`/menu/index`、`/department/index`、`/workEmployee/index`、`/workContact/index`、`/workContact/contactFieldPivot?contactId=910001&employeeId=2&isContact=1`、`/lossContact/index`、`/workContactTag/index`、`/workRoom/index`、`/workRoom/detail?workRoomId=910001`、`/workRoom/statistics?workRoomId=910001`、`/channelCode/index`、`/channelCode/statistics?channelCodeId=910001`、`/channelCode/store`、`/mediumGroup/index`、`/greeting/index`、`/greeting/store`、`/roomWelcome/index`、`/roomWelcome/create`、`/workRoomAutoPull/index`、`/workRoomAutoPull/store`、`/roomTagPull/index`、`/roomTagPull/create`、`/roomTagPull/detail?id=917001`、`/roomTagPull/contactDetail?id=917001`、`/autoTag/keywordIndex`、`/autoTag/keywordCreate`、`/autoTag/keywordShow?idRow=918001`、`/autoTag/joinRoomIndex`、`/autoTag/joinRoomCreate`、`/autoTag/joinRoomShow?idRow=918002`、`/autoTag/dayPartIndex`、`/autoTag/dayPartCreate`、`/autoTag/dayPartShow?idRow=918003`、`/contactMessageBatchSend/index`、`/contactMessageBatchSend/store`、`/contactMessageBatchSend/show?batchId=915001`、`/roomMessageBatchSend/index`、`/roomMessageBatchSend/store`、`/roomMessageBatchSend/show?batchId=916001`、`/workFission/taskpage`、`/workFission/create`、`/workFission/edit?id=985001`、`/workFission/invite?id=985001`、`/workFission/dataShow?id=985001`、`/officialAccount/index`、`/officialAccount/create`、`/contactField/index`、`/chatTool/customer`、`/chatTool/enhance`、`/statistics/contact`、`/statistics/employee`、`/contactTransfer/resignIndex`、`/contactTransfer/workIndex`、`/contactTransfer/workAllotRecord` 和 `/contactTransfer/resignAllotRecord` 共 61 个 dashboard 页面，并额外访问 Go 原生 `/dashboard/sensitiveWords/page` 敏感词管理页、`/dashboard/lottery/page` 抽奖活动页、`/dashboard/radar/page` 互动雷达页、`/dashboard/shopCode/page` 门店活码页、`/dashboard/contactSop/page` 个人 SOP 页、`/dashboard/roomSop/page` 群 SOP 页、`/dashboard/roomFission/page` 群裂变页、`/dashboard/roomQuality/page` 群质检页、`/dashboard/roomCalendar/page` 群日历页、`/dashboard/roomRemind/page` 客户群提醒页、`/dashboard/roomInfinitePull/page` 无限拉群页、`/dashboard/roomClockIn/page` 群打卡页和 `/dashboard/saasAlert/page` SaaS 告警管理页，等待对应 Go dashboard API 返回 200，并把页面渲染到 `/404`、同源接口、非导航取消类本地请求失败或本地静态资源的 4xx/5xx 视为失败。
- sidebar 登录、客户详情页、个人 SOP、群 SOP、批量加好友和素材库浏览器验收由 `scripts/smoke_sidebar_frontend_contact.sh` 覆盖，脚本会在无 PHP、无原 `mochat/` 源码、无外部 manifest 的独立环境里先通过 `/login?agentId=1&target=/contact?agentId=1` 验证旧 dist、Go `/sidebar/agent/auth`、fake 企业微信 OAuth code 回调和前端 `/auth` 写 cookie 链路，再访问 `/contact?agentId=1`、`/contactSop?id=900001&agentId=1`、`/roomSop?id=900001`、`/contactBatchAdd?batchId=900001` 与 `/medium?agentId=1`，验证企微 JSSDK 配置、客户详情、客户画像、互动轨迹、个人 SOP 提醒弹窗、个人 SOP 详情、群 SOP 详情、群 SOP 完成状态写库、批量加好友分配手机号、素材分组、素材列表和本地静态资源。Go runtime 同时兼容旧 dist 里可能出现的 `/undefined/sidebar/*` API 前缀；内置 sidebar dist 已补齐个人 SOP 详情页初始空 `task/contact` 结构，避免页面加载期出现 `task.content` 运行时错误。
- operation 任务宝浏览器验收由 `scripts/smoke_operation_frontend_work_fission.sh` 覆盖，脚本会在无 PHP、无原 `mochat/` 源码、无外部 manifest 的独立环境里启动 fake 微信开放平台 API，先验证 `/auth/workFission` 公众号 OAuth 外跳参数，再用 code 回调写入 Go operation session，随后访问 `/workFission?id=900001` 和 `/speed`，验证 `openUserInfo/workFission` 同源别名、海报、任务数据、邀请好友、领奖写入和本地静态资源。Go runtime 同时兼容旧 dist 里可能出现的 `/undefined/operation/*` API 前缀。
