package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkContactUpdateWritesProfileAndSyncsWeCom(t *testing.T) {
	store := &fakeWorkReadStore{
		users:                  map[int]User{1: {ID: 1}},
		workContactUpdateFound: true,
		workContactUpdateResult: WorkContactUpdateResult{
			WXUserID:         "go-user-1",
			WXExternalUserID: "external-user-1",
			AddedWXTagIDs:    []string{"wx-tag-2"},
		},
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomWelcomeCredentialFound: true,
	}
	authorizer := &recordingAuthorizer{}
	client := &fakeWorkContactUpdateClient{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", authorizer).
		WithWorkContactUpdateClient(client)

	req := httptest.NewRequest(http.MethodPut, "/dashboard/workContact/update", strings.NewReader(`{"contactId":21,"employeeId":99,"remark":"新备注","description":"新描述","businessNo":"B-2","tag":[2,2,0]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if authorizer.permissionKey != "/dashboard/workContact/update#put" || authorizer.corpID != 7 || authorizer.workEmployeeID != 88 {
		t.Fatalf("authorizer = %#v", authorizer)
	}
	values := store.lastWorkContactUpdate
	if values.CorpID != 7 || values.ContactID != 21 || values.EmployeeID != 99 || !values.HasTag {
		t.Fatalf("values = %#v", values)
	}
	if values.Remark == nil || *values.Remark != "新备注" || values.Description == nil || *values.Description != "新描述" || values.BusinessNo == nil || *values.BusinessNo != "B-2" {
		t.Fatalf("string values = %#v", values)
	}
	if len(values.TagIDs) != 1 || values.TagIDs[0] != 2 {
		t.Fatalf("tag ids = %#v", values.TagIDs)
	}
	if store.lastCredentialCorpID != 7 {
		t.Fatalf("credential corp = %d", store.lastCredentialCorpID)
	}
	if client.remarkCalls != 1 || client.markTagCalls != 1 {
		t.Fatalf("client calls = remark %d mark %d", client.remarkCalls, client.markTagCalls)
	}
	if client.remarkPayload.UserID != "go-user-1" || client.remarkPayload.ExternalUserID != "external-user-1" {
		t.Fatalf("remark payload = %#v", client.remarkPayload)
	}
	if client.remarkPayload.Remark == nil || *client.remarkPayload.Remark != "新备注" || client.remarkPayload.Description == nil || *client.remarkPayload.Description != "新描述" {
		t.Fatalf("remark strings = %#v", client.remarkPayload)
	}
	if len(client.markPayload.AddTag) != 1 || client.markPayload.AddTag[0] != "wx-tag-2" {
		t.Fatalf("mark payload = %#v", client.markPayload)
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["code"].(float64) != 200 {
		t.Fatalf("body = %#v", body)
	}
}

func TestWorkContactUpdateMissingRelationReturnsSuccessWithoutWeCom(t *testing.T) {
	store := &fakeWorkReadStore{
		users: map[int]User{1: {ID: 1}},
	}
	client := &fakeWorkContactUpdateClient{}
	handler := NewWorkReadHandlerWithAuthorizer(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "", &recordingAuthorizer{}).
		WithWorkContactUpdateClient(client)

	req := httptest.NewRequest(http.MethodPut, "/dashboard/workContact/update", strings.NewReader(`{"contactId":21,"employeeId":99,"remark":"新备注"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if client.remarkCalls != 0 || client.markTagCalls != 0 || store.lastCredentialCorpID != 0 {
		t.Fatalf("unexpected side effects: client=%#v store=%#v", client, store)
	}
}

func TestWorkContactUpdateValidatesRequiredIDs(t *testing.T) {
	handler := NewWorkReadHandlerWithAuthorizer(
		&fakeWorkReadStore{users: map[int]User{1: {ID: 1}}},
		staticAdminCache("7-88"),
		HeaderUserIDResolver{},
		"",
		&recordingAuthorizer{},
	)

	for _, tc := range []struct {
		name string
		body string
		msg  string
	}{
		{name: "contact", body: `{"employeeId":99}`, msg: "客户id必传"},
		{name: "employee", body: `{"contactId":21}`, msg: "员工id必传"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/dashboard/workContact/update", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()
			handler.WorkContactUpdate(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
			body := decodeBody(t, rec.Body.Bytes())
			if body["msg"] != tc.msg {
				t.Fatalf("msg = %#v", body["msg"])
			}
		})
	}
}

func TestSidebarWorkContactUpdateUsesSidebarEmployee(t *testing.T) {
	store := &fakeWorkReadStore{
		sidebarEmployees:       map[int]SidebarEmployee{5: {ID: 5, CorpID: 7, LogUserID: 1}},
		workContactUpdateFound: true,
		workContactUpdateResult: WorkContactUpdateResult{
			WXUserID:         "go-user-5",
			WXExternalUserID: "external-user-1",
			AddedWXTagIDs:    []string{"wx-tag-2"},
		},
		roomWelcomeCredential:      RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "ww-go", ContactSecret: "contact-secret"},
		roomWelcomeCredentialFound: true,
	}
	client := &fakeWorkContactUpdateClient{}
	handler := NewWorkReadHandler(store, nil, HeaderUserIDResolver{}, "").
		WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}).
		WithWorkContactUpdateClient(client)

	req := httptest.NewRequest(http.MethodPut, "/sidebar/workContact/update", strings.NewReader(`{"contactId":21,"employeeId":99,"remark":"侧边栏备注","tag":[2]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkContactUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	values := store.lastWorkContactUpdate
	if values.CorpID != 7 || values.EmployeeID != 5 || values.ContactID != 21 {
		t.Fatalf("values = %#v", values)
	}
	if values.Remark == nil || *values.Remark != "侧边栏备注" {
		t.Fatalf("remark = %#v", values.Remark)
	}
	if client.remarkCalls != 1 || client.markTagCalls != 1 {
		t.Fatalf("client calls = remark %d mark %d", client.remarkCalls, client.markTagCalls)
	}
	if store.lastCredentialCorpID != 7 {
		t.Fatalf("credential corp = %d", store.lastCredentialCorpID)
	}
}

func TestSidebarWorkContactUpdateRequiresContactID(t *testing.T) {
	handler := NewWorkReadHandler(
		&fakeWorkReadStore{sidebarEmployees: map[int]SidebarEmployee{5: {ID: 5, CorpID: 7}}},
		nil,
		HeaderUserIDResolver{},
		"",
	).WithSidebarEmployeeResolver(HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"})

	req := httptest.NewRequest(http.MethodPut, "/sidebar/workContact/update", strings.NewReader(`{"remark":"侧边栏备注"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-Employee-ID", "5")
	rec := httptest.NewRecorder()
	handler.SidebarWorkContactUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["msg"] != "客户id必传" {
		t.Fatalf("msg = %#v", body["msg"])
	}
}

type fakeWorkContactUpdateClient struct {
	remarkCalls   int
	markTagCalls  int
	remarkPayload WorkContactRemarkPayload
	markPayload   WorkContactMarkTagsPayload
	credential    RoomWelcomeCorpCredential
}

func (c *fakeWorkContactUpdateClient) UpdateExternalContactRemark(_ context.Context, credential RoomWelcomeCorpCredential, payload WorkContactRemarkPayload) error {
	c.remarkCalls++
	c.credential = credential
	c.remarkPayload = payload
	return nil
}

func (c *fakeWorkContactUpdateClient) MarkExternalContactTags(_ context.Context, credential RoomWelcomeCorpCredential, payload WorkContactMarkTagsPayload) error {
	c.markTagCalls++
	c.credential = credential
	c.markPayload = payload
	return nil
}
