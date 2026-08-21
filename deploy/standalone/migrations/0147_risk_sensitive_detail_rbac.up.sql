INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', seed.`http_method`, seed.`path_pattern`, seed.`scope_required`, 1, 1
FROM `mochat_go_dashboard_permissions` p
INNER JOIN (
  SELECT 'dashboard.ai_insight.v2_risk' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/risk/records/detail' AS `path_pattern`, 1 AS `scope_required`
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'GET', '/dashboard/risk/scanner-status', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'GET', '/dashboard/sensitiveWordsMonitor/status', 1
) seed ON seed.`permission_code` = p.`code`
ON DUPLICATE KEY UPDATE `scope_required` = VALUES(`scope_required`), `status` = 1, `version` = 1;
