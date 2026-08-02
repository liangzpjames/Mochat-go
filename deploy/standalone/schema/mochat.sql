-- ----------------------------
-- Table structure for mc_business_log
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_business_log` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `business_id` int(11) NOT NULL DEFAULT '0' COMMENT '相应业务id',
  `params` json DEFAULT NULL COMMENT '参数',
  `event` smallint(6) NOT NULL DEFAULT '4' COMMENT '事件',
  `operation_id` int(11) NOT NULL DEFAULT '0' COMMENT '操作人id（mc_work_employee.id）',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='业务日志表';

-- ----------------------------
-- Table structure for mc_channel_code
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_channel_code` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '企业id',
  `group_id` int(11) NOT NULL DEFAULT '0' COMMENT '渠道码分组id（mc_channel_code_group.id）',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '活码名称',
  `qrcode_url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '二维码地址',
  `wx_config_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '二维码凭证',
  `auto_add_friend` tinyint(4) NOT NULL DEFAULT '0' COMMENT '自动添加好友（1.开启，2.关闭）',
  `tags` json NOT NULL COMMENT '客户标签',
  `type` tinyint(4) NOT NULL DEFAULT '0' COMMENT '类型（1.单人，2.多人）',
  `drainage_employee` json NOT NULL COMMENT '引流成员设置',
  `welcome_message` json NOT NULL COMMENT '欢迎语设置',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='渠道码表';

-- ----------------------------
-- Table structure for mc_channel_code_group
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_channel_code_group` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL COMMENT '企业id',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '分组名称',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='渠道码-分组表';

-- ----------------------------
-- Table structure for mc_chat_tool
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_chat_tool` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `page_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '侧边栏页面名称',
  `page_flag` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '侧边栏页面标识',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `status` tinyint(4) DEFAULT '1' COMMENT '状态 0否 1是',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='企业侧边工具栏';

-- ----------------------------
-- Table structure for mc_contact_employee_process
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_employee_process` (
  `id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID（corp.id）',
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '员工ID（mc_work_employee.id）',
  `contact_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '外部联系人ID（mc_work_contact.id）',
  `contact_process_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '跟进流程ID',
  `content` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '跟进内容',
  `file_url` json DEFAULT NULL COMMENT '附件地址',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='通讯录-客户-跟进记录(中间表) ';

-- ----------------------------
-- Table structure for mc_contact_employee_track
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_employee_track` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '通讯录ID(mc_work_employee.id)',
  `contact_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '外部联系人ID work_contact.id',
  `event` tinyint(4) NOT NULL DEFAULT '0' COMMENT '事件',
  `content` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '内容',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID corp.id',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='通讯录 - 客户 - 轨迹互动';

-- ----------------------------
-- Table structure for mc_contact_field
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_field` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '字段标识 input-name',
  `label` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '字段名称 input-label',
  `type` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '字段类型 input-type 0text 1radio 2 checkbox 3select 4file 5date 6dateTime 7number 8rate',
  `options` json DEFAULT NULL COMMENT '字段可选值 input-options',
  `order` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '排序',
  `status` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '状态 0不展示 1展示',
  `is_sys` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '是否为系统字段 0否1是',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户高级属性';

-- ----------------------------
-- Table structure for mc_contact_field_pivot
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_field_pivot` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `contact_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户表ID（work_contact.id）',
  `contact_field_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '高级属性表ID(contact_field.id）',
  `value` text COLLATE utf8mb4_unicode_ci COMMENT '高级属性值',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='(客户-高级属性-中间表)用户画像';

-- ----------------------------
-- Table structure for mc_contact_sop
-- ----------------------------
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

-- ----------------------------
-- Table structure for mc_contact_sop_log
-- ----------------------------
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

-- ----------------------------
-- Table structure for mc_room_sop
-- ----------------------------
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

-- ----------------------------
-- Table structure for mc_room_sop_log
-- ----------------------------
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

-- ----------------------------
-- Table structure for mc_contact_batch_add_allot
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_batch_add_allot` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `import_id` int(11) NOT NULL DEFAULT '0' COMMENT '客户账号表ID',
  `employee_id` int(11) NOT NULL DEFAULT '0' COMMENT '跟进员工ID',
  `type` tinyint(4) NOT NULL DEFAULT '0' COMMENT '状态 0回收 1分配',
  `operate_id` int(11) NOT NULL DEFAULT '0' COMMENT '操作人ID（如果有）',
  `created_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `employee_id_type_index` (`employee_id`,`type`) COMMENT '统计索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='批量新增客户分配记录表';

-- ----------------------------
-- Table structure for mc_contact_batch_add_config
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_batch_add_config` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '企业ID',
  `pending_status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '待处理客户提醒开关 0关 1开',
  `pending_time_out` int(11) NOT NULL DEFAULT '0' COMMENT '待处理客户提醒超时天数',
  `pending_reminder_time` time NOT NULL DEFAULT '00:00:00' COMMENT '待处理客户提醒时间',
  `pending_leader_id` int(11) NOT NULL DEFAULT '0' COMMENT '通知管理员ID',
  `undone_status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '成员未添加客户提醒开关 0关 1开',
  `undone_time_out` int(11) NOT NULL DEFAULT '0' COMMENT '成员未添加客户提醒超时天数',
  `undone_reminder_time` time NOT NULL DEFAULT '00:00:00' COMMENT '成员未添加客户提醒时间',
  `recycle_status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '回收客户开关 0关 1开',
  `recycle_time_out` int(11) NOT NULL DEFAULT '0' COMMENT '客户超过天数回收',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='批量新增客户配置表';

-- ----------------------------
-- Table structure for mc_contact_batch_add_import
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_batch_add_import` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '企业ID（冗余）',
  `record_id` int(11) NOT NULL DEFAULT '0' COMMENT '导入记录ID',
  `phone` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '客户手机号',
  `upload_at` timestamp NULL DEFAULT NULL COMMENT '导入时间',
  `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '添加状态 0待分配 1待添加 2待通过 3已添加',
  `add_at` timestamp NULL DEFAULT NULL COMMENT '添加时间',
  `employee_id` int(11) NOT NULL DEFAULT '0' COMMENT '分配员工',
  `allot_num` int(11) NOT NULL DEFAULT '0' COMMENT '分配次数',
  `remark` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '备注',
  `tags` json NOT NULL COMMENT '添加成功后标签',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `employee_id,status_index` (`employee_id`,`status`) COMMENT '统计索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='批量新增客户账号表';

-- ----------------------------
-- Table structure for mc_contact_batch_add_import_record
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_batch_add_import_record` (
  `id` int(10) NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '企业ID',
  `title` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '导入任务名称',
  `upload_at` timestamp NULL DEFAULT NULL COMMENT '上传时间',
  `allot_employee` json NOT NULL COMMENT '分配客服',
  `tags` json NOT NULL COMMENT '客户标签',
  `import_num` int(11) NOT NULL DEFAULT '0' COMMENT '导入客户数量',
  `add_num` int(11) NOT NULL DEFAULT '0' COMMENT '已添加客户数',
  `file_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '上传文件名',
  `file_url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '上传文件地址',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='批量新增客户导入记录表';

-- ----------------------------
-- Table structure for mc_contact_message_batch_send
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_message_batch_send` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID （mc_corp.id）',
  `user_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '用户ID【mc_user.id】',
  `user_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用户名称【mc_user.name】',
  `employee_ids` json NOT NULL COMMENT '员工ids',
  `filter_params` json DEFAULT NULL COMMENT '筛选客户参数',
  `filter_params_detail` json DEFAULT NULL COMMENT '筛选客户参数显示详情',
  `content` json NOT NULL COMMENT '群发消息内容',
  `send_way` tinyint(4) NOT NULL DEFAULT '1' COMMENT '发送方式（1-立即发送，2-定时发送）',
  `definite_time` timestamp NULL DEFAULT NULL COMMENT '定时发送时间',
  `send_time` timestamp NULL DEFAULT NULL COMMENT '发送时间',
  `send_employee_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '发送成员数量',
  `send_contact_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '发送客户数量',
  `send_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '已发送数量',
  `not_send_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '未发送数量',
  `received_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '已送达数量',
  `not_received_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '未送达数量',
  `receive_limit_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户接收已达上限',
  `not_friend_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '因不是好友发送失败',
  `send_status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '状态（0-未发送，1-已发送）',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户消息群发表';

-- ----------------------------
-- Table structure for mc_contact_message_batch_send_employee
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_message_batch_send_employee` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `batch_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户消息群发id （mc_contact_message_batch_send.id)',
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '员工id （mc_work_employee.id)',
  `wx_user_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信userId （mc_work_employee.wx_user_id)',
  `send_contact_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '发送客户数量',
  `err_code` varchar(10) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '0' COMMENT '返回码',
  `err_msg` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '对返回码的文本描述内容',
  `msg_id` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业群发消息的id，可用于获取群发消息发送结果',
  `send_time` timestamp NULL DEFAULT NULL COMMENT '发送时间',
  `last_sync_time` timestamp NULL DEFAULT NULL COMMENT '最后一次同步结果时间',
  `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '状态（0-未发送，1-已发送, 2-发送失败）',
  `receive_status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '接收状态(0-未接收，1-已接收，2-接收失败)',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户消息群发成员表';

-- ----------------------------
-- Table structure for mc_contact_message_batch_send_result
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_message_batch_send_result` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `batch_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户消息群发id （mc_contact_message_batch_send.id)',
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '员工id （mc_work_employee.id)',
  `contact_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户表id（work_contact.id）',
  `external_user_id` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人userid',
  `user_id` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业服务人员的userid',
  `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '发送状态 0-未发送 1-已发送 2-因客户不是好友导致发送失败 3-因客户已经收到其他群发消息导致发送失败',
  `send_time` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '发送时间，发送状态为1时返回',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户消息群发结果表';

