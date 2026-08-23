INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT permission.`id`, 'api', route.`http_method`, route.`path_pattern`, 1, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'POST' AS `http_method`, '/dashboard/ai-insight/smart-analysis/rules' AS `path_pattern`
  UNION ALL SELECT 'PUT', '/dashboard/ai-insight/smart-analysis/rules'
  UNION ALL SELECT 'DELETE', '/dashboard/ai-insight/smart-analysis/rules'
  UNION ALL SELECT 'POST', '/dashboard/ai-insight/smart-analysis/rules/status'
) route
WHERE permission.`code` = 'dashboard.ai_insight.smart_analysis'
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id` = permission.`id`
      AND existing.`resource_type` = 'api'
      AND existing.`http_method` = route.`http_method`
      AND existing.`path_pattern` = route.`path_pattern`
  );

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
