-- Phase 4 Dashboard page RBAC.
-- Data preflights intentionally precede the first DDL because MariaDB/MySQL DDL implicitly commits.

SET @dashboard_missing_subscription_count := (
  SELECT COUNT(*)
  FROM `mochat_go_saas_tenant_packages` p
  LEFT JOIN `mochat_go_saas_subscriptions` s
    ON s.`tenant_id` = p.`tenant_id`
   AND s.`deleted_at` IS NULL
  WHERE p.`deleted_at` IS NULL
    AND p.`status` = 1
    AND (p.`starts_at` IS NULL OR p.`starts_at` <= NOW())
    AND (p.`expires_at` IS NULL OR p.`expires_at` > NOW())
    AND s.`id` IS NULL
);
SET @dashboard_subscription_guard_sql := IF(
  @dashboard_missing_subscription_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0127 missing subscription for active tenant package'''
);
PREPARE dashboard_subscription_guard_stmt FROM @dashboard_subscription_guard_sql;
EXECUTE dashboard_subscription_guard_stmt;
DEALLOCATE PREPARE dashboard_subscription_guard_stmt;

SET @dashboard_dangling_legacy_role_count := (
  SELECT COUNT(*)
  FROM `mc_rbac_user_role` ur
  LEFT JOIN `mc_user` u ON u.`id` = CAST(ur.`user_id` AS UNSIGNED)
  LEFT JOIN `mc_rbac_role` r ON r.`id` = ur.`role_id`
  WHERE ur.`deleted_at` IS NULL
    AND (u.`id` IS NULL OR r.`id` IS NULL)
);
SET @dashboard_dangling_role_guard_sql := IF(
  @dashboard_dangling_legacy_role_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0127 dangling legacy user-role relationship'''
);
PREPARE dashboard_dangling_role_guard_stmt FROM @dashboard_dangling_role_guard_sql;
EXECUTE dashboard_dangling_role_guard_stmt;
DEALLOCATE PREPARE dashboard_dangling_role_guard_stmt;

SET @dashboard_cross_tenant_legacy_role_count := (
  SELECT COUNT(*)
  FROM `mc_rbac_user_role` ur
  INNER JOIN `mc_user` u ON u.`id` = CAST(ur.`user_id` AS UNSIGNED)
  INNER JOIN `mc_rbac_role` r ON r.`id` = ur.`role_id`
  WHERE ur.`deleted_at` IS NULL
    AND u.`deleted_at` IS NULL
    AND r.`deleted_at` IS NULL
    AND u.`tenant_id` <> r.`tenant_id`
);
SET @dashboard_legacy_role_guard_sql := IF(
  @dashboard_cross_tenant_legacy_role_count = 0,
  'SELECT 1',
  'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0127 cross-tenant legacy user-role relationship'''
);
PREPARE dashboard_legacy_role_guard_stmt FROM @dashboard_legacy_role_guard_sql;
EXECUTE dashboard_legacy_role_guard_stmt;
DEALLOCATE PREPARE dashboard_legacy_role_guard_stmt;

ALTER TABLE `mc_user`
  ADD COLUMN `dashboard_access_version` bigint(20) unsigned NOT NULL DEFAULT 1 AFTER `isSuperAdmin`,
  ADD UNIQUE INDEX `uni_dashboard_user_tenant_id_id` (`tenant_id`, `id`);

ALTER TABLE `mc_rbac_role`
  ADD COLUMN `dashboard_access_version` bigint(20) unsigned NOT NULL DEFAULT 1 AFTER `data_permission`,
  ADD UNIQUE INDEX `uni_dashboard_role_tenant_id_id` (`tenant_id`, `id`);

