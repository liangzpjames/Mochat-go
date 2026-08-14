-- 0139 durable WeCom capability ledger preflight
-- Every statement before the first ALTER or CREATE is metadata or data validation.

SET @wecom_0139_parent_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('mc_corp','mc_user','mochat_go_tenant_corp_bindings','mc_contact_message_batch_send','mc_room_message_batch_send')) <> 5
  OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_corp' AND column_name IN ('id','tenant_id')) <> 2
  OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_user' AND column_name IN ('id','tenant_id')) <> 2
  OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name IN ('tenant_id','corp_id','version')) <> 3
  OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND column_name IN ('id','corp_id')) <> 2
  OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND column_name IN ('id','corp_id')) <> 2
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mc_corp','mc_user') AND column_name IN ('id','tenant_id') AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO'))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name IN ('tenant_id','corp_id') AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO'))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name = 'version' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO'))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mc_contact_message_batch_send','mc_room_message_batch_send') AND column_name IN ('id','corp_id') AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO'))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mc_contact_message_batch_send','mc_room_message_batch_send') AND column_name = 'tenant_id' AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable IN ('YES','NO') AND column_default IS NULL))
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_corp' AND index_name = 'uni_mc_corp_tenant_id_id' AND (non_unique <> 0 OR sub_part IS NOT NULL OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics s2 WHERE s2.table_schema = DATABASE() AND s2.table_name = 'mc_corp' AND s2.index_name = 'uni_mc_corp_tenant_id_id'), '') <> 'tenant_id,id'))
  OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_corp' AND index_name = 'uni_mc_corp_tenant_id_id') = 0
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_user' AND index_name = 'uni_dashboard_user_tenant_id_id' AND (non_unique <> 0 OR sub_part IS NOT NULL OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics s2 WHERE s2.table_schema = DATABASE() AND s2.table_name = 'mc_user' AND s2.index_name = 'uni_dashboard_user_tenant_id_id'), '') <> 'tenant_id,id'))
  OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_user' AND index_name = 'uni_dashboard_user_tenant_id_id') = 0
  OR EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND constraint_name = 'fk_tenant_corp_binding_corp' AND constraint_type <> 'FOREIGN KEY')
  OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND constraint_name = 'fk_tenant_corp_binding_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id'
);
SET @wecom_0139_parent_guard_sql := IF(@wecom_0139_parent_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible parent or dependency schema''');
PREPARE wecom_0139_parent_guard_stmt FROM @wecom_0139_parent_guard_sql;
EXECUTE wecom_0139_parent_guard_stmt;
DEALLOCATE PREPARE wecom_0139_parent_guard_stmt;

SET @wecom_0139_contact_tenant_check_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('mc_contact_message_batch_send','mc_corp')) <> 2,
  'SELECT @wecom_0139_contact_tenant_bad := 1',
  IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND column_name = 'tenant_id') = 0,
    'SELECT @wecom_0139_contact_tenant_bad := (SELECT COUNT(*) FROM mc_contact_message_batch_send b LEFT JOIN mc_corp c ON c.id = b.corp_id WHERE c.id IS NULL OR c.tenant_id IS NULL)',
    'SELECT @wecom_0139_contact_tenant_bad := (SELECT COUNT(*) FROM mc_contact_message_batch_send b LEFT JOIN mc_corp c ON c.id = b.corp_id WHERE c.id IS NULL OR c.tenant_id IS NULL OR (b.tenant_id IS NOT NULL AND c.tenant_id <> b.tenant_id))'
  )
);
PREPARE wecom_0139_contact_tenant_check_stmt FROM @wecom_0139_contact_tenant_check_sql;
EXECUTE wecom_0139_contact_tenant_check_stmt;
DEALLOCATE PREPARE wecom_0139_contact_tenant_check_stmt;
SET @wecom_0139_room_tenant_check_sql := IF(
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('mc_room_message_batch_send','mc_corp')) <> 2,
  'SELECT @wecom_0139_room_tenant_bad := 1',
  IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND column_name = 'tenant_id') = 0,
    'SELECT @wecom_0139_room_tenant_bad := (SELECT COUNT(*) FROM mc_room_message_batch_send b LEFT JOIN mc_corp c ON c.id = b.corp_id WHERE c.id IS NULL OR c.tenant_id IS NULL)',
    'SELECT @wecom_0139_room_tenant_bad := (SELECT COUNT(*) FROM mc_room_message_batch_send b LEFT JOIN mc_corp c ON c.id = b.corp_id WHERE c.id IS NULL OR c.tenant_id IS NULL OR (b.tenant_id IS NOT NULL AND c.tenant_id <> b.tenant_id))'
  )
);
PREPARE wecom_0139_room_tenant_check_stmt FROM @wecom_0139_room_tenant_check_sql;
EXECUTE wecom_0139_room_tenant_check_stmt;
DEALLOCATE PREPARE wecom_0139_room_tenant_check_stmt;
SET @wecom_0139_parent_data_guard_sql := IF(COALESCE(@wecom_0139_contact_tenant_bad, 0) + COALESCE(@wecom_0139_room_tenant_bad, 0) = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible batch tenant data''');
PREPARE wecom_0139_parent_data_guard_stmt FROM @wecom_0139_parent_data_guard_sql;
EXECUTE wecom_0139_parent_data_guard_stmt;
DEALLOCATE PREPARE wecom_0139_parent_data_guard_stmt;

