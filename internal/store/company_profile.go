package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jiyi/mochat-go/internal/companyprofile"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcredentials"
)

type companyProfileQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type companyBindingRecord struct {
	ActorUserID        int
	TenantID           int
	CorpID             int
	Status             int
	Version            uint64
	EmployeeGeneration uint64
	ContactGeneration  uint64
	AgentGeneration    uint64
	CallbackGeneration uint64
	VerifiedWXCorpID   string
	VerifiedCorpName   string
	VerifiedAt         sql.NullTime
	DisplayName        string
	LegacyWXCorpID     string
	Ciphertext         string
	KeyID              string
	UpdatedAt          sql.NullTime
}

func (s *MySQLStore) GetProfile(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (companyprofile.Profile, error) {
	if s == nil || s.db == nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	if err := s.checkCompanyActor(ctx, s.db, principal, false); err != nil {
		return companyprofile.Profile{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, s.db, principal, false)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	return s.companyProfileFromBinding(ctx, s.db, binding)
}

func (s *MySQLStore) UpdateProfile(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input companyprofile.UpdateProfileInput) (companyprofile.Profile, error) {
	if s == nil || s.db == nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	defer rollbackQuietly(tx)
	if err := s.checkCompanyActor(ctx, tx, principal, true); err != nil {
		return companyprofile.Profile{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if binding.Version != input.ExpectedVersion {
		return companyprofile.Profile{}, companyprofile.ErrVersionConflict
	}
	displayName := strings.TrimSpace(input.DisplayName)
	changed := []string{"displayName"}
	resultVersion, err := updateCompanyBindingVersionTx(ctx, tx, binding, principal.UserID, input.ExpectedVersion,
		func() error {
			result, execErr := tx.ExecContext(ctx, `
				UPDATE mc_corp
				SET name = ?, updated_at = NOW()
				WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
				displayName, binding.CorpID, binding.TenantID)
			if execErr != nil {
				return execErr
			}
			return requireCompanyRows(result, 1)
		},
		"dashboard.company.profile.update", "company", strconv.Itoa(binding.CorpID), changed, input.RequestID)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if resultVersion == 0 {
		return companyprofile.Profile{}, errors.New("company profile version was not advanced")
	}
	if err := tx.Commit(); err != nil {
		return companyprofile.Profile{}, err
	}
	return s.GetProfile(ctx, principal)
}

func (s *MySQLStore) GetVerificationSnapshot(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (companyprofile.VerificationSnapshot, error) {
	if s == nil || s.db == nil {
		return companyprofile.VerificationSnapshot{}, companyprofile.ErrStoreUnavailable
	}
	if err := s.checkCompanyActor(ctx, s.db, principal, false); err != nil {
		return companyprofile.VerificationSnapshot{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, s.db, principal, false)
	if err != nil {
		return companyprofile.VerificationSnapshot{}, err
	}
	credential, found, err := loadEncryptedCorpCredentialByID(ctx, s.db, binding.CorpID, false)
	if err != nil {
		return companyprofile.VerificationSnapshot{}, err
	}
	if !found {
		return companyprofile.VerificationSnapshot{}, companyprofile.ErrNotFound
	}
	secret, err := s.decodeEncryptedCorpCredential(credential)
	if err != nil {
		return companyprofile.VerificationSnapshot{}, companyprofile.ErrStoreUnavailable
	}
	wxCorpID := strings.TrimSpace(binding.VerifiedWXCorpID)
	if wxCorpID == "" {
		wxCorpID = strings.TrimSpace(credential.WXCorpID)
	}
	return companyprofile.VerificationSnapshot{
		Verified:       wxCorpID != "" && strings.TrimSpace(binding.VerifiedWXCorpID) != "",
		WXCorpID:       wxCorpID,
		BindingVersion: binding.Version,
		Credentials: companyprofile.VerificationCredentials{
			EmployeeSecret: secret.EmployeeSecret,
			ContactSecret:  secret.ContactSecret,
			CallbackToken:  secret.CallbackToken,
			EncodingAESKey: secret.EncodingAESKey,
			ChatSecret:     secret.ChatSecret,
		},
	}, nil
}

func (s *MySQLStore) CommitVerification(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input companyprofile.VerifyInput, result companyprofile.VerificationResult) (companyprofile.Profile, error) {
	if s == nil || s.db == nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	defer rollbackQuietly(tx)
	if err := s.checkCompanyActor(ctx, tx, principal, true); err != nil {
		return companyprofile.Profile{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if binding.Version != input.ExpectedVersion {
		return companyprofile.Profile{}, companyprofile.ErrVersionConflict
	}
	wxCorpID := strings.TrimSpace(result.WXCorpID)
	corpName := strings.TrimSpace(result.CorpName)
	if wxCorpID == "" || corpName == "" {
		return companyprofile.Profile{}, companyprofile.ErrCredentialInvalid
	}
	if strings.TrimSpace(binding.VerifiedWXCorpID) != "" && strings.TrimSpace(binding.VerifiedWXCorpID) != wxCorpID {
		return companyprofile.Profile{}, companyprofile.ErrCorpIDImmutable
	}
	var existingTenant, existingCorp int
	err = tx.QueryRowContext(ctx, `
		SELECT tenant_id, corp_id
		FROM mochat_go_tenant_corp_bindings
		WHERE verified_wx_corpid = ? AND NOT (tenant_id = ? AND corp_id = ?)
		LIMIT 1 FOR UPDATE`, wxCorpID, binding.TenantID, binding.CorpID).Scan(&existingTenant, &existingCorp)
	if err == nil {
		return companyprofile.Profile{}, companyprofile.ErrTenantAccessDenied
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	credentialStorage, err := reencryptCompanyCorpCredential(s, corpCredentialRecord{
		ID: binding.CorpID, TenantID: binding.TenantID, WXCorpID: binding.LegacyWXCorpID,
		Ciphertext: binding.Ciphertext, KeyID: binding.KeyID,
	}, wxCorpID)
	if err != nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	var resultVersion uint64
	resultVersion, err = updateCompanyBindingVersionTx(ctx, tx, binding, principal.UserID, input.ExpectedVersion,
		func() error {
			updated, execErr := tx.ExecContext(ctx, `
				UPDATE mochat_go_tenant_corp_bindings
				SET status = 2, verified_wx_corpid = ?, verified_corp_name = ?, verified_at = NOW(), updated_at = NOW()
				WHERE tenant_id = ? AND corp_id = ? AND version = ? AND status IN (1,2)`,
				wxCorpID, corpName, binding.TenantID, binding.CorpID, input.ExpectedVersion)
			if execErr != nil {
				return execErr
			}
			if err := requireCompanyRows(updated, 1); err != nil {
				return err
			}
			updated, execErr = tx.ExecContext(ctx, `
				UPDATE mc_corp
				SET wx_corpid = ?, employee_secret = '', contact_secret = '', token = '', encoding_aes_key = '', chat_secret = '',
					wecom_credentials_ciphertext = ?, wecom_credentials_key_id = ?, updated_at = NOW()
				WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
				wxCorpID, credentialStorage.Ciphertext, credentialStorage.KeyID, binding.CorpID, binding.TenantID)
			if execErr != nil {
				return execErr
			}
			return requireCompanyRows(updated, 1)
		},
		"dashboard.company.verify", "company", strconv.Itoa(binding.CorpID), []string{"verifiedCorpID", "authoritativeCorpName", "bindingStatus"}, input.RequestID)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if resultVersion == 0 {
		return companyprofile.Profile{}, errors.New("company verification version was not advanced")
	}
	if err := tx.Commit(); err != nil {
		return companyprofile.Profile{}, err
	}
	return s.GetProfile(ctx, principal)
}

func (s *MySQLStore) RotateWeComCredentials(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input companyprofile.WeComCredentialsInput) (companyprofile.Profile, error) {
	return s.rotateCorpCredentials(ctx, principal, input, companyCredentialRotationPolicyForWeComInput(input), credentialRotationGroupsForWeComInput(input), "dashboard.company.wecom_credentials.rotate", []string{"employeeSecret", "contactSecret", "callbackToken", "encodingAESKey", "chatSecret"})
}

func (s *MySQLStore) ConfigureApplication(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input companyprofile.ApplicationCredentialsInput) (companyprofile.Profile, error) {
	if s == nil || s.db == nil || s.weComCredentialCipher == nil || !s.weComCredentialCipher.ConfigStatus().EncryptionConfigured {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	defer rollbackQuietly(tx)
	if err := s.checkCompanyActor(ctx, tx, principal, true); err != nil {
		return companyprofile.Profile{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if binding.Version != input.ExpectedVersion {
		return companyprofile.Profile{}, companyprofile.ErrVersionConflict
	}
	current, found, err := loadEncryptedCorpCredentialByID(ctx, tx, binding.CorpID, true)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if !found || current.TenantID != binding.TenantID {
		return companyprofile.Profile{}, companyprofile.ErrNotFound
	}
	credential, err := companyCredentialForRotation(s, current)
	if err != nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	credential.EmployeeSecret = strings.TrimSpace(input.Secret)
	credential.ContactSecret = strings.TrimSpace(input.Secret)
	callbackGenerated := false
	if !companyCallbackConfigurationValid(credential.CallbackToken, credential.EncodingAESKey) {
		if !companyCallbackConfigurationValid(input.CallbackToken, input.EncodingAESKey) {
			input.CallbackToken, input.EncodingAESKey, err = companyprofile.GenerateCallbackConfiguration()
			if err != nil {
				return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
			}
		}
		credential.CallbackToken = strings.TrimSpace(input.CallbackToken)
		credential.EncodingAESKey = strings.TrimSpace(input.EncodingAESKey)
		callbackGenerated = true
	}
	corpStorage, err := s.encodeCorpCredential(binding.TenantID, current.WXCorpID, credential)
	if err != nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	agent, agentFound, err := loadCompanyApplicationAgent(ctx, tx, binding, strings.TrimSpace(input.WXAgentID))
	if err != nil {
		return companyprofile.Profile{}, err
	}
	agentStorage, err := s.encodeAgentCredential(binding.CorpID, strings.TrimSpace(input.WXAgentID), wecomcredentials.AgentCredential{WXSecret: strings.TrimSpace(input.Secret)})
	if err != nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	changedFields := []string{"applicationAgentId", "applicationSecret", "employeeSecret", "contactSecret", "agentSecret"}
	if callbackGenerated {
		changedFields = append(changedFields, "callbackToken", "encodingAESKey")
	}
	groups := companyCredentialRotationGroups{Employee: true, Contact: true, Agent: true, Callback: callbackGenerated}
	_, err = updateCompanyBindingVersionTx(ctx, tx, binding, principal.UserID, input.ExpectedVersion,
		func() error {
			corpResult, execErr := tx.ExecContext(ctx, `
				UPDATE mc_corp
				SET employee_secret='', contact_secret='', token='', encoding_aes_key='',
					wecom_credentials_ciphertext=?, wecom_credentials_key_id=?, updated_at=NOW()
				WHERE id=? AND tenant_id=? AND deleted_at IS NULL`,
				corpStorage.Ciphertext, corpStorage.KeyID, binding.CorpID, binding.TenantID)
			if execErr != nil {
				return execErr
			}
			if err := requireCompanyRows(corpResult, 1); err != nil {
				return err
			}
			if agentFound {
				agentResult, execErr := tx.ExecContext(ctx, `
					UPDATE mc_work_agent
					SET wx_agent_id=?, wx_secret='', wecom_credentials_ciphertext=?, wecom_credentials_key_id=?, updated_at=NOW()
					WHERE id=? AND corp_id=? AND deleted_at IS NULL`,
					strings.TrimSpace(input.WXAgentID), agentStorage.Ciphertext, agentStorage.KeyID, agent.ID, binding.CorpID)
				if execErr != nil {
					return execErr
				}
				if err := requireCompanyRows(agentResult, 1); err != nil {
					return err
				}
				if err := incrementCompanyCredentialGenerationsTx(ctx, tx, binding, groups); err != nil {
					return err
				}
				return clearCompanyBindingVerificationTx(ctx, tx, binding)
			}
			agentResult, execErr := tx.ExecContext(ctx, `
				INSERT INTO mc_work_agent
				(corp_id, wx_agent_id, wx_secret, wecom_credentials_ciphertext, wecom_credentials_key_id, created_at, updated_at)
				VALUES (?, ?, '', ?, ?, NOW(), NOW())`,
				binding.CorpID, strings.TrimSpace(input.WXAgentID), agentStorage.Ciphertext, agentStorage.KeyID)
			if execErr != nil {
				return execErr
			}
			if err := requireCompanyRows(agentResult, 1); err != nil {
				return err
			}
			if err := incrementCompanyCredentialGenerationsTx(ctx, tx, binding, groups); err != nil {
				return err
			}
			return clearCompanyBindingVerificationTx(ctx, tx, binding)
		}, "dashboard.company.application_credentials.configure", "company", strconv.Itoa(binding.CorpID), changedFields, input.RequestID)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if err := tx.Commit(); err != nil {
		return companyprofile.Profile{}, err
	}
	return s.GetProfile(ctx, principal)
}

func (s *MySQLStore) RotateArchiveCredentials(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input companyprofile.ArchiveCredentialsInput) (companyprofile.Profile, error) {
	return s.rotateCorpCredentials(ctx, principal, companyprofile.WeComCredentialsInput{
		ChatSecret: input.ChatSecret, RSAPublicKey: input.RSAPublicKey, RSAPrivateKey: input.RSAPrivateKey,
		ExpectedVersion: input.ExpectedVersion, RequestID: input.RequestID,
	}, companyCredentialRotationPreservesVerification, companyCredentialRotationGroups{}, "dashboard.company.archive_credentials.rotate", []string{"chatSecret", "archiveRsaPublicKey", "archiveRsaPrivateKey"})
}

func (s *MySQLStore) GetCallbackConfiguration(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (companyprofile.CallbackConfiguration, error) {
	if s == nil || s.db == nil {
		return companyprofile.CallbackConfiguration{}, companyprofile.ErrStoreUnavailable
	}
	if err := s.checkCompanyActor(ctx, s.db, principal, false); err != nil {
		return companyprofile.CallbackConfiguration{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, s.db, principal, false)
	if err != nil {
		return companyprofile.CallbackConfiguration{}, err
	}
	current, found, err := loadEncryptedCorpCredentialByID(ctx, s.db, binding.CorpID, false)
	if err != nil {
		return companyprofile.CallbackConfiguration{}, err
	}
	if !found || current.TenantID != binding.TenantID {
		return companyprofile.CallbackConfiguration{}, companyprofile.ErrNotFound
	}
	credential, err := companyCredentialForRotation(s, current)
	if err != nil {
		return companyprofile.CallbackConfiguration{}, companyprofile.ErrStoreUnavailable
	}
	return companyprofile.CallbackConfiguration{
		CorpID: binding.CorpID, Token: credential.CallbackToken, EncodingAESKey: credential.EncodingAESKey,
		Configured:     companyCallbackConfigurationValid(credential.CallbackToken, credential.EncodingAESKey),
		BindingVersion: binding.Version,
	}, nil
}

func companyCallbackConfigurationValid(token, encodingAESKey string) bool {
	token = strings.TrimSpace(token)
	encodingAESKey = strings.TrimSpace(encodingAESKey)
	if len(token) < 1 || len(token) > 32 || len(encodingAESKey) != 43 {
		return false
	}
	for _, character := range encodingAESKey {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')) {
			return false
		}
	}
	return true
}

func (s *MySQLStore) RegenerateCallbackConfiguration(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input companyprofile.CallbackConfigurationInput) (companyprofile.CallbackConfiguration, error) {
	token, aesKey := input.Token, input.EncodingAESKey
	if _, err := s.rotateCorpCredentials(ctx, principal, companyprofile.WeComCredentialsInput{
		CallbackToken: &token, EncodingAESKey: &aesKey, ExpectedVersion: input.ExpectedVersion, RequestID: input.RequestID,
	}, companyCredentialRotationPreservesVerification, companyCredentialRotationGroups{Callback: true}, "dashboard.company.callback_configuration.rotate", []string{"callbackToken", "encodingAESKey"}); err != nil {
		return companyprofile.CallbackConfiguration{}, err
	}
	return s.GetCallbackConfiguration(ctx, principal)
}

type companyCredentialRotationPolicy uint8

const (
	companyCredentialRotationPreservesVerification companyCredentialRotationPolicy = iota
	companyCredentialRotationInvalidatesVerification
)

func companyCredentialRotationPolicyForWeComInput(input companyprofile.WeComCredentialsInput) companyCredentialRotationPolicy {
	if input.EmployeeSecret != nil || input.ContactSecret != nil {
		return companyCredentialRotationInvalidatesVerification
	}
	return companyCredentialRotationPreservesVerification
}

type companyCredentialRotationGroups struct {
	Employee bool
	Contact  bool
	Agent    bool
	Callback bool
}

func credentialRotationGroupsForWeComInput(input companyprofile.WeComCredentialsInput) companyCredentialRotationGroups {
	return companyCredentialRotationGroups{
		Employee: input.EmployeeSecret != nil,
		Contact:  input.ContactSecret != nil,
		Callback: input.CallbackToken != nil || input.EncodingAESKey != nil,
	}
}

func (s *MySQLStore) rotateCorpCredentials(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input companyprofile.WeComCredentialsInput, policy companyCredentialRotationPolicy, groups companyCredentialRotationGroups, action string, fields []string) (companyprofile.Profile, error) {
	if s == nil || s.db == nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	if s.weComCredentialCipher == nil || !s.weComCredentialCipher.ConfigStatus().EncryptionConfigured {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	defer rollbackQuietly(tx)
	if err := s.checkCompanyActor(ctx, tx, principal, true); err != nil {
		return companyprofile.Profile{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if binding.Version != input.ExpectedVersion {
		return companyprofile.Profile{}, companyprofile.ErrVersionConflict
	}
	if input.WXCorpID != nil {
		return companyprofile.Profile{}, companyprofile.ErrCorpIDImmutable
	}
	current, found, err := loadEncryptedCorpCredentialByID(ctx, tx, binding.CorpID, true)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if !found || current.TenantID != binding.TenantID {
		return companyprofile.Profile{}, companyprofile.ErrNotFound
	}
	secret, err := companyCredentialForRotation(s, current)
	if err != nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	if input.EmployeeSecret != nil {
		secret.EmployeeSecret = strings.TrimSpace(*input.EmployeeSecret)
	}
	if input.ContactSecret != nil {
		secret.ContactSecret = strings.TrimSpace(*input.ContactSecret)
	}
	if input.CallbackToken != nil {
		secret.CallbackToken = strings.TrimSpace(*input.CallbackToken)
	}
	if input.EncodingAESKey != nil {
		secret.EncodingAESKey = strings.TrimSpace(*input.EncodingAESKey)
	}
	if input.ChatSecret != nil {
		secret.ChatSecret = strings.TrimSpace(*input.ChatSecret)
	}
	if input.RSAPublicKey != nil {
		secret.ArchiveRSAPublicKey = strings.TrimSpace(*input.RSAPublicKey)
	}
	if input.RSAPrivateKey != nil {
		secret.ArchiveRSAPrivateKey = strings.TrimSpace(*input.RSAPrivateKey)
	}
	wxCorpID := strings.TrimSpace(current.WXCorpID)
	if wxCorpID == "" && binding.Status != 1 {
		return companyprofile.Profile{}, companyprofile.ErrTenantAccessDenied
	}
	storage, err := s.encodeCorpCredential(binding.TenantID, wxCorpID, secret)
	if err != nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	resultVersion, err := updateCompanyBindingVersionTx(ctx, tx, binding, principal.UserID, input.ExpectedVersion,
		func() error {
			updated, execErr := tx.ExecContext(ctx, `
				UPDATE mc_corp
				SET wx_corpid = ?, employee_secret = '', contact_secret = '', token = '', encoding_aes_key = '', chat_secret = '',
					wecom_credentials_ciphertext = ?, wecom_credentials_key_id = ?, updated_at = NOW()
				WHERE id = ? AND tenant_id = ? AND deleted_at IS NULL`,
				wxCorpID, storage.Ciphertext, storage.KeyID, binding.CorpID, binding.TenantID)
			if execErr != nil {
				return execErr
			}
			if err := requireCompanyRows(updated, 1); err != nil {
				return err
			}
			if err := incrementCompanyCredentialGenerationsTx(ctx, tx, binding, groups); err != nil {
				return err
			}
			if policy == companyCredentialRotationInvalidatesVerification {
				return clearCompanyBindingVerificationTx(ctx, tx, binding)
			}
			return nil
		}, action, "company", strconv.Itoa(binding.CorpID), fields, input.RequestID)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if resultVersion == 0 {
		return companyprofile.Profile{}, errors.New("company credential version was not advanced")
	}
	if err := tx.Commit(); err != nil {
		return companyprofile.Profile{}, err
	}
	return s.GetProfile(ctx, principal)
}

func (s *MySQLStore) RotateAgentCredentials(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, input companyprofile.AgentCredentialsInput) (companyprofile.Profile, error) {
	if input.ExpectedVersion == 0 || (input.AgentID <= 0 && strings.TrimSpace(input.WXAgentID) == "") {
		return companyprofile.Profile{}, companyprofile.ErrInvalidRequest
	}
	if s == nil || s.db == nil || s.weComCredentialCipher == nil || !s.weComCredentialCipher.ConfigStatus().EncryptionConfigured {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	defer rollbackQuietly(tx)
	if err := s.checkCompanyActor(ctx, tx, principal, true); err != nil {
		return companyprofile.Profile{}, err
	}
	binding, err := s.loadCompanyBinding(ctx, tx, principal, true)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if binding.Version != input.ExpectedVersion {
		return companyprofile.Profile{}, companyprofile.ErrVersionConflict
	}
	agent, found, err := loadCompanyAgentCredential(ctx, tx, binding, input)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if !found {
		return companyprofile.Profile{}, companyprofile.ErrNotFound
	}
	var storage agentCredentialStorage
	changedFields := []string{"agentSecret"}
	if input.WXSecret == nil || strings.TrimSpace(*input.WXSecret) == "" {
		if _, err := s.decodeAgentCredentialForCompany(agent); err != nil {
			return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
		}
		storage = agentCredentialStorage{Ciphertext: agent.Ciphertext, KeyID: agent.KeyID}
		changedFields = []string{"agentConfiguration"}
	} else {
		current, err := s.decodeAgentCredentialForCompany(agent)
		if err != nil {
			return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
		}
		current.WXSecret = strings.TrimSpace(*input.WXSecret)
		storage, err = s.encodeAgentCredential(binding.CorpID, agent.WXAgentID, current)
		if err != nil {
			return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
		}
	}
	resultVersion, err := updateCompanyBindingVersionTx(ctx, tx, binding, principal.UserID, input.ExpectedVersion,
		func() error {
			updated, execErr := tx.ExecContext(ctx, `
				UPDATE mc_work_agent
				SET wx_secret = '', wecom_credentials_ciphertext = ?, wecom_credentials_key_id = ?, updated_at = NOW()
				WHERE id = ? AND corp_id = ? AND deleted_at IS NULL`, storage.Ciphertext, storage.KeyID, agent.ID, binding.CorpID)
			if execErr != nil {
				return execErr
			}
			if err := requireCompanyAgentRowsOrMatched(ctx, tx, updated, binding, agent.ID, storage); err != nil {
				return err
			}
			if err := incrementCompanyCredentialGenerationsTx(ctx, tx, binding, companyCredentialRotationGroups{Agent: true}); err != nil {
				return err
			}
			return nil
		}, "dashboard.company.agent_credentials.rotate", "company_agent", strconv.Itoa(agent.ID), changedFields, input.RequestID)
	if err != nil {
		return companyprofile.Profile{}, err
	}
	if resultVersion == 0 {
		return companyprofile.Profile{}, errors.New("company agent credential version was not advanced")
	}
	if err := tx.Commit(); err != nil {
		return companyprofile.Profile{}, err
	}
	return s.GetProfile(ctx, principal)
}

func (s *MySQLStore) ListAudits(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, filter companyprofile.AuditFilter) (companyprofile.AuditPage, error) {
	if s == nil || s.db == nil {
		return companyprofile.AuditPage{}, companyprofile.ErrStoreUnavailable
	}
	if err := s.checkCompanyActor(ctx, s.db, principal, false); err != nil {
		return companyprofile.AuditPage{}, err
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PerPage <= 0 || filter.PerPage > 100 {
		filter.PerPage = 20
	}
	targetID := strconv.Itoa(principal.CorpID)
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mochat_go_dashboard_permission_audits WHERE tenant_id = ? AND target_type = 'company' AND target_id = ?`, principal.TenantID, targetID).Scan(&total); err != nil {
		return companyprofile.AuditPage{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, action, target_type, target_id, COALESCE(before_json,''), COALESCE(after_json,''),
		       expected_version, result_version, request_id, created_at
		FROM mochat_go_dashboard_permission_audits
		WHERE tenant_id = ? AND target_type = 'company' AND target_id = ?
		ORDER BY id DESC LIMIT ? OFFSET ?`, principal.TenantID, targetID, filter.PerPage, (filter.Page-1)*filter.PerPage)
	if err != nil {
		return companyprofile.AuditPage{}, err
	}
	defer rows.Close()
	page := companyprofile.AuditPage{Items: []companyprofile.Audit{}, Page: filter.Page, PerPage: filter.PerPage, Total: total}
	for rows.Next() {
		var item companyprofile.Audit
		var beforeJSON, afterJSON []byte
		var expected, result sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Action, &item.TargetType, &item.TargetID, &beforeJSON, &afterJSON, &expected, &result, &item.RequestID, &item.CreatedAt); err != nil {
			return companyprofile.AuditPage{}, err
		}
		item.ChangedFields = changedFieldsFromAudit(beforeJSON, afterJSON)
		if expected.Valid {
			value := uint64(expected.Int64)
			item.ExpectedVersion = &value
		}
		if result.Valid {
			value := uint64(result.Int64)
			item.ResultVersion = &value
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return companyprofile.AuditPage{}, err
	}
	return page, nil
}

type companyActorFacts struct {
	UserStatus     int
	IsSuperAdmin   int
	IdentityStatus int
	TenantID       int
	AuthVersion    uint64
	Activated      bool
}

func companyActorFactsAllowed(facts companyActorFacts, principal dashboardprincipal.DashboardPrincipal, hasCompanyPermission bool) bool {
	expectedSuperAdmin := 0
	if principal.IsSuperAdmin {
		expectedSuperAdmin = 1
	}
	return facts.TenantID == principal.TenantID &&
		facts.UserStatus == 1 && facts.IsSuperAdmin == expectedSuperAdmin && facts.IdentityStatus == 1 &&
		facts.AuthVersion == principal.AuthVersion && facts.Activated &&
		(principal.IsSuperAdmin || hasCompanyPermission)
}

// businessActorFactsAllowed is deliberately separate from companyActorFactsAllowed:
// company settings require a superadmin, while a page-scoped business mutation
// only requires an active authenticated dashboard user and a live corp binding.
func businessActorFactsAllowed(facts companyActorFacts, principal dashboardprincipal.DashboardPrincipal, bindingActive bool) bool {
	return facts.TenantID == principal.TenantID &&
		facts.UserStatus == 1 && facts.IdentityStatus == 1 && facts.AuthVersion == principal.AuthVersion &&
		facts.Activated && bindingActive
}

func companyActorQuery(forUpdate bool) string {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	return `
		SELECT u.status, COALESCE(u.isSuperAdmin, 0), d.status, u.tenant_id,
		       d.auth_version, d.activated_at IS NOT NULL
		FROM mc_user u
		JOIN mochat_go_dashboard_identities d ON d.user_id = u.id
		WHERE u.id = ? AND u.tenant_id = ? AND u.deleted_at IS NULL
		LIMIT 1` + suffix
}

func (s *MySQLStore) checkCompanyActor(ctx context.Context, queryer companyProfileQueryer, principal dashboardprincipal.DashboardPrincipal, forUpdate bool) error {
	if principal.UserID <= 0 || principal.TenantID <= 0 || principal.CorpID <= 0 {
		return companyprofile.ErrPermissionDenied
	}
	var facts companyActorFacts
	err := queryer.QueryRowContext(ctx, companyActorQuery(forUpdate), principal.UserID, principal.TenantID).Scan(
		&facts.UserStatus, &facts.IsSuperAdmin, &facts.IdentityStatus, &facts.TenantID, &facts.AuthVersion, &facts.Activated)
	if errors.Is(err, sql.ErrNoRows) {
		return companyprofile.ErrPermissionDenied
	}
	if err != nil {
		return companyprofile.ErrStoreUnavailable
	}
	hasCompanyPermission := dashboardprincipal.HasPermissionCode(ctx, principal, "dashboard.company_setting.website")
	if !companyActorFactsAllowed(facts, principal, hasCompanyPermission) {
		return companyprofile.ErrPermissionDenied
	}
	return nil
}

func (s *MySQLStore) loadCompanyBinding(ctx context.Context, queryer companyProfileQueryer, principal dashboardprincipal.DashboardPrincipal, forUpdate bool) (companyBindingRecord, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	row := queryer.QueryRowContext(ctx, `
		SELECT b.tenant_id, b.corp_id, b.status, b.version,
		       b.employee_credential_generation, b.contact_credential_generation,
		       b.agent_credential_generation, b.callback_credential_generation,
		       COALESCE(b.verified_wx_corpid,''), COALESCE(b.verified_corp_name,''), b.verified_at,
		       COALESCE(c.name,''), COALESCE(c.wx_corpid,''),
		       COALESCE(c.wecom_credentials_ciphertext,''), COALESCE(c.wecom_credentials_key_id,''), c.updated_at
		FROM mochat_go_tenant_corp_bindings b
		JOIN mc_corp c ON c.id = b.corp_id AND c.tenant_id = b.tenant_id AND c.deleted_at IS NULL
		WHERE b.tenant_id = ? AND b.corp_id = ?`+suffix, principal.TenantID, principal.CorpID)
	var item companyBindingRecord
	if err := row.Scan(&item.TenantID, &item.CorpID, &item.Status, &item.Version,
		&item.EmployeeGeneration, &item.ContactGeneration, &item.AgentGeneration, &item.CallbackGeneration,
		&item.VerifiedWXCorpID, &item.VerifiedCorpName,
		&item.VerifiedAt, &item.DisplayName, &item.LegacyWXCorpID, &item.Ciphertext, &item.KeyID, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return companyBindingRecord{}, companyprofile.ErrNotFound
		}
		return companyBindingRecord{}, companyprofile.ErrStoreUnavailable
	}
	if item.TenantID != principal.TenantID || item.CorpID != principal.CorpID || item.Version == 0 {
		return companyBindingRecord{}, companyprofile.ErrTenantAccessDenied
	}
	if item.Status == 3 {
		return companyBindingRecord{}, companyprofile.ErrTenantAccessDenied
	}
	if item.Status != 1 && item.Status != 2 {
		return companyBindingRecord{}, companyprofile.ErrTenantAccessDenied
	}
	item.ActorUserID = principal.UserID
	return item, nil
}

func (s *MySQLStore) companyProfileFromBinding(ctx context.Context, queryer companyProfileQueryer, binding companyBindingRecord) (companyprofile.Profile, error) {
	status := ""
	switch binding.Status {
	case 1:
		status = string(dashboardprincipal.CorpBindingStatusPending)
	case 2:
		status = string(dashboardprincipal.CorpBindingStatusActive)
	case 3:
		status = string(dashboardprincipal.CorpBindingStatusSuspended)
	default:
		return companyprofile.Profile{}, companyprofile.ErrTenantAccessDenied
	}
	corpConfigured := false
	employeeConfigured := false
	contactConfigured := false
	callbackTokenConfigured := false
	callbackAESConfigured := false
	archiveConfigured := false
	if strings.TrimSpace(binding.Ciphertext) != "" && s != nil {
		credential, decryptErr := s.decodeEncryptedCorpCredential(corpCredentialRecord{
			ID: binding.CorpID, TenantID: binding.TenantID, WXCorpID: binding.LegacyWXCorpID,
			Ciphertext: binding.Ciphertext, KeyID: binding.KeyID,
		})
		if decryptErr != nil {
			return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
		}
		employeeConfigured, contactConfigured, callbackTokenConfigured, callbackAESConfigured = deriveCorpCredentialFacts(credential)
		corpConfigured = employeeConfigured || contactConfigured || callbackTokenConfigured || callbackAESConfigured || strings.TrimSpace(credential.ChatSecret) != ""
		archiveConfigured = strings.TrimSpace(credential.ChatSecret) != "" &&
			strings.TrimSpace(credential.ArchiveRSAPublicKey) != "" && strings.TrimSpace(credential.ArchiveRSAPrivateKey) != ""
	} else if strings.TrimSpace(binding.Ciphertext) != "" || strings.TrimSpace(binding.KeyID) != "" {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	var agentRecord companyAgentCredentialRecord
	agentIDConfigured := false
	agentSecretConfigured := false
	var agentKey sql.NullString
	var agentUpdated sql.NullTime
	var applicationAgentID string
	err := queryer.QueryRowContext(ctx, `
		SELECT a.id, a.corp_id, c.tenant_id, COALESCE(a.wx_agent_id,''),
		       COALESCE(a.wecom_credentials_ciphertext,''), COALESCE(a.wecom_credentials_key_id,''),
		       a.updated_at
		FROM mc_work_agent a
		JOIN mc_corp c ON c.id = a.corp_id AND c.tenant_id = ? AND c.deleted_at IS NULL
		WHERE a.corp_id = ? AND `+authoritativeApplicationAgentSelectionSQL()+` LIMIT 1`, binding.TenantID, binding.CorpID).Scan(
		&agentRecord.ID, &agentRecord.CorpID, &agentRecord.TenantID, &agentRecord.WXAgentID,
		&agentRecord.Ciphertext, &agentRecord.KeyID, &agentUpdated)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	} else if err != nil {
		return companyprofile.Profile{}, companyprofile.ErrStoreUnavailable
	}
	if err == nil && agentRecord.ID > 0 {
		applicationAgentID = strings.TrimSpace(agentRecord.WXAgentID)
		agentIDConfigured, agentSecretConfigured = deriveAgentCredentialFacts(agentRecord.WXAgentID, agentRecord.Ciphertext, agentRecord.KeyID, "")
		agentKey.String = strings.TrimSpace(agentRecord.KeyID)
		agentKey.Valid = agentKey.String != ""
		if agentIDConfigured && agentRecord.Ciphertext != "" && agentRecord.KeyID != "" && s != nil {
			decoded, decodeErr := s.decodeAgentCredentialForCompany(agentRecord)
			if decodeErr == nil {
				_, agentSecretConfigured = deriveAgentCredentialFacts(agentRecord.WXAgentID, agentRecord.Ciphertext, agentRecord.KeyID, decoded.WXSecret)
			}
		}
	}
	profile := companyprofile.Profile{
		TenantID: binding.TenantID, CorpID: binding.CorpID, DisplayName: binding.DisplayName,
		AuthoritativeCorpName: binding.VerifiedCorpName, BindingStatus: status, BindingVersion: binding.Version,
		CredentialGenerations: companyprofile.CredentialGenerationSet{Employee: binding.EmployeeGeneration, Contact: binding.ContactGeneration, Agent: binding.AgentGeneration, Callback: binding.CallbackGeneration},
		ApplicationAgentID:    strings.TrimSpace(applicationAgentID),
		Credentials: companyprofile.CredentialStatuses{
			WeCom:    companyprofile.CredentialStatus{Configured: corpConfigured, EmployeeConfigured: employeeConfigured, ContactConfigured: contactConfigured, CallbackTokenConfigured: callbackTokenConfigured, CallbackAESConfigured: callbackAESConfigured, KeyID: strings.TrimSpace(binding.KeyID), UpdatedAt: companyNullableTime(binding.UpdatedAt)},
			Agent:    companyprofile.CredentialStatus{Configured: agentIDConfigured && agentSecretConfigured, AgentIDConfigured: agentIDConfigured, AgentSecretConfigured: agentSecretConfigured, KeyID: strings.TrimSpace(agentKey.String), UpdatedAt: companyNullableTime(agentUpdated)},
			Archive:  companyprofile.CredentialStatus{Configured: archiveConfigured, KeyID: strings.TrimSpace(binding.KeyID), UpdatedAt: companyNullableTime(binding.UpdatedAt)},
			Callback: companyprofile.CredentialStatus{Configured: callbackTokenConfigured && callbackAESConfigured, CallbackTokenConfigured: callbackTokenConfigured, CallbackAESConfigured: callbackAESConfigured, KeyID: strings.TrimSpace(binding.KeyID), UpdatedAt: companyNullableTime(binding.UpdatedAt)},
		},
	}
	if strings.TrimSpace(binding.VerifiedWXCorpID) != "" {
		profile.WXCorpID = strings.TrimSpace(binding.VerifiedWXCorpID)
	}
	if binding.VerifiedAt.Valid {
		value := binding.VerifiedAt.Time
		profile.VerifiedAt = &value
	}
	if binding.UpdatedAt.Valid {
		profile.UpdatedAt = binding.UpdatedAt.Time
	} else {
		profile.UpdatedAt = time.Unix(0, 0).UTC()
	}
	return profile, nil
}

// authoritativeApplicationAgentSelectionSQL is shared by provider status and
// outbound agent-message credential lookup. The selected agent must be active;
// report-enter priority and the remaining order are part of the contract.
func authoritativeApplicationAgentSelectionSQL() string {
	return "a.close = 0 AND a.deleted_at IS NULL ORDER BY (a.is_reportenter = 1) DESC, a.updated_at DESC, a.id ASC"
}

func deriveCorpCredentialFacts(credential wecomcredentials.CorpCredential) (employee, contact, callbackToken, callbackAES bool) {
	return strings.TrimSpace(credential.EmployeeSecret) != "",
		strings.TrimSpace(credential.ContactSecret) != "",
		strings.TrimSpace(credential.CallbackToken) != "",
		strings.TrimSpace(credential.EncodingAESKey) != ""
}

// deriveAgentCredentialFacts intentionally requires all storage envelope
// fields plus the decrypted secret. KeyID or ciphertext alone is never agent
// readiness evidence.
func deriveAgentCredentialFacts(wxAgentID, ciphertext, keyID, decryptedSecret string) (idConfigured, secretConfigured bool) {
	idConfigured = strings.TrimSpace(wxAgentID) != ""
	secretConfigured = idConfigured && strings.TrimSpace(ciphertext) != "" && strings.TrimSpace(keyID) != "" && strings.TrimSpace(decryptedSecret) != ""
	return idConfigured, secretConfigured
}

func companyNullableTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}

func clearCompanyBindingVerificationTx(ctx context.Context, tx *sql.Tx, binding companyBindingRecord) error {
	updated, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_tenant_corp_bindings
		SET verified_wx_corpid = '', verified_corp_name = '', verified_at = NULL, updated_at = NOW()
		WHERE tenant_id = ? AND corp_id = ? AND version = ? AND status IN (1, 2)`,
		binding.TenantID, binding.CorpID, binding.Version)
	if err != nil {
		return err
	}
	return requireCompanyRows(updated, 1)
}

func incrementCompanyCredentialGenerationsTx(ctx context.Context, tx *sql.Tx, binding companyBindingRecord, groups companyCredentialRotationGroups) error {
	if !groups.Employee && !groups.Contact && !groups.Agent && !groups.Callback {
		return nil
	}
	assignments := make([]string, 0, 4)
	if groups.Employee {
		assignments = append(assignments, "employee_credential_generation = employee_credential_generation + 1")
	}
	if groups.Contact {
		assignments = append(assignments, "contact_credential_generation = contact_credential_generation + 1")
	}
	if groups.Agent {
		assignments = append(assignments, "agent_credential_generation = agent_credential_generation + 1")
	}
	if groups.Callback {
		assignments = append(assignments, "callback_credential_generation = callback_credential_generation + 1")
	}
	query := `UPDATE mochat_go_tenant_corp_bindings SET ` + strings.Join(assignments, ", ") + `, updated_at = NOW() WHERE tenant_id = ? AND corp_id = ? AND version = ? AND status IN (1,2)`
	updated, err := tx.ExecContext(ctx, query, binding.TenantID, binding.CorpID, binding.Version)
	if err != nil {
		return err
	}
	return requireCompanyRows(updated, 1)
}

func updateCompanyBindingVersionTx(ctx context.Context, tx *sql.Tx, binding companyBindingRecord, actorUserID int, expected uint64, mutation func() error, action, targetType, targetID string, changedFields []string, requestID string) (uint64, error) {
	if binding.Version != expected {
		return 0, companyprofile.ErrVersionConflict
	}
	if err := mutation(); err != nil {
		return 0, err
	}
	resultVersion := expected + 1
	updated, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_tenant_corp_bindings
		SET version = ?, updated_at = NOW()
		WHERE tenant_id = ? AND corp_id = ? AND version = ? AND status IN (1,2)`,
		resultVersion, binding.TenantID, binding.CorpID, expected)
	if err != nil {
		return 0, err
	}
	if err := requireCompanyRows(updated, 1); err != nil {
		return 0, err
	}
	beforeJSON, _ := json.Marshal(map[string]any{"changedFields": []string{}})
	afterJSON, _ := json.Marshal(map[string]any{"changedFields": changedFields})
	auditResult, err := tx.ExecContext(ctx, `
		INSERT INTO mochat_go_dashboard_permission_audits
		(tenant_id, actor_user_id, action, target_type, target_id, before_json, after_json, expected_version, result_version, request_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		binding.TenantID, actorUserID, action, targetType, targetID, string(beforeJSON), string(afterJSON), expected, resultVersion, strings.TrimSpace(requestID))
	if err != nil {
		return 0, err
	}
	if err := requireCompanyRows(auditResult, 1); err != nil {
		return 0, err
	}
	return resultVersion, nil
}

func requireCompanyRows(result sql.Result, expected int64) error {
	if result == nil {
		return errors.New("company profile mutation returned no result")
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != expected {
		return fmt.Errorf("company profile mutation affected %d rows, want %d", rows, expected)
	}
	return nil
}

func requireCompanyAgentRowsOrMatched(ctx context.Context, tx *sql.Tx, result sql.Result, binding companyBindingRecord, agentID int, storage agentCredentialStorage) error {
	if result == nil {
		return errors.New("company profile mutation returned no result")
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 1 {
		return nil
	}
	if rows != 0 {
		return fmt.Errorf("company profile mutation affected %d rows, want 1", rows)
	}
	var matched int
	if scanErr := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM mc_work_agent a
			INNER JOIN mc_corp c ON c.id = a.corp_id AND c.tenant_id = ? AND c.deleted_at IS NULL
			INNER JOIN mochat_go_tenant_corp_bindings b
				ON b.tenant_id = c.tenant_id AND b.corp_id = a.corp_id
			WHERE b.tenant_id = ? AND b.corp_id = ? AND b.version = ? AND b.status = ? AND b.status IN (1, 2)
			  AND a.id = ? AND a.corp_id = ? AND a.deleted_at IS NULL
			  AND COALESCE(CAST(a.wecom_credentials_ciphertext AS CHAR), '') = ?
			  AND COALESCE(a.wecom_credentials_key_id, '') = ?`,
		binding.TenantID, binding.TenantID, binding.CorpID, binding.Version, binding.Status, agentID, binding.CorpID, storage.Ciphertext, storage.KeyID).Scan(&matched); scanErr == nil && matched == 1 {
		return nil
	}
	return fmt.Errorf("company profile mutation affected 0 rows, want 1")
}

type companyAgentCredentialRecord struct {
	ID         int
	CorpID     int
	TenantID   int
	WXAgentID  string
	Ciphertext string
	KeyID      string
}

func loadCompanyApplicationAgent(ctx context.Context, tx *sql.Tx, binding companyBindingRecord, wxAgentID string) (companyAgentCredentialRecord, bool, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT a.id, a.corp_id, c.tenant_id, COALESCE(a.wx_agent_id,''),
		       COALESCE(a.wecom_credentials_ciphertext,''), COALESCE(a.wecom_credentials_key_id,'')
		FROM mc_work_agent a
		JOIN mc_corp c ON c.id=a.corp_id AND c.tenant_id=? AND c.deleted_at IS NULL
		WHERE a.corp_id=? AND `+authoritativeApplicationAgentSelectionSQL()+`
		FOR UPDATE`, binding.TenantID, binding.CorpID)
	if err != nil {
		return companyAgentCredentialRecord{}, false, err
	}
	defer rows.Close()
	items := make([]companyAgentCredentialRecord, 0, 2)
	for rows.Next() {
		var item companyAgentCredentialRecord
		if err := rows.Scan(&item.ID, &item.CorpID, &item.TenantID, &item.WXAgentID, &item.Ciphertext, &item.KeyID); err != nil {
			return companyAgentCredentialRecord{}, false, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return companyAgentCredentialRecord{}, false, err
	}
	if len(items) == 0 {
		return companyAgentCredentialRecord{}, false, nil
	}
	requestedAgentID := strings.TrimSpace(wxAgentID)
	if requestedAgentID != "" {
		for index := 1; index < len(items); index++ {
			if strings.TrimSpace(items[index].WXAgentID) == requestedAgentID {
				// An input that names another active agent would create duplicate
				// application identities if copied onto the canonical row. Fail
				// closed before any credential or binding mutation.
				return companyAgentCredentialRecord{}, false, companyprofile.ErrInvalidRequest
			}
		}
	}
	return items[0], true, nil
}

func loadCompanyAgentCredential(ctx context.Context, queryer companyProfileQueryer, binding companyBindingRecord, input companyprofile.AgentCredentialsInput) (companyAgentCredentialRecord, bool, error) {
	where := "a.corp_id = ? AND a.deleted_at IS NULL"
	args := []any{binding.CorpID}
	if input.AgentID > 0 {
		where += " AND a.id = ?"
		args = append(args, input.AgentID)
	} else {
		where += " AND a.wx_agent_id = ?"
		args = append(args, strings.TrimSpace(input.WXAgentID))
	}
	row := queryer.QueryRowContext(ctx, `
		SELECT a.id, a.corp_id, c.tenant_id, COALESCE(a.wx_agent_id,''),
		       COALESCE(a.wecom_credentials_ciphertext,''), COALESCE(a.wecom_credentials_key_id,'')
		FROM mc_work_agent a
		JOIN mc_corp c ON c.id = a.corp_id AND c.tenant_id = ? AND c.deleted_at IS NULL
		WHERE `+where+` LIMIT 1 FOR UPDATE`, append([]any{binding.TenantID}, args...)...)
	var item companyAgentCredentialRecord
	if err := row.Scan(&item.ID, &item.CorpID, &item.TenantID, &item.WXAgentID, &item.Ciphertext, &item.KeyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return companyAgentCredentialRecord{}, false, nil
		}
		return companyAgentCredentialRecord{}, false, err
	}
	if item.TenantID != binding.TenantID || item.CorpID != binding.CorpID {
		return companyAgentCredentialRecord{}, false, companyprofile.ErrTenantAccessDenied
	}
	return item, true, nil
}

func (s *MySQLStore) decodeAgentCredentialForCompany(item companyAgentCredentialRecord) (wecomcredentials.AgentCredential, error) {
	if s == nil || s.weComCredentialCipher == nil || strings.TrimSpace(item.Ciphertext) == "" || strings.TrimSpace(item.KeyID) == "" {
		return wecomcredentials.AgentCredential{}, errors.New("company agent credential is not encrypted")
	}
	return s.weComCredentialCipher.DecryptAgent(item.CorpID, item.WXAgentID, item.KeyID, item.Ciphertext)
}

func changedFieldsFromAudit(beforeJSON, afterJSON []byte) []string {
	var payload struct {
		ChangedFields []string `json:"changedFields"`
	}
	if err := json.Unmarshal(afterJSON, &payload); err != nil {
		_ = json.Unmarshal(beforeJSON, &payload)
	}
	return append([]string(nil), payload.ChangedFields...)
}
