ALTER TABLE mc_work_employee_department
  DROP INDEX idx_mc_wed_employee_deleted_department;

ALTER TABLE mc_work_employee
  DROP INDEX idx_mc_we_corp_status_deleted;

ALTER TABLE mc_work_contact_room
  DROP INDEX idx_mc_wcr_room_status_deleted_updated,
  DROP INDEX idx_mc_wcr_room_status_deleted_join;

ALTER TABLE mc_work_room
  DROP INDEX idx_mc_wr_corp_deleted_created_owner;

ALTER TABLE mc_work_contact_employee
  DROP INDEX idx_mc_wce_corp_status_deleted_employee,
  DROP INDEX idx_mc_wce_corp_deleted_create_employee;
