-- Reconcile default Dashboard corp and employee bindings for databases that
-- already applied 0099 before tenant 1 was included. This migration is
-- intentionally additive and idempotent: existing corps/employees are never
-- modified or deleted, and every write is scoped by tenant_id.

INSERT INTO `mc_corp` (
  `name`, `wx_corpid`, `social_code`, `employee_secret`, `event_callback`,
  `contact_secret`, `token`, `encoding_aes_key`, `tenant_id`, `created_at`,
  `updated_at`, `deleted_at`
)
SELECT
  CONCAT(TRIM(t.`name`), '演示企业'),
  CONCAT('fake_tenant_', t.`id`),
  '', '', '', '', '', '', t.`id`, NOW(), NOW(), NULL
FROM `mc_tenant` t
WHERE t.`status` = 1
  AND t.`deleted_at` IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM `mc_corp` c
    WHERE c.`tenant_id` = t.`id` AND c.`deleted_at` IS NULL
  );

INSERT INTO `mc_work_employee` (
  `wx_user_id`, `corp_id`, `name`, `mobile`, `status`, `log_user_id`,
  `audit_status`, `created_at`, `updated_at`, `deleted_at`
)
SELECT
  CONCAT('bootstrap_', u.`id`), c.`id`, u.`name`, u.`phone`, 1, u.`id`,
  1, NOW(), NOW(), NULL
FROM `mc_user` u
JOIN `mc_corp` c ON c.`tenant_id` = u.`tenant_id` AND c.`deleted_at` IS NULL
WHERE u.`status` = 1
  AND u.`isSuperAdmin` = 1
  AND u.`deleted_at` IS NULL
  AND c.`id` = (
    SELECT MIN(c2.`id`)
    FROM `mc_corp` c2
    WHERE c2.`tenant_id` = u.`tenant_id` AND c2.`deleted_at` IS NULL
  )
  AND NOT EXISTS (
    SELECT 1 FROM `mc_work_employee` e
    WHERE e.`corp_id` = c.`id`
      AND e.`log_user_id` = u.`id`
      AND e.`deleted_at` IS NULL
  );
