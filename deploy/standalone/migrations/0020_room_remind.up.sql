CREATE TABLE IF NOT EXISTS `mc_room_remind` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '名称',
  `rooms` json NOT NULL COMMENT '群聊',
  `is_qrcode` tinyint(1) NOT NULL COMMENT '发送带二维码图片（0：不提醒，1：提醒）',
  `is_link` tinyint(1) NOT NULL COMMENT '发送链接分享（0：不提醒，1：提醒）',
  `is_miniprogram` tinyint(1) NOT NULL COMMENT '发送小程序（0：不提醒，1：提醒）',
  `is_card` tinyint(1) NOT NULL COMMENT '发送名片（0：不提醒，1：提醒）',
  `is_keyword` tinyint(1) NOT NULL COMMENT '发送关键词（0：不提醒，1：提醒）',
  `keyword` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '关键词',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '状态（0：关闭，1：开启）',
  `tenant_id` int(11) DEFAULT NULL COMMENT '租户id',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `create_user_id` int(11) DEFAULT NULL COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_remind_corp_deleted` (`corp_id`, `deleted_at`),
  KEY `idx_mc_room_remind_tenant` (`tenant_id`),
  KEY `idx_mc_room_remind_create_user` (`create_user_id`),
  KEY `idx_mc_room_remind_status` (`status`),
  KEY `idx_mc_room_remind_keyword` (`keyword`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户群提醒-基本信息表';

CREATE TABLE IF NOT EXISTS `mc_room_remind_record` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `remind_id` int(11) NOT NULL COMMENT '提醒id',
  `message_id` int(11) NOT NULL COMMENT '消息id',
  `room_id` int(11) NOT NULL COMMENT '群聊',
  `type` tinyint(1) NOT NULL COMMENT '类型（1：二维码，2：链接，3：小程序，4：名片，5：关键词）',
  `content` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '内容',
  `keyword` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '关键词',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_remind_record_remind` (`remind_id`, `deleted_at`),
  KEY `idx_mc_room_remind_record_room` (`room_id`),
  KEY `idx_mc_room_remind_record_corp` (`corp_id`, `deleted_at`),
  KEY `idx_mc_room_remind_record_type` (`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户群提醒-提醒记录表';

INSERT IGNORE INTO `mc_rbac_menu` (`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`) VALUES
('606', '54', '客户群提醒', '3', '#1#-#54#-#606#', '', '1', '1', '1', '/dashboard/roomRemind/index', '2', '0', '系统', '8', '2021-09-07 01:30:00', '2021-09-07 01:30:00', NULL),
('607', '606', '群提醒列表接口', '4', '#1#-#54#-#606#-#607#', '', '1', '1', '2', '/dashboard/roomRemind/index#get', '1', '0', '系统', '99', '2021-09-07 01:30:00', '2021-09-07 01:30:00', NULL),
('608', '606', '群提醒删除接口', '4', '#1#-#54#-#606#-#608#', '', '1', '1', '2', '/dashboard/roomRemind/destroy#delete', '1', '0', '系统', '99', '2021-09-07 01:30:00', '2021-09-07 01:30:00', NULL),
('609', '606', '群提醒详情接口', '4', '#1#-#54#-#606#-#609#', '', '1', '1', '2', '/dashboard/roomRemind/info#get', '1', '0', '系统', '99', '2021-09-07 01:30:00', '2021-09-07 01:30:00', NULL),
('610', '606', '群提醒状态接口', '4', '#1#-#54#-#606#-#610#', '', '1', '1', '2', '/dashboard/roomRemind/status#get', '1', '0', '系统', '99', '2021-09-07 01:30:00', '2021-09-07 01:30:00', NULL),
('611', '606', '群提醒新建接口', '4', '#1#-#54#-#606#-#611#', '', '1', '1', '2', '/dashboard/roomRemind/store#post', '1', '0', '系统', '99', '2021-09-07 01:30:00', '2021-09-07 01:30:00', NULL),
('612', '606', '群提醒更新接口', '4', '#1#-#54#-#606#-#612#', '', '1', '1', '2', '/dashboard/roomRemind/update#put', '1', '0', '系统', '99', '2021-09-07 01:30:00', '2021-09-07 01:30:00', NULL);
