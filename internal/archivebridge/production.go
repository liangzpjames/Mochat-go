package archivebridge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"jiyi/mochat-go/internal/wecomarchivedemo"
	"jiyi/mochat-go/internal/wecomcredentials"
)

type FinanceSDKFactory func(corpID, archiveSecret string) (wecomarchivedemo.FinanceSDK, error)

// MySQLProductionProvider reads only authoritative binding metadata from the
// binding source. Decryption remains inside the DriverFactory method and the
// plaintext credential is never returned to the registrar or Store.
type MySQLProductionProvider struct {
	db            *sql.DB
	credentials   *wecomcredentials.Manager
	stateRoot     string
	newFinanceSDK FinanceSDKFactory
}

func NewMySQLProductionProvider(db *sql.DB, credentials *wecomcredentials.Manager, stateRoot string, newFinanceSDK FinanceSDKFactory) (*MySQLProductionProvider, error) {
	if db == nil || credentials == nil || !credentials.ConfigStatus().EncryptionConfigured || strings.TrimSpace(stateRoot) == "" {
		return nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
	}
	if newFinanceSDK == nil {
		newFinanceSDK = wecomarchivedemo.NewFinanceSDK
	}
	return &MySQLProductionProvider{db: db, credentials: credentials, stateRoot: strings.TrimSpace(stateRoot), newFinanceSDK: newFinanceSDK}, nil
}

