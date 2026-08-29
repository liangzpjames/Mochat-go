package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"jiyi/mochat-go/internal/dashboard"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSaveKeywordEntryLocksLibraryAndCommitsEntryWithVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM mochat_go_keyword_libraries.*tenant_id=\?.*corp_id=\?.*FOR UPDATE`).
		WithArgs(int64(41), 11, 27).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_keyword_entries")).
		WithArgs(11, 27, int64(41), "needle", "enabled").
		WillReturnResult(sqlmock.NewResult(81, 1))
	mock.ExpectExec(`UPDATE mochat_go_keyword_libraries SET draft_version=draft_version\+1.*tenant_id=\?.*corp_id=\?`).
		WithArgs(int64(41), 11, 27).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	id, err := NewMySQLStore(db).SaveKeywordEntry(context.Background(), 11, 27, dashboard.KeywordEntry{LibraryID: 41, Keyword: "needle", Status: "enabled"})
	if err != nil || id != 81 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveKeywordEntryRollsBackEntryWhenVersionUpdateFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM mochat_go_keyword_libraries.*FOR UPDATE`).
		WithArgs(int64(41), 11, 27).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO mochat_go_keyword_entries")).WillReturnResult(sqlmock.NewResult(81, 1))
	mock.ExpectExec(`UPDATE mochat_go_keyword_libraries SET draft_version=draft_version\+1`).WillReturnError(errors.New("injected version failure"))
	mock.ExpectRollback()

	id, err := NewMySQLStore(db).SaveKeywordEntry(context.Background(), 11, 27, dashboard.KeywordEntry{LibraryID: 41, Keyword: "needle", Status: "enabled"})
	if err == nil || id != 0 {
		t.Fatalf("id=%d err=%v, want rollback", id, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveKeywordEntryRejectsWrongTenantWithoutPartialWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM mochat_go_keyword_libraries.*FOR UPDATE`).
		WithArgs(int64(41), 99, 27).
		WillReturnError(sqlmock.ErrCancelled)
	mock.ExpectRollback()

	if _, err := NewMySQLStore(db).SaveKeywordEntry(context.Background(), 99, 27, dashboard.KeywordEntry{LibraryID: 41, Keyword: "needle", Status: "enabled"}); err == nil {
		t.Fatal("wrong tenant unexpectedly wrote keyword")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestKeywordEntryAtomicityAndConcurrentVersionsAgainstIsolatedMySQL(t *testing.T) {
	dsn := integrationDSNForTask6(t)
	db := task6IntegrationDB(t, dsn)
	store := NewMySQLStore(db)
	ctx := context.Background()
	result, err := db.Exec(`INSERT INTO mochat_go_keyword_libraries(tenant_id,corp_id,name,description,match_mode,status) VALUES(11,27,'task6','','contains','enabled')`)
	if err != nil {
		t.Fatal(err)
	}
	libraryID, _ := result.LastInsertId()

	if _, err := db.Exec(`CREATE TRIGGER task6_fail_keyword_version BEFORE UPDATE ON mochat_go_keyword_libraries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='task6 version failure'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveKeywordEntry(ctx, 11, 27, dashboard.KeywordEntry{LibraryID: libraryID, Keyword: "rollback", Status: "enabled"}); err == nil {
		t.Fatal("version failure unexpectedly committed entry")
	}
	if _, err := db.Exec(`DROP TRIGGER task6_fail_keyword_version`); err != nil {
		t.Fatal(err)
	}
	var partial int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_keyword_entries WHERE library_id=? AND keyword='rollback'`, libraryID).Scan(&partial); err != nil || partial != 0 {
		t.Fatalf("partial entries=%d err=%v", partial, err)
	}

	const workers = 16
	var failures atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		keyword := fmt.Sprintf("keyword-%02d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.SaveKeywordEntry(ctx, 11, 27, dashboard.KeywordEntry{LibraryID: libraryID, Keyword: keyword, Status: "enabled"}); err != nil {
				failures.Add(1)
			}
		}()
	}
	wg.Wait()
	if failures.Load() != 0 {
		t.Fatalf("concurrent failures=%d", failures.Load())
	}
	var entries, draftVersion int
	if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_keyword_entries WHERE tenant_id=11 AND corp_id=27 AND library_id=?`, libraryID).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT draft_version FROM mochat_go_keyword_libraries WHERE id=? AND tenant_id=11 AND corp_id=27`, libraryID).Scan(&draftVersion); err != nil {
		t.Fatal(err)
	}
	if entries != workers || draftVersion != 1+workers {
		t.Fatalf("entries=%d draft_version=%d", entries, draftVersion)
	}
	if _, err := store.SaveKeywordEntry(ctx, 99, 27, dashboard.KeywordEntry{LibraryID: libraryID, Keyword: "wrong-tenant", Status: "enabled"}); err == nil {
		t.Fatal("wrong tenant unexpectedly wrote keyword")
	}
}

func integrationDSNForTask6(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("MOCHAT_GO_MYSQL_INTEGRATION_DSN"))
	if dsn == "" {
		t.Skip("SKIP: MOCHAT_GO_MYSQL_INTEGRATION_DSN is not set; isolated MariaDB/MySQL DSN is required")
	}
	return dsn
}
