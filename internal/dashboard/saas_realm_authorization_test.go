package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/saasauth"
)

func TestSaaSAdminSaaSPrincipalUsesSaaSIdentityWithoutLegacyUser(t *testing.T) {
	store := &pureSaaSAdminStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{
		users:    map[int]User{},
		packages: []SaaSAdminPackage{{Code: "growth", Name: "Growth", Status: 1}},
	}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/packages", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "700")
	req = req.WithContext(saasauth.WithPrincipal(context.Background(), saasauth.Principal{UserID: 700, AuthVersion: 1}))
	rec := httptest.NewRecorder()

	handler.Packages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSaaSAdminSaaSPrincipalCanReadProfileAndMutatePlatformState(t *testing.T) {
	store := &pureSaaSAdminStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{
		users:           map[int]User{},
		packages:        []SaaSAdminPackage{},
		provisionResult: SaaSAdminTenantProvisionResult{TenantID: 88, AdminUserID: 188},
	}}
	handler := NewSaaSAdminHandler(store, nil, 1, "test-secret")

	request := func(method, path, body string) (*httptest.ResponseRecorder, *http.Request) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(saasauth.WithPrincipal(context.Background(), saasauth.Principal{UserID: 700, AuthVersion: 1}))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		return rec, req
	}

	profileRec, profileReq := request(http.MethodGet, "/dashboard/saasAdmin/accessProfile", "")
	handler.AccessProfile(profileRec, profileReq)
	if profileRec.Code != http.StatusOK {
		t.Fatalf("profile status = %d body=%s", profileRec.Code, profileRec.Body.String())
	}

	packageRec, packageReq := request(http.MethodPost, "/dashboard/saasAdmin/package", `{"code":"growth","name":"Growth","status":1}`)
	handler.UpsertPackage(packageRec, packageReq)
	if packageRec.Code != http.StatusOK {
		t.Fatalf("package status = %d body=%s", packageRec.Code, packageRec.Body.String())
	}
	if store.lastPackageUpsert.ActorUserID != 700 || store.lastPackageUpsert.ActorTenantID != 1 {
		t.Fatalf("package actor = %+v", store.lastPackageUpsert)
	}

	provisionRec, provisionReq := request(http.MethodPost, "/dashboard/saasAdmin/tenants/provision", `{"tenantName":"New tenant","adminPhone":"13800138088","adminName":"Tenant admin","password":"abc123","roleName":"Admin","packageCode":"growth","expiresAt":"2028-07-09","configCopyMode":"missing"}`)
	handler.ProvisionTenant(provisionRec, provisionReq)
	if provisionRec.Code != http.StatusOK {
		t.Fatalf("provision status = %d body=%s", provisionRec.Code, provisionRec.Body.String())
	}
	if store.lastProvision.ActorUserID != 700 || store.lastProvision.ActorTenantID != 1 {
		t.Fatalf("provision actor = %+v", store.lastProvision)
	}
}

func TestSaaSAdminSaaSPrincipalRejectsForgedHeaderInactiveAndUnprivilegedActors(t *testing.T) {
	newRequest := func(actor SaaSAdminActor, store SaaSAdminOverviewStore, withPrincipal bool) (*httptest.ResponseRecorder, *http.Request) {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/packages", nil)
		req.Header.Set("X-Mochat-Go-User-ID", "700")
		if withPrincipal {
			req = req.WithContext(saasauth.WithPrincipal(context.Background(), saasauth.Principal{UserID: 700, AuthVersion: 1}))
		}
		rec := httptest.NewRecorder()
		return rec, req
	}

	for _, tc := range []struct {
		name       string
		actor      SaaSAdminActor
		withBearer bool
		wantStatus int
	}{
		{name: "inactive SaaS identity", actor: SaaSAdminActor{ID: 700, Status: 2, IsPlatformSuperAdmin: true}, withBearer: true, wantStatus: http.StatusUnauthorized},
		{name: "active SaaS identity without permission", actor: SaaSAdminActor{ID: 700, Status: 1}, withBearer: true, wantStatus: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &pureSaaSAdminAccessStore{fakeSaaSAdminAccessStore: &fakeSaaSAdminAccessStore{fakeSaaSAdminStore: &fakeSaaSAdminStore{users: map[int]User{}}}}
			store.actor = tc.actor
			handler := NewSaaSAdminHandler(store, nil, 1)
			rec, req := newRequest(tc.actor, store, tc.withBearer)
			handler.Packages(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}

	store := &fakeSaaSAdminStore{users: map[int]User{}}
	handler := NewSaaSAdminHandler(store, nil, 1)
	rec, req := newRequest(SaaSAdminActor{}, store, false)
	handler.Packages(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged header status = %d body=%s", rec.Code, rec.Body.String())
	}
}

type pureSaaSAdminStore struct {
	*fakeSaaSAdminStore
	actor SaaSAdminActor
}

type pureSaaSAdminAccessStore struct {
	*fakeSaaSAdminAccessStore
	actor SaaSAdminActor
}

func (s *pureSaaSAdminAccessStore) SaaSAdminActorByID(_ context.Context, userID int) (SaaSAdminActor, bool, error) {
	if userID != 700 {
		return SaaSAdminActor{}, false, nil
	}
	return s.actor, true, nil
}

func (s *pureSaaSAdminStore) SaaSAdminActorByID(_ context.Context, userID int) (SaaSAdminActor, bool, error) {
	if userID != 700 {
		return SaaSAdminActor{}, false, nil
	}
	if s.actor.ID != 0 {
		return s.actor, true, nil
	}
	return SaaSAdminActor{ID: userID, Name: "SaaS root", Status: 1, IsPlatformSuperAdmin: true}, true, nil
}
