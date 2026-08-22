-- Reconcile the effective Dashboard page-resource seed with the authoritative
-- catalog. Existing rows are preserved and only missing exact contracts are added.
-- An earlier development overlay disabled the channel update mapping. Restore the
-- 0153-owned contract before the idempotent seed insert so historical workspaces
-- and clean databases converge on the same active resource.
UPDATE `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
SET resource.`status` = 1,
    resource.`deleted_at` = NULL,
    resource.`scope_required` = 1,
    resource.`version` = resource.`version` + 1
WHERE permission.`code` = 'dashboard.acquisition.v2_channel_code'
  AND resource.`resource_type` = 'api'
  AND resource.`http_method` = 'PUT'
  AND resource.`path_pattern` = '/dashboard/channelCode/update';

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', resource_seed.`http_method`, resource_seed.`path_pattern`, resource_seed.`scope_required`, 1, 1
FROM `mochat_go_dashboard_permissions` p
INNER JOIN (
  SELECT 'dashboard.chat.v2_all' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/workMessage/globalOverview' AS `path_pattern`, 0 AS `scope_required`
  UNION ALL SELECT 'dashboard.chat.v2_all', 'PUT', '/dashboard/workMessage/focus', 0
  UNION ALL SELECT 'dashboard.chat.v2_all', 'DELETE', '/dashboard/workMessage/focus', 0
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'GET', '/dashboard/workMessage/staffDirectory', 1
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'GET', '/dashboard/workMessage/staffDetail', 1
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'PUT', '/dashboard/workMessage/focus', 1
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'DELETE', '/dashboard/workMessage/focus', 1
  UNION ALL SELECT 'dashboard.chat.v2_customer', 'GET', '/dashboard/workMessage/customerDirectory', 1
  UNION ALL SELECT 'dashboard.chat.v2_customer', 'GET', '/dashboard/workMessage/customerConversations', 1
  UNION ALL SELECT 'dashboard.chat.v2_customer', 'GET', '/dashboard/workMessage/customerDetail', 1
  UNION ALL SELECT 'dashboard.chat.v2_customer', 'PUT', '/dashboard/workMessage/focus', 1
  UNION ALL SELECT 'dashboard.chat.v2_customer', 'DELETE', '/dashboard/workMessage/focus', 1
  UNION ALL SELECT 'dashboard.chat.v2_group', 'GET', '/dashboard/workMessage/roomDirectory', 1
  UNION ALL SELECT 'dashboard.chat.v2_group', 'GET', '/dashboard/workMessage/roomProfile', 1
  UNION ALL SELECT 'dashboard.chat.v2_group', 'GET', '/dashboard/workMessage/roomMessages', 1
  UNION ALL SELECT 'dashboard.chat.v2_group', 'GET', '/dashboard/workMessage/roomMembers', 1
  UNION ALL SELECT 'dashboard.chat.v2_group', 'GET', '/dashboard/workMessage/roomFilterOptions', 1
  UNION ALL SELECT 'dashboard.chat.trajectory', 'GET', '/dashboard/workMessage/trajectoryDay', 1
  UNION ALL SELECT 'dashboard.chat.export', 'GET', '/dashboard/workMessage/exportCandidates', 1
  UNION ALL SELECT 'dashboard.chat.export', 'GET', '/dashboard/workMessage/exportTasks', 1
  UNION ALL SELECT 'dashboard.chat.export', 'POST', '/dashboard/workMessage/exportTasks', 1
  UNION ALL SELECT 'dashboard.chat.export', 'GET', '/dashboard/workMessage/exportDownload', 1
  UNION ALL SELECT 'dashboard.customer.inheritance', 'GET', '/dashboard/contactTransfer/info', 0
  UNION ALL SELECT 'dashboard.customer.inheritance', 'GET', '/dashboard/contactTransfer/room', 0
  UNION ALL SELECT 'dashboard.customer.inheritance', 'GET', '/dashboard/contactTransfer/log', 0
  UNION ALL SELECT 'dashboard.customer.inheritance', 'GET', '/dashboard/workEmployee/index', 0
  UNION ALL SELECT 'dashboard.customer.inheritance', 'POST', '/dashboard/contactTransfer/sync', 0
  UNION ALL SELECT 'dashboard.customer.inheritance', 'POST', '/dashboard/contactTransfer/index', 0
  UNION ALL SELECT 'dashboard.customer.inheritance', 'POST', '/dashboard/contactTransfer/room', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'GET', '/dashboard/risk/records/detail', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'GET', '/dashboard/risk/scanner-status', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'GET', '/dashboard/sensitiveWordsMonitor/status', 0
  UNION ALL SELECT 'dashboard.ai_insight.session_analysis', 'GET', '/dashboard/ai-insight/session-analysis/records', 1
  UNION ALL SELECT 'dashboard.ai_insight.session_analysis', 'GET', '/dashboard/ai-insight/session-analysis/detail', 1
  UNION ALL SELECT 'dashboard.ai_insight.session_analysis', 'GET', '/dashboard/ai-insight/session-analysis/status', 1
  UNION ALL SELECT 'dashboard.ai_insight.session_analysis', 'GET', '/dashboard/ai-insight/session-analysis/export', 1
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'GET', '/dashboard/ai-insight/smart-analysis/records', 1
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'GET', '/dashboard/ai-insight/smart-analysis/detail', 1
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'GET', '/dashboard/ai-insight/smart-analysis/status', 1
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'GET', '/dashboard/ai-insight/smart-analysis/rules', 1
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'POST', '/dashboard/ai-insight/smart-analysis/rules', 1
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'PUT', '/dashboard/ai-insight/smart-analysis/rules', 1
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'DELETE', '/dashboard/ai-insight/smart-analysis/rules', 1
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'POST', '/dashboard/ai-insight/smart-analysis/rules/status', 1
) resource_seed ON resource_seed.`permission_code` = p.`code`
WHERE NOT EXISTS (
  SELECT 1
  FROM `mochat_go_dashboard_permission_resources` existing
  WHERE existing.`permission_id` = p.`id`
    AND existing.`resource_type` = 'api'
    AND existing.`http_method` = resource_seed.`http_method`
    AND existing.`path_pattern` = resource_seed.`path_pattern`
    AND existing.`status` = 1
    AND existing.`deleted_at` IS NULL
);

-- 0147 originally classified tenant-wide scanner status as employee-scoped.
-- Correct the two exact contracts without broadening any record/detail access.
UPDATE `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
SET resource.`scope_required` = 0,
    resource.`version` = resource.`version` + 1
