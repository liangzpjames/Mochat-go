CREATE TABLE IF NOT EXISTS `mochat_go_audio_objects` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT 0,
  `user_id` int(10) unsigned NOT NULL DEFAULT 0,
  `employee_id` int(10) unsigned NOT NULL DEFAULT 0,
  `corp_id` int(10) unsigned NOT NULL DEFAULT 0,
  `original_name` varchar(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `relative_path` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `content_type` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `size_bytes` bigint(20) unsigned NOT NULL DEFAULT 0,
  `duration_seconds` int(10) unsigned NOT NULL DEFAULT 0,
  `sha256` char(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  `deleted_at` timestamp NULL DEFAULT NULL,
  `deleted_by` int(10) unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_audio_objects_corp_created` (`corp_id`, `deleted_at`, `created_at`),
  KEY `idx_audio_objects_path` (`relative_path`(191))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Phase 3 Final 文件录音音频元数据';

CREATE TABLE IF NOT EXISTS `mochat_go_ai_analysis` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT 0,
  `corp_id` int(10) unsigned NOT NULL DEFAULT 0,
  `page` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'succeeded',
  `payload` json DEFAULT NULL,
  `error` varchar(1024) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NULL DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_ai_analysis_corp_page_created` (`corp_id`, `page`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Phase 3 Final AI 洞察结果落库';
