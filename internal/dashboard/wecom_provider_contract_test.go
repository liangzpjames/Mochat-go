package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/modules/providers"
)

func TestRoomWelcomeWeComClientStandardEmployeeSyncContract(t *testing.T) {
	const (
		corpID         = "ww-standard-corp"
		employeeSecret = "employee-secret-not-for-output"
		accessToken    = "employee-access-token"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		query := r.URL.Query()
		switch r.URL.Path {
		case "/cgi-bin/gettoken":
			if query.Get("corpid") != corpID || query.Get("corpsecret") != employeeSecret {
				t.Fatalf("gettoken query = %s", query.Encode())
			}
			_, _ = w.Write([]byte(`{"errcode":0,"access_token":"employee-access-token","expires_in":7200}`))
		case "/cgi-bin/department/list":
			if query.Get("access_token") != accessToken {
				t.Fatalf("department token = %q", query.Get("access_token"))
			}
			_, _ = w.Write([]byte(`{"errcode":0,"department":[{"id":1,"name":"Root","parentid":0,"order":1},{"id":7,"name":"Sales","parentid":1,"order":2}]}`))
		case "/cgi-bin/user/list":
			if query.Get("access_token") != accessToken || query.Get("department_id") != "7" || query.Get("fetch_child") != "0" {
				t.Fatalf("user list query = %s", query.Encode())
			}
			_, _ = w.Write([]byte(`{"errcode":0,"userlist":[{"userid":"u-7","name":"Ada","department":[7],"is_leader_in_dept":[1],"order":[3],"status":1}]}`))
		default:
			t.Fatalf("unexpected WeCom endpoint %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewRoomWelcomeWeComClient(server.URL)
	credential := WorkEmployeeSyncCredential{WXCorpID: corpID, EmployeeSecret: employeeSecret}
	departments, err := client.Departments(context.Background(), credential)
	if err != nil {
		t.Fatalf("Departments() error = %v", err)
	}
	if len(departments) != 2 || departments[1].WXDepartmentID != 7 || departments[1].WXParentID != 1 {
		t.Fatalf("departments = %#v", departments)
	}
	users, err := client.DepartmentUsers(context.Background(), credential, 7)
	if err != nil {
		t.Fatalf("DepartmentUsers() error = %v", err)
	}
	if len(users) != 1 || users[0].WXUserID != "u-7" || users[0].Name != "Ada" || users[0].DepartmentIDs[0] != 7 {
		t.Fatalf("users = %#v", users)
	}
}

func TestRoomWelcomeWeComClientStandardSyncErrorIsStableAndRedacted(t *testing.T) {
	const secret = "provider-secret-must-not-leak"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/cgi-bin/gettoken" {
			t.Fatalf("unexpected endpoint %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"errcode":40001,"errmsg":"` + secret + `"}`))
	}))
	defer server.Close()

	_, err := NewRoomWelcomeWeComClient(server.URL).Departments(context.Background(), WorkEmployeeSyncCredential{
		WXCorpID: "ww-corp", EmployeeSecret: secret,
	})
	if err == nil || !strings.Contains(err.Error(), "WECOM_API_ERROR_40001") {
		t.Fatalf("error = %v, want stable WeCom machine code", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error = %v leaks provider secret", err)
	}
}

func TestRoomWelcomeWeComClientStatusComesFromRuntimeComponent(t *testing.T) {
	status := NewRoomWelcomeWeComClient("http://wecom-runtime.example").Status()
	if status.Kind != "wecom_standard" || status.Source != providers.SourceExternal || status.State != providers.StateLimited || status.Code != "wecom.tenant_credentials_required" {
		t.Fatalf("status = %#v, want limited external runtime status", status)
	}
	if strings.Contains(status.Reason, "secret") || strings.Contains(status.Reason, "token") {
		t.Fatalf("status reason contains sensitive material: %q", status.Reason)
	}
}
