package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContactSOPTipInfoReturnsSidebarReminderPayload(t *testing.T) {
	store := &fakeContactSOPStore{
		employee: SidebarEmployee{ID: 7, CorpID: 9, LogUserID: 3},
		tips: []ContactSOPItem{{
			ID:           55,
			ContactSOPID: 66,
			Creator:      "管理员",
			Time:         "2026-07-04 10:30:00",
			TipTime:      "10:30",
			TaskRaw:      `{"content":[{"type":"text","value":"跟进客户"},{"type":"image","value":"sop/a.png"}]}`,
			Contact: ContactSOPContact{
				ID:               88,
				Name:             "客户A",
				Avatar:           "avatar/a.png",
				WXExternalUserID: "external-a",
				UpdatedAt:        "2026-07-04 09:00:00",
			},
		}},
	}
	handler := NewContactSOPHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "http://api.example.com")
	req := httptest.NewRequest(http.MethodGet, "/sidebar/contactSop/getSopTipInfo?contactId=88", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "7")
	rec := httptest.NewRecorder()

	handler.GetSOPTipInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code int              `json:"code"`
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 200 || len(envelope.Data) != 1 {
		t.Fatalf("envelope = %#v", envelope)
	}
	item := envelope.Data[0]
	if item["tipTime"] != "10:30" || item["creator"] != "管理员" {
		t.Fatalf("item = %#v", item)
	}
	contact := item["contact"].(map[string]any)
	if contact["avatar"] != "http://api.example.com/static/avatar/a.png" {
		t.Fatalf("contact = %#v", contact)
	}
	task := item["task"].(map[string]any)
	content := task["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content = %#v", content)
	}
	image := content[1].(map[string]any)
	if image["value"] != "http://api.example.com/static/sop/a.png" {
		t.Fatalf("image = %#v", image)
	}
	if store.tipEmployeeID != 7 || store.tipContactID != 88 {
		t.Fatalf("store args employee=%d contact=%d", store.tipEmployeeID, store.tipContactID)
	}
}

func TestContactSOPInfoReturnsSingleReminder(t *testing.T) {
	store := &fakeContactSOPStore{
		employee: SidebarEmployee{ID: 7, CorpID: 9, LogUserID: 3},
		info: ContactSOPItem{
			ID:      55,
			Creator: "管理员",
			TaskRaw: `{"content":[{"type":"text","value":"跟进客户"}]}`,
			Contact: ContactSOPContact{
				ID:               88,
				Name:             "客户A",
				WXExternalUserID: "external-a",
			},
		},
		infoFound: true,
	}
	handler := NewContactSOPHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "")
	req := httptest.NewRequest(http.MethodGet, "/sidebar/contactSop/getSopInfo?id=55", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "7")
	rec := httptest.NewRecorder()

	handler.GetSOPInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.infoEmployeeID != 7 || store.infoID != 55 {
		t.Fatalf("store args employee=%d id=%d", store.infoEmployeeID, store.infoID)
	}
}

type fakeContactSOPStore struct {
	employee  SidebarEmployee
	tips      []ContactSOPItem
	info      ContactSOPItem
	infoFound bool

	tipEmployeeID  int
	tipContactID   int
	infoEmployeeID int
	infoID         int
}

func (s *fakeContactSOPStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	if employeeID != s.employee.ID {
		return SidebarEmployee{}, false, nil
	}
	return s.employee, true, nil
}

func (s *fakeContactSOPStore) ContactSOPTips(_ context.Context, employeeID int, contactID int) ([]ContactSOPItem, error) {
	s.tipEmployeeID = employeeID
	s.tipContactID = contactID
	return s.tips, nil
}

func (s *fakeContactSOPStore) ContactSOPInfo(_ context.Context, employeeID int, id int) (ContactSOPItem, bool, error) {
	s.infoEmployeeID = employeeID
	s.infoID = id
	return s.info, s.infoFound, nil
}
