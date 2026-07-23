CREATE TABLE IF NOT EXISTS `mc_room_infinite` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '名称',
  `avatar` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '二维码头像',
  `title_status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '群名称设置（0：关闭，1：开启）',
  `title` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '群名称',
  `describe_status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '入群引导语（0：关闭，1：开启）',
  `describe` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '入群引导语',
  `logo` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '头像',
  `qw_code` json NOT NULL COMMENT '企微活码（qrcode，upper_limit，status状态（0：未开始，1：拉人中，2：已停用））',
  `total_num` int(11) NOT NULL DEFAULT '0' COMMENT '扫码人数',
  `tenant_id` int(11) DEFAULT NULL COMMENT '租户id',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `create_user_id` int(11) DEFAULT NULL COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_infinite_corp_deleted` (`corp_id`, `deleted_at`),
  KEY `idx_mc_room_infinite_tenant` (`tenant_id`),
  KEY `idx_mc_room_infinite_create_user` (`create_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='无限拉群-基本信息表';

INSERT IGNORE INTO `mc_rbac_menu` (`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`) VALUES
('613', '54', '无限拉群', '3', '#1#-#54#-#613#', '', '1', '1', '1', '/dashboard/roomInfinitePull/index', '2', '0', '系统', '7', '2021-09-07 01:40:00', '2021-09-07 01:40:00', NULL),
('614', '613', '新建拉群', '4', '#1#-#54#-#613#-#614#', '', '1', '1', '1', '/dashboard/roomInfinitePull/create', '2', '0', '系统', '99', '2021-09-07 01:40:00', '2021-09-07 01:40:00', NULL),
('615', '613', '详情', '4', '#1#-#54#-#613#-#615#', '', '1', '1', '1', '/dashboard/roomInfinitePull/show', '2', '0', '系统', '99', '2021-09-07 01:40:00', '2021-09-07 01:40:00', NULL),
('616', '613', '无限拉群列表接口', '4', '#1#-#54#-#613#-#616#', '', '1', '1', '2', '/dashboard/roomInfinitePull/index#get', '1', '0', '系统', '99', '2021-09-07 01:40:00', '2021-09-07 01:40:00', NULL),
('617', '613', '无限拉群删除接口', '4', '#1#-#54#-#613#-#617#', '', '1', '1', '2', '/dashboard/roomInfinitePull/destroy#delete', '1', '0', '系统', '99', '2021-09-07 01:40:00', '2021-09-07 01:40:00', NULL),
('618', '613', '无限拉群详情接口', '4', '#1#-#54#-#613#-#618#', '', '1', '1', '2', '/dashboard/roomInfinitePull/info#get', '1', '0', '系统', '99', '2021-09-07 01:40:00', '2021-09-07 01:40:00', NULL),
('619', '613', '无限拉群新建接口', '4', '#1#-#54#-#613#-#619#', '', '1', '1', '2', '/dashboard/roomInfinitePull/store#post', '1', '0', '系统', '99', '2021-09-07 01:40:00', '2021-09-07 01:40:00', NULL),
('620', '613', '无限拉群更新接口', '4', '#1#-#54#-#613#-#620#', '', '1', '1', '2', '/dashboard/roomInfinitePull/update#put', '1', '0', '系统', '99', '2021-09-07 01:40:00', '2021-09-07 01:40:00', NULL);
