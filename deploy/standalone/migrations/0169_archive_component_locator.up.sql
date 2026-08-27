-- Encrypted data-zone component locators. Message content remains inside the
-- data-zone provider and is never stored in ordinary message/media columns.
CREATE TABLE IF NOT EXISTS `mochat_go_archive_component_locators` (
  `id` char(36) COLLATE utf8mb4_bin NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
  `msgid` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_identity` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `content_policy` enum('component') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'component',
  `public_key_version` int(10) unsigned NOT NULL,
  `locator_ciphertext` longtext COLLATE utf8mb4_bin NOT NULL,
  `locator_key_id` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `status` enum('available','revoked','failed') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'available',
  `last_error_code` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_archive_component_scope_message` (`tenant_id`,`corp_id`,`msgid`),
  KEY `idx_archive_component_scope_status` (`tenant_id`,`corp_id`,`status`,`updated_at`),
  CONSTRAINT `fk_archive_component_corp` FOREIGN KEY (`tenant_id`,`corp_id`) REFERENCES `mc_corp` (`tenant_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Encrypted data-zone archive display locators';

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`,`resource_type`,`http_method`,`path_pattern`,`scope_required`,`status`,`version`)
SELECT permission.`id`, 'api', seed.`http_method`, seed.`path_pattern`, 1, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'POST' AS `http_method`, '/dashboard/archive/components/{id}/session' AS `path_pattern`
  UNION ALL SELECT 'GET', '/dashboard/archive/components/session/{token}'
) seed ON 1 = 1
WHERE permission.`code` IN (
  'dashboard.chat.v2_all',
  'dashboard.chat.v2_staff',
  'dashboard.chat.v2_customer',
  'dashboard.chat.v2_group'
)
  AND NOT EXISTS (
    SELECT 1
    FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id` = permission.`id`
      AND existing.`resource_type` = 'api'
      AND existing.`http_method` = seed.`http_method`
      AND existing.`path_pattern` = seed.`path_pattern`
  );
