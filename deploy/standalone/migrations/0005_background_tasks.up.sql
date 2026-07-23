-- Go-owned background task ledger for standalone runtime workers and crons.
-- Runtime SQLRecorder still creates or repairs these tables for legacy installs.

CREATE TABLE IF NOT EXISTS `mochat_go_background_tasks` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `started_at` datetime DEFAULT NULL,
  `stopped_at` datetime DEFAULT NULL,
  `last_error` text COLLATE utf8mb4_unicode_ci,
  `start_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `failure_count` bigint(20) unsigned NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_mochat_go_background_tasks_name` (`name`),
  KEY `idx_mochat_go_background_tasks_status_updated` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版后台任务状态表';

CREATE TABLE IF NOT EXISTS `mochat_go_background_task_runs` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `run_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `started_at` datetime DEFAULT NULL,
  `stopped_at` datetime DEFAULT NULL,
  `last_error` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_mochat_go_background_task_runs_run_id` (`run_id`),
  KEY `idx_mochat_go_background_task_runs_name_started` (`name`, `started_at`),
  KEY `idx_mochat_go_background_task_runs_status_updated` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版后台任务运行实例表';

CREATE TABLE IF NOT EXISTS `mochat_go_background_task_executions` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `tenant_id` int(10) unsigned NOT NULL DEFAULT '0' COMMENT '租户 ID，无法解析时为 0',
  `execution_id` varchar(160) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `task_run_id` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `task_name` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `kind` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `started_at` datetime DEFAULT NULL,
  `stopped_at` datetime DEFAULT NULL,
  `duration_ms` bigint(20) DEFAULT NULL,
  `last_error` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_mochat_go_background_task_executions_id` (`execution_id`),
  KEY `idx_mochat_go_background_task_executions_task_run` (`task_run_id`),
  KEY `idx_mochat_go_background_task_executions_task_kind_started` (`task_name`, `kind`, `started_at`),
  KEY `idx_mochat_go_background_task_executions_tenant_kind_status` (`tenant_id`, `kind`, `status`, `started_at`),
  KEY `idx_mochat_go_background_task_executions_status_updated` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Go 独立版后台任务执行历史表';
