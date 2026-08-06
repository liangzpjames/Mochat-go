package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type interceptProvider struct {
	tenantID int
	filter   any
	library  KeywordLibrary
	entry    KeywordEntry
	rule     MessageInterceptRule
	decision MessageInterceptDecision
}

func (p *interceptProvider) TenantIDByCorpID(context.Context, int) (int, error) {
	return p.tenantID, nil
}

func (p *interceptProvider) KeywordLibraryPage(_ context.Context, f KeywordLibraryFilter) (KeywordLibraryPage, error) {
	p.filter = f
	return KeywordLibraryPage{Items: []KeywordLibrary{{ID: 1, Name: "营销词库", MatchMode: "contains", Status: "enabled", PublishedVersion: 2}}, Total: 1, Page: f.Page, PerPage: f.PerPage}, nil
}
func (p *interceptProvider) KeywordEntryPage(_ context.Context, f KeywordEntryFilter) (KeywordEntryPage, error) {
	p.filter = f
	return KeywordEntryPage{Items: []KeywordEntry{{ID: 1, LibraryID: 1, Keyword: "加微信", Status: "enabled"}}, Total: 1}, nil
}
func (p *interceptProvider) MessageInterceptRulePage(_ context.Context, f MessageInterceptRuleFilter) (MessageInterceptRulePage, error) {
	p.filter = f
	return MessageInterceptRulePage{Items: []MessageInterceptRule{{ID: 1, Name: "营销拦截", LibraryID: 1, LibraryVersion: 2, Decision: "blocked", Status: "enabled"}}, Total: 1}, nil
}
func (p *interceptProvider) MessageInterceptRecordPage(_ context.Context, f MessageInterceptRecordFilter) (MessageInterceptRecordPage, error) {
	p.filter = f
	return MessageInterceptRecordPage{Items: []MessageInterceptRecord{{ID: 1, RuleName: "营销拦截", MessageContent: "加微信", Decision: "blocked", AuditStatus: "pending"}}, Total: 1}, nil
}

func (p *interceptProvider) SaveKeywordLibrary(_ context.Context, v KeywordLibrary) (int64, error) {
	if err := ValidateKeywordLibrary(v); err != nil {
		return 0, err
	}
	p.library = v
	return 9, nil
}
func (p *interceptProvider) SetKeywordLibraryStatus(context.Context, int, int, int64, string) (bool, error) {
	return true, nil
}
func (p *interceptProvider) DeleteKeywordLibrary(context.Context, int, int, int64) (bool, error) {
	return true, nil
}
func (p *interceptProvider) SaveKeywordEntry(_ context.Context, _ int, _ int, v KeywordEntry) (int64, error) {
	if err := ValidateKeywordEntry(v); err != nil {
		return 0, err
	}
	p.entry = v
	return 8, nil
}
func (p *interceptProvider) SetKeywordEntryStatus(context.Context, int, int, int64, string) (bool, error) {
	return true, nil
}
func (p *interceptProvider) DeleteKeywordEntry(context.Context, int, int, int64) (bool, error) {
	return true, nil
}
func (p *interceptProvider) PublishKeywordLibrary(context.Context, int, int, int64, int64) (int, error) {
	return 3, nil
}
func (p *interceptProvider) SaveMessageInterceptRule(_ context.Context, v MessageInterceptRule) (int64, error) {
	if err := ValidateMessageInterceptRule(v); err != nil {
		return 0, err
	}
	p.rule = v
	return 7, nil
}
func (p *interceptProvider) SetMessageInterceptRuleStatus(context.Context, int, int, int64, string) (bool, error) {
	return true, nil
}
func (p *interceptProvider) DeleteMessageInterceptRule(context.Context, int, int, int64) (bool, error) {
	return true, nil
}
func (p *interceptProvider) EvaluateMessageIntercept(context.Context, int, int, MessageInterceptEvaluation) (MessageInterceptDecision, error) {
	p.decision = MessageInterceptDecision{Decision: "blocked", Matched: true, MatchedKeywords: []string{"加微信"}, Explanation: "命中营销词库"}
	return p.decision, nil
}
func (p *interceptProvider) AuditMessageInterceptRecords(context.Context, int, int, int64, []int64, string, string) (int64, error) {
	return 1, nil
}

func newInterceptHandler() (*MessageInterceptHandler, *interceptProvider) {
	p := &interceptProvider{tenantID: 23}
	return NewMessageInterceptHandler(p, staticCache("5-9"), riskHandlerResolver{}, nil), p
}