CREATE TABLE `mochat_go_dashboard_permissions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL,
  `permission_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'page',
  `path` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL,
  `group_code` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `sort` int(11) NOT NULL DEFAULT 0,
  `restriction` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'grantable',
  `superadmin_only` tinyint(1) NOT NULL DEFAULT 0,
  `status` tinyint(4) NOT NULL DEFAULT 1,
  `version` bigint(20) unsigned NOT NULL DEFAULT 1,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_dashboard_permissions_code` (`code`),
  UNIQUE KEY `uni_dashboard_permissions_path` (`path`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard page permission catalog';

CREATE TABLE `mochat_go_dashboard_permission_resources` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `permission_id` bigint(20) unsigned NOT NULL,
  `resource_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'api',
  `http_method` varchar(10) COLLATE utf8mb4_unicode_ci NOT NULL,
  `path_pattern` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL,
  `scope_required` tinyint(1) NOT NULL DEFAULT 0,
  `status` tinyint(4) NOT NULL DEFAULT 1,
  `version` bigint(20) unsigned NOT NULL DEFAULT 1,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_dashboard_permission_resource` (`http_method`, `path_pattern`, `permission_id`),
  KEY `idx_dashboard_permission_resource_match` (`http_method`, `path_pattern`, `status`),
  CONSTRAINT `fk_dashboard_permission_resource_permission` FOREIGN KEY (`permission_id`) REFERENCES `mochat_go_dashboard_permissions` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard API to page permission mapping';

CREATE TABLE `mochat_go_dashboard_user_roles` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(11) NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `role_id` int(11) NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_dashboard_user_roles` (`tenant_id`, `user_id`, `role_id`),
  KEY `idx_dashboard_user_roles_role` (`tenant_id`, `role_id`, `user_id`),
  CONSTRAINT `fk_dashboard_user_roles_user` FOREIGN KEY (`tenant_id`, `user_id`) REFERENCES `mc_user` (`tenant_id`, `id`),
  CONSTRAINT `fk_dashboard_user_roles_role` FOREIGN KEY (`tenant_id`, `role_id`) REFERENCES `mc_rbac_role` (`tenant_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard tenant user multi-role relation';

CREATE TABLE `mochat_go_dashboard_role_permissions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(11) NOT NULL,
  `role_id` int(11) NOT NULL,
  `permission_id` bigint(20) unsigned NOT NULL,
  `data_scope` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'self',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_dashboard_role_permissions` (`tenant_id`, `role_id`, `permission_id`),
  CONSTRAINT `fk_dashboard_role_permissions_role` FOREIGN KEY (`tenant_id`, `role_id`) REFERENCES `mc_rbac_role` (`tenant_id`, `id`),
  CONSTRAINT `fk_dashboard_role_permissions_permission` FOREIGN KEY (`permission_id`) REFERENCES `mochat_go_dashboard_permissions` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard tenant role permissions';

CREATE TABLE `mochat_go_dashboard_user_permissions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(11) NOT NULL,
  `user_id` int(10) unsigned NOT NULL,
  `permission_id` bigint(20) unsigned NOT NULL,
  `effect` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'allow',
  `data_scope` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'self',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_dashboard_user_permissions` (`tenant_id`, `user_id`, `permission_id`),
  CONSTRAINT `fk_dashboard_user_permissions_user` FOREIGN KEY (`tenant_id`, `user_id`) REFERENCES `mc_user` (`tenant_id`, `id`),
  CONSTRAINT `fk_dashboard_user_permissions_permission` FOREIGN KEY (`permission_id`) REFERENCES `mochat_go_dashboard_permissions` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Dashboard tenant direct user permissions';

CREATE TABLE `mochat_go_dashboard_permission_audits` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(11) NOT NULL,
  `actor_user_id` int(10) unsigned NULL,
  `action` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `target_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `target_id` varchar(64) NOT NULL,
  `before_json` json DEFAULT NULL,
  `after_json` json DEFAULT NULL,
  `expected_version` bigint(20) unsigned DEFAULT NULL,
  `result_version` bigint(20) unsigned DEFAULT NULL,
  `request_id` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_dashboard_permission_audits_tenant_time` (`tenant_id`, `created_at`, `id`),
  KEY `idx_dashboard_permission_audits_target` (`tenant_id`, `target_type`, `target_id`, `id`),
  CONSTRAINT `fk_dashboard_audit_actor` FOREIGN KEY (`tenant_id`, `actor_user_id`) REFERENCES `mc_user` (`tenant_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Append-only Dashboard permission audit';

INSERT INTO `mochat_go_dashboard_permissions`
  (`code`, `permission_type`, `path`, `name`, `group_code`, `sort`, `restriction`, `superadmin_only`, `status`, `version`)
VALUES
  ('dashboard.index', 'page', '/index', '数据概览', NULL, 1, 'grantable', 0, 1, 1),
  ('dashboard.chat.v2_all', 'page', '/chat/v2-all', '全局消息', 'conversation', 2, 'grantable', 0, 1, 1),
  ('dashboard.chat.v2_staff', 'page', '/chat/v2-staff', '员工会话', 'conversation', 3, 'grantable', 0, 1, 1),
  ('dashboard.chat.v2_customer', 'page', '/chat/v2-customer', '客户会话', 'conversation', 4, 'grantable', 0, 1, 1),
  ('dashboard.chat.v2_group', 'page', '/chat/v2-group', '群聊会话', 'conversation', 5, 'grantable', 0, 1, 1),
  ('dashboard.chat.trajectory', 'page', '/chat/trajectory', '会话轨迹', 'conversation', 6, 'grantable', 0, 1, 1),
  ('dashboard.chat.export', 'page', '/chat/export', '会话导出', 'conversation', 7, 'grantable', 0, 1, 1),
  ('dashboard.chat.file_audio', 'page', '/chat/file-audio', '文件录音', 'conversation', 8, 'grantable', 0, 1, 1),
  ('dashboard.chat.resign_staff', 'page', '/chat/resign-staff', '离职员工', 'conversation', 9, 'grantable', 0, 1, 1),
  ('dashboard.chat.refuse_archive', 'page', '/chat/refuse-archive', '拒绝存档', 'conversation', 10, 'grantable', 0, 1, 1),
  ('dashboard.customer.inheritance', 'page', '/customer/inheritance', '客户继承', 'conversation', 11, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.v2_risk', 'page', '/ai-insight/v2/risk', '风险行为', 'risk-warning', 12, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.v2_sensitive_word', 'page', '/ai-insight/v2/sensitive-word', '敏感词', 'risk-warning', 13, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.v2_timeout', 'page', '/ai-insight/v2/timeout', '超时预警', 'risk-warning', 14, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.v2_customer_loss', 'page', '/ai-insight/v2/customer-loss', '客户流失', 'risk-warning', 15, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.v2_message_intercept', 'page', '/ai-insight/v2/message-intercept', '消息拦截', 'risk-warning', 16, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.v2_keyword_library', 'page', '/ai-insight/v2/keyword-library', '关键词库', 'risk-warning', 17, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.v2_silent_customer', 'page', '/ai-insight/v2/silent-customer', '沉默客户', 'risk-warning', 18, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.session_analysis', 'page', '/ai-insight/session-analysis', '会话分析', 'ai-insight', 19, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.smart_analysis', 'page', '/ai-insight/smart-analysis', '智能分析', 'ai-insight', 20, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.emotion', 'page', '/ai-insight/emotion', '情绪识别', 'ai-insight', 21, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.employee_score', 'page', '/ai-insight/employee-score', '员工评分', 'ai-insight', 22, 'grantable', 0, 1, 1),
  ('dashboard.ai_insight.communication_keyword', 'page', '/ai-insight/communication-keyword', '沟通关键词', 'ai-insight', 23, 'grantable', 0, 1, 1),
  ('dashboard.acquisition.v2_channel_code', 'page', '/acquisition/v2-channel-code', '渠道活码', 'marketing-tools', 24, 'grantable', 0, 1, 1),
  ('dashboard.acquisition.group_code', 'page', '/acquisition/group-code', '群活码', 'marketing-tools', 25, 'grantable', 0, 1, 1),
  ('dashboard.acquisition.redirect_link', 'page', '/acquisition/redirect-link', '获客链接', 'marketing-tools', 26, 'grantable', 0, 1, 1),
  ('dashboard.acquisition.wechat_customer_service', 'page', '/acquisition/wechat-customer-service', '微信客服', 'marketing-tools', 27, 'grantable', 0, 1, 1),
  ('dashboard.acquisition.live_code_short_chain', 'page', '/acquisition/live-code-short-chain', '活码短链', 'marketing-tools', 28, 'grantable', 0, 1, 1),
  ('dashboard.acquisition.group_template', 'page', '/acquisition/group-template', '一键加群', 'marketing-tools', 29, 'grantable', 0, 1, 1),
  ('dashboard.acquisition.precise_group_send', 'page', '/acquisition/precise-group-send', '精准群发', 'marketing-tools', 30, 'grantable', 0, 1, 1),
  ('dashboard.acquisition.friends_circle', 'page', '/acquisition/friends-circle', '朋友圈', 'marketing-tools', 31, 'grantable', 0, 1, 1),
  ('dashboard.acquisition.material_management', 'page', '/acquisition/material-management', '素材管理', 'marketing-tools', 32, 'grantable', 0, 1, 1),
  ('dashboard.customer.clue_default', 'page', '/customer/clue/default', '线索池', 'scrm', 33, 'grantable', 0, 1, 1),
  ('dashboard.customer.contact', 'page', '/customer/contact', '联系人', 'scrm', 34, 'grantable', 0, 1, 1),
  ('dashboard.customer.friends', 'page', '/customer/friends', '好友', 'scrm', 35, 'grantable', 0, 1, 1),
  ('dashboard.customer.opportunity', 'page', '/customer/opportunity', '商机', 'scrm', 36, 'grantable', 0, 1, 1),
  ('dashboard.customer.public_sea', 'page', '/customer/public-sea', '公海', 'scrm', 37, 'grantable', 0, 1, 1),
  ('dashboard.customer.group', 'page', '/customer/group', '客户群', 'scrm', 38, 'grantable', 0, 1, 1),
  ('dashboard.customer.tags', 'page', '/customer/tags', '标签', 'scrm', 39, 'grantable', 0, 1, 1),
  ('dashboard.customer.order', 'page', '/customer/order', '订单', 'scrm', 40, 'grantable', 0, 1, 1),
  ('dashboard.customer.settings', 'page', '/customer/settings', '设置', 'scrm', 41, 'grantable', 0, 1, 1),
  ('dashboard.data.customer', 'page', '/data/customer', '客户分析', 'data-reports', 42, 'grantable', 0, 1, 1),
  ('dashboard.data.employee', 'page', '/data/employee', '会话分析', 'data-reports', 43, 'grantable', 0, 1, 1),
  ('dashboard.data.conversion', 'page', '/data/conversion', '转化分析', 'data-reports', 44, 'grantable', 0, 1, 1),
  ('dashboard.data.behavior', 'page', '/data/behavior', '行为分析', 'data-reports', 45, 'grantable', 0, 1, 1),
  ('dashboard.data.report', 'page', '/data/report', '综合报表', 'data-reports', 46, 'grantable', 0, 1, 1),
  ('dashboard.ai_setting.ai_knowledge_base', 'page', '/ai-setting/ai-knowledge-base', 'AI 知识库', 'ai-settings', 47, 'grantable', 0, 1, 1),
  ('dashboard.ai_setting.agent', 'page', '/ai-setting/agent', '智能体管理', 'ai-settings', 48, 'grantable', 0, 1, 1),
  ('dashboard.company_setting.website', 'page', '/company-setting/website', '企业信息', 'company-settings', 49, 'grantable', 0, 1, 1),
  ('dashboard.company_setting.staff', 'page', '/company-setting/staff', '员工权限', 'company-settings', 50, 'superadmin_only', 1, 1, 1),
  ('dashboard.setting.role', 'page', '/setting/role', '角色管理', 'company-settings', 51, 'superadmin_only', 1, 1, 1),
  ('dashboard.setting.additional', 'page', '/setting/additional', '附加权限', 'company-settings', 52, 'superadmin_only', 1, 1, 1),
  ('dashboard.setting.authorization', 'page', '/setting/authorization', '授权管理', 'company-settings', 53, 'superadmin_only', 1, 1, 1);

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', resource_seed.`http_method`, resource_seed.`path_pattern`, resource_seed.`scope_required`, 1, 1
FROM `mochat_go_dashboard_permissions` p
INNER JOIN (
  SELECT 'dashboard.index' AS `permission_code`, 'GET' AS `http_method`, '/dashboard/reports/overview' AS `path_pattern`, 1 AS `scope_required`
  UNION ALL SELECT 'dashboard.index', 'GET', '/dashboard/workEmployee/index', 0
  UNION ALL SELECT 'dashboard.index', 'GET', '/dashboard/workDepartment/pageIndex', 0
  UNION ALL SELECT 'dashboard.chat.v2_all', 'GET', '/dashboard/workMessage/toUsers', 0
  UNION ALL SELECT 'dashboard.chat.v2_all', 'GET', '/dashboard/workMessage/detail', 0
  UNION ALL SELECT 'dashboard.chat.v2_all', 'GET', '/dashboard/workMessage/fromUsers', 0
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'GET', '/dashboard/workMessage/toUsers', 1
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'GET', '/dashboard/workMessage/detail', 1
  UNION ALL SELECT 'dashboard.chat.v2_staff', 'GET', '/dashboard/workMessage/fromUsers', 1
  UNION ALL SELECT 'dashboard.chat.v2_customer', 'GET', '/dashboard/workMessage/toUsers', 1
  UNION ALL SELECT 'dashboard.chat.v2_customer', 'GET', '/dashboard/workMessage/detail', 1
  UNION ALL SELECT 'dashboard.chat.v2_customer', 'GET', '/dashboard/workMessage/fromUsers', 1
  UNION ALL SELECT 'dashboard.chat.v2_group', 'GET', '/dashboard/workMessage/toUsers', 1
  UNION ALL SELECT 'dashboard.chat.v2_group', 'GET', '/dashboard/workMessage/detail', 1
  UNION ALL SELECT 'dashboard.chat.v2_group', 'GET', '/dashboard/workMessage/fromUsers', 1
  UNION ALL SELECT 'dashboard.chat.trajectory', 'GET', '/dashboard/workMessage/toUsers', 1
  UNION ALL SELECT 'dashboard.chat.trajectory', 'GET', '/dashboard/workMessage/detail', 1
  UNION ALL SELECT 'dashboard.chat.trajectory', 'GET', '/dashboard/workMessage/fromUsers', 0
  UNION ALL SELECT 'dashboard.chat.export', 'GET', '/dashboard/workMessage/toUsers', 1
  UNION ALL SELECT 'dashboard.chat.export', 'GET', '/dashboard/workMessage/detail', 1
  UNION ALL SELECT 'dashboard.chat.file_audio', 'GET', '/dashboard/chat/media', 0
  UNION ALL SELECT 'dashboard.chat.file_audio', 'POST', '/dashboard/chat/media', 0
  UNION ALL SELECT 'dashboard.chat.file_audio', 'DELETE', '/dashboard/chat/media/{id}', 0
  UNION ALL SELECT 'dashboard.chat.resign_staff', 'GET', '/dashboard/contactTransfer/info', 0
  UNION ALL SELECT 'dashboard.chat.refuse_archive', 'GET', '/dashboard/refuse-archive/records', 0
  UNION ALL SELECT 'dashboard.chat.refuse_archive', 'POST', '/dashboard/refuse-archive/follow-up', 0
  UNION ALL SELECT 'dashboard.customer.inheritance', 'GET', '/dashboard/contactTransfer/unassignedList', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'GET', '/dashboard/risk/records', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'GET', '/dashboard/risk/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'POST', '/dashboard/risk/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'PUT', '/dashboard/risk/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'PUT', '/dashboard/risk/rules/status', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'DELETE', '/dashboard/risk/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_risk', 'POST', '/dashboard/risk/records/audit', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'GET', '/dashboard/sensitiveWord/index', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'GET', '/dashboard/sensitiveWordGroup/select', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'POST', '/dashboard/sensitiveWord/store', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'PUT', '/dashboard/sensitiveWord/statusUpdate', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'PUT', '/dashboard/sensitiveWord/move', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'DELETE', '/dashboard/sensitiveWord/destroy', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'POST', '/dashboard/sensitiveWordGroup/store', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'PUT', '/dashboard/sensitiveWordGroup/update', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'GET', '/dashboard/sensitiveWordsMonitor/index', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_sensitive_word', 'GET', '/dashboard/sensitiveWordsMonitor/show', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'GET', '/dashboard/timeout-warning/records', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'GET', '/dashboard/timeout-warning/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'GET', '/dashboard/timeout-warning/settings', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'POST', '/dashboard/timeout-warning/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'PUT', '/dashboard/timeout-warning/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'PUT', '/dashboard/timeout-warning/rules/status', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'DELETE', '/dashboard/timeout-warning/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'POST', '/dashboard/timeout-warning/records/audit', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'PUT', '/dashboard/timeout-warning/records/assign', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_timeout', 'PUT', '/dashboard/timeout-warning/settings', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_customer_loss', 'GET', '/dashboard/workContact/lossContact', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_message_intercept', 'GET', '/dashboard/message-intercept/records', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_message_intercept', 'GET', '/dashboard/message-intercept/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_message_intercept', 'POST', '/dashboard/message-intercept/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_message_intercept', 'PUT', '/dashboard/message-intercept/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_message_intercept', 'PUT', '/dashboard/message-intercept/rules/status', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_message_intercept', 'DELETE', '/dashboard/message-intercept/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_message_intercept', 'POST', '/dashboard/message-intercept/records/audit', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_message_intercept', 'GET', '/dashboard/keyword-library/libraries', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'GET', '/dashboard/keyword-library/libraries', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'POST', '/dashboard/keyword-library/libraries', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'PUT', '/dashboard/keyword-library/libraries', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'PUT', '/dashboard/keyword-library/libraries/status', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'POST', '/dashboard/keyword-library/libraries/publish', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'DELETE', '/dashboard/keyword-library/libraries', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'GET', '/dashboard/keyword-library/entries', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'POST', '/dashboard/keyword-library/entries', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'PUT', '/dashboard/keyword-library/entries', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'PUT', '/dashboard/keyword-library/entries/status', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_keyword_library', 'DELETE', '/dashboard/keyword-library/entries', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_silent_customer', 'GET', '/dashboard/silent-customer/records', 1
  UNION ALL SELECT 'dashboard.ai_insight.v2_silent_customer', 'GET', '/dashboard/silent-customer/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_silent_customer', 'POST', '/dashboard/silent-customer/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_silent_customer', 'PUT', '/dashboard/silent-customer/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_silent_customer', 'PUT', '/dashboard/silent-customer/rules/status', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_silent_customer', 'DELETE', '/dashboard/silent-customer/rules', 0
  UNION ALL SELECT 'dashboard.ai_insight.v2_silent_customer', 'POST', '/dashboard/silent-customer/records/action', 1
  UNION ALL SELECT 'dashboard.ai_insight.session_analysis', 'GET', '/dashboard/ai-insight/session-analysis', 1
  UNION ALL SELECT 'dashboard.ai_insight.smart_analysis', 'GET', '/dashboard/ai-insight/smart-analysis', 1
  UNION ALL SELECT 'dashboard.ai_insight.emotion', 'GET', '/dashboard/ai-insight/emotion', 1
  UNION ALL SELECT 'dashboard.ai_insight.employee_score', 'GET', '/dashboard/ai-insight/employee-score', 1
  UNION ALL SELECT 'dashboard.ai_insight.communication_keyword', 'GET', '/dashboard/ai-insight/communication-keyword', 1
  UNION ALL SELECT 'dashboard.acquisition.v2_channel_code', 'GET', '/dashboard/channelCode/index', 1
  UNION ALL SELECT 'dashboard.acquisition.v2_channel_code', 'POST', '/dashboard/channelCode/store', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'GET', '/dashboard/workRoomAutoPull/index', 1
  UNION ALL SELECT 'dashboard.acquisition.group_code', 'POST', '/dashboard/workRoomAutoPull/store', 1
  UNION ALL SELECT 'dashboard.acquisition.redirect_link', 'GET', '/dashboard/acquisitionLink/index', 0
  UNION ALL SELECT 'dashboard.acquisition.redirect_link', 'POST', '/dashboard/acquisitionLink/store', 0
  UNION ALL SELECT 'dashboard.acquisition.redirect_link', 'POST', '/dashboard/acquisitionLink/authorize', 0
  UNION ALL SELECT 'dashboard.acquisition.wechat_customer_service', 'GET', '/dashboard/customerService/index', 1
  UNION ALL SELECT 'dashboard.acquisition.wechat_customer_service', 'POST', '/dashboard/customerService/store', 1
  UNION ALL SELECT 'dashboard.acquisition.wechat_customer_service', 'POST', '/dashboard/customerService/sync', 1
  UNION ALL SELECT 'dashboard.acquisition.live_code_short_chain', 'GET', '/dashboard/liveCodeShortChain/index', 0
  UNION ALL SELECT 'dashboard.acquisition.live_code_short_chain', 'POST', '/dashboard/liveCodeShortChain/store', 0
  UNION ALL SELECT 'dashboard.acquisition.live_code_short_chain', 'POST', '/dashboard/liveCodeShortChain/disable', 0
  UNION ALL SELECT 'dashboard.acquisition.group_template', 'GET', '/dashboard/workRoomAutoPull/index', 1
  UNION ALL SELECT 'dashboard.acquisition.group_template', 'POST', '/dashboard/workRoomAutoPull/store', 1
  UNION ALL SELECT 'dashboard.acquisition.group_template', 'GET', '/dashboard/materialSelector/index', 0
  UNION ALL SELECT 'dashboard.acquisition.precise_group_send', 'GET', '/dashboard/contactMessageBatchSend/index', 1
  UNION ALL SELECT 'dashboard.acquisition.precise_group_send', 'POST', '/dashboard/contactMessageBatchSend/store', 1
  UNION ALL SELECT 'dashboard.acquisition.precise_group_send', 'GET', '/dashboard/roomMessageBatchSend/index', 1
  UNION ALL SELECT 'dashboard.acquisition.precise_group_send', 'POST', '/dashboard/roomMessageBatchSend/store', 1
  UNION ALL SELECT 'dashboard.acquisition.precise_group_send', 'GET', '/dashboard/materialSelector/index', 0
  UNION ALL SELECT 'dashboard.acquisition.friends_circle', 'GET', '/dashboard/friendsCircle/taskIndex', 1
  UNION ALL SELECT 'dashboard.acquisition.friends_circle', 'GET', '/dashboard/friendsCircle/materialIndex', 0
  UNION ALL SELECT 'dashboard.acquisition.friends_circle', 'GET', '/dashboard/friendsCircle/taskResultIndex', 1
  UNION ALL SELECT 'dashboard.acquisition.friends_circle', 'GET', '/dashboard/friendsCircle/exportData', 1
  UNION ALL SELECT 'dashboard.acquisition.friends_circle', 'POST', '/dashboard/friendsCircle/publish', 1
  UNION ALL SELECT 'dashboard.acquisition.friends_circle', 'POST', '/dashboard/friendsCircle/taskStore', 1
  UNION ALL SELECT 'dashboard.acquisition.friends_circle', 'POST', '/dashboard/friendsCircle/materialStore', 0
  UNION ALL SELECT 'dashboard.acquisition.friends_circle', 'GET', '/dashboard/materialSelector/index', 0
  UNION ALL SELECT 'dashboard.acquisition.material_management', 'GET', '/dashboard/mediumGroup/index', 0
  UNION ALL SELECT 'dashboard.acquisition.material_management', 'POST', '/dashboard/mediumGroup/store', 0
  UNION ALL SELECT 'dashboard.acquisition.material_management', 'GET', '/dashboard/medium/index', 0
  UNION ALL SELECT 'dashboard.acquisition.material_management', 'POST', '/dashboard/medium/store', 0
  UNION ALL SELECT 'dashboard.acquisition.material_management', 'POST', '/dashboard/medium/batchGroupUpdate', 0
  UNION ALL SELECT 'dashboard.acquisition.material_management', 'POST', '/dashboard/medium/batchDestroy', 0
  UNION ALL SELECT 'dashboard.acquisition.material_management', 'GET', '/dashboard/workDepartment/pageIndex', 0
  UNION ALL SELECT 'dashboard.customer.clue_default', 'GET', '/dashboard/workEmployee/index', 1
  UNION ALL SELECT 'dashboard.customer.clue_default', 'GET', '/dashboard/scrm/leads', 1
  UNION ALL SELECT 'dashboard.customer.clue_default', 'POST', '/dashboard/scrm/leads', 1
  UNION ALL SELECT 'dashboard.customer.clue_default', 'GET', '/dashboard/scrm/leads/duplicates', 1
  UNION ALL SELECT 'dashboard.customer.clue_default', 'POST', '/dashboard/scrm/leads/assignments', 1
  UNION ALL SELECT 'dashboard.customer.clue_default', 'POST', '/dashboard/scrm/leads/transition', 1
  UNION ALL SELECT 'dashboard.customer.contact', 'GET', '/dashboard/scrm/contacts', 1
  UNION ALL SELECT 'dashboard.customer.contact', 'GET', '/dashboard/scrm/contacts/{id}', 1
  UNION ALL SELECT 'dashboard.customer.contact', 'POST', '/dashboard/scrm/contacts/{id}/follow-ups', 1
  UNION ALL SELECT 'dashboard.customer.contact', 'GET', '/dashboard/scrm/contacts/{id}/follow-ups', 1
  UNION ALL SELECT 'dashboard.customer.contact', 'PUT', '/dashboard/scrm/assignments', 1
  UNION ALL SELECT 'dashboard.customer.contact', 'POST', '/dashboard/scrm/assignments/release', 1
  UNION ALL SELECT 'dashboard.customer.contact', 'GET', '/dashboard/scrm/tags', 0
  UNION ALL SELECT 'dashboard.customer.contact', 'PUT', '/dashboard/scrm/tags/{id}/contacts', 1
  UNION ALL SELECT 'dashboard.customer.contact', 'POST', '/dashboard/scrm/opportunities', 1
  UNION ALL SELECT 'dashboard.customer.friends', 'GET', '/dashboard/workContact/index', 1
  UNION ALL SELECT 'dashboard.customer.friends', 'GET', '/dashboard/workContact/show', 1
  UNION ALL SELECT 'dashboard.customer.opportunity', 'GET', '/dashboard/scrm/opportunities', 1
  UNION ALL SELECT 'dashboard.customer.opportunity', 'POST', '/dashboard/scrm/opportunities', 1
  UNION ALL SELECT 'dashboard.customer.opportunity', 'POST', '/dashboard/scrm/opportunities/{id}/stage', 1
  UNION ALL SELECT 'dashboard.customer.opportunity', 'GET', '/dashboard/scrm/contacts/{id}/follow-ups', 1
  UNION ALL SELECT 'dashboard.customer.opportunity', 'POST', '/dashboard/scrm/contacts/{id}/follow-ups', 1
  UNION ALL SELECT 'dashboard.customer.public_sea', 'GET', '/dashboard/scrm/assignments', 1
  UNION ALL SELECT 'dashboard.customer.public_sea', 'PUT', '/dashboard/scrm/assignments', 1
  UNION ALL SELECT 'dashboard.customer.public_sea', 'POST', '/dashboard/scrm/assignments/release', 1
  UNION ALL SELECT 'dashboard.customer.public_sea', 'POST', '/dashboard/scrm/assignments/claim', 1
  UNION ALL SELECT 'dashboard.customer.public_sea', 'POST', '/dashboard/scrm/assignments/claim/batch', 1
  UNION ALL SELECT 'dashboard.customer.public_sea', 'GET', '/dashboard/scrm/contacts', 1
  UNION ALL SELECT 'dashboard.customer.public_sea', 'GET', '/dashboard/workEmployee/index', 1
  UNION ALL SELECT 'dashboard.customer.public_sea', 'GET', '/dashboard/scrm/settings', 0
  UNION ALL SELECT 'dashboard.customer.group', 'GET', '/dashboard/workRoom/index', 1
  UNION ALL SELECT 'dashboard.customer.group', 'GET', '/dashboard/workRoom/roomIndex', 1
  UNION ALL SELECT 'dashboard.customer.tags', 'GET', '/dashboard/scrm/tags', 0
  UNION ALL SELECT 'dashboard.customer.tags', 'POST', '/dashboard/scrm/tags', 0
  UNION ALL SELECT 'dashboard.customer.tags', 'PUT', '/dashboard/scrm/tags/{id}', 0
  UNION ALL SELECT 'dashboard.customer.tags', 'DELETE', '/dashboard/scrm/tags/{id}', 0
  UNION ALL SELECT 'dashboard.customer.tags', 'POST', '/dashboard/scrm/tags/{id}/move', 0
  UNION ALL SELECT 'dashboard.customer.tags', 'GET', '/dashboard/scrm/tags/{id}/delete-preview', 0
  UNION ALL SELECT 'dashboard.customer.tags', 'PUT', '/dashboard/scrm/tags/{id}/contacts', 1
  UNION ALL SELECT 'dashboard.customer.tags', 'POST', '/dashboard/scrm/tag-groups', 0
  UNION ALL SELECT 'dashboard.customer.tags', 'PUT', '/dashboard/scrm/tag-groups/{id}', 0
  UNION ALL SELECT 'dashboard.customer.order', 'GET', '/dashboard/scrm/orders', 1
  UNION ALL SELECT 'dashboard.customer.order', 'POST', '/dashboard/scrm/orders', 1
  UNION ALL SELECT 'dashboard.customer.order', 'GET', '/dashboard/scrm/orders/{id}', 1
  UNION ALL SELECT 'dashboard.customer.order', 'PUT', '/dashboard/scrm/orders/{id}/transition', 1
  UNION ALL SELECT 'dashboard.customer.order', 'GET', '/dashboard/scrm/contacts', 1
  UNION ALL SELECT 'dashboard.customer.order', 'POST', '/dashboard/scrm/contacts', 1
  UNION ALL SELECT 'dashboard.customer.order', 'GET', '/dashboard/scrm/opportunities', 1
  UNION ALL SELECT 'dashboard.customer.settings', 'GET', '/dashboard/scrm/settings', 0
  UNION ALL SELECT 'dashboard.customer.settings', 'PUT', '/dashboard/scrm/settings', 0
  UNION ALL SELECT 'dashboard.data.customer', 'GET', '/dashboard/reports/customer', 1
  UNION ALL SELECT 'dashboard.data.customer', 'GET', '/dashboard/workEmployee/index', 1
  UNION ALL SELECT 'dashboard.data.customer', 'GET', '/dashboard/workDepartment/pageIndex', 0
  UNION ALL SELECT 'dashboard.data.employee', 'GET', '/dashboard/reports/employee', 1
  UNION ALL SELECT 'dashboard.data.employee', 'GET', '/dashboard/workEmployee/index', 1
  UNION ALL SELECT 'dashboard.data.employee', 'GET', '/dashboard/workDepartment/pageIndex', 0
  UNION ALL SELECT 'dashboard.data.conversion', 'GET', '/dashboard/reports/conversion', 1
  UNION ALL SELECT 'dashboard.data.conversion', 'GET', '/dashboard/workEmployee/index', 1
  UNION ALL SELECT 'dashboard.data.conversion', 'GET', '/dashboard/workDepartment/pageIndex', 0
  UNION ALL SELECT 'dashboard.data.behavior', 'GET', '/dashboard/reports/behavior', 1
  UNION ALL SELECT 'dashboard.data.behavior', 'GET', '/dashboard/workEmployee/index', 1
  UNION ALL SELECT 'dashboard.data.behavior', 'GET', '/dashboard/workDepartment/pageIndex', 0
  UNION ALL SELECT 'dashboard.data.report', 'GET', '/dashboard/reports/report', 1
  UNION ALL SELECT 'dashboard.data.report', 'GET', '/dashboard/workEmployee/index', 1
  UNION ALL SELECT 'dashboard.data.report', 'GET', '/dashboard/workDepartment/pageIndex', 0
  UNION ALL SELECT 'dashboard.ai_setting.ai_knowledge_base', 'GET', '/dashboard/ai-settings/knowledge-bases', 0
  UNION ALL SELECT 'dashboard.ai_setting.ai_knowledge_base', 'POST', '/dashboard/ai-settings/knowledge-bases', 0
  UNION ALL SELECT 'dashboard.ai_setting.ai_knowledge_base', 'PUT', '/dashboard/ai-settings/knowledge-bases/{id}', 0
  UNION ALL SELECT 'dashboard.ai_setting.ai_knowledge_base', 'DELETE', '/dashboard/ai-settings/knowledge-bases/{id}', 0
  UNION ALL SELECT 'dashboard.ai_setting.agent', 'GET', '/dashboard/ai-settings/agents', 0
  UNION ALL SELECT 'dashboard.ai_setting.agent', 'POST', '/dashboard/ai-settings/agents', 0
  UNION ALL SELECT 'dashboard.ai_setting.agent', 'PUT', '/dashboard/ai-settings/agents/{id}', 0
  UNION ALL SELECT 'dashboard.ai_setting.agent', 'DELETE', '/dashboard/ai-settings/agents/{id}', 0
  UNION ALL SELECT 'dashboard.company_setting.website', 'GET', '/dashboard/corp/index', 0
  UNION ALL SELECT 'dashboard.company_setting.website', 'GET', '/dashboard/corp/show', 0
  UNION ALL SELECT 'dashboard.company_setting.website', 'POST', '/dashboard/corp/store', 0
  UNION ALL SELECT 'dashboard.company_setting.website', 'PUT', '/dashboard/corp/update', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'GET', '/dashboard/access/users', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'GET', '/dashboard/access/users/{id}', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'PUT', '/dashboard/access/users/{id}', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'GET', '/dashboard/user/index', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'GET', '/dashboard/user/show', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'GET', '/dashboard/workDepartment/selectByPhone', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'GET', '/dashboard/role/select', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'POST', '/dashboard/user/store', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'PUT', '/dashboard/user/update', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'PUT', '/dashboard/user/statusUpdate', 0
  UNION ALL SELECT 'dashboard.company_setting.staff', 'PUT', '/dashboard/user/passwordReset', 0
  UNION ALL SELECT 'dashboard.setting.role', 'GET', '/dashboard/access/roles', 0
  UNION ALL SELECT 'dashboard.setting.role', 'POST', '/dashboard/access/roles', 0
  UNION ALL SELECT 'dashboard.setting.role', 'PUT', '/dashboard/access/roles/{id}', 0
  UNION ALL SELECT 'dashboard.setting.role', 'PUT', '/dashboard/access/roles/{id}/status', 0
  UNION ALL SELECT 'dashboard.setting.role', 'DELETE', '/dashboard/access/roles/{id}', 0
  UNION ALL SELECT 'dashboard.setting.role', 'GET', '/dashboard/role/index', 0
  UNION ALL SELECT 'dashboard.setting.role', 'GET', '/dashboard/role/show', 0
  UNION ALL SELECT 'dashboard.setting.role', 'GET', '/dashboard/role/showEmployee', 0
  UNION ALL SELECT 'dashboard.setting.role', 'GET', '/dashboard/role/permissionShow', 0
  UNION ALL SELECT 'dashboard.setting.role', 'POST', '/dashboard/role/store', 0
  UNION ALL SELECT 'dashboard.setting.role', 'PUT', '/dashboard/role/update', 0
  UNION ALL SELECT 'dashboard.setting.role', 'PUT', '/dashboard/role/statusUpdate', 0
  UNION ALL SELECT 'dashboard.setting.role', 'DELETE', '/dashboard/role/destroy', 0
  UNION ALL SELECT 'dashboard.setting.role', 'POST', '/dashboard/role/permissionStore', 0
  UNION ALL SELECT 'dashboard.setting.additional', 'GET', '/dashboard/access/catalog', 0
  UNION ALL SELECT 'dashboard.setting.additional', 'GET', '/dashboard/menu/index', 0
  UNION ALL SELECT 'dashboard.setting.additional', 'GET', '/dashboard/menu/select', 0
  UNION ALL SELECT 'dashboard.setting.additional', 'GET', '/dashboard/menu/show', 0
  UNION ALL SELECT 'dashboard.setting.additional', 'GET', '/dashboard/menu/iconIndex', 0
  UNION ALL SELECT 'dashboard.setting.additional', 'POST', '/dashboard/menu/store', 0
  UNION ALL SELECT 'dashboard.setting.additional', 'PUT', '/dashboard/menu/update', 0
  UNION ALL SELECT 'dashboard.setting.additional', 'PUT', '/dashboard/menu/statusUpdate', 0
  UNION ALL SELECT 'dashboard.setting.additional', 'DELETE', '/dashboard/menu/destroy', 0
  UNION ALL SELECT 'dashboard.setting.authorization', 'GET', '/dashboard/access/audits', 0
  UNION ALL SELECT 'dashboard.setting.authorization', 'GET', '/dashboard/menu/index', 0
  UNION ALL SELECT 'dashboard.setting.authorization', 'GET', '/dashboard/menu/select', 0
  UNION ALL SELECT 'dashboard.setting.authorization', 'GET', '/dashboard/role/permissionShow', 0
  UNION ALL SELECT 'dashboard.setting.authorization', 'POST', '/dashboard/role/permissionStore', 0
) resource_seed ON resource_seed.`permission_code` = p.`code`;

INSERT INTO `mochat_go_dashboard_user_roles` (`tenant_id`, `user_id`, `role_id`, `created_at`, `updated_at`)
SELECT u.`tenant_id`, u.`id`, r.`id`, COALESCE(ur.`created_at`, NOW()), COALESCE(ur.`updated_at`, NOW())
FROM `mc_rbac_user_role` ur
INNER JOIN `mc_user` u ON u.`id` = CAST(ur.`user_id` AS UNSIGNED)
INNER JOIN `mc_rbac_role` r ON r.`id` = ur.`role_id` AND r.`tenant_id` = u.`tenant_id`
WHERE ur.`deleted_at` IS NULL
  AND u.`deleted_at` IS NULL
  AND r.`deleted_at` IS NULL
ON DUPLICATE KEY UPDATE `updated_at` = VALUES(`updated_at`);

INSERT INTO `mochat_go_dashboard_role_permissions`
  (`tenant_id`, `role_id`, `permission_id`, `data_scope`, `created_at`, `updated_at`)
SELECT r.`tenant_id`, r.`id`, p.`id`,
       CASE WHEN m.`data_permission` = 2 THEN 'tenant' ELSE 'self' END,
       COALESCE(rm.`created_at`, NOW()), COALESCE(rm.`updated_at`, NOW())
FROM `mc_rbac_role_menu` rm
INNER JOIN `mc_rbac_role` r ON r.`id` = rm.`role_id` AND r.`deleted_at` IS NULL
INNER JOIN `mc_rbac_menu` m ON m.`id` = rm.`menu_id` AND m.`deleted_at` IS NULL
INNER JOIN `mochat_go_dashboard_permission_resources` pr
  ON pr.`path_pattern` = SUBSTRING_INDEX(SUBSTRING_INDEX(m.`link_url`, '@', 1), '#', 1)
 AND pr.`status` = 1
 AND pr.`deleted_at` IS NULL
INNER JOIN `mochat_go_dashboard_permissions` p ON p.`id` = pr.`permission_id` AND p.`superadmin_only` = 0
WHERE m.`link_url` <> ''
ON DUPLICATE KEY UPDATE `data_scope` = VALUES(`data_scope`), `updated_at` = VALUES(`updated_at`);

INSERT INTO `mochat_go_dashboard_permission_audits`
  (`tenant_id`, `actor_user_id`, `action`, `target_type`, `target_id`, `before_json`, `after_json`, `request_id`, `created_at`)
SELECT r.`tenant_id`, NULL, 'migration.backfill', 'tenant', CAST(r.`tenant_id` AS CHAR), NULL,
       JSON_OBJECT('migration', '0127_dashboard_page_rbac', 'legacyRelations', COUNT(*)),
       'migration:0127', NOW()
FROM `mc_rbac_role` r
WHERE r.`deleted_at` IS NULL
GROUP BY r.`tenant_id`;
