ALTER TABLE `mochat_go_saas_packages`
  ADD COLUMN `radars` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '互动雷达上限，0 表示不限' AFTER `shop_codes`;
