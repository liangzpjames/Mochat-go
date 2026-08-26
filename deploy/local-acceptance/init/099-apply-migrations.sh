#!/bin/sh
set -eu

# Match the standalone compose baseline: the aggregated schema plus the
# historical seed/incremental files through 0104_scrm_opportunity_owner.
# Controlled 0130/0131 are
# deliberately outside a generic local bootstrap and are never ledgered here.
for migration in /mochat-migrations/*.up.sql; do
  name="$(basename "${migration}")"
  version="${name%%_*}"
  if [ "${version}" -ge 0002 ] && [ "${version}" -le 0104 ]; then
    mariadb --protocol=socket -uroot -p"${MARIADB_ROOT_PASSWORD}" "${MARIADB_DATABASE}" < "${migration}"
  fi
done

# The acceptance stack needs only these additive contracts. This exact list
# keeps unrelated and controlled migrations closed while exercising the
# production archive, media, auth and Dashboard schemas used by this task.
for name in \
  0127_dashboard_page_rbac.up.sql \
  0129_identity_realms_single_corp_schema.up.sql \
  0133_archive_simulation_registry.up.sql \
  0138_archive_source_sync.up.sql \
  0143_group_conversation_workspace.up.sql \
  0166_wecom_integration_and_archive_media.up.sql
do
  mariadb --protocol=socket -uroot -p"${MARIADB_ROOT_PASSWORD}" "${MARIADB_DATABASE}" < "/mochat-migrations/${name}"
done