SET @wecom_0139_binding_generation_invalid := (
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name IN ('employee_credential_generation','contact_credential_generation','agent_credential_generation','callback_credential_generation') AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND (column_default = '1' OR column_default = '''1''')))
);
SET @wecom_0139_binding_generation_guard_sql := IF(@wecom_0139_binding_generation_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible credential generation column''');
PREPARE wecom_0139_binding_generation_guard_stmt FROM @wecom_0139_binding_generation_guard_sql;
EXECUTE wecom_0139_binding_generation_guard_stmt;
DEALLOCATE PREPARE wecom_0139_binding_generation_guard_stmt;

SET @wecom_0139_parent_index_invalid := (
  (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND index_name = 'uk_wecom_contact_batch_scope') > 0 AND (SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND index_name = 'uk_wecom_contact_batch_scope') <> 'tenant_id,corp_id,id'
  OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND index_name = 'uk_wecom_contact_batch_scope') > 0 AND ((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND index_name = 'uk_wecom_contact_batch_scope') <> 0 OR (SELECT SUM(sub_part IS NOT NULL) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND index_name = 'uk_wecom_contact_batch_scope') <> 0 OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND index_name = 'uk_wecom_contact_batch_scope') <> 3)
  OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND index_name = 'uk_wecom_room_batch_scope') > 0 AND (SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND index_name = 'uk_wecom_room_batch_scope') <> 'tenant_id,corp_id,id'
  OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND index_name = 'uk_wecom_room_batch_scope') > 0 AND ((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND index_name = 'uk_wecom_room_batch_scope') <> 0 OR (SELECT SUM(sub_part IS NOT NULL) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND index_name = 'uk_wecom_room_batch_scope') <> 0 OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND index_name = 'uk_wecom_room_batch_scope') <> 3)
);
SET @wecom_0139_parent_index_guard_sql := IF(@wecom_0139_parent_index_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible batch scope index''');
PREPARE wecom_0139_parent_index_guard_stmt FROM @wecom_0139_parent_index_guard_sql;
EXECUTE wecom_0139_parent_index_guard_stmt;
DEALLOCATE PREPARE wecom_0139_parent_index_guard_stmt;

