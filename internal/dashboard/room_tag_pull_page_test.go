package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomTagPullContactDetailPageServesStandaloneConsole(t *testing.T) {
	handler := NewRoomTagPullContactDetailPageHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/roomTagPull/contactDetail?id=917001", nil)
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
		"MoChat Go 标签建群客户明细",
		"/dashboard/roomTagPull/show",
		"/dashboard/roomTagPull/showContact",
		"mochat_go_room_tag_pull_token",
		"客户明细",
		"员工任务",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatalf("page should not depend on external assets")
	}
}

func TestRoomTagPullContactDetailPageRejectsPost(t *testing.T) {
	handler := NewRoomTagPullContactDetailPageHandler()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/roomTagPull/contactDetail", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}
