CREATE TABLE IF NOT EXISTS `mc_room_calendar` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '日历名称',
  `rooms` json DEFAULT NULL COMMENT '群聊',
  `on_off` tinyint(1) DEFAULT '1' COMMENT '开关（1：开，2：关）',
  `tenant_id` int(11) DEFAULT NULL COMMENT '租户id',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `create_user_id` int(11) DEFAULT NULL COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_calendar_corp_deleted` (`corp_id`, `deleted_at`),
  KEY `idx_mc_room_calendar_tenant` (`tenant_id`),
  KEY `idx_mc_room_calendar_create_user` (`create_user_id`),
  KEY `idx_mc_room_calendar_on_off` (`on_off`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群日历-基本信息表';

CREATE TABLE IF NOT EXISTS `mc_room_calendar_push` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `room_calendar_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '群日历id',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '推送内容名称',
  `day` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '推送时间',
  `push_content` json DEFAULT NULL COMMENT '发送内容',
  `on_off` tinyint(1) DEFAULT '1' COMMENT '开关（1：开，2：关）',
  `status` tinyint(1) DEFAULT '1' COMMENT '状态（1：未推送，2：已推送）',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_calendar_push_calendar` (`room_calendar_id`, `deleted_at`),
  KEY `idx_mc_room_calendar_push_day` (`day`),
  KEY `idx_mc_room_calendar_push_status` (`status`, `on_off`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群日历-推送信息表';

CREATE TABLE IF NOT EXISTS `mc_room_calendar_record` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `room_calendar_id` int(11) NOT NULL COMMENT '群日历id',
  `push_ids` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '推送消息ids',
  `day` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '推送时间',
  `room_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '群聊id',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_calendar_record_calendar` (`room_calendar_id`, `deleted_at`),
  KEY `idx_mc_room_calendar_record_room` (`room_id`),
  KEY `idx_mc_room_calendar_record_day` (`day`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群日历-推送信息记录表';

INSERT IGNORE INTO `mc_rbac_menu` (`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`) VALUES
('596', '54', '群日历', '3', '#1#-#54#-#596#', '', '1', '1', '1', '/dashboard/roomCalendar/index', '2', '0', '系统', '3', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL),
('597', '596', '创建日历', '4', '#1#-#54#-#596#-#597#', '', '1', '1', '1', '/dashboard/roomCalendar/create', '2', '0', '系统', '99', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL),
('598', '596', '群日历详情', '4', '#1#-#54#-#596#-#598#', '', '1', '1', '1', '/dashboard/roomCalendar/show', '2', '0', '系统', '99', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL),
('599', '596', '群日历列表接口', '4', '#1#-#54#-#596#-#599#', '', '1', '1', '2', '/dashboard/roomCalendar/index#get', '1', '0', '系统', '99', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL),
('600', '596', '群日历设置群聊接口', '4', '#1#-#54#-#596#-#600#', '', '1', '1', '2', '/dashboard/roomCalendar/addRoom#post', '1', '0', '系统', '99', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL),
('601', '596', '群日历删除群聊接口', '4', '#1#-#54#-#596#-#601#', '', '1', '1', '2', '/dashboard/roomCalendar/destroyRoom#delete', '1', '0', '系统', '99', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL),
('602', '596', '群日历新建接口', '4', '#1#-#54#-#596#-#602#', '', '1', '1', '2', '/dashboard/roomCalendar/store#post', '1', '0', '系统', '99', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL),
('603', '596', '群日历删除接口', '4', '#1#-#54#-#596#-#603#', '', '1', '1', '2', '/dashboard/roomCalendar/destroy#delete', '1', '0', '系统', '99', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL),
('604', '596', '群日历详情接口', '4', '#1#-#54#-#596#-#604#', '', '1', '1', '2', '/dashboard/roomCalendar/show#get', '1', '0', '系统', '99', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL),
('605', '596', '群日历更新接口', '4', '#1#-#54#-#596#-#605#', '', '1', '1', '2', '/dashboard/roomCalendar/update#put', '1', '0', '系统', '99', '2021-09-07 01:20:00', '2021-09-07 01:20:00', NULL);
