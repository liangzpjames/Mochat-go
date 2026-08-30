package dashboard

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/saascompliance"
)

type fakeComplianceExportOpener struct {
	events *[]string
}

func (f fakeComplianceExportOpener) OpenExport(context.Context, int64, saascompliance.Actor) (saascompliance.ExportDownload, error) {
	*f.events = append(*f.events, "artifact")
	return saascompliance.ExportDownload{Filename: "compliance.tar.gz", Reader: io.NopCloser(strings.NewReader("compliance-body"))}, nil
}

func TestComplianceExportDownloadClearsDeadlineAfterArtifactBeforeBody(t *testing.T) {
	events := []string{}
	store := &fakeSaaSAdminStore{users: map[int]User{7: {ID: 7, TenantID: 1, IsSuperAdmin: 1}}}
	handler := NewSaaSAdminHandler(store, HeaderUserIDResolver{}, 1).withComplianceExportOpener(fakeComplianceExportOpener{events: &events})
	req := httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/complianceExportDownload?exportId=9", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "7")
	recorder := &orderedDeadlineRecorder{ResponseRecorder: httptest.NewRecorder(), events: &events}
	handler.ComplianceExportDownload(recorder, req)
	if strings.Join(events, ",") != "artifact,deadline,body" || recorder.Code != http.StatusOK {
		t.Fatalf("events=%v status=%d body=%q", events, recorder.Code, recorder.Body.String())
	}

	unauthorizedEvents := []string{}
	unauthorized := &orderedDeadlineRecorder{ResponseRecorder: httptest.NewRecorder(), events: &unauthorizedEvents}
	handler.ComplianceExportDownload(unauthorized, httptest.NewRequest(http.MethodGet, "/dashboard/saasAdmin/complianceExportDownload?exportId=9", nil))
	if strings.Contains(strings.Join(unauthorizedEvents, ","), "deadline") {
		t.Fatalf("unauthorized response cleared deadline: %v", unauthorizedEvents)
	}
}
