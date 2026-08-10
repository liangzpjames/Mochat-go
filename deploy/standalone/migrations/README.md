# Standalone 增量迁移规范

`cmd/mochat-migrate` 会先执行 `deploy/standalone/schema/mochat.sql` 作为 `0001_initial_schema`，再按文件名顺序发现并执行本目录中的增量迁移。当前 `0002_seed_core_data.up.sql` 是独立版核心 seed，负责写入默认客户资料字段、侧边栏工具和 RBAC 菜单数据；`0003_saas_provisioning.up.sql` 负责创建 Go 独立版 SaaS 元数据表；`0004_saas_storage_objects.up.sql` 负责创建 Go 独立版上传文件账本；`0005_background_tasks.up.sql` 负责创建 Go 独立版后台任务账本；`0006_saas_alerts.up.sql` 负责创建 Go 独立版 SaaS 告警账本；`0007_wechat_component_tickets.up.sql` 负责创建 Go 独立版微信开放平台 `component_verify_ticket` 账本；`0008_saas_alert_notifications.up.sql` 负责创建 Go 独立版 SaaS 告警通知 outbox；`0009_contact_batch_add_rbac.up.sql` 负责补齐批量加好友后台菜单和接口权限；`0010_sensitive_words.up.sql` 负责创建敏感词词库、分组和触发监控表；`0011_sop_rbac.up.sql` 负责确保个人 SOP/群 SOP 主表和触达日志表存在，并补齐后台接口权限；`0012_shop_code.up.sql` 负责创建门店活码业务表并补齐页面菜单和接口权限；`0013_radar.up.sql` 负责创建互动雷达业务表并补齐页面菜单和接口权限；`0014_auto_tag.up.sql` 负责创建自动标签和消息存档业务表、补齐会话存档配置字段并写入页面菜单和接口权限；`0015_lottery.up.sql` 负责创建抽奖活动业务表并补齐页面菜单和接口权限；`0016_room_fission.up.sql` 负责创建群裂变业务表并补齐页面菜单和接口权限；`0017_room_clock_in.up.sql` 负责创建群打卡业务表并补齐页面菜单和接口权限；`0018_room_quality.up.sql` 负责创建群质检业务表并补齐页面菜单和接口权限；`0019_room_calendar.up.sql` 负责创建群日历业务表并补齐页面菜单和接口权限；`0020_room_remind.up.sql` 负责创建客户群提醒业务表并补齐页面菜单和接口权限；`0021_room_infinite_pull.up.sql` 负责创建无限拉群业务表并补齐页面菜单和接口权限；`0022_work_message_archive_sync.up.sql` 负责扩展会话存档游标并增加消息分表同步索引；`0023_saas_alert_settings.up.sql` 负责创建租户级 SaaS 告警通知配置表；`0024_saas_package_extended_limits.up.sql` 负责补齐 SaaS 套餐表的扩展额度列；`0025_saas_radar_limit.up.sql` 负责补齐互动雷达 SaaS 套餐额度列；`0026_saas_lottery_limit.up.sql` 负责补齐抽奖活动 SaaS 套餐额度列；`0027_saas_room_infinite_pull_limit.up.sql` 负责补齐无限拉群 SaaS 套餐额度列；`0028_saas_room_fission_limit.up.sql` 负责补齐群裂变 SaaS 套餐额度列；`0029_saas_room_clock_in_limit.up.sql` 负责补齐群打卡 SaaS 套餐额度列；`0030_saas_room_operation_limits.up.sql` 负责补齐群质检、群日历和客户群提醒 SaaS 套餐额度列；`0031_saas_sop_limits.up.sql` 负责补齐个人 SOP 和群 SOP SaaS 套餐额度列；`0032_saas_sensitive_word_limit.up.sql` 负责补齐敏感词词库 SaaS 套餐额度列；`0033_saas_admin_operation_logs.up.sql` 负责创建 SaaS 总后台操作日志表；`0034_saas_billing_events.up.sql` 负责创建 SaaS 续费账单事件表；`0035_saas_admin_tasks.up.sql` 负责创建 SaaS 总后台运营任务表；`0036_saas_notification_policy_controls.up.sql` 负责扩展租户通知策略的事件订阅、最低级别、免打扰时段和每小时限流字段，并增加投递窗口索引。

`0037_saas_notification_health_index.up.sql` 为跨租户通知送达健康度窗口增加租户、通道、创建时间和状态联合索引。

`0038_saas_notification_slo_index.up.sql` 为平台级通知 SLO 趋势增加通道、创建日期、租户、状态和送达时间联合索引。

`0039_saas_subscription_lifecycle.up.sql` 创建租户当前订阅与状态事件表，并从已有租户套餐回填首个订阅快照和迁移事件。

`0040_saas_payment_collection.up.sql` 创建 SaaS 收款订单与支付回调事件账本，支持幂等收款、催缴和结算审计。

`0041_saas_payment_refunds.up.sql` 创建 SaaS 退款账本，并扩展支付订单与回调事件的退款金额、退款单和关联索引，支持部分/全额退款、预占、防超退和财务冲销。

`0042_saas_billing_invoices.up.sql` 创建租户开票资料和蓝票/红票单据账本，并扩展支付订单的开票预占、已开票、已红冲金额，支持租户自助申请、平台受理、原子防超开和财务净额校验。

`0043_saas_payment_settlements.up.sql` 创建支付渠道结算批次和明细账本，支持原始文件摘要幂等、支付/退款流水匹配、差异分类、人工处理、乐观锁重对账和关账审计。

`0044_saas_payment_settlement_sync.up.sql` 创建支付结算同步渠道状态和运行记录，支持 Bridge 增量游标、单渠道运行锁、僵死任务接管、dry-run、运行统计和失败审计。

`0045_saas_admin_rbac.up.sql` 创建平台总后台角色、角色权限、成员授权状态和成员角色绑定，内置平台运营、平台财务、客户成功、平台审计和只读观察员五类不可修改角色，并为自定义角色与成员授权提供乐观锁版本。

