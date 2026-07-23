package dashboard

import (
	"context"
	"testing"
)

func TestSaaSAdminCriticalApprovalPoliciesUseLockedTwoPersonBaseline(t *testing.T) {
	criticalCount := 0
	for _, policy := range SaaSAdminApprovalPolicies() {
		if policy.RiskLevel != SaaSAdminApprovalRiskCritical {
			continue
		}
		criticalCount++
		if !policy.Enabled || policy.AmountThresholdCents != 0 || policy.RequiredApprovals < 2 ||
			!saasAdminApprovalPolicyGovernanceLocked(policy) || saasAdminApprovalPolicyMinimumApprovals(policy) != 2 ||
			!saasAdminApprovalPolicyAmountThresholdLocked(policy) {
			t.Fatalf("critical policy is not locked: %+v", policy)
		}
	}
	if criticalCount != 31 {
		t.Fatalf("critical policy count = %d", criticalCount)
	}
	tenantEnable, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionTenantEnable)
	if !found || tenantEnable.RiskLevel != SaaSAdminApprovalRiskCritical ||
		tenantEnable.RequiredApprovals != 2 || tenantEnable.AmountThresholdCents != 0 ||
		tenantEnable.RequiredPermission != SaaSAdminPermissionTenantsManage || tenantEnable.TargetType != "tenant" {
		t.Fatalf("tenant enable policy = %+v found=%t", tenantEnable, found)
	}
	domainCreate, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionTenantDomainCreate)
	if !found || domainCreate.RiskLevel != SaaSAdminApprovalRiskCritical ||
		domainCreate.RequiredApprovals != 2 || domainCreate.AmountThresholdCents != 0 ||
		domainCreate.RequiredPermission != SaaSAdminPermissionDomainsManage ||
		domainCreate.TargetType != SaaSAdminOperationTargetTenantDomain {
		t.Fatalf("tenant domain create policy = %+v found=%t", domainCreate, found)
	}
	domain, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionTenantDomainCommand)
	if !found || domain.RiskLevel != SaaSAdminApprovalRiskCritical ||
		domain.RequiredApprovals != 2 || domain.AmountThresholdCents != 0 ||
		domain.RequiredPermission != SaaSAdminPermissionDomainsManage ||
		domain.TargetType != SaaSAdminOperationTargetTenantDomain {
		t.Fatalf("tenant domain policy = %+v found=%t", domain, found)
	}
	refund, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionPaymentRefundCreate)
	if !found || refund.RequiredApprovals != 2 || refund.AmountThresholdCents != 0 {
		t.Fatalf("refund policy = %+v found=%t", refund, found)
	}
	invoiceIssue, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionInvoiceIssue)
	if !found || invoiceIssue.RequiredApprovals != 2 || invoiceIssue.AmountThresholdCents != 0 ||
		invoiceIssue.RequiredPermission != SaaSAdminPermissionFinanceManage {
		t.Fatalf("invoice issue policy = %+v found=%t", invoiceIssue, found)
	}
	paymentOrderCreate, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionPaymentOrderCreate)
	if !found || paymentOrderCreate.RequiredApprovals != 2 || paymentOrderCreate.AmountThresholdCents != 0 ||
		paymentOrderCreate.RequiredPermission != SaaSAdminPermissionFinanceManage ||
		paymentOrderCreate.TargetType != SaaSAdminOperationTargetPaymentOrder {
		t.Fatalf("payment order create policy = %+v found=%t", paymentOrderCreate, found)
	}
	settlement, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionPaymentSettlementClose)
	if !found || settlement.RiskLevel != SaaSAdminApprovalRiskCritical || !saasAdminApprovalPolicyGovernanceLocked(settlement) ||
		settlement.RequiredApprovals != 2 || saasAdminApprovalPolicyMinimumApprovals(settlement) != 2 ||
		!saasAdminApprovalPolicyAmountThresholdLocked(settlement) {
		t.Fatalf("settlement policy = %+v found=%t", settlement, found)
	}
	settlementReopen, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionPaymentSettlementReopen)
	if !found || settlementReopen.RiskLevel != SaaSAdminApprovalRiskCritical ||
		settlementReopen.RequiredApprovals != 2 || settlementReopen.AmountThresholdCents != 0 ||
		settlementReopen.RequiredPermission != SaaSAdminPermissionFinanceManage ||
		settlementReopen.TargetType != SaaSAdminOperationTargetSettlementBatch {
		t.Fatalf("settlement reopen policy = %+v found=%t", settlementReopen, found)
	}
	settlementResolve, found := SaaSAdminApprovalPolicyByAction(SaaSAdminApprovalActionPaymentSettlementResolve)
	if !found || settlementResolve.RiskLevel != SaaSAdminApprovalRiskCritical ||
		settlementResolve.RequiredApprovals != 2 || settlementResolve.AmountThresholdCents != 0 ||
		settlementResolve.RequiredPermission != SaaSAdminPermissionFinanceManage ||
		settlementResolve.TargetType != SaaSAdminOperationTargetSettlementEntry {
		t.Fatalf("settlement resolve policy = %+v found=%t", settlementResolve, found)
	}
}

