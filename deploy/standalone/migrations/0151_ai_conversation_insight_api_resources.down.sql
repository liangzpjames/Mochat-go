DELETE pr
FROM `mochat_go_dashboard_permission_resources` pr
JOIN `mochat_go_dashboard_permissions` p ON p.`id` = pr.`permission_id`
WHERE p.`code` IN ('dashboard.ai_insight.session_analysis', 'dashboard.ai_insight.smart_analysis')
  AND pr.`resource_type` = 'api'
  AND pr.`path_pattern` LIKE '/dashboard/ai-insight/%analysis/%';
