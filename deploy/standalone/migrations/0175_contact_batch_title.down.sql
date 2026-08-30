SET @contact_batch_title_down_sql := IF(
  (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND column_name = 'batch_title') = 1,
  'ALTER TABLE `mc_contact_message_batch_send` DROP COLUMN `batch_title`',
  'SELECT 1'
);
PREPARE contact_batch_title_down_stmt FROM @contact_batch_title_down_sql;
EXECUTE contact_batch_title_down_stmt;
DEALLOCATE PREPARE contact_batch_title_down_stmt;
