package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChannelCodeWorkspaceStatisticsReturnsVerifiedSummary(t *testing.T) {
	store := &fakeChannelCodeStatisticsStore{
		fakeChannelCodeStore: &fakeChannelCodeStore{users: map[int]User{1: {ID: 1}}},
		page: ChannelCodeStatisticsPage{
			Summary: ChannelCodeStatisticsSummary{
				AddedAttempts: 8, LostAttempts: 2, AddedCustomers: 6, RetainedCustomers: 5, CodeCount: 2,
				Available: true, AsOf: "2026-08-21T10:00:00+08:00", Timezone: "Asia/Shanghai",
				Definition: "按渠道码归因的客户关联状态变化",
			},
			Rows: []ChannelCodeStatisticsRow{{ID: 11, Name: "展会引流", AddedCustomers: 6}},
			Total: 1, TotalPage: 1, PerPage: 20,
		},
	}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCode/workspaceStatistics?startDate=2026-08-01&endDate=2026-08-21", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkspaceStatistics(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	data := body["data"].(map[string]any)
	summary := data["summary"].(map[string]any)
	if int(summary["addedCustomers"].(float64)) != 6 || summary["available"] != true {
		t.Fatalf("summary = %#v", summary)
	}
	if summary["timezone"] != "Asia/Shanghai" || !strings.Contains(summary["definition"].(string), "归因") {
		t.Fatalf("summary metadata = %#v", summary)
	}
}

func TestChannelCodeCSVSafeCellProtectsSpreadsheetFormulas(t *testing.T) {
	for _, value := range []string{"=SUM(A1)", "+cmd", "-cmd", "@cmd"} {
		if got := csvSafeCell(value); got[0] != '\'' {
			t.Fatalf("unsafe cell %q became %q", value, got)
		}
	}
	if got := csvSafeCell("普通名称"); got != "普通名称" {
		t.Fatalf("normal cell = %q", got)
	}
}

type fakeChannelCodeStatisticsStore struct {
	*fakeChannelCodeStore
	page ChannelCodeStatisticsPage
}

func (s *fakeChannelCodeStatisticsStore) ChannelCodeWorkspaceStatistics(_ context.Context, _ ChannelCodeStatisticsFilter) (ChannelCodeStatisticsPage, error) {
	return s.page, nil
}
