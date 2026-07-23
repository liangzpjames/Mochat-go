ALTER TABLE `mochat_go_saas_packages`
  ADD COLUMN `lotteries` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '抽奖活动上限，0 表示不限' AFTER `radars`;
