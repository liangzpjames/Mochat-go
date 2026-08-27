package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/dashboard"
)

func TestContactMessageBatchPageScopesLegacyRowsThroughCorpTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &MySQLStore{db: db}
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT COUNT(*)
		FROM mc_contact_message_batch_send batch
		JOIN mc_corp corp ON corp.id=batch.corp_id
		WHERE batch.user_id = ? AND batch.deleted_at IS NULL AND corp.tenant_id = ? AND batch.corp_id = ?`)).
		WithArgs(31, 7, 9).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	page, err := store.ContactMessageBatchSendPage(context.Background(), dashboard.ContactMessageBatchSendFilter{
		TenantID: 7, CorpID: 9, UserID: 31, Page: 1, PerPage: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("page=%#v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
