package dashboard

import (
	"context"
	"errors"

	"jiyi/mochat-go/internal/saasauth"
)

// SaaSAdminActor is the control-plane identity used by SaaS administration.
// It deliberately has no business tenant or corp ownership fields.
type SaaSAdminActor struct {
	ID                   int
	Name                 string
	Phone                string
	Status               int
	IsPlatformSuperAdmin bool
}

// SaaSAdminActorStore resolves actors from the SaaS identity realm.
// Implementations must not fall back to mc_user.
type SaaSAdminActorStore interface {
	SaaSAdminActorByID(ctx context.Context, userID int) (SaaSAdminActor, bool, error)
}

var ErrSaaSAdminActorStoreUnavailable = errors.New("SaaS admin actor store is not configured")

func resolveSaaSAdminActor(ctx context.Context, store any, platformTenantID int) (User, bool, error) {
	principal, err := saasauth.PrincipalFromContext(ctx)
	if err != nil {
		return User{}, false, nil
	}
	actorStore, ok := store.(SaaSAdminActorStore)
	if !ok {
		return User{}, false, ErrSaaSAdminActorStoreUnavailable
	}
	actor, found, err := actorStore.SaaSAdminActorByID(ctx, principal.UserID)
	if err != nil || !found {
		return User{}, found, err
	}
	return User{
		ID:           actor.ID,
		Name:         actor.Name,
		Phone:        actor.Phone,
		Status:       actor.Status,
		TenantID:     platformTenantID,
		IsSuperAdmin: boolToInt(actor.IsPlatformSuperAdmin),
	}, true, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
