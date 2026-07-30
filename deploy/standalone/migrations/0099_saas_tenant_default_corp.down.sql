-- 企业记录可能已产生业务数据，回滚时不删除；仅还原本迁移提升的标准版额度。

UPDATE `mochat_go_saas_packages`
SET `max_corps` = 0,
    `updated_at` = NOW()
WHERE `code` = 'standard'
  AND `max_corps` = 1
  AND `deleted_at` IS NULL;

UPDATE `mochat_go_saas_tenant_packages`
SET `limits_json` = JSON_SET(
      COALESCE(`limits_json`, JSON_OBJECT()),
      '$.maxCorps',
      0
    ),
    `updated_at` = NOW()
WHERE `package_code` = 'standard'
  AND `deleted_at` IS NULL
  AND CAST(JSON_UNQUOTE(JSON_EXTRACT(`limits_json`, '$.maxCorps')) AS UNSIGNED) = 1;
