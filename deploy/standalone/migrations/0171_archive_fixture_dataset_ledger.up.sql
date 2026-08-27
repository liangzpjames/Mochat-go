CREATE TABLE IF NOT EXISTS `mochat_go_archive_fixture_datasets` (
  `dataset` varchar(96) COLLATE utf8mb4_bin NOT NULL,
  `tenant_id` int(10) unsigned NOT NULL,
  `corp_id` int(10) unsigned NOT NULL,
  `integration_mode` enum('self_built','third_party_delegated') COLLATE utf8mb4_unicode_ci NOT NULL,
  `source_identity` varchar(191) COLLATE utf8mb4_bin NOT NULL,
  `status` enum('seeding','ready','cleaning') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'seeding',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`dataset`),
  UNIQUE KEY `uk_archive_fixture_source` (`tenant_id`,`corp_id`,`source_identity`),
  KEY `idx_archive_fixture_scope_status` (`tenant_id`,`corp_id`,`status`,`updated_at`),
  CONSTRAINT `fk_archive_fixture_corp` FOREIGN KEY (`tenant_id`,`corp_id`) REFERENCES `mc_corp` (`tenant_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Local acceptance archive fixture ownership ledger';
