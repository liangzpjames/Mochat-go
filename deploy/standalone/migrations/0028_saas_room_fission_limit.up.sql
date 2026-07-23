ALTER TABLE `mochat_go_saas_packages`
  ADD COLUMN `room_fissions` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '群裂变上限，0 表示不限' AFTER `room_infinite_pulls`;
