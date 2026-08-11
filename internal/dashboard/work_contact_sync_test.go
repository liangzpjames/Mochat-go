package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkContactSyncPullsContactsAndStoresWithRBAC(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1}},
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomWelcomeCredentialFound: true,
		workContactSyncEmployees: []WorkContactSyncEmployee{
			{ID: 11, WXUserID: "zhangsan"},
			{ID: 12, WXUserID: "lisi"},
		},
	}
	client := &fakeWorkContactSyncClient{
		lists: map[string][]string{
			"zhangsan": {"external-1", "external-2", "external-1"},
		},
		noContact: map[string]bool{"lisi": true},
		details: map[string]WorkContactSyncContact{
			"external-1": {
				WXExternalUserID: "external-1",
				Name:             "客户一",
				FollowUsers: []WorkContactSyncFollowUser{{
					UserID: "zhangsan",
					Tags:   []WorkContactSyncTag{{WXContactTagID: "tag-1", GroupName: "阶段", TagName: "高意向", Type: 1}},
				}},
			},
			"external-2": {
				WXExternalUserID: "external-2",
				Name:             "客户二",
				FollowUsers:      []WorkContactSyncFollowUser{{UserID: "zhangsan"}},
			},
		},
	}
	authorizer := &recordingAuthorizer{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", authorizer).
		WithWorkContactSyncClient(client)

	req := authenticatedDashboardRequestForTestAs(http.MethodPut, "/dashboard/workContact/synContact", nil, 1, 1, 7, 88)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactSync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workContact/synContact#put" || authorizer.corpID != 7 || authorizer.workEmployeeID != 88 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	if client.credential.WXCorpID != "ww-go" || client.credential.ContactSecret != "contact-secret" {
		t.Fatalf("credential = %#v", client.credential)
	}
	if got := client.listUsers; len(got) != 2 || got[0] != "zhangsan" || got[1] != "lisi" {
		t.Fatalf("list users = %#v", got)
	}
	if got := client.detailIDs; len(got) != 2 || got[0] != "external-1" || got[1] != "external-2" {
		t.Fatalf("detail ids = %#v", got)
	}
	if store.lastWorkContactSyncCorpID != 7 || len(store.lastWorkContactSyncBundles) != 2 {
		t.Fatalf("sync call = corp %d bundles %#v", store.lastWorkContactSyncCorpID, store.lastWorkContactSyncBundles)
	}
	first := store.lastWorkContactSyncBundles[0]
	if first.Employee.ID != 11 || len(first.ExternalUserIDs) != 2 || len(first.Contacts) != 2 || first.NoContact {
		t.Fatalf("first bundle = %#v", first)
	}
	second := store.lastWorkContactSyncBundles[1]
	if second.Employee.ID != 12 || !second.NoContact || len(second.Contacts) != 0 {
		t.Fatalf("second bundle = %#v", second)
	}
}

func TestWorkContactSyncReturnsSaaSQuotaExceeded(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1, TenantID: 9}},
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomWelcomeCredentialFound: true,
		workContactSyncEmployees:   []WorkContactSyncEmployee{{ID: 11, WXUserID: "zhangsan"}},
		workContactSyncErr: NewSaaSQuotaExceededError(SaaSQuotaStatus{
			Metric:     SaaSMetricContacts,
			TenantID:   9,
			Current:    10,
			Limit:      10,
			Additional: 1,
		}),
	}
	client := &fakeWorkContactSyncClient{
		lists: map[string][]string{
			"zhangsan": {"external-1"},
		},
		details: map[string]WorkContactSyncContact{
			"external-1": {WXExternalUserID: "external-1", FollowUsers: []WorkContactSyncFollowUser{{UserID: "zhangsan"}}},
		},
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", &recordingAuthorizer{}).
		WithWorkContactSyncClient(client)

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workContact/synContact", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactSync(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "套餐额度已达上限：客户数 10/10" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

func TestWorkContactSyncRequiresDashboardPrincipal(t *testing.T) {
	store := &fakeWorkReadStore{users: map[int]User{1: {ID: 1}}}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88,8-99"), HeaderUserIDResolver{}, "", &recordingAuthorizer{}).
		WithWorkContactSyncClient(&fakeWorkContactSyncClient{})

	req := httptest.NewRequest(http.MethodPut, "/dashboard/workContact/synContact", nil)
	rec := httptest.NewRecorder()
	handler.WorkContactSync(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkContactSyncRequiresEmployees(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                      map[int]User{1: {ID: 1}},
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomWelcomeCredentialFound: true,
	}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", &recordingAuthorizer{}).
		WithWorkContactSyncClient(&fakeWorkContactSyncClient{})

	req := authenticatedDashboardRequestForTest(http.MethodPut, "/dashboard/workContact/synContact", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactSync(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "查询不到有效的企业微信成员信息" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

type fakeWorkContactSyncClient struct {
	credential RoomWelcomeCorpCredential
	lists      map[string][]string
	noContact  map[string]bool
	details    map[string]WorkContactSyncContact
	listUsers  []string
	detailIDs  []string
}

func (c *fakeWorkContactSyncClient) ExternalContactList(_ context.Context, credential RoomWelcomeCorpCredential, wxUserID string) ([]string, bool, error) {
	c.credential = credential
	c.listUsers = append(c.listUsers, wxUserID)
	return c.lists[wxUserID], c.noContact[wxUserID], nil
}

func (c *fakeWorkContactSyncClient) ExternalContactDetail(_ context.Context, credential RoomWelcomeCorpCredential, wxExternalUserID string) (WorkContactSyncContact, error) {
	c.credential = credential
	c.detailIDs = append(c.detailIDs, wxExternalUserID)
	return c.details[wxExternalUserID], nil
}

var _ WorkContactSyncClient = (*fakeWorkContactSyncClient)(nil)
