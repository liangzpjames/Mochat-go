package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteMachineEnvelopeUsesNumericHTTPCodeAndMachineErrorCode(t *testing.T) {
	for _, test := range []struct {
		name        string
		status      int
		machineCode string
	}{
		{name: "tenant access denied", status: http.StatusForbidden, machineCode: DashboardTenantAccessDeniedCode},
		{name: "dashboard permission denied", status: http.StatusForbidden, machineCode: DashboardPermissionDeniedCode},
		{name: "unauthorized", status: http.StatusUnauthorized, machineCode: "UNAUTHORIZED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			writeMachineEnvelope(response, test.status, test.machineCode, "拒绝访问", map[string]any{"requestId": "contract-test"})

			if response.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.status, response.Body.String())
			}
			var body struct {
				Code      int            `json:"code"`
				ErrorCode string         `json:"errorCode"`
				Msg       string         `json:"msg"`
				Data      map[string]any `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v body=%s", err, response.Body.String())
			}
			if body.Code != test.status || body.ErrorCode != test.machineCode || body.Msg != "拒绝访问" {
				t.Fatalf("body=%+v want code=%d errorCode=%q", body, test.status, test.machineCode)
			}
			if body.Data["requestId"] != "contract-test" {
				t.Fatalf("data=%v", body.Data)
			}
		})
	}
}
