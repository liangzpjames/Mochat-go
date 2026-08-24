DELETE resource
FROM `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission
  ON permission.`id` = resource.`permission_id`
WHERE permission.`code` IN (
    'dashboard.ai_insight.session_analysis',
    'dashboard.ai_insight.smart_analysis'
  )
  AND resource.`resource_type` = 'api'
  AND resource.`http_method` = 'GET'
  AND resource.`path_pattern` IN (
    '/dashboard/ai-insight/session-analysis/filter-options',
    '/dashboard/ai-insight/smart-analysis/filter-options'
  );
