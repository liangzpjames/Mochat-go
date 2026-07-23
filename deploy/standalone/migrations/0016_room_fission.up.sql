CREATE TABLE IF NOT EXISTS `mc_room_fission` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `official_account_id` int(11) DEFAULT '0' COMMENT '公众号id',
  `active_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '活动名称',
  `end_time` timestamp NULL DEFAULT NULL COMMENT '活动结束时间',
  `target_count` int(11) DEFAULT '0' COMMENT '活动目标人数',
  `new_friend` tinyint(1) DEFAULT '0' COMMENT '必须新好友才能助力（0：否，1：是）',
  `delete_invalid` tinyint(1) DEFAULT '0' COMMENT '好友退出全部群聊后助力失效（0：否，1：是）',
  `receive_employees` json DEFAULT NULL COMMENT '领奖客服成员',
  `auto_pass` tinyint(1) DEFAULT NULL COMMENT '自动通过好友申请',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '状态（1：进行中，2：已完成）',
  `tenant_id` int(11) DEFAULT NULL COMMENT '租户id',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `create_user_id` int(11) NOT NULL DEFAULT '0' COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_fission_corp_deleted` (`corp_id`, `deleted_at`),
  KEY `idx_mc_room_fission_tenant` (`tenant_id`),
  KEY `idx_mc_room_fission_create_user` (`create_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群裂变-基础信息主表';

CREATE TABLE IF NOT EXISTS `mc_room_fission_contact` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `union_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户微信id',
  `nickname` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户微信昵称',
  `avatar` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户微信头像',
  `parent_union_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT '0' COMMENT '上级（被谁邀请来的）',
  `level` tinyint(1) DEFAULT '0' COMMENT '裂变等级',
  `contact_id` int(11) NOT NULL DEFAULT '0' COMMENT '客户ID',
  `employee` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '添加的员工',
  `invite_count` int(11) NOT NULL DEFAULT '0' COMMENT '邀请数量',
  `loss` tinyint(1) DEFAULT '0' COMMENT '是否已流失（被删除好友）（0：否，1：是）',
  `status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '完成状态。（0：未完成，1：已完成）',
  `receive_status` tinyint(1) DEFAULT '0' COMMENT '领取状态（0：未领取，1：已领取）',
  `is_new` tinyint(1) NOT NULL DEFAULT '0' COMMENT '新客户（0：老，1：新）',
  `external_user_id` varchar(55) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '外部联系人external_userid',
  `room_id` int(255) DEFAULT NULL COMMENT '群聊ID',
  `join_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '入群状态（0：未入群，1：已入群）',
  `write_off` tinyint(1) NOT NULL DEFAULT '0' COMMENT '核销（0：未核销，1：已核销）',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_fission_contact_fission` (`fission_id`, `deleted_at`),
  KEY `idx_mc_room_fission_contact_contact` (`contact_id`),
  KEY `idx_mc_room_fission_contact_room` (`room_id`),
  KEY `idx_mc_room_fission_contact_status` (`status`, `write_off`, `join_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群裂变-客户参与';

CREATE TABLE IF NOT EXISTS `mc_room_fission_invite` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `type` tinyint(1) NOT NULL DEFAULT '2' COMMENT '类型（1：邀请，2：暂不邀请）',
  `employees` json DEFAULT NULL COMMENT '所属员工',
  `choose_contact` json DEFAULT NULL COMMENT '筛选客户条件',
  `text` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '邀请文案',
  `link_title` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '邀请链接标题',
  `link_desc` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '邀请链接描述',
  `link_pic` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '邀请链接封面图',
  `wx_link_pic` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '微信图片地址',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_fission_invite_fission` (`fission_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群裂变-邀请客户参与';

CREATE TABLE IF NOT EXISTS `mc_room_fission_poster` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `cover_pic` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '海报背景图片',
  `avatar_show` tinyint(1) DEFAULT NULL COMMENT '头像是否显示。0：不显示，1：显示',
  `nickname_show` tinyint(1) DEFAULT NULL COMMENT '昵称是否显示。0：不显示，1：显示',
  `nickname_color` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '颜色',
  `qrcode_w` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码宽度',
  `qrcode_h` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码高度',
  `qrcode_x` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码X值',
  `qrcode_y` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码Y值',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_fission_poster_fission` (`fission_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群裂变-海报';

CREATE TABLE IF NOT EXISTS `mc_room_fission_room` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `room_qrcode` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '群聊二维码',
  `room_wx_qrcode` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '群聊二维码微信图片地址',
  `room` json DEFAULT NULL COMMENT '群聊',
  `room_max` int(11) DEFAULT '0' COMMENT '群人数上限',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_fission_room_fission` (`fission_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群裂变-群聊';

CREATE TABLE IF NOT EXISTS `mc_room_fission_welcome` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `text` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '文字欢迎语',
  `link_title` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '链接标题',
  `link_desc` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '链接描述',
  `link_pic` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '链接封面地址',
  `link_wx_url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '微信图片地址',
  `template_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '欢迎语素材id',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_room_fission_welcome_fission` (`fission_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群裂变-欢迎语';

INSERT IGNORE INTO `mc_rbac_menu` (`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`) VALUES
('247', '330', '群裂变', '3', '#1#-#330#-#247#', '', '1', '1', '1', '/dashboard/roomFission/index', '2', '0', '系统', '99', '2021-04-21 08:48:37', '2021-08-09 17:01:36', NULL),
('248', '247', '创建', '4', '#1#-#330#-#247#-#248#', '', '1', '1', '1', '/dashboard/roomFission/create', '2', '0', '系统', '99', '2021-04-21 08:50:02', '2021-08-09 17:01:36', NULL),
('249', '247', '邀请', '4', '#1#-#330#-#247#-#249#', '', '1', '1', '1', '/dashboard/roomFission/invite', '2', '0', '系统', '99', '2021-04-21 09:03:37', '2021-08-09 17:01:36', NULL),
('250', '247', '修改', '4', '#1#-#330#-#247#-#250#', '', '1', '1', '1', '/dashboard/roomFission/update', '2', '0', '系统', '99', '2021-04-21 09:03:53', '2021-08-09 17:01:36', NULL),
('332', '247', '数据详情', '4', '#1#-#330#-#247#-#332#', '', '1', '1', '1', '/dashboard/roomFission/dataShow', '2', '0', '系统', '99', '2021-07-07 10:18:56', '2021-08-09 17:01:36', NULL),
('503', '247', '群裂变列表接口', '4', '#1#-#330#-#247#-#503#', '', '1', '1', '2', '/dashboard/roomFission/index#get', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL),
('504', '247', '群裂变删除接口', '4', '#1#-#330#-#247#-#504#', '', '1', '1', '2', '/dashboard/roomFission/destroy#delete', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL),
('505', '247', '群裂变弹窗接口', '4', '#1#-#330#-#247#-#505#', '', '1', '1', '2', '/dashboard/roomFission/info#get', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL),
('506', '247', '群裂变邀请接口', '4', '#1#-#330#-#247#-#506#', '', '1', '1', '2', '/dashboard/roomFission/invite#post', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL),
('507', '247', '群裂变详情接口', '4', '#1#-#330#-#247#-#507#', '', '1', '1', '2', '/dashboard/roomFission/show#get', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL),
('508', '247', '群裂变详情客户接口', '4', '#1#-#330#-#247#-#508#', '', '1', '1', '2', '/dashboard/roomFission/showContact#get', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL),
('509', '247', '群裂变详情群聊接口', '4', '#1#-#330#-#247#-#509#', '', '1', '1', '2', '/dashboard/roomFission/showRoom#get', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL),
('510', '247', '群裂变新建接口', '4', '#1#-#330#-#247#-#510#', '', '1', '1', '2', '/dashboard/roomFission/store#post', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL),
('511', '247', '群裂变更新接口', '4', '#1#-#330#-#247#-#511#', '', '1', '1', '2', '/dashboard/roomFission/update#put', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL),
('512', '247', '群裂变核销接口', '4', '#1#-#330#-#247#-#512#', '', '1', '1', '2', '/dashboard/roomFission/writeOff#get', '1', '0', '系统', '99', '2021-09-06 23:53:14', '2021-09-06 23:53:14', NULL);
