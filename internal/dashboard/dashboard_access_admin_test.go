package dashboard

import (
	"context"
	"errors"
	"testing"
)

type fakeDashboardAccessAdminStore struct {
	identities map[int]DashboardAccessIdentity
	catalog    []DashboardPermissionDefinition
	users      DashboardAccessUserPage
	user       DashboardAccessUserDetail
	userFound  bool
	roles      DashboardAccessRolePage
	role       DashboardAccessRoleDetail
	roleFound  bool
	audits     DashboardPermissionAuditPage
	writeErr   error
	lastTenant int
	lastActor  int
	writeCalls int
}

func (store *fakeDashboardAccessAdminStore) DashboardAccessIdentity(_ context.Context, userID int) (DashboardAccessIdentity, bool, error) {
	identity, ok := store.identities[userID]
	return identity, ok, nil
}

func (store *fakeDashboardAccessAdminStore) DashboardPermissionCatalog(context.Context) ([]DashboardPermissionDefinition, error) {
	return append([]DashboardPermissionDefinition(nil), store.catalog...), nil
}

func (store *fakeDashboardAccessAdminStore) DashboardPermissionGrants(context.Context, int, int) ([]DashboardPermissionGrantFact, error) {
	return nil, nil
}

func (store *fakeDashboardAccessAdminStore) DashboardEmployeeScope(context.Context, int, int, int) (DashboardEmployeeScope, bool, error) {
	return DashboardEmployeeScope{}, false, nil
}

func (store *fakeDashboardAccessAdminStore) DashboardAccessUsers(_ context.Context, tenantID, page, perPage int) (DashboardAccessUserPage, error) {
	store.lastTenant = tenantID
	return store.users, nil
}

func (store *fakeDashboardAccessAdminStore) DashboardAccessUser(_ context.Context, tenantID, userID int) (DashboardAccessUserDetail, bool, error) {
	store.lastTenant = tenantID
	if store.userFound && store.user.ID == userID {
		return store.user, true, nil
	}
	return DashboardAccessUserDetail{}, false, nil
}

func (store *fakeDashboardAccessAdminStore) DashboardAccessRoles(_ context.Context, tenantID, page, perPage int) (DashboardAccessRolePage, error) {
	store.lastTenant = tenantID
	return store.roles, nil
}

func (store *fakeDashboardAccessAdminStore) DashboardAccessRole(_ context.Context, tenantID, roleID int) (DashboardAccessRoleDetail, bool, error) {
	store.lastTenant = tenantID
	if store.roleFound && store.role.ID == roleID {
		return store.role, true, nil
	}
	return DashboardAccessRoleDetail{}, false, nil
}

func (store *fakeDashboardAccessAdminStore) DashboardPermissionAudits(_ context.Context, tenantID int, filter DashboardPermissionAuditFilter) (DashboardPermissionAuditPage, error) {
	store.lastTenant = tenantID
	return store.audits, nil
}

func (store *fakeDashboardAccessAdminStore) ReplaceUserDashboardAccess(_ context.Context, command ReplaceUserDashboardAccessCommand) (DashboardAccessUserDetail, error) {
	store.lastTenant, store.lastActor = command.TenantID, command.ActorUserID
	store.writeCalls++
	return store.user, store.writeErr
}

func (store *fakeDashboardAccessAdminStore) CreateDashboardRole(_ context.Context, command CreateDashboardRoleCommand) (DashboardAccessRoleDetail, error) {
	store.lastTenant, store.lastActor = command.TenantID, command.ActorUserID
	store.writeCalls++
	return store.role, store.writeErr
}

func (store *fakeDashboardAccessAdminStore) UpdateDashboardRole(_ context.Context, command UpdateDashboardRoleCommand) (DashboardAccessRoleDetail, error) {
	store.lastTenant, store.lastActor = command.TenantID, command.ActorUserID
	store.writeCalls++
	return store.role, store.writeErr
}

func (store *fakeDashboardAccessAdminStore) UpdateDashboardRoleStatus(_ context.Context, command UpdateDashboardRoleStatusCommand) (DashboardAccessRoleDetail, error) {
	store.lastTenant, store.lastActor = command.TenantID, command.ActorUserID
	store.writeCalls++
	return store.role, store.writeErr
}

func (store *fakeDashboardAccessAdminStore) DeleteDashboardRole(_ context.Context, command DeleteDashboardRoleCommand) error {
	store.lastTenant, store.lastActor = command.TenantID, command.ActorUserID
	store.writeCalls++
	return store.writeErr
}

func newDashboardAccessAdminFixture() (*DashboardAccessAdminService, *fakeDashboardAccessAdminStore) {
	store := &fakeDashboardAccessAdminStore{
		identities: map[int]DashboardAccessIdentity{
			1: {UserID: 1, TenantID: 9, UserName: "管理员", CorpName: "极义科技", Status: 1, IsSuperAdmin: true},
			2: {UserID: 2, TenantID: 9, UserName: "普通用户", Status: 1},
		},
		catalog: []DashboardPermissionDefinition{
			{ID: 1, Code: "dashboard.index", Path: "/index", Name: "数据概览"},
			{ID: 50, Code: "dashboard.company_setting.staff", Path: "/company-setting/staff", Name: "员工权限", SuperadminOnly: true},
		},
		user:      DashboardAccessUserDetail{ID: 7, TenantID: 9, Name: "目标用户", Version: 3},
		userFound: true,
		role:      DashboardAccessRoleDetail{ID: 8, TenantID: 9, Name: "销售", Status: 1, Version: 4},
		roleFound: true,
	}
	return NewDashboardAccessAdminService(store), store
}

