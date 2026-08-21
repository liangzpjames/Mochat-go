package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChannelCodeIndexReturnsWorkspaceMetadata(t *testing.T) {
	store := &fakeChannelCodeStore{
		users: map[int]User{1: {ID: 1}},
		channelPage: ChannelCodeListPage{
			Items: []ChannelCodeListItem{{
				ID:        900001,
				GroupID:   7,
				GroupName: "华东",
				Name:      "展会引流",
				QRCodeURL: "uploads/channel/900001.png",
			}},
			Total: 1, TotalPage: 1, PerPage: 20,
		},
	}
	handler := NewChannelCodeHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{})
	req := authenticatedDashboardRequestForTest(http.MethodGet, "/dashboard/channelCode/index?name=展会&creator=李娜&employeeId=99&state=active", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Index(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	rows := body["data"].(map[string]any)["list"].([]any)
	row := rows[0].(map[string]any)
	if _, ok := row["creator"]; !ok {
		t.Fatalf("creator metadata missing: %#v", row)
	}
	if _, ok := row["createdAt"]; !ok {
		t.Fatalf("createdAt metadata missing: %#v", row)
	}
}

func (s *fakeChannelCodeStore) ChannelCodeWorkspacePage(_ context.Context, filter ChannelCodeWorkspaceFilter) (ChannelCodeListPage, error) {
	s.channelFilter = ChannelCodeListFilter{Name: filter.Name, Page: filter.Page, PerPage: filter.PerPage}
	return s.channelPage, nil
}
