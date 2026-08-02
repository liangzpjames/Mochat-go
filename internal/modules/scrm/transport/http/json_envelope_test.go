package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONWrapsSuccessfulDataInDashboardEnvelope(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeJSON(recorder, 200, map[string]any{"data": map[string]any{"items": []string{}}})

	var response struct {
		Code int            `json:"code"`
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || response.Msg != "success" || response.Data == nil {
		t.Fatalf("response = %#v, want code=200 msg=success data", response)
	}
}
