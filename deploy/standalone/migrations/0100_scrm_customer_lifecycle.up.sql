CREATE TABLE `mochat_go_scrm_contacts` (
  `id` varchar(36) NOT NULL,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `name` varchar(200) NOT NULL,
  `phone` varchar(64) NOT NULL DEFAULT '',
  `version` bigint unsigned NOT NULL DEFAULT 1,
  `deleted_at` datetime(6) NULL,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), KEY `idx_scrm_contacts_scope` (`tenant_id`,`corp_id`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `mochat_go_scrm_assignments` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `contact_id` varchar(36) NOT NULL, `owner_id` bigint unsigned NULL, `status` varchar(32) NOT NULL,
  `version` bigint unsigned NOT NULL DEFAULT 1, `deleted_at` datetime(6) NULL,
  `created_at` datetime(6) NOT NULL, `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_scrm_assignment_contact` (`tenant_id`,`corp_id`,`contact_id`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `mochat_go_scrm_stages` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `name` varchar(100) NOT NULL, `sort_order` int NOT NULL DEFAULT 0, `version` bigint unsigned NOT NULL DEFAULT 1,
  `deleted_at` datetime(6) NULL, `created_at` datetime(6) NOT NULL, `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_scrm_stage_name` (`tenant_id`,`corp_id`,`name`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `mochat_go_scrm_opportunities` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `contact_id` varchar(36) NOT NULL, `stage_id` varchar(36) NOT NULL, `status` varchar(32) NOT NULL,
  `lost_reason` varchar(500) NOT NULL DEFAULT '', `version` bigint unsigned NOT NULL DEFAULT 1,
  `deleted_at` datetime(6) NULL, `created_at` datetime(6) NOT NULL, `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), KEY `idx_scrm_opportunity_stage` (`tenant_id`,`corp_id`,`stage_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `mochat_go_scrm_follow_ups` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `contact_id` varchar(36) NOT NULL, `content` text NOT NULL, `created_by` bigint unsigned NOT NULL,
  `created_at` datetime(6) NOT NULL, PRIMARY KEY (`id`), KEY `idx_scrm_follow_up_contact` (`tenant_id`,`corp_id`,`contact_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `mochat_go_scrm_tags` (
  `id` varchar(36) NOT NULL, `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL,
  `name` varchar(100) NOT NULL, `version` bigint unsigned NOT NULL DEFAULT 1, `deleted_at` datetime(6) NULL,
  `created_at` datetime(6) NOT NULL, `updated_at` datetime(6) NOT NULL,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_scrm_tag_name` (`tenant_id`,`corp_id`,`name`,`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `mochat_go_scrm_contact_tags` (
  `tenant_id` bigint unsigned NOT NULL, `corp_id` bigint unsigned NOT NULL, `contact_id` varchar(36) NOT NULL, `tag_id` varchar(36) NOT NULL,
  `created_at` datetime(6) NOT NULL, PRIMARY KEY (`tenant_id`,`corp_id`,`contact_id`,`tag_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
