ALTER TABLE `mochat_go_saas_packages`
  ADD COLUMN `channel_codes` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '渠道活码上限，0 表示不限' AFTER `max_agents`,
  ADD COLUMN `shop_codes` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '门店活码上限，0 表示不限' AFTER `channel_codes`,
  ADD COLUMN `contact_message_batches` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群发任务上限，0 表示不限' AFTER `storage_mb`,
  ADD COLUMN `room_message_batches` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群群发任务上限，0 表示不限' AFTER `contact_message_batches`,
  ADD COLUMN `room_tag_pulls` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '标签建群任务上限，0 表示不限' AFTER `room_message_batches`,
  ADD COLUMN `work_room_auto_pulls` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '自动拉群活码上限，0 表示不限' AFTER `room_tag_pulls`,
  ADD COLUMN `work_fissions` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '裂变活动上限，0 表示不限' AFTER `work_room_auto_pulls`,
  ADD COLUMN `official_accounts` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '公众号授权上限，0 表示不限' AFTER `work_fissions`,
  ADD COLUMN `async_executions` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '异步执行量上限，0 表示不限' AFTER `official_accounts`;
