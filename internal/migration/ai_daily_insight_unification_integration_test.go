//go:build integration

package migration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

const aiDailyMigrationIntegrationDSNEnv = "MOCHAT_GO_AI_INSIGHT_MYSQL_INTEGRATION_DSN"

func TestAIDailyInsightUnification0165RetriesEveryPartialStageOnRealMariaDB(t *testing.T) {
	db := aiDailyMigrationIntegrationDB(t)
	up := aiDailyMigrationStatements(t, "up")
	down := aiDailyMigrationStatements(t, "down")

	for cut := 1; cut < len(up); cut++ {
		t.Run("up_after_statement_"+string(rune('0'+cut)), func(t *testing.T) {
			resetAIDailyMigrationFixture(t, db)
			execAIDailyMigrationStatements(t, db, up[:cut])
			execAIDailyMigrationStatements(t, db, up)
			execAIDailyMigrationStatements(t, db, up)
			assertAIDailyMigrationUpState(t, db)
		})
	}

	for cut := 1; cut < len(down); cut++ {
		t.Run("down_after_statement_"+string(rune('0'+cut)), func(t *testing.T) {
			resetAIDailyMigrationFixture(t, db)
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
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid AI insight integration DSN")
	}
	if !isAIDailyMigrationIntegrationSchema(config.DBName) {
		t.Fatal("AI daily migration integration DSN must target a dedicated test schema")
	}
	config.MultiStatements = false
	db, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		t.Fatal("connect AI daily migration integration database")
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS mochat_go_ai_analysis")
		_, _ = db.Exec("DROP TABLE IF EXISTS mochat_go_ai_conversation_insights")
		_ = db.Close()
	})
	return db
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

func resetAIDailyMigrationFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		"DROP TABLE IF EXISTS mochat_go_ai_analysis",
		"DROP TABLE IF EXISTS mochat_go_ai_conversation_insights",
		`CREATE TABLE mochat_go_ai_conversation_insights (
			id bigint unsigned NOT NULL AUTO_INCREMENT,
			tenant_id int unsigned NOT NULL,
			corp_id int unsigned NOT NULL,
			analysis_type varchar(24) NOT NULL,
			rule_id bigint unsigned NOT NULL DEFAULT 0,
			rule_version_id bigint unsigned NOT NULL DEFAULT 0,
			conversation_key varchar(191) NOT NULL,
			employee_id bigint unsigned NOT NULL DEFAULT 0,
			employee_name varchar(120) NOT NULL DEFAULT '',
			employee_avatar varchar(512) NOT NULL DEFAULT '',
			target_type varchar(24) NOT NULL DEFAULT '',
			target_id varchar(191) NOT NULL DEFAULT '',
			target_name varchar(191) NOT NULL DEFAULT '',
			target_avatar varchar(512) NOT NULL DEFAULT '',
			source_started_at datetime(6) NULL,
			source_ended_at datetime(6) NULL,
			source_message_count int unsigned NOT NULL DEFAULT 0,
			source_fingerprint char(64) NOT NULL,
			status varchar(16) NOT NULL DEFAULT 'pending',
			summary varchar(1200) NOT NULL DEFAULT '',
			result_json json NOT NULL,
			error_summary varchar(500) NOT NULL DEFAULT '',
			provider varchar(64) NOT NULL DEFAULT '',
			model varchar(128) NOT NULL DEFAULT '',
			prompt_version varchar(32) NOT NULL DEFAULT '',
			generated_at datetime(6) NULL,
			created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			PRIMARY KEY (id),
			UNIQUE KEY uq_ai_conversation_source (tenant_id,corp_id,analysis_type,rule_version_id,conversation_key,source_fingerprint)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE mochat_go_ai_analysis (id bigint unsigned NOT NULL AUTO_INCREMENT, PRIMARY KEY (id)) ENGINE=InnoDB`,
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