SET @wecom_0139_operations_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND (
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') <> 29
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'PRIMARY'), '') <> 'id'
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND column_name IN ('id','tenant_id','corp_id','capability','action','credential_group','credential_generation','idempotency_key','status','provider_request_id','provider_object_id','actual_agent_id','external_success','callback_evidence','target_total','success_total','failure_total','error_code','actor_user_id','actor_source','request_id','lease_token','lease_expires_at','attempt','requested_at','started_at','finished_at','created_at','updated_at')) <> 29
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND ((column_name IN ('id','credential_generation') AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('tenant_id','corp_id','target_total','success_total','failure_total','attempt') AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('capability','action','idempotency_key','status','provider_request_id','provider_object_id','actual_agent_id','error_code','actor_source','request_id','lease_token') AND NOT (data_type = 'varchar' AND is_nullable = 'NO' AND collation_name = 'utf8mb4_unicode_ci')) OR (column_name IN ('external_success','callback_evidence') AND NOT (data_type = 'tinyint' AND is_nullable = 'NO')) OR (column_name = 'actor_user_id' AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'YES')) OR (column_name IN ('lease_expires_at','started_at','finished_at') AND NOT (data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'YES')) OR (column_name IN ('requested_at','created_at','updated_at') AND NOT (data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO'))))
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_scope_id') <> 3
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND ((column_name = 'capability' AND character_maximum_length <> 64) OR (column_name = 'action' AND character_maximum_length <> 32) OR (column_name = 'credential_group' AND character_maximum_length <> 16) OR (column_name = 'idempotency_key' AND character_maximum_length <> 128) OR (column_name = 'status' AND character_maximum_length <> 24) OR (column_name IN ('provider_request_id','provider_object_id','actual_agent_id','request_id','lease_token') AND character_maximum_length <> 128) OR (column_name = 'error_code' AND character_maximum_length <> 96) OR (column_name = 'actor_source' AND character_maximum_length <> 16)))
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_scope_id'), '') <> 'tenant_id,corp_id,id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_scope_id'), -1) <> 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_status') <> 4
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_status'), '') <> 'tenant_id,corp_id,status,updated_at'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_agent') <> 3
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_agent'), '') <> 'tenant_id,corp_id,actual_agent_id'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_idempotency'), '') <> 'tenant_id,corp_id,capability,action,credential_generation,idempotency_key'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_idempotency') <> 6
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_idempotency'), -1) <> 0
    OR (SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_status') <> 'tenant_id,corp_id,status,updated_at'
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND constraint_name = 'fk_wecom_capability_operation_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id'
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND constraint_name = 'fk_wecom_capability_operation_actor'), '') <> 'tenant_id=mc_user.tenant_id,actor_user_id=mc_user.id'
  )
);
SET @wecom_0139_operations_guard_sql := IF(@wecom_0139_operations_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible capability operations table''');
PREPARE wecom_0139_operations_guard_stmt FROM @wecom_0139_operations_guard_sql;
EXECUTE wecom_0139_operations_guard_stmt;
DEALLOCATE PREPARE wecom_0139_operations_guard_stmt;

SET @wecom_0139_dispatches_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') = 1 AND (
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') <> 20
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'PRIMARY'), '') <> 'id'
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND column_name IN ('id','tenant_id','corp_id','operation_id','dispatch_kind','chunk_no','target_id','idempotency_key','status','provider_request_id','provider_message_id','provider_object_id','credential_generation','lease_token','lease_expires_at','attempt','next_poll_at','last_error_code','created_at','updated_at')) <> 20
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND ((column_name IN ('id','operation_id','credential_generation') AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('tenant_id','corp_id','chunk_no','attempt') AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('dispatch_kind','target_id','idempotency_key','status','provider_request_id','provider_message_id','provider_object_id','lease_token','last_error_code') AND NOT (data_type = 'varchar' AND is_nullable = 'NO' AND collation_name = 'utf8mb4_unicode_ci')) OR (column_name IN ('lease_expires_at','next_poll_at') AND NOT (data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'YES')) OR (column_name IN ('created_at','updated_at') AND NOT (data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO'))))
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_scope_id'), '') <> 'tenant_id,corp_id,id'
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND ((column_name = 'dispatch_kind' AND character_maximum_length <> 32) OR (column_name = 'target_id' AND character_maximum_length <> 255) OR (column_name = 'idempotency_key' AND character_maximum_length <> 128) OR (column_name = 'status' AND character_maximum_length <> 24) OR (column_name IN ('provider_request_id','provider_message_id','provider_object_id','lease_token') AND character_maximum_length <> 128) OR (column_name = 'last_error_code' AND character_maximum_length <> 96)))
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_scope_id') <> 3
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_scope_id'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_idempotency'), '') <> 'tenant_id,corp_id,idempotency_key'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_idempotency') <> 3
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_idempotency'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_target_chunk'), '') <> 'tenant_id,corp_id,operation_id,dispatch_kind,target_id,chunk_no'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_target_chunk') <> 6
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_target_chunk'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'idx_wecom_capability_dispatch_claim'), '') <> 'tenant_id,corp_id,status,next_poll_at'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'idx_wecom_capability_dispatch_claim') <> 4
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND constraint_name = 'fk_wecom_capability_dispatch_operation'), '') <> 'tenant_id=mochat_go_wecom_capability_operations.tenant_id,corp_id=mochat_go_wecom_capability_operations.corp_id,operation_id=mochat_go_wecom_capability_operations.id'
  )
);
SET @wecom_0139_dispatches_guard_sql := IF(@wecom_0139_dispatches_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible capability dispatches table''');
PREPARE wecom_0139_dispatches_guard_stmt FROM @wecom_0139_dispatches_guard_sql;
EXECUTE wecom_0139_dispatches_guard_stmt;
DEALLOCATE PREPARE wecom_0139_dispatches_guard_stmt;

