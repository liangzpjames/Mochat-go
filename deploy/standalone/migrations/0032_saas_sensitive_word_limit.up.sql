ALTER TABLE `mochat_go_saas_packages`
  ADD COLUMN `sensitive_words` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '敏感词词库上限，0 表示不限' AFTER `room_sops`;
