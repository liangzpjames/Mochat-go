package store

import (
	"context"
	"database/sql"
	"errors"

	"jiyi/mochat-go/internal/dashboard"
)

// SaaSAdminActorByID resolves only control-plane identities. The platform
// tenant ID is intentionally supplied by the handler as request scope; it is
// not read from or synthesized in mc_user.
func (s *MySQLStore) SaaSAdminActorByID(ctx context.Context, userID int) (dashboard.SaaSAdminActor, bool, error) {
	if s == nil || s.db == nil || userID <= 0 {
		return dashboard.SaaSAdminActor{}, false, nil
	}
	var actor dashboard.SaaSAdminActor
	var isRoot int
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, COALESCE(u.name, ''), COALESCE(u.phone, ''), u.status,
			CASE WHEN EXISTS (
				SELECT 1
				FROM mochat_go_saas_admin_user_roles ur
				INNER JOIN mochat_go_saas_admin_roles r ON r.id = ur.role_id
				INNER JOIN mochat_go_saas_admin_role_permissions rp ON rp.role_id = r.id
				WHERE ur.user_id = u.id
				  AND r.code = 'platform_root'
				  AND r.status = 1
				  AND r.is_system = 1
				  AND rp.permission_code = '*'
			) THEN 1 ELSE 0 END
		FROM mochat_go_saas_admin_users u
		WHERE u.id = ?
		LIMIT 1
	`, userID).Scan(&actor.ID, &actor.Name, &actor.Phone, &actor.Status, &isRoot)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.SaaSAdminActor{}, false, nil
	}
	if err != nil {
		return dashboard.SaaSAdminActor{}, false, err
	}
	actor.IsPlatformSuperAdmin = isRoot == 1
	return actor, true, nil
}