SET @wecom_0139_results_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results') = 1 AND (
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results') <> 12
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'PRIMARY'), '') <> 'id'
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND column_name IN ('id','tenant_id','corp_id','operation_id','target_kind','target_id','status','provider_target_id','error_code','error_message_safe','created_at','updated_at')) <> 12
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND ((column_name = 'id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('tenant_id','corp_id') AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name = 'operation_id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('target_kind','target_id','status','provider_target_id','error_code','error_message_safe') AND NOT (data_type = 'varchar' AND is_nullable = 'NO' AND collation_name = 'utf8mb4_unicode_ci')) OR (column_name IN ('created_at','updated_at') AND NOT (data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO'))))
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_scope_id'), '') <> 'tenant_id,corp_id,id'
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND ((column_name = 'target_kind' AND character_maximum_length <> 32) OR (column_name IN ('target_id','provider_target_id','error_message_safe') AND character_maximum_length <> 255) OR (column_name = 'status' AND character_maximum_length <> 24) OR (column_name = 'error_code' AND character_maximum_length <> 96)))
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_scope_id') <> 3
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_scope_id'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND constraint_name = 'fk_wecom_capability_result_operation'), '') <> 'tenant_id=mochat_go_wecom_capability_operations.tenant_id,corp_id=mochat_go_wecom_capability_operations.corp_id,operation_id=mochat_go_wecom_capability_operations.id'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_target') <> 5
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_target'), '') <> 'tenant_id,corp_id,operation_id,target_kind,target_id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_target'), -1) <> 0
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'idx_wecom_capability_result_operation') <> 4
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'idx_wecom_capability_result_operation'), '') <> 'tenant_id,corp_id,operation_id,updated_at'
  )
);
SET @wecom_0139_results_guard_sql := IF(@wecom_0139_results_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible capability results table''');
PREPARE wecom_0139_results_guard_stmt FROM @wecom_0139_results_guard_sql;
EXECUTE wecom_0139_results_guard_stmt;
DEALLOCATE PREPARE wecom_0139_results_guard_stmt;

SET @wecom_0139_audits_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits') = 1 AND (
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits') <> 16
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND index_name = 'PRIMARY'), '') <> 'id'
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND column_name IN ('id','tenant_id','corp_id','operation_id','dispatch_id','from_status','to_status','action','actor_user_id','actor_source','request_id','error_code','target_total','success_total','failure_total','created_at')) <> 16
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND ((column_name = 'id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('tenant_id','corp_id','target_total','success_total','failure_total') AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('operation_id','dispatch_id','actor_user_id') AND NOT (data_type IN ('bigint','int') AND column_type LIKE '%unsigned%' AND ((column_name = 'dispatch_id' OR column_name = 'actor_user_id') AND is_nullable = 'YES' OR (column_name = 'operation_id' AND is_nullable = 'NO')))) OR (column_name IN ('from_status','to_status','action','actor_source','request_id','error_code') AND NOT (data_type = 'varchar' AND is_nullable = 'NO' AND collation_name = 'utf8mb4_unicode_ci')) OR (column_name = 'created_at' AND NOT (data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO'))))
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND ((column_name = 'operation_id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name = 'dispatch_id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'YES')) OR (column_name = 'actor_user_id' AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'YES'))))
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND constraint_name = 'fk_wecom_capability_audit_operation'), '') <> 'tenant_id=mochat_go_wecom_capability_operations.tenant_id,corp_id=mochat_go_wecom_capability_operations.corp_id,operation_id=mochat_go_wecom_capability_operations.id'
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND ((column_name IN ('from_status','to_status') AND character_maximum_length <> 24) OR (column_name = 'action' AND character_maximum_length <> 32) OR (column_name = 'actor_source' AND character_maximum_length <> 16) OR (column_name = 'request_id' AND character_maximum_length <> 128) OR (column_name = 'error_code' AND character_maximum_length <> 96)))
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND constraint_name = 'fk_wecom_capability_audit_dispatch'), '') <> 'tenant_id=mochat_go_wecom_capability_dispatches.tenant_id,corp_id=mochat_go_wecom_capability_dispatches.corp_id,dispatch_id=mochat_go_wecom_capability_dispatches.id'
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND constraint_name = 'fk_wecom_capability_audit_actor'), '') <> 'tenant_id=mc_user.tenant_id,actor_user_id=mc_user.id'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND index_name = 'idx_wecom_capability_audit_operation'), '') <> 'tenant_id,corp_id,operation_id,created_at'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND index_name = 'idx_wecom_capability_audit_operation') <> 4
  )
);
SET @wecom_0139_audits_guard_sql := IF(@wecom_0139_audits_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible capability audits table''');
PREPARE wecom_0139_audits_guard_stmt FROM @wecom_0139_audits_guard_sql;
EXECUTE wecom_0139_audits_guard_stmt;
DEALLOCATE PREPARE wecom_0139_audits_guard_stmt;

