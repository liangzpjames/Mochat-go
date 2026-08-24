-- Forward-only integrity ledger. The route natural key may have existed before
-- 0155 and the audit table may already contain compliance evidence, so rollback
-- deliberately preserves both. Re-applying 0155 is idempotent.
SELECT 1;
