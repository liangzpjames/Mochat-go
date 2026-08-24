ALTER TABLE `mochat_go_ai_knowledge_documents`
  MODIFY COLUMN `knowledge_base_id` varchar(36)
  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL;

ALTER TABLE `mochat_go_ai_knowledge_chunks`
  MODIFY COLUMN `knowledge_base_id` varchar(36)
  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL;
