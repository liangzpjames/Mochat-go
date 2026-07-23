package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoomSOPInfoReturnsSidebarPayload(t *testing.T) {
	store := &fakeRoomSOPStore{
		employee: SidebarEmployee{ID: 7, CorpID: 9, LogUserID: 3},
		info: RoomSOPItem{
			ID:        55,
			RoomSOPID: 66,
			Creator:   "管理员",
			Time:      "2026-07-04 10:30:00",
			State:     0,
			TaskRaw:   `{"content":[{"type":"text","value":"群提醒"},{"type":"image","value":"sop/room.png"}]}`,
			Room: RoomSOPRoom{
				ID:         88,
				Name:       "客户群A",
				WXChatID:   "chat-a",
				CreateTime: "2026-07-04 09:00:00",
			},
		},
		infoFound: true,
	}
	handler := NewRoomSOPHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "http://api.example.com")
	req := httptest.NewRequest(http.MethodGet, "/sidebar/roomSop/getSopInfo?id=55", nil)
	req.Header.Set("X-Mochat-Go-Employee-ID", "7")
	rec := httptest.NewRecorder()

	handler.GetSOPInfo(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 200 || envelope.Data["creator"] != "管理员" || envelope.Data["state"].(float64) != 0 {
		t.Fatalf("envelope = %#v", envelope)
	}
	room := envelope.Data["room"].(map[string]any)
	if room["name"] != "客户群A" || room["wxChatId"] != "chat-a" {
		t.Fatalf("room = %#v", room)
	}
	task := envelope.Data["task"].(map[string]any)
	content := task["content"].([]any)
	image := content[1].(map[string]any)
	if image["value"] != "http://api.example.com/static/sop/room.png" {
		t.Fatalf("image = %#v", image)
	}
	if store.infoEmployeeID != 7 || store.infoID != 55 {
		t.Fatalf("store args employee=%d id=%d", store.infoEmployeeID, store.infoID)
	}
}

func TestRoomSOPLogStateMarksDone(t *testing.T) {
	store := &fakeRoomSOPStore{
		employee:  SidebarEmployee{ID: 7, CorpID: 9, LogUserID: 3},
		markFound: true,
	}
	handler := NewRoomSOPHandler(store, HeaderUserIDResolver{HeaderName: "X-Mochat-Go-Employee-ID"}, "")
	req := httptest.NewRequest(http.MethodPut, "/sidebar/roomSop/logState", bytes.NewBufferString(`{"id":55}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-Employee-ID", "7")
	rec := httptest.NewRecorder()

	handler.LogState(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.markEmployeeID != 7 || store.markID != 55 {
		t.Fatalf("store args employee=%d id=%d", store.markEmployeeID, store.markID)
	}
}

type fakeRoomSOPStore struct {
	employee  SidebarEmployee
	info      RoomSOPItem
	infoFound bool
	markFound bool

	infoEmployeeID int
	infoID         int
	markEmployeeID int
	markID         int
}

func (s *fakeRoomSOPStore) SidebarEmployeeByID(_ context.Context, employeeID int) (SidebarEmployee, bool, error) {
	if employeeID != s.employee.ID {
		return SidebarEmployee{}, false, nil
	}
	return s.employee, true, nil
}

func (s *fakeRoomSOPStore) RoomSOPInfo(_ context.Context, employeeID int, id int) (RoomSOPItem, bool, error) {
	s.infoEmployeeID = employeeID
	s.infoID = id
	return s.info, s.infoFound, nil
}

func (s *fakeRoomSOPStore) MarkRoomSOPDone(_ context.Context, employeeID int, id int) (bool, error) {
	s.markEmployeeID = employeeID
	s.markID = id
	return s.markFound, nil
}
