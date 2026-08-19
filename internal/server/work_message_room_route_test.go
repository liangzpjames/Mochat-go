package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"jiyi/mochat-go/internal/config"
)

func TestWorkMessageRoomRoutesDispatchToDedicatedHandlers(t *testing.T) {
	handler := func(body string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
	}
	srv, err := New(config.Config{},
		WithWorkMessageRoomDirectoryHandler(handler("directory")),
		WithWorkMessageRoomProfileHandler(handler("profile")),
		WithWorkMessageRoomMessagesHandler(handler("messages")),
		WithWorkMessageRoomMembersHandler(handler("members")),
		WithWorkMessageRoomFilterOptionsHandler(handler("options")),
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path string
		want string
	}{
		{"/dashboard/workMessage/roomDirectory", "directory"},
		{"/dashboard/workMessage/roomProfile", "profile"},
		{"/dashboard/workMessage/roomMessages", "messages"},
		{"/dashboard/workMessage/roomMembers", "members"},
		{"/dashboard/workMessage/roomFilterOptions", "options"},
	} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, test.path, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != test.want {
			t.Fatalf("%s: status=%d body=%q", test.path, rec.Code, rec.Body.String())
		}
	}
}
