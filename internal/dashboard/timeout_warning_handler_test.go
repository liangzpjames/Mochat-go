package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestTimeoutWarningHandlerResolvesTenantAndCorp(t *testing.T) {
	provider := &timeoutHandlerProvider{tenantID: 23}
	handler := NewTimeoutWarningHandler(provider, staticCache("5-9"), riskHandlerResolver{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/dashboard/timeout-warning/rules", nil)
	rec := httptest.NewRecorder()
	handler.Rules(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if provider.ruleFilter.TenantID != 23 || provider.ruleFilter.CorpID != 5 {
		t.Fatalf("filter=%+v", provider.ruleFilter)
	}
}
