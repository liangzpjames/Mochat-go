ALTER TABLE `mc_work_message_id`
  MODIFY COLUMN `last_id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '最后一次查询最大id或seq';

ALTER TABLE `mc_work_message_1`
  ADD KEY `idx_mc_work_message_1_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_1_corp_seq` (`corp_id`, `seq`);
ALTER TABLE `mc_work_message_2`
  ADD KEY `idx_mc_work_message_2_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_2_corp_seq` (`corp_id`, `seq`);
ALTER TABLE `mc_work_message_3`
  ADD KEY `idx_mc_work_message_3_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_3_corp_seq` (`corp_id`, `seq`);
ALTER TABLE `mc_work_message_4`
  ADD KEY `idx_mc_work_message_4_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_4_corp_seq` (`corp_id`, `seq`);
ALTER TABLE `mc_work_message_5`
  ADD KEY `idx_mc_work_message_5_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_5_corp_seq` (`corp_id`, `seq`);
ALTER TABLE `mc_work_message_6`
  ADD KEY `idx_mc_work_message_6_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_6_corp_seq` (`corp_id`, `seq`);
ALTER TABLE `mc_work_message_7`
  ADD KEY `idx_mc_work_message_7_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_7_corp_seq` (`corp_id`, `seq`);
ALTER TABLE `mc_work_message_8`
  ADD KEY `idx_mc_work_message_8_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_8_corp_seq` (`corp_id`, `seq`);
ALTER TABLE `mc_work_message_9`
  ADD KEY `idx_mc_work_message_9_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_9_corp_seq` (`corp_id`, `seq`);
ALTER TABLE `mc_work_message_10`
  ADD KEY `idx_mc_work_message_10_corp_msgid` (`corp_id`, `msgid`),
  ADD KEY `idx_mc_work_message_10_corp_seq` (`corp_id`, `seq`);
