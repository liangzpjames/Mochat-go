//go:build integration

package migration_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jiyi/mochat-go/internal/integrationtestdb"
	. "jiyi/mochat-go/internal/migration"
	"jiyi/mochat-go/internal/migration/testharness"
)

const aiDailyMigrationIntegrationDSNEnv = "MOCHAT_GO_AI_INSIGHT_MYSQL_INTEGRATION_DSN"

func TestAIDailyInsightUnification0165RetriesEveryPartialStageOnRealMariaDB(t *testing.T) {
	up := aiDailyMigrationStatements(t, "up")
	down := aiDailyMigrationStatements(t, "down")

	for cut := 1; cut < len(up); cut++ {
		t.Run("up_after_statement_"+string(rune('0'+cut)), func(t *testing.T) {
			db := aiDailyMigrationIntegrationDB(t)
			seedAIDailyMigrationFixture(t, db)
			execAIDailyMigrationStatements(t, db, up[:cut])
			execAIDailyMigrationStatements(t, db, up)
			execAIDailyMigrationStatements(t, db, up)
			assertAIDailyMigrationUpState(t, db)
		})
	}

	for cut := 1; cut < len(down); cut++ {
		t.Run("down_after_statement_"+string(rune('0'+cut)), func(t *testing.T) {
			db := aiDailyMigrationIntegrationDB(t)
			seedAIDailyMigrationFixture(t, db)
			execAIDailyMigrationStatements(t, db, up)
			prepareAIDailyMigrationDownDuplicate(t, db)
			execAIDailyMigrationStatements(t, db, down[:cut])
			execAIDailyMigrationStatements(t, db, down)
			execAIDailyMigrationStatements(t, db, down)
			assertAIDailyMigrationDownState(t, db)
		})
	}
}

func aiDailyMigrationIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(aiDailyMigrationIntegrationDSNEnv))
	if dsn == "" {
		if os.Getenv("MOCHAT_REQUIRE_MYSQL_INTEGRATION") == "1" {
			t.Fatalf("%s is required when MOCHAT_REQUIRE_MYSQL_INTEGRATION=1", aiDailyMigrationIntegrationDSNEnv)
		}
		t.Skipf("%s is required for MariaDB integration tests", aiDailyMigrationIntegrationDSNEnv)
	}
	database := integrationtestdb.NewIsolated(t, dsn)
	evidence, err := testharness.NewControlledEvidence("ai-daily-0165")
	if err != nil {
		t.Fatal(err)
	}
	if err := testharness.ApplyThrough(context.Background(), database.DB, filepath.Join("..", ".."), "0164_saas_tenant_ai_provider", evidence); err != nil {
		t.Fatalf("apply production migration registry through 0164: %v", err)
	}
	database.DB.SetMaxOpenConns(1)
	database.DB.SetMaxIdleConns(1)
	return database.DB
}

func aiDailyMigrationStatements(t *testing.T, direction string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0165_ai_daily_insight_unification."+direction+".sql"))
	if err != nil {
		t.Fatal(err)
	}
	statements, err := SplitSQLStatements(string(body))
	if err != nil {
		t.Fatal(err)
	}
	return statements
}

func seedAIDailyMigrationFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`INSERT INTO mochat_go_ai_conversation_insights
			(tenant_id,corp_id,analysis_type,rule_version_id,conversation_key,source_fingerprint,status,result_json,generated_at,created_at)
		VALUES
			(1,2,'session',3,'employee:customer',REPEAT('a',64),'succeeded',JSON_OBJECT(),'2026-08-24 16:30:00','2026-08-24 16:30:00'),
			(1,2,'session',3,'employee:customer',REPEAT('b',64),'succeeded',JSON_OBJECT(),'2026-08-25 02:00:00','2026-08-25 02:00:00')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func prepareAIDailyMigrationDownDuplicate(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO mochat_go_ai_conversation_insights
		(tenant_id,corp_id,analysis_type,rule_id,rule_version_id,conversation_key,analysis_date,employee_id,employee_name,employee_avatar,target_type,target_id,target_name,target_avatar,source_started_at,source_ended_at,source_message_count,source_fingerprint,status,summary,result_json,error_summary,provider,model,prompt_version,previous_insight_id,previous_score,previous_summary,previous_generated_at,generated_at,created_at,updated_at)
	SELECT tenant_id,corp_id,analysis_type,rule_id,rule_version_id,conversation_key,'2026-08-26',employee_id,employee_name,employee_avatar,target_type,target_id,target_name,target_avatar,source_started_at,source_ended_at,source_message_count,source_fingerprint,status,summary,result_json,error_summary,provider,model,prompt_version,previous_insight_id,previous_score,previous_summary,previous_generated_at,'2026-08-26 02:00:00','2026-08-26 02:00:00','2026-08-26 02:00:00'
	FROM mochat_go_ai_conversation_insights WHERE id=2`)
	if err != nil {
		t.Fatal(err)
	}
}

func execAIDailyMigrationStatements(t *testing.T, db *sql.DB, statements []string) {
	t.Helper()
	for index, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("statement %d failed: %v", index+1, err)
		}
	}
}

func assertAIDailyMigrationUpState(t *testing.T, db *sql.DB) {
	t.Helper()
	assertAIDailyMigrationObjectCount(t, db, "information_schema.columns", "table_schema=DATABASE() AND table_name='mochat_go_ai_conversation_insights' AND column_name='analysis_date'", 1)
	assertAIDailyMigrationObjectCount(t, db, "information_schema.statistics", "table_schema=DATABASE() AND table_name='mochat_go_ai_conversation_insights' AND index_name='uq_ai_conversation_daily'", 6)
	assertAIDailyMigrationObjectCount(t, db, "information_schema.tables", "table_schema=DATABASE() AND table_name='mochat_go_ai_analysis'", 0)
	var id, rows int
	var analysisDate string
	if err := db.QueryRow("SELECT COUNT(*), MIN(id), DATE_FORMAT(MIN(analysis_date),'%Y-%m-%d') FROM mochat_go_ai_conversation_insights").Scan(&rows, &id, &analysisDate); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || id != 2 || analysisDate != "2026-08-25" {
		t.Fatalf("up data state rows=%d id=%d analysis_date=%s, want 1/2/2026-08-25", rows, id, analysisDate)
	}
}

func assertAIDailyMigrationDownState(t *testing.T, db *sql.DB) {
	t.Helper()
	assertAIDailyMigrationObjectCount(t, db, "information_schema.columns", "table_schema=DATABASE() AND table_name='mochat_go_ai_conversation_insights' AND column_name='analysis_date'", 0)
	assertAIDailyMigrationObjectCount(t, db, "information_schema.statistics", "table_schema=DATABASE() AND table_name='mochat_go_ai_conversation_insights' AND index_name='uq_ai_conversation_source'", 6)
	assertAIDailyMigrationObjectCount(t, db, "information_schema.tables", "table_schema=DATABASE() AND table_name='mochat_go_ai_analysis'", 1)
	var rows, id int
	if err := db.QueryRow("SELECT COUNT(*), MIN(id) FROM mochat_go_ai_conversation_insights").Scan(&rows, &id); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || id != 3 {
		t.Fatalf("down data state rows=%d id=%d, want newest source row 1/3", rows, id)
	}
}

func assertAIDailyMigrationObjectCount(t *testing.T, db *sql.DB, table, where string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE " + where).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s count=%d, want %d", table, got, want)
	}
}
