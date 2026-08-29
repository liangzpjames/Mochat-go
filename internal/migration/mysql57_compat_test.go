package migration

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQL57ConditionalAlterUsesMetadataAndPreservesImmutableSource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	columnQuery := regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND COLUMN_NAME=?")
	indexQuery := regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND INDEX_NAME=?")
	mock.ExpectQuery(columnQuery).WithArgs("one", "value").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(indexQuery).WithArgs("one", "idx_value").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	original := "ALTER TABLE `one` ADD COLUMN IF NOT EXISTS `value` int, ADD KEY IF NOT EXISTS `idx_value` (`value`)"
	compatible, err := mysql57ConditionalAlterStatement(context.Background(), db, original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(compatible, "ADD COLUMN") || strings.Contains(compatible, "IF NOT EXISTS") || !strings.Contains(compatible, "ADD KEY `idx_value`") {
		t.Fatalf("compatible statement=%s", compatible)
	}
	if !strings.Contains(original, "ADD COLUMN IF NOT EXISTS") {
		t.Fatal("immutable source changed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSplitSQLTopLevelCommasKeepsIndexAndGeneratedExpressionsTogether(t *testing.T) {
	clauses := splitSQLTopLevelCommas("ADD KEY idx (a,b), ADD COLUMN generated int GENERATED ALWAYS AS (IF(a=1,2,3)) STORED")
	if len(clauses) != 2 {
		t.Fatalf("clauses=%v", clauses)
	}
}
