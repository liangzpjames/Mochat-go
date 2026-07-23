CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_approval_policies` (
  `action_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '受保护动作',
  `enabled` tinyint(1) unsigned NOT NULL DEFAULT '1' COMMENT '是否要求审批',
  `amount_threshold_cents` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '金额达到该值时要求审批，仅退款动作使用',
  `required_approvals` tinyint(3) unsigned NOT NULL DEFAULT '1' COMMENT '所需不同审批人数',
  `sla_minutes` int(10) unsigned NOT NULL DEFAULT '240' COMMENT '审批 SLA 分钟数',
  `reminder_minutes` int(10) unsigned NOT NULL DEFAULT '60' COMMENT '提醒间隔分钟数',
  `expiry_hours` smallint(5) unsigned NOT NULL DEFAULT '24' COMMENT '审批有效期小时数',
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`action_type`),
  KEY `idx_mochat_go_saas_admin_approval_policy_enabled` (`enabled`, `action_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 总后台审批策略';

INSERT IGNORE INTO `mochat_go_saas_admin_approval_policies`
  (`action_type`, `enabled`, `amount_threshold_cents`, `required_approvals`, `sla_minutes`, `reminder_minutes`, `expiry_hours`, `version`, `updated_by`, `created_at`, `updated_at`)
VALUES
  ('tenant.disable', 1, 0, 1, 240, 60, 24, 1, 0, NOW(), NOW()),
  ('payment.refund.create', 1, 0, 1, 120, 30, 24, 1, 0, NOW(), NOW()),
  ('payment.settlement.close', 1, 0, 2, 240, 60, 24, 1, 0, NOW(), NOW()),
  ('access.role.save', 1, 0, 2, 120, 30, 12, 1, 0, NOW(), NOW()),
  ('access.assignment.save', 1, 0, 2, 120, 30, 12, 1, 0, NOW(), NOW());

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_approval_decisions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `approval_id` bigint(20) unsigned NOT NULL,
  `reviewer_user_id` int(10) unsigned NOT NULL,
  `reviewer_tenant_id` int(10) unsigned NOT NULL,
  `delegated_from_user_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '通过委托代为审批的委托人',
  `decision` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'approve 或 reject',
  `reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uni_mochat_go_saas_admin_approval_decision_reviewer` (`approval_id`, `reviewer_user_id`),
  KEY `idx_mochat_go_saas_admin_approval_decision_approval` (`approval_id`, `id`),
  KEY `idx_mochat_go_saas_admin_approval_decision_delegate` (`delegated_from_user_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 总后台审批决定';

CREATE TABLE IF NOT EXISTS `mochat_go_saas_admin_approval_delegations` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `delegator_user_id` int(10) unsigned NOT NULL COMMENT '委托人',
  `delegate_user_id` int(10) unsigned NOT NULL COMMENT '受托人',
  `starts_at` timestamp NOT NULL,
  `ends_at` timestamp NOT NULL,
  `status` tinyint(3) unsigned NOT NULL DEFAULT '1' COMMENT '1 启用，2 停用',
  `reason` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  `version` int(10) unsigned NOT NULL DEFAULT '1',
  `created_by` int(10) unsigned NOT NULL DEFAULT '0',
  `updated_by` int(10) unsigned NOT NULL DEFAULT '0',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mochat_go_saas_admin_approval_delegator_time` (`delegator_user_id`, `status`, `starts_at`, `ends_at`),
  KEY `idx_mochat_go_saas_admin_approval_delegate_time` (`delegate_user_id`, `status`, `starts_at`, `ends_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版 SaaS 总后台审批委托';

ALTER TABLE `mochat_go_saas_admin_approvals`
  ADD COLUMN `policy_version` int(10) unsigned NOT NULL DEFAULT '1' AFTER `required_permission`,
  ADD COLUMN `required_approvals` tinyint(3) unsigned NOT NULL DEFAULT '1' AFTER `policy_version`,
  ADD COLUMN `approval_count` tinyint(3) unsigned NOT NULL DEFAULT '0' AFTER `required_approvals`,
  ADD COLUMN `reminder_minutes` int(10) unsigned NOT NULL DEFAULT '60' AFTER `approval_count`,
  ADD COLUMN `sla_due_at` timestamp NULL DEFAULT NULL AFTER `decision_reason`,
  ADD COLUMN `next_reminder_at` timestamp NULL DEFAULT NULL AFTER `sla_due_at`,
  ADD COLUMN `last_reminded_at` timestamp NULL DEFAULT NULL AFTER `next_reminder_at`,
  ADD COLUMN `reminder_count` int(10) unsigned NOT NULL DEFAULT '0' AFTER `last_reminded_at`,
  ADD KEY `idx_mochat_go_saas_admin_approval_reminder` (`status`, `next_reminder_at`),
  ADD KEY `idx_mochat_go_saas_admin_approval_sla` (`status`, `sla_due_at`);

UPDATE `mochat_go_saas_admin_approvals`
SET `policy_version` = 1,
    `required_approvals` = 1,
    `approval_count` = IF(`status` IN ('approved', 'executing', 'executed'), 1, 0),
    `reminder_minutes` = 60,
    `sla_due_at` = DATE_ADD(COALESCE(`created_at`, NOW()), INTERVAL 240 MINUTE),
    `next_reminder_at` = IF(`status` IN ('pending', 'approved'), DATE_ADD(COALESCE(`created_at`, NOW()), INTERVAL 60 MINUTE), NULL)
WHERE `policy_version` = 1;

INSERT IGNORE INTO `mochat_go_saas_admin_role_permissions` (`role_id`, `permission_code`, `created_at`)
SELECT r.id, 'platform.approvals.manage', NOW()
FROM `mochat_go_saas_admin_roles` r
WHERE r.code = 'platform_approver';
