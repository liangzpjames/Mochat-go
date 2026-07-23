package migration

import "testing"

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
