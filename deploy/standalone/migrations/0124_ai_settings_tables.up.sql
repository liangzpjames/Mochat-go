CREATE TABLE IF NOT EXISTS mochat_go_ai_knowledge_bases (
  id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, corp_id BIGINT NOT NULL,
  name VARCHAR(128) NOT NULL, description VARCHAR(512) NOT NULL DEFAULT '',
  document_count INT NOT NULL DEFAULT 0, status TINYINT(1) NOT NULL DEFAULT 1,
  created_by BIGINT NOT NULL, updated_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL, deleted_at DATETIME(6) NULL,
  PRIMARY KEY (id), KEY idx_ai_kb_scope (tenant_id, corp_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS mochat_go_ai_agents (
  id VARCHAR(36) NOT NULL, tenant_id BIGINT NOT NULL, corp_id BIGINT NOT NULL,
  name VARCHAR(128) NOT NULL, description VARCHAR(512) NOT NULL DEFAULT '',
  knowledge_base_ids JSON NOT NULL, status TINYINT(1) NOT NULL DEFAULT 1,
  created_by BIGINT NOT NULL, updated_by BIGINT NOT NULL,
  created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL, deleted_at DATETIME(6) NULL,
  PRIMARY KEY (id), KEY idx_ai_agent_scope (tenant_id, corp_id, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
