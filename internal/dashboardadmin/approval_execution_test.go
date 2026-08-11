package dashboardadmin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestExecuteApprovalRequiresResolvedGovernancePermissionAndCarriesReference(t *testing.T) {
	store := &recordingStore{resend: ResendActivationResult{TenantID: 41, DashboardUserID: 52, Version: 5}}
	service := NewService(store)
	raw := json.RawMessage(`{"tenantId":41,"targetUserId":52,"expectedVersion":4}`)

	if _, err := service.ExecuteApproval(context.Background(), Actor{UserID: 9, Active: true, Permissions: []string{"platform.audit.read"}}, ApprovalActionActivationResend, raw, 81, 6); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("unprojected execution actor error=%v, want ErrPermissionDenied", err)
	}

	result, err := service.ExecuteApproval(context.Background(), NewSaaSApprovalExecutionActor(9), ApprovalActionActivationResend, raw, 81, 6)
	if err != nil {
		t.Fatalf("resolved execution actor: %v", err)
	}
	if result["version"] != uint64(5) || store.seenResendInput.ApprovalExecutionID != 81 || store.seenResendInput.ApprovalExecutionVersion != 6 {
		t.Fatalf("result=%v input=%+v, want approval reference forwarded to Store", result, store.seenResendInput)
	}
}

func TestExecuteApprovalCarriesReferenceForEveryDashboardGovernanceAction(t *testing.T) {
	store := &recordingStore{}
	service := NewService(store)
	provisionJSON, err := json.Marshal(validProvisionInput())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		action string
		raw    json.RawMessage
	}{
		{name: "provision", action: ApprovalActionTenantProvision, raw: provisionJSON},
		{name: "resend", action: ApprovalActionActivationResend, raw: json.RawMessage(`{"tenantId":41,"targetUserId":52,"expectedVersion":4}`)},
		{name: "replace", action: ApprovalActionSuperAdminReplace, raw: json.RawMessage(`{"tenantId":41,"currentAdminId":52,"newAdminId":63,"expectedVersion":4}`)},
		{name: "status", action: ApprovalActionSuperAdminStatus, raw: json.RawMessage(`{"tenantId":41,"targetUserId":52,"enabled":false,"expectedVersion":4}`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.ExecuteApproval(context.Background(), NewSaaSApprovalExecutionActor(9), test.action, test.raw, 81, 6); err != nil {
				t.Fatalf("execute: %v", err)
			}
			switch test.action {
			case ApprovalActionTenantProvision:
				if store.seenInput.ApprovalExecutionID != 81 || store.seenInput.ApprovalExecutionVersion != 6 {
					t.Fatalf("provision reference=%d/%d", store.seenInput.ApprovalExecutionID, store.seenInput.ApprovalExecutionVersion)
				}
			case ApprovalActionActivationResend:
				if store.seenResendInput.ApprovalExecutionID != 81 || store.seenResendInput.ApprovalExecutionVersion != 6 {
					t.Fatalf("resend reference=%d/%d", store.seenResendInput.ApprovalExecutionID, store.seenResendInput.ApprovalExecutionVersion)
				}
			case ApprovalActionSuperAdminReplace:
				if store.seenReplaceInput.ApprovalExecutionID != 81 || store.seenReplaceInput.ApprovalExecutionVersion != 6 {
					t.Fatalf("replace reference=%d/%d", store.seenReplaceInput.ApprovalExecutionID, store.seenReplaceInput.ApprovalExecutionVersion)
				}
			case ApprovalActionSuperAdminStatus:
				if store.seenStatusInput.ApprovalExecutionID != 81 || store.seenStatusInput.ApprovalExecutionVersion != 6 {
					t.Fatalf("status reference=%d/%d", store.seenStatusInput.ApprovalExecutionID, store.seenStatusInput.ApprovalExecutionVersion)
				}
			}
		})
	}
}
