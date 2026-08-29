package archivebridge

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"

	"jiyi/mochat-go/internal/wecomarchivedemo"
	"jiyi/mochat-go/internal/wecomcredentials"

	"github.com/DATA-DOG/go-sqlmock"
)

type productionFinanceSDK struct {
	closed bool
}

const productionCredentialEligibilityQuery = "(?s)FROM mc_corp corp.*INNER JOIN mc_tenant tenant.*tenant.status=1.*INNER JOIN mochat_go_tenant_corp_bindings binding.*binding.status=2.*binding.verified_at IS NOT NULL.*INNER JOIN mochat_go_wecom_integrations integration.*integration.slot='current'.*integration.status='active'.*integration.mode=binding.wecom_integration_mode.*integration.verified_at IS NOT NULL.*integration.verified_wx_corpid<>''.*binding.verified_wx_corpid=integration.verified_wx_corpid.*JSON_VALID\\(integration.scope_json\\)=1.*JSON_CONTAINS\\(integration.scope_json,JSON_QUOTE\\('archive.read'\\)\\)=1.*JSON_VALID\\(integration.missing_capabilities_json\\)=1.*JSON_LENGTH\\(integration.missing_capabilities_json\\)=0.*integration.verification_level<>'local_contract'.*WHERE corp.tenant_id=\\?.*corp.id=\\?"

func (s *productionFinanceSDK) GetChatData(uint64, uint32, int) ([]byte, error) {
	return []byte(`{"errcode":0,"chatdata":[]}`), nil
}

func (s *productionFinanceSDK) DecryptData(string, string) ([]byte, error) {
	return nil, nil
}

func (s *productionFinanceSDK) GetMediaData(context.Context, string, string, int) (wecomarchivedemo.MediaChunk, error) {
	return wecomarchivedemo.MediaChunk{Data: []byte("production-contract-media"), Finished: true}, nil
}

func (s *productionFinanceSDK) Close() error {
	s.closed = true
	return nil
}

func TestMySQLProductionProviderLoadsMetadataAndBuildsFinanceFromEncryptedCredential(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:   base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")),
		EncryptionKeyID: "archive-production-v1", RequireEncryption: true, DedicatedConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	privateKey := productionPrivateKeyPEM(t)
	ciphertext, keyID, err := manager.EncryptCorp(11, "ww-production", wecomcredentials.CorpCredential{
		ChatSecret: "controlled-archive-secret", ArchiveRSAPublicKey: "controlled-public-key", ArchiveRSAPrivateKey: privateKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-production", IntegrationMode: ModeSelfBuilt}
	mock.ExpectQuery("(?s)FROM mochat_go_tenant_corp_bindings.*integration.verified_wx_corpid<>''.*integration.verified_at IS NOT NULL.*binding.status=2.*binding.verified_at IS NOT NULL.*binding.verified_wx_corpid=integration.verified_wx_corpid.*JSON_VALID\\(integration.scope_json\\)=1.*JSON_CONTAINS\\(integration.scope_json,JSON_QUOTE\\('archive.read'\\)\\)=1.*JSON_VALID\\(integration.missing_capabilities_json\\)=1.*JSON_LENGTH\\(integration.missing_capabilities_json\\)=0.*integration.verification_level<>'local_contract'").WillReturnRows(
		sqlmock.NewRows([]string{"tenant_id", "corp_id", "wx_corpid", "integration_mode"}).AddRow(binding.TenantID, binding.CorpID, binding.WXCorpID, binding.IntegrationMode),
	)
	mock.ExpectQuery(productionCredentialEligibilityQuery).WithArgs(binding.TenantID, binding.CorpID).WillReturnRows(
		sqlmock.NewRows([]string{"tenant_id", "corp_id", "wx_corpid", "credential_ciphertext", "credential_key_id"}).AddRow(binding.TenantID, binding.CorpID, binding.WXCorpID, ciphertext, keyID),
	)
	sdk := &productionFinanceSDK{}
	provider, err := NewMySQLProductionProvider(db, manager, t.TempDir(), func(corpID, secret string) (wecomarchivedemo.FinanceSDK, error) {
		if corpID != binding.WXCorpID || secret != "controlled-archive-secret" {
			t.Fatalf("factory credential scope=%q secret_match=%t", corpID, secret == "controlled-archive-secret")
		}
		return sdk, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := provider.ProductionBindings(context.Background())
	if err != nil || len(bindings) != 1 || bindings[0] != binding {
		t.Fatalf("bindings=%+v err=%v", bindings, err)
	}
	driver, closer, err := provider.NewFinanceDriver(context.Background(), binding)
	if err != nil {
		t.Fatal(err)
	}
	page, err := driver.FetchPage(context.Background(), 7, 10)
	if err != nil || page.StartSeq != 7 || page.NextSeq != 7 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if err := closer.Close(); err != nil || !sdk.closed {
		t.Fatalf("sdk closed=%t err=%v", sdk.closed, err)
	}
	if _, _, err := provider.NewDataZoneDriver(context.Background(), Binding{TenantID: 21, CorpID: 37, WXCorpID: "ww-delegated", IntegrationMode: ModeThirdPartyDelegated}); ErrorCode(err) != "ARCHIVE_DRIVER_UNAVAILABLE" {
		t.Fatalf("data-zone error=%v code=%s", err, ErrorCode(err))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLProductionProviderRejectsRevokedEligibilityBeforeCredentialUse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager, err := wecomcredentials.NewManager(wecomcredentials.Config{
		EncryptionKey:   base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")),
		EncryptionKeyID: "archive-production-v1", RequireEncryption: true, DedicatedConfigured: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding := Binding{TenantID: 11, CorpID: 27, WXCorpID: "ww-production", IntegrationMode: ModeSelfBuilt}
	mock.ExpectQuery("(?s)FROM mochat_go_tenant_corp_bindings.*integration.verification_level<>'local_contract'").WillReturnRows(
		sqlmock.NewRows([]string{"tenant_id", "corp_id", "wx_corpid", "integration_mode"}).AddRow(binding.TenantID, binding.CorpID, binding.WXCorpID, binding.IntegrationMode),
	)
	mock.ExpectQuery(productionCredentialEligibilityQuery).WithArgs(binding.TenantID, binding.CorpID).WillReturnRows(
		sqlmock.NewRows([]string{"tenant_id", "corp_id", "wx_corpid", "credential_ciphertext", "credential_key_id"}),
	)
	factoryCalled := false
	provider, err := NewMySQLProductionProvider(db, manager, t.TempDir(), func(string, string) (wecomarchivedemo.FinanceSDK, error) {
		factoryCalled = true
		return &productionFinanceSDK{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	bindings, err := provider.ProductionBindings(context.Background())
	if err != nil || len(bindings) != 1 {
		t.Fatalf("bindings=%+v err=%v", bindings, err)
	}
	if driver, closer, err := provider.NewFinanceDriver(context.Background(), bindings[0]); driver != nil || closer != nil || ErrorCode(err) != "ARCHIVE_DRIVER_UNAVAILABLE" {
		t.Fatalf("driver=%v closer=%v err=%v code=%s", driver, closer, err, ErrorCode(err))
	}
	if factoryCalled {
		t.Fatal("finance SDK factory ran after archive eligibility was revoked")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func productionPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}
