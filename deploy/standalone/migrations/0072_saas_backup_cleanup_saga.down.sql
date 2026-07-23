DELETE FROM `mochat_go_saas_admin_approval_policies`
WHERE `action_type` = 'backup.retention.cleanup';

UPDATE `mochat_go_saas_backup_runs`
SET `cleanup_run_id` = NULL
WHERE `cleanup_run_id` IS NOT NULL;

DROP TABLE IF EXISTS `mochat_go_saas_backup_cleanup_items`;
DROP TABLE IF EXISTS `mochat_go_saas_backup_cleanup_runs`;

ALTER TABLE `mochat_go_saas_backup_runs`
  DROP KEY `idx_mochat_go_saas_backup_cleanup`,
  DROP COLUMN `cleanup_run_id`;
