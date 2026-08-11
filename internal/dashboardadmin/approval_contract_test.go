package dashboardadmin

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDashboardAdminApprovalActionsNormalizeWithoutActorOrCredentialMaterial(t *testing.T) {
	provisionBody := map[string]any{
		"tenantName": "Acme", "packageId": 11, "limits": completeApprovalLimits(),
		"subscription":         map[string]any{"status": "trialing", "billingCycle": "custom", "startsAt": "2026-08-11T00:00:00Z", "expiresAt": "2026-09-11T00:00:00Z"},
		"adminLoginIdentifier": "13800000001", "adminName": "管理员", "idempotencyKey": "approval-provision", "expectedVersion": 3,
	}
	encodedProvision, err := json.Marshal(provisionBody)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		action string
		body   string
	}{
		{name: "provision", action: ApprovalActionTenantProvision, body: string(encodedProvision)},
		{name: "resend", action: ApprovalActionActivationResend, body: `{"tenantId":41,"targetUserId":52,"expectedVersion":4}`},
		{name: "replace", action: ApprovalActionSuperAdminReplace, body: `{"tenantId":41,"currentAdminId":52,"newAdminId":63,"expectedVersion":4}`},
		{name: "status", action: ApprovalActionSuperAdminStatus, body: `{"tenantId":41,"targetUserId":52,"enabled":false,"expectedVersion":4}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			plan, err := NormalizeSaaSAdminApprovalPayload(test.action, json.RawMessage(test.body))
			if err != nil {
				t.Fatalf("normalize error: %v", err)
			}
			if plan.ActionType != test.action || len(plan.NormalizedJSON) == 0 {
				t.Fatalf("plan=%+v, want normalized action", plan)
			}
			for _, forbidden := range []string{"actorId", "actorUserId", "password", "activationToken", "tokenDigest"} {
				if strings.Contains(string(plan.NormalizedJSON), forbidden) {
					t.Fatalf("normalized approval payload contains forbidden field %q", forbidden)
				}
			}
		})
	}
}

func completeApprovalLimits() map[string]int64 {
	return map[string]int64{
		"maxCorps": 0, "maxUsers": 0, "maxContacts": 0, "maxRooms": 0, "maxAgents": 0, "channelCodes": 0,
		"shopCodes": 0, "radars": 0, "lotteries": 0, "roomInfinitePulls": 0, "roomFissions": 0, "roomClockIns": 0,
		"roomQualities": 0, "roomCalendars": 0, "roomReminds": 0, "contactSops": 0, "roomSops": 0,
		"sensitiveWords": 0, "storageMb": 0, "contactMessageBatches": 0, "roomMessageBatches": 0, "roomTagPulls": 0,
		"workRoomAutoPulls": 0, "workFissions": 0, "officialAccounts": 0, "asyncExecutions": 0,
	}
}

func TestDashboardAdminApprovalPayloadRejectsActorAndTrailingJSON(t *testing.T) {
	for _, body := range []string{
		`{"tenantId":41,"targetUserId":52,"expectedVersion":4,"actorId":700}`,
		`{"tenantId":41,"targetUserId":52,"expectedVersion":4} {}`,
	} {
		if _, err := NormalizeSaaSAdminApprovalPayload(ApprovalActionActivationResend, json.RawMessage(body)); err == nil {
			t.Fatalf("payload %q was accepted", body)
		}
	}
}
