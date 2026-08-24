-- Roll back only seed records that remain unused. Historical insight rows keep
-- their rule-version dependency intact, so this migration never cascades into
-- mochat_go_ai_conversation_insights.

DELETE version
FROM `mochat_go_ai_analysis_rule_versions` version
INNER JOIN `mochat_go_ai_analysis_rules` rule
  ON rule.`id` = version.`rule_id`
 AND rule.`tenant_id` = version.`tenant_id`
 AND rule.`corp_id` = version.`corp_id`
LEFT JOIN `mochat_go_ai_conversation_insights` insight
  ON insight.`rule_version_id` = version.`id`
 AND insight.`tenant_id` = version.`tenant_id`
 AND insight.`corp_id` = version.`corp_id`
LEFT JOIN `mochat_go_ai_insight_runs` insight_run
  ON insight_run.`rule_version_id` = version.`id`
 AND insight_run.`tenant_id` = version.`tenant_id`
 AND insight_run.`corp_id` = version.`corp_id`
WHERE rule.`system_key` = 'session-analysis'
  AND version.`version` = 1
  AND version.`created_by` = 0
  AND insight.`id` IS NULL
  AND insight_run.`id` IS NULL;

DELETE rule
FROM `mochat_go_ai_analysis_rules` rule
LEFT JOIN `mochat_go_ai_analysis_rule_versions` version
  ON version.`rule_id` = rule.`id`
 AND version.`tenant_id` = rule.`tenant_id`
 AND version.`corp_id` = rule.`corp_id`
WHERE rule.`system_key` = 'session-analysis'
  AND rule.`created_by` = 0
  AND rule.`updated_by` = 0
  AND version.`id` IS NULL;

DELETE smart_agent
FROM `mochat_go_ai_agents` smart_agent
WHERE smart_agent.`system_key` = 'smart-analysis'
  AND smart_agent.`name` = '智能分析助手'
  AND smart_agent.`created_by` = 0
  AND smart_agent.`updated_by` = 0;

UPDATE `mochat_go_ai_analysis_rules`
SET `name` = '默认智能分析规则', `updated_at` = NOW()
WHERE `system_key` = 'default-smart-analysis'
  AND `deleted_at` IS NULL
  AND `name` = '智能分析规则';

-- Prompt columns are deliberately retained. Dropping them would destroy
-- prompt history on retained rule versions, including versions referenced by
-- historical insights, and this rollback must not cascade-delete that data.
