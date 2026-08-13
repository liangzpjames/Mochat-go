package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type closureProvider struct {
	tenantID int
	filter   any
	saved    SilentCustomerRule
	refused  RefuseArchiveRecord
}

func (p *closureProvider) TenantIDByCorpID(context.Context, int) (int, error) {
	return p.tenantID, nil
}

// MessageInterceptProvider stubs so NewPhase33ClosureHandler compiles.
func (p *closureProvider) KeywordLibraryPage(context.Context, KeywordLibraryFilter) (KeywordLibraryPage, error) {
	return KeywordLibraryPage{}, nil
}
func (p *closureProvider) KeywordEntryPage(context.Context, KeywordEntryFilter) (KeywordEntryPage, error) {
	return KeywordEntryPage{}, nil
}
func (p *closureProvider) MessageInterceptRulePage(context.Context, MessageInterceptRuleFilter) (MessageInterceptRulePage, error) {
	return MessageInterceptRulePage{}, nil
}
func (p *closureProvider) MessageInterceptRecordPage(context.Context, MessageInterceptRecordFilter) (MessageInterceptRecordPage, error) {
	return MessageInterceptRecordPage{}, nil
}

func (p *closureProvider) SilentRulePage(_ context.Context, f SilentRuleFilter) (SilentRulePage, error) {
	p.filter = f
	return SilentRulePage{Items: []SilentCustomerRule{{ID: 1, Name: "沉默30天", SilentDays: 30, Status: "enabled"}}, Total: 1, Page: f.Page, PerPage: f.PerPage}, nil
}
func (p *closureProvider) SilentRecordPage(_ context.Context, f SilentRecordFilter) (SilentRecordPage, error) {
	p.filter = f
	return SilentRecordPage{Items: []SilentCustomerRecord{{ID: 2, CustomerName: "李雷", Status: "pending"}}, Total: 1}, nil
}
func (p *closureProvider) RefuseArchivePage(_ context.Context, f RefuseArchiveFilter) (RefuseArchivePage, error) {
	p.filter = f
	return RefuseArchivePage{Items: []RefuseArchiveRecord{{ID: 3, SubjectName: "张三", AuthorizationStatus: "refused"}}, Total: 1}, nil
}

func (p *closureProvider) SaveSilentRule(_ context.Context, v SilentCustomerRule) (int64, error) {
	if err := ValidateSilentRule(v); err != nil {
		return 0, err
	}
	p.saved = v
	return 11, nil
}
func (p *closureProvider) SetSilentRuleStatus(context.Context, int, int, int64, string) (bool, error) {
	return true, nil
}
func (p *closureProvider) DeleteSilentRule(context.Context, int, int, int64) (bool, error) {
	return true, nil
}
func (p *closureProvider) EvaluateSilentCustomer(context.Context, int, int, SilentCustomerActivity) (int64, error) {
	return 2, nil
}
func (p *closureProvider) ActSilentRecords(context.Context, int, int, int64, []int64, string, int64, string) (int64, error) {
	return 2, nil
}

func TestSilentActRejectsRestrictedScopeBeforeMutation(t *testing.T) {
	p := &closureProvider{}
	h := NewPhase33ClosureHandler(p, staticCache("5-9"), riskHandlerResolver{}, nil)
	req := authenticatedClosureRequest(http.MethodPost, "/dashboard/silent-customer/records/action", strings.NewReader(`{"ids":[1],"action":"assign","assignedEmployeeId":99}`))
	req = req.WithContext(WithDashboardAccessContext(req.Context(), DashboardAccessContext{UserID: 7, TenantID: 23, CorpID: 5, WorkEmployeeID: 9, ScopeRequired: true, Scope: DataScopeDepartment, AllowedEmployeeIDs: []int{9}}))
	rec := httptest.NewRecorder()
	h.ActSilent(rec, req)
	body := decodeBody(t, rec.Body.Bytes())
	if rec.Code != http.StatusForbidden || body["errorCode"] != DashboardPermissionDeniedCode {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
func (p *closureProvider) UpsertRefuseArchive(_ context.Context, _ int, _ int, _ int64, v RefuseArchiveRecord) (int64, error) {
	p.refused = v
	return 21, nil
}
func (p *closureProvider) FollowUpRefuseArchive(context.Context, int, int, int64, int64, string, string) (bool, error) {
	return true, nil
}

type denyAuthorizer struct{}

func (denyAuthorizer) Resolve(context.Context, int, string, int, int) (AccessContext, error) {
	return AccessContext{}, errors.New("denied")
}

func newClosureHandler() (*Phase33ClosureHandler, *closureProvider) {
	p := &closureProvider{tenantID: 23}
	return NewPhase33ClosureHandler(p, staticCache("5-9"), riskHandlerResolver{}, nil), p
}

func authenticatedClosureRequest(method, target string, body io.Reader) *http.Request {
	return authenticatedDashboardRequestForTestAs(method, target, body, 7, 23, 5, 9)
}

func envelopeData(t *testing.T, body string) map[string]any {
	t.Helper()
	var envelope struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, body)
	}
	if envelope.Code != 0 {
		t.Fatalf("envelope code=%d msg=%s", envelope.Code, envelope.Msg)
	}
	return envelope.Data
}

