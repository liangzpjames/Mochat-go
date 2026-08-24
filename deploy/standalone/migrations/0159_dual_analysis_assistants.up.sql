-- Split the fixed analysis configuration into independent session and smart
-- assistants. Prompt fields live on both the mutable rule and its immutable
-- version so future execution can reproduce the prompt that produced a result.

ALTER TABLE `mochat_go_ai_analysis_rules`
  ADD COLUMN `customer_analysis_prompt` TEXT NULL AFTER `objective`,
  ADD COLUMN `employee_qa_prompt` TEXT NULL AFTER `customer_analysis_prompt`;

ALTER TABLE `mochat_go_ai_analysis_rule_versions`
  ADD COLUMN `customer_analysis_prompt` TEXT NULL AFTER `objective`,
  ADD COLUMN `employee_qa_prompt` TEXT NULL AFTER `customer_analysis_prompt`;

-- The new smart assistant inherits the session assistant's knowledge links
-- once. Later assistant updates are intentionally independent.
INSERT INTO `mochat_go_ai_agents`
  (`id`, `tenant_id`, `corp_id`, `system_key`, `name`, `description`, `knowledge_base_ids`, `status`, `created_by`, `updated_by`, `created_at`, `updated_at`, `deleted_at`)
SELECT UUID(), binding.`tenant_id`, binding.`corp_id`, 'smart-analysis',
  '智能分析助手', '根据企业微信会话生成可追溯的智能分析结论。',
  COALESCE(session_agent.`knowledge_base_ids`, JSON_ARRAY()), 1, 0, 0, NOW(6), NOW(6), NULL
FROM `mochat_go_tenant_corp_bindings` binding
LEFT JOIN `mochat_go_ai_agents` session_agent
  ON session_agent.`tenant_id` = binding.`tenant_id`
 AND session_agent.`corp_id` = binding.`corp_id`
 AND session_agent.`system_key` = 'session-analysis'
 AND session_agent.`deleted_at` IS NULL
WHERE binding.`status` = 2
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_ai_agents` existing
    WHERE existing.`tenant_id` = binding.`tenant_id`
      AND existing.`corp_id` = binding.`corp_id`
      AND existing.`system_key` = 'smart-analysis'
  );

INSERT INTO `mochat_go_ai_analysis_rules`
  (`tenant_id`, `corp_id`, `system_key`, `name`, `objective`, `customer_analysis_prompt`, `employee_qa_prompt`,
   `conversation_types_json`, `target_scope`, `target_ids_json`, `lookback_days`, `minimum_messages`, `status`,
   `current_version`, `created_by`, `updated_by`, `created_at`, `updated_at`)
SELECT binding.`tenant_id`, binding.`corp_id`, 'session-analysis',
  '会话分析规则', '分析客户会话并提供客户经营与员工服务改进建议。',
  '请基于会话内容分析客户购买意向、流失风险和核心需求，说明证据并给出下一步跟进建议。',
  '请基于会话内容完成员工服务质检，识别客户异议并给出可执行的改进建议。',
  JSON_ARRAY('direct', 'group'), 'all', JSON_ARRAY(), 30, 2, 'enabled', 1, 0, 0, NOW(), NOW()
FROM `mochat_go_tenant_corp_bindings` binding
WHERE binding.`status` = 2
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_ai_analysis_rules` existing
    WHERE existing.`tenant_id` = binding.`tenant_id`
      AND existing.`corp_id` = binding.`corp_id`
      AND existing.`system_key` = 'session-analysis'
  );

INSERT INTO `mochat_go_ai_analysis_rule_versions`
  (`tenant_id`, `corp_id`, `rule_id`, `version`, `objective`, `customer_analysis_prompt`, `employee_qa_prompt`,
   `conversation_types_json`, `target_scope`, `target_ids_json`, `lookback_days`, `minimum_messages`, `created_by`, `created_at`)
SELECT rule.`tenant_id`, rule.`corp_id`, rule.`id`, rule.`current_version`, rule.`objective`,
  rule.`customer_analysis_prompt`, rule.`employee_qa_prompt`, rule.`conversation_types_json`, rule.`target_scope`,
  rule.`target_ids_json`, rule.`lookback_days`, rule.`minimum_messages`, 0, NOW()
FROM `mochat_go_ai_analysis_rules` rule
WHERE rule.`system_key` = 'session-analysis'
  AND rule.`deleted_at` IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_ai_analysis_rule_versions` version
    WHERE version.`tenant_id` = rule.`tenant_id`
      AND version.`corp_id` = rule.`corp_id`
      AND version.`rule_id` = rule.`id`
      AND version.`version` = rule.`current_version`
  );

-- Keep the established persistence key and immutable version history; only the
-- display name changes to distinguish this rule from the session-analysis one.
UPDATE `mochat_go_ai_analysis_rules`
SET `name` = '智能分析规则', `updated_at` = NOW()
WHERE `system_key` = 'default-smart-analysis'
  AND `deleted_at` IS NULL
  AND `name` <> '智能分析规则';
