ALTER TABLE `mc_medium`
  ADD COLUMN `scope_type` varchar(16) NOT NULL DEFAULT 'public' COMMENT '素材作用域 public department personal' AFTER `user_name`,
  ADD COLUMN `scope_id` int(10) unsigned NOT NULL DEFAULT 0 COMMENT '部门或个人用户标识' AFTER `scope_type`,
  ADD COLUMN `sidebar_visible` tinyint(1) NOT NULL DEFAULT 1 COMMENT '聊天侧边栏可见' AFTER `scope_id`,
  ADD COLUMN `status` varchar(16) NOT NULL DEFAULT 'available' COMMENT 'processing available failed disabled' AFTER `sidebar_visible`,
  ADD KEY `idx_medium_scope` (`corp_id`, `scope_type`, `scope_id`, `deleted_at`),
  ADD KEY `idx_medium_selector` (`corp_id`, `status`, `sidebar_visible`, `deleted_at`);

INSERT IGNORE INTO `mc_rbac_menu`
(`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`)
VALUES
(115001, 20, '素材批量移动', 4, '#1#-#14#-#20#-#115001#', '', 1, 1, 2, '/dashboard/medium/batchGroupUpdate#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(115002, 20, '素材引用检查', 4, '#1#-#14#-#20#-#115002#', '', 1, 1, 2, '/dashboard/medium/referenceCheck#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(115003, 20, '素材批量删除', 4, '#1#-#14#-#20#-#115003#', '', 1, 1, 2, '/dashboard/medium/batchDestroy#post', 2, 0, '系统', 99, NOW(), NOW(), NULL),
(115004, 20, '素材统一选择', 4, '#1#-#14#-#20#-#115004#', '', 1, 1, 2, '/dashboard/materialSelector/index#get', 2, 0, '系统', 99, NOW(), NOW(), NULL);

INSERT IGNORE INTO `mc_rbac_role_menu` (`role_id`, `menu_id`, `created_at`, `updated_at`)
SELECT DISTINCT parent_access.`role_id`, material_action.`id`, NOW(), NOW()
FROM `mc_rbac_role_menu` AS parent_access
JOIN `mc_rbac_menu` AS material_action ON material_action.`id` IN (115001, 115002, 115003, 115004)
WHERE parent_access.`menu_id` = 20;
