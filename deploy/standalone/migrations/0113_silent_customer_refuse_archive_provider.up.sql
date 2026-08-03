CREATE TABLE IF NOT EXISTS `mochat_go_silent_customer_rules` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,
 `name` varchar(80) NOT NULL,`silent_days` int unsigned NOT NULL,`status` varchar(16) NOT NULL DEFAULT 'enabled',`trigger_count` bigint unsigned NOT NULL DEFAULT 0,
 `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,`updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 PRIMARY KEY(`id`),UNIQUE KEY `uk_silent_rule_name`(`tenant_id`,`corp_id`,`name`),KEY `idx_silent_rule_scope`(`tenant_id`,`corp_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS `mochat_go_silent_customer_records` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,`rule_id` bigint unsigned NOT NULL,`rule_name` varchar(80) NOT NULL,
 `customer_id` varchar(128) NOT NULL,`customer_name` varchar(80) NOT NULL DEFAULT '',`employee_id` bigint unsigned NOT NULL DEFAULT 0,`employee_name` varchar(80) NOT NULL DEFAULT '',
 `last_interaction_at` datetime NOT NULL,`silent_days` int unsigned NOT NULL,`status` varchar(24) NOT NULL DEFAULT 'pending',`assigned_employee_id` bigint unsigned NOT NULL DEFAULT 0,
 `follow_up_note` varchar(255) NOT NULL DEFAULT '',`created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,`updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 PRIMARY KEY(`id`),UNIQUE KEY `uk_silent_record_rule_customer`(`tenant_id`,`corp_id`,`rule_id`,`customer_id`),KEY `idx_silent_record_scope`(`tenant_id`,`corp_id`,`status`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS `mochat_go_silent_customer_audits` (`id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,`record_id` bigint unsigned NOT NULL,`action` varchar(24) NOT NULL,`actor_id` bigint unsigned NOT NULL DEFAULT 0,`remark` varchar(255) NOT NULL DEFAULT '',`created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(`id`),KEY `idx_silent_audit_record`(`tenant_id`,`corp_id`,`record_id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS `mochat_go_refuse_archive_records` (
 `id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,`subject_type` varchar(16) NOT NULL,`subject_id` varchar(128) NOT NULL,`subject_name` varchar(80) NOT NULL DEFAULT '',
 `employee_id` bigint unsigned NOT NULL DEFAULT 0,`employee_name` varchar(80) NOT NULL DEFAULT '',`authorization_status` varchar(16) NOT NULL,`source` varchar(32) NOT NULL DEFAULT 'archive_provider',
 `refused_at` datetime NULL,`authorized_at` datetime NULL,`last_follow_up_at` datetime NULL,`follow_up_status` varchar(24) NOT NULL DEFAULT 'unfollowed',`follow_up_note` varchar(255) NOT NULL DEFAULT '',
 `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,`updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 PRIMARY KEY(`id`),UNIQUE KEY `uk_refuse_archive_subject`(`tenant_id`,`corp_id`,`subject_type`,`subject_id`,`employee_id`),KEY `idx_refuse_archive_scope`(`tenant_id`,`corp_id`,`authorization_status`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS `mochat_go_refuse_archive_audits` (`id` bigint unsigned NOT NULL AUTO_INCREMENT,`tenant_id` bigint unsigned NOT NULL DEFAULT 0,`corp_id` bigint unsigned NOT NULL,`record_id` bigint unsigned NOT NULL,`action` varchar(24) NOT NULL,`from_status` varchar(24) NOT NULL DEFAULT '',`to_status` varchar(24) NOT NULL DEFAULT '',`actor_id` bigint unsigned NOT NULL DEFAULT 0,`remark` varchar(255) NOT NULL DEFAULT '',`created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(`id`),KEY `idx_refuse_archive_audit_record`(`tenant_id`,`corp_id`,`record_id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
