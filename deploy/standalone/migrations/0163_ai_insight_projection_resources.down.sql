-- Conservative rollback: resource ownership is unknown because the up migration
-- intentionally keeps matching pre-existing catalog rows. Preserve all resource
-- rows rather than deleting entries that may belong to catalog sync or operators.
-- Older application code has no matching handlers, so retained catalog entries
-- do not create a successful request path.
DO 0;