SET @wecom_0139_events_invalid := (
  (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events') = 1 AND (
    (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events') <> 16
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND index_name = 'PRIMARY'), '') <> 'id'
    OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND column_name IN ('id','tenant_id','corp_id','operation_id','dispatch_id','from_status','to_status','action','actor_user_id','actor_source','request_id','error_code','target_total','success_total','failure_total','created_at')) <> 16
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND ((column_name = 'id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('tenant_id','corp_id','target_total','success_total','failure_total') AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name IN ('operation_id','dispatch_id','actor_user_id') AND NOT (data_type IN ('bigint','int') AND column_type LIKE '%unsigned%' AND ((column_name = 'dispatch_id' OR column_name = 'actor_user_id') AND is_nullable = 'YES' OR (column_name = 'operation_id' AND is_nullable = 'NO')))) OR (column_name IN ('from_status','to_status','action','actor_source','request_id','error_code') AND NOT (data_type = 'varchar' AND is_nullable = 'NO' AND collation_name = 'utf8mb4_unicode_ci')) OR (column_name = 'created_at' AND NOT (data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO'))))
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND ((column_name = 'operation_id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO')) OR (column_name = 'dispatch_id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'YES')) OR (column_name = 'actor_user_id' AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'YES'))))
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND constraint_name = 'fk_wecom_capability_event_operation'), '') <> 'tenant_id=mochat_go_wecom_capability_operations.tenant_id,corp_id=mochat_go_wecom_capability_operations.corp_id,operation_id=mochat_go_wecom_capability_operations.id'
    OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND ((column_name IN ('from_status','to_status') AND character_maximum_length <> 24) OR (column_name = 'action' AND character_maximum_length <> 32) OR (column_name = 'actor_source' AND character_maximum_length <> 16) OR (column_name = 'request_id' AND character_maximum_length <> 128) OR (column_name = 'error_code' AND character_maximum_length <> 96)))
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND constraint_name = 'fk_wecom_capability_event_dispatch'), '') <> 'tenant_id=mochat_go_wecom_capability_dispatches.tenant_id,corp_id=mochat_go_wecom_capability_dispatches.corp_id,dispatch_id=mochat_go_wecom_capability_dispatches.id'
    OR COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND constraint_name = 'fk_wecom_capability_event_actor'), '') <> 'tenant_id=mc_user.tenant_id,actor_user_id=mc_user.id'
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND index_name = 'idx_wecom_capability_event_operation'), '') <> 'tenant_id,corp_id,operation_id,created_at'
    OR (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND index_name = 'idx_wecom_capability_event_operation') <> 4
  )
);
SET @wecom_0139_events_guard_sql := IF(@wecom_0139_events_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible capability events table''');
PREPARE wecom_0139_events_guard_stmt FROM @wecom_0139_events_guard_sql;
EXECUTE wecom_0139_events_guard_stmt;
DEALLOCATE PREPARE wecom_0139_events_guard_stmt;

SET @wecom_0139_parent_fk_invalid := (
  EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND constraint_name = 'fk_wecom_contact_batch_corp' AND constraint_type <> 'FOREIGN KEY')
  OR EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND constraint_name = 'fk_wecom_room_batch_corp' AND constraint_type <> 'FOREIGN KEY')
  OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND constraint_name = 'fk_wecom_contact_batch_corp') > 0 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND constraint_name = 'fk_wecom_contact_batch_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id'
  OR (SELECT COUNT(*) FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND constraint_name = 'fk_wecom_room_batch_corp') > 0 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND constraint_name = 'fk_wecom_room_batch_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id'
);
SET @wecom_0139_parent_fk_guard_sql := IF(@wecom_0139_parent_fk_invalid = 0, 'SELECT 1', 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible parent foreign key''');
PREPARE wecom_0139_parent_fk_guard_stmt FROM @wecom_0139_parent_fk_guard_sql;
EXECUTE wecom_0139_parent_fk_guard_stmt;
DEALLOCATE PREPARE wecom_0139_parent_fk_guard_stmt;

