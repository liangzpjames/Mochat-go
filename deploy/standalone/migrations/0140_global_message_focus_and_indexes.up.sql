CREATE TABLE IF NOT EXISTS `mochat_go_work_message_focus` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` bigint unsigned NOT NULL,
  `corp_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `work_employee_id` int NOT NULL,
  `to_user_type` tinyint NOT NULL,
  `to_user_id` int NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_work_message_focus_subject` (`tenant_id`,`corp_id`,`user_id`,`work_employee_id`,`to_user_type`,`to_user_id`),
  KEY `idx_work_message_focus_list` (`tenant_id`,`corp_id`,`user_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE `mc_work_message_1` ADD KEY `idx_mc_work_message_1_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_2` ADD KEY `idx_mc_work_message_2_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_3` ADD KEY `idx_mc_work_message_3_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_4` ADD KEY `idx_mc_work_message_4_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_5` ADD KEY `idx_mc_work_message_5_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_6` ADD KEY `idx_mc_work_message_6_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_7` ADD KEY `idx_mc_work_message_7_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_8` ADD KEY `idx_mc_work_message_8_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_9` ADD KEY `idx_mc_work_message_9_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
ALTER TABLE `mc_work_message_10` ADD KEY `idx_mc_work_message_10_global` (`corp_id`,`to_user_type`,`msg_data_time`,`work_employee_id`,`to_user_id`,`msg_type`);
