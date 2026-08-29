package sqlscript

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"
)

type QueryExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var mysqlSignalPattern = regexp.MustCompile(`(?is)^\s*SIGNAL\s+SQLSTATE\s+'([0-9A-Z]{5})'\s+SET\s+MESSAGE_TEXT\s*=\s*'((?:''|[^'])*)'\s*$`)

// ExecuteStatement preserves migration SIGNAL guards on MySQL 5.7, whose
// prepared-statement protocol rejects SIGNAL before EXECUTE. The selected
// dynamic SQL is resolved from the same pinned connection and only the exact
// SIGNAL form used by migrations is converted to its server-equivalent error.
func ExecuteStatement(ctx context.Context, execer QueryExecer, statement string) error {
	_, err := execer.ExecContext(ctx, statement)
	if err == nil {
		return nil
	}
	variable, ok := preparedUserVariable(statement)
	if !ok || !isUnsupportedPreparedStatement(err) {
		return err
	}
	var dynamicSQL sql.NullString
	if queryErr := execer.QueryRowContext(ctx, "SELECT "+variable).Scan(&dynamicSQL); queryErr != nil || !dynamicSQL.Valid {
		return err
	}
	match := mysqlSignalPattern.FindStringSubmatch(dynamicSQL.String)
	if len(match) != 3 {
		return err
	}
	var state [5]byte
	copy(state[:], match[1])
	return &mysqldriver.MySQLError{
		Number:   1644,
		SQLState: state,
		Message:  strings.ReplaceAll(match[2], "''", "'"),
	}
}

func preparedUserVariable(statement string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(statement))
	if len(fields) != 4 || !strings.EqualFold(fields[0], "PREPARE") || !strings.EqualFold(fields[2], "FROM") {
		return "", false
	}
	variable := fields[3]
	if len(variable) < 2 || variable[0] != '@' {
		return "", false
	}
	for _, char := range variable[1:] {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' {
			return "", false
		}
	}
	return variable, true
}

func isUnsupportedPreparedStatement(err error) bool {
	var mysqlErr *mysqldriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1295
}
