-- Unify AI insight persistence around one conversation-scoped daily result.
-- Historical DATETIME values are written by the Go services in UTC; convert
-- them with numeric offsets so this backfill does not depend on MySQL timezone
-- tables being installed.
ALTER TABLE `mochat_go_ai_conversation_insights`
  ADD COLUMN IF NOT EXISTS `analysis_date` date NULL AFTER `conversation_key`,
  ADD COLUMN IF NOT EXISTS `previous_insight_id` bigint unsigned NULL AFTER `prompt_version`,
  ADD COLUMN IF NOT EXISTS `previous_score` decimal(6,2) NULL AFTER `previous_insight_id`,
  ADD COLUMN IF NOT EXISTS `previous_summary` varchar(1200) NOT NULL DEFAULT '' AFTER `previous_score`,
  ADD COLUMN IF NOT EXISTS `previous_generated_at` datetime(6) NULL AFTER `previous_summary`;

UPDATE `mochat_go_ai_conversation_insights`
SET `analysis_date` = COALESCE(
  DATE(CONVERT_TZ(COALESCE(`generated_at`, `source_ended_at`, `created_at`), '+00:00', '+08:00')),
  DATE(COALESCE(`generated_at`, `source_ended_at`, `created_at`)),
  CURDATE()
)
WHERE `analysis_date` IS NULL;

-- Keep the newest physical row for each historical daily logical result before
-- replacing the source-fingerprint uniqueness contract.
DELETE duplicate_row
FROM `mochat_go_ai_conversation_insights` duplicate_row
INNER JOIN `mochat_go_ai_conversation_insights` retained_row
  ON retained_row.`tenant_id` = duplicate_row.`tenant_id`
 AND retained_row.`corp_id` = duplicate_row.`corp_id`
 AND retained_row.`analysis_type` = duplicate_row.`analysis_type`
 AND retained_row.`rule_version_id` = duplicate_row.`rule_version_id`
 AND retained_row.`conversation_key` = duplicate_row.`conversation_key`
 AND retained_row.`analysis_date` = duplicate_row.`analysis_date`
 AND retained_row.`id` > duplicate_row.`id`;

ALTER TABLE `mochat_go_ai_conversation_insights`
  MODIFY COLUMN IF EXISTS `analysis_date` date NOT NULL,
  DROP INDEX IF EXISTS `uq_ai_conversation_source`,
  ADD UNIQUE KEY IF NOT EXISTS `uq_ai_conversation_daily` (`tenant_id`,`corp_id`,`analysis_type`,`rule_version_id`,`conversation_key`,`analysis_date`),
  ADD KEY IF NOT EXISTS `idx_ai_insight_previous` (`tenant_id`,`corp_id`,`analysis_type`,`rule_version_id`,`conversation_key`,`analysis_date`,`status`);

DROP TABLE IF EXISTS `mochat_go_ai_analysis`;