SET @wecom_0139_generation_employee_ddl := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name = 'employee_credential_generation') = 0, 'ALTER TABLE mochat_go_tenant_corp_bindings ADD COLUMN employee_credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 1', 'SELECT 1');
PREPARE wecom_0139_generation_employee_ddl_stmt FROM @wecom_0139_generation_employee_ddl;
EXECUTE wecom_0139_generation_employee_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_generation_employee_ddl_stmt;
SET @wecom_0139_generation_contact_ddl := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name = 'contact_credential_generation') = 0, 'ALTER TABLE mochat_go_tenant_corp_bindings ADD COLUMN contact_credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 1', 'SELECT 1');
PREPARE wecom_0139_generation_contact_ddl_stmt FROM @wecom_0139_generation_contact_ddl;
EXECUTE wecom_0139_generation_contact_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_generation_contact_ddl_stmt;
SET @wecom_0139_generation_agent_ddl := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name = 'agent_credential_generation') = 0, 'ALTER TABLE mochat_go_tenant_corp_bindings ADD COLUMN agent_credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 1', 'SELECT 1');
PREPARE wecom_0139_generation_agent_ddl_stmt FROM @wecom_0139_generation_agent_ddl;
EXECUTE wecom_0139_generation_agent_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_generation_agent_ddl_stmt;
SET @wecom_0139_generation_callback_ddl := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_tenant_corp_bindings' AND column_name = 'callback_credential_generation') = 0, 'ALTER TABLE mochat_go_tenant_corp_bindings ADD COLUMN callback_credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 1', 'SELECT 1');
PREPARE wecom_0139_generation_callback_ddl_stmt FROM @wecom_0139_generation_callback_ddl;
EXECUTE wecom_0139_generation_callback_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_generation_callback_ddl_stmt;

SET @wecom_0139_contact_tenant_ddl := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND column_name = 'tenant_id') = 0, 'ALTER TABLE mc_contact_message_batch_send ADD COLUMN tenant_id INT UNSIGNED NULL AFTER id', 'SELECT 1');
PREPARE wecom_0139_contact_tenant_ddl_stmt FROM @wecom_0139_contact_tenant_ddl;
EXECUTE wecom_0139_contact_tenant_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_contact_tenant_ddl_stmt;
SET @wecom_0139_room_tenant_ddl := IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND column_name = 'tenant_id') = 0, 'ALTER TABLE mc_room_message_batch_send ADD COLUMN tenant_id INT UNSIGNED NULL AFTER id', 'SELECT 1');
PREPARE wecom_0139_room_tenant_ddl_stmt FROM @wecom_0139_room_tenant_ddl;
EXECUTE wecom_0139_room_tenant_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_room_tenant_ddl_stmt;
UPDATE mc_contact_message_batch_send b JOIN mc_corp c ON c.id = b.corp_id SET b.tenant_id = c.tenant_id WHERE b.tenant_id IS NULL;
UPDATE mc_room_message_batch_send b JOIN mc_corp c ON c.id = b.corp_id SET b.tenant_id = c.tenant_id WHERE b.tenant_id IS NULL;
ALTER TABLE mc_contact_message_batch_send MODIFY COLUMN tenant_id INT UNSIGNED NOT NULL;
ALTER TABLE mc_room_message_batch_send MODIFY COLUMN tenant_id INT UNSIGNED NOT NULL;

