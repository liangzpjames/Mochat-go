-- 0139 rollback preflight
-- A rollback only removes objects whose 0139 signatures are still complete.

SET @wecom_0139_down_signature_invalid := (
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mochat_go_wecom_capability_operations','mochat_go_wecom_capability_dispatches','mochat_go_wecom_capability_operation_results','mochat_go_wecom_capability_operation_audits','mochat_go_wecom_capability_operation_events') AND column_name = 'id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND column_default IS NULL AND LOWER(extra) LIKE '%auto_increment%'))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mochat_go_wecom_capability_operations','mochat_go_wecom_capability_dispatches','mochat_go_wecom_capability_operation_results') AND column_name = 'updated_at' AND NOT (data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO' AND LOWER(COALESCE(column_default,'')) = 'current_timestamp(6)' AND REPLACE(LOWER(REPLACE(COALESCE(extra,''), ' ', '')), 'default_generated', '') LIKE '%onupdatecurrent_timestamp(6)%'))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mochat_go_wecom_capability_operation_audits','mochat_go_wecom_capability_operation_events') AND column_name = 'created_at' AND NOT (data_type = 'datetime' AND datetime_precision = 6 AND is_nullable = 'NO' AND LOWER(COALESCE(column_default,'')) = 'current_timestamp(6)'))
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_scope_id') <> 3
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_scope_id'), '') <> 'tenant_id,corp_id,id'
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_idempotency') <> 6
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_idempotency'), '') <> 'tenant_id,corp_id,capability,action,credential_generation,idempotency_key'
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_status') <> 4
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_status'), '') <> 'tenant_id,corp_id,status,updated_at'
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_agent') <> 3
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_agent'), '') <> 'tenant_id,corp_id,actual_agent_id'
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_scope_id') <> 3
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_idempotency') <> 3
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_target_chunk') <> 6
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'idx_wecom_capability_dispatch_claim') <> 4
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_scope_id') <> 3
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_target') <> 5
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'idx_wecom_capability_result_operation') <> 4
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND index_name = 'idx_wecom_capability_audit_operation') <> 4
  OR (SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events') = 1 AND (SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND index_name = 'idx_wecom_capability_event_operation') <> 4
  OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name IN ('mochat_go_wecom_capability_dispatches','mochat_go_wecom_capability_operation_results','mochat_go_wecom_capability_operation_audits','mochat_go_wecom_capability_operation_events') AND delete_rule <> 'RESTRICT')
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND (
    (column_name IN ('credential_generation','target_total','success_total','failure_total','attempt') AND NOT (column_default = '0' OR column_default = CONCAT(CHAR(39),'0',CHAR(39))))
    OR (column_name IN ('provider_request_id','provider_object_id','actual_agent_id','error_code','request_id','lease_token') AND NOT (column_default = '' OR column_default = CONCAT(CHAR(39),CHAR(39)) OR column_default = CONCAT(CHAR(34),CHAR(34))))
    OR (column_name = 'actor_source' AND NOT (LOWER(column_default) = 'user' OR LOWER(column_default) = CONCAT(CHAR(39),'user',CHAR(39))))
    OR (column_name IN ('external_success','callback_evidence') AND NOT (column_default = '0' OR column_default = CONCAT(CHAR(39),'0',CHAR(39))))
    OR (column_name IN ('requested_at','created_at') AND LOWER(COALESCE(column_default,'')) <> 'current_timestamp(6)')
  ))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND (
    (column_name IN ('credential_generation','attempt') AND NOT (column_default = '0' OR column_default = CONCAT(CHAR(39),'0',CHAR(39))))
    OR (column_name = 'chunk_no' AND NOT (column_default IS NULL OR UPPER(TRIM(column_default)) = 'NULL'))
    OR (column_name IN ('provider_request_id','provider_message_id','provider_object_id','last_error_code','lease_token') AND NOT (column_default = '' OR column_default = CONCAT(CHAR(39),CHAR(39)) OR column_default = CONCAT(CHAR(34),CHAR(34))))
    OR (column_name = 'created_at' AND LOWER(COALESCE(column_default,'')) <> 'current_timestamp(6)')
  ))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND (
    (column_name IN ('provider_target_id','error_code','error_message_safe') AND NOT (column_default = '' OR column_default = CONCAT(CHAR(39),CHAR(39)) OR column_default = CONCAT(CHAR(34),CHAR(34))))
    OR (column_name = 'created_at' AND LOWER(COALESCE(column_default,'')) <> 'current_timestamp(6)')
  ))
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mochat_go_wecom_capability_operation_audits','mochat_go_wecom_capability_operation_events') AND (
    (column_name IN ('from_status','request_id','error_code') AND NOT (column_default = '' OR column_default = CONCAT(CHAR(39),CHAR(39)) OR column_default = CONCAT(CHAR(34),CHAR(34))))
    OR (column_name IN ('target_total','success_total','failure_total') AND NOT (column_default = '0' OR column_default = CONCAT(CHAR(39),'0',CHAR(39))))
    OR (column_name = 'created_at' AND LOWER(COALESCE(column_default,'')) <> 'current_timestamp(6)')
  ))
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND constraint_name = 'fk_wecom_capability_dispatch_operation'), '') <> 'tenant_id=mochat_go_wecom_capability_operations.tenant_id,corp_id=mochat_go_wecom_capability_operations.corp_id,operation_id=mochat_go_wecom_capability_operations.id')
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND constraint_name = 'fk_wecom_capability_operation_corp'), '') <> 'tenant_id=mc_corp.tenant_id,corp_id=mc_corp.id')
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND constraint_name = 'fk_wecom_capability_operation_actor'), '') <> 'tenant_id=mc_user.tenant_id,actor_user_id=mc_user.id')
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND constraint_name = 'fk_wecom_capability_result_operation'), '') <> 'tenant_id=mochat_go_wecom_capability_operations.tenant_id,corp_id=mochat_go_wecom_capability_operations.corp_id,operation_id=mochat_go_wecom_capability_operations.id')
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND constraint_name = 'fk_wecom_capability_audit_operation'), '') <> 'tenant_id=mochat_go_wecom_capability_operations.tenant_id,corp_id=mochat_go_wecom_capability_operations.corp_id,operation_id=mochat_go_wecom_capability_operations.id')
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND constraint_name = 'fk_wecom_capability_audit_dispatch'), '') <> 'tenant_id=mochat_go_wecom_capability_dispatches.tenant_id,corp_id=mochat_go_wecom_capability_dispatches.corp_id,dispatch_id=mochat_go_wecom_capability_dispatches.id')
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND constraint_name = 'fk_wecom_capability_audit_actor'), '') <> 'tenant_id=mc_user.tenant_id,actor_user_id=mc_user.id')
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND constraint_name = 'fk_wecom_capability_event_operation'), '') <> 'tenant_id=mochat_go_wecom_capability_operations.tenant_id,corp_id=mochat_go_wecom_capability_operations.corp_id,operation_id=mochat_go_wecom_capability_operations.id')
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND constraint_name = 'fk_wecom_capability_event_dispatch'), '') <> 'tenant_id=mochat_go_wecom_capability_dispatches.tenant_id,corp_id=mochat_go_wecom_capability_dispatches.corp_id,dispatch_id=mochat_go_wecom_capability_dispatches.id')
  OR ((SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events') = 1 AND COALESCE((SELECT GROUP_CONCAT(CONCAT(column_name,'=',referenced_table_name,'.',referenced_column_name) ORDER BY ordinal_position SEPARATOR ',') FROM information_schema.key_column_usage WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND constraint_name = 'fk_wecom_capability_event_actor'), '') <> 'tenant_id=mc_user.tenant_id,actor_user_id=mc_user.id')
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND index_name IN ('uk_wecom_capability_operation_scope_id','uk_wecom_capability_operation_idempotency','uk_wecom_capability_dispatch_scope_id','uk_wecom_capability_dispatch_idempotency','uk_wecom_capability_dispatch_target_chunk','uk_wecom_capability_result_scope_id','uk_wecom_capability_result_target') AND non_unique <> 0)
);

