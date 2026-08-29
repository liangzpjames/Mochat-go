package store

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestWeWorkCallbackCorpLookupRequiresActiveVerifiedBinding(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	mock.ExpectQuery(regexp.QuoteMeta(authoritativeWeWorkCallbackCorpSelect) + `(?s).*WHERE c.id=\?`).
		WithArgs(27).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "wx_corpid", "ciphertext", "key_id"}))

	if _, found, err := store.WeWorkCallbackCorpByID(context.Background(), 27); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestWeWorkCallbackStoredFailureRedactsProviderURLCredentials(t *testing.T) {
	got := truncateWeWorkCallbackError(`POST "https://qyapi.weixin.qq.com/cgi-bin/user/get?access_token=do-not-store": timeout`)
	if strings.Contains(got, "do-not-store") || strings.Contains(strings.ToLower(got), "access_token=") {
		t.Fatalf("stored failure was not redacted: %q", got)
	}
}

func TestWeWorkCallbackCorpLookupRejectsAmbiguousVerifiedWXCorpID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	mock.ExpectQuery(regexp.QuoteMeta(authoritativeWeWorkCallbackCorpSelect) + `(?s).*WHERE c.wx_corpid=\?.*LIMIT 2`).
		WithArgs("wx-shared").
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "wx_corpid", "ciphertext", "key_id"}).
			AddRow(27, 3, "wx-shared", "cipher-1", "key-1").
			AddRow(28, 4, "wx-shared", "cipher-2", "key-2"))

	if _, found, err := store.WeWorkCallbackCorpByWXID(context.Background(), "wx-shared"); err == nil || found {
		t.Fatalf("found=%v err=%v, want ambiguous lookup failure", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
