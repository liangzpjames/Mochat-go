CREATE TABLE `mochat_go_scrm_assignment_collaborators` (
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `assignment_id` varchar(36) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `created_at` datetime(6) NOT NULL,
  PRIMARY KEY (`tenant_id`,`corp_id`,`assignment_id`,`user_id`),
  KEY `idx_scrm_assignment_collaborator_user` (`tenant_id`,`corp_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