SET @wecom_0139_down_external_fk_invalid := EXISTS (
  SELECT 1
  FROM information_schema.referential_constraints
  WHERE constraint_schema = DATABASE()
    AND referenced_table_name IN (
      'mochat_go_wecom_capability_operations',
      'mochat_go_wecom_capability_dispatches',
      'mochat_go_wecom_capability_operation_results',
      'mochat_go_wecom_capability_operation_audits',
      'mochat_go_wecom_capability_operation_events'
    )
    AND table_name NOT IN (
      'mochat_go_wecom_capability_operations',
      'mochat_go_wecom_capability_dispatches',
      'mochat_go_wecom_capability_operation_results',
      'mochat_go_wecom_capability_operation_audits',
      'mochat_go_wecom_capability_operation_events'
    )
);

SET @wecom_0139_down_index_invalid := (
  EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND (
    COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'PRIMARY'), '') <> 'id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'PRIMARY'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_scope_id'), '') <> 'tenant_id,corp_id,id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_scope_id'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_idempotency'), '') <> 'tenant_id,corp_id,capability,action,credential_generation,idempotency_key'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'uk_wecom_capability_operation_idempotency'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_status'), '') <> 'tenant_id,corp_id,status,updated_at'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_status'), -1) <> 1
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_agent'), '') <> 'tenant_id,corp_id,actual_agent_id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'idx_wecom_capability_operation_agent'), -1) <> 1
  ))
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND (
    COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'PRIMARY'), '') <> 'id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'PRIMARY'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_scope_id'), '') <> 'tenant_id,corp_id,id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_scope_id'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_idempotency'), '') <> 'tenant_id,corp_id,idempotency_key'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_idempotency'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_target_chunk'), '') <> 'tenant_id,corp_id,operation_id,dispatch_kind,target_id,chunk_no'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'uk_wecom_capability_dispatch_target_chunk'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'idx_wecom_capability_dispatch_claim'), '') <> 'tenant_id,corp_id,status,next_poll_at'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND index_name = 'idx_wecom_capability_dispatch_claim'), -1) <> 1
  ))
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND (
    COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'PRIMARY'), '') <> 'id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'PRIMARY'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_scope_id'), '') <> 'tenant_id,corp_id,id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_scope_id'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_target'), '') <> 'tenant_id,corp_id,operation_id,target_kind,target_id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'uk_wecom_capability_result_target'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'idx_wecom_capability_result_operation'), '') <> 'tenant_id,corp_id,operation_id,updated_at'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND index_name = 'idx_wecom_capability_result_operation'), -1) <> 1
  ))
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND (
    COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND index_name = 'PRIMARY'), '') <> 'id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND index_name = 'PRIMARY'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND index_name = 'idx_wecom_capability_audit_operation'), '') <> 'tenant_id,corp_id,operation_id,created_at'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND index_name = 'idx_wecom_capability_audit_operation'), -1) <> 1
  ))
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND (
    COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND index_name = 'PRIMARY'), '') <> 'id'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND index_name = 'PRIMARY'), -1) <> 0
    OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND index_name = 'idx_wecom_capability_event_operation'), '') <> 'tenant_id,corp_id,operation_id,created_at'
    OR COALESCE((SELECT MAX(non_unique) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND index_name = 'idx_wecom_capability_event_operation'), -1) <> 1
  ))
);

