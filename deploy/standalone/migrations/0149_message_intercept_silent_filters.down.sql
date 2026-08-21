ALTER TABLE mochat_go_silent_customer_records
  DROP KEY idx_silent_record_assignee;

ALTER TABLE mochat_go_message_intercept_records
  DROP KEY idx_intercept_record_filters;

UPDATE mochat_go_message_intercept_rules
SET conversation_scopes_json = REPLACE(REPLACE(conversation_scopes_json, '"single"', '"customer"'), '"group"', '"room"')
WHERE conversation_scopes_json LIKE '%"single"%' OR conversation_scopes_json LIKE '%"group"%';
