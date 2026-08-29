package migration

import "jiyi/mochat-go/internal/sqlscript"

func SplitSQLStatements(script string) ([]string, error) {
	return sqlscript.Split(script)
}
