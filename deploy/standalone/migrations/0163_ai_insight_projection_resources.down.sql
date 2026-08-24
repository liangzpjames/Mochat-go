-- Remove only the fifteen projection resources introduced by 0163.
DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
INNER JOIN (
  SELECT 'dashboard.ai_insight.emotion' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/ai-insight/emotion/records' AS `path_pattern`
  UNION ALL SELECT 'dashboard.ai_insight.emotion', 'GET', '/dashboard/ai-insight/emotion/detail'
  UNION ALL SELECT 'dashboard.ai_insight.emotion', 'GET', '/dashboard/ai-insight/emotion/status'
  UNION ALL SELECT 'dashboard.ai_insight.emotion', 'GET', '/dashboard/ai-insight/emotion/filter-options'
  UNION ALL SELECT 'dashboard.ai_insight.emotion', 'GET', '/dashboard/ai-insight/emotion/export'
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/records'
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/detail'
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/status'
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/filter-options'
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score/export'
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/records'
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/detail'
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/status'
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/filter-options'
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword/export'
) seed ON seed.`permission_code` = permission.`code`
  AND seed.`http_method` = resource.`http_method`
  AND seed.`path_pattern` = resource.`path_pattern`
WHERE resource.`resource_type` = 'api'
  AND resource.`http_method` = 'GET';