SET @wecom_0139_down_invalid := (
  EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations') <> 29 OR COALESCE((SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND index_name = 'PRIMARY'), '') <> 'id' OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND column_name IN ('id','tenant_id','corp_id','capability','action','credential_group','credential_generation','idempotency_key','status','provider_request_id','provider_object_id','actual_agent_id','external_success','callback_evidence','target_total','success_total','failure_total','error_code','actor_user_id','actor_source','request_id','lease_token','lease_expires_at','attempt','requested_at','started_at','finished_at','created_at','updated_at')) <> 29))
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches') <> 20 OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND column_name IN ('id','tenant_id','corp_id','operation_id','dispatch_kind','chunk_no','target_id','idempotency_key','status','provider_request_id','provider_message_id','provider_object_id','credential_generation','lease_token','lease_expires_at','attempt','next_poll_at','last_error_code','created_at','updated_at')) <> 20))
  OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND constraint_name = 'fk_wecom_capability_dispatch_operation' AND delete_rule <> 'RESTRICT')
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results') <> 12 OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND column_name IN ('id','tenant_id','corp_id','operation_id','target_kind','target_id','status','provider_target_id','error_code','error_message_safe','created_at','updated_at')) <> 12))
  OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND constraint_name = 'fk_wecom_capability_result_operation' AND delete_rule <> 'RESTRICT')
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits') <> 16 OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND column_name IN ('id','tenant_id','corp_id','operation_id','dispatch_id','from_status','to_status','action','actor_user_id','actor_source','request_id','error_code','target_total','success_total','failure_total','created_at')) <> 16))
  OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND constraint_name IN ('fk_wecom_capability_audit_operation','fk_wecom_capability_audit_dispatch','fk_wecom_capability_audit_actor') AND delete_rule <> 'RESTRICT')
  OR EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND ((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events') <> 16 OR (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND column_name IN ('id','tenant_id','corp_id','operation_id','dispatch_id','from_status','to_status','action','actor_user_id','actor_source','request_id','error_code','target_total','success_total','failure_total','created_at')) <> 16))
  OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND constraint_name IN ('fk_wecom_capability_event_operation','fk_wecom_capability_event_dispatch','fk_wecom_capability_event_actor') AND delete_rule <> 'RESTRICT')
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
  OR EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mc_contact_message_batch_send','mc_room_message_batch_send') AND column_name = 'tenant_id' AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND (column_default IS NULL OR UPPER(TRIM(column_default)) = 'NULL' OR column_default = '0' OR column_default = '''0''')))
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_contact_message_batch_send' AND column_name = 'tenant_id' AND index_name NOT IN ('uk_wecom_contact_batch_scope'))
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'mc_room_message_batch_send' AND column_name = 'tenant_id' AND index_name NOT IN ('uk_wecom_room_batch_scope'))
  OR EXISTS (SELECT 1 FROM information_schema.key_column_usage k JOIN information_schema.table_constraints tc ON tc.constraint_schema = k.constraint_schema AND tc.table_name = k.table_name AND tc.constraint_name = k.constraint_name WHERE k.constraint_schema = DATABASE() AND k.table_name = 'mc_contact_message_batch_send' AND k.column_name = 'tenant_id' AND tc.constraint_type = 'FOREIGN KEY' AND k.constraint_name <> 'fk_wecom_contact_batch_corp')
  OR EXISTS (SELECT 1 FROM information_schema.key_column_usage k JOIN information_schema.table_constraints tc ON tc.constraint_schema = k.constraint_schema AND tc.table_name = k.table_name AND tc.constraint_name = k.constraint_name WHERE k.constraint_schema = DATABASE() AND k.table_name = 'mc_room_message_batch_send' AND k.column_name = 'tenant_id' AND tc.constraint_type = 'FOREIGN KEY' AND k.constraint_name <> 'fk_wecom_room_batch_corp')
  OR @wecom_0139_down_external_fk_invalid
  OR @wecom_0139_down_index_invalid
  OR @wecom_0139_down_signature_invalid
);
SET @wecom_0139_down_parent_residual := (
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name IN ('mc_contact_message_batch_send','mc_room_message_batch_send') AND column_name = 'tenant_id' AND NOT (data_type = 'int' AND numeric_precision = 10 AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND (column_default IS NULL OR UPPER(TRIM(column_default)) = 'NULL' OR column_default = '0' OR column_default = '''0''')))
  OR EXISTS (SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name IN ('mc_contact_message_batch_send','mc_room_message_batch_send') AND index_name IN ('uk_wecom_contact_batch_scope','uk_wecom_room_batch_scope') AND (non_unique <> 0 OR sub_part IS NOT NULL))
  OR EXISTS (SELECT 1 FROM information_schema.key_column_usage k JOIN information_schema.table_constraints tc ON tc.constraint_schema = k.constraint_schema AND tc.table_name = k.table_name AND tc.constraint_name = k.constraint_name WHERE k.constraint_schema = DATABASE() AND k.table_name IN ('mc_contact_message_batch_send','mc_room_message_batch_send') AND k.column_name = 'tenant_id' AND tc.constraint_type = 'FOREIGN KEY' AND k.constraint_name NOT IN ('fk_wecom_contact_batch_corp','fk_wecom_room_batch_corp'))
);
SET @wecom_0139_down_operations_residual := EXISTS (
  SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations'
  AND (
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND column_name = 'id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND (column_default IS NULL OR UPPER(TRIM(column_default)) = 'NULL') AND LOWER(extra) LIKE '%auto_increment%'))
    OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operations' AND constraint_name IN ('fk_wecom_capability_operation_corp','fk_wecom_capability_operation_actor') AND delete_rule <> 'RESTRICT')
  )
);
SET @wecom_0139_down_dispatch_residual := EXISTS (
  SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches'
  AND (
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND column_name = 'id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND (column_default IS NULL OR UPPER(TRIM(column_default)) = 'NULL') AND LOWER(extra) LIKE '%auto_increment%'))
    OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_dispatches' AND constraint_name = 'fk_wecom_capability_dispatch_operation' AND delete_rule <> 'RESTRICT')
  )
);
SET @wecom_0139_down_result_residual := EXISTS (
  SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results'
  AND (
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND column_name = 'id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND (column_default IS NULL OR UPPER(TRIM(column_default)) = 'NULL') AND LOWER(extra) LIKE '%auto_increment%'))
    OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_results' AND constraint_name = 'fk_wecom_capability_result_operation' AND delete_rule <> 'RESTRICT')
  )
);
SET @wecom_0139_down_audit_residual := EXISTS (
  SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits'
  AND (
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND column_name = 'id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND (column_default IS NULL OR UPPER(TRIM(column_default)) = 'NULL') AND LOWER(extra) LIKE '%auto_increment%'))
    OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_audits' AND constraint_name IN ('fk_wecom_capability_audit_operation','fk_wecom_capability_audit_dispatch','fk_wecom_capability_audit_actor') AND delete_rule <> 'RESTRICT')
  )
);
SET @wecom_0139_down_event_residual := EXISTS (
  SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events'
  AND (
  EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND column_name = 'id' AND NOT (data_type = 'bigint' AND column_type LIKE '%unsigned%' AND is_nullable = 'NO' AND (column_default IS NULL OR UPPER(TRIM(column_default)) = 'NULL') AND LOWER(extra) LIKE '%auto_increment%'))
    OR EXISTS (SELECT 1 FROM information_schema.referential_constraints WHERE constraint_schema = DATABASE() AND table_name = 'mochat_go_wecom_capability_operation_events' AND constraint_name IN ('fk_wecom_capability_event_operation','fk_wecom_capability_event_dispatch','fk_wecom_capability_event_actor') AND delete_rule <> 'RESTRICT')
  )
);
SET @wecom_0139_down_reason := CASE
  WHEN @wecom_0139_down_external_fk_invalid THEN 'external_fk'
  WHEN @wecom_0139_down_parent_residual THEN 'parent'
  WHEN @wecom_0139_down_operations_residual THEN 'operations'
  WHEN @wecom_0139_down_dispatch_residual THEN 'dispatches'
  WHEN @wecom_0139_down_result_residual THEN 'results'
  WHEN @wecom_0139_down_audit_residual THEN 'audits'
  WHEN @wecom_0139_down_event_residual THEN 'events'
  ELSE 'unclassified'
END;
SET @wecom_0139_down_guard_sql := CASE
  WHEN @wecom_0139_down_external_fk_invalid THEN 'SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 rollback blocked by external foreign key'''
  WHEN @wecom_0139_down_invalid = 0 THEN 'SELECT 1'
  ELSE CONCAT('SIGNAL SQLSTATE ''45000'' SET MESSAGE_TEXT = ''0139 incompatible rollback residual: ', @wecom_0139_down_reason, '''')
END;
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
