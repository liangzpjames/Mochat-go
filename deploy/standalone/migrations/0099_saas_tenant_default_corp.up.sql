-- 确保标准版客户至少可创建一个企业，并为已有无企业租户补齐演示企业。

UPDATE `mochat_go_saas_packages`
SET `max_corps` = 1,
    `updated_at` = NOW()
WHERE `code` = 'standard'
  AND `max_corps` < 1
  AND `deleted_at` IS NULL;

UPDATE `mochat_go_saas_tenant_packages`
SET `limits_json` = JSON_SET(
      COALESCE(`limits_json`, JSON_OBJECT()),
      '$.maxCorps',
      1
    ),
    `updated_at` = NOW()
WHERE `package_code` = 'standard'
  AND `deleted_at` IS NULL
  AND COALESCE(
        CAST(JSON_UNQUOTE(JSON_EXTRACT(`limits_json`, '$.maxCorps')) AS UNSIGNED),
        0
      ) < 1;

INSERT INTO `mc_corp` (
  `name`,
  `wx_corpid`,
  `social_code`,
  `employee_secret`,
  `event_callback`,
  `contact_secret`,
  `token`,
  `encoding_aes_key`,
  `tenant_id`,
  `created_at`,
  `updated_at`,
  `deleted_at`
)
SELECT
  CONCAT(TRIM(t.`name`), '演示企业'),
  CONCAT('fake_tenant_', t.`id`),
  '',
  '',
  '',
  '',
  '',
  '',
  t.`id`,
  NOW(),
  NOW(),
  NULL
FROM `mc_tenant` t
WHERE t.`id` <> 1
  AND t.`status` = 1
  AND t.`deleted_at` IS NULL
  AND NOT EXISTS (
    SELECT 1
    FROM `mc_corp` c
    WHERE c.`tenant_id` = t.`id`
      AND c.`deleted_at` IS NULL
  );
