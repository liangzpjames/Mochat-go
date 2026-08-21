DELETE r
FROM `mochat_go_dashboard_permission_resources` r
INNER JOIN `mochat_go_dashboard_permissions` p ON p.`id` = r.`permission_id`
WHERE p.`code` IN ('dashboard.ai_insight.v2_risk', 'dashboard.ai_insight.v2_sensitive_word')
  AND r.`http_method` = 'GET'
  AND r.`path_pattern` IN ('/dashboard/risk/records/detail', '/dashboard/risk/scanner-status', '/dashboard/sensitiveWordsMonitor/status');
