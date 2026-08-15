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

// RED→GREEN：会话归档分表使用 work_employee_id（无 employee_id/tenant_id 列），
// 员工报表必须仍能从分区读取数据，而不是返回“会话归档表不可用”。
func TestEmployeeReportReadsArchivePartitionsWithWorkEmployeeID(t *testing.T) {
	dsn := os.Getenv("MOCHAT_MYSQL_DSN")
	if dsn == "" {
		t.Skip("MOCHAT_MYSQL_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	const corpID = 1
	if _, err := db.ExecContext(ctx, `
		INSERT INTO mc_work_message_1
			(corp_id, msgid, seq, work_employee_id, to_user_type, to_user_id, sender_type, action, type, msg_type, content, content_text, room_id, status, msg_data_time, created_at, updated_at, deleted_at)
		VALUES (?, 'reporting-tdd-archive-employee', 1, 1001, 1, 2001, 0, 0, 100, 100, '{"text":"tdd"}', 'reporting tdd archive employee', 0, 0, NOW(), NOW(), NOW(), NULL)
		ON DUPLICATE KEY UPDATE content_text = VALUES(content_text)`, corpID); err != nil {
		t.Fatal(err)
	}
	service := NewSQLService(db)
	q := ReportQuery{
		TenantID: 1,
		CorpID:   corpID,
		StartAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndAt:    time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		Timezone: "Asia/Shanghai",
		Page:     1,
		PageSize: 20,
	}
	result, err := service.Query(ctx, EmployeeReport, q)
	if err != nil {
		t.Fatalf("employee report failed: %v", err)
	}
	if result.Freshness.Status != "available" {
		t.Fatalf("employee report provider not available: %+v", result.Freshness)
	}
	employeeCount, ok := result.Summary["employee"]
	if !ok || employeeCount == nil || *employeeCount < 1 {
		t.Fatalf("employee count missing or zero: %+v", result.Summary)
	}
}