SET @wecom_0139_contact_scope_ddl := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND index_name = 'uk_wecom_contact_batch_scope') = 0, 'ALTER TABLE mc_contact_message_batch_send ADD UNIQUE KEY uk_wecom_contact_batch_scope (tenant_id,corp_id,id)', 'SELECT 1');
PREPARE wecom_0139_contact_scope_ddl_stmt FROM @wecom_0139_contact_scope_ddl;
EXECUTE wecom_0139_contact_scope_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_contact_scope_ddl_stmt;
SET @wecom_0139_room_scope_ddl := IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND index_name = 'uk_wecom_room_batch_scope') = 0, 'ALTER TABLE mc_room_message_batch_send ADD UNIQUE KEY uk_wecom_room_batch_scope (tenant_id,corp_id,id)', 'SELECT 1');
PREPARE wecom_0139_room_scope_ddl_stmt FROM @wecom_0139_room_scope_ddl;
EXECUTE wecom_0139_room_scope_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_room_scope_ddl_stmt;
SET @wecom_0139_contact_fk_ddl := IF((SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND constraint_name = 'fk_wecom_contact_batch_corp') = 0, 'ALTER TABLE mc_contact_message_batch_send ADD CONSTRAINT fk_wecom_contact_batch_corp FOREIGN KEY (tenant_id,corp_id) REFERENCES mc_corp (tenant_id,id) ON DELETE RESTRICT', 'SELECT 1');
PREPARE wecom_0139_contact_fk_ddl_stmt FROM @wecom_0139_contact_fk_ddl;
EXECUTE wecom_0139_contact_fk_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_contact_fk_ddl_stmt;
SET @wecom_0139_room_fk_ddl := IF((SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND constraint_name = 'fk_wecom_room_batch_corp') = 0, 'ALTER TABLE mc_room_message_batch_send ADD CONSTRAINT fk_wecom_room_batch_corp FOREIGN KEY (tenant_id,corp_id) REFERENCES mc_corp (tenant_id,id) ON DELETE RESTRICT', 'SELECT 1');
PREPARE wecom_0139_room_fk_ddl_stmt FROM @wecom_0139_room_fk_ddl;
EXECUTE wecom_0139_room_fk_ddl_stmt;
DEALLOCATE PREPARE wecom_0139_room_fk_ddl_stmt;

