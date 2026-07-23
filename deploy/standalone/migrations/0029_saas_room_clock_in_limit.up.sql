ALTER TABLE `mochat_go_saas_packages`
  ADD COLUMN `room_clock_ins` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '群打卡上限，0 表示不限' AFTER `room_fissions`;
