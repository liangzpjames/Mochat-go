package migration

import (
	"fmt"
	"strings"
)

func SplitSQLStatements(script string) ([]string, error) {
	statements := make([]string, 0)
	var b strings.Builder
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false
	escaped := false

	for i := 0; i < len(script); i++ {
		ch := script[i]
		var next byte
		if i+1 < len(script) {
			next = script[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				b.WriteByte(ch)
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if !inSingle && !inDouble && !inBacktick {
			if ch == '-' && next == '-' && isSQLCommentBoundary(script, i+2) {
				inLineComment = true
				i++
				continue
			}
			if ch == '#' {
				inLineComment = true
				continue
			}
			if ch == '/' && next == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		b.WriteByte(ch)

		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && (inSingle || inDouble) {
			escaped = true
			continue
		}

		switch ch {
		case '\'':
			if !inDouble && !inBacktick {
				if inSingle && next == '\'' {
					b.WriteByte(next)
					i++
					continue
				}
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !inBacktick {
				if inDouble && next == '"' {
					b.WriteByte(next)
					i++
					continue
				}
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		case ';':
			if !inSingle && !inDouble && !inBacktick {
				statement := strings.TrimSpace(b.String())
				statement = strings.TrimSuffix(statement, ";")
				statement = strings.TrimSpace(statement)
				if statement != "" {
					statements = append(statements, statement)
				}
				b.Reset()
			}
		}
	}

	if inSingle || inDouble || inBacktick || inBlockComment {
		return nil, fmt.Errorf("unterminated SQL script")
	}
	statement := strings.TrimSpace(b.String())
	if statement != "" {
		statements = append(statements, statement)
	}
	return statements, nil
}

func isSQLCommentBoundary(script string, pos int) bool {
	if pos >= len(script) {
		return true
	}
	ch := script[pos]
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}
