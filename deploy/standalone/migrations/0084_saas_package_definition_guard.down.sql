DELETE FROM `mochat_go_saas_admin_approval_policies`
WHERE `action_type` = 'package.upsert';

ALTER TABLE `mochat_go_saas_packages`
  DROP COLUMN `version`;
