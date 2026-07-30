CREATE TABLE `mochat_go_scrm_leads` (
  `id` varchar(36) NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `business_key` varchar(128) NOT NULL,
  `name` varchar(200) NOT NULL,
  `source` varchar(32) NOT NULL,
  `status` varchar(32) NOT NULL,
  `version` bigint unsigned NOT NULL DEFAULT 1,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_scrm_leads_tenant_business_key` (`tenant_id`,`business_key`),
  KEY `idx_scrm_leads_tenant_created_id` (`tenant_id`,`created_at`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