-- ----------------------------
-- Table structure for mc_contact_process
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_contact_process` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT 'corp表id',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '名称',
  `description` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '描述',
  `order` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '排序',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户跟进状态';

-- ----------------------------
-- Table structure for mc_corp
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_corp` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业名称',
  `wx_corpid` char(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业微信ID',
  `social_code` char(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业代码(企业统一社会信用代码)',
  `employee_secret` char(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业通讯录secret',
  `event_callback` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '事件回调地址',
  `contact_secret` char(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业外部联系人secret',
  `token` char(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '回调token',
  `encoding_aes_key` char(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '回调消息加密串',
  `tenant_id` int(11) DEFAULT '0' COMMENT '租户ID',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='企业';

-- ----------------------------
-- Table structure for mc_corp_day_data
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_corp_day_data` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '企业id',
  `add_contact_num` int(11) NOT NULL DEFAULT '0' COMMENT '新增客户数',
  `add_room_num` int(11) NOT NULL DEFAULT '0' COMMENT '新增社群数',
  `add_into_room_num` int(11) NOT NULL DEFAULT '0' COMMENT '新增入群数',
  `loss_contact_num` int(11) NOT NULL DEFAULT '0' COMMENT '流失客户数',
  `quit_room_num` int(11) NOT NULL DEFAULT '0' COMMENT '退群数',
  `date` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '日期',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='企业日数据';

-- ----------------------------
-- Table structure for mc_greeting
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_greeting` (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键',
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '企业ID',
  `type` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '欢迎语类型',
  `words` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '欢迎语文本',
  `medium_id` int(11) NOT NULL DEFAULT '0' COMMENT '欢迎语素材',
  `range_type` tinyint(4) NOT NULL DEFAULT '1' COMMENT '适用成员类型【1-全部成员(默认)】',
  `employees` json NOT NULL COMMENT '适用成员',
  `created_at` timestamp NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='欢迎语';

-- ----------------------------
-- Table structure for mc_medium
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_medium` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `media_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '素材媒体标识[有效期3天]',
  `last_upload_time` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '上一次微信素材上传的时间戳',
  `type` tinyint(3) unsigned NOT NULL DEFAULT '1' COMMENT '类型 1文本、2图片、3音频、4视频、5小程序、6文件素材',
  `is_sync` tinyint(1) NOT NULL DEFAULT '1' COMMENT '是否同步素材库(1-同步2-不同步，默认:1)',
  `content` json NOT NULL COMMENT '具体内容:',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID(mc_corp.id)',
  `medium_group_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '素材分组ID medium_group.id',
  `user_id` int(11) NOT NULL DEFAULT '0' COMMENT '上传者ID',
  `user_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '上传者名称',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='素材库 ';

-- ----------------------------
-- Table structure for mc_medium_group
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_medium_group` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '名称',
  `order` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '排序',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='素材库-分组';

-- ----------------------------
-- Table structure for mc_official_account
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_official_account` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `app_type` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `appid` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '第三方平台 appid',
  `authorized_status` tinyint(1) DEFAULT NULL COMMENT '授权状态（1：授权成功，2：更新授权，3：取消授权）',
  `authorizer_appid` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '公众号或小程序的 appid',
  `authorization_code` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '授权码，可用于获取授权信息',
  `pre_auth_code` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '预授权码\r\n预授权码',
  `head_img` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '头像',
  `avatar` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '头像服务器地址',
  `business_info` json DEFAULT NULL COMMENT '{"open_pay": 0, "open_shake": 0, "open_scan": 0, "open_card": 0, "open_store": 0}',
  `modules` json DEFAULT NULL COMMENT '["contact_way_region", "raffle_activity", "check_in", "radar"]',
  `news_offset` tinyint(1) DEFAULT NULL COMMENT '（0：已关闭，1：开启）',
  `nickname` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '企业昵称',
  `service_type_info` tinyint(1) DEFAULT '0' COMMENT '公众号类型（0：订阅号，1由历史老帐号升级后的订阅号：，2：服务号）',
  `verify_type_info` tinyint(1) DEFAULT '0' COMMENT '服务号\r\n公众号认证类型(-1:未认证，0：微信认证，:1：新浪微博认证，2：腾讯微博认证，3：已资质认证通过但还未通过名称认证，4：已资质认证通过、还未通过名称认证，但通过了新浪微博认证',
  `original_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `func_info` json DEFAULT NULL COMMENT '[\r\n      {\r\n        "funcscope_category": {\r\n          "id": 1\r\n        }\r\n      },\r\n      {\r\n        "funcscope_category": {\r\n          "id": 2\r\n        }\r\n      }\r\n    ]',
  `principal_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `alias` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '公众号所设置的微信号，可能为空',
  `qrcode_url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码图片的 UR',
  `local_qrcode_url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码图片的 UR(服务器地址）',
  `callback_suffix` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `callback_verified` tinyint(1) DEFAULT NULL,
  `user_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '原始id',
  `encoding_aes_key` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '第三方平台消息加解密  Key',
  `notify_url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `secret` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '第三方平台appserect',
  `token` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '第三方平台token',
  `create_time` int(11) DEFAULT NULL COMMENT '授权时间',
  `tenant_id` int(11) DEFAULT '0' COMMENT '租户id',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `create_user_id` int(11) DEFAULT NULL COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='公众号授权表';

-- ----------------------------
-- Table structure for mc_official_account_set
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_official_account_set` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `official_account_id` tinyint(1) DEFAULT NULL,
  `type` tinyint(1) DEFAULT NULL COMMENT '授权模块（1：群打卡，2：抽奖活动，3：门店活码，4：互动雷达）',
  `tenant_id` int(11) DEFAULT NULL COMMENT '租户id',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `create_user_id` int(11) DEFAULT NULL COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='公众号设置表';

-- ----------------------------
-- Table structure for mc_plugin
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_plugin` (
  `id` int(11) NOT NULL,
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '企业id',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '插件名称',
  `version` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '版本号',
  `content` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '简介',
  `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '状态（1-启用，2-禁用）',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='插件表';

-- ----------------------------
-- Table structure for mc_rbac_menu
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_rbac_menu` (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键',
  `parent_id` int(11) NOT NULL COMMENT '父级ID',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '名称',
  `level` tinyint(4) NOT NULL DEFAULT '1' COMMENT '菜单等级【1-一级菜单2-二级菜单···】',
  `path` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'ID路径【id-id-id】',
  `icon` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '图标标识',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '状态【1-启动(默认)2-禁用】',
  `link_type` tinyint(4) NOT NULL DEFAULT '1' COMMENT '链接类型【1-内部链接(默认)2-外部链接】',
  `is_page_menu` tinyint(4) NOT NULL DEFAULT '1' COMMENT '是否为页面菜单 1-是 2-否',
  `link_url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '链接地址【pathinfo#method】',
  `data_permission` tinyint(1) NOT NULL DEFAULT '1' COMMENT '数据权限 【1-启用 2不启用（查看企业下数据）】',
  `operate_id` int(11) NOT NULL DEFAULT '0' COMMENT '操作人ID【mc_user.id】',
  `operate_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '操作人姓名【mc_user.name】',
  `sort` int(11) DEFAULT '99' COMMENT '排序',
  `created_at` timestamp NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='菜单表';

-- ----------------------------
-- Table structure for mc_rbac_role
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_rbac_role` (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键',
  `tenant_id` int(11) NOT NULL COMMENT '租户ID【mc_tenant.id】',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '角色名称',
  `remarks` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '角色描述',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '状态【1-启动(默认)2-禁用】',
  `operate_id` int(11) NOT NULL COMMENT '操作人ID【mc_user.id】',
  `operate_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '操作人ID【mc_user.name】',
  `data_permission` json DEFAULT NULL COMMENT '企业部门数据权限，例子[{`corpId`: `1`, `permissionType`: 1}] // 1-是(所选择企业)本用户部门 2-否 （本用户）\r\n',
  `created_at` timestamp NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色表';

-- ----------------------------
-- Table structure for mc_rbac_role_menu
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_rbac_role_menu` (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键',
  `role_id` int(11) NOT NULL DEFAULT '0' COMMENT '角色ID【mc_rbac_role.id】',
  `menu_id` int(11) NOT NULL DEFAULT '0' COMMENT '菜单ID【mc_rbac_menu.id】',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT NULL COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色-权限对应表';

-- ----------------------------
-- Table structure for mc_rbac_user_role
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_rbac_user_role` (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` int(11) NOT NULL DEFAULT '0' COMMENT '用户ID【mc_user.id】',
  `role_id` int(11) NOT NULL DEFAULT '0' COMMENT '角色ID【mc_rbac_role.id】',
  `created_at` timestamp NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户角色关联表';

-- ----------------------------
-- Table structure for mc_room_message_batch_send
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_room_message_batch_send` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID （mc_corp.id）',
  `user_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '用户ID【mc_user.id】',
  `user_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '用户名称【mc_user.name】',
  `employee_ids` json NOT NULL COMMENT '员工ids',
  `batch_title` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '群发名称',
  `content` json NOT NULL COMMENT '群发消息内容',
  `send_way` tinyint(4) NOT NULL DEFAULT '1' COMMENT '发送方式（1-立即发送，2-定时发送）',
  `definite_time` timestamp NULL DEFAULT NULL COMMENT '定时发送时间',
  `send_time` timestamp NULL DEFAULT NULL COMMENT '发送时间',
  `send_room_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '发送成员数量',
  `send_employee_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '发送客户数量',
  `send_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '已发送数量',
  `not_send_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '未发送数量',
  `received_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '已送达数量',
  `not_received_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '未送达数量',
  `send_status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '状态（0-未发送，1-已发送）',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户群消息群发表';

-- ----------------------------
-- Table structure for mc_room_message_batch_send_employee
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_room_message_batch_send_employee` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `batch_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群消息群发id （mc_contact_message_batch_send.id)',
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '员工id （mc_work_employee.id)',
  `wx_user_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信userId （mc_work_employee.wx_user_id)',
  `send_room_total` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '发送群数量',
  `err_code` varchar(10) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '0' COMMENT '返回码',
  `err_msg` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '对返回码的文本描述内容',
  `msg_id` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业群发消息的id，可用于获取群发消息发送结果',
  `send_time` timestamp NULL DEFAULT NULL COMMENT '发送时间',
  `last_sync_time` timestamp NULL DEFAULT NULL COMMENT '最后一次同步结果时间',
  `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '状态（0-未发送，1-已发送, 2-发送失败）',
  `receive_status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '接收状态(0-未接收，1-已接收，2-接收失败)',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户群消息群发成员表';

-- ----------------------------
-- Table structure for mc_room_message_batch_send_result
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_room_message_batch_send_result` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `batch_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群消息群发id （mc_contact_message_batch_send.id)',
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '员工id （mc_work_employee.id)',
  `room_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群id（work_room.id）',
  `room_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '客户群名称（work_room.name）',
  `room_employee_num` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群成员数量',
  `room_create_time` timestamp NULL DEFAULT NULL COMMENT '群聊创建时间',
  `chat_id` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部客户群id，群发消息到客户不吐出该字段',
  `user_id` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业服务人员的userid',
  `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '发送状态 0-未发送 1-已发送 2-因客户不是好友导致发送失败 3-因客户已经收到其他群发消息导致发送失败',
  `send_time` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '发送时间，发送状态为1时返回',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户群消息群发结果表';

-- ----------------------------
-- Table structure for mc_room_tag_pull
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_room_tag_pull` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '任务名称',
  `employees` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '群发员工',
  `choose_contact` json DEFAULT NULL COMMENT '筛选客户条件',
  `guide` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '入群引导语',
  `rooms` json DEFAULT NULL COMMENT '群聊',
  `filter_contact` tinyint(1) DEFAULT '1' COMMENT '过滤客户（0：否，1：是）',
  `contact_num` int(11) DEFAULT NULL COMMENT '客户数量',
  `wx_tid` json DEFAULT NULL COMMENT '企业群发消息的id',
  `tenant_id` int(11) DEFAULT NULL COMMENT '租户id',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `create_user_id` int(11) DEFAULT NULL COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='标签建群-基本信息表';

-- ----------------------------
-- Table structure for mc_room_tag_pull_contact
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_room_tag_pull_contact` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `room_tag_pull_id` int(11) NOT NULL COMMENT '标签建群id(mc_room_tag_pull.id)',
  `contact_id` int(11) NOT NULL COMMENT '客户id',
  `wx_external_userid` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户wx_external_userid',
  `contact_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户名称',
  `employee_id` int(11) NOT NULL COMMENT '员工id',
  `wx_user_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '员工wx_user_id',
  `send_status` tinyint(1) DEFAULT '0' COMMENT '发送状态：0-未发送 1-已发送 2-因客户不是好友导致发送失败 3-因客户已经收到其他群发消息导致发送失',
  `is_join_room` tinyint(1) DEFAULT '0' COMMENT '是否入群（0：否，1：是）',
  `room_id` int(11) NOT NULL COMMENT '客户群id(mc_work_room.id)',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='标签建群-客户表';

-- ----------------------------
-- Table structure for mc_room_welcome_template
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_room_welcome_template` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` varchar(11) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '0' COMMENT '企业表ID（mc_crop.id）',
  `msg_text` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '欢迎语1（文字）',
  `complex_type` varchar(50) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '欢迎语2类型',
  `msg_complex` json DEFAULT NULL COMMENT '欢迎语2（图片、链接、小程序）',
  `complex_template_id` varchar(55) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '欢迎语2素材id',
  `create_user_id` int(11) NOT NULL DEFAULT '0' COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='入群欢迎语表';

-- ----------------------------
-- Table structure for mc_sys_log
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_sys_log` (
  `id` int(11) NOT NULL AUTO_INCREMENT COMMENT '主键',
  `url_path` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '请求链接【/user/create】',
  `method` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '请求方法【get|post|put】',
  `query` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'GET参数',
  `body` json NOT NULL COMMENT 'body参数',
  `menu_id` int(11) NOT NULL COMMENT '菜单ID【mc_rbac_menu.id】',
  `menu_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '菜单名称【mc_rbac_menu.name】',
  `operate_id` int(11) NOT NULL DEFAULT '0' COMMENT '操作人ID【mc_user.id】',
  `operate_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '操作人姓名【mc_user.name】',
  `created_at` timestamp NULL DEFAULT NULL COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='系统日志';

-- ----------------------------
-- Table structure for mc_system_config
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_system_config` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '名称',
  `remark` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '备注',
  `description` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '描述',
  `tenant_id` int(11) DEFAULT NULL COMMENT '租户id',
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id',
  `creator_id` int(11) DEFAULT NULL COMMENT '创建人ID',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `uni_name` (`name`) USING BTREE COMMENT '唯一索引-名称'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='数据字典表';

-- ----------------------------
-- Table structure for mc_system_config_value
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_system_config_value` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '类别 （1-系统 2-代理 3-租户）',
  `target_id` int(11) NOT NULL DEFAULT '0' COMMENT '外部id(tenant_id/user_id/agent_id等)',
  `config_id` int(11) NOT NULL DEFAULT '0' COMMENT '配置id',
  `value` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '配置值',
  `description` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '描述',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='数据字典值表';

-- ----------------------------
-- Table structure for mc_tenant
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_tenant` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '租户名称',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '租户状态[1-正常2-停用]',
  `logo` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '租户Logo地址',
  `login_background` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '登录页背景图地址',
  `url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT '' COMMENT '网站地址',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` timestamp NULL DEFAULT NULL COMMENT '删除时间',
  `copyright` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '租户版权',
  `server_ips` json DEFAULT NULL COMMENT '服务器IPs',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='租户表';

-- ----------------------------
-- Table structure for mc_user
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_user` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `phone` char(11) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '手机号',
  `password` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '密码',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '姓名',
  `gender` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '性别 0未定义 1男 2女',
  `department` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '部门',
  `position` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '职务',
  `login_time` timestamp NULL DEFAULT NULL COMMENT '上一次登陆时间',
  `status` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '状态 0未启用 1正常 2禁用',
  `tenant_id` int(11) NOT NULL DEFAULT '1' COMMENT '租户ID(mc_tenant.id)',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `isSuperAdmin` tinyint(1) DEFAULT '0' COMMENT '是否为超级管理员 - 0否1是',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='(子账户)系统管理员';

-- ----------------------------
-- Table structure for mc_work_agent
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_agent` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL COMMENT '企业ID',
  `wx_agent_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信应用ID',
  `wx_secret` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信应用secret',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '应用名称',
  `square_logo_url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '应用方形头像',
  `description` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '应用详情',
  `close` tinyint(4) NOT NULL DEFAULT '0' COMMENT '应用是否被停用 0否1是',
  `redirect_domain` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '应用可信域名',
  `report_location_flag` tinyint(4) NOT NULL DEFAULT '0' COMMENT '应用是否打开地理位置上报 0：不上报；1：进入会话上报；',
  `is_reportenter` tinyint(4) NOT NULL DEFAULT '0' COMMENT '是否上报用户进入应用事件。0：不接收；1：接收',
  `home_url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '应用主页url',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='企业应用表';

-- ----------------------------
-- Table structure for mc_work_contact
-- ----------------------------

CREATE TABLE IF NOT EXISTS `mc_work_contact` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID（mc_crop.id）',
  `wx_external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人姓名',
  `nick_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人昵称',
  `avatar` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人的头像',
  `follow_up_status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '跟进状态（1.未跟进 2.跟进中 3.已拒绝 4.已成交 5.已复购）',
  `type` tinyint(3) unsigned NOT NULL DEFAULT '1' COMMENT '外部联系人的类型，1表示该外部联系人是微信用户，2表示该外部联系人是企业微信用户',
  `gender` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '外部联系人性别 0-未知 1-男性 2-女性',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `position` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人的职位，如果外部企业或用户选择隐藏职位，则不返回，仅当联系人类型是企业微信用户时有此字段\r\n',
  `corp_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人所在企业的简称，仅当联系人类型是企业微信用户时有此字段\r\n',
  `corp_full_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人所在企业的主体名称',
  `external_profile` json DEFAULT NULL COMMENT '外部联系人的自定义展示信息',
  `business_no` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人编号',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人表（客户列表）';

-- ----------------------------
-- Table structure for mc_work_contact_employee
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_contact_employee` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '通讯录表ID（work_employee.id）',
  `contact_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户表ID（work_contact.id）',
  `remark` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '员工对此外部联系人的备注',
  `description` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '员工对此外部联系人的描述',
  `remark_corp_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '员工对此客户备注的企业名称',
  `remark_mobiles` json DEFAULT NULL COMMENT '员工对此客户备注的手机号码',
  `add_way` int(10) unsigned NOT NULL COMMENT '表示添加客户的来源\r\n0\r\n未知来源\r\n1\r\n扫描二维码\r\n2\r\n搜索手机号\r\n3\r\n名片分享\r\n4\r\n群聊\r\n5\r\n手机通讯录\r\n6\r\n微信联系人\r\n7\r\n来自微信的添加好友申请\r\n8\r\n安装第三方应用时自动添加的客服人员\r\n9\r\n搜索邮箱\r\n201\r\n内部成员共享\r\n202\r\n管理员/负责人分配\r\n',
  `oper_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '发起添加的userid，如果成员主动添加，为成员的userid；如果是客户主动添加，则为客户的外部联系人userid；如果是内部成员共享/管理员分配，则为对应的成员/管理员userid\r\n',
  `state` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '企业自定义的state参数，用于区分客户具体是通过哪个「联系我」添加，由企业通过创建「联系我」方式指定\r\n',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID（corp.id）',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '1.正常 2.删除 3.拉黑',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '员工添加此外部联系人的时间',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mc_wce_corp_deleted_create_employee` (`corp_id`,`deleted_at`,`create_time`,`employee_id`),
  KEY `idx_mc_wce_corp_status_deleted_employee` (`corp_id`,`status`,`deleted_at`,`employee_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='通讯录 - 客户 中间表';

-- ----------------------------
-- Table structure for mc_work_contact_room
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_contact_room` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `wx_user_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `contact_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户表id（work_contact.id）',
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '员工ID (work_employee.id)',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '仅当群成员类型是微信用户（包括企业成员未添加好友），且企业或第三方服务商绑定了微信开发者ID有此字段',
  `room_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群表id（work_room.id）',
  `join_scene` tinyint(3) unsigned DEFAULT '3' COMMENT '入群方式1 - 由成员邀请入群（直接邀请入群）2 - 由成员邀请入群（通过邀请链接入群）3 - 通过扫描群二维码入群\r\n1 - 由成员邀请入群（直接邀请入群）\r\n2 - 由成员邀请入群（通过邀请链接入群）\r\n3 - 通过扫描群二维码入群',
  `type` tinyint(3) unsigned NOT NULL DEFAULT '1' COMMENT '成员类型（1 - 企业成员 2 - 外部联系人）\r\n1 - 企业成员\r\n2 - 外部联系人',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '成员状态。1 - 正常2 -退群',
  `join_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '入群时间',
  `out_time` varchar(50) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '退群时间',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mc_wcr_room_status_deleted_join` (`room_id`,`deleted_at`,`status`,`join_time`),
  KEY `idx_mc_wcr_room_status_deleted_updated` (`room_id`,`deleted_at`,`status`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户 - 客户群 关联表';

-- ----------------------------
-- Table structure for mc_work_contact_tag
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_contact_tag` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT '企业标签ID',
  `wx_contact_tag_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信企业标签ID',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID （mc_corp.id）',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '标签名称',
  `order` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '排序',
  `contact_tag_group_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户标签分组ID（mc_work_contract_tag_group.id）',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户标签';

-- ----------------------------
-- Table structure for mc_work_contact_tag_group
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_contact_tag_group` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `wx_group_id` varchar(60) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信企业标签分组ID',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID （mc_corp.id）',
  `group_name` varchar(30) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '客户标签分组名称',
  `order` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '排序',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户标签 - 分组';

-- ----------------------------
-- Table structure for mc_work_contact_tag_pivot
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_contact_tag_pivot` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `contact_id` int(10) unsigned NOT NULL COMMENT '客户表ID（work_contact.id）',
  `employee_id` int(11) NOT NULL DEFAULT '0' COMMENT '员工表id（work_employee.id）',
  `contact_tag_id` int(10) unsigned NOT NULL COMMENT '客户标签表ID（work_contact_tag.id）',
  `type` tinyint(4) NOT NULL DEFAULT '0' COMMENT '该成员添加此外部联系人所打标签类型, 1-企业设置, 2-用户自定义',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户-标签关联表';

-- ----------------------------
-- Table structure for mc_work_department
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_department` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT '部门ID',
  `wx_department_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '微信部门自增ID',
  `corp_id` int(10) unsigned NOT NULL COMMENT '企业表ID（mc_corp.id）',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '部门名称',
  `parent_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '父部门ID',
  `wx_parentid` int(10) unsigned NOT NULL COMMENT '微信父部门ID',
  `order` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '排序',
  `level` tinyint(4) NOT NULL DEFAULT '0' COMMENT '部门级别',
  `path` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT ' ' COMMENT '父ID路径【#id#-#id#】',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='(通讯录)部门管理';

-- ----------------------------
-- Table structure for mc_work_employee
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_employee` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `wx_user_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT 'wx.userId',
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '所属企业corpid（mc_corp.id）',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '名称',
  `mobile` char(11) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '手机号',
  `position` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '职位信息',
  `gender` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '性别。0表示未定义，1表示男性，2表示女性',
  `email` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '邮箱',
  `avatar` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '头像url',
  `thumb_avatar` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '头像缩略图',
  `telephone` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '座机',
  `alias` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '别名',
  `extattr` json DEFAULT NULL COMMENT '扩展属性',
  `status` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '激活状态: 1=已激活，2=已禁用，4=未激活，5=退出企业',
  `qr_code` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '员工二维码',
  `external_profile` json DEFAULT NULL COMMENT '员工对外属性',
  `external_position` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT '' COMMENT '员工对外职位',
  `address` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '地址',
  `open_user_id` char(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '全局唯一id',
  `wx_main_department_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '微信端主部门ID',
  `main_department_id` int(11) NOT NULL DEFAULT '0' COMMENT '主部门id(mc_work_department.id)',
  `log_user_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '子账户ID(mc_user.id)',
  `contact_auth` tinyint(1) NOT NULL DEFAULT '2' COMMENT '是否配置外部联系人权限（1.是 2.否）',
  `audit_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '存档状态（0：未开通，1：已开通）',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mc_we_corp_status_deleted` (`corp_id`,`status`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='企业通讯录';

-- ----------------------------
-- Table structure for mc_work_employee_department
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_employee_department` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `employee_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '通讯录员工(mc_work_department.id)',
  `department_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '通讯录部门ID (mc_work_department.id)',
  `is_leader_in_dept` tinyint(4) NOT NULL DEFAULT '0' COMMENT '所在的部门内是否为上级',
  `order` int(11) NOT NULL DEFAULT '0' COMMENT '排序',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mc_wed_employee_deleted_department` (`employee_id`,`deleted_at`,`department_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='(通讯录 - 通讯录部门)中间表';

-- ----------------------------
-- Table structure for mc_work_employee_statistic
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_employee_statistic` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) DEFAULT NULL,
  `employee_id` int(11) NOT NULL COMMENT '成员id (mc_work_employee.id)',
  `new_apply_cnt` int(11) NOT NULL COMMENT '发起申请数成员通过「搜索手机号」、「扫一扫」、「从微信好友中添加」、「从群聊中添加」、「添加共享、分配给我的客户」、「添加单向、双向删除好友关系的好友」、「从新的联系人推荐中添加」等渠道主动向客户发起的好友申请数量',
  `new_contact_cnt` int(11) NOT NULL COMMENT '新增客户数',
  `chat_cnt` int(11) NOT NULL COMMENT '聊天总数',
  `message_cnt` int(11) NOT NULL COMMENT '发送消息数',
  `reply_percentage` int(11) NOT NULL COMMENT '已回复聊天占比',
  `avg_reply_time` int(11) NOT NULL COMMENT '平均首次回复时长',
  `negative_feedback_cnt` int(11) NOT NULL COMMENT '删除/拉黑成员的客户数',
  `syn_time` timestamp NULL DEFAULT NULL COMMENT '同步时间',
  `created_at` timestamp NULL DEFAULT NULL,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='成员统计表';

-- ----------------------------
-- Table structure for mc_work_employee_tag
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_employee_tag` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `wx_tagid` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '微信通许录标签 id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID（mc_corp.id）',
  `tag_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '标签名称',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='(通讯录)标签';

-- ----------------------------
-- Table structure for mc_work_employee_tag_pivot
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_employee_tag_pivot` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `employee_id` int(10) unsigned NOT NULL COMMENT '通讯录员工ID',
  `tag_id` int(10) unsigned NOT NULL COMMENT 'wx标签ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='(通讯录 - 标签)中间表';

-- ----------------------------
-- Table structure for mc_work_fission
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_fission` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '企业表ID（mc_crop.id）',
  `active_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '活动名称',
  `service_employees` json DEFAULT NULL COMMENT '客服成员',
  `auto_pass` tinyint(1) DEFAULT NULL COMMENT '自动通过好友申请',
  `auto_add_tag` tinyint(1) DEFAULT NULL COMMENT '自动添加客户标签',
  `contact_tags` json DEFAULT NULL COMMENT '标签组',
  `end_time` timestamp NULL DEFAULT NULL COMMENT '活动结束时间',
  `qr_code_invalid` int(11) DEFAULT NULL COMMENT '二维码有效期（天）为空则是立即失效',
  `tasks` json DEFAULT NULL COMMENT '裂变任务',
  `new_friend` tinyint(1) DEFAULT NULL COMMENT '必须新好友才能助力',
  `delete_invalid` tinyint(1) DEFAULT NULL COMMENT '删除员工后助力失效',
  `receive_prize` tinyint(1) DEFAULT NULL COMMENT '领奖方式：0联系客服，1兑换链接',
  `receive_prize_employees` json DEFAULT NULL COMMENT '领奖-员工',
  `receive_links` json DEFAULT NULL COMMENT '领奖-兑换链接',
  `receive_qrcode` json DEFAULT NULL COMMENT '领奖-员工二维码',
  `create_user_id` int(11) NOT NULL DEFAULT '0' COMMENT '创建人ID',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='裂变-基础信息主表';

-- ----------------------------
-- Table structure for mc_work_fission_contact
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_fission_contact` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `union_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户微信id',
  `nickname` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户微信昵称',
  `avatar` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户微信头像',
  `contact_superior_user_parent` int(11) DEFAULT '0' COMMENT '上级（被谁邀请来的）',
  `level` tinyint(1) DEFAULT '0' COMMENT '裂变等级',
  `employee` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '添加的员工',
  `invite_count` int(11) NOT NULL DEFAULT '0' COMMENT '邀请数量',
  `loss` tinyint(1) DEFAULT '0' COMMENT '是否已流失（被删除好友）',
  `status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '完成状态。（0：未完成，1：已完成）',
  `receive_level` tinyint(1) DEFAULT '0' COMMENT '已领取奖励阶段',
  `is_new` tinyint(1) NOT NULL DEFAULT '0' COMMENT '新客户（0：老，1：新）',
  `external_user_id` varchar(55) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '外部联系人external_userid',
  `qrcode_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码ID',
  `qrcode_url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码图片链接',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='裂变-客户参与';

-- ----------------------------
-- Table structure for mc_work_fission_invite
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_fission_invite` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `type` tinyint(1) DEFAULT '2' COMMENT '类型（1：邀请，2：暂不邀请）',
  `text` text COLLATE utf8mb4_unicode_ci COMMENT '邀请文案',
  `link_title` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '邀请链接标题',
  `link_desc` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '邀请链接描述',
  `link_pic` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '邀请链接封面图',
  `wx_link_pic` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '微信图片地址',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='裂变-邀请客户参与';

-- ----------------------------
-- Table structure for mc_work_fission_poster
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_fission_poster` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `poster_type` tinyint(1) DEFAULT NULL COMMENT '裂变海报：0海报,1个人名片',
  `cover_pic` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '海报背景图片',
  `wx_cover_pic` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '背景图片微信地址',
  `foward_text` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '海报转发话术',
  `avatar_show` tinyint(1) DEFAULT NULL COMMENT '头像是否显示。0：不显示，1：显示',
  `nickname_show` tinyint(1) DEFAULT NULL COMMENT '昵称是否显示。0：不显示，1：显示',
  `nickname_color` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '颜色',
  `card_corp_image_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '个人名片企业形象名称',
  `card_corp_name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '个人名片企业名称',
  `card_corp_logo` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '个人名片企业logo',
  `qrcode_w` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码宽度',
  `qrcode_h` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码高度',
  `qrcode_x` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码X值',
  `qrcode_y` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码Y值',
  `qrcode_id` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码ID',
  `qrcode_url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '二维码图片链接',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='裂变-海报';

-- ----------------------------
-- Table structure for mc_work_fission_push
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_fission_push` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `push_employee` tinyint(4) DEFAULT NULL COMMENT '员工推送',
  `push_contact` tinyint(4) DEFAULT NULL COMMENT '客户推送',
  `msg_text` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '消息1（文字）',
  `msg_complex` json DEFAULT NULL COMMENT '消息2（图片、链接、小程序）',
  `msg_complex_type` varchar(55) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '消息2类型（image|link|applets）',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='裂变-推送';

-- ----------------------------
-- Table structure for mc_work_fission_welcome
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_fission_welcome` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `fission_id` int(11) NOT NULL DEFAULT '0' COMMENT '活动ID',
  `msg_text` text COLLATE utf8mb4_unicode_ci COMMENT '文字欢迎语',
  `link_title` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '链接标题',
  `link_desc` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '链接描述',
  `link_cover_url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '链接封面地址',
  `link_wx_url` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '微信图片地址',
  `deleted_at` timestamp NULL DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='裂变-欢迎语';

-- ----------------------------
-- Table structure for mc_work_room
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_room` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业表ID（mc_corp.id）',
  `wx_chat_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '客户群ID',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '客户群名称',
  `owner_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '群主ID（work_employee.id）',
  `notice` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '群公告',
  `status` tinyint(3) unsigned NOT NULL DEFAULT '0' COMMENT '客户群状态（0 - 正常 1 - 跟进人离职 2 - 离职继承中 3 - 离职继承完成）',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '群创建时间',
  `room_max` int(11) NOT NULL DEFAULT '0' COMMENT '群成员上限',
  `room_group_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '分组id（work_room_group.id）',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mc_wr_corp_deleted_created_owner` (`corp_id`,`deleted_at`,`created_at`,`owner_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户群表';

-- ----------------------------
-- Table structure for mc_work_room_auto_pull
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_room_auto_pull` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL COMMENT '企业表ID(mc_corp.id)',
  `qrcode_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '二维码名称',
  `qrcode_url` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '二维码地址',
  `wx_config_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '二维码凭证',
  `is_verified` tinyint(3) unsigned NOT NULL DEFAULT '2' COMMENT '添加验证 （1:需验证 2:直接通过）',
  `leading_words` text COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '入群引导语',
  `tags` json NOT NULL COMMENT '群标签 [{`tag_id`: `1`,`type`: 1,`tag_name`:`标签`,group_id:`1` ,group_name`:分组名称}]',
  `employees` json NOT NULL COMMENT '使用成员[{`id`: `1`,name`:`成员`}]',
  `rooms` json NOT NULL COMMENT '群[{`id`: `1`,`type`: 1,`name`:`成员`,room_max:''群上限''}]',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='自动拉群表';

-- ----------------------------
-- Table structure for mc_work_room_group
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_room_group` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL COMMENT '企业表ID（mc_corp.id）',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '分组名称',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户群分组管理表';

-- ----------------------------
-- Table structure for mc_work_transfer_log
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_transfer_log` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) DEFAULT NULL COMMENT '企业id (corp.id)',
  `status` tinyint(1) DEFAULT NULL COMMENT '客服类型：1离职分配 2在职分配',
  `type` tinyint(1) DEFAULT NULL COMMENT '分配类型：1客户转接 2群聊转接',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户/群聊 名称',
  `contact_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '客户/群聊 WxId',
  `handover_employee_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '原客服的WxId',
  `takeover_employee_id` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '接替的客服的WxId',
  `state` tinyint(1) DEFAULT NULL COMMENT '转接状态：1接替完毕 2等待接替 3客户拒绝 4接替成员客户达到上限 5无接替记录',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='客户/群聊 分配记录表';

-- ----------------------------
-- Table structure for mc_work_unassigned
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unassigned` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL COMMENT '企业id(corp.id)',
  `handover_userid` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '离职成员的userid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '外部联系人userid',
  `dimission_time` int(11) NOT NULL COMMENT '成员离职时间',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC COMMENT='离职成员-客户存储表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_0
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_0` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_1
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_1` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_2
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_2` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_3
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_3` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_4
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_4` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_5
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_5` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_6
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_6` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_7
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_7` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_8
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_8` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_unionid_external_userid_mapping_9
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_unionid_external_userid_mapping_9` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT COMMENT 'id',
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业id',
  `unionid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人在微信开放平台的唯一身份标识（微信unionid）',
  `openid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '微信公众号的openid',
  `external_userid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '外部联系人external_userid',
  `pending_id` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subject_type` tinyint(1) NOT NULL DEFAULT '0' COMMENT '主体类型：0表示主体是企业, 1表示主体是服务商',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `cuo-userid` (`corp_id`,`unionid`,`openid`,`external_userid`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人unionid-external_userid映射表';

-- ----------------------------
-- Table structure for mc_work_update_time
-- ----------------------------
CREATE TABLE IF NOT EXISTS `mc_work_update_time` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(11) NOT NULL DEFAULT '0' COMMENT '企业表ID（mc_crop.id）',
  `type` tinyint(4) NOT NULL DEFAULT '0' COMMENT '类型（1.通讯录，2.客户，3.标签，4.部门 5.会放内容存档 6.企业数据）',
  `last_update_time` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '最后一次同步时间',
  `error_msg` json DEFAULT NULL COMMENT '错误信息',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='同步时间表';
