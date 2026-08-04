package store

import (
	"strings"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
)

func TestMediumWhereSelectorVisibleScopesIncludePublicPersonalAndDepartment(t *testing.T) {
	where, args := mediumWhere(dashboard.MediumFilter{
		CorpID:          7,
		SelectorVisible: true,
		UserID:          11,
		EmployeeID:      22,
		Status:          "available",
	}, "m")
	joined := strings.Join(where, " AND ")
	if !strings.Contains(joined, "m.scope_type = 'public'") ||
		!strings.Contains(joined, "m.scope_type = 'personal'") ||
		!strings.Contains(joined, "m.scope_type = 'department'") ||
		!strings.Contains(joined, "mc_work_employee_department") {
		t.Fatalf("selector scope SQL = %s", joined)
	}
	if len(args) != 4 || args[0] != 7 || args[1] != 11 || args[2] != 22 || args[3] != "available" {
		t.Fatalf("selector scope args = %#v", args)
	}
}
