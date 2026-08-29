SET @contact_batch_title_up_sql := IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND column_name = 'batch_title') = 0,
  'ALTER TABLE `mc_contact_message_batch_send` ADD COLUMN `batch_title` varchar(100) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '''' AFTER `user_name`',
  'SELECT 1'
);
PREPARE contact_batch_title_up_stmt FROM @contact_batch_title_up_sql;
EXECUTE contact_batch_title_up_stmt;
DEALLOCATE PREPARE contact_batch_title_up_stmt;
