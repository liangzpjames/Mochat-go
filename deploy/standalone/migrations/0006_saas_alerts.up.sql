-- SaaS operational alert ledger for standalone Go runtime quota signals.
-- The table is Go-owned metadata and does not modify upstream MoChat tables.

CREATE TABLE IF NOT EXISTS `mochat_go_saas_alerts` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `alert_key` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警聚合键',
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID',
  `alert_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警类型',
  `severity` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警级别',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'open' COMMENT '状态：open/resolved',
  `metric` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'SaaS 用量指标',
  `period_key` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'lifetime' COMMENT '统计周期键',
  `current_value` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '当前用量',
  `limit_value` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '额度，0 表示不限',
  `additional_value` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '本次额外用量',
  `occurrence_count` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '触发次数',
  `source` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警来源',
  `message` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '告警摘要',
  `context_json` json DEFAULT NULL COMMENT '告警上下文',
  `first_seen_at` datetime DEFAULT NULL COMMENT '首次触发时间',
  `last_seen_at` datetime DEFAULT NULL COMMENT '最近触发时间',
  `resolved_at` datetime DEFAULT NULL COMMENT '解决时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_alerts_key` (`alert_key`),
  KEY `idx_mochat_go_saas_alerts_tenant_status` (`tenant_id`, `status`, `last_seen_at`),
  KEY `idx_mochat_go_saas_alerts_metric_status` (`metric`, `status`, `last_seen_at`),
  KEY `idx_mochat_go_saas_alerts_type_status` (`alert_type`, `status`, `last_seen_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 告警账本';
