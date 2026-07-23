package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShopCodePageServesStandaloneConsole(t *testing.T) {
	handler := NewShopCodePageHandler()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/shopCode/page", nil)
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
		"MoChat Go 门店活码",
		"/dashboard/shopCode/index",
		"/dashboard/shopCode/info",
		"/dashboard/shopCode/location",
		"/dashboard/shopCode/addressKeyWordList",
		"/dashboard/shopCode/searchCity",
		"/dashboard/shopCode/share",
		"/dashboard/shopCode/pageInfo",
		"/dashboard/shopCode/pageSet",
		"/dashboard/shopCode/show",
		"/dashboard/shopCode/showContact",
		"/dashboard/shopCode/showShop",
		"/dashboard/shopCode/store",
		"/dashboard/shopCode/update",
		"/dashboard/shopCode/status",
		"/dashboard/shopCode/updateEmployee",
		"/dashboard/shopCode/updateQrcode",
		"/dashboard/shopCode/batchContactTags",
		"/dashboard/shopCode/destroy",
		"mochat_go_shop_code_token",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page missing %q", want)
		}
	}
	if strings.Contains(body, "http://") || strings.Contains(body, "https://") {
		t.Fatalf("page should not depend on external assets")
	}
}

func TestShopCodePageRejectsPost(t *testing.T) {
	handler := NewShopCodePageHandler()

	req := httptest.NewRequest(http.MethodPost, "/dashboard/shopCode/page", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", rec.Header().Get("Allow"))
	}
}
