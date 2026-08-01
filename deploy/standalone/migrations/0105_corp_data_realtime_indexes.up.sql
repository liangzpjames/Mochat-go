SET @corp_data_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_contact_employee'
     AND index_name = 'idx_mc_wce_corp_deleted_create_employee') = 0,
  'ALTER TABLE mc_work_contact_employee ADD INDEX idx_mc_wce_corp_deleted_create_employee (corp_id, deleted_at, create_time, employee_id)',
  'SELECT 1'
);
PREPARE corp_data_index_stmt FROM @corp_data_index_ddl;
EXECUTE corp_data_index_stmt;
DEALLOCATE PREPARE corp_data_index_stmt;

SET @corp_data_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_contact_employee'
     AND index_name = 'idx_mc_wce_corp_status_deleted_employee') = 0,
  'ALTER TABLE mc_work_contact_employee ADD INDEX idx_mc_wce_corp_status_deleted_employee (corp_id, status, deleted_at, employee_id)',
  'SELECT 1'
);
PREPARE corp_data_index_stmt FROM @corp_data_index_ddl;
EXECUTE corp_data_index_stmt;
DEALLOCATE PREPARE corp_data_index_stmt;

SET @corp_data_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_room'
     AND index_name = 'idx_mc_wr_corp_deleted_created_owner') = 0,
  'ALTER TABLE mc_work_room ADD INDEX idx_mc_wr_corp_deleted_created_owner (corp_id, deleted_at, created_at, owner_id)',
  'SELECT 1'
);
PREPARE corp_data_index_stmt FROM @corp_data_index_ddl;
EXECUTE corp_data_index_stmt;
DEALLOCATE PREPARE corp_data_index_stmt;

SET @corp_data_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_contact_room'
     AND index_name = 'idx_mc_wcr_room_status_deleted_join') = 0,
  'ALTER TABLE mc_work_contact_room ADD INDEX idx_mc_wcr_room_status_deleted_join (room_id, deleted_at, status, join_time)',
  'SELECT 1'
);
PREPARE corp_data_index_stmt FROM @corp_data_index_ddl;
EXECUTE corp_data_index_stmt;
DEALLOCATE PREPARE corp_data_index_stmt;

SET @corp_data_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_contact_room'
     AND index_name = 'idx_mc_wcr_room_status_deleted_updated') = 0,
  'ALTER TABLE mc_work_contact_room ADD INDEX idx_mc_wcr_room_status_deleted_updated (room_id, deleted_at, status, updated_at)',
  'SELECT 1'
);
PREPARE corp_data_index_stmt FROM @corp_data_index_ddl;
EXECUTE corp_data_index_stmt;
DEALLOCATE PREPARE corp_data_index_stmt;

SET @corp_data_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_employee'
     AND index_name = 'idx_mc_we_corp_status_deleted') = 0,
  'ALTER TABLE mc_work_employee ADD INDEX idx_mc_we_corp_status_deleted (corp_id, status, deleted_at)',
  'SELECT 1'
);
PREPARE corp_data_index_stmt FROM @corp_data_index_ddl;
EXECUTE corp_data_index_stmt;
DEALLOCATE PREPARE corp_data_index_stmt;

SET @corp_data_index_ddl = IF(
  (SELECT COUNT(*) FROM information_schema.statistics
   WHERE table_schema = DATABASE() AND table_name = 'mc_work_employee_department'
     AND index_name = 'idx_mc_wed_employee_deleted_department') = 0,
  'ALTER TABLE mc_work_employee_department ADD INDEX idx_mc_wed_employee_deleted_department (employee_id, deleted_at, department_id)',
  'SELECT 1'
);
PREPARE corp_data_index_stmt FROM @corp_data_index_ddl;
EXECUTE corp_data_index_stmt;
DEALLOCATE PREPARE corp_data_index_stmt;
