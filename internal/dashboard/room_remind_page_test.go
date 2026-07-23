package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomRemindPageServesStandaloneConsole(t *testing.T) {
	handler := NewRoomRemindPageHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/roomRemind/page", nil)
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
		"MoChat Go 客户群提醒",
		"/dashboard/roomRemind/index",
		"/dashboard/roomRemind/store",
		"/dashboard/roomRemind/status",
		"/dashboard/roomRemind/destroy",
		"/dashboard/task/roomRemind",
		"mochat_go_room_remind_token",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatalf("page should not depend on external assets")
	}
}

func TestRoomRemindPageRejectsPost(t *testing.T) {
	handler := NewRoomRemindPageHandler()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/roomRemind/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}
