-- Repair development/provider data where a library advertised a published
-- version but the immutable version snapshot had not been materialized.
INSERT INTO mochat_go_keyword_versions
  (tenant_id, corp_id, library_id, version, entry_count, publisher_id)
SELECT l.tenant_id, l.corp_id, l.id, l.published_version,
       COUNT(e.id), 0
FROM mochat_go_keyword_libraries l
LEFT JOIN mochat_go_keyword_entries e
  ON e.tenant_id = l.tenant_id
 AND e.corp_id = l.corp_id
 AND e.library_id = l.id
 AND e.status = 'enabled'
WHERE l.published_version > 0
  AND NOT EXISTS (
    SELECT 1
    FROM mochat_go_keyword_versions v
    WHERE v.tenant_id = l.tenant_id
      AND v.corp_id = l.corp_id
      AND v.library_id = l.id
      AND v.version = l.published_version
  )
GROUP BY l.tenant_id, l.corp_id, l.id, l.published_version;

INSERT INTO mochat_go_keyword_version_entries
  (tenant_id, corp_id, library_id, version, source_entry_id, keyword)
SELECT l.tenant_id, l.corp_id, l.id, l.published_version, e.id, e.keyword
FROM mochat_go_keyword_libraries l
JOIN mochat_go_keyword_entries e
  ON e.tenant_id = l.tenant_id
 AND e.corp_id = l.corp_id
 AND e.library_id = l.id
 AND e.status = 'enabled'
WHERE l.published_version > 0
  AND NOT EXISTS (
    SELECT 1
    FROM mochat_go_keyword_version_entries ve
    WHERE ve.tenant_id = l.tenant_id
      AND ve.corp_id = l.corp_id
      AND ve.library_id = l.id
      AND ve.version = l.published_version
      AND ve.source_entry_id = e.id
  );
