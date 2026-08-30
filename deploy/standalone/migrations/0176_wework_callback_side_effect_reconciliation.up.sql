ALTER TABLE `mochat_go_wework_callback_side_effects`
  ADD COLUMN `version` bigint(20) unsigned NOT NULL DEFAULT 1 AFTER `status`,
  ADD COLUMN `reconciliation_fence` bigint(20) unsigned NOT NULL DEFAULT 0 AFTER `version`,
  ADD COLUMN `unknown_at` datetime(6) NULL DEFAULT NULL AFTER `reconciliation_fence`,
  ADD COLUMN `reconcile_after` datetime(6) NULL DEFAULT NULL AFTER `unknown_at`,
  ADD COLUMN `last_decision` varchar(40) NOT NULL DEFAULT '' AFTER `reconcile_after`,
  ADD COLUMN `last_reason` varchar(255) NOT NULL DEFAULT '' AFTER `last_decision`,
  ADD COLUMN `last_evidence_kind` varchar(40) NOT NULL DEFAULT '' AFTER `last_reason`,
  ADD COLUMN `last_evidence_ref` varchar(255) NOT NULL DEFAULT '' AFTER `last_evidence_kind`,
  ADD COLUMN `last_reconciled_by` int(10) unsigned NULL DEFAULT NULL AFTER `last_evidence_ref`,
  ADD COLUMN `last_reconciled_at` datetime(6) NULL DEFAULT NULL AFTER `last_reconciled_by`,
  ADD KEY `idx_wework_callback_side_effect_unknown` (`tenant_id`,`corp_id`,`status`,`unknown_at`,`event_key`,`action_key`);

UPDATE `mochat_go_wework_callback_side_effects`
SET `unknown_at`=`updated_at`, `reconcile_after`=DATE_ADD(`updated_at`, INTERVAL 15 MINUTE)
WHERE `status`='unknown' AND `unknown_at` IS NULL;

ALTER TABLE `mochat_go_dashboard_permission_audits`
  MODIFY COLUMN `request_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '';

CREATE TABLE `mochat_go_wework_callback_side_effect_commands` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
  `event_key` char(64) COLLATE utf8mb4_bin NOT NULL,
  `action_key` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `request_id` varchar(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `decision` varchar(40) NOT NULL,
  `request_fingerprint` binary(32) NOT NULL,
  `reservation_token` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `expected_version` bigint(20) unsigned NOT NULL,
  `expected_inbox_lease_fence` bigint(20) unsigned NOT NULL,
  `result_status` varchar(16) NOT NULL,
  `result_version` bigint(20) unsigned NOT NULL,
  `result_inbox_lease_fence` bigint(20) unsigned NOT NULL,
  `replay_scheduled` tinyint(1) NOT NULL DEFAULT 0,
  `remaining_unknown_actions` int(10) unsigned NOT NULL DEFAULT 0,
  `actor_user_id` int(10) unsigned NOT NULL,
  `reason` varchar(255) NOT NULL,
  `evidence_kind` varchar(40) NOT NULL,
  `evidence_ref` varchar(255) NOT NULL,
  `operation_audit_id` bigint(20) unsigned NOT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_wework_callback_side_effect_command_request` (`tenant_id`,`corp_id`,`request_id`),
  KEY `idx_wework_callback_side_effect_command_scope` (`tenant_id`,`corp_id`,`event_key`,`action_key`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Idempotent operator decisions for callback side-effect recovery';

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`,`resource_type`,`http_method`,`path_pattern`,`scope_required`,`status`,`version`)
SELECT permission.`id`, 'api', resources.`http_method`, resources.`path_pattern`, resources.`scope_required`, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'dashboard.company_setting.website' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/company/callback-side-effects' AS `path_pattern`, 0 AS `scope_required`
  UNION ALL SELECT 'dashboard.company_setting.website', 'GET', '/dashboard/company/callback-side-effects/{eventKey}/{actionKey}', 0
  UNION ALL SELECT 'dashboard.company_setting.website', 'POST', '/dashboard/company/callback-side-effects/{eventKey}/{actionKey}/reconcile', 0
) resources ON permission.`code`=resources.`permission_code`
WHERE permission.`code`='dashboard.company_setting.website'
  AND permission.`deleted_at` IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id`=permission.`id`
      AND existing.`http_method`=resources.`http_method`
      AND existing.`path_pattern`=resources.`path_pattern`
  );
