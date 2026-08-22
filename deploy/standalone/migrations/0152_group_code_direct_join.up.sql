ALTER TABLE `mc_work_room_auto_pull`
  ADD COLUMN `provider_kind` varchar(20) NOT NULL DEFAULT 'contact_way' AFTER `wx_config_id`,
  ADD COLUMN `auto_create_room` tinyint unsigned NOT NULL DEFAULT 0 AFTER `provider_kind`,
  ADD COLUMN `room_base_name` varchar(30) NOT NULL DEFAULT '' AFTER `auto_create_room`,
  ADD COLUMN `room_base_id` int unsigned NOT NULL DEFAULT 1 AFTER `room_base_name`;
