package dashboard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type timeoutHandlerProvider struct {
	tenantID   int
	ruleFilter TimeoutRuleFilter
}

func (p *timeoutHandlerProvider) TenantIDByCorpID(context.Context, int) (int, error) {
	return p.tenantID, nil
}
func (p *timeoutHandlerProvider) TimeoutRulePage(_ context.Context, f TimeoutRuleFilter) (TimeoutRulePage, error) {
	p.ruleFilter = f
	return TimeoutRulePage{}, nil
}
func (p *timeoutHandlerProvider) TimeoutRecordPage(context.Context, TimeoutRecordFilter) (TimeoutRecordPage, error) {
	return TimeoutRecordPage{}, nil
}
func (p *timeoutHandlerProvider) TimeoutSettings(context.Context, int, int) (TimeoutSettings, error) {
	return TimeoutSettings{}, nil
}

func TestTimeoutWarningHandlerIgnoresLegacyLoginCache(t *testing.T) {
	provider := &timeoutHandlerProvider{tenantID: 23}
	handler := NewTimeoutWarningHandler(provider, panicLoginCache{}, riskHandlerResolver{}, nil)

	readRequest := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/timeout-warning/rules?corpId=999", nil, 7, 23, 5, 9)
	readResponse := httptest.NewRecorder()
	handler.Rules(readResponse, readRequest)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read status=%d body=%s", readResponse.Code, readResponse.Body.String())
	}
	if provider.ruleFilter.TenantID != 23 || provider.ruleFilter.CorpID != 5 {
		t.Fatalf("read filter=%+v", provider.ruleFilter)
	}

	writeRequest := authenticatedDashboardRequestForTestAs(http.MethodPost, "/dashboard/timeout-warning/rules?corpId=999", strings.NewReader(`{"name":"test"}`), 7, 23, 5, 9)
	writeResponse := httptest.NewRecorder()
	handler.CreateRule(writeResponse, writeRequest)
	if writeResponse.Code == http.StatusUnauthorized {
		t.Fatalf("write status=%d body=%s", writeResponse.Code, writeResponse.Body.String())
	}
}

func TestTimeoutWarningHandlerAuthSemantics(t *testing.T) {
	t.Run("missing context is machine unauthorized", func(t *testing.T) {
		handler := NewTimeoutWarningHandler(&timeoutHandlerProvider{}, panicLoginCache{}, riskHandlerResolver{}, nil)
		response := httptest.NewRecorder()
		handler.Rules(response, httptest.NewRequest(http.MethodGet, "/dashboard/timeout-warning/rules", nil))
		body := decodeBody(t, response.Body.Bytes())
		if response.Code != http.StatusUnauthorized || body["errorCode"] != "UNAUTHORIZED" {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("permission denial uses context identity and machine code", func(t *testing.T) {
		authorizer := &recordingAuthorizer{err: errors.New("denied")}
		handler := NewTimeoutWarningHandler(&timeoutHandlerProvider{}, panicLoginCache{}, riskHandlerResolver{}, authorizer)
		request := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/timeout-warning/rules?corpId=999", nil, 7, 23, 5, 9)
		response := httptest.NewRecorder()
		handler.Rules(response, request)
		body := decodeBody(t, response.Body.Bytes())
		if response.Code != http.StatusForbidden || body["errorCode"] != DashboardPermissionDeniedCode {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if authorizer.corpID != 5 || authorizer.workEmployeeID != 9 || authorizer.permissionKey != "/ai-insight/v2/timeout#read" {
			t.Fatalf("authorizer=%+v", authorizer)
		}
	})
}

func TestTimeoutWarningHandlerResolvesTenantAndCorp(t *testing.T) {
	provider := &timeoutHandlerProvider{tenantID: 23}
	handler := NewTimeoutWarningHandler(provider, staticCache("5-9"), riskHandlerResolver{}, nil)
	req := authenticatedDashboardRequestForTestAs(http.MethodGet, "/dashboard/timeout-warning/rules", nil, 7, 23, 5, 9)
	rec := httptest.NewRecorder()
	handler.Rules(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if provider.ruleFilter.TenantID != 23 || provider.ruleFilter.CorpID != 5 {
		t.Fatalf("filter=%+v", provider.ruleFilter)
	}
}
