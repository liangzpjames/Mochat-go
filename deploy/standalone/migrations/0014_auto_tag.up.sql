CREATE TABLE IF NOT EXISTS `mc_auto_tag` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `type` tinyint(1) DEFAULT '1' COMMENT '类型（1：关键词打标签。2：客户入群行为打标签。3：分时段打标签）',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '规则名称',
  `employees` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '生效成员',
  `fuzzy_match_keyword` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '模糊匹配关键词',
  `exact_match_keyword` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '精准匹配关键词',
  `tag_rule` json DEFAULT NULL COMMENT '标签规则',
  `tags` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '标签组',
  `on_off` tinyint(1) DEFAULT '1' COMMENT '规则状态（1：开，2：关）',
  `mark_tag_count` int(11) DEFAULT '0' COMMENT '已打标签数',
  `tenant_id` int(11) DEFAULT NULL COMMENT '租户id',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `create_user_id` int(11) DEFAULT NULL COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_auto_tag_corp_type` (`corp_id`, `type`),
  KEY `idx_mc_auto_tag_create_user` (`create_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='自动打标签-基本信息表';

CREATE TABLE IF NOT EXISTS `mc_auto_tag_record` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `auto_tag_id` int(11) NOT NULL COMMENT '标签id(mc_auto_tag.id)',
  `contact_id` int(11) NOT NULL COMMENT '客户id(mc_work_contact.id)',
  `tag_rule_id` int(11) DEFAULT NULL COMMENT '标签规则ID',
  `wx_external_userid` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户wx_external_userid',
  `employee_id` int(11) DEFAULT NULL COMMENT '所属员工id(mc_work_employee.id)',
  `keyword` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '触发关键词',
  `contact_room_id` int(11) DEFAULT '0' COMMENT '客户群id',
  `tags` json DEFAULT NULL COMMENT '标签',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业ID(mc_corp.id)',
  `trigger_count` int(11) DEFAULT NULL COMMENT '触发次数',
  `status` tinyint(1) DEFAULT NULL COMMENT '状态（0：未打标签，1：已打标签）',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_auto_tag_record_corp_tag` (`corp_id`, `auto_tag_id`),
  KEY `idx_mc_auto_tag_record_contact` (`contact_id`),
  KEY `idx_mc_auto_tag_record_employee` (`employee_id`),
  KEY `idx_mc_auto_tag_record_room` (`contact_room_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='自动打标签-记录表';

CREATE TABLE IF NOT EXISTS `mc_work_message_id` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID（mc_corp.id）',
  `type` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '类型（1：群消息提醒）',
  `last_id` int(11) NOT NULL DEFAULT '0' COMMENT '最后一次查询最大id',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mc_work_message_id_corp_type` (`corp_id`, `type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='会话内容查询记录';