func TestPhase33ClosureSilentRulesScopesTenantAndCorp(t *testing.T) {
	h, p := newClosureHandler()
	req := authenticatedClosureRequest(http.MethodGet, "/dashboard/silent-customer/rules?name=%E6%B2%89%E9%BB%98&page=1&perPage=50", nil)
	rec := httptest.NewRecorder()
	h.SilentRules(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	f, ok := p.filter.(SilentRuleFilter)
	if !ok {
		t.Fatalf("filter type=%T", p.filter)
	}
	if f.CorpID != 5 || f.TenantID != 23 || f.Name != "沉默" || f.Page != 1 || f.PerPage != 50 {
		t.Fatalf("filter=%+v", f)
	}
	if !strings.Contains(rec.Body.String(), "沉默30天") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestPhase33ClosureSaveSilentRuleWritesTenantCorp(t *testing.T) {
	h, p := newClosureHandler()
	req := authenticatedClosureRequest(http.MethodPost, "/dashboard/silent-customer/rules",
		strings.NewReader(`{"name":"沉默30天","silentDays":30,"status":"enabled"}`))
	rec := httptest.NewRecorder()
	h.SaveSilentRule(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if p.saved.TenantID != 23 || p.saved.CorpID != 5 || p.saved.SilentDays != 30 {
		t.Fatalf("saved=%+v", p.saved)
	}
	if id := envelopeData(t, rec.Body.String())["id"]; id != float64(11) {
		t.Fatalf("id=%v", id)
	}
}

func TestPhase33ClosureSaveSilentRuleRejectsInvalid(t *testing.T) {
	h, _ := newClosureHandler()
	req := authenticatedClosureRequest(http.MethodPost, "/dashboard/silent-customer/rules",
		strings.NewReader(`{"name":"","silentDays":0,"status":"enabled"}`))
	rec := httptest.NewRecorder()
	h.SaveSilentRule(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPhase33ClosureSaveSilentRuleBadJSON(t *testing.T) {
	h, _ := newClosureHandler()
	req := authenticatedClosureRequest(http.MethodPost, "/dashboard/silent-customer/rules", strings.NewReader(`{`))
	rec := httptest.NewRecorder()
	h.SaveSilentRule(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestPhase33ClosureEvaluateSilentReturnsMatchCount(t *testing.T) {
	h, _ := newClosureHandler()
	req := authenticatedClosureRequest(http.MethodPost, "/dashboard/silent-customer/evaluate",
		strings.NewReader(`{"customerId":"c1","customerName":"李雷","lastInteractionAt":"2026-08-01T00:00:00+08:00"}`))
	rec := httptest.NewRecorder()
	h.EvaluateSilent(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if matched := envelopeData(t, rec.Body.String())["matched"]; matched != float64(2) {
		t.Fatalf("matched=%v", matched)
	}
}

func TestPhase33ClosureActSilentRecords(t *testing.T) {
	h, _ := newClosureHandler()
	req := authenticatedClosureRequest(http.MethodPost, "/dashboard/silent-customer/records/action",
		strings.NewReader(`{"ids":[1,2],"action":"assign","assignedEmployeeId":9,"remark":"跟进"}`))
	rec := httptest.NewRecorder()
	h.ActSilent(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if updated := envelopeData(t, rec.Body.String())["updated"]; updated != float64(2) {
		t.Fatalf("updated=%v", updated)
	}
}

func TestPhase33ClosureRefuseRecordsScopesFilter(t *testing.T) {
	h, p := newClosureHandler()
	req := authenticatedClosureRequest(http.MethodGet, "/dashboard/refuse-archive/records?subject=%E5%BC%A0&authorizationStatus=refused&page=1&perPage=50", nil)
	rec := httptest.NewRecorder()
	h.RefuseRecords(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	f, ok := p.filter.(RefuseArchiveFilter)
	if !ok {
		t.Fatalf("filter type=%T", p.filter)
	}
	if f.CorpID != 5 || f.TenantID != 23 || f.Subject != "张" || f.AuthorizationStatus != "refused" {
		t.Fatalf("filter=%+v", f)
	}
}

func TestPhase33ClosureSyncRefuseUpserts(t *testing.T) {
	h, p := newClosureHandler()
	req := authenticatedClosureRequest(http.MethodPost, "/dashboard/refuse-archive/sync",
		strings.NewReader(`{"subjectType":"customer","subjectId":"c1","subjectName":"李雷","authorizationStatus":"refused"}`))
	rec := httptest.NewRecorder()
	h.SyncRefuse(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if p.refused.SubjectID != "c1" || p.refused.AuthorizationStatus != "refused" {
		t.Fatalf("refused=%+v", p.refused)
	}
	if id := envelopeData(t, rec.Body.String())["id"]; id != float64(21) {
		t.Fatalf("id=%v", id)
	}
}

func TestPhase33ClosureFollowRefuse(t *testing.T) {
	h, _ := newClosureHandler()
	req := authenticatedClosureRequest(http.MethodPost, "/dashboard/refuse-archive/follow-up",
		strings.NewReader(`{"id":1,"status":"contacted","note":"已电话沟通"}`))
	rec := httptest.NewRecorder()
	h.FollowRefuse(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if updated := envelopeData(t, rec.Body.String())["updated"]; updated != true {
		t.Fatalf("updated=%v", updated)
	}
}

func TestPhase33ClosurePermissionDenied(t *testing.T) {
	p := &closureProvider{tenantID: 23}
	h := NewPhase33ClosureHandler(p, staticCache("5-9"), riskHandlerResolver{}, denyAuthorizer{})
	req := authenticatedClosureRequest(http.MethodGet, "/dashboard/silent-customer/rules", nil)
	rec := httptest.NewRecorder()
	h.SilentRules(rec, req)
	body := decodeBody(t, rec.Body.Bytes())
	if rec.Code != http.StatusForbidden || body["errorCode"] != DashboardPermissionDeniedCode {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestValidateSilentRuleBounds(t *testing.T) {
	ok := SilentCustomerRule{Name: "规则", SilentDays: 30, Status: "enabled"}
	if err := ValidateSilentRule(ok); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	for _, v := range []SilentCustomerRule{
		{Name: "", SilentDays: 30, Status: "enabled"},
		{Name: "规则", SilentDays: 0, Status: "enabled"},
		{Name: "规则", SilentDays: 366, Status: "enabled"},
		{Name: "规则", SilentDays: 30, Status: "other"},
	} {
		if err := ValidateSilentRule(v); err == nil {
			t.Fatalf("expected error for %+v", v)
		}
	}
}

func TestValidateSilentActivityRequiresValidTime(t *testing.T) {
	if err := ValidateSilentActivity(SilentCustomerActivity{CustomerID: "c1", LastInteractionAt: "2026-08-01T00:00:00+08:00"}); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	if err := ValidateSilentActivity(SilentCustomerActivity{CustomerID: "", LastInteractionAt: "2026-08-01T00:00:00+08:00"}); err == nil {
		t.Fatal("expected customer id error")
	}
	if err := ValidateSilentActivity(SilentCustomerActivity{CustomerID: "c1", LastInteractionAt: "not-a-time"}); err == nil {
		t.Fatal("expected time error")
	}
}

func TestValidateRefuseArchiveRejectsBadSubject(t *testing.T) {
	ok := RefuseArchiveRecord{SubjectType: "customer", SubjectID: "c1", AuthorizationStatus: "refused"}
	if err := ValidateRefuseArchive(ok); err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	for _, v := range []RefuseArchiveRecord{
		{SubjectType: "robot", SubjectID: "c1", AuthorizationStatus: "refused"},
		{SubjectType: "customer", SubjectID: "", AuthorizationStatus: "refused"},
		{SubjectType: "customer", SubjectID: "c1", AuthorizationStatus: "maybe"},
	} {
		if err := ValidateRefuseArchive(v); err == nil {
			t.Fatalf("expected error for %+v", v)
		}
	}
}
