package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"testing"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/modules/providers"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
	"jiyi/mochat-go/internal/wecomcredentials"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestArchiveComponentLocatorUsesEncryptedTenantBoundTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := archiveComponentTestCipher(t)
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT id.*FROM mochat_go_archive_component_locators").WithArgs(int64(11), int64(27), "dz-msg-1").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO mochat_go_archive_component_locators").WithArgs(
		sqlmock.AnyArg(), int64(11), int64(27), "dz-msg-1", "wecom:third_party_delegated:ww-local", uint32(1), sqlmock.AnyArg(), "component-v1",
	).WillReturnResult(sqlmock.NewResult(1, 1))
	message := archiveprovider.Message{
		Source: providers.SourceExternal, SourceID: "wecom:third_party_delegated:ww-local", Namespace: "wecom:third_party_delegated:ww-local",
		MsgID: "dz-msg-1", ContentPolicy: archiveprovider.ContentPolicyComponent,
		Component: &archiveprovider.ComponentDescriptor{MessageID: "dz-msg-1", PublicKeyVersion: 1, EncryptedSecretKey: "wrapped-private-key"},
	}
	if err := upsertArchiveComponentTx(context.Background(), tx, manager, archiveprovider.Scope{TenantID: 11, CorpID: 27}, message); err != nil {
		t.Fatal(err)
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveComponentLocatorReadDecryptsOnlyMatchingTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := archiveComponentTestCipher(t)
	id := "014c1da7-1b2e-4aa1-90aa-a6a0d6f53380"
	ciphertext, keyID, err := manager.EncryptArchiveComponent(11, id, wecomcredentials.ArchiveComponentCredential{MessageID: "dz-msg-1", PublicKeyVersion: 1, EncryptedSecretKey: "wrapped-private-key"})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("(?s)SELECT locator.id,locator.tenant_id.*binding.wecom_integration_mode='third_party_delegated'.*locator.id=\\? AND locator.tenant_id=\\? AND locator.corp_id=\\?.*EXISTS.*mc_work_message_1.*message.to_user_type IN \\(\\?,\\?,\\?\\).*AND \\(message.to_user_type=\\? OR message.to_user_type=\\? OR message.to_user_type=\\?\\)").WithArgs(id, 11, 27, 0, 1, 2, 0, 1, 2).WillReturnRows(sqlmock.NewRows([]string{
		"id", "tenant_id", "corp_id", "wx_corpid", "msgid", "source_identity", "public_key_version", "locator_ciphertext", "locator_key_id",
	}).AddRow(id, 11, 27, "ww-local", "dz-msg-1", "wecom:third_party_delegated:ww-local", 1, ciphertext, keyID))
	item, found, err := NewMySQLStore(db).WithWeComCredentialCipher(manager).ArchiveComponentByID(context.Background(), dashboard.ArchiveComponentFilter{
		ID: id, TenantID: 11, CorpID: 27, ConversationScopes: []dashboard.ArchiveMediaConversationScope{{ConversationType: 0}, {ConversationType: 1}, {ConversationType: 2}},
	})
	if err != nil || !found || item.EncryptedSecretKey != "wrapped-private-key" || item.WXCorpID != "ww-local" {
		t.Fatalf("item=%#v found=%v err=%v", item, found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveComponentLocatorReadAppliesRestrictedEmployeeScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := archiveComponentTestCipher(t)
	id := "014c1da7-1b2e-4aa1-90aa-a6a0d6f53380"
	mock.ExpectQuery("(?s)message.to_user_type IN \\(\\?\\).*AND \\(\\(message.to_user_type=\\? AND message.work_employee_id IN \\(\\?,\\?\\)\\)\\)").
		WithArgs(id, 11, 27, 1, 1, 100, 101).
		WillReturnError(sql.ErrNoRows)
	_, found, err := NewMySQLStore(db).WithWeComCredentialCipher(manager).ArchiveComponentByID(context.Background(), dashboard.ArchiveComponentFilter{
		ID: id, TenantID: 11, CorpID: 27,
		ConversationScopes: []dashboard.ArchiveMediaConversationScope{{ConversationType: 1, RestrictEmployeeIDs: true, AllowedEmployeeIDs: []int{100, 101}}},
	})
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func archiveComponentTestCipher(t *testing.T) *wecomcredentials.Manager {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{EncryptionKey: key, EncryptionKeyID: "component-v1", RequireEncryption: true})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}
