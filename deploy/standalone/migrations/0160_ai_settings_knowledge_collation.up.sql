-- Align knowledge-base join keys with the original AI settings tables.
-- Without this, MariaDB rejects assistant statistics queries after a real
-- document is associated because 0124 used general_ci while 0156 used
-- unicode_ci for the child tables.

ALTER TABLE `mochat_go_ai_knowledge_documents`
  MODIFY COLUMN `knowledge_base_id` varchar(36)
  CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL;

ALTER TABLE `mochat_go_ai_knowledge_chunks`
  MODIFY COLUMN `knowledge_base_id` varchar(36)
  CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL;
