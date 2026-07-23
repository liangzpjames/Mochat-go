ALTER TABLE `mochat_go_saas_compliance_policies`
  ADD COLUMN `service_account_usage_retention_days` int(10) unsigned NOT NULL DEFAULT '90' COMMENT '服务账号 OpenAPI 日用量保留天数' AFTER `audit_retention_days`;
