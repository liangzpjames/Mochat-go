package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitSQLStatementsHandlesCommentsAndQuotedSemicolons(t *testing.T) {
	script := `
-- comment with ; should be ignored
CREATE TABLE one (name varchar(255) DEFAULT 'a;b');
INSERT INTO one VALUES ("x;y", 'z'';q');
/* block ; comment */
CREATE TABLE ` + "`two;name`" + ` (id int);
`
	statements, err := SplitSQLStatements(script)
	if err != nil {
		t.Fatal(err)
	}
	if len(statements) != 3 {
		t.Fatalf("statement count = %d: %#v", len(statements), statements)
	}
	if statements[0] != "CREATE TABLE one (name varchar(255) DEFAULT 'a;b')" {
		t.Fatalf("first = %q", statements[0])
	}
	if statements[1] != `INSERT INTO one VALUES ("x;y", 'z'';q')` {
		t.Fatalf("second = %q", statements[1])
	}
	if statements[2] != "CREATE TABLE `two;name` (id int)" {
		t.Fatalf("third = %q", statements[2])
	}
}

func TestSplitSQLStatementsRejectsUnterminatedScript(t *testing.T) {
	if _, err := SplitSQLStatements("INSERT INTO one VALUES ('unterminated);"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSplitSQLStatementsDoesNotEmit0138HeaderCommentAsSQL(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "deploy", "standalone", "migrations", "0138_archive_source_sync.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	statements, err := SplitSQLStatements(string(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(statements) < 10 {
		t.Fatalf("0138 statements=%d, want the complete migration", len(statements))
	}
	for index, statement := range statements {
		if strings.Contains(strings.ToLower(statement), "an existing same-named table") {
			t.Fatalf("statement %d contains a fragment of the header comment: %q", index, statement)
		}
	}
}

func TestSplitSQLStatementsEndsDashCommentAtCarriageReturn(t *testing.T) {
	statements, err := SplitSQLStatements("-- comment;\rCREATE TABLE one (id int);")
	if err != nil {
		t.Fatal(err)
	}
	if len(statements) != 1 || statements[0] != "CREATE TABLE one (id int)" {
		t.Fatalf("statements=%#v", statements)
	}
}
