package mysqlerror

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestIsMissingTable(t *testing.T) {
	if !IsMissingTable(&mysql.MySQLError{Number: 1146}) {
		t.Fatal("1146 should be missing table")
	}
	if IsMissingTable(&mysql.MySQLError{Number: 1062}) || IsMissingTable(errors.New("missing")) {
		t.Fatal("unrelated errors must not match")
	}
}
