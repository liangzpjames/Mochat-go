-- Tenant-scoped WeCom integration slots and durable archive-media objects.
-- Credentials are encrypted by the application and never appear in this
-- migration, audit columns, or permission seeds.
CREATE TABLE IF NOT EXISTS `mochat_go_wecom_integrations` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
  `mode` enum('self_built','third_party_delegated') COLLATE utf8mb4_unicode_ci NOT NULL,
  `slot` enum('current','candidate') COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` enum('unconfigured','pending_verification','active','suspended','revoked','failed') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'unconfigured',
  `verified_wx_corpid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `agent_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `provider_app_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `credential_ciphertext` longtext COLLATE utf8mb4_bin NULL,
  `credential_key_id` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `credential_hint` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `scope_json` json NOT NULL,
  `scope_digest` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `missing_capabilities_json` json NOT NULL,
  `generation` bigint(20) unsigned NOT NULL DEFAULT 1,
  `version` bigint(20) unsigned NOT NULL DEFAULT 1,
  `verified_at` datetime(6) NULL DEFAULT NULL,
  `activated_at` datetime(6) NULL DEFAULT NULL,
  `suspended_at` datetime(6) NULL DEFAULT NULL,
  `revoked_at` datetime(6) NULL DEFAULT NULL,
  `last_error_code` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `last_error_at` datetime(6) NULL DEFAULT NULL,
  `last_audit_at` datetime(6) NULL DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_wecom_integration_scope_slot` (`tenant_id`,`corp_id`,`slot`),
  KEY `idx_wecom_integration_scope_status` (`tenant_id`,`corp_id`,`status`,`slot`),
  KEY `idx_wecom_integration_provider` (`provider_app_id`,`status`),
  CONSTRAINT `fk_wecom_integration_corp` FOREIGN KEY (`tenant_id`,`corp_id`) REFERENCES `mc_corp` (`tenant_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tenant WeCom current and candidate integrations';

CREATE TABLE IF NOT EXISTS `mochat_go_archive_media_objects` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
  `msgid` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_identity` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sdk_file_id_hash` char(64) COLLATE utf8mb4_bin NOT NULL,
  `media_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `media_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `mime_type` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `expected_size_bytes` bigint(20) unsigned NOT NULL DEFAULT 0,
  `status` enum('pending','fetching','ready','failed','missing','corrupt') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `index_buf` mediumtext COLLATE utf8mb4_bin NULL,
  `bytes_received` bigint(20) unsigned NOT NULL DEFAULT 0,
  `sha256` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `storage_path` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `attempt` int(10) unsigned NOT NULL DEFAULT 0,
  `lease_token` varchar(96) COLLATE utf8mb4_bin NOT NULL DEFAULT '',
  `lease_expires_at` datetime(6) NULL DEFAULT NULL,
  `heartbeat_at` datetime(6) NULL DEFAULT NULL,
  `last_error_code` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `last_error` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  `completed_at` datetime(6) NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_archive_media_source_identity` (`tenant_id`,`corp_id`,`msgid`,`sdk_file_id_hash`),
  KEY `idx_archive_media_claim_lease` (`status`,`lease_expires_at`,`id`),
  KEY `idx_archive_media_scope_msgid` (`tenant_id`,`corp_id`,`msgid`),
  CONSTRAINT `fk_archive_media_corp` FOREIGN KEY (`tenant_id`,`corp_id`) REFERENCES `mc_corp` (`tenant_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Durable WeCom archive media retrieval objects';

-- A verified non-demo binding is the only historical condition allowed to
-- become active. Pending bindings and `fake_tenant_%` demo corps stay
-- unconfigured and must be explicitly verified through the new integration
-- workflow.
INSERT INTO `mochat_go_wecom_integrations`
  (`id`,`tenant_id`,`corp_id`,`mode`,`slot`,`status`,`verified_wx_corpid`,`scope_json`,`scope_digest`,`missing_capabilities_json`,`generation`,`version`,`verified_at`,`activated_at`,`last_audit_at`)
SELECT UUID(), b.`tenant_id`, b.`corp_id`, 'self_built', 'current', 'active', b.`verified_wx_corpid`,
       JSON_ARRAY(), SHA2('', 256), JSON_ARRAY(), 1, 1, b.`verified_at`, b.`verified_at`, NOW(6)
FROM `mochat_go_tenant_corp_bindings` b
INNER JOIN `mc_corp` c ON c.`tenant_id` = b.`tenant_id` AND c.`id` = b.`corp_id` AND c.`deleted_at` IS NULL
WHERE b.`status` = 2
  AND COALESCE(b.`verified_wx_corpid`, '') <> ''
  AND c.`wx_corpid` NOT LIKE 'fake_tenant_%'
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_wecom_integrations` existing
    WHERE existing.`tenant_id` = b.`tenant_id` AND existing.`corp_id` = b.`corp_id` AND existing.`slot` = 'current'
  );

INSERT INTO `mochat_go_wecom_integrations`
  (`id`,`tenant_id`,`corp_id`,`mode`,`slot`,`status`,`scope_json`,`scope_digest`,`missing_capabilities_json`,`generation`,`version`,`last_audit_at`)
SELECT UUID(), b.`tenant_id`, b.`corp_id`, 'self_built', 'current', 'unconfigured',
       JSON_ARRAY(), SHA2('', 256), JSON_ARRAY(), 1, 1, NOW(6)
FROM `mochat_go_tenant_corp_bindings` b
INNER JOIN `mc_corp` c ON c.`tenant_id` = b.`tenant_id` AND c.`id` = b.`corp_id` AND c.`deleted_at` IS NULL
WHERE (b.`status` <> 2 OR c.`wx_corpid` LIKE 'fake_tenant_%' OR COALESCE(b.`verified_wx_corpid`, '') = '')
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_wecom_integrations` existing
    WHERE existing.`tenant_id` = b.`tenant_id` AND existing.`corp_id` = b.`corp_id` AND existing.`slot` = 'current'
  );

-- A later authenticated media endpoint uses the existing global-message
-- permission and retains normal employee data-scope checks.
INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`,`resource_type`,`http_method`,`path_pattern`,`scope_required`,`status`,`version`)
SELECT p.`id`, 'api', seed.`http_method`, '/dashboard/archive/media/{id}/content', 1, 1, 1
FROM `mochat_go_dashboard_permissions` p
INNER JOIN (
  SELECT 'GET' AS `http_method`
  UNION ALL SELECT 'HEAD'
) seed ON 1 = 1
WHERE p.`code` = 'dashboard.chat.v2_all'
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id` = p.`id`
      AND existing.`resource_type` = 'api'
      AND existing.`http_method` = seed.`http_method`
      AND existing.`path_pattern` = '/dashboard/archive/media/{id}/content'
  );

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`,`permission_code`,`created_at`)
SELECT r.`id`, seed.`permission_code`, NOW()
FROM `mochat_go_saas_admin_roles` r
INNER JOIN (
  SELECT 'platform_operations' AS `role_code`, 'platform.wecom_integrations.read' AS `permission_code`
  UNION ALL SELECT 'platform_operations', 'platform.wecom_integrations.manage'
  UNION ALL SELECT 'platform_auditor', 'platform.wecom_integrations.read'
  UNION ALL SELECT 'platform_readonly', 'platform.wecom_integrations.read'
) seed ON seed.`role_code` = r.`code`;
