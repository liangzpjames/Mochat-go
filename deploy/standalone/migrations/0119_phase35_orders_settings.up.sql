CREATE TABLE IF NOT EXISTS mochat_go_scrm_orders (
  id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, corp_id BIGINT NOT NULL,
  contact_id VARCHAR(36) NOT NULL, opportunity_id VARCHAR(36) NULL,
  amount_cents BIGINT NOT NULL, currency CHAR(3) NOT NULL DEFAULT 'CNY',
  status VARCHAR(20) NOT NULL, version BIGINT NOT NULL DEFAULT 1,
  idempotency_key VARCHAR(128) NOT NULL, created_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL, deleted_at DATETIME(6) NULL,
  PRIMARY KEY (id), UNIQUE KEY uk_scrm_order_idempotency (tenant_id, corp_id, idempotency_key),
  KEY idx_scrm_order_scope (tenant_id, corp_id, updated_at), KEY idx_scrm_order_contact (tenant_id, corp_id, contact_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS mochat_go_scrm_order_audit (
  id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, corp_id BIGINT NOT NULL, order_id VARCHAR(36) NOT NULL,
  action VARCHAR(32) NOT NULL, actor_id BIGINT NOT NULL, from_version BIGINT NOT NULL, to_version BIGINT NOT NULL,
  payload_json JSON NOT NULL, created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id), KEY idx_scrm_order_audit (tenant_id, corp_id, order_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS mochat_go_scrm_settings (
  id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, corp_id BIGINT NOT NULL,
  setting_type VARCHAR(32) NOT NULL, setting_key VARCHAR(64) NOT NULL, label VARCHAR(128) NOT NULL,
  value_json JSON NOT NULL, enabled TINYINT(1) NOT NULL DEFAULT 1, version BIGINT NOT NULL DEFAULT 1,
  created_by BIGINT NOT NULL, updated_by BIGINT NOT NULL, created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL, deleted_at DATETIME(6) NULL,
  PRIMARY KEY (id), UNIQUE KEY uk_scrm_setting_key (tenant_id, corp_id, setting_type, setting_key),
  KEY idx_scrm_setting_scope (tenant_id, corp_id, setting_type, enabled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
