-- Register the three session-analysis projections as employee-scoped read APIs.
INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT permission.`id`, 'api', seed.`http_method`, seed.`path_pattern`, seed.`scope_required`, 1, 1
FROM `mochat_go_dashboard_permissions` permission
INNER JOIN (
  SELECT 'dashboard.ai_insight.emotion' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/ai-insight/emotion/records' AS `path_pattern`, 1 AS `scope_required`
  UNION ALL SELECT 'dashboard.ai_insight.emotion', 'GET', '/dashboard/ai-insight/emotion/detail', 1
  UNION ALL SELECT 'dashboard.ai_insight.emotion', 'GET', '/dashboard/ai-insight/emotion/status', 1
  UNION ALL SELECT 'dashboard.ai_insight.emotion', 'GET', '/dashboard/ai-insight/emotion/filter-options', 1
  UNION ALL SELECT 'dashboard.ai_insight.emotion', 'GET', '/dashboard/ai-insight/emotion/export', 1
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/records', 1
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/detail', 1
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/status', 1
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/filter-options', 1
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/export', 1
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/records', 1
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/detail', 1
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/status', 1
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/filter-options', 1
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/export', 1
) seed ON seed.`permission_code` = permission.`code`
WHERE NOT EXISTS (
  SELECT 1
  FROM `mochat_go_dashboard_permission_resources` existing
  WHERE existing.`permission_id` = permission.`id`
    AND existing.`resource_type` = 'api'
    AND existing.`http_method` = seed.`http_method`
    AND existing.`path_pattern` = seed.`path_pattern`
);
