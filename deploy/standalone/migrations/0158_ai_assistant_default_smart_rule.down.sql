-- This rollback intentionally does not restore smart-analysis write grants.
-- 0158 did not persist per-row provenance, so an already-applied database
-- cannot distinguish grants disabled by 0158 from grants disabled or deleted
-- before it. Conservatively preserving the current RBAC state avoids reviving
-- access that an administrator or an earlier migration had already removed.

UPDATE `mochat_go_dashboard_permissions`
SET `name` = '智能体管理', `version` = `version` + 1
WHERE `code` = 'dashboard.ai_setting.agent' AND `name` = '分析助手';

DELETE version
FROM `mochat_go_ai_analysis_rule_versions` version
INNER JOIN `mochat_go_ai_analysis_rules` rule
  ON rule.`id` = version.`rule_id`
  AND rule.`tenant_id` = version.`tenant_id`
  AND rule.`corp_id` = version.`corp_id`
WHERE rule.`system_key` = 'default-smart-analysis';

DELETE FROM `mochat_go_ai_analysis_rules`
WHERE `system_key` = 'default-smart-analysis';

ALTER TABLE `mochat_go_ai_analysis_rules`
  DROP INDEX `uq_ai_rules_system_key`,
  DROP COLUMN `system_key`;
