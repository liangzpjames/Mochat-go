package dashboardadmin

import "testing"

func TestSaaSApprovalExecutionActorCarriesResolvedGovernancePermission(t *testing.T) {
	actor := NewSaaSApprovalExecutionActor(9)
	if actor.UserID != 9 || !actor.Active || !actor.HasPermission(PermissionTenantsManage) {
		t.Fatalf("approval execution actor=%+v, want active governance permission", actor)
	}
}
