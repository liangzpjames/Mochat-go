CREATE TABLE IF NOT EXISTS mochat_go_phase35_acceptance_resources (
  id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, corp_id BIGINT NOT NULL,
  environment_id VARCHAR(96) NOT NULL, resource_type VARCHAR(24) NOT NULL, resource_id VARCHAR(36) NOT NULL,
  created_by BIGINT NOT NULL, created_at DATETIME(6) NOT NULL, deleted_by BIGINT NULL, deleted_at DATETIME(6) NULL,
  PRIMARY KEY (id), UNIQUE KEY uk_phase35_acceptance_resource (tenant_id,corp_id,environment_id,resource_type,resource_id),
  KEY idx_phase35_acceptance_scope (tenant_id,corp_id,environment_id,deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS mochat_go_phase35_acceptance_audit (
  id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, corp_id BIGINT NOT NULL,
  environment_id VARCHAR(96) NOT NULL, action VARCHAR(24) NOT NULL, resource_count INT NOT NULL,
  actor_id BIGINT NOT NULL, created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id), KEY idx_phase35_acceptance_audit (tenant_id,corp_id,environment_id,created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
