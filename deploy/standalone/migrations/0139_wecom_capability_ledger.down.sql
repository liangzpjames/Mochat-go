-- 0139 rollback preflight
-- A rollback only removes objects whose 0139 signatures are still complete.

SET @wecom_0139_down_invalid := (
  EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') <> 29 OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'PRIMARY'), '') <> 'id' OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND column_name IN ('id','tenant_id','corp_id','capability','action','credential_group','credential_generation','idempotency_key','status','provider_request_id','provider_object_id','actual_agent_id','external_success','callback_evidence','target_total','success_total','failure_total','error_code','actor_user_id','actor_source','request_id','lease_token','lease_expires_at','attempt','requested_at','started_at','finished_at','created_at','updated_at')) <> 29))
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') <> 20 OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND column_name IN ('id','tenant_id','corp_id','operation_id','dispatch_kind','chunk_no','target_id','idempotency_key','status','provider_request_id','provider_message_id','provider_object_id','credential_generation','lease_token','lease_expires_at','attempt','next_poll_at','last_error_code','created_at','updated_at')) <> 20))
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results') <> 12 OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND column_name IN ('id','tenant_id','corp_id','operation_id','target_kind','target_id','status','provider_target_id','error_code','error_message_safe','created_at','updated_at')) <> 12))
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits') <> 16 OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND column_name IN ('id','tenant_id','corp_id','operation_id','dispatch_id','from_status','to_status','action','actor_user_id','actor_source','request_id','error_code','target_total','success_total','failure_total','created_at')) <> 16))
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events') <> 16 OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND column_name IN ('id','tenant_id','corp_id','operation_id','dispatch_id','from_status','to_status','action','actor_user_id','actor_source','request_id','error_code','target_total','success_total','failure_total','created_at')) <> 16))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name IN ('employee_credential_generation','contact_credential_generation','agent_credential_generation','callback_credential_generation') AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND (column_default = '1' OR column_default = '''1''')))
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name IN ('mc_contact_message_batch_send','mc_room_message_batch_send') AND index_name IN ('uk_wecom_contact_batch_scope','uk_wecom_room_batch_scope') AND (non_unique <> 0 OR sub_part IS NOT NULL))
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND index_name = 'uk_wecom_contact_batch_scope' AND COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics s WHERE s.table_schema = DATABASE() AND s.table_name = 'mc_contact_message_batch_send' AND s.index_name = 'uk_wecom_contact_batch_scope'), '') <> 'tenant_id,corp_id,id')
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND index_name = 'uk_wecom_room_batch_scope' AND COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics s WHERE s.table_schema = DATABASE() AND s.table_name = 'mc_room_message_batch_send' AND s.index_name = 'uk_wecom_room_batch_scope'), '') <> 'tenant_id,corp_id,id')
  OR EXISTS (SELECT 1 FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name IN ('mc_contact_message_batch_send','mc_room_message_batch_send') AND constraint_name IN ('fk_wecom_contact_batch_corp','fk_wecom_room_batch_corp') AND (referenced_table_name <> 'mc_corp' OR referenced_column_name NOT IN ('tenant_id','id')))
  OR EXISTS (SELECT 1 FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND constraint_name = 'fk_wecom_contact_batch_corp' AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage k WHERE k.constraint_schema = DATABASE() AND k.table_name = 'mc_contact_message_batch_send' AND k.constraint_name = 'fk_wecom_contact_batch_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id')
  OR EXISTS (SELECT 1 FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND constraint_name = 'fk_wecom_room_batch_corp' AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage k WHERE k.constraint_schema = DATABASE() AND k.table_name = 'mc_room_message_batch_send' AND k.constraint_name = 'fk_wecom_room_batch_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id')
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_idempotency'), '') <> 'tenant_id,corp_id,capability,action,credential_generation,idempotency_key')
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_idempotency') <> 6)
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage k WHERE k.constraint_schema = DATABASE() AND k.table_name = 'mochat_go_wecom_capability_operations' AND k.constraint_name = 'fk_wecom_capability_operation_actor'), '') <> 'tenant_id=mc_user.tenant_id,actor_user_id=mc_user.id')
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mc_contact_message_batch_send','mc_room_message_batch_send') AND column_name = 'tenant_id' AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable IN ('YES','NO') AND column_default IS NULL))
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND column_name = 'tenant_id' AND index_name NOT IN ('uk_wecom_contact_batch_scope'))
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND column_name = 'tenant_id' AND index_name NOT IN ('uk_wecom_room_batch_scope'))
  OR EXISTS (SELECT 1 FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND column_name = 'tenant_id' AND constraint_name NOT IN ('fk_wecom_contact_batch_corp'))
  OR EXISTS (SELECT 1 FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND column_name = 'tenant_id' AND constraint_name NOT IN ('fk_wecom_room_batch_corp'))
);
SET @wecom_0139_down_guard_sql := IF(@wecom_0139_down_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible rollback residual''');
PREPARE wecom_0139_down_guard_stmt FROM @wecom_0139_down_guard_sql;
EXECUTE wecom_0139_down_guard_stmt;
DEALLOCATE PREPARE wecom_0139_down_guard_stmt;

