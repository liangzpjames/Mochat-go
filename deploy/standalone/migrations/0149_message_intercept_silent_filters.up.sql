-- Normalize legacy demo/provider scope values to the dashboard contract.
UPDATE mochat_go_message_intercept_rules
SET conversation_scopes_json = REPLACE(REPLACE(conversation_scopes_json, '"customer"', '"single"'), '"room"', '"group"')
WHERE conversation_scopes_json LIKE '%"customer"%' OR conversation_scopes_json LIKE '%"room"%';

ALTER TABLE mochat_go_message_intercept_records
  ADD KEY idx_intercept_record_filters (tenant_id, corp_id, conversation_type, decision, audit_status, occurred_at);

ALTER TABLE mochat_go_silent_customer_records
  ADD KEY idx_silent_record_assignee (tenant_id, corp_id, assigned_employee_id, status, updated_at);
