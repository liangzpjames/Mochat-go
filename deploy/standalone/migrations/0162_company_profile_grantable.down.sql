-- Conservative rollback: resource ownership and previous restriction are unknown
-- because the up migration intentionally reconciles pre-existing catalog rows.
-- Preserve both rows instead of deleting or overwriting data not proven to be
-- created by this migration. Runtime RBAC remains fail-closed if older code is
-- restored because its deny-only policy still guards these endpoints.
DO 0;
