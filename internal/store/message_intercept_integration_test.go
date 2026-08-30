package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestDeleteKeywordEntryLocksLibraryBeforeEntryAndChecksEveryWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT library_id FROM mochat_go_keyword_entries WHERE id=\? AND tenant_id=\? AND corp_id=\?`).
		WithArgs(int64(81), 11, 27).
		WillReturnRows(sqlmock.NewRows([]string{"library_id"}).AddRow(41))
	mock.ExpectQuery(`SELECT id FROM mochat_go_keyword_libraries.*FOR UPDATE`).
		WithArgs(int64(41), 11, 27).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectQuery(`SELECT id FROM mochat_go_keyword_entries.*library_id=\?.*FOR UPDATE`).
		WithArgs(int64(81), 11, 27, int64(41)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(81))
	mock.ExpectExec(`DELETE FROM mochat_go_keyword_entries`).
		WithArgs(int64(81), 11, 27, int64(41)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE mochat_go_keyword_libraries SET draft_version=draft_version\+1`).
		WithArgs(int64(41), 11, 27).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	deleted, err := NewMySQLStore(db).DeleteKeywordEntry(context.Background(), 11, 27, 81)
	if err != nil || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSetKeywordEntryStatusReturnsRowsAffectedErrorAndRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT library_id FROM mochat_go_keyword_entries`).WillReturnRows(sqlmock.NewRows([]string{"library_id"}).AddRow(41))
	mock.ExpectQuery(`SELECT id FROM mochat_go_keyword_libraries.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(41))
	mock.ExpectQuery(`SELECT id FROM mochat_go_keyword_entries.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(81))
	mock.ExpectExec(`UPDATE mochat_go_keyword_entries SET status=\?`).WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected failure")))
	mock.ExpectRollback()

	updated, err := NewMySQLStore(db).SetKeywordEntryStatus(context.Background(), 11, 27, 81, "disabled")
	if err == nil || updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestKeywordEntryAtomicityAndConcurrentVersionsAgainstIsolatedMySQL(t *testing.T) {
	db := newCurrentStoreIntegrationDB(t)
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

	deleteEntryID, err := store.SaveKeywordEntry(ctx, 11, 27, dashboard.KeywordEntry{LibraryID: libraryID, Keyword: "delete-once", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	var beforeDeleteVersion int
	if err := db.QueryRow(`SELECT draft_version FROM mochat_go_keyword_libraries WHERE id=?`, libraryID).Scan(&beforeDeleteVersion); err != nil {
		t.Fatal(err)
	}
	var deleteSuccesses atomic.Int64
	var deleteFailures atomic.Int64
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deleted, err := store.DeleteKeywordEntry(ctx, 11, 27, deleteEntryID)
			if err != nil {
				deleteFailures.Add(1)
				return
			}
			if deleted {
				deleteSuccesses.Add(1)
			}
		}()
	}
	wg.Wait()
	if deleteFailures.Load() != 0 || deleteSuccesses.Load() != 1 {
		t.Fatalf("same-entry delete successes=%d failures=%d", deleteSuccesses.Load(), deleteFailures.Load())
	}
	var afterDeleteVersion int
	if err := db.QueryRow(`SELECT draft_version FROM mochat_go_keyword_libraries WHERE id=?`, libraryID).Scan(&afterDeleteVersion); err != nil {
		t.Fatal(err)
	}
	if afterDeleteVersion != beforeDeleteVersion+1 {
		t.Fatalf("same-entry delete draft_version before=%d after=%d", beforeDeleteVersion, afterDeleteVersion)
	}

	for attempt := 0; attempt < 8; attempt++ {
		entryID, err := store.SaveKeywordEntry(ctx, 11, 27, dashboard.KeywordEntry{LibraryID: libraryID, Keyword: fmt.Sprintf("save-delete-%d", attempt), Status: "enabled"})
		if err != nil {
			t.Fatal(err)
		}
		var versionBeforeRace int
		if err := db.QueryRow(`SELECT draft_version FROM mochat_go_keyword_libraries WHERE id=?`, libraryID).Scan(&versionBeforeRace); err != nil {
			t.Fatal(err)
		}
		raceCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		start := make(chan struct{})
		saveResult := make(chan error, 1)
		deleteResult := make(chan struct {
			deleted bool
			err     error
		}, 1)
		go func() {
			<-start
			_, err := store.SaveKeywordEntry(raceCtx, 11, 27, dashboard.KeywordEntry{ID: entryID, LibraryID: libraryID, Keyword: fmt.Sprintf("save-delete-updated-%d", attempt), Status: "disabled"})
			saveResult <- err
		}()
		go func() {
			<-start
			deleted, err := store.DeleteKeywordEntry(raceCtx, 11, 27, entryID)
			deleteResult <- struct {
				deleted bool
				err     error
			}{deleted: deleted, err: err}
		}()
		close(start)
		saveErr := <-saveResult
		deleteOutcome := <-deleteResult
		cancel()
		if saveErr != nil && !errors.Is(saveErr, sql.ErrNoRows) {
			t.Fatalf("attempt=%d save err=%v", attempt, saveErr)
		}
		if deleteOutcome.err != nil || !deleteOutcome.deleted {
			t.Fatalf("attempt=%d delete=%v err=%v", attempt, deleteOutcome.deleted, deleteOutcome.err)
		}
		var remaining, versionAfterRace int
		if err := db.QueryRow(`SELECT COUNT(*) FROM mochat_go_keyword_entries WHERE id=?`, entryID).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT draft_version FROM mochat_go_keyword_libraries WHERE id=?`, libraryID).Scan(&versionAfterRace); err != nil {
			t.Fatal(err)
		}
		expectedDelta := 1
		if saveErr == nil {
			expectedDelta = 2
		}
		if remaining != 0 || versionAfterRace != versionBeforeRace+expectedDelta {
			t.Fatalf("attempt=%d remaining=%d version before=%d after=%d delta=%d", attempt, remaining, versionBeforeRace, versionAfterRace, expectedDelta)
		}
	}
}