func TestSaaSAdminCriticalApprovalPoliciesCannotBeWeakened(t *testing.T) {
	for _, policy := range SaaSAdminApprovalPolicies() {
		if policy.RiskLevel != SaaSAdminApprovalRiskCritical {
			continue
		}
		for _, body := range []saasAdminApprovalPolicyBody{
			{ActionType: policy.ActionType, Enabled: false, RequiredApprovals: 2, SLAMinutes: policy.SLAMinutes, ReminderMinutes: policy.ReminderMinutes, ExpiryHours: policy.ExpiryHours, ExpectedVersion: 1},
			{ActionType: policy.ActionType, Enabled: true, RequiredApprovals: 1, SLAMinutes: policy.SLAMinutes, ReminderMinutes: policy.ReminderMinutes, ExpiryHours: policy.ExpiryHours, ExpectedVersion: 1},
		} {
			if err := validateSaaSAdminApprovalPolicyBody(&body); err == nil {
				t.Fatalf("expected critical policy weakening to fail: %+v", body)
			}
		}
	}
	refundThreshold := saasAdminApprovalPolicyBody{
		ActionType: SaaSAdminApprovalActionPaymentRefundCreate, Enabled: true, AmountThresholdCents: 1,
		RequiredApprovals: 2, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 24, ExpectedVersion: 1,
	}
	if err := validateSaaSAdminApprovalPolicyBody(&refundThreshold); err == nil {
		t.Fatal("expected refund threshold bypass to fail")
	}
	settlementWeakening := saasAdminApprovalPolicyBody{
		ActionType: SaaSAdminApprovalActionPaymentSettlementClose, Enabled: false,
		RequiredApprovals: 1, SLAMinutes: 180, ReminderMinutes: 30, ExpiryHours: 24, ExpectedVersion: 1,
	}
	if err := validateSaaSAdminApprovalPolicyBody(&settlementWeakening); err == nil {
		t.Fatal("expected settlement policy weakening to fail")
	}
	settlementReopenWeakening := saasAdminApprovalPolicyBody{
		ActionType: SaaSAdminApprovalActionPaymentSettlementReopen, Enabled: true, AmountThresholdCents: 1,
		RequiredApprovals: 1, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, ExpectedVersion: 1,
	}
	if err := validateSaaSAdminApprovalPolicyBody(&settlementReopenWeakening); err == nil {
		t.Fatal("expected settlement reopen policy weakening to fail")
	}
	settlementResolveWeakening := saasAdminApprovalPolicyBody{
		ActionType: SaaSAdminApprovalActionPaymentSettlementResolve, Enabled: false, AmountThresholdCents: 1,
		RequiredApprovals: 1, SLAMinutes: 240, ReminderMinutes: 60, ExpiryHours: 24, ExpectedVersion: 1,
	}
	if err := validateSaaSAdminApprovalPolicyBody(&settlementResolveWeakening); err == nil {
		t.Fatal("expected settlement resolve policy weakening to fail")
	}
	domainWeakening := saasAdminApprovalPolicyBody{
		ActionType: SaaSAdminApprovalActionTenantDomainCommand, Enabled: false, AmountThresholdCents: 1,
		RequiredApprovals: 1, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, ExpectedVersion: 1,
	}
	if err := validateSaaSAdminApprovalPolicyBody(&domainWeakening); err == nil {
		t.Fatal("expected tenant domain policy weakening to fail")
	}
	domainCreateWeakening := saasAdminApprovalPolicyBody{
		ActionType: SaaSAdminApprovalActionTenantDomainCreate, Enabled: false, AmountThresholdCents: 1,
		RequiredApprovals: 1, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 12, ExpectedVersion: 1,
	}
	if err := validateSaaSAdminApprovalPolicyBody(&domainCreateWeakening); err == nil {
		t.Fatal("expected tenant domain create policy weakening to fail")
	}
}

func TestSaaSAdminEffectiveCriticalPolicyClampsUnsafeStoredValues(t *testing.T) {
	store := &fakeSaaSAdminApprovalStore{policies: []SaaSAdminApprovalPolicy{{
		ActionType: SaaSAdminApprovalActionPaymentRefundCreate, Enabled: false, AmountThresholdCents: 1000,
		RequiredApprovals: 1, SLAMinutes: 120, ReminderMinutes: 30, ExpiryHours: 24, Version: 9,
	}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1)
	policy, err := handler.effectiveSaaSAdminApprovalPolicy(context.Background(), SaaSAdminApprovalActionPaymentRefundCreate)
	if err != nil {
		t.Fatal(err)
	}
	if !policy.Enabled || policy.AmountThresholdCents != 0 || policy.RequiredApprovals != 2 || policy.Version != 9 {
		t.Fatalf("effective critical policy = %+v", policy)
	}
	payload := saasAdminApprovalPolicyPayload(policy)
	if payload["governanceLocked"] != true || payload["minimumApprovals"] != 2 || payload["amountThresholdLocked"] != true {
		t.Fatalf("critical policy payload = %+v", payload)
	}
}
