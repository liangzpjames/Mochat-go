package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRoomWelcomeCorpCredentialByIDFallsBackToExplicitLocalSimulationCredential(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, COALESCE(tenant_id, 0), COALESCE(wx_corpid, ''),")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "wx_corpid", "ciphertext", "key_id"}).
			AddRow(1, 1, "wwSIM00000000000001", `{"v":1}`, "preview-wecom-v1"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(contact_secret, '') FROM mc_corp WHERE id = ? AND deleted_at IS NULL")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"contact_secret"}).AddRow("SIM-CONTACT-SECRET-0001"))

	credential, found, err := NewMySQLStore(db).RoomWelcomeCorpCredentialByID(context.Background(), 1)
	if err != nil {
		t.Fatalf("RoomWelcomeCorpCredentialByID() error = %v", err)
	}
	if !found || credential.WXCorpID != "wwSIM00000000000001" || credential.ContactSecret != "SIM-CONTACT-SECRET-0001" {
		t.Fatalf("credential=%+v found=%t", credential, found)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