`0046_saas_admin_approvals.up.sql` 创建高风险审批单和不可变审批事件表，新增审批查看、复核、执行权限及平台审批人内置角色，并将退款、租户停用、结算关账、平台角色和成员授权变更纳入双人复核执行。

`0047_saas_admin_approval_governance.up.sql` 创建持久化审批策略、不可变逐票决定和审批委托表，扩展审批单的策略快照、法定票数、SLA 与提醒状态，并新增审批治理权限。

`0048_saas_admin_system_health.up.sql` 创建平台健康扫描和系统事故台账，保存检查快照、事故聚合状态、负责人、处置结论和乐观锁版本，并新增系统健康查看与处置权限。

`0049_saas_service_accounts.up.sql` 创建租户服务账号和 API Key 台账，保存 Scope、IP/CIDR 白名单、过期时间、密钥状态、轮换宽限和使用摘要，并新增平台集成查看与管理权限。API Key 明文只在创建或轮换响应中显示一次，数据库只保存 HMAC-SHA256 摘要、非敏感前缀和后四位；0049 最初使用 `SimpleJWTSecret` 作为 pepper，当前已由 0065 的独立 pepper 密钥环替代。

`0050_saas_backup_recovery.up.sql` 创建全局备份策略、备份运行和隔离恢复演练台账，保存密钥 ID、工件 SHA-256、迁移/表元数据、完整性校验、恢复目标指纹和审计关联，不保存加密密钥或恢复 DSN。运行表和演练表使用唯一活动槽阻止并发，并新增 `platform.backups.read/manage` 灾备查看与管理权限。

`0051_saas_backup_resilience.up.sql` 为灾备策略增加强制异地副本开关，为备份运行增加 S3 兼容副本位置、版本、SHA-256、大小、上传与校验状态，为恢复演练增加目标生命周期和销毁结果。该迁移不保存对象存储密钥、备份密钥或恢复管理 DSN。

`0052_saas_compliance_lifecycle.up.sql` 创建租户数据合规策略、法律保留、加密导出、擦除请求、持久化擦除步骤和不可逆租户墓碑六张表，新增 `platform.compliance.read/manage` 权限，并写入需要两人复核的 `tenant.data.erase` 审批策略。数据库只保存密钥 ID、工件摘要和审计证据，不保存合规导出密钥。

`0053_saas_identity_security.up.sql` 创建租户身份安全策略、用户登录安全状态、TOTP/恢复码凭据、二次认证挑战、可撤销登录会话、不可变登录事件和身份安全事故七张表，并新增 `platform.identity.read/manage` 权限。数据库不保存 MFA 明文密钥、恢复码、会话 token 或登录手机号明文事件索引。

`0054_saas_branding_profiles.up.sql` 创建平台与租户品牌白标档案，保存产品名称、本地资产路径、颜色、支持与文档入口、状态和乐观锁版本，并新增 `platform.branding.read/manage` 权限。该迁移不保存二进制图片、许可声明替换值或任何第三方资产访问凭据。

`0055_saas_tenant_domains.up.sql` 创建租户自定义域名台账，保存 DNS TXT 校验令牌、验证状态、主域名和乐观锁版本，并新增 `platform.domains.read/manage` 权限。域名必须先通过 `_mochat.<hostname>` TXT 所有权校验，才能参与按 Host 的登录品牌与账号租户路由。

`0056_saas_domain_delivery.up.sql` 新增域名路由/TLS 当前状态、持久化交付任务和签名回调事件三张表。活动任务以 `active_domain_id` 唯一键保证单域名串行交付，回调以 `event_id` 幂等；只保存非敏感证书引用和有效期，不保存证书 PEM 或私钥。

`0057_saas_release_readiness.up.sql` 创建六类生产发布证据台账和不可变发布候选门禁快照，要求真实 HTTPS 证据、执行环境、复核人和源码 SHA-256 指纹完整一致后才生成 `ready` 候选；新增 `platform.release.read/manage` 权限，不保存凭据或密钥。

`0058_saas_release_evidence_integrity.up.sql` 为每项发布证据增加工件 SHA-256 与字节大小；已通过证据只有同时固化真实地址、源码指纹和非空工件身份才算完整，审计和不可变候选快照会同步保留这两个字段。

`0059_saas_service_account_rate_limits.up.sql` 为服务账号增加每分钟和每日成功请求限额、当前窗口用量与拒绝计数，并创建按自然日和规范化路由聚合的 OpenAPI 用量台账。

`0060_saas_service_account_usage_retention.up.sql` 在租户数据合规策略中增加服务账号 OpenAPI 日用量保留天数，默认 90 天，供总后台、维护命令和定时清理共用。

`0061_saas_service_account_usage_alerts.up.sql` 为服务账号增加 OpenAPI 日用量和限流拒绝预警策略、冷却及评估/通知时间戳，供统一告警和通知 outbox 评估使用。

`0062_saas_audit_integrity.up.sql` 为总后台操作审计增加按租户维护的 SHA-256 结构摘要链、legacy 日志锚点、校验记录和审计治理权限；敏感业务快照仍可按合规策略脱敏，结构字段篡改、删除或断链可被发现。两张完整性治理表已进入应用层租户合规数据清单，以审计留存动作参与加密导出和可恢复擦除编排。

`0063_saas_audit_anchor_signatures.up.sql` 增加审计链 HMAC-SHA256 签名检查点表。签名载荷固定记录 legacy 锚点、签名链头、算法、密钥 ID 和 UTC 签名时间；密钥只从运行环境读取，独立 JSON 证据不保存在数据库中。检查点按审计策略保留并进入租户合规数据清单。

`0064_saas_audit_anchor_remote_immutability.up.sql` 为签名检查点增加 S3 Object Lock 异地证据状态、对象键、版本 ID、ETag、SHA-256、大小、留存模式和到期时间。对象内容仍以 0063 的规范化 JSON 为准，数据库只保存定位与校验元数据，不保存访问密钥。

文件命名规则：

```text
0023_some_change.up.sql
0023_some_change.down.sql
```

要求：

