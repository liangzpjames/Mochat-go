CREATE TABLE IF NOT EXISTS `mochat_go_wechat_component_tickets` (
  `component_appid` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '微信开放平台第三方平台 appid',
  `component_verify_ticket` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '微信开放平台 component_verify_ticket',
  `create_time` bigint(20) DEFAULT NULL COMMENT '微信推送的 CreateTime',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`component_appid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版微信开放平台 ticket 表';
