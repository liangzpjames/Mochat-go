-- Rollback restores the former schema contract only. The removed legacy rows
-- cannot be reconstructed truthfully, so the legacy table is recreated empty.
ALTER TABLE `mochat_go_ai_conversation_insights`
  DROP INDEX IF EXISTS `idx_ai_insight_previous`,
  DROP INDEX IF EXISTS `uq_ai_conversation_daily`;

-- Multiple daily rows may share a source fingerprint. Keep the newest one so
-- the historical source-fingerprint unique key can be restored safely.
DELETE duplicate_row
FROM `mochat_go_ai_conversation_insights` duplicate_row
INNER JOIN `mochat_go_ai_conversation_insights` retained_row
  ON retained_row.`tenant_id` = duplicate_row.`tenant_id`
 AND retained_row.`corp_id` = duplicate_row.`corp_id`
 AND retained_row.`analysis_type` = duplicate_row.`analysis_type`
 AND retained_row.`rule_version_id` = duplicate_row.`rule_version_id`
 AND retained_row.`conversation_key` = duplicate_row.`conversation_key`
 AND retained_row.`source_fingerprint` = duplicate_row.`source_fingerprint`
 AND retained_row.`id` > duplicate_row.`id`;

ALTER TABLE `mochat_go_ai_conversation_insights`
  ADD UNIQUE KEY IF NOT EXISTS `uq_ai_conversation_source` (`tenant_id`,`corp_id`,`analysis_type`,`rule_version_id`,`conversation_key`,`source_fingerprint`),
  DROP COLUMN IF EXISTS `previous_generated_at`,
  DROP COLUMN IF EXISTS `previous_summary`,
  DROP COLUMN IF EXISTS `previous_score`,
  DROP COLUMN IF EXISTS `previous_insight_id`,
  DROP COLUMN IF EXISTS `analysis_date`;

CREATE TABLE IF NOT EXISTS `mochat_go_ai_analysis` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT 0,
  `corp_id` int(10) unsigned NOT NULL DEFAULT 0,
  `page` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'succeeded',
  `payload` json DEFAULT NULL,
  `error` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ai_analysis_corp_page_created` (`corp_id`, `page`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Phase 3 Final AI 洞察结果落库';
