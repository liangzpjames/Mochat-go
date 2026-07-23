package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContactTransferInfoReturnsAssignedContacts(t *testing.T) {
	store := &fakeContactTransferStore{
		users: map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		assigned: []ContactTransferContactItem{{
			ContactID: 31, EmployeeID: 21, ContactWXID: "external-31", EmployeeWXID: "employee-21",
			ContactName: "备注客户", NickName: "客户A", CorpName: "客户公司", EmployeeName: "员工A",
			Tags: []string{"高意向"}, TransferState: "等待接替", AddTime: "2026-07-03 10:20", AddWay: 202,
		}},
	}
	handler := NewContactTransferHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, nil)

	req := httptest.NewRequest(http.MethodGet, `/dashboard/contactTransfer/info?contactName=%E5%AE%A2%E6%88%B7&employeeId=[21]&addTimeStart=2026-07-01&addTimeEnd=2026-07-04`, nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.Info(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastAssignedFilter.CorpID != 7 || store.lastAssignedFilter.ContactName != "客户" || len(store.lastAssignedFilter.EmployeeIDs) != 1 || store.lastAssignedFilter.EmployeeIDs[0] != 21 {
		t.Fatalf("filter = %+v", store.lastAssignedFilter)
	}
	body := decodeBody(t, rec.Body.Bytes())
	item := body["data"].([]any)[0].(map[string]any)
	if item["contactWxId"] != "external-31" || item["transferState"] != "等待接替" || item["addWay"] != "管理员/负责人分配" {
		t.Fatalf("item = %#v", item)
	}
}

func TestContactTransferSaveUnassignedListSyncsWeComData(t *testing.T) {
	store := &fakeContactTransferStore{
		users:      map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
	}
	client := &fakeContactTransferWeComClient{
		unassigned: []ContactTransferUnassignedSeed{{HandoverUserID: "employee-21", ExternalUserID: "external-31", DimissionTime: 1710000000}},
	}
	handler := NewContactTransferHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, client)

	req := httptest.NewRequest(http.MethodGet, "/dashboard/contactTransfer/saveUnassignedList", nil)
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.SaveUnassignedList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if client.getUnassignedCredential.WXCorpID != "wwid" || store.replacedCorpID != 7 || len(store.replacedItems) != 1 || store.replacedItems[0].ExternalUserID != "external-31" {
		t.Fatalf("credential=%+v replaced corp=%d items=%+v", client.getUnassignedCredential, store.replacedCorpID, store.replacedItems)
	}
}

