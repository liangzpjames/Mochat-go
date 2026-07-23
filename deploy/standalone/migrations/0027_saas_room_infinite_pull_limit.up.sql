ALTER TABLE `mochat_go_saas_packages`
  ADD COLUMN `room_infinite_pulls` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '无限拉群上限，0 表示不限' AFTER `lotteries`;
