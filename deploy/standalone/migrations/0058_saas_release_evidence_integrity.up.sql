ALTER TABLE `mochat_go_saas_release_evidence`
  ADD COLUMN `artifact_sha256` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' AFTER `source_fingerprint`,
  ADD COLUMN `artifact_size_bytes` bigint(20) unsigned NOT NULL DEFAULT '0' AFTER `artifact_sha256`,
  ADD KEY `idx_mochat_go_saas_release_evidence_artifact` (`artifact_sha256`, `id`);
