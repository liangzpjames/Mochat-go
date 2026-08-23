-- Collapse smart analysis to one system-owned rule per tenant/corp. Existing
-- custom rules remain for historical result joins, but runtime selection only
-- consumes the system key introduced here.

ALTER TABLE `mochat_go_ai_analysis_rules`
  ADD COLUMN `system_key` varchar(64) NULL AFTER `corp_id`;

CREATE UNIQUE INDEX `uq_ai_rules_system_key`
  ON `mochat_go_ai_analysis_rules` (`tenant_id`, `corp_id`, `system_key`);

INSERT INTO `mochat_go_ai_analysis_rules`
  (`tenant_id`, `corp_id`, `system_key`, `name`, `objective`, `conversation_types_json`,
   `target_scope`, `target_ids_json`, `lookback_days`, `minimum_messages`, `status`,
   `current_version`, `created_by`, `updated_by`, `created_at`, `updated_at`)
SELECT binding.`tenant_id`, binding.`corp_id`, 'default-smart-analysis',
  '默认智能分析规则', '识别客户意向、沟通质量、风险信号和建议跟进动作',
  JSON_ARRAY('direct', 'group'), 'all', JSON_ARRAY(), 30, 2, 'enabled', 1, 0, 0, NOW(), NOW()
FROM `mochat_go_tenant_corp_bindings` binding
WHERE binding.`status` = 2
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_ai_analysis_rules` existing
    WHERE existing.`tenant_id` = binding.`tenant_id`
      AND existing.`corp_id` = binding.`corp_id`
      AND existing.`system_key` = 'default-smart-analysis'
  );

INSERT INTO `mochat_go_ai_analysis_rule_versions`
  (`tenant_id`, `corp_id`, `rule_id`, `version`, `objective`, `conversation_types_json`,
   `target_scope`, `target_ids_json`, `lookback_days`, `minimum_messages`, `created_by`, `created_at`)
SELECT rule.`tenant_id`, rule.`corp_id`, rule.`id`, rule.`current_version`, rule.`objective`,
  rule.`conversation_types_json`, rule.`target_scope`, rule.`target_ids_json`,
  rule.`lookback_days`, rule.`minimum_messages`, 0, NOW()
FROM `mochat_go_ai_analysis_rules` rule
WHERE rule.`system_key` = 'default-smart-analysis'
  AND rule.`deleted_at` IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_ai_analysis_rule_versions` version
    WHERE version.`tenant_id` = rule.`tenant_id`
      AND version.`corp_id` = rule.`corp_id`
      AND version.`rule_id` = rule.`id`
      AND version.`version` = rule.`current_version`
  );

UPDATE `mochat_go_dashboard_permissions`
SET `name` = '分析助手', `version` = `version` + 1
WHERE `code` = 'dashboard.ai_setting.agent' AND `name` <> '分析助手';

-- DELETE resource rows for independent rule creation, mutation, deletion and
-- status changes. The fixed rule is now saved atomically through AI settings.
DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
WHERE permission.`code` = 'dashboard.ai_insight.smart_analysis'
  AND resource.`resource_type` = 'api'
  AND (
    (resource.`path_pattern` = '/dashboard/ai-insight/smart-analysis/rules'
      AND resource.`http_method` IN ('POST', 'PUT', 'DELETE'))
    OR (resource.`path_pattern` = '/dashboard/ai-insight/smart-analysis/rules/status'
      AND resource.`http_method` = 'POST')
  );
