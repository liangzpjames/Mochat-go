UPDATE `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
INNER JOIN (
  SELECT 'dashboard.ai_insight.smart_analysis' AS `permission_code`, 'POST' AS `http_method`, '/dashboard/ai-insight/smart-analysis/rules' AS `path_pattern`
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'PUT', '/dashboard/ai-insight/smart-analysis/rules'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'DELETE', '/dashboard/ai-insight/smart-analysis/rules'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'POST', '/dashboard/ai-insight/smart-analysis/rules/status'
) restoration_seed ON restoration_seed.`permission_code` = permission.`code`
  AND restoration_seed.`http_method` = resource.`http_method`
  AND restoration_seed.`path_pattern` = resource.`path_pattern`
SET resource.`status` = 1,
    resource.`deleted_at` = NULL,
    resource.`version` = resource.`version` + 1
WHERE resource.`resource_type` = 'api'
  AND resource.`status` = 0;

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
