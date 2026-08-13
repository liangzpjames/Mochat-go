package dashboard

import (
	"context"
	"errors"
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

type panicLoginCache struct{}

func (panicLoginCache) UserCorpCache(context.Context, int) (string, error) {
	panic("legacy UserCorpCache must not be called by Dashboard page handlers")
}

func TestRiskBehaviorHandlerIgnoresLegacyLoginCache(t *testing.T) {
	provider := &riskHandlerProvider{tenantID: 23}
	handler := NewRiskBehaviorHandler(provider, panicLoginCache{}, riskHandlerResolver{}, nil)

	readRequest := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/risk/rules?corpId=999", nil, 7, 23, 5, 9)
	readResponse := httptest.NewRecorder()
	handler.Rules(readResponse, readRequest)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read status=%d body=%s", readResponse.Code, readResponse.Body.String())
	}
	if provider.filter.TenantID != 23 || provider.filter.CorpID != 5 {
		t.Fatalf("read filter=%+v", provider.filter)
	}

	writeRequest := authenticatedDashboardRequestForTestAs(http.MethodPost, "/dashboard/risk/rules?corpId=999", strings.NewReader(`{"name":"test"}`), 7, 23, 5, 9)
	writeResponse := httptest.NewRecorder()
	handler.CreateRule(writeResponse, writeRequest)
	if writeResponse.Code == http.StatusUnauthorized {
		t.Fatalf("write status=%d body=%s", writeResponse.Code, writeResponse.Body.String())
	}
}

func TestRiskBehaviorHandlerAuthSemantics(t *testing.T) {
	t.Run("missing context is machine unauthorized", func(t *testing.T) {
		handler := NewRiskBehaviorHandler(&riskHandlerProvider{}, panicLoginCache{}, riskHandlerResolver{}, nil)
		response := httptest.NewRecorder()
		handler.Rules(response, httptest.NewRequest(http.MethodGet, "/dashboard/risk/rules", nil))
		body := decodeBody(t, response.Body.Bytes())
		if response.Code != http.StatusUnauthorized || body["errorCode"] != "UNAUTHORIZED" {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("permission denial uses context identity and machine code", func(t *testing.T) {
		authorizer := &recordingAuthorizer{err: errors.New("denied")}
		handler := NewRiskBehaviorHandler(&riskHandlerProvider{}, panicLoginCache{}, riskHandlerResolver{}, authorizer)
		request := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/risk/rules?corpId=999", nil, 7, 23, 5, 9)
		response := httptest.NewRecorder()
		handler.Rules(response, request)
		body := decodeBody(t, response.Body.Bytes())
		if response.Code != http.StatusForbidden || body["errorCode"] != DashboardPermissionDeniedCode {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if authorizer.corpID != 5 || authorizer.workEmployeeID != 9 || authorizer.permissionKey != "/ai-insight/v2/risk#read" {
			t.Fatalf("authorizer=%+v", authorizer)
		}
	})
}

func TestRiskBehaviorHandlerResolvesTenantFromCorp(t *testing.T) {
	provider := &riskHandlerProvider{tenantID: 23}
	handler := NewRiskBehaviorHandler(provider, staticCache("5-9"), riskHandlerResolver{}, nil)
	req := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/risk/rules", nil, 7, 23, 5, 9)
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
	req := authenticatedDashboardRequestForTestAs(http.MethodPost, "/dashboard/risk/records/audit", strings.NewReader(`{"ids":[1],"action":"approve"}`), 7, 23, 5, 9)
	req = req.WithContext(WithDashboardAccessContext(req.Context(), DashboardAccessContext{UserID: 7, TenantID: 23, CorpID: 5, WorkEmployeeID: 9, ScopeRequired: true, Scope: DataScopeSelf, AllowedEmployeeIDs: []int{9}}))
	rec := httptest.NewRecorder()
	handler.AuditRecords(rec, req)
	body := decodeBody(t, rec.Body.Bytes())
	if rec.Code != http.StatusForbidden || body["errorCode"] != DashboardPermissionDeniedCode {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
