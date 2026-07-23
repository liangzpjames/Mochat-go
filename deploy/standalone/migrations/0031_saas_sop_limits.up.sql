ALTER TABLE `mochat_go_saas_packages`
  ADD COLUMN `contact_sops` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '个人SOP规则上限，0 表示不限' AFTER `room_reminds`,
  ADD COLUMN `room_sops` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '群SOP规则上限，0 表示不限' AFTER `contact_sops`;
