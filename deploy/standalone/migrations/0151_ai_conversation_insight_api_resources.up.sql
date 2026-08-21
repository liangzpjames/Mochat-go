-- Backfill API permission resources for existing installations that already
-- applied 0148 before the workspace endpoints were added to its seed block.
INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', seed.`http_method`, seed.`path_pattern`, 1, 1, 1
FROM `mochat_go_dashboard_permissions` p
JOIN (
  SELECT 'dashboard.ai_insight.session_analysis' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/ai-insight/session-analysis/records' AS `path_pattern`
  UNION ALL SELECT 'dashboard.ai_insight.session_analysis', 'GET', '/dashboard/ai-insight/session-analysis/detail'
  UNION ALL SELECT 'dashboard.ai_insight.session_analysis', 'GET', '/dashboard/ai-insight/session-analysis/status'
  UNION ALL SELECT 'dashboard.ai_insight.session_analysis', 'GET', '/dashboard/ai-insight/session-analysis/export'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'GET', '/dashboard/ai-insight/smart-analysis/records'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'GET', '/dashboard/ai-insight/smart-analysis/detail'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'GET', '/dashboard/ai-insight/smart-analysis/status'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'GET', '/dashboard/ai-insight/smart-analysis/rules'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'POST', '/dashboard/ai-insight/smart-analysis/rules'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'PUT', '/dashboard/ai-insight/smart-analysis/rules'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'DELETE', '/dashboard/ai-insight/smart-analysis/rules'
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'POST', '/dashboard/ai-insight/smart-analysis/rules/status'
) seed ON seed.`permission_code` = p.`code`
WHERE NOT EXISTS (
  SELECT 1 FROM `mochat_go_dashboard_permission_resources` existing
  WHERE existing.`permission_id` = p.`id`
    AND existing.`resource_type` = 'api'
    AND existing.`http_method` = seed.`http_method`
    AND existing.`path_pattern` = seed.`path_pattern`
);
