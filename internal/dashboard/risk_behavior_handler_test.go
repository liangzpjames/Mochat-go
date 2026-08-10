package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type riskHandlerProvider struct {
	tenantID int
	filter   RiskRuleFilter
}

func (p *riskHandlerProvider) TenantIDByCorpID(context.Context, int) (int, error) {
	return p.tenantID, nil
}

func (p *riskHandlerProvider) RiskRulePage(_ context.Context, filter RiskRuleFilter) (RiskRulePage, error) {
	p.filter = filter
	return RiskRulePage{}, nil
}

func (p *riskHandlerProvider) RiskRecordPage(context.Context, RiskRecordFilter) (RiskRecordPage, error) {
	return RiskRecordPage{}, nil
}

type riskHandlerResolver struct{}

func (riskHandlerResolver) UserID(*http.Request) (int, error) { return 7, nil }

func TestRiskBehaviorHandlerResolvesTenantFromCorp(t *testing.T) {
	provider := &riskHandlerProvider{tenantID: 23}
	handler := NewRiskBehaviorHandler(provider, staticCache("5-9"), riskHandlerResolver{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/risk/rules", nil)
	rec := httptest.NewRecorder()

	handler.Rules(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if provider.filter.CorpID != 5 || provider.filter.TenantID != 23 {
		t.Fatalf("filter=%+v", provider.filter)
	}
}

func TestRiskAuditRejectsRestrictedScopeBeforeMutation(t *testing.T) {
	handler := NewRiskBehaviorHandler(&riskHandlerProvider{}, staticCache("5-9"), riskHandlerResolver{}, nil)
	req := httptest.NewRequest(http.MethodPost, "/dashboard/risk/records/audit", strings.NewReader(`{"ids":[1],"action":"approve"}`))
	req = req.WithContext(WithDashboardAccessContext(req.Context(), DashboardAccessContext{ScopeRequired: true, Scope: DataScopeSelf, AllowedEmployeeIDs: []int{9}}))
	rec := httptest.NewRecorder()
	handler.AuditRecords(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
