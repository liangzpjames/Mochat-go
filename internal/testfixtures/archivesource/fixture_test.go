package archivesource

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

type recordingExecutor struct {
	statements []string
}

func (r *recordingExecutor) ExecContext(_ context.Context, statement string, _ ...any) (sql.Result, error) {
	r.statements = append(r.statements, statement)
	return nil, nil
}

func TestPrepareDashboardPermissionDependenciesCreatesThe0127TablesAndStaffResource(t *testing.T) {
	executor := &recordingExecutor{}
	if err := PrepareDashboardPermissionDependencies(context.Background(), executor); err != nil {
		t.Fatal(err)
	}
	joined := strings.ToLower(strings.Join(executor.statements, "\n"))
	for _, fragment := range []string{
		"create table mochat_go_dashboard_permissions",
		"create table mochat_go_dashboard_permission_resources",
		"dashboard.company_setting.staff",
		"uni_dashboard_permission_resource",
	} {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("fixture SQL missing %q: %s", fragment, joined)
		}
	}
}
