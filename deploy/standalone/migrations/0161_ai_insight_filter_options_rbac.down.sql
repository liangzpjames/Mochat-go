-- Forward security fix: keep the scoped filter-option resources on rollback.
-- Their natural key is not migration-owned, so deleting by route could remove
-- a resource that predated 0161. Re-applying the up migration is idempotent.
SELECT 1;
