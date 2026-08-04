CREATE TABLE `mc_phase34_acquisition_links` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `name` VARCHAR(100) NOT NULL,
  `target_url` VARCHAR(2048) NOT NULL,
  `authorization_status` VARCHAR(32) NOT NULL DEFAULT 'unauthorized',
  `status` VARCHAR(32) NOT NULL DEFAULT 'draft',
  `visit_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `conversion_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `disabled_at` DATETIME NULL,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  KEY `idx_phase34_acquisition_links_corp_name` (`corp_id`, `name`, `deleted_at`),
  KEY `idx_phase34_acquisition_links_corp_status` (`corp_id`, `status`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `mc_phase34_customer_services` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `name` VARCHAR(100) NOT NULL,
  `account` VARCHAR(128) NOT NULL,
  `employee_ids` TEXT NOT NULL,
  `receive_mode` VARCHAR(32) NOT NULL DEFAULT 'round_robin',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending_sync',
  `sync_reason` VARCHAR(500) NOT NULL DEFAULT '',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `disabled_at` DATETIME NULL,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  KEY `idx_phase34_customer_services_corp_name` (`corp_id`, `name`, `deleted_at`),
  KEY `idx_phase34_customer_services_corp_status` (`corp_id`, `status`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `mc_phase34_short_links` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `name` VARCHAR(100) NOT NULL,
  `token` VARCHAR(64) NOT NULL,
  `target_type` VARCHAR(32) NOT NULL DEFAULT 'url',
  `target_url` VARCHAR(2048) NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'active',
  `visit_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `disabled_at` DATETIME NULL,
  `deleted_at` DATETIME NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_phase34_short_links_token` (`token`),
  KEY `idx_phase34_short_links_corp_name` (`corp_id`, `name`, `deleted_at`),
  KEY `idx_phase34_short_links_corp_status` (`corp_id`, `status`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `mc_phase34_short_link_visits` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `short_link_id` BIGINT UNSIGNED NOT NULL,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `token` VARCHAR(64) NOT NULL,
  `referer` VARCHAR(2048) NOT NULL DEFAULT '',
  `user_agent` VARCHAR(1000) NOT NULL DEFAULT '',
  `visited_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_phase34_short_link_visits_link` (`short_link_id`, `visited_at`),
  KEY `idx_phase34_short_link_visits_corp` (`corp_id`, `visited_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO `mc_rbac_menu`
(`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`)
VALUES
(116001, 20, '获客链接查询', 4, '#1#-#14#-#20#-#116001#', '', 1, 1, 2, '/dashboard/acquisitionLink/index#get', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(116002, 20, '获客链接草稿', 4, '#1#-#14#-#20#-#116002#', '', 1, 1, 2, '/dashboard/acquisitionLink/store#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(116003, 20, '获客链接授权', 4, '#1#-#14#-#20#-#116003#', '', 1, 1, 2, '/dashboard/acquisitionLink/authorize#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(116004, 20, '微信客服查询', 4, '#1#-#14#-#20#-#116004#', '', 1, 1, 2, '/dashboard/customerService/index#get', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(116005, 20, '微信客服配置', 4, '#1#-#14#-#20#-#116005#', '', 1, 1, 2, '/dashboard/customerService/store#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(116006, 20, '微信客服同步', 4, '#1#-#14#-#20#-#116006#', '', 1, 1, 2, '/dashboard/customerService/sync#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(116007, 20, '活码短链查询', 4, '#1#-#14#-#20#-#116007#', '', 1, 1, 2, '/dashboard/liveCodeShortChain/index#get', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(116008, 20, '活码短链创建', 4, '#1#-#14#-#20#-#116008#', '', 1, 1, 2, '/dashboard/liveCodeShortChain/store#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(116009, 20, '活码短链停用', 4, '#1#-#14#-#20#-#116009#', '', 1, 1, 2, '/dashboard/liveCodeShortChain/disable#post', 2, 0, '系统', 99, NOW(), NOW(), NULL);

INSERT IGNORE INTO `mc_rbac_role_menu` (`role_id`, `menu_id`, `created_at`, `updated_at`)
SELECT DISTINCT parent_access.`role_id`, phase34_action.`id`, NOW(), NOW()
FROM `mc_rbac_role_menu` AS parent_access
JOIN `mc_rbac_menu` AS phase34_action ON phase34_action.`id` BETWEEN 116001 AND 116009
WHERE parent_access.`menu_id` = 20;
