package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkContactBatchLabelingWritesMissingPivots(t *testing.T) {
	store := &fakeWorkReadStore{
		users:              map[int]User{1: {ID: 1}},
		batchLabelInserted: 3,
	}
	handler := NewWorkReadHandler(store, staticAdminCache("7-88"), HeaderUserIDResolver{}, "")

	req := httptest.NewRequest(http.MethodPost, "/dashboard/workContact/batchLabeling", strings.NewReader(`{"contactId":"21,22,21","tagId":"3,4,0,3"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mochat-Go-User-ID", "1")
	rec := httptest.NewRecorder()
	handler.WorkContactBatchLabeling(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if store.lastBatchLabelEmployeeID != 88 {
		t.Fatalf("employeeID = %d", store.lastBatchLabelEmployeeID)
	}
	if len(store.lastBatchLabelContactIDs) != 2 || store.lastBatchLabelContactIDs[0] != 21 || store.lastBatchLabelContactIDs[1] != 22 {
		t.Fatalf("contactIDs = %#v", store.lastBatchLabelContactIDs)
	}
	if len(store.lastBatchLabelTagIDs) != 2 || store.lastBatchLabelTagIDs[0] != 3 || store.lastBatchLabelTagIDs[1] != 4 {
		t.Fatalf("tagIDs = %#v", store.lastBatchLabelTagIDs)
	}
	body := decodeBody(t, rec.Body.Bytes())
	if body["code"].(float64) != 200 || len(body["data"].([]any)) != 0 {
		t.Fatalf("body = %#v", body)
	}
}

func TestWorkContactBatchLabelingValidatesRequiredParams(t *testing.T) {
	handler := NewWorkReadHandler(
		&fakeWorkReadStore{users: map[int]User{1: {ID: 1}}},
		staticAdminCache("7-88"),
		HeaderUserIDResolver{},
		"",
	)

	for _, tc := range []struct {
		name string
		body string
		msg  string
	}{
		{name: "contact", body: `{"tagId":"3"}`, msg: "客户id必传"},
		{name: "tag", body: `{"contactId":"21"}`, msg: "标签id必传"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/dashboard/workContact/batchLabeling", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Mochat-Go-User-ID", "1")
			rec := httptest.NewRecorder()
			handler.WorkContactBatchLabeling(rec, req)

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
