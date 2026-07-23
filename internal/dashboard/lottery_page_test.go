package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLotteryPageServesStandaloneConsole(t *testing.T) {
	handler := NewLotteryPageHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/lottery/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") || !strings.Contains(contentType, "charset=utf-8") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"MoChat Go 抽奖活动",
		"/dashboard/lottery/index",
		"/dashboard/lottery/show",
		"/dashboard/lottery/share",
		"/dashboard/lottery/showContact",
		"/dashboard/lottery/store",
		"/dashboard/lottery/update",
		"/dashboard/lottery/writeOff",
		"/dashboard/lottery/destroy",
		"mochat_go_lottery_token",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatalf("page should not depend on external assets")
	}
}

func TestLotteryPageRejectsPost(t *testing.T) {
	handler := NewLotteryPageHandler()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/lottery/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}
