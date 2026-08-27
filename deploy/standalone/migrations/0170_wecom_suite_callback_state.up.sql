CREATE TABLE IF NOT EXISTS `mochat_go_wecom_suite_tickets` (
  `suite_id` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL,
  `ticket_ciphertext` longtext COLLATE utf8mb4_bin NOT NULL,
  `ticket_key_id` varchar(64) COLLATE utf8mb4_bin NOT NULL,
  `received_at` datetime(6) NOT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`suite_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Encrypted WeCom suite ticket state';

CREATE TABLE IF NOT EXISTS `mochat_go_wecom_suite_callback_events` (
  `suite_id` varchar(191) COLLATE utf8mb4_unicode_ci NOT NULL,
  `event_digest` char(64) COLLATE utf8mb4_bin NOT NULL,
  `event_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` enum('processing','completed','failed') COLLATE utf8mb4_unicode_ci NOT NULL,
  `attempt` int(10) unsigned NOT NULL DEFAULT 1,
  `received_at` datetime(6) NOT NULL,
  `completed_at` datetime(6) NULL DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`suite_id`,`event_digest`),
  KEY `idx_wecom_suite_callback_status` (`status`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Replay-safe WeCom suite callback ledger';
