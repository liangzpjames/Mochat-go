package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContactTagSyncPullsWeComTagsAndStores(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1}},
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomWelcomeCredentialFound: true,
	}
	client := &fakeContactTagSyncClient{groups: []WorkContactTagSyncGroup{{
		WXGroupID: "et-group-1",
		GroupName: "重点客户",
		Order:     2,
		Tags: []WorkContactTagSyncTag{
			{WXContactTagID: "et-tag-1", Name: "高意向", Order: 10},
		},
	}}}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", authorizer).
		WithWorkContactTagSyncClient(client)

	req := authenticatedDashboardRequestForTestAs(http.MethodPut, "/dashboard/workContactTag/synContactTag", nil, 1, 1, 7, 88)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagSync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workContactTag/synContactTag#put" || authorizer.corpID != 7 || authorizer.workEmployeeID != 88 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if client.credential.WXCorpID != "ww-go" || client.credential.ContactSecret != "contact-secret" {
		t.Fatalf("credential = %#v", client.credential)
	}
	if store.lastContactTagSyncCorpID != 7 || len(store.lastContactTagSyncGroups) != 1 {
		t.Fatalf("sync call = corp %d groups %#v", store.lastContactTagSyncCorpID, store.lastContactTagSyncGroups)
	}
	if tag := store.lastContactTagSyncGroups[0].Tags[0]; tag.WXContactTagID != "et-tag-1" || tag.Name != "高意向" {
		t.Fatalf("tag = %#v", tag)
	}
}

func TestContactTagSyncRequiresDashboardPrincipal(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88,8-99"), HeaderUserIDResolver{}, "", &recordingAuthorizer{}).
		WithWorkContactTagSyncClient(&fakeContactTagSyncClient{})

	req := httptest.NewRequest(http.MethodPut, "/dashboard/workContactTag/synContactTag", nil)
	rec := httptest.NewRecorder()
	handler.ContactTagSync(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestContactTagSyncRequiresCorpCredential(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", &recordingAuthorizer{}).
		WithWorkContactTagSyncClient(&fakeContactTagSyncClient{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workContactTag/synContactTag", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.ContactTagSync(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "企业授权信息错误" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

type fakeContactTagSyncClient struct {
	groups     []WorkContactTagSyncGroup
	credential RoomWelcomeCorpCredential
	err        error
}

func (c *fakeContactTagSyncClient) CorpTags(_ context.Context, credential RoomWelcomeCorpCredential) ([]WorkContactTagSyncGroup, error) {
	c.credential = credential
	if c.err != nil {
		return nil, c.err
	}
	return c.groups, nil
}

var _ WorkContactTagSyncClient = (*fakeContactTagSyncClient)(nil)
