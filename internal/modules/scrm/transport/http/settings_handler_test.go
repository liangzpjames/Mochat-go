package http

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jiyi/mochat-go/internal/modules/scrm/adapters/mysql"
)

type settingsPrincipal struct{}

func (settingsPrincipal) Resolve(*nethttp.Request) (Principal, error) {
	return Principal{UserID: 7, TenantID: 9, CorpID: 42}, nil
}

type settingsAuth struct{}

func (settingsAuth) Authorize(context.Context, Principal, int64, string) error { return nil }

type settingsRepo struct {
	got   mysql.SCRMSetting
	items []mysql.SCRMSetting
}

func (r *settingsRepo) List(context.Context, int64, int64, string) ([]mysql.SCRMSetting, error) {
	return r.items, nil
}

func TestSettingsGetReturnsActorAndUpdateTime(t *testing.T) {
	at := time.Date(2026, 8, 6, 8, 0, 0, 0, time.UTC)
	repo := &settingsRepo{items: []mysql.SCRMSetting{{ID: "s1", TenantID: 9, CorpID: 42, Type: "customer_source", Key: "web", UpdatedBy: 4, UpdatedAt: at, Version: 2}}}
	h := NewSettingsHandler(repo, settingsPrincipal{}, settingsAuth{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/scrm/settings?corpId=42", nil))
	if w.Code != 200 {
		t.Fatalf("status=%d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"updatedBy":4`) || !strings.Contains(w.Body.String(), "2026-08-06T08:00:00Z") {
		t.Fatalf("body=%s", w.Body.String())
	}
}
func (r *settingsRepo) Upsert(_ context.Context, s mysql.SCRMSetting, _ int64) (mysql.SCRMSetting, error) {
	r.got = s
	return s, nil
}

func TestSettingsPutAcceptsCorpIDFromJSONBody(t *testing.T) {
	repo := &settingsRepo{}
	h := NewSettingsHandler(repo, settingsPrincipal{}, settingsAuth{})
	body := `{"corpId":42,"type":"funnel","key":"stage","value":{"x":1},"enabled":true}`
	r := httptest.NewRequest("PUT", "/scrm/settings", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if repo.got.TenantID != 9 || repo.got.CorpID != 42 {
		t.Fatalf("scope=%+v", repo.got)
	}
	var envelope map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
}
