CREATE TABLE IF NOT EXISTS `mochat_go_wework_callback_side_effects` (
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
  `event_key` char(64) COLLATE utf8mb4_bin NOT NULL,
  `action_key` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `payload_hash` char(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `status` enum('pending','unknown','sent') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `last_error` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sent_at` datetime(6) NULL DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`tenant_id`,`corp_id`,`event_key`,`action_key`),
  KEY `idx_wework_callback_side_effect_status` (`status`,`updated_at`),
  CONSTRAINT `fk_wework_callback_side_effect_event` FOREIGN KEY (`tenant_id`,`corp_id`,`event_key`) REFERENCES `mochat_go_wework_callback_inbox` (`tenant_id`,`corp_id`,`event_key`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Fail-closed durable intents for callback external side effects';