func (p *MySQLProductionProvider) ProductionBindings(ctx context.Context) ([]Binding, error) {
	if p == nil || p.db == nil || ctx == nil {
		return nil, &BridgeError{Code: "ARCHIVE_BINDING_SOURCE_UNAVAILABLE"}
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT binding.tenant_id,
		       binding.corp_id,
		       integration.verified_wx_corpid,
		       binding.wecom_integration_mode
		FROM mochat_go_tenant_corp_bindings binding
		INNER JOIN mc_tenant tenant
		        ON tenant.id=binding.tenant_id
		       AND tenant.status=1
		       AND tenant.deleted_at IS NULL
		INNER JOIN mc_corp corp
		        ON corp.id=binding.corp_id
		       AND corp.tenant_id=binding.tenant_id
		       AND corp.deleted_at IS NULL
		INNER JOIN mochat_go_wecom_integrations integration
		        ON integration.tenant_id=binding.tenant_id
		       AND integration.corp_id=binding.corp_id
		       AND integration.mode=binding.wecom_integration_mode
		       AND integration.slot='current'
		       AND integration.status='active'
		WHERE integration.verified_wx_corpid<>''
		  AND integration.verified_at IS NOT NULL
		  AND binding.status=2
		  AND binding.verified_at IS NOT NULL
		  AND binding.verified_wx_corpid=integration.verified_wx_corpid
		  AND JSON_VALID(integration.scope_json)=1
		  AND JSON_CONTAINS(integration.scope_json,JSON_QUOTE('archive.read'))=1
		  AND JSON_VALID(integration.missing_capabilities_json)=1
		  AND JSON_LENGTH(integration.missing_capabilities_json)=0
		  AND integration.verification_level<>'local_contract'
		ORDER BY binding.tenant_id ASC,binding.corp_id ASC
	`)
	if err != nil {
		return nil, &BridgeError{Code: "ARCHIVE_BINDING_SOURCE_UNAVAILABLE", Cause: err}
	}
	defer rows.Close()
	bindings := make([]Binding, 0)
	for rows.Next() {
		var binding Binding
		if err := rows.Scan(&binding.TenantID, &binding.CorpID, &binding.WXCorpID, &binding.IntegrationMode); err != nil {
			return nil, &BridgeError{Code: "ARCHIVE_BINDING_SOURCE_UNAVAILABLE", Cause: err}
		}
		if err := normalizeBinding(&binding); err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, &BridgeError{Code: "ARCHIVE_BINDING_SOURCE_UNAVAILABLE", Cause: err}
	}
	return bindings, nil
}

func (p *MySQLProductionProvider) NewFinanceDriver(ctx context.Context, binding Binding) (FinanceDriver, io.Closer, error) {
	if p == nil || p.db == nil || p.credentials == nil || ctx == nil || normalizeBinding(&binding) != nil || binding.IntegrationMode != ModeSelfBuilt {
		return nil, nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
	}
	var (
		tenantID   int64
		corpID     int64
		wxCorpID   string
		ciphertext string
		keyID      string
	)
	err := p.db.QueryRowContext(ctx, `
		SELECT corp.tenant_id,
		       corp.id,
		       corp.wx_corpid,
		       COALESCE(CAST(corp.wecom_credentials_ciphertext AS CHAR),''),
		       COALESCE(corp.wecom_credentials_key_id,'')
		FROM mc_corp corp
		INNER JOIN mc_tenant tenant
		        ON tenant.id=corp.tenant_id
		       AND tenant.status=1
		       AND tenant.deleted_at IS NULL
		INNER JOIN mochat_go_tenant_corp_bindings binding
		        ON binding.tenant_id=corp.tenant_id
		       AND binding.corp_id=corp.id
		       AND binding.status=2
		       AND binding.verified_at IS NOT NULL
		       AND binding.wecom_integration_mode='self_built'
		INNER JOIN mochat_go_wecom_integrations integration
		        ON integration.tenant_id=binding.tenant_id
		       AND integration.corp_id=binding.corp_id
		       AND integration.slot='current'
		       AND integration.status='active'
		       AND integration.mode=binding.wecom_integration_mode
		       AND integration.verified_at IS NOT NULL
		       AND integration.verified_wx_corpid<>''
		       AND binding.verified_wx_corpid=integration.verified_wx_corpid
		       AND JSON_VALID(integration.scope_json)=1
		       AND JSON_CONTAINS(integration.scope_json,JSON_QUOTE('archive.read'))=1
		       AND JSON_VALID(integration.missing_capabilities_json)=1
		       AND JSON_LENGTH(integration.missing_capabilities_json)=0
		       AND integration.verification_level<>'local_contract'
		WHERE corp.tenant_id=?
		  AND corp.id=?
		  AND corp.deleted_at IS NULL
		LIMIT 1
	`, binding.TenantID, binding.CorpID).Scan(&tenantID, &corpID, &wxCorpID, &ciphertext, &keyID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
	}
	if err != nil {
		return nil, nil, &BridgeError{Code: "ARCHIVE_DRIVER_INITIALIZATION_FAILED", Cause: err}
	}
	if tenantID != binding.TenantID || corpID != binding.CorpID || strings.TrimSpace(wxCorpID) != binding.WXCorpID || strings.TrimSpace(ciphertext) == "" || strings.TrimSpace(keyID) == "" {
		return nil, nil, &BridgeError{Code: "ARCHIVE_BINDING_MISMATCH"}
	}
	credential, err := p.credentials.DecryptCorp(int(tenantID), binding.WXCorpID, keyID, ciphertext)
	if err != nil {
		return nil, nil, &BridgeError{Code: "ARCHIVE_DRIVER_INITIALIZATION_FAILED", Cause: err}
	}
	if strings.TrimSpace(credential.ChatSecret) == "" || strings.TrimSpace(credential.ArchiveRSAPrivateKey) == "" {
		return nil, nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
	}
	sdk, err := p.newFinanceSDK(binding.WXCorpID, credential.ChatSecret)
	if err != nil || sdk == nil {
		return nil, nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE", Cause: err}
	}
	evidence, err := wecomarchivedemo.NewEvidenceStore(filepath.Join(p.stateRoot, fmt.Sprintf("%d-%d", binding.TenantID, binding.CorpID)))
	if err != nil {
		return nil, nil, closeFinanceSDKAfterFailure(&BridgeError{Code: "ARCHIVE_DRIVER_INITIALIZATION_FAILED", Cause: err}, sdk)
	}
	driver, err := wecomarchivedemo.NewArchiveService(sdk, credential.ArchiveRSAPrivateKey, evidence, 100, 5)
	if err != nil {
		return nil, nil, closeFinanceSDKAfterFailure(&BridgeError{Code: "ARCHIVE_DRIVER_INITIALIZATION_FAILED", Cause: err}, sdk)
	}
	return driver, sdk, nil
}

func (p *MySQLProductionProvider) NewDataZoneDriver(context.Context, Binding) (DataZoneDriver, io.Closer, error) {
	return nil, nil, &BridgeError{Code: "ARCHIVE_DRIVER_UNAVAILABLE"}
}

func closeFinanceSDKAfterFailure(primary error, sdk wecomarchivedemo.FinanceSDK) error {
	if sdk == nil {
		return primary
	}
	if err := sdk.Close(); err != nil {
		return errors.Join(primary, &BridgeError{Code: "ARCHIVE_DRIVER_CLOSE_FAILED", Cause: err})
	}
	return primary
}
