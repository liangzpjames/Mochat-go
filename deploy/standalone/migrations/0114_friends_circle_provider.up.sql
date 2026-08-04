CREATE TABLE `mc_friends_circle_tasks` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `task_name` VARCHAR(100) NOT NULL,
  `send_way` VARCHAR(20) NOT NULL,
  `content` TEXT NOT NULL,
  `target_employees` TEXT NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'draft',
  `completed_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `target_total` INT UNSIGNED NOT NULL DEFAULT 0,
  `external_task_id` VARCHAR(128) NOT NULL DEFAULT '',
  `failure_reason` VARCHAR(500) NOT NULL DEFAULT '',
  `start_at` DATETIME NULL,
  `end_at` DATETIME NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_friends_circle_tasks_corp_created` (`corp_id`, `created_at`),
  KEY `idx_friends_circle_tasks_corp_status` (`corp_id`, `status`),
  KEY `idx_friends_circle_tasks_corp_external` (`corp_id`, `external_task_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE `mc_friends_circle_materials` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `creator_name` VARCHAR(100) NOT NULL DEFAULT '',
  `name` VARCHAR(100) NOT NULL,
  `type` VARCHAR(20) NOT NULL,
  `content` TEXT NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'available',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_friends_circle_materials_corp_created` (`corp_id`, `created_at`),
  KEY `idx_friends_circle_materials_corp_type` (`corp_id`, `type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO `mc_rbac_menu` (`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`) VALUES
(114001, 0, '朋友圈任务查询', 4, '#114001#', '', 1, 1, 2, '/dashboard/friendsCircle/taskIndex#get', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(114002, 0, '朋友圈素材查询', 4, '#114002#', '', 1, 1, 2, '/dashboard/friendsCircle/materialIndex#get', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(114003, 0, '朋友圈任务草稿', 4, '#114003#', '', 1, 1, 2, '/dashboard/friendsCircle/taskStore#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(114004, 0, '朋友圈素材新增', 4, '#114004#', '', 1, 1, 2, '/dashboard/friendsCircle/materialStore#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(114005, 0, '朋友圈发布', 4, '#114005#', '', 1, 1, 2, '/dashboard/friendsCircle/publish#post', 2, 0, '系统', 99, NOW(), NOW(), NULL);
