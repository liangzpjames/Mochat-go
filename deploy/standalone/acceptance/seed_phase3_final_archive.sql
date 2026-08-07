-- Phase 3 Final acceptance: seed conversation archive rows for AI insight.
-- Expects corp_id and employee_id from the default tenant-1 corp.
INSERT INTO mc_work_message_1
  (corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id, sender_type, action, type, msg_type, content, content_text, room_id, status, msg_data_time, created_at, updated_at, deleted_at)
SELECT c.id, 'P36-ARCH-001', 1, e.id, 1, 9001, 0, 0, 100, 100, '{"text":"客户咨询套餐价格"}', '客户咨询套餐价格和有效期', 0, 0, NOW(), NOW(), NOW(), NULL
FROM mc_corp c
LEFT JOIN mc_work_employee e ON e.corp_id = c.id AND e.log_user_id = 1 AND e.deleted_at IS NULL
WHERE c.tenant_id = 1 AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM mc_work_message_1 m WHERE m.corp_id = c.id AND m.msgid = 'P36-ARCH-001')
LIMIT 1;

INSERT INTO mc_work_message_1
  (corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id, sender_type, action, type, msg_type, content, content_text, room_id, status, msg_data_time, created_at, updated_at, deleted_at)
SELECT c.id, 'P36-ARCH-002', 2, e.id, 1, 9001, 0, 0, 100, 100, '{"text":"希望今天答复"}', '客户表示希望今天得到答复，对时效敏感', 0, 0, NOW(), NOW(), NOW(), NULL
FROM mc_corp c
LEFT JOIN mc_work_employee e ON e.corp_id = c.id AND e.log_user_id = 1 AND e.deleted_at IS NULL
WHERE c.tenant_id = 1 AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM mc_work_message_1 m WHERE m.corp_id = c.id AND m.msgid = 'P36-ARCH-002')
LIMIT 1;

INSERT INTO mc_work_message_1
  (corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id, sender_type, action, type, msg_type, content, content_text, room_id, status, msg_data_time, created_at, updated_at, deleted_at)
SELECT c.id, 'P36-ARCH-003', 3, e.id, 1, 9001, 0, 0, 100, 100, '{"text":"询问产品功能"}', '客户询问自动化标签与群发功能是否支持', 0, 0, NOW(), NOW(), NOW(), NULL
FROM mc_corp c
LEFT JOIN mc_work_employee e ON e.corp_id = c.id AND e.log_user_id = 1 AND e.deleted_at IS NULL
WHERE c.tenant_id = 1 AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM mc_work_message_1 m WHERE m.corp_id = c.id AND m.msgid = 'P36-ARCH-003')
LIMIT 1;

INSERT INTO mc_work_message_1
  (corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id, sender_type, action, type, msg_type, content, content_text, room_id, status, msg_data_time, created_at, updated_at, deleted_at)
SELECT c.id, 'P36-ARCH-004', 4, e.id, 1, 9001, 0, 0, 100, 100, '{"text":"感谢介绍"}', '客户感谢介绍并表示会考虑', 0, 0, NOW(), NOW(), NOW(), NULL
FROM mc_corp c
LEFT JOIN mc_work_employee e ON e.corp_id = c.id AND e.log_user_id = 1 AND e.deleted_at IS NULL
WHERE c.tenant_id = 1 AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM mc_work_message_1 m WHERE m.corp_id = c.id AND m.msgid = 'P36-ARCH-004')
LIMIT 1;

INSERT INTO mc_work_message_1
  (corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id, sender_type, action, type, msg_type, content, content_text, room_id, status, msg_data_time, created_at, updated_at, deleted_at)
SELECT c.id, 'P36-ARCH-005', 5, e.id, 1, 9001, 0, 0, 100, 100, '{"text":"价格偏高"}', '客户反馈价格偏高，希望了解折扣方案', 0, 0, NOW(), NOW(), NOW(), NULL
FROM mc_corp c
LEFT JOIN mc_work_employee e ON e.corp_id = c.id AND e.log_user_id = 1 AND e.deleted_at IS NULL
WHERE c.tenant_id = 1 AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM mc_work_message_1 m WHERE m.corp_id = c.id AND m.msgid = 'P36-ARCH-005')
LIMIT 1;
