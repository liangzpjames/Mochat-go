package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkMessageTrajectoryFilterBuildsShanghaiDayRange(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	req := httptest.NewRequest(http.MethodGet, "/dashboard/workMessage/trajectoryDay?employeeId=9&date=2026-08-19&conversationType=room", nil)
	rec := httptest.NewRecorder()
	filter, ok := workMessageTrajectoryFilter(rec, req, 10, 7, 1, AccessContext{DataPermission: DataPermissionDepartment, DeptEmployeeIDs: []int{9}}, now)
	if !ok {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if filter.EmployeeID != 9 || filter.Date != "2026-08-19" || filter.ConversationType != WorkMessageTrajectoryRoom || filter.StartAt != "2026-08-19 00:00:00" || filter.EndAt != "2026-08-20 00:00:00" {
		t.Fatalf("filter=%+v", filter)
	}
}

func TestWorkMessageTrajectoryFilterRejectsInvalidInputs(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	for _, tc := range []struct{ name, path string }{
		{"missing employee", "/dashboard/workMessage/trajectoryDay?date=2026-08-19"},
		{"invalid employee", "/dashboard/workMessage/trajectoryDay?employeeId=x&date=2026-08-19"},
		{"invalid date", "/dashboard/workMessage/trajectoryDay?employeeId=9&date=2026-02-30"},
		{"future date", "/dashboard/workMessage/trajectoryDay?employeeId=9&date=2026-08-20"},
		{"invalid type", "/dashboard/workMessage/trajectoryDay?employeeId=9&date=2026-08-19&conversationType=internal-room"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			_, ok := workMessageTrajectoryFilter(rec, httptest.NewRequest(http.MethodGet, tc.path, nil), 10, 7, 1, AccessContext{DataPermission: DataPermissionAll}, now)
			if ok || rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":400`) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