func TestDashboardAccessAdminProfileIsOpenButManagementRequiresSuperadmin(t *testing.T) {
	service, _ := newDashboardAccessAdminFixture()
	if _, err := service.Profile(context.Background(), 2, 0); err != nil {
		t.Fatalf("ordinary profile: %v", err)
	}
	if _, err := service.Catalog(context.Background(), 2); !errors.Is(err, ErrDashboardAccessAdminForbidden) {
		t.Fatalf("ordinary catalog error=%v", err)
	}
	if _, err := service.Users(context.Background(), 2, 1, 20); !errors.Is(err, ErrDashboardAccessAdminForbidden) {
		t.Fatalf("ordinary users error=%v", err)
	}
}

func TestDashboardAccessAdminReadsOnlyAuthenticatedTenantAndHidesCrossTenantIDs(t *testing.T) {
	service, store := newDashboardAccessAdminFixture()
	user, err := service.User(context.Background(), 1, 7)
	if err != nil || user.ID != 7 || store.lastTenant != 9 {
		t.Fatalf("user=%+v err=%v tenant=%d", user, err, store.lastTenant)
	}
	store.user = DashboardAccessUserDetail{ID: 7, TenantID: 10}
	if _, err := service.User(context.Background(), 1, 7); !errors.Is(err, ErrDashboardAccessAdminNotFound) {
		t.Fatalf("cross tenant user error=%v", err)
	}
	if _, err := service.Role(context.Background(), 1, 404); !errors.Is(err, ErrDashboardAccessAdminNotFound) {
		t.Fatalf("unknown role error=%v", err)
	}
}

func TestDashboardAccessAdminRejectsSuperadminOnlyGrantsBeforeStoreWrite(t *testing.T) {
	service, store := newDashboardAccessAdminFixture()
	_, err := service.ReplaceUserAccess(context.Background(), 1, 7, ReplaceUserDashboardAccessInput{
		DirectPermissions: []DashboardPermissionAssignment{{Code: "dashboard.company_setting.staff", Scope: DataScopeTenant}},
		ExpectedVersion:   3,
	})
	if !errors.Is(err, ErrDashboardAccessAdminInvalid) || store.writeCalls != 0 {
		t.Fatalf("error=%v writeCalls=%d", err, store.writeCalls)
	}
	_, err = service.CreateRole(context.Background(), 1, CreateDashboardRoleInput{
		Name: "越权角色", Status: 1,
		Permissions: []DashboardPermissionAssignment{{Code: "dashboard.company_setting.staff", Scope: DataScopeTenant}},
	})
	if !errors.Is(err, ErrDashboardAccessAdminInvalid) || store.writeCalls != 0 {
		t.Fatalf("error=%v writeCalls=%d", err, store.writeCalls)
	}
}

func TestDashboardAccessAdminRejectsReservedRoleRemarksBeforeStoreWrite(t *testing.T) {
	for _, remark := range []string{"系统预置全权限角色", "bootstrap full-access role"} {
		t.Run(remark, func(t *testing.T) {
			service, store := newDashboardAccessAdminFixture()
			_, err := service.CreateRole(context.Background(), 1, CreateDashboardRoleInput{Name: "伪装角色", Remark: remark, Status: 1})
			if !errors.Is(err, ErrDashboardAccessAdminInvalid) || store.writeCalls != 0 {
				t.Fatalf("create error=%v writeCalls=%d", err, store.writeCalls)
			}
			_, err = service.UpdateRole(context.Background(), 1, 8, UpdateDashboardRoleInput{Name: "伪装角色", Remark: remark, ExpectedVersion: 4})
			if !errors.Is(err, ErrDashboardAccessAdminInvalid) || store.writeCalls != 0 {
				t.Fatalf("update error=%v writeCalls=%d", err, store.writeCalls)
			}
		})
	}
}

func TestDashboardAccessAdminPropagatesVersionAndMemberConflicts(t *testing.T) {
	service, store := newDashboardAccessAdminFixture()
	store.writeErr = ErrDashboardAccessAdminConflict
	if _, err := service.UpdateRoleStatus(context.Background(), 1, 8, UpdateDashboardRoleStatusInput{Status: 2, ExpectedVersion: 3}); !errors.Is(err, ErrDashboardAccessAdminConflict) {
		t.Fatalf("version error=%v", err)
	}
	store.writeErr = ErrDashboardAccessRoleHasMembers
	if err := service.DeleteRole(context.Background(), 1, 8, DeleteDashboardRoleInput{ExpectedVersion: 4}); !errors.Is(err, ErrDashboardAccessRoleHasMembers) {
		t.Fatalf("member error=%v", err)
	}
}

func TestDashboardAccessAdminCommandsDeriveTenantAndActor(t *testing.T) {
	service, store := newDashboardAccessAdminFixture()
	_, err := service.ReplaceUserAccess(context.Background(), 1, 7, ReplaceUserDashboardAccessInput{
		RoleIDs: []int{8}, DirectPermissions: []DashboardPermissionAssignment{{Code: "dashboard.index", Scope: DataScopeSelf}}, ExpectedVersion: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.lastTenant != 9 || store.lastActor != 1 {
		t.Fatalf("tenant=%d actor=%d", store.lastTenant, store.lastActor)
	}
}
