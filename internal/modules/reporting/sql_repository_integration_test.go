//go:build integration

package reporting

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Runs against a retained-volume schema when MOCHAT_MYSQL_DSN is provided.
// The test intentionally uses an empty tenant/corp so it proves every report
// kind executes real SQL without leaking data or requiring fixtures.
func TestSQLRepositoryAgainstRetainedMariaDBSchema(t *testing.T) {
	dsn := os.Getenv("MOCHAT_MYSQL_DSN")
	if dsn == "" {
		t.Skip("MOCHAT_MYSQL_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	service := NewSQLService(db)
	q := ReportQuery{TenantID: 1, CorpID: 1, StartAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), Timezone: "Asia/Shanghai", Page: 1, PageSize: 20}
	for _, kind := range []ReportKind{CustomerReport, ConversionReport, EmployeeReport, BehaviorReport, DetailReport, OverviewReport} {
		if _, err := service.Query(ctx, kind, q); err != nil {
			t.Fatalf("kind %s failed against retained schema: %v", kind, err)
		}
	}
	for _, stage := range []string{"lead", "contact", "opportunity", "won", "order"} {
		stageQuery := q
		stageQuery.Stage = stage
		result, err := service.Query(ctx, ConversionReport, stageQuery)
		if err != nil {
			t.Fatalf("conversion stage %s failed against retained schema: %v", stage, err)
		}
		if int(result.Pagination.Total) != int(*result.Summary[stage]) {
			t.Fatalf("stage %s pagination total=%d summary=%v", stage, result.Pagination.Total, *result.Summary[stage])
		}
	}
}