WHERE resource.`resource_type` = 'api'
  AND resource.`status` = 1
  AND resource.`deleted_at` IS NULL
  AND (
    (permission.`code` = 'dashboard.ai_insight.v2_risk'
      AND resource.`http_method` = 'GET'
      AND resource.`path_pattern` = '/dashboard/risk/scanner-status')
    OR
    (permission.`code` = 'dashboard.ai_insight.v2_sensitive_word'
      AND resource.`http_method` = 'GET'
      AND resource.`path_pattern` = '/dashboard/sensitiveWordsMonitor/status')
  );

-- Deactivate exact stale grants. Media writes were disabled by 0145 and must
-- remain disabled; export now uses the task endpoints seeded above.
UPDATE `mochat_go_dashboard_permission_resources` resource
INNER JOIN `mochat_go_dashboard_permissions` permission ON permission.`id` = resource.`permission_id`
INNER JOIN (
  SELECT 'dashboard.chat.export' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/workMessage/toUsers' AS `path_pattern`
  UNION ALL SELECT 'dashboard.chat.export', 'GET', '/dashboard/workMessage/detail'
  UNION ALL SELECT 'dashboard.chat.file_audio', 'POST', '/dashboard/chat/media'
  UNION ALL SELECT 'dashboard.chat.file_audio', 'DELETE', '/dashboard/chat/media/{id}'
) deactivation_seed ON deactivation_seed.`permission_code` = permission.`code`
  AND deactivation_seed.`http_method` = resource.`http_method`
  AND deactivation_seed.`path_pattern` = resource.`path_pattern`
SET resource.`status` = 0,
    resource.`deleted_at` = COALESCE(resource.`deleted_at`, CURRENT_TIMESTAMP),
    resource.`version` = resource.`version` + 1
WHERE resource.`resource_type` = 'api'
  AND resource.`status` = 1
  AND resource.`deleted_at` IS NULL;
