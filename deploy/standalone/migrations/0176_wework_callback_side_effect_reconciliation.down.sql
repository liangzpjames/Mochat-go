DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id`=resource.`permission_id`
WHERE permission.`code`='dashboard.company_setting.website'
  AND ((resource.`http_method`='GET' AND resource.`path_pattern`='/dashboard/company/callback-side-effects')
    OR (resource.`http_method`='GET' AND resource.`path_pattern`='/dashboard/company/callback-side-effects/{eventKey}/{actionKey}')
    OR (resource.`http_method`='POST' AND resource.`path_pattern`='/dashboard/company/callback-side-effects/{eventKey}/{actionKey}/reconcile'));

DROP TABLE `mochat_go_wework_callback_side_effect_commands`;

UPDATE `mochat_go_dashboard_permission_audits`
SET `request_id`=LEFT(`request_id`,96)
WHERE `action`='dashboard.company.callback_side_effect.reconcile' AND CHAR_LENGTH(`request_id`)>96;

ALTER TABLE `mochat_go_dashboard_permission_audits`
  MODIFY COLUMN `request_id` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '';

ALTER TABLE `mochat_go_wework_callback_side_effects`
  DROP INDEX `idx_wework_callback_side_effect_unknown`,
  DROP COLUMN `last_reconciled_at`,
  DROP COLUMN `last_reconciled_by`,
  DROP COLUMN `last_evidence_ref`,
  DROP COLUMN `last_evidence_kind`,
  DROP COLUMN `last_reason`,
  DROP COLUMN `last_decision`,
  DROP COLUMN `reconcile_after`,
  DROP COLUMN `unknown_at`,
  DROP COLUMN `reconciliation_fence`,
  DROP COLUMN `version`;
