CREATE TABLE IF NOT EXISTS `mc_sensitive_word_group` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业 ID',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '敏感词分组名称',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mc_sensitive_word_group_corp` (`corp_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='敏感词分组表';

CREATE TABLE IF NOT EXISTS `mc_sensitive_word` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业 ID',
  `group_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '敏感词分组 ID',
  `name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '敏感词',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '状态：1 开启，2 关闭',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mc_sensitive_word_corp_group` (`corp_id`, `group_id`, `deleted_at`),
  KEY `idx_mc_sensitive_word_corp_status` (`corp_id`, `status`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='敏感词词库表';

CREATE TABLE IF NOT EXISTS `mc_sensitive_words_monitor` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '企业 ID',
  `sensitive_word_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '敏感词 ID',
  `sensitive_word_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '触发敏感词',
  `source` tinyint(4) NOT NULL DEFAULT '1' COMMENT '触发来源：1 客户，2 员工',
  `trigger_user_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '触发人 ID，员工时为 mc_work_employee.id',
  `trigger_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '触发人名称',
  `work_room_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群 ID',
  `trigger_scenario` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '触发场景',
  `sender` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '发送人名称',
  `msg_type` int(10) unsigned NOT NULL DEFAULT '1' COMMENT '消息类型',
  `send_time` datetime DEFAULT NULL COMMENT '消息发送时间',
  `content` longtext COLLATE utf8mb4_unicode_ci COMMENT '消息内容 JSON 或文本',
  `conversation_json` longtext COLLATE utf8mb4_unicode_ci COMMENT '对话详情 JSON',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mc_sensitive_words_monitor_corp_time` (`corp_id`, `send_time`, `deleted_at`),
  KEY `idx_mc_sensitive_words_monitor_word` (`corp_id`, `sensitive_word_id`, `deleted_at`),
  KEY `idx_mc_sensitive_words_monitor_employee` (`corp_id`, `trigger_user_id`, `deleted_at`),
  KEY `idx_mc_sensitive_words_monitor_room` (`corp_id`, `work_room_id`, `deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='敏感词触发监控表';