func TestContactTransferTransferCustomerWritesSuccessLog(t *testing.T) {
	store := &fakeContactTransferStore{
		users:       map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		credential:  RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		contactName: "客户A",
	}
	client := &fakeContactTransferWeComClient{customerResponse: map[string]any{"errcode": float64(0), "errmsg": "ok"}}
	handler := NewContactTransferHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, client)

	body := `{"type":1,"takeoverUserId":"employee-99","list":"[{\"contactWxId\":\"external-31\",\"employeeWxId\":\"employee-21\"}]"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/contactTransfer/index", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TransferCustomer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(client.transferredCustomers) != 1 || client.transferredCustomers[0].ExternalUserID != "external-31" || client.transferredCustomers[0].HandoverUserID != "employee-21" || client.transferredCustomers[0].TakeoverUserID != "employee-99" {
		t.Fatalf("transfers = %+v", client.transferredCustomers)
	}
	if len(store.createdLogs) != 1 || store.createdLogs[0].Status != 1 || store.createdLogs[0].Type != 1 || store.createdLogs[0].Name != "客户A" || store.createdLogs[0].TakeoverEmployeeID != "employee-99" {
		t.Fatalf("logs = %+v", store.createdLogs)
	}
}

func TestContactTransferTransferRoomSkipsFailedChats(t *testing.T) {
	store := &fakeContactTransferStore{
		users:      map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwid", ContactSecret: "secret"},
		roomNames:  map[string]string{"chat-ok": "成交群"},
	}
	client := &fakeContactTransferWeComClient{failedChats: []map[string]any{{"chat_id": "chat-failed", "errcode": float64(90001)}}}
	handler := NewContactTransferHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, client)

	body := `{"takeoverUserId":"employee-99","list":"[\"chat-ok\",\"chat-failed\"]"}`
	req := httptest.NewRequest(http.MethodPost, "/dashboard/contactTransfer/room", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.TransferRoom(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(client.transferredRooms) != 2 || client.transferredRooms[0] != "chat-ok" || client.transferredRooms[1] != "chat-failed" {
		t.Fatalf("rooms = %+v", client.transferredRooms)
	}
	if len(store.createdLogs) != 1 || store.createdLogs[0].Type != 2 || store.createdLogs[0].ContactID != "chat-ok" || store.createdLogs[0].Name != "成交群" {
		t.Fatalf("logs = %+v", store.createdLogs)
	}
}

type fakeContactTransferStore struct {
	users              map[int]User
	credential         RoomWelcomeCorpCredential
	assigned           []ContactTransferContactItem
	unassigned         ContactTransferUnassignedPage
	rooms              []ContactTransferRoomItem
	logs               []ContactTransferLogItem
	contactName        string
	roomNames          map[string]string
	lastAssignedFilter ContactTransferContactFilter
	lastUnassigned     ContactTransferUnassignedFilter
	lastLogFilter      ContactTransferLogFilter
	replacedCorpID     int
	replacedItems      []ContactTransferUnassignedSeed
	createdLogs        []ContactTransferLogWrite
}

func (s *fakeContactTransferStore) UserByID(_ context.Context, userID int) (User, bool, error) {
	user, ok := s.users[userID]
	return user, ok, nil
}

func (s *fakeContactTransferStore) EmployeeIDByUserCorp(_ context.Context, _ int, _ int) (int, error) {
	return 99, nil
}

func (s *fakeContactTransferStore) FirstEmployeeByUser(_ context.Context, _ int) (int, int, bool, error) {
	return 7, 99, true, nil
}

func (s *fakeContactTransferStore) RoomWelcomeCorpCredentialByID(_ context.Context, _ int) (RoomWelcomeCorpCredential, bool, error) {
	return s.credential, s.credential.WXCorpID != "", nil
}

func (s *fakeContactTransferStore) ContactTransferAssignedContacts(_ context.Context, filter ContactTransferContactFilter) ([]ContactTransferContactItem, error) {
	s.lastAssignedFilter = filter
	return s.assigned, nil
}

func (s *fakeContactTransferStore) ContactTransferUnassignedContacts(_ context.Context, filter ContactTransferUnassignedFilter) (ContactTransferUnassignedPage, error) {
	s.lastUnassigned = filter
	return s.unassigned, nil
}

func (s *fakeContactTransferStore) ContactTransferRooms(_ context.Context, _ int, _ string) ([]ContactTransferRoomItem, error) {
	return s.rooms, nil
}

func (s *fakeContactTransferStore) ContactTransferLogs(_ context.Context, filter ContactTransferLogFilter) ([]ContactTransferLogItem, error) {
	s.lastLogFilter = filter
	return s.logs, nil
}

func (s *fakeContactTransferStore) ReplaceContactTransferUnassigned(_ context.Context, corpID int, items []ContactTransferUnassignedSeed) error {
	s.replacedCorpID = corpID
	s.replacedItems = append([]ContactTransferUnassignedSeed{}, items...)
	return nil
}

func (s *fakeContactTransferStore) CreateContactTransferLog(_ context.Context, values ContactTransferLogWrite) error {
	s.createdLogs = append(s.createdLogs, values)
	return nil
}

func (s *fakeContactTransferStore) WorkContactNameByExternalUserID(_ context.Context, _ int, _ string) (string, bool, error) {
	return s.contactName, s.contactName != "", nil
}

func (s *fakeContactTransferStore) WorkRoomNameByWXChatID(_ context.Context, _ int, wxChatID string) (string, bool, error) {
	name := s.roomNames[wxChatID]
	return name, name != "", nil
}

type fakeContactTransferWeComClient struct {
	unassigned                 []ContactTransferUnassignedSeed
	customerResponse           map[string]any
	failedChats                []map[string]any
	getUnassignedCredential    RoomWelcomeCorpCredential
	transferredCustomers       []fakeTransferredCustomer
	transferredRooms           []string
	transferRoomTakeoverUser   string
	transferCustomerCredential RoomWelcomeCorpCredential
}

type fakeTransferredCustomer struct {
	ExternalUserID string
	HandoverUserID string
	TakeoverUserID string
}

func (c *fakeContactTransferWeComClient) GetUnassigned(_ context.Context, credential RoomWelcomeCorpCredential) ([]ContactTransferUnassignedSeed, error) {
	c.getUnassignedCredential = credential
	return c.unassigned, nil
}

func (c *fakeContactTransferWeComClient) TransferCustomer(_ context.Context, credential RoomWelcomeCorpCredential, externalUserIDs []string, handoverUserID string, takeoverUserID string, _ string) (map[string]any, error) {
	c.transferCustomerCredential = credential
	for _, externalUserID := range externalUserIDs {
		c.transferredCustomers = append(c.transferredCustomers, fakeTransferredCustomer{ExternalUserID: externalUserID, HandoverUserID: handoverUserID, TakeoverUserID: takeoverUserID})
	}
	if c.customerResponse == nil {
		return map[string]any{"errcode": float64(0)}, nil
	}
	return c.customerResponse, nil
}

func (c *fakeContactTransferWeComClient) TransferGroupChat(_ context.Context, _ RoomWelcomeCorpCredential, chatIDs []string, takeoverUserID string) ([]map[string]any, error) {
	c.transferredRooms = append([]string{}, chatIDs...)
	c.transferRoomTakeoverUser = takeoverUserID
	if c.failedChats == nil {
		return []map[string]any{}, nil
	}
	return c.failedChats, nil
}
