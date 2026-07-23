package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContactSOPPageServesStandaloneConsole(t *testing.T) {
	handler := NewContactSOPPageHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/contactSop/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := assertSOPPageBasics(t, rec)
	for _, want := range []string{
		"MoChat Go 个人 SOP",
		"/dashboard/contactSop/index",
		"/dashboard/contactSop/info",
		"/dashboard/contactSop/store",
		"/dashboard/contactSop/update",
		"/dashboard/contactSop/setEmployee",
		"/dashboard/contactSop/state",
		"/dashboard/contactSop/destroy",
		"mochat_go_contact_sop_token",
		"contactSopId",
		"employeeIds",
		"contactIds",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
}

func TestRoomSOPPageServesStandaloneConsole(t *testing.T) {
	handler := NewRoomSOPPageHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/roomSop/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := assertSOPPageBasics(t, rec)
	for _, want := range []string{
		"MoChat Go 群 SOP",
		"/dashboard/roomSop/index",
		"/dashboard/roomSop/info",
		"/dashboard/roomSop/store",
		"/dashboard/roomSop/update",
		"/dashboard/roomSop/setRoom",
		"/dashboard/roomSop/state",
		"/dashboard/roomSop/destroy",
		"mochat_go_room_sop_token",
		"roomSopId",
		"roomIds",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
}

func TestSOPPagesRejectPost(t *testing.T) {
	for name, handler := range map[string]http.Handler{
		"contact": NewContactSOPPageHandler(),
		"room":    NewRoomSOPPageHandler(),
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/dashboard/contactSop/page", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
			}
			if rec.Header().Get("Allow") != "GET, HEAD" {
				t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
			}
		})
	}
}

func assertSOPPageBasics(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") || !strings.Contains(contentType, "charset=utf-8") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	body := rec.Body.String()
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatalf("page should not depend on external assets")
	}
	return body
}
