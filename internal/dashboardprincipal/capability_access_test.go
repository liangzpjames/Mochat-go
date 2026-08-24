package dashboardprincipal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestVisibleProviderCapabilitiesUsesExplicitPermissionMapping(t *testing.T) {
	principal := DashboardPrincipal{IsSuperAdmin: false}
	ctx := WithCapabilityAccess(context.Background(), false, []string{"dashboard.acquisition.precise_group_send"})
	got := VisibleProviderCapabilities(ctx, principal)
	want := []string{"contact_batch_send", "room_batch_send"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("capabilities=%#v, want %#v", got, want)
	}
	unknown := WithCapabilityAccess(context.Background(), false, []string{"/dashboard/contactMessageBatchSend/index#get", "dashboard.unknown"})
	if got := VisibleProviderCapabilities(unknown, principal); len(got) != 0 {
		t.Fatalf("unknown permission exposed capabilities=%#v", got)
	}
}

func TestHasPermissionCodeFailsClosedForOrdinaryPrincipal(t *testing.T) {
	ordinary := DashboardPrincipal{}
	granted := WithCapabilityAccess(context.Background(), false, []string{"dashboard.company_setting.website"})
	if !HasPermissionCode(granted, ordinary, "dashboard.company_setting.website") {
		t.Fatal("exact permission code was not recognized")
	}
	for name, ctx := range map[string]context.Context{
		"missing context": context.Background(),
		"unknown code":    granted,
		"spoofed admin":   WithCapabilityAccess(context.Background(), true, []string{"dashboard.company_setting.website"}),
	} {
		code := "dashboard.company_setting.website"
		if name == "unknown code" {
			code = "dashboard.company_setting.unknown"
		}
		if HasPermissionCode(ctx, ordinary, code) {
			t.Fatalf("%s unexpectedly authorized", name)
		}
	}
	if !HasPermissionCode(context.Background(), DashboardPrincipal{IsSuperAdmin: true}, "dashboard.company_setting.website") {
		t.Fatal("real superadmin was not authorized")
	}
}

func TestProviderPageCapabilityMappingUsesCatalogPageCodes(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(sourceFile), "..", "dashboard", "dashboard_page_catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Pages []struct {
			Code string `json:"code"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(body, &catalog); err != nil {
		t.Fatal(err)
	}
	pageCodes := make(map[string]struct{}, len(catalog.Pages))
	for _, page := range catalog.Pages {
		pageCodes[page.Code] = struct{}{}
	}
	for pageCode := range ProviderPageCapabilityMapping {
		if _, ok := pageCodes[pageCode]; !ok {
			t.Fatalf("mapping page code %q is absent from dashboard_page_catalog.json", pageCode)
		}
	}
}