CREATE TABLE IF NOT EXISTS mochat_go_wecom_capability_operations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id INT UNSIGNED NOT NULL,
  corp_id INT UNSIGNED NOT NULL,
  capability VARCHAR(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  action VARCHAR(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  credential_group VARCHAR(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 0,
  idempotency_key VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  status VARCHAR(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  provider_request_id VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  provider_object_id VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  actual_agent_id VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  external_success TINYINT(1) NOT NULL DEFAULT 0,
  callback_evidence TINYINT(1) NOT NULL DEFAULT 0,
  target_total INT UNSIGNED NOT NULL DEFAULT 0,
  success_total INT UNSIGNED NOT NULL DEFAULT 0,
  failure_total INT UNSIGNED NOT NULL DEFAULT 0,
  error_code VARCHAR(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  actor_user_id INT UNSIGNED NULL,
  actor_source VARCHAR(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'user',
  request_id VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  lease_token VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  attempt INT UNSIGNED NOT NULL DEFAULT 0,
  requested_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  started_at DATETIME(6) NULL,
  finished_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_wecom_capability_operation_scope_id (tenant_id,corp_id,id),
  UNIQUE KEY uk_wecom_capability_operation_idempotency (tenant_id,corp_id,capability,action,credential_generation,idempotency_key),
  KEY idx_wecom_capability_operation_status (tenant_id,corp_id,status,updated_at),
  KEY idx_wecom_capability_operation_agent (tenant_id,corp_id,actual_agent_id),
  CONSTRAINT fk_wecom_capability_operation_corp FOREIGN KEY (tenant_id,corp_id) REFERENCES mc_corp (tenant_id,id) ON DELETE RESTRICT,
  CONSTRAINT fk_wecom_capability_operation_actor FOREIGN KEY (tenant_id,actor_user_id) REFERENCES mc_user (tenant_id,id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS mochat_go_wecom_capability_dispatches (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id INT UNSIGNED NOT NULL,
  corp_id INT UNSIGNED NOT NULL,
  operation_id BIGINT UNSIGNED NOT NULL,
  dispatch_kind VARCHAR(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  chunk_no INT UNSIGNED NOT NULL,
  target_id VARCHAR(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  idempotency_key VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  status VARCHAR(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  provider_request_id VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  provider_message_id VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  provider_object_id VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  credential_generation BIGINT UNSIGNED NOT NULL DEFAULT 0,
  lease_token VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  lease_expires_at DATETIME(6) NULL,
  attempt INT UNSIGNED NOT NULL DEFAULT 0,
  next_poll_at DATETIME(6) NULL,
  last_error_code VARCHAR(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_wecom_capability_dispatch_scope_id (tenant_id,corp_id,id),
  UNIQUE KEY uk_wecom_capability_dispatch_idempotency (tenant_id,corp_id,idempotency_key),
  UNIQUE KEY uk_wecom_capability_dispatch_target_chunk (tenant_id,corp_id,operation_id,dispatch_kind,target_id,chunk_no),
  KEY idx_wecom_capability_dispatch_claim (tenant_id,corp_id,status,next_poll_at),
  CONSTRAINT fk_wecom_capability_dispatch_operation FOREIGN KEY (tenant_id,corp_id,operation_id) REFERENCES mochat_go_wecom_capability_operations (tenant_id,corp_id,id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS mochat_go_wecom_capability_operation_results (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id INT UNSIGNED NOT NULL,
  corp_id INT UNSIGNED NOT NULL,
  operation_id BIGINT UNSIGNED NOT NULL,
  target_kind VARCHAR(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  target_id VARCHAR(255) COLLATE utf8mb4_unicode_ci NOT NULL,
  status VARCHAR(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  provider_target_id VARCHAR(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  error_code VARCHAR(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  error_message_safe VARCHAR(255) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uk_wecom_capability_result_scope_id (tenant_id,corp_id,id),
  UNIQUE KEY uk_wecom_capability_result_target (tenant_id,corp_id,operation_id,target_kind,target_id),
  KEY idx_wecom_capability_result_operation (tenant_id,corp_id,operation_id,updated_at),
  CONSTRAINT fk_wecom_capability_result_operation FOREIGN KEY (tenant_id,corp_id,operation_id) REFERENCES mochat_go_wecom_capability_operations (tenant_id,corp_id,id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS mochat_go_wecom_capability_operation_audits (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id INT UNSIGNED NOT NULL,
  corp_id INT UNSIGNED NOT NULL,
  operation_id BIGINT UNSIGNED NOT NULL,
  dispatch_id BIGINT UNSIGNED NULL,
  from_status VARCHAR(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  to_status VARCHAR(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  action VARCHAR(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  actor_user_id INT UNSIGNED NULL,
  actor_source VARCHAR(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  request_id VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  error_code VARCHAR(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  target_total INT UNSIGNED NOT NULL DEFAULT 0,
  success_total INT UNSIGNED NOT NULL DEFAULT 0,
  failure_total INT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  KEY idx_wecom_capability_audit_operation (tenant_id,corp_id,operation_id,created_at),
  CONSTRAINT fk_wecom_capability_audit_operation FOREIGN KEY (tenant_id,corp_id,operation_id) REFERENCES mochat_go_wecom_capability_operations (tenant_id,corp_id,id) ON DELETE CASCADE,
  CONSTRAINT fk_wecom_capability_audit_dispatch FOREIGN KEY (tenant_id,corp_id,dispatch_id) REFERENCES mochat_go_wecom_capability_dispatches (tenant_id,corp_id,id) ON DELETE CASCADE,
  CONSTRAINT fk_wecom_capability_audit_actor FOREIGN KEY (tenant_id,actor_user_id) REFERENCES mc_user (tenant_id,id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS mochat_go_wecom_capability_operation_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tenant_id INT UNSIGNED NOT NULL,
  corp_id INT UNSIGNED NOT NULL,
  operation_id BIGINT UNSIGNED NOT NULL,
  dispatch_id BIGINT UNSIGNED NULL,
  from_status VARCHAR(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  to_status VARCHAR(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  action VARCHAR(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  actor_user_id INT UNSIGNED NULL,
  actor_source VARCHAR(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  request_id VARCHAR(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  error_code VARCHAR(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  target_total INT UNSIGNED NOT NULL DEFAULT 0,
  success_total INT UNSIGNED NOT NULL DEFAULT 0,
  failure_total INT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  KEY idx_wecom_capability_event_operation (tenant_id,corp_id,operation_id,created_at),
  CONSTRAINT fk_wecom_capability_event_operation FOREIGN KEY (tenant_id,corp_id,operation_id) REFERENCES mochat_go_wecom_capability_operations (tenant_id,corp_id,id) ON DELETE CASCADE,
  CONSTRAINT fk_wecom_capability_event_dispatch FOREIGN KEY (tenant_id,corp_id,dispatch_id) REFERENCES mochat_go_wecom_capability_dispatches (tenant_id,corp_id,id) ON DELETE CASCADE,
  CONSTRAINT fk_wecom_capability_event_actor FOREIGN KEY (tenant_id,actor_user_id) REFERENCES mc_user (tenant_id,id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
