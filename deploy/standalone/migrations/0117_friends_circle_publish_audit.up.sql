ALTER TABLE `mc_friends_circle_tasks`
  ADD COLUMN `publish_attempts` INT UNSIGNED NOT NULL DEFAULT 0 AFTER `external_task_id`,
  ADD COLUMN `last_callback_at` DATETIME NULL AFTER `failure_reason`;

CREATE TABLE `mc_friends_circle_task_results` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `corp_id` BIGINT UNSIGNED NOT NULL,
  `task_id` BIGINT UNSIGNED NOT NULL,
  `target_employee_id` BIGINT UNSIGNED NOT NULL,
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending',
  `failure_code` VARCHAR(64) NOT NULL DEFAULT '',
  `failure_reason` VARCHAR(500) NOT NULL DEFAULT '',
  `occurred_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_friends_circle_result_target` (`corp_id`, `task_id`, `target_employee_id`),
  KEY `idx_friends_circle_results_task_status` (`corp_id`, `task_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO `mc_rbac_menu`
(`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`)
VALUES
(117001, 20, '朋友圈任务结果明细', 4, '#1#-#14#-#20#-#117001#', '', 1, 1, 2, '/dashboard/friendsCircle/taskResultIndex#get', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(117002, 20, '朋友圈任务结果导出', 4, '#1#-#14#-#20#-#117002#', '', 1, 1, 2, '/dashboard/friendsCircle/export#get', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(117003, 0, '朋友圈 Provider 回调', 4, '#117003#', '', 1, 1, 2, '/dashboard/friendsCircle/providerCallback#post', 2, 0, '系统', 99, NOW(), NOW(), NULL);

INSERT IGNORE INTO `mc_rbac_role_menu` (`role_id`, `menu_id`, `created_at`, `updated_at`)
SELECT DISTINCT parent_access.`role_id`, result_action.`id`, NOW(), NOW()
FROM `mc_rbac_role_menu` AS parent_access
JOIN `mc_rbac_menu` AS result_action ON result_action.`id` IN (117001, 117002)
WHERE parent_access.`menu_id` = 20;