SET @wecom_0139_down_event_sql := IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events') = 1, 'DROP TABLE mochat_go_wecom_capability_operation_events', 'SELECT 1');
PREPARE wecom_0139_down_event_stmt FROM @wecom_0139_down_event_sql;
EXECUTE wecom_0139_down_event_stmt;
DEALLOCATE PREPARE wecom_0139_down_event_stmt;
SET @wecom_0139_down_audit_sql := IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits') = 1, 'DROP TABLE mochat_go_wecom_capability_operation_audits', 'SELECT 1');
PREPARE wecom_0139_down_audit_stmt FROM @wecom_0139_down_audit_sql;
EXECUTE wecom_0139_down_audit_stmt;
DEALLOCATE PREPARE wecom_0139_down_audit_stmt;
SET @wecom_0139_down_result_sql := IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results') = 1, 'DROP TABLE mochat_go_wecom_capability_operation_results', 'SELECT 1');
PREPARE wecom_0139_down_result_stmt FROM @wecom_0139_down_result_sql;
EXECUTE wecom_0139_down_result_stmt;
DEALLOCATE PREPARE wecom_0139_down_result_stmt;
SET @wecom_0139_down_dispatch_sql := IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') = 1, 'DROP TABLE mochat_go_wecom_capability_dispatches', 'SELECT 1');
PREPARE wecom_0139_down_dispatch_stmt FROM @wecom_0139_down_dispatch_sql;
EXECUTE wecom_0139_down_dispatch_stmt;
DEALLOCATE PREPARE wecom_0139_down_dispatch_stmt;
SET @wecom_0139_down_operation_sql := IF((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1, 'DROP TABLE mochat_go_wecom_capability_operations', 'SELECT 1');
PREPARE wecom_0139_down_operation_stmt FROM @wecom_0139_down_operation_sql;
EXECUTE wecom_0139_down_operation_stmt;
DEALLOCATE PREPARE wecom_0139_down_operation_stmt;

SET @wecom_0139_down_contact_fk_sql := IF((SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND constraint_name = 'fk_wecom_contact_batch_corp') = 1, 'ALTER TABLE mc_contact_message_batch_send DROP FOREIGN KEY fk_wecom_contact_batch_corp', 'SELECT 1');
PREPARE wecom_0139_down_contact_fk_stmt FROM @wecom_0139_down_contact_fk_sql;
EXECUTE wecom_0139_down_contact_fk_stmt;
DEALLOCATE PREPARE wecom_0139_down_contact_fk_stmt;
SET @wecom_0139_down_room_fk_sql := IF((SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND constraint_name = 'fk_wecom_room_batch_corp') = 1, 'ALTER TABLE mc_room_message_batch_send DROP FOREIGN KEY fk_wecom_room_batch_corp', 'SELECT 1');
PREPARE wecom_0139_down_room_fk_stmt FROM @wecom_0139_down_room_fk_sql;
EXECUTE wecom_0139_down_room_fk_stmt;
DEALLOCATE PREPARE wecom_0139_down_room_fk_stmt;
SET @wecom_0139_down_contact_index_sql := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND index_name = 'uk_wecom_contact_batch_scope') > 0, 'ALTER TABLE mc_contact_message_batch_send DROP INDEX uk_wecom_contact_batch_scope', 'SELECT 1');
PREPARE wecom_0139_down_contact_index_stmt FROM @wecom_0139_down_contact_index_sql;
EXECUTE wecom_0139_down_contact_index_stmt;
DEALLOCATE PREPARE wecom_0139_down_contact_index_stmt;
SET @wecom_0139_down_room_index_sql := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND index_name = 'uk_wecom_room_batch_scope') > 0, 'ALTER TABLE mc_room_message_batch_send DROP INDEX uk_wecom_room_batch_scope', 'SELECT 1');
PREPARE wecom_0139_down_room_index_stmt FROM @wecom_0139_down_room_index_sql;
EXECUTE wecom_0139_down_room_index_stmt;
DEALLOCATE PREPARE wecom_0139_down_room_index_stmt;

