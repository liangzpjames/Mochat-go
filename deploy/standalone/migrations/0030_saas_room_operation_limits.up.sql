ALTER TABLE `mochat_go_saas_packages`
  ADD COLUMN `room_qualities` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '群质检规则上限，0 表示不限' AFTER `room_clock_ins`,
  ADD COLUMN `room_calendars` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '群日历上限，0 表示不限' AFTER `room_qualities`,
  ADD COLUMN `room_reminds` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '客户群提醒上限，0 表示不限' AFTER `room_calendars`;
