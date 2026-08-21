package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoomWelcomeWeComClientUsesLocalSimulationForDevelopmentTransferCredentials(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		http.Error(w, `{"errcode":40013,"errmsg":"invalid corpid"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	result, err := NewRoomWelcomeWeComClient(server.URL).TransferCustomer(
		context.Background(),
		RoomWelcomeCorpCredential{WXCorpID: "wwSIM00000000000001", ContactSecret: "SIM-CONTACT-SECRET-0001"},
		[]string{"wmSIMEXT000000020"},
		"lifang",
		"zhangwei",
		"",
	)
	if err != nil {
		t.Fatalf("TransferCustomer() error = %v", err)
	}
	if numericMapInt(result, "errcode") != 0 {
		t.Fatalf("TransferCustomer() result = %#v, want success", result)
	}
	if calls != 0 {
		t.Fatalf("development simulation called external provider %d times", calls)
	}
}

func TestRoomWelcomeWeComClientUsesLocalSimulationForDevelopmentGroupTransferCredentials(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		http.Error(w, `{"errcode":40013,"errmsg":"invalid corpid"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	failed, err := NewRoomWelcomeWeComClient(server.URL).TransferGroupChat(
		context.Background(),
		RoomWelcomeCorpCredential{WXCorpID: "wwSIM00000000000001", ContactSecret: "SIM-CONTACT-SECRET-0001"},
		[]string{"wrSIM000000001"},
		"zhangwei",
	)
	if err != nil {
		t.Fatalf("TransferGroupChat() error = %v", err)
	}
	if len(failed) != 0 {
		t.Fatalf("TransferGroupChat() failed = %#v, want no failed chats", failed)
	}
	if calls != 0 {
		t.Fatalf("development simulation called external provider %d times", calls)
	}
}

func TestContactTransferSaveUnassignedListSkipsExternalProviderForDevelopmentSimulation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		http.Error(w, `{"errcode":40013,"errmsg":"invalid corpid"}`, http.StatusBadRequest)
	}))
	defer server.Close()

	store := &fakeContactTransferStore{
		users:      map[int]User{1: {ID: 1, IsSuperAdmin: 1}},
		credential: RoomWelcomeCorpCredential{CorpID: 7, WXCorpID: "wwSIM00000000000001", ContactSecret: "SIM-CONTACT-SECRET-0001"},
	}
	handler := NewContactTransferHandler(store, staticAdminCache("7-99"), HeaderUserIDResolver{}, nil, NewRoomWelcomeWeComClient(server.URL))
	req := authenticatedDashboardRequestForTest(http.MethodPost, "/dashboard/contactTransfer/sync", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.SaveUnassignedList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if calls != 0 {
		t.Fatalf("development simulation called external provider %d times", calls)
	}
	if len(store.replacedItems) != 0 {
		t.Fatalf("simulation unexpectedly replaced local snapshot: %#v", store.replacedItems)
	}
}