- 每个 `.up.sql` 必须有同名 `.down.sql`。
- 版本号和描述来自 `.up.sql` 去掉后缀后的文件名，例如 `0023_add_indexes.up.sql` 的版本是 `0023_add_indexes`。
- `rollback` 只回滚最后一条已应用且存在 `.down.sql` 的增量迁移。
- `0001_initial_schema` 只负责建表，没有 down 脚本，禁止 rollback，避免误删生产库。
- `0002_seed_core_data` 是核心默认数据 seed，使用 `INSERT IGNORE` 写入，便于旧环境从一体化 `0001_initial_schema` 过渡时补记 seed 迁移；它带有只删除对应 seed 主键的 down 脚本，用于测试和早期环境回滚，生产环境回滚前要确认没有把这些默认菜单或字段改成业务自定义数据。
- `0003_saas_provisioning` 只创建 `mochat_go_` 前缀的 SaaS 元数据表，记录套餐、租户套餐、用量计数、seed 版本和租户开通记录，不改原 MoChat 业务表。
- `0004_saas_storage_objects` 只创建 `mochat_go_` 前缀的上传文件账本，记录上传入口、租户、用户、员工、企业、相对路径、MIME 和字节数，用于 `storage_mb` 套餐额度计算，不改原 MoChat 业务表。
- `0005_background_tasks` 只创建 `mochat_go_` 前缀的后台任务账本，记录任务状态、运行实例和带租户归属的执行历史，用于 `async_executions` 套餐额度计算，不改原 MoChat 业务表。
- `0006_saas_alerts` 只创建 `mochat_go_` 前缀的 SaaS 告警账本，按租户、指标、周期聚合运行时额度超限信号，不改原 MoChat 业务表。
- `0007_wechat_component_tickets` 只创建 `mochat_go_` 前缀的微信开放平台 ticket 账本，保存 `authEventCallback` 周期推送的 `component_verify_ticket`，用于公众号预授权、授权回调、网页 OAuth 和开放平台测试消息链路，不改原 MoChat 业务表。
- `0008_saas_alert_notifications` 只创建 `mochat_go_` 前缀的 SaaS 告警通知 outbox，记录 webhook 通知状态、尝试次数、失败原因和下次重试时间，不改原 MoChat 业务表。
- `0009_contact_batch_add_rbac` 只写入 `mc_rbac_menu` 的批量加好友后台页面和接口权限，不改业务数据。
- `0010_sensitive_words` 创建 `mc_sensitive_word_group`、`mc_sensitive_word` 和 `mc_sensitive_words_monitor`，用于敏感词词库、分组和触发监控业务数据。
- `0011_sop_rbac` 确保 `mc_contact_sop`、`mc_contact_sop_log`、`mc_room_sop`、`mc_room_sop_log` 存在，并写入 `mc_rbac_menu` 的个人 SOP 和群 SOP 后台接口权限；对应 down 只删除权限 seed，不删除业务表。
- `0012_shop_code` 创建 `mc_shop_code`、`mc_shop_code_page`、`mc_shop_code_record`，并写入 `mc_rbac_menu` 的门店活码页面菜单和后台接口权限。
- `0013_radar` 创建 `mc_radar`、`mc_radar_channel`、`mc_radar_channel_link`、`mc_radar_record`，并写入 `mc_rbac_menu` 的互动雷达页面菜单和后台接口权限。
- `0014_auto_tag` 创建 `mc_auto_tag`、`mc_auto_tag_record`、`mc_work_message_id` 和 `mc_work_message_1` 至 `mc_work_message_10`，补齐 `mc_corp` 会话存档配置字段，并写入 `mc_rbac_menu` 的自动标签、消息存档页面菜单和后台接口权限。
- `0015_lottery` 创建 `mc_lottery`、`mc_lottery_contact`、`mc_lottery_contact_record`、`mc_lottery_prize`，并写入 `mc_rbac_menu` 的抽奖活动页面菜单和后台接口权限。
- `0016_room_fission` 创建 `mc_room_fission`、`mc_room_fission_contact`、`mc_room_fission_invite`、`mc_room_fission_poster`、`mc_room_fission_room`、`mc_room_fission_welcome`，并写入 `mc_rbac_menu` 的群裂变页面菜单和后台接口权限。
- `0017_room_clock_in` 创建 `mc_room_clock_in`、`mc_room_clock_in_contact`、`mc_room_clock_in_record`，并写入 `mc_rbac_menu` 的群打卡页面菜单和后台接口权限。
- `0018_room_quality` 创建 `mc_room_quality`、`mc_room_quality_contact`，并写入 `mc_rbac_menu` 的群质检页面菜单和后台接口权限。
- `0019_room_calendar` 创建 `mc_room_calendar`、`mc_room_calendar_push`、`mc_room_calendar_record`，并写入 `mc_rbac_menu` 的群日历页面菜单和后台接口权限。
- `0020_room_remind` 创建 `mc_room_remind`、`mc_room_remind_record`，并写入 `mc_rbac_menu` 的客户群提醒页面菜单和后台接口权限。
- `0021_room_infinite_pull` 创建 `mc_room_infinite`，并写入 `mc_rbac_menu` 的无限拉群页面菜单和后台接口权限。
- `0022_work_message_archive_sync` 将 `mc_work_message_id.last_id` 扩展为 bigint，并为 `mc_work_message_1` 至 `mc_work_message_10` 增加 `corp_id+msgid`、`corp_id+seq` 索引，用于 Go 会话存档同步 cron 的游标和幂等入库。
- `0023_saas_alert_settings` 只创建 `mochat_go_` 前缀的租户级 SaaS 告警通知配置表，记录 webhook 开关、URL、签名密钥、模板、HTTP 重试和 outbox 重试策略，不改原 MoChat 业务表。
- `0024_saas_package_extended_limits` 只扩展 `mochat_go_saas_packages` 的套餐额度列，补齐 `channel_codes`、`shop_codes`、群发任务、标签建群、自动拉群、裂变、公众号和异步执行量额度，不改原 MoChat 业务表。
- `0025_saas_radar_limit` 只扩展 `mochat_go_saas_packages.radars` 额度列，用于互动雷达数量限制，不改原 MoChat 业务表。
- `0026_saas_lottery_limit` 只扩展 `mochat_go_saas_packages.lotteries` 额度列，用于抽奖活动数量限制，不改原 MoChat 业务表。
- `0027_saas_room_infinite_pull_limit` 只扩展 `mochat_go_saas_packages.room_infinite_pulls` 额度列，用于无限拉群数量限制，不改原 MoChat 业务表。
- `0028_saas_room_fission_limit` 只扩展 `mochat_go_saas_packages.room_fissions` 额度列，用于群裂变数量限制，不改原 MoChat 业务表。
- `0029_saas_room_clock_in_limit` 只扩展 `mochat_go_saas_packages.room_clock_ins` 额度列，用于群打卡数量限制，不改原 MoChat 业务表。
- `0030_saas_room_operation_limits` 只扩展 `mochat_go_saas_packages.room_qualities`、`room_calendars`、`room_reminds` 额度列，用于群质检、群日历和客户群提醒数量限制，不改原 MoChat 业务表。
- `0031_saas_sop_limits` 只扩展 `mochat_go_saas_packages.contact_sops`、`room_sops` 额度列，用于个人 SOP 和群 SOP 规则数量限制，不改原 MoChat 业务表。
- `0032_saas_sensitive_word_limit` 只扩展 `mochat_go_saas_packages.sensitive_words` 额度列，用于敏感词词库数量限制，不改原 MoChat 业务表。
- `0033_saas_admin_operation_logs` 只创建 `mochat_go_` 前缀的 SaaS 总后台操作日志表，记录套餐维护、平台开户、租户套餐分配、租户开停等平台运营动作，不改原 MoChat 业务表。
- `0034_saas_billing_events` 只创建 `mochat_go_` 前缀的 SaaS 账单事件表，记录租户续费、金额、订单号、支付时间、前后到期时间和操作人，不改原 MoChat 业务表。
- `0035_saas_admin_tasks` 只创建 `mochat_go_` 前缀的 SaaS 总后台运营任务表，记录任务类型、状态、请求参数、预览结果、执行结果、应用时间和操作人，用于套餐快照同步、租户续费、平台开户等平台运营动作的二次确认与留痕，不改原 MoChat 业务表。
- `0036_saas_notification_policy_controls` 扩展 `mochat_go_saas_alert_settings` 的事件订阅、最低严重级别、免打扰时段、IANA 时区和每小时限流字段，并为 `mochat_go_saas_alert_notifications` 增加租户通道投递窗口索引；默认值保持升级前的全类型、warning 及以上、不启用免打扰和不限流行为。
- `0037_saas_notification_health_index` 只为 `mochat_go_saas_alert_notifications` 增加健康度窗口联合索引，支持按租户、通道和创建时间聚合送达率、积压、延期、失败、耗尽、关闭和策略抑制，不改写通知业务数据。
- `0038_saas_notification_slo_index` 只为 `mochat_go_saas_alert_notifications` 增加平台级 SLO 窗口联合索引，支持按通道、通知创建日期、租户、结果状态和送达时间聚合长期成功率与时延达标率，不改写通知业务数据。
- `0039_saas_subscription_lifecycle` 只创建 `mochat_go_saas_subscriptions` 和 `mochat_go_saas_subscription_events`；当前表保存套餐快照、六态、试用/周期/宽限期时间、期末取消、最新账单与乐观锁版本，事件表保存前后状态、操作人、来源、幂等键和载荷。迁移只从已有租户套餐回填，默认给有限期套餐 7 天宽限期，不为没有套餐的存量租户创建阻断访问的订阅。
- `0040_saas_payment_collection` 只创建 `mochat_go_saas_payment_orders` 和 `mochat_go_saas_payment_webhook_events`；订单表保存租户、套餐、金额、币种、计费周期、服务期、支付状态、催缴时间和乐观锁版本，回调表保存支付提供方事件 ID、载荷摘要、处理结果和错误，不保存支付密钥。
- `0041_saas_payment_refunds` 创建 `mochat_go_saas_payment_refunds`，并为支付订单增加退款预占、已退款金额和最近退款 ID，为支付回调增加退款 ID、平台退款单和渠道退款单。退款表保存状态、金额、币种、原因、权益动作、回调/账单/订阅审计关联与乐观锁版本；不保存支付密钥，部分退款默认保留订阅权益。
- `0042_saas_billing_invoices` 创建 `mochat_go_saas_billing_profiles` 和 `mochat_go_saas_invoice_documents`，并为支付订单增加蓝票预占、已开蓝票、红票预占、已红冲和最近单据 ID。开票资料按租户唯一保存并使用乐观锁；单据区分 `invoice/credit_note`，保存订单与原蓝票关联、资料快照、金额、状态、幂等键、提供方单号和操作审计关联，不保存税控平台密钥。
- `0043_saas_payment_settlements` 创建 `mochat_go_saas_payment_settlement_batches` 和 `mochat_go_saas_payment_settlement_entries`。批次保存渠道、结算周期、来源 SHA-256、匹配/差异/处理汇总、带符号的交易/手续费/净额/差额、关账审计和乐观锁版本；明细保存渠道流水、平台与渠道订单/退款标识、匹配内部账本、预期金额/币种/状态、差异代码、人工处理和原始 JSON。渠道流水、来源摘要、匹配订单和匹配退款均有唯一约束，不保存支付渠道密钥或银行卡数据。
- `0044_saas_payment_settlement_sync` 创建 `mochat_go_saas_payment_settlement_sync_states` 和 `mochat_go_saas_payment_settlement_sync_runs`。状态表按渠道唯一保存增量游标、当前运行、最近尝试/成功、错误和版本；运行表保存 `cron/manual` 来源、`running/succeeded/previewed/failed` 状态、前后游标、拉取/导入/幂等/差异统计、操作人和总后台审计关联。支付渠道密钥继续由内部 Bridge 管理，不进入 Go SaaS 数据库。
- `0045_saas_admin_rbac` 创建 `mochat_go_saas_admin_roles`、`mochat_go_saas_admin_role_permissions`、`mochat_go_saas_admin_user_access` 和 `mochat_go_saas_admin_user_roles`。角色按平台租户隔离，自定义角色和成员授权使用乐观锁；内置角色不可修改，平台超级管理员保留隐式 `*` 权限，普通平台成员只获得已启用角色授予的总后台权限。
- `0046_saas_admin_approvals` 创建 `mochat_go_saas_admin_approvals` 和 `mochat_go_saas_admin_approval_events`。审批单保存规范化请求摘要、状态、发起/复核/执行人、有效期、执行租约、业务副作用提交时间/操作日志 ID、结果和乐观锁版本；事件表只追加状态变更。五类业务写事务会原子写入副作用标记，进程在业务提交后、审批完成前中断时，恢复只补齐审批状态而不重复业务动作。迁移新增 `platform.approvals.read/review/execute` 三项权限和 `platform_approver` 内置角色，发起人不能复核或执行自己的申请，授予本人权限的申请也不能由本人复核或执行。
- `0047_saas_admin_approval_governance` 创建 `mochat_go_saas_admin_approval_policies`、`mochat_go_saas_admin_approval_decisions` 和 `mochat_go_saas_admin_approval_delegations`。策略按动作保存启用状态、退款金额阈值、会签票数、SLA、提醒间隔、有效期和乐观锁版本；审批申请保存策略版本及全部执行快照。逐票决定按实际复核人唯一且只追加，保留委托来源；委托按平台租户隔离并保存有效时间、状态、原因和版本。审批单额外保存批准票数、SLA 到期、下次/最近提醒和提醒次数。迁移新增 `platform.approvals.manage` 并授予平台审批人角色；退款阈值、会签、委托和提醒都由服务端事务与授权校验控制。
- `0048_saas_admin_system_health` 创建 `mochat_go_saas_admin_health_scans` 和 `mochat_go_saas_admin_system_incidents`。扫描表保存触发方式、健康状态、问题计数、事故变化、通知数、检查窗口、完整快照和审计关联；事故表按稳定检查键唯一聚合，保存严重度、状态、当前/阈值、发生次数、首次/最近检测、认领/解决人、负责人、结论和乐观锁版本。迁移新增 `platform.system.read/manage`，分别授予平台运营的查看/处置权限和审计、只读角色的查看权限；down 会删除对应 seed 和两张表。
- `0049_saas_service_accounts` 创建 `mochat_go_saas_service_accounts` 和 `mochat_go_saas_service_account_keys`。服务账号绑定单一租户，保存只读 Scope、IP/CIDR 白名单、过期时间和使用摘要；API Key 只保存 HMAC-SHA256 摘要、非敏感前缀和后四位，支持 active/retiring/revoked、轮换宽限、吊销和使用追踪。迁移新增 `platform.integrations.read/manage` 权限。
- `0050_saas_backup_recovery` 创建 `mochat_go_saas_backup_policies`、`mochat_go_saas_backup_runs` 和 `mochat_go_saas_restore_drills`。默认策略每 1440 分钟备份、保留 30 天且至少保留 7 个成功工件，要求加密，最大备份年龄 1800 分钟，恢复演练间隔 30 天。备份和演练的 `active_slot` 唯一索引防止同类操作并发；down 会删除两项灾备权限 seed 和三张台账表，生产回滚前必须先保全工件与台账。
- `0051_saas_backup_resilience` 在原灾备台账上增加历史密钥轮换所需的运行可用性、S3 兼容异地副本状态和自动临时恢复库清理状态；异地副本索引支持按状态和完成时间巡检。down 只移除 0051 新增字段与索引，不删除 0050 的本地备份与恢复台账。
- `0052_saas_compliance_lifecycle` 创建 `mochat_go_saas_compliance_policies`、`mochat_go_saas_legal_holds`、`mochat_go_saas_data_exports`、`mochat_go_saas_erasure_requests`、`mochat_go_saas_erasure_steps` 和 `mochat_go_saas_tenant_tombstones`。活动槽防止同一租户重复保留、导出或擦除；擦除步骤可恢复，最终仅保留匿名墓碑、验证 SHA-256 与脱敏审计。down 会删除审批/权限 seed 和六张合规台账表，生产回滚前必须保全必要证据。
- `0053_saas_identity_security` 创建 `mochat_go_saas_identity_policies`、`mochat_go_saas_identity_user_states`、`mochat_go_saas_identity_mfa_credentials`、`mochat_go_saas_identity_auth_challenges`、`mochat_go_saas_identity_sessions`、`mochat_go_saas_identity_login_events` 和 `mochat_go_saas_identity_security_incidents`。策略按租户控制失败锁定、MFA、IP/CIDR、会话和保留期；活动槽阻止同一用户并行登录挑战，稳定键聚合重复安全事故。down 会删除两项身份权限 seed 和七张表，现有 MFA、活动会话与登录安全证据会不可逆丢失，生产回滚前必须完成加密备份并安排用户重新登录。
- `0054_saas_branding_profiles` 创建 `mochat_go_saas_branding_profiles`，以 `tenant_id` 为主键保存一租户一档的品牌配置，使用 `version` 执行乐观锁并保存更新人。迁移为平台运营角色授予查看与管理权限，为审批、审计和只读角色授予查看权限。down 会删除这些角色 seed 和品牌档案表；回滚前应导出当前品牌配置，并准备恢复默认 MoChat Go 登录界面。
- `0055_saas_tenant_domains` 创建 `mochat_go_saas_tenant_domains`，以软删除唯一键阻止一个域名同时绑定多个租户，以主域名槽唯一键保证一个租户最多一个主域名。迁移为平台运营角色授予查看与管理权限，为审批、审计和只读角色授予查看权限。down 会删除这些角色 seed 和域名台账；回滚前必须解除外部 DNS、反向代理与证书配置，避免请求回落到平台默认租户。
- `0056_saas_domain_delivery` 创建 `mochat_go_saas_tenant_domain_deliveries` 、`mochat_go_saas_tenant_domain_delivery_jobs` 和 `mochat_go_saas_tenant_domain_delivery_events`，用于把已验证域名交付到外部 Ingress/证书控制面。down 只删除交付状态和事件，保留 0055 域名所有权台账。
- `0057_saas_release_readiness` 创建 `mochat_go_saas_release_evidence` 和 `mochat_go_saas_release_candidates`。证据项使用乐观锁并记录平台操作审计，候选快照只追加且同时固化六类证据状态；只有必需项数量、通过数和目标源码指纹匹配数均为 6 时才可标记 `ready`。down 会删除两项发布权限 seed 和两张台账表，生产回滚前应先导出发布证据与候选快照。
- `0058_saas_release_evidence_integrity` 在发布证据台账增加 `artifact_sha256`、`artifact_size_bytes` 和查询索引。down 只删除新增字段和索引；回滚后历史候选快照仍保留已经固化的工件身份。
- `0059_saas_service_account_rate_limits` 在服务账号增加每分钟/每日限额和当前窗口计数，并创建 `mochat_go_saas_service_account_usage_daily` 路由级日用量表。down 会删除路由用量历史和新增限流字段，生产回滚前应先导出必要的集成用量证据。
- `0060_saas_service_account_usage_retention` 为 `mochat_go_saas_compliance_policies` 增加 `service_account_usage_retention_days`，默认 90 天。过期用量清理会跳过处于活动法律保留的租户；down 只删除策略字段，不删除已保留的用量台账。
- `0061_saas_service_account_usage_alerts` 增加服务账号预警开关、用量百分比、拒绝数阈值、冷却和最近评估/通知时间。评估器复用既有统一告警与通知 outbox，条件恢复时关闭尚未送达的旧通知；down 会删除策略字段，已进入统一告警和通知账本的历史记录保留供审计。
- `0062_saas_audit_integrity` 增加审计日志前序摘要、当前摘要和算法版本，按受影响租户维护 legacy 锚点与链头，并持久化手动、维护命令和 cron 校验结果；down 会移除完整性治理表、摘要字段和新增权限。
- `0063_saas_audit_anchor_signatures` 增加签名检查点元数据、独立证据摘要、导出/校验状态和历史密钥 ID；down 只删除检查点表，不删除独立证据文件，避免回滚同时销毁外部防篡改证据。
- `0064_saas_audit_anchor_remote_immutability` 增加远端不可变证据定位、版本、摘要、留存和校验状态；down 只删除这些数据库字段，不删除已受 Object Lock 保护的对象版本。回滚前应先保留对象清单和版本 ID，避免失去远端证据索引。
- `0065_saas_service_account_key_pepper_ring` 为服务账号 API Key 增加 `hash_key_id` 和保护状态索引。存量 Key 默认标记为 `legacy-jwt`，可在兼容窗口继续使用并逐个轮换到独立 pepper；down 会删除密钥 ID，回滚后所有 Key 再次依赖 Dashboard JWT secret，生产回滚前必须确认该 secret 仍可用。
- `0066_saas_alert_credential_encryption` 为租户通知策略增加 AES-256-GCM 凭据密文与密钥 ID。应用支持先读取旧明文、再批量轮换并清空 `webhook_url`/`webhook_secret`；down 只移除密文列，执行前必须确认已准备可接受的旧明文回退方案，否则加密后的通知目标将无法恢复。
- `0067_wecom_credential_encryption` 为 `mc_corp` 和 `mc_work_agent` 增加 AES-256-GCM 企微凭据密文、密钥 ID 与保护状态索引。应用兼容读取旧明文，轮换后会清空企业 Secret、通讯录 Secret、回调 Token/AES Key、会话存档 Secret 和应用 Secret；down 只移除密文列，若明文已清空，回滚将导致这些凭据无法恢复，生产执行前必须先完成受控导出、重新配置或其他可验证恢复方案。
- `0068_wechat_open_credential_encryption` 为 `mochat_go_wechat_component_tickets` 和 `mc_official_account` 增加 AES-256-GCM 凭据密文、密钥 ID 与保护状态索引。应用兼容读取旧明文，轮换后会清空组件 Ticket、组件认证资料、公众号授权码、预授权码和刷新 Token；down 只移除密文列，若明文已清空，回滚将导致这些凭据无法恢复，生产执行前必须先完成受控导出、重新授权或其他可验证恢复方案。
- `0069_saas_release_candidate_approval` 将发布候选门禁纳入高风险审批策略，默认要求两名不同审批人会签且审批单 6 小时内有效。审批执行时重新远端校验六类证据，并在候选事务中原子写入审批副作用操作 ID；down 只删除该动作的策略 seed，不删除候选、审批或审计历史。
- `0070_saas_approval_policy_change_guard` 将审批策略变更本身纳入固定双人会签，阻止单个管理员先关闭或降级策略再绕过高风险门禁。治理策略自身不能停用且会签人数不得低于 2；审批执行与策略更新、操作日志和副作用标记在同一事务提交。down 只删除治理策略 seed，不删除历史审批或审计记录。
- `0071_saas_backup_policy_change_guard` 将平台数据库备份策略变更纳入固定双人会签，覆盖启停、加密、异地副本、执行频率与保留参数。直接写接口返回 428；审批载荷冻结，审批执行与策略更新、操作日志和副作用标记在同一事务提交。down 只删除该动作的策略 seed，不删除备份策略、历史审批或审计记录。
- `0072_saas_backup_cleanup_saga` 将备份保留清理纳入固定双人会签，并新增清理任务与步骤台账。审批请求由服务端冻结策略版本、截止时间和候选备份快照；执行时先原子绑定候选并记录审批副作用，再按异地副本、本地工件、数据库记录顺序断点执行。失败步骤保留本地副本并允许按原冻结范围重试；down 会解除未完成绑定、删除任务台账和策略 seed，再移除绑定字段，不删除已经完成清理的审计记录。
- `0073_saas_compliance_export_deletion_saga` 将合规导出提前删除纳入不可关闭的双人会签，并在导出台账保存审批绑定、工件/记录检查点、执行租约、尝试次数和失败原因。审批请求冻结导出编号、租户、状态、工件名、摘要、大小及保留期限；执行前再次校验法律保留、未终结擦除引用和冻结快照，再按加密工件、数据库台账顺序断点推进。失败任务可由后台或维护任务续跑；down 会删除策略 seed、索引和状态字段，不删除历史审批与操作审计。
- `0074_saas_compliance_legal_hold_release_guard` 将解除法律保留纳入不可关闭的双人会签。审批请求冻结保留编号、租户、状态、保留原因、起止时间、版本和解除依据；执行时再次校验冻结快照，并在解除事务中原子写入操作审计与审批副作用标记。down 只删除策略 seed，不删除法律保留、历史审批或操作审计。
- `0075_saas_compliance_policy_change_guard` 将租户数据合规生命周期策略变更纳入不可关闭的双人会签，覆盖导出保留、擦除宽限、近期导出门禁、账务与审计保留以及服务账号用量保留。审批申请校验并冻结完整策略与当前版本；执行时在同一事务内更新策略、写操作审计并标记审批副作用。down 只删除策略 seed，不删除合规策略、历史审批或操作审计。
- `0076_saas_identity_policy_change_guard` 将租户身份安全策略变更纳入不可关闭的双人会签，覆盖密码失败锁定、会话有效期与并发上限、强制 MFA、登录 IP 白名单及身份数据保留期。审批申请校验并冻结标准化策略与当前租户版本；执行时在同一事务内更新策略、写操作审计并标记审批副作用。down 只删除策略 seed，不删除身份策略、历史审批或操作审计。
- `0077_saas_tenant_disable_approval_guard` 将 critical 级租户停用门禁固定为至少两名不同复核人会签，并禁止审批治理关闭或降到单人。up 会启用既有 `tenant.disable` 策略并把会签人数提升到至少 2；down 只把正好为 2 的会签人数恢复为旧的一票默认，不删除审批、租户状态、订阅事件或操作审计。
- `0078_saas_critical_approval_policy_guard` 将 13 条 critical 审批策略统一固定为启用、零绕过阈值和至少两名不同复核人，并把退款从金额分段单人审批升级为全额双人会签。up 会修正数据库中已被关闭、设置阈值或降到单人的 critical 策略；down 只在新装默认状态下把退款恢复为旧的一票默认，不主动降级其他已修复策略，也不删除审批、退款或操作审计。
- `0079_saas_service_account_key_revoke_guard` 将服务账号 API Key 吊销纳入不可关闭的双人会签。审批申请会读取并冻结服务账号、Key 和当前版本，直接吊销接口返回 428；审批执行在吊销事务中原子写入操作审计和审批副作用标记。down 只删除策略 seed，不恢复已吊销 Key，也不删除历史审批或操作审计。
- `0080_saas_service_account_update_guard` 将服务账号更新纳入不可关闭的双人会签。账号状态、作用域、CIDR、额度、预警和有效期会连同当前版本一起冻结，直接更新接口返回 428；审批执行在账号更新事务中原子写入操作审计和审批副作用标记。down 只删除策略 seed，不回滚已执行的账号变更，也不删除历史审批或操作审计。
- `0081_saas_service_account_key_rotate_guard` 将服务账号 API Key 轮换纳入不可关闭的双人会签。审批申请只冻结账号版本、密钥名称、有效期和旧 Key 宽限期，不生成或保存密钥材料；审批执行时才生成新 Key，并在轮换事务中原子写入新 Key、旧 Key 宽限状态、操作审计和审批副作用标记。明文 Key 仅在成功执行响应中展示一次，审批结果只保存脱敏 Key 元数据。down 只删除策略 seed，不恢复已轮换 Key，也不删除历史审批或操作审计。
- `0082_saas_service_account_create_guard` 将服务账号创建纳入不可关闭的双人会签。审批申请冻结标准化账号配置、首个 Key 名称和有效期，不生成或保存密钥材料；审批执行时才生成首个 Key，并在同一事务中写入账号、Key、操作审计和审批副作用标记。明文 Key 仅在成功执行响应中展示一次，审批结果只保存脱敏账号与 Key 元数据。down 只删除策略 seed，不删除已创建账号、Key、历史审批或操作审计。
- `0083_saas_identity_mfa_reset_guard` 将平台管理员重置用户 MFA 纳入不可关闭的双人会签。审批申请冻结用户、租户、MFA 凭据版本和标准化原因；执行时重新校验快照，并在同一事务内禁用 MFA、清空认证材料、失效待处理挑战、撤销全部活动会话、写入 critical 安全事件和操作审计，再标记审批副作用。down 只删除策略 seed，不恢复已重置的 MFA、会话、历史审批或操作审计。
- `0084_saas_package_definition_guard` 为平台套餐定义增加乐观锁版本，并将创建、启停、说明及全部额度变更纳入不可关闭的双人会签。审批申请冻结当前套餐快照、目标定义和租户影响分析；执行时按冻结版本原子更新套餐、写入操作审计并标记审批副作用。down 只删除策略 seed 和版本列，不回滚已执行的套餐定义，也不删除历史审批或操作审计。
- `0085_saas_tenant_package_assignment_guard` 为租户套餐绑定增加独立乐观锁版本，并将平台分配套餐纳入不可关闭的双人会签。审批申请冻结租户状态、当前绑定、目标套餐定义版本和额度影响；执行时按冻结版本原子更新或恢复绑定、同步订阅、写入操作审计并标记审批副作用。续费、平台开户和套餐快照同步会递增绑定版本，使审批等待期内发生的业务变更令旧申请返回冲突。down 只删除策略 seed 和绑定版本列，不回滚已执行的套餐分配，也不删除历史审批或操作审计。
- `0086_saas_tenant_provision_approval_guard` 为运营任务增加乐观锁版本，并将平台开户纳入不可关闭的双人会签。审批申请冻结套餐定义版本、开户预览、任务版本和任务请求 SHA-256；管理员密码只以 PHP 兼容 bcrypt 哈希进入冻结载荷，API 仅返回存在标记。执行时在同一事务内创建租户、管理员、角色菜单、套餐、订阅和开通记录，更新任务状态、写入操作审计并标记审批副作用。down 只删除策略 seed 和任务版本列，不回滚已开通租户，也不删除历史审批或操作审计。
- `0087_saas_tenant_renewal_approval_guard` 将租户续费纳入不可关闭的双人会签。审批申请冻结租户状态、当前套餐绑定版本、目标套餐定义版本、订阅存在性与版本，以及可选运营任务版本和请求 SHA-256；执行时在同一事务内更新或恢复套餐绑定、写续费账单、同步订阅、更新任务状态、写入操作审计并标记审批副作用。down 只删除策略 seed，不回滚已执行续费，也不删除账单、订阅事件、历史审批或操作审计。
- `0088_saas_subscription_transition_approval_guard` 将总后台人工订阅状态迁移纳入不可关闭的 critical 双人会签。审批申请冻结租户状态、订阅 ID、版本和完整状态快照；批准执行时在同一事务内更新订阅、写订阅事件与操作审计并标记审批副作用。自动订阅对账继续按系统任务运行。down 只删除策略 seed，不回滚已执行迁移，也不删除订阅事件、历史审批或操作审计。
- `0089_saas_invoice_issue_approval_guard` 将蓝票和红票的正式开具纳入不可关闭的 critical 双人会签。审批申请冻结单据身份、状态、版本、票面快照，以及支付订单版本和退款、开票、红冲金额台账；执行时重新校验全部冻结状态，并在同一事务内更新单据、调整订单金额、写操作审计并标记审批副作用。`processing/failed/canceled` 仍可由财务直接处理。down 只删除策略 seed，不回滚已开具单据，也不删除历史审批或操作审计。
- `0090_saas_payment_order_create_approval_guard` 将创建收款订单纳入不可关闭的 critical 双人会签，并在订单中固化套餐定义版本和完整额度快照。审批申请冻结正常租户、启用套餐、服务期和应收金额；执行时重新锁定并校验租户状态与套餐版本，在同一事务内创建订单、写操作审计并标记审批副作用。支付成功按订单快照授予权益，避免等待付款期间套餐定义变化导致权益漂移；旧订单继续兼容原有实时套餐读取。down 只删除策略 seed 和订单快照列，不回滚已创建或已支付订单，也不删除历史审批与操作审计。
- `0091_saas_payment_settlement_close_guard` 将关闭支付渠道结算批次纳入不可关闭的 critical 双人会签。审批申请冻结批次编号、账期、渠道、币种、状态、来源摘要、对账计数、金额台账、版本和备注；执行时重新锁定并校验完整快照，只允许关闭无未解决差异的 `reconciled` 批次，并在同一事务内更新状态、写操作审计和标记审批副作用。down 不主动降级已经修复的关键策略，因为无法可靠恢复升级前的自定义安全配置，也不回滚已关闭批次或删除历史审批和审计。
- `0092_saas_payment_settlement_reopen_guard` 将重开已关闭支付渠道结算批次纳入不可关闭的 critical 双人会签。审批申请使用 `schemaVersion=2` 冻结完整批次账务、导入/对账元数据与关账审计；执行时重新锁定并校验全部冻结字段，只允许重开关账审计完整的 `closed` 批次，并在同一事务内恢复 `reconciled` 状态、写操作审计和标记审批副作用。执行器继续兼容 0091 已发起的旧版关账审批；down 删除新动作策略 seed，不修改批次、历史审批或审计。
- `0093_saas_payment_settlement_resolve_guard` 将解决、忽略和重新打开支付渠道结算差异纳入不可关闭的 critical 双人会签。审批申请冻结完整差异条目和所属批次账务；执行时按固定锁序重新校验版本、金额、匹配结果和处理审计，再原子更新条目、批次差异汇总、操作审计和审批副作用。down 只删除策略 seed，不修改结算数据、历史审批或审计。
- `0094_saas_tenant_domain_command_guard` 将租户主域名切换、域名启停、DNS 校验令牌轮换和域名删除纳入不可关闭的 critical 双人会签。审批申请冻结目标域名和同租户全部有效域名的路由快照；执行时重新锁定并校验快照，再原子提交域名状态、交付任务、操作审计和审批副作用。down 只删除策略 seed，不回滚已执行的域名或外部路由变更，也不删除历史审批与审计。
- `0095_saas_tenant_domain_create_guard` 将新增租户自定义域名纳入不可关闭的 critical 双人会签。审批申请只冻结标准化租户 ID 和 hostname，不生成 DNS 校验令牌；执行时重新锁定租户并校验状态、域名唯一性和数量上限，再生成令牌，并在同一事务内创建待验证绑定、交付台账、操作审计和审批副作用。审批持久化结果不保存令牌；DNS 所有权验证仍保持直接执行。down 只删除策略 seed，不删除已创建域名、历史审批或审计。
- `0096_saas_tenant_enable_approval_guard` 将已停用业务租户的重新启用纳入不可关闭的 critical 双人会签。审批申请冻结租户状态、名称及订阅 ID、状态和版本；执行时重新锁定并校验全部快照，再在同一事务内恢复租户、同步订阅、写操作审计和审批副作用。down 只删除策略 seed，不回滚已执行的租户或订阅状态，也不删除历史审批与审计。
- 迁移器会识别拆分前的一体化 `0001_initial_schema` checksum，旧库如果已经通过旧版 `0001` 建表并写入 seed，升级后执行 `apply` 会接受 legacy checksum，并用 `0002_seed_core_data` 补齐迁移记录。
- `0127_dashboard_page_rbac` 在任何 DDL 前校验套餐订阅和旧 RBAC 关系一致性，随后建立 53 页权限目录、API 资源映射、多角色/直接权限关系与追加式审计，并为用户和角色增加集合级乐观锁版本。MariaDB/MySQL DDL 会隐式提交，因此迁移按阶段执行且仅在全部成功后写 ledger；down 使用 `information_schema` 与动态 SQL 兼容恢复部分完成状态。旧菜单仅按规范化的真实 API 资源回填，四个永久 `superadmin_only` 管理页不会继承旧授权。
