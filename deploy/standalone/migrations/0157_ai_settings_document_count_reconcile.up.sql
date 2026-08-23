-- Historical AI knowledge-base rows allowed callers to write document_count
-- directly. Reconcile that legacy display value with the private document
-- ledger introduced in 0156 before the upload workflow becomes authoritative.
UPDATE `mochat_go_ai_knowledge_bases` knowledge_base
LEFT JOIN (
  SELECT `tenant_id`, `corp_id`, `knowledge_base_id`, COUNT(*) AS `document_count`
  FROM `mochat_go_ai_knowledge_documents`
  WHERE `deleted_at` IS NULL
  GROUP BY `tenant_id`, `corp_id`, `knowledge_base_id`
) documents
  ON documents.`tenant_id` = knowledge_base.`tenant_id`
 AND documents.`corp_id` = knowledge_base.`corp_id`
 AND documents.`knowledge_base_id` COLLATE utf8mb4_general_ci = knowledge_base.`id`
SET knowledge_base.`document_count` = COALESCE(documents.`document_count`, 0);
