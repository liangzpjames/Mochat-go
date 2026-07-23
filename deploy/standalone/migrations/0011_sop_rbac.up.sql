CREATE TABLE IF NOT EXISTS `mc_contact_sop` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `creator_id` int(11) DEFAULT NULL COMMENT '创建人id',
  `name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '规则名称',
  `setting` text COLLATE utf8mb4_unicode_ci COMMENT '推送内容（json）',
  `employee_ids` text COLLATE utf8mb4_unicode_ci COMMENT '客服成员id（json）',
  `state` tinyint(1) DEFAULT NULL COMMENT '开关：0关 1开',
  `contact_ids` text COLLATE utf8mb4_unicode_ci COMMENT '触发的客户id（json）',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='个人SOP记录表';

CREATE TABLE IF NOT EXISTS `mc_contact_sop_log` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) DEFAULT NULL COMMENT 'work_corp.id',
  `contact_sop_id` int(11) DEFAULT NULL COMMENT 'work_sop_personal.id',
  `employee` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '员工wxid',
  `contact` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户wxid',
  `task` text COLLATE utf8mb4_unicode_ci COMMENT '触发的规则json',
  `created_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='个人SOP触达记录表';

CREATE TABLE IF NOT EXISTS `mc_room_sop` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `creator_id` int(11) DEFAULT NULL COMMENT '创建人id',
  `name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '规则名称',
  `setting` text COLLATE utf8mb4_unicode_ci COMMENT '推送内容（json）',
  `room_ids` text COLLATE utf8mb4_unicode_ci COMMENT '群聊id（json）',
  `state` tinyint(1) DEFAULT NULL COMMENT '开关：0关 1开',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群SOP记录表';

CREATE TABLE IF NOT EXISTS `mc_room_sop_log` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) DEFAULT NULL COMMENT 'work_corp.id',
  `room_sop_id` int(11) DEFAULT NULL COMMENT 'work_sop_room.id',
  `room_id` int(11) DEFAULT NULL COMMENT 'work_room.id',
  `state` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否已完成：0否，1是',
  `employee` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客服wxid',
  `contact` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户wxid',
  `task` text COLLATE utf8mb4_unicode_ci COMMENT '触发的规则json',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='群SOP触达记录表';

INSERT IGNORE INTO `mc_rbac_menu` (`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`) VALUES
('459', '260', '个人SOP列表接口', '4', '#1#-#14#-#260#-#459#', '', '1', '1', '2', '/dashboard/contactSop/index#get', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('460', '260', '个人SOP删除接口', '4', '#1#-#14#-#260#-#460#', '', '1', '1', '2', '/dashboard/contactSop/delete#delete', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('461', '260', '个人SOP删除接口', '4', '#1#-#14#-#260#-#461#', '', '1', '1', '2', '/dashboard/contactSop/destroy#delete', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('462', '260', '个人SOP详情接口', '4', '#1#-#14#-#260#-#462#', '', '1', '1', '2', '/dashboard/contactSop/detail#get', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('463', '260', '个人SOP编辑接口', '4', '#1#-#14#-#260#-#463#', '', '1', '1', '2', '/dashboard/contactSop/edit#put', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('464', '260', '个人SOP信息接口', '4', '#1#-#14#-#260#-#464#', '', '1', '1', '2', '/dashboard/contactSop/info#get', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('465', '260', '个人SOP状态接口', '4', '#1#-#14#-#260#-#465#', '', '1', '1', '2', '/dashboard/contactSop/logState#put', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('466', '260', '个人SOP设置员工接口', '4', '#1#-#14#-#260#-#466#', '', '1', '1', '2', '/dashboard/contactSop/setEmployee#put', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('467', '260', '个人SOP修改状态接口', '4', '#1#-#14#-#260#-#467#', '', '1', '1', '2', '/dashboard/contactSop/state#put', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('468', '260', '个人SOP新建接口', '4', '#1#-#14#-#260#-#468#', '', '1', '1', '2', '/dashboard/contactSop/store#post', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('469', '260', '个人SOP修改接口', '4', '#1#-#14#-#260#-#469#', '', '1', '1', '2', '/dashboard/contactSop/update#put', '1', '0', '系统', '99', '2021-09-06 19:00:54', '2021-09-06 19:00:54', NULL),
('532', '264', '群sop列表接口', '4', '#1#-#54#-#264#-#532#', '', '1', '1', '2', '/dashboard/roomSop/index#get', '1', '0', '系统', '99', '2021-09-07 00:44:29', '2021-09-07 00:44:29', NULL),
('533', '264', '群sop删除接口', '4', '#1#-#54#-#264#-#533#', '', '1', '1', '2', '/dashboard/roomSop/destroy#delete', '1', '0', '系统', '99', '2021-09-07 00:44:29', '2021-09-07 00:44:29', NULL),
('534', '264', '群sop详情接口', '4', '#1#-#54#-#264#-#534#', '', '1', '1', '2', '/dashboard/roomSop/info#get', '1', '0', '系统', '99', '2021-09-07 00:44:29', '2021-09-07 00:44:29', NULL),
('535', '264', '群sop设置群聊接口', '4', '#1#-#54#-#264#-#535#', '', '1', '1', '2', '/dashboard/roomSop/setRoom#put', '1', '0', '系统', '99', '2021-09-07 00:44:29', '2021-09-07 00:44:29', NULL),
('536', '264', '群sop状态接口', '4', '#1#-#54#-#264#-#536#', '', '1', '1', '2', '/dashboard/roomSop/state#put', '1', '0', '系统', '99', '2021-09-07 00:44:29', '2021-09-07 00:44:29', NULL),
('537', '264', '群sop新建接口', '4', '#1#-#54#-#264#-#537#', '', '1', '1', '2', '/dashboard/roomSop/store#post', '1', '0', '系统', '99', '2021-09-07 00:44:29', '2021-09-07 00:44:29', NULL),
('538', '264', '群sop更新接口', '4', '#1#-#54#-#264#-#538#', '', '1', '1', '2', '/dashboard/roomSop/update#put', '1', '0', '系统', '99', '2021-09-07 00:44:29', '2021-09-07 00:44:29', NULL);
