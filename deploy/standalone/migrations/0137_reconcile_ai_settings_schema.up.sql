-- Reconcile AI settings tables after historical 0124 ledger/DDL drift.

CREATE TABLE IF NOT EXISTS `mochat_go_ai_knowledge_bases` (
  `id` varchar(36) NOT NULL,
  `tenant_id` bigint NOT NULL,
  `corp_id` bigint NOT NULL,
  `name` varchar(128) NOT NULL,
  `description` varchar(512) NOT NULL DEFAULT '',
  `document_count` int NOT NULL DEFAULT 0,
  `status` tinyint(1) NOT NULL DEFAULT 1,
  `created_by` bigint NOT NULL,
  `updated_by` bigint NOT NULL,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  `deleted_at` datetime(6) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ai_kb_scope` (`tenant_id`,`corp_id`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `mochat_go_ai_agents` (
  `id` varchar(36) NOT NULL,
  `tenant_id` bigint NOT NULL,
  `corp_id` bigint NOT NULL,
  `name` varchar(128) NOT NULL,
  `description` varchar(512) NOT NULL DEFAULT '',
  `knowledge_base_ids` json NOT NULL,
  `status` tinyint(1) NOT NULL DEFAULT 1,
  `created_by` bigint NOT NULL,
  `updated_by` bigint NOT NULL,
  `created_at` datetime(6) NOT NULL,
  `updated_at` datetime(6) NOT NULL,
  `deleted_at` datetime(6) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_ai_agent_scope` (`tenant_id`,`corp_id`,`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
