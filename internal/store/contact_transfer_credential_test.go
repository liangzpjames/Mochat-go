package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"jiyi/mochat-go/internal/dashboard"
)

func TestRoomWelcomeCorpCredentialByIDRejectsPlaintextLocalSimulationCredential(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, COALESCE(tenant_id, 0), COALESCE(wx_corpid, ''),")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "wx_corpid", "ciphertext", "key_id"}).
			AddRow(1, 1, "wwSIM00000000000001", `{"v":1}`, "preview-wecom-v1"))
	credential, found, err := NewMySQLStore(db).RoomWelcomeCorpCredentialByID(context.Background(), 1)
	if err == nil || found || credential != (dashboard.RoomWelcomeCorpCredential{}) {
		t.Fatalf("plaintext simulation credential must fail closed: credential=%+v found=%t err=%v", credential, found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