CREATE TABLE IF NOT EXISTS `mc_work_message_1` (
  `id` int(11) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL DEFAULT '0',
  `msgid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `seq` bigint(20) NOT NULL DEFAULT '0',
  `work_employee_id` int(11) NOT NULL DEFAULT '0',
  `to_user_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '0员工 1客户 2群',
  `to_user_id` int(11) NOT NULL DEFAULT '0',
  `sender_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '0当前员工 1对方',
  `action` tinyint(1) NOT NULL DEFAULT '0',
  `type` tinyint(3) NOT NULL DEFAULT '100',
  `msg_type` tinyint(3) NOT NULL DEFAULT '100',
  `content` json DEFAULT NULL,
  `content_text` text COLLATE utf8mb4_unicode_ci,
  `room_id` int(11) NOT NULL DEFAULT '0',
  `status` tinyint(1) NOT NULL DEFAULT '0',
  `msg_data_time` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_mc_work_message_1_corp_employee` (`corp_id`, `work_employee_id`),
  KEY `idx_mc_work_message_1_to_user` (`to_user_type`, `to_user_id`),
  KEY `idx_mc_work_message_1_msg_time` (`msg_data_time`),
  KEY `idx_mc_work_message_1_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='会话内容存档分表1';

CREATE TABLE IF NOT EXISTS `mc_work_message_2` LIKE `mc_work_message_1`;
CREATE TABLE IF NOT EXISTS `mc_work_message_3` LIKE `mc_work_message_1`;
CREATE TABLE IF NOT EXISTS `mc_work_message_4` LIKE `mc_work_message_1`;
CREATE TABLE IF NOT EXISTS `mc_work_message_5` LIKE `mc_work_message_1`;
CREATE TABLE IF NOT EXISTS `mc_work_message_6` LIKE `mc_work_message_1`;
CREATE TABLE IF NOT EXISTS `mc_work_message_7` LIKE `mc_work_message_1`;
CREATE TABLE IF NOT EXISTS `mc_work_message_8` LIKE `mc_work_message_1`;
CREATE TABLE IF NOT EXISTS `mc_work_message_9` LIKE `mc_work_message_1`;
CREATE TABLE IF NOT EXISTS `mc_work_message_10` LIKE `mc_work_message_1`;

ALTER TABLE `mc_corp`
  ADD COLUMN `chat_admin` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '会话存档企业负责人' AFTER `social_code`,
  ADD COLUMN `chat_admin_phone` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '会话存档负责人电话' AFTER `chat_admin`,
  ADD COLUMN `chat_admin_idcard` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '会话存档负责人身份证' AFTER `chat_admin_phone`,
  ADD COLUMN `chat_apply_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '会话存档申请进度' AFTER `chat_admin_idcard`,
  ADD COLUMN `chat_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '会话存档状态' AFTER `chat_apply_status`,
  ADD COLUMN `chat_secret` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '会话内容存档secret' AFTER `chat_status`,
  ADD COLUMN `service_contact_url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '客服联系方式图片' AFTER `chat_secret`,
  ADD COLUMN `chat_whitelist_ip` json DEFAULT NULL COMMENT '会话存档白名单IP' AFTER `service_contact_url`,
  ADD COLUMN `chat_rsa_key` json DEFAULT NULL COMMENT '会话存档RSA密钥' AFTER `chat_whitelist_ip`;

INSERT IGNORE INTO `mc_rbac_menu` (`id`, `parent_id`, `name`, `level`, `path`, `icon`, `status`, `link_type`, `is_page_menu`, `link_url`, `data_permission`, `operate_id`, `operate_name`, `sort`, `created_at`, `updated_at`, `deleted_at`) VALUES
('75', '74', '消息存档', '3', '#1#-#74#-#75#', '', '1', '1', '1', '/dashboard/workMessage/index', '2', '0', '系统', '99', '2020-12-31 19:22:08', '2021-08-09 17:01:36', NULL),
('76', '75', '聊天记录查看', '4', '#1#-#74#-#75#-#76#', '', '1', '1', '1', '/dashboard/workMessage/toUsers', '2', '0', '系统', '99', '2020-12-31 19:22:08', '2021-08-09 17:01:36', NULL),
('88', '74', '消息存档配置', '3', '#1#-#74#-#88#', '', '1', '1', '1', '/dashboard/workMessageConfig/corpShow', '2', '0', '系统', '99', '2020-12-31 19:22:08', '2021-08-09 17:01:36', NULL),
('89', '88', '查找(按钮)', '4', '#1#-#74#-#88#-#89#', '', '1', '1', '1', '/dashboard/workMessageConfig/corpShow@search', '2', '0', '系统', '99', '2020-12-31 19:22:08', '2021-08-09 17:01:36', NULL),
('90', '88', '查看(按钮)', '4', '#1#-#74#-#88#-#90#', '', '1', '1', '1', '/dashboard/workMessageConfig/corpShow@check', '2', '0', '系统', '99', '2020-12-31 19:22:08', '2021-08-09 17:01:36', NULL),
('91', '88', '列表操作', '4', '#1#-#74#-#88#-#91#', '', '1', '1', '1', '/dashboard/workMessageConfig/corpIndex#get', '2', '0', '系统', '99', '2020-12-31 19:22:08', '2021-08-09 17:01:36', NULL),
('92', '88', '查看操作', '4', '#1#-#74#-#88#-#92#', '', '1', '1', '1', '/dashboard/workMessageConfig/corpShow#get', '2', '0', '系统', '99', '2020-12-31 19:22:08', '2021-08-09 17:01:36', NULL),
('93', '88', '保存操作', '4', '#1#-#74#-#88#-#93#', '', '1', '1', '1', '/dashboard/workMessageConfig/corpStore#post', '2', '0', '系统', '99', '2020-12-31 19:22:08', '2021-08-09 17:01:36', NULL),
('139', '76', '聊天内容列表操作', '5', '#1#-#74#-#75#-#76#-#139#', '', '1', '1', '2', '/dashboard/workMessage/toUsers#get', '2', '0', '系统', '99', '2020-12-31 19:22:10', '2021-08-09 17:01:36', NULL),
('140', '76', '聊天内容详情操作', '5', '#1#-#74#-#75#-#76#-#140#', '', '1', '1', '2', '/dashboard/workMessage/index#get', '2', '0', '系统', '99', '2020-12-31 19:22:10', '2021-08-09 17:01:36', NULL),
('141', '75', '聊天配置详情操作', '4', '#1#-#74#-#75#-#141#', '', '1', '1', '2', '/dashboard/workMessageConfig/stepCreate#get', '2', '0', '系统', '99', '2020-12-31 19:22:10', '2021-08-09 17:01:36', NULL),
('142', '75', '聊天配置编辑操作', '4', '#1#-#74#-#75#-#142#', '', '1', '1', '2', '/dashboard/workMessageConfig/stepUpdate#put', '2', '0', '系统', '99', '2020-12-31 19:22:11', '2021-08-09 17:01:36', NULL),
('299', '14', '关键词打标签', '3', '#1#-#14#-#299#', '', '1', '1', '1', '/dashboard/autoTag/keywordIndex', '2', '0', '系统', '99', '2021-06-10 17:56:55', '2021-08-09 17:01:36', NULL),
('302', '299', '创建项目', '4', '#1#-#14#-#299#-#303#', '', '2', '1', '1', '/dashboard/autoTag/keywordCreate', '2', '0', '系统', '99', '2021-06-11 08:56:15', '2021-08-09 17:01:36', NULL),
('303', '299', '详情', '4', '#1#-#298#-#299#-#303#', '', '2', '1', '1', '/dashboard/autoTag/keywordShow', '2', '0', '系统', '99', '2021-06-11 10:25:38', '2021-08-09 17:01:36', NULL),
('304', '14', '客户入群行为打标签', '3', '#1#-#14#-#304#', '', '1', '1', '1', '/dashboard/autoTag/joinRoomIndex', '2', '0', '系统', '99', '2021-06-11 14:36:59', '2021-08-09 17:01:36', NULL),
('305', '304', '添加规则', '4', '#1#-#14#-#304#-#305#', '', '2', '1', '1', '/dashboard/autoTag/joinRoomCreate', '2', '0', '系统', '99', '2021-06-11 14:37:27', '2021-08-09 17:01:36', NULL),
('306', '304', '规则详情', '4', '#1#-#14#-#304#-#306#', '', '2', '1', '1', '/dashboard/autoTag/joinRoomShow', '2', '0', '系统', '99', '2021-06-11 15:47:58', '2021-08-09 17:01:36', NULL),
('307', '14', '分时段打标签', '3', '#1#-#14#-#307#', '', '1', '1', '1', '/dashboard/autoTag/dayPartIndex', '2', '0', '系统', '99', '2021-06-11 16:25:39', '2021-08-09 17:01:36', NULL),
('308', '307', '分时段打标签添加规则', '4', '#1#-#14#-#307#-#308#', '', '2', '1', '1', '/dashboard/autoTag/dayPartCreate', '2', '0', '系统', '99', '2021-06-11 16:50:12', '2021-08-09 17:01:36', NULL),
('309', '307', '分时段打标签详情', '4', '#1#-#14#-#307#-#309#', '', '2', '1', '1', '/dashboard/autoTag/dayPartShow', '2', '0', '系统', '99', '2021-06-11 17:20:35', '2021-08-09 17:01:36', NULL),
('337', '14', '自动打标签', '3', '#1#-#14#-#337#', '', '1', '1', '1', '/dashboard/autoTag/ruleTagging', '2', '0', '系统', '99', '2021-07-17 15:40:48', '2021-08-09 17:01:36', NULL),
('451', '337', '规则列表接口', '4', '#1#-#14#-#337#-#451#', '', '1', '1', '2', '/dashboard/autoTag/index#get', '1', '0', '系统', '99', '2021-09-06 18:54:33', '2021-09-06 18:54:33', NULL),
('452', '337', '规则删除接口', '4', '#1#-#14#-#337#-#452#', '', '1', '1', '2', '/dashboard/autoTag/destroy#delete', '1', '0', '系统', '99', '2021-09-06 18:54:33', '2021-09-06 18:54:33', NULL),
('453', '337', '规则启用禁用接口', '4', '#1#-#14#-#337#-#453#', '', '1', '1', '2', '/dashboard/autoTag/onOff#put', '1', '0', '系统', '99', '2021-09-06 18:54:33', '2021-09-06 18:54:33', NULL),
('454', '337', '详情接口', '4', '#1#-#14#-#337#-#454#', '', '1', '1', '2', '/dashboard/autoTag/show#get', '1', '0', '系统', '99', '2021-09-06 18:54:33', '2021-09-06 18:54:33', NULL),
('455', '337', '关键词打标签记录接口', '4', '#1#-#14#-#337#-#455#', '', '1', '1', '2', '/dashboard/autoTag/showContactKeyWord#get', '1', '0', '系统', '99', '2021-09-06 18:54:33', '2021-09-06 18:54:33', NULL),
('456', '337', '入群打标签记录接口', '4', '#1#-#14#-#337#-#456#', '', '1', '1', '2', '/dashboard/autoTag/showContactRoom#get', '1', '0', '系统', '99', '2021-09-06 18:54:33', '2021-09-06 18:54:33', NULL),
('457', '337', '分时段打标签记录接口', '4', '#1#-#14#-#337#-#457#', '', '1', '1', '2', '/dashboard/autoTag/showContactTime#get', '1', '0', '系统', '99', '2021-09-06 18:54:33', '2021-09-06 18:54:33', NULL),
('458', '337', '新建规则接口', '4', '#1#-#14#-#337#-#458#', '', '1', '1', '2', '/dashboard/autoTag/store#post', '1', '0', '系统', '99', '2021-09-06 18:54:33', '2021-09-06 18:54:33', NULL),
('493', '337', '关键词打标签任务接口', '4', '#1#-#14#-#337#-#493#', '', '1', '1', '2', '/Task/AutoTag/KeyWordTag#get', '1', '0', '系统', '99', '2021-09-06 18:54:33', '2021-09-06 18:54:33', NULL);
