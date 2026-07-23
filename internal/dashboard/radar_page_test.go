package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRadarPageServesStandaloneConsole(t *testing.T) {
	handler := NewRadarPageHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/radar/page", nil)
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
		"MoChat Go 互动雷达",
		"/dashboard/radar/index",
		"/dashboard/radar/show",
		"/dashboard/radar/indexChannel",
		"/dashboard/radar/indexChannelLink",
		"/dashboard/radar/showChannel",
		"/dashboard/radar/showContact",
		"/dashboard/radar/store",
		"/dashboard/radar/storeChannel",
		"/dashboard/radar/storeChannelLink",
		"/dashboard/radar/update",
		"/dashboard/radar/destroy",
		"mochat_go_radar_token",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatalf("page should not depend on external assets")
	}
}

func TestRadarPageRejectsPost(t *testing.T) {
	handler := NewRadarPageHandler()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/radar/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}
