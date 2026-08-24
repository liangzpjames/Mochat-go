-- Uploaded document metadata, parsed chunks and private objects are durable
-- business data. The system-key column is also consumed by later migrations.
-- Refuse rollback so the migration record stays applied and no data is orphaned.
SIGNAL SQLSTATE '45000'
  SET MESSAGE_TEXT = '0156 rollback blocked: knowledge runtime contains durable data';
