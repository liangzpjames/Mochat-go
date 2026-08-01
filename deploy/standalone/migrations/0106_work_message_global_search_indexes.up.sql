SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_1'
     AND index_name = 'idx_mc_work_message_1_global_search') = 0,
  'ALTER TABLE mc_work_message_1 ADD INDEX idx_mc_work_message_1_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;

SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_2'
     AND index_name = 'idx_mc_work_message_2_global_search') = 0,
  'ALTER TABLE mc_work_message_2 ADD INDEX idx_mc_work_message_2_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;

SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_3'
     AND index_name = 'idx_mc_work_message_3_global_search') = 0,
  'ALTER TABLE mc_work_message_3 ADD INDEX idx_mc_work_message_3_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;

SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_4'
     AND index_name = 'idx_mc_work_message_4_global_search') = 0,
  'ALTER TABLE mc_work_message_4 ADD INDEX idx_mc_work_message_4_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;

SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_5'
     AND index_name = 'idx_mc_work_message_5_global_search') = 0,
  'ALTER TABLE mc_work_message_5 ADD INDEX idx_mc_work_message_5_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;

SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_6'
     AND index_name = 'idx_mc_work_message_6_global_search') = 0,
  'ALTER TABLE mc_work_message_6 ADD INDEX idx_mc_work_message_6_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;

SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_7'
     AND index_name = 'idx_mc_work_message_7_global_search') = 0,
  'ALTER TABLE mc_work_message_7 ADD INDEX idx_mc_work_message_7_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;

SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_8'
     AND index_name = 'idx_mc_work_message_8_global_search') = 0,
  'ALTER TABLE mc_work_message_8 ADD INDEX idx_mc_work_message_8_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;

SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_9'
     AND index_name = 'idx_mc_work_message_9_global_search') = 0,
  'ALTER TABLE mc_work_message_9 ADD INDEX idx_mc_work_message_9_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;

SET @work_message_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_message_10'
     AND index_name = 'idx_mc_work_message_10_global_search') = 0,
  'ALTER TABLE mc_work_message_10 ADD INDEX idx_mc_work_message_10_global_search (corp_id, work_employee_id, deleted_at, to_user_type, msg_data_time, to_user_id, seq)',
  'SELECT 1'
);
PREPARE work_message_index_stmt FROM @work_message_index_ddl;
EXECUTE work_message_index_stmt;
DEALLOCATE PREPARE work_message_index_stmt;
