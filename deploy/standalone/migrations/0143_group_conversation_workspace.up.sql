CREATE TABLE IF NOT EXISTS `mochat_go_work_message_participant_identity` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `corp_id` int unsigned NOT NULL,
  `msgid` varchar(128) NOT NULL,
  `seq` bigint NOT NULL,
  `sender_wx_id` varchar(128) NOT NULL DEFAULT '',
  `room_wx_id` varchar(128) NOT NULL DEFAULT '',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_mg_wmpi_corp_msg_seq` (`corp_id`,`msgid`,`seq`),
  KEY `idx_mg_wmpi_corp_sender` (`corp_id`,`sender_wx_id`),
  KEY `idx_mg_wmpi_corp_room` (`corp_id`,`room_wx_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', 'GET', resource_seed.`path_pattern`, 1, 1, 1
FROM `mochat_go_dashboard_permissions` p
INNER JOIN (
  SELECT 'dashboard.chat.v2_group' AS `permission_code`, '/dashboard/workMessage/roomDirectory' AS `path_pattern`
  UNION ALL SELECT 'dashboard.chat.v2_group', '/dashboard/workMessage/roomProfile'
  UNION ALL SELECT 'dashboard.chat.v2_group', '/dashboard/workMessage/roomMessages'
  UNION ALL SELECT 'dashboard.chat.v2_group', '/dashboard/workMessage/roomMembers'
  UNION ALL SELECT 'dashboard.chat.v2_group', '/dashboard/workMessage/roomFilterOptions'
) resource_seed ON resource_seed.`permission_code` = p.`code`
WHERE NOT EXISTS (
  SELECT 1
  FROM `mochat_go_dashboard_permission_resources` existing
  WHERE existing.`permission_id` = p.`id`
    AND existing.`resource_type` = 'api'
    AND existing.`http_method` = 'GET'
    AND existing.`path_pattern` = resource_seed.`path_pattern`
);

-- Conversation trajectory uses the same archive and staff data scope as the
-- existing conversation workspace endpoints.
INSERT INTO `mochat_go_dashboard_permission_resources`
  (`permission_id`, `resource_type`, `http_method`, `path_pattern`, `scope_required`, `status`, `version`)
SELECT p.`id`, 'api', 'GET', '/dashboard/workMessage/trajectoryDay', 1, 1, 1
FROM `mochat_go_dashboard_permissions` p
WHERE p.`code` = 'dashboard.chat.trajectory'
  AND NOT EXISTS (
    SELECT 1 FROM `mochat_go_dashboard_permission_resources` existing
    WHERE existing.`permission_id` = p.`id`
      AND existing.`resource_type` = 'api'
      AND existing.`http_method` = 'GET'
      AND existing.`path_pattern` = '/dashboard/workMessage/trajectoryDay'
  );
