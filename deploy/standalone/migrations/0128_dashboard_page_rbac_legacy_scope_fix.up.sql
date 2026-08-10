-- Correct the 0127 legacy role scope backfill without rewriting its applied checksum.
-- Legacy semantics:
--   menu.data_permission = 2 -> tenant scope (scope control disabled)
--   menu.data_permission = 1 + role permissionType = 1 -> department scope
--   menu.data_permission = 1 + role permissionType = 2 -> self scope
--   menu.data_permission = 1 + no corp override -> department scope
-- The new model has one scope per tenant role/page. Mixed corp-specific legacy
-- scopes therefore use the most restrictive value and emit an admin-review audit.

UPDATE `mochat_go_dashboard_role_permissions` rp
INNER JOIN (
  SELECT
    r.`tenant_id`,
    r.`id` AS `role_id`,
    p.`id` AS `permission_id`,
    CASE MIN(
      CASE
        WHEN m.`data_permission` = 2 THEN 3
        WHEN COALESCE(CAST(r.`data_permission` AS CHAR), '') REGEXP '"(permissionType|permission_type)"[[:space:]]*:[[:space:]]*2[[:space:]]*[,}]' THEN 1
        ELSE 2
      END
    )
      WHEN 1 THEN 'self'
      WHEN 2 THEN 'department'
      ELSE 'tenant'
    END AS `corrected_scope`
  FROM `mc_rbac_role_menu` rm
  INNER JOIN `mc_rbac_role` r ON r.`id` = rm.`role_id` AND r.`deleted_at` IS NULL
  INNER JOIN `mc_rbac_menu` m ON m.`id` = rm.`menu_id` AND m.`deleted_at` IS NULL
  INNER JOIN `mochat_go_dashboard_permission_resources` pr
    ON pr.`path_pattern` = SUBSTRING_INDEX(SUBSTRING_INDEX(m.`link_url`, '@', 1), '#', 1)
   AND pr.`status` = 1
   AND pr.`deleted_at` IS NULL
  INNER JOIN `mochat_go_dashboard_permissions` p
    ON p.`id` = pr.`permission_id`
   AND p.`superadmin_only` = 0
  WHERE m.`link_url` <> ''
  GROUP BY r.`tenant_id`, r.`id`, p.`id`
) legacy_scope
  ON legacy_scope.`tenant_id` = rp.`tenant_id`
 AND legacy_scope.`role_id` = rp.`role_id`
 AND legacy_scope.`permission_id` = rp.`permission_id`
SET rp.`data_scope` = legacy_scope.`corrected_scope`,
    rp.`updated_at` = NOW();

INSERT INTO `mochat_go_dashboard_permission_audits`
  (`tenant_id`, `actor_user_id`, `action`, `target_type`, `target_id`, `before_json`, `after_json`, `request_id`, `created_at`)
SELECT
  r.`tenant_id`,
  NULL,
  'migration.legacy_scope_review',
  'role',
  CAST(r.`id` AS CHAR),
  JSON_OBJECT('legacyDataPermission', r.`data_permission`),
  JSON_OBJECT(
    'migration', '0128_dashboard_page_rbac_legacy_scope_fix',
    'requiresAdminReview', TRUE,
    'fallbackScope', 'self',
    'reason', 'legacy role contains corp-specific department and self scopes'
  ),
  CONCAT('migration:0128:role:', r.`tenant_id`, ':', r.`id`),
  NOW()
FROM `mc_rbac_role` r
WHERE r.`deleted_at` IS NULL
  AND COALESCE(CAST(r.`data_permission` AS CHAR), '') REGEXP '"(permissionType|permission_type)"[[:space:]]*:[[:space:]]*1[[:space:]]*[,}]'
  AND COALESCE(CAST(r.`data_permission` AS CHAR), '') REGEXP '"(permissionType|permission_type)"[[:space:]]*:[[:space:]]*2[[:space:]]*[,}]'
  AND EXISTS (
    SELECT 1
    FROM `mc_rbac_role_menu` rm
    WHERE rm.`role_id` = r.`id`
  )
  AND NOT EXISTS (
    SELECT 1
    FROM `mochat_go_dashboard_permission_audits` a
    WHERE a.`tenant_id` = r.`tenant_id`
      AND a.`request_id` = CONCAT('migration:0128:role:', r.`tenant_id`, ':', r.`id`)
  );