SET @wecom_0139_down_contact_tenant_sql := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND column_name = 'tenant_id') = 1, 'ALTER TABLE mc_contact_message_batch_send DROP COLUMN tenant_id', 'SELECT 1');
PREPARE wecom_0139_down_contact_tenant_stmt FROM @wecom_0139_down_contact_tenant_sql;
EXECUTE wecom_0139_down_contact_tenant_stmt;
DEALLOCATE PREPARE wecom_0139_down_contact_tenant_stmt;
SET @wecom_0139_down_room_tenant_sql := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND column_name = 'tenant_id') = 1, 'ALTER TABLE mc_room_message_batch_send DROP COLUMN tenant_id', 'SELECT 1');
PREPARE wecom_0139_down_room_tenant_stmt FROM @wecom_0139_down_room_tenant_sql;
EXECUTE wecom_0139_down_room_tenant_stmt;
DEALLOCATE PREPARE wecom_0139_down_room_tenant_stmt;

SET @wecom_0139_down_generation_employee_sql := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name = 'employee_credential_generation') = 1, 'ALTER TABLE mochat_go_tenant_corp_bindings DROP COLUMN employee_credential_generation', 'SELECT 1');
PREPARE wecom_0139_down_generation_employee_stmt FROM @wecom_0139_down_generation_employee_sql;
EXECUTE wecom_0139_down_generation_employee_stmt;
DEALLOCATE PREPARE wecom_0139_down_generation_employee_stmt;
SET @wecom_0139_down_generation_contact_sql := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name = 'contact_credential_generation') = 1, 'ALTER TABLE mochat_go_tenant_corp_bindings DROP COLUMN contact_credential_generation', 'SELECT 1');
PREPARE wecom_0139_down_generation_contact_stmt FROM @wecom_0139_down_generation_contact_sql;
EXECUTE wecom_0139_down_generation_contact_stmt;
DEALLOCATE PREPARE wecom_0139_down_generation_contact_stmt;
SET @wecom_0139_down_generation_agent_sql := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name = 'agent_credential_generation') = 1, 'ALTER TABLE mochat_go_tenant_corp_bindings DROP COLUMN agent_credential_generation', 'SELECT 1');
PREPARE wecom_0139_down_generation_agent_stmt FROM @wecom_0139_down_generation_agent_sql;
EXECUTE wecom_0139_down_generation_agent_stmt;
DEALLOCATE PREPARE wecom_0139_down_generation_agent_stmt;
SET @wecom_0139_down_generation_callback_sql := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name = 'callback_credential_generation') = 1, 'ALTER TABLE mochat_go_tenant_corp_bindings DROP COLUMN callback_credential_generation', 'SELECT 1');
PREPARE wecom_0139_down_generation_callback_stmt FROM @wecom_0139_down_generation_callback_sql;
EXECUTE wecom_0139_down_generation_callback_stmt;
DEALLOCATE PREPARE wecom_0139_down_generation_callback_stmt;