func TestMessageInterceptLibrariesScopesTenantAndCorp(t *testing.T) {
	h, p := newInterceptHandler()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/keyword-library/libraries?name=%E8%90%A5%E9%94%80&page=1&perPage=50", nil)
	rec := httptest.NewRecorder()
	h.Libraries(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	f, ok := p.filter.(KeywordLibraryFilter)
	if !ok {
		t.Fatalf("filter type=%T", p.filter)
	}
	if f.CorpID != 5 || f.TenantID != 23 || f.Name != "营销" || f.Page != 1 || f.PerPage != 50 {
		t.Fatalf("filter=%+v", f)
	}
	if !strings.Contains(rec.Body.String(), "营销词库") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestMessageInterceptSaveLibraryPersistsScope(t *testing.T) {
	h, p := newInterceptHandler()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/keyword-library/libraries",
		strings.NewReader(`{"name":"营销词库","description":"营销关键词","matchMode":"contains","status":"enabled"}`))
	rec := httptest.NewRecorder()
	h.SaveLibrary(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if p.library.TenantID != 23 || p.library.CorpID != 5 || p.library.Name != "营销词库" {
		t.Fatalf("library=%+v", p.library)
	}
	if id := envelopeData(t, rec.Body.String())["id"]; id != float64(9) {
		t.Fatalf("id=%v", id)
	}
}

func TestMessageInterceptSaveLibraryRejectsInvalid(t *testing.T) {
	h, _ := newInterceptHandler()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/keyword-library/libraries",
		strings.NewReader(`{"name":"","matchMode":"contains","status":"enabled"}`))
	rec := httptest.NewRecorder()
	h.SaveLibrary(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMessageInterceptPublishLibrary(t *testing.T) {
	h, _ := newInterceptHandler()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/keyword-library/libraries/publish",
		strings.NewReader(`{"id":1}`))
	rec := httptest.NewRecorder()
	h.PublishLibrary(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if version := envelopeData(t, rec.Body.String())["version"]; version != float64(3) {
		t.Fatalf("version=%v", version)
	}
}

func TestMessageInterceptSaveEntry(t *testing.T) {
	h, p := newInterceptHandler()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/keyword-library/entries",
		strings.NewReader(`{"libraryId":1,"keyword":"加微信","status":"enabled"}`))
	rec := httptest.NewRecorder()
	h.SaveEntry(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if p.entry.LibraryID != 1 || p.entry.Keyword != "加微信" {
		t.Fatalf("entry=%+v", p.entry)
	}
	if id := envelopeData(t, rec.Body.String())["id"]; id != float64(8) {
		t.Fatalf("id=%v", id)
	}
}

func TestMessageInterceptSaveRulePersistsScope(t *testing.T) {
	h, p := newInterceptHandler()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/message-intercept/rules",
		strings.NewReader(`{"name":"营销拦截","libraryId":1,"libraryVersion":2,"conversationScopes":["single","group"],"decision":"blocked","status":"enabled"}`))
	rec := httptest.NewRecorder()
	h.SaveRule(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if p.rule.TenantID != 23 || p.rule.CorpID != 5 || p.rule.LibraryVersion != 2 {
		t.Fatalf("rule=%+v", p.rule)
	}
	if id := envelopeData(t, rec.Body.String())["id"]; id != float64(7) {
		t.Fatalf("id=%v", id)
	}
}

func TestMessageInterceptEvaluateReturnsDecision(t *testing.T) {
	h, _ := newInterceptHandler()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/message-intercept/evaluate",
		strings.NewReader(`{"conversationType":"single","conversationId":"c1","messageId":"m1","senderId":"e1","senderName":"张三","content":"加微信","occurredAt":"2026-08-01T10:00:00+08:00"}`))
	rec := httptest.NewRecorder()
	h.Evaluate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"decision":"blocked"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestMessageInterceptRecordsScopesFilter(t *testing.T) {
	h, p := newInterceptHandler()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/message-intercept/records?keyword=%E5%8A%A0&decision=blocked&ruleId=1&page=1&perPage=20", nil)
	rec := httptest.NewRecorder()
	h.Records(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	f, ok := p.filter.(MessageInterceptRecordFilter)
	if !ok {
		t.Fatalf("filter type=%T", p.filter)
	}
	if f.CorpID != 5 || f.TenantID != 23 || f.Keyword != "加" || f.Decision != "blocked" || f.RuleID != 1 {
		t.Fatalf("filter=%+v", f)
	}
}

func TestMessageInterceptAudit(t *testing.T) {
	h, _ := newInterceptHandler()
	req := httptest.NewRequest(http.MethodPost, "/dashboard/message-intercept/records/audit",
		strings.NewReader(`{"ids":[1,2],"action":"confirmed","remark":"复核通过"}`))
	rec := httptest.NewRecorder()
	h.Audit(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if updated := envelopeData(t, rec.Body.String())["updated"]; updated != float64(1) {
		t.Fatalf("updated=%v", updated)
	}
}

func TestMessageInterceptPermissionDenied(t *testing.T) {
	p := &interceptProvider{tenantID: 23}
	h := NewMessageInterceptHandler(p, staticCache("5-9"), riskHandlerResolver{}, denyAuthorizer{})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/keyword-library/libraries", nil)
	rec := httptest.NewRecorder()
	h.Libraries(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestValidateKeywordLibraryAndEntry(t *testing.T) {
	okLibrary := KeywordLibrary{Name: "词库", MatchMode: "contains", Status: "enabled"}
	if err := ValidateKeywordLibrary(okLibrary); err != nil {
		t.Fatalf("expected valid library: %v", err)
	}
	for _, v := range []KeywordLibrary{
		{Name: "", MatchMode: "contains", Status: "enabled"},
		{Name: "词库", MatchMode: "regex", Status: "enabled"},
		{Name: "词库", MatchMode: "contains", Status: "other"},
	} {
		if err := ValidateKeywordLibrary(v); err == nil {
			t.Fatalf("expected library error for %+v", v)
		}
	}
	okEntry := KeywordEntry{LibraryID: 1, Keyword: "加微信", Status: "enabled"}
	if err := ValidateKeywordEntry(okEntry); err != nil {
		t.Fatalf("expected valid entry: %v", err)
	}
	for _, v := range []KeywordEntry{
		{LibraryID: 0, Keyword: "加微信", Status: "enabled"},
		{LibraryID: 1, Keyword: "", Status: "enabled"},
		{LibraryID: 1, Keyword: "加微信", Status: "other"},
	} {
		if err := ValidateKeywordEntry(v); err == nil {
			t.Fatalf("expected entry error for %+v", v)
		}
	}
}
