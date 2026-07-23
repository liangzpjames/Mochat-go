ALTER TABLE `mochat_go_saas_release_evidence`
  DROP KEY `idx_mochat_go_saas_release_evidence_artifact`,
  DROP COLUMN `artifact_size_bytes`,
  DROP COLUMN `artifact_sha256`;
