package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/wechatopencredentials"
)

type weChatComponentTicketCredentialRecord struct {
	ComponentAppID string
	Ticket         string
	Ciphertext     string
	KeyID          string
}

type officialAccountCredentialRecord struct {
	ID                     int
	TenantID               int
	CorpID                 int
	ComponentAppID         string
	AuthorizerAppID        string
	AuthorizationCode      string
	PreAuthCode            string
	ComponentAESKey        string
	ComponentToken         string
	ComponentSecret        string
	Ciphertext             string
	KeyID                  string
	AuthorizerRefreshToken string
}

type weChatComponentTicketCredentialStorage struct {
	Ticket     string
	Ciphertext string
	KeyID      string
}

type officialAccountCredentialStorage struct {
	AuthorizationCode string
	PreAuthCode       string
	ComponentAESKey   string
	ComponentToken    string
	ComponentSecret   string
	Ciphertext        string
	KeyID             string
}

func (s *MySQLStore) loadWeChatComponentTicketCredential(ctx context.Context, queryer queryRower, componentAppID string, forUpdate bool) (weChatComponentTicketCredentialRecord, bool, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	row := queryer.QueryRowContext(ctx, `
		SELECT component_appid, COALESCE(component_verify_ticket, ''),
		       COALESCE(component_verify_ticket_ciphertext, ''), COALESCE(credential_key_id, '')
		FROM mochat_go_wechat_component_tickets
		WHERE component_appid = ?`+suffix, strings.TrimSpace(componentAppID))
	return scanWeChatComponentTicketCredential(row)
}

func scanWeChatComponentTicketCredential(scanner rowScanner) (weChatComponentTicketCredentialRecord, bool, error) {
	var item weChatComponentTicketCredentialRecord
	if err := scanner.Scan(&item.ComponentAppID, &item.Ticket, &item.Ciphertext, &item.KeyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return weChatComponentTicketCredentialRecord{}, false, nil
		}
		return weChatComponentTicketCredentialRecord{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) decodeWeChatComponentTicketCredential(item weChatComponentTicketCredentialRecord) (wechatopencredentials.TicketCredential, error) {
	if strings.TrimSpace(item.Ciphertext) != "" {
		if s.weChatOpenCredentialCipher == nil {
			return wechatopencredentials.TicketCredential{}, errors.New("WeChat Open credential encryption manager is not configured")
		}
		return s.weChatOpenCredentialCipher.DecryptTicket(item.ComponentAppID, item.KeyID, item.Ciphertext)
	}
	return wechatopencredentials.TicketCredential{ComponentVerifyTicket: strings.TrimSpace(item.Ticket)}, nil
}

func (s *MySQLStore) encodeWeChatComponentTicketCredential(componentAppID string, credential wechatopencredentials.TicketCredential) (weChatComponentTicketCredentialStorage, error) {
	if s.weChatOpenCredentialCipher != nil && s.weChatOpenCredentialCipher.ConfigStatus().EncryptionConfigured {
		ciphertext, keyID, err := s.weChatOpenCredentialCipher.EncryptTicket(componentAppID, credential)
		if err != nil {
			return weChatComponentTicketCredentialStorage{}, err
		}
		return weChatComponentTicketCredentialStorage{Ciphertext: ciphertext, KeyID: keyID}, nil
	}
	return weChatComponentTicketCredentialStorage{Ticket: strings.TrimSpace(credential.ComponentVerifyTicket)}, nil
}

func (s *MySQLStore) loadOfficialAccountCredentialByID(ctx context.Context, queryer queryRower, id int, forUpdate bool) (officialAccountCredentialRecord, bool, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	row := queryer.QueryRowContext(ctx, officialAccountCredentialSelect(`WHERE id = ? AND deleted_at IS NULL`)+suffix, id)
	return scanOfficialAccountCredentialRecord(row)
}

func officialAccountCredentialSelect(suffix string) string {
	return `
		SELECT id, COALESCE(tenant_id, 0), COALESCE(corp_id, 0), COALESCE(appid, ''), COALESCE(authorizer_appid, ''),
		       COALESCE(authorization_code, ''), COALESCE(pre_auth_code, ''), COALESCE(encoding_aes_key, ''),
		       COALESCE(token, ''), COALESCE(secret, ''), COALESCE(wechat_credentials_ciphertext, ''),
		       COALESCE(wechat_credentials_key_id, '')
		FROM mc_official_account
	` + suffix
}

func scanOfficialAccountCredentialRecord(scanner rowScanner) (officialAccountCredentialRecord, bool, error) {
	var item officialAccountCredentialRecord
	if err := scanner.Scan(&item.ID, &item.TenantID, &item.CorpID, &item.ComponentAppID, &item.AuthorizerAppID,
		&item.AuthorizationCode, &item.PreAuthCode, &item.ComponentAESKey, &item.ComponentToken, &item.ComponentSecret,
		&item.Ciphertext, &item.KeyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return officialAccountCredentialRecord{}, false, nil
		}
		return officialAccountCredentialRecord{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) decodeOfficialAccountCredential(item officialAccountCredentialRecord) (wechatopencredentials.OfficialAccountCredential, error) {
	if strings.TrimSpace(item.Ciphertext) != "" {
		if s.weChatOpenCredentialCipher == nil {
			return wechatopencredentials.OfficialAccountCredential{}, errors.New("WeChat Open credential encryption manager is not configured")
		}
		return s.weChatOpenCredentialCipher.DecryptOfficialAccount(item.TenantID, item.AuthorizerAppID, item.KeyID, item.Ciphertext)
	}
	return wechatopencredentials.OfficialAccountCredential{
		ComponentSecret: item.ComponentSecret, ComponentToken: item.ComponentToken, ComponentAESKey: item.ComponentAESKey,
		AuthorizationCode: item.AuthorizationCode, PreAuthCode: item.PreAuthCode,
	}, nil
}

func (s *MySQLStore) encodeOfficialAccountCredential(tenantID int, authorizerAppID string, credential wechatopencredentials.OfficialAccountCredential) (officialAccountCredentialStorage, error) {
	if s.weChatOpenCredentialCipher != nil && s.weChatOpenCredentialCipher.ConfigStatus().EncryptionConfigured {
		ciphertext, keyID, err := s.weChatOpenCredentialCipher.EncryptOfficialAccount(tenantID, authorizerAppID, credential)
		if err != nil {
			return officialAccountCredentialStorage{}, err
		}
		return officialAccountCredentialStorage{Ciphertext: ciphertext, KeyID: keyID}, nil
	}
	return officialAccountCredentialStorage{
		AuthorizationCode: strings.TrimSpace(credential.AuthorizationCode),
		PreAuthCode:       strings.TrimSpace(credential.PreAuthCode),
		ComponentAESKey:   strings.TrimSpace(credential.ComponentAESKey),
		ComponentToken:    strings.TrimSpace(credential.ComponentToken),
		ComponentSecret:   strings.TrimSpace(credential.ComponentSecret),
	}, nil
}

func officialAccountCredentialFromAuthorization(values dashboard.OfficialAccountAuthorization) wechatopencredentials.OfficialAccountCredential {
	return wechatopencredentials.OfficialAccountCredential{
		ComponentSecret: values.ComponentSecret, ComponentToken: values.ComponentToken, ComponentAESKey: values.ComponentAESKey,
		AuthorizationCode: values.AuthorizationCode, PreAuthCode: values.PreAuthCode,
		AuthorizerRefreshToken: values.AuthorizerRefreshToken,
	}
}

func mergeOfficialAccountCredential(current wechatopencredentials.OfficialAccountCredential, update wechatopencredentials.OfficialAccountCredential) wechatopencredentials.OfficialAccountCredential {
	if strings.TrimSpace(update.ComponentSecret) != "" {
		current.ComponentSecret = update.ComponentSecret
	}
	if strings.TrimSpace(update.ComponentToken) != "" {
		current.ComponentToken = update.ComponentToken
	}
	if strings.TrimSpace(update.ComponentAESKey) != "" {
		current.ComponentAESKey = update.ComponentAESKey
	}
	if strings.TrimSpace(update.AuthorizationCode) != "" {
		current.AuthorizationCode = update.AuthorizationCode
	}
	if strings.TrimSpace(update.PreAuthCode) != "" {
		current.PreAuthCode = update.PreAuthCode
	}
	if strings.TrimSpace(update.AuthorizerRefreshToken) != "" {
		current.AuthorizerRefreshToken = update.AuthorizerRefreshToken
	}
	return current
}

func (s *MySQLStore) officialAccountOAuthInfoFromRecord(item officialAccountCredentialRecord) (dashboard.OfficialAccountOAuthInfo, error) {
	credential, err := s.decodeOfficialAccountCredential(item)
	if err != nil {
		return dashboard.OfficialAccountOAuthInfo{}, err
	}
	return dashboard.OfficialAccountOAuthInfo{
		ID: item.ID, CorpID: item.CorpID, ComponentAppID: item.ComponentAppID, AuthorizerAppID: item.AuthorizerAppID,
		ComponentSecret: credential.ComponentSecret, ComponentToken: credential.ComponentToken,
		ComponentAESKey: credential.ComponentAESKey, AuthorizationCode: credential.AuthorizationCode,
		PreAuthCode: credential.PreAuthCode, AuthorizerRefreshToken: credential.AuthorizerRefreshToken,
	}, nil
}

func (s *MySQLStore) scanOfficialAccountOAuthInfo(scanner rowScanner) (dashboard.OfficialAccountOAuthInfo, bool, error) {
	item, found, err := scanOfficialAccountCredentialRecord(scanner)
	if err != nil || !found {
		return dashboard.OfficialAccountOAuthInfo{}, found, err
	}
	info, err := s.officialAccountOAuthInfoFromRecord(item)
	if err != nil {
		return dashboard.OfficialAccountOAuthInfo{}, false, err
	}
	return info, true, nil
}

func (s *MySQLStore) SaaSWeChatOpenCredentialProtection(ctx context.Context) (dashboard.SaaSWeChatOpenCredentialProtectionStatus, error) {
	status := dashboard.SaaSWeChatOpenCredentialProtectionStatus{ActiveKeyID: "primary", UnavailableKeyIDs: []string{}}
	if s.weChatOpenCredentialCipher != nil {
		config := s.weChatOpenCredentialCipher.ConfigStatus()
		status.EncryptionConfigured = config.EncryptionConfigured
		status.RequireEncryption = config.RequireEncryption
		status.DedicatedConfigured = config.DedicatedConfigured
		status.ActiveKeyID = config.ActiveKeyID
		status.KeyCount = config.KeyCount
	}
	unavailable := map[string]struct{}{}
	ticketRows, err := s.db.QueryContext(ctx, `
		SELECT component_appid, COALESCE(component_verify_ticket, ''),
		       COALESCE(component_verify_ticket_ciphertext, ''), COALESCE(credential_key_id, '')
		FROM mochat_go_wechat_component_tickets
		ORDER BY component_appid ASC
	`)
	if err != nil {
		return dashboard.SaaSWeChatOpenCredentialProtectionStatus{}, err
	}
	for ticketRows.Next() {
		item, _, scanErr := scanWeChatComponentTicketCredential(ticketRows)
		if scanErr != nil {
			ticketRows.Close()
			return dashboard.SaaSWeChatOpenCredentialProtectionStatus{}, scanErr
		}
		if !weChatComponentTicketCredentialConfigured(item) {
			continue
		}
		status.ConfiguredCredentialCount++
		status.ComponentTicketCount++
		updateWeChatOpenProtectionStatus(&status, unavailable, item.Ciphertext, item.KeyID, strings.TrimSpace(item.Ticket) != "", func() error {
			_, decryptErr := s.decodeWeChatComponentTicketCredential(item)
			return decryptErr
		})
	}
	if err := ticketRows.Close(); err != nil {
		return dashboard.SaaSWeChatOpenCredentialProtectionStatus{}, err
	}
	if err := ticketRows.Err(); err != nil {
		return dashboard.SaaSWeChatOpenCredentialProtectionStatus{}, err
	}

	accountRows, err := s.db.QueryContext(ctx, officialAccountCredentialSelect(`WHERE deleted_at IS NULL ORDER BY id ASC`))
	if err != nil {
		return dashboard.SaaSWeChatOpenCredentialProtectionStatus{}, err
	}
	for accountRows.Next() {
		item, _, scanErr := scanOfficialAccountCredentialRecord(accountRows)
		if scanErr != nil {
			accountRows.Close()
			return dashboard.SaaSWeChatOpenCredentialProtectionStatus{}, scanErr
		}
		if !officialAccountCredentialConfigured(item) {
			continue
		}
		status.ConfiguredCredentialCount++
		status.OfficialAccountCount++
		updateWeChatOpenProtectionStatus(&status, unavailable, item.Ciphertext, item.KeyID, officialAccountLegacyPlaintextPresent(item), func() error {
			_, decryptErr := s.decodeOfficialAccountCredential(item)
			return decryptErr
		})
	}
	if err := accountRows.Close(); err != nil {
		return dashboard.SaaSWeChatOpenCredentialProtectionStatus{}, err
	}
	if err := accountRows.Err(); err != nil {
		return dashboard.SaaSWeChatOpenCredentialProtectionStatus{}, err
	}

	for keyID := range unavailable {
		status.UnavailableKeyIDs = append(status.UnavailableKeyIDs, keyID)
	}
	sort.Strings(status.UnavailableKeyIDs)
	status.UnavailableKeyCount = len(status.UnavailableKeyIDs)
	status.Healthy = status.UnavailableKeyCount == 0 && status.DecryptFailureCount == 0 && status.RotationRequiredCount == 0
	if status.ConfiguredCredentialCount > 0 && !status.EncryptionConfigured {
		status.Healthy = false
	}
	return status, nil
}

func updateWeChatOpenProtectionStatus(status *dashboard.SaaSWeChatOpenCredentialProtectionStatus, unavailable map[string]struct{}, ciphertext string, keyID string, plaintextPresent bool, decrypt func() error) {
	ciphertext = strings.TrimSpace(ciphertext)
	keyID = strings.TrimSpace(keyID)
	if ciphertext == "" {
		status.LegacyPlaintextCount++
		status.RotationRequiredCount++
		return
	}
	status.EncryptedCredentialCount++
	if keyID == status.ActiveKeyID && !plaintextPresent {
		status.ActiveKeyCredentialCount++
	} else {
		status.RotationRequiredCount++
	}
	if keyID == "" || status.KeyCount == 0 {
		if keyID == "" {
			keyID = "(missing)"
		}
		unavailable[keyID] = struct{}{}
		return
	}
	if decrypt != nil {
		if err := decrypt(); err != nil {
			if strings.Contains(err.Error(), "is unavailable") {
				unavailable[keyID] = struct{}{}
			} else {
				status.DecryptFailureCount++
			}
		}
	}
}

func (s *MySQLStore) CheckSaaSWeChatOpenCredentialProtection(ctx context.Context) error {
	status, err := s.SaaSWeChatOpenCredentialProtection(ctx)
	if err != nil {
		return err
	}
	if status.Healthy {
		return nil
	}
	return fmt.Errorf("WeChat Open credential protection unhealthy: configured=%d encrypted=%d legacy=%d rotation_required=%d unavailable_keys=%d decrypt_failures=%d",
		status.ConfiguredCredentialCount, status.EncryptedCredentialCount, status.LegacyPlaintextCount,
		status.RotationRequiredCount, status.UnavailableKeyCount, status.DecryptFailureCount)
}

func (s *MySQLStore) RotateSaaSWeChatOpenCredentials(ctx context.Context, tenantID int, limit int) (dashboard.SaaSWeChatOpenCredentialRotationResult, error) {
	result := dashboard.SaaSWeChatOpenCredentialRotationResult{TenantID: tenantID, Limit: limit}
	if tenantID < 0 {
		return result, errors.New("tenant id must be non-negative")
	}
	if limit <= 0 || limit > 1000 {
		return result, errors.New("limit must be between 1 and 1000")
	}
	if s.weChatOpenCredentialCipher == nil || !s.weChatOpenCredentialCipher.ConfigStatus().EncryptionConfigured {
		return result, errors.New("WeChat Open credential encryption key is not configured")
	}
	result.ActiveKeyID = s.weChatOpenCredentialCipher.ConfigStatus().ActiveKeyID
	type candidate struct{ resource, id string }
	candidates := make([]candidate, 0, limit)
	if tenantID == 0 {
		rows, err := s.db.QueryContext(ctx, `
			SELECT component_appid
			FROM mochat_go_wechat_component_tickets
			WHERE (component_verify_ticket <> '' OR COALESCE(component_verify_ticket_ciphertext, '') <> '')
			  AND (COALESCE(component_verify_ticket_ciphertext, '') = '' OR credential_key_id <> ? OR component_verify_ticket <> '')
			ORDER BY component_appid ASC
			LIMIT ?
		`, result.ActiveKeyID, limit)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var componentAppID string
			if err := rows.Scan(&componentAppID); err != nil {
				rows.Close()
				return result, err
			}
			candidates = append(candidates, candidate{resource: "component_ticket", id: componentAppID})
		}
		if err := rows.Close(); err != nil {
			return result, err
		}
		if err := rows.Err(); err != nil {
			return result, err
		}
	}
	remaining := limit - len(candidates)
	if remaining > 0 {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id
			FROM mc_official_account
			WHERE deleted_at IS NULL
			  AND (? = 0 OR tenant_id = ?)
			  AND (authorization_code <> '' OR pre_auth_code <> '' OR encoding_aes_key <> '' OR token <> '' OR secret <> '' OR COALESCE(wechat_credentials_ciphertext, '') <> '')
			  AND (COALESCE(wechat_credentials_ciphertext, '') = '' OR wechat_credentials_key_id <> ? OR authorization_code <> '' OR pre_auth_code <> '' OR encoding_aes_key <> '' OR token <> '' OR secret <> '')
			ORDER BY id ASC
			LIMIT ?
		`, tenantID, tenantID, result.ActiveKeyID, remaining)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return result, err
			}
			candidates = append(candidates, candidate{resource: "official_account", id: strconv.Itoa(id)})
		}
		if err := rows.Close(); err != nil {
			return result, err
		}
		if err := rows.Err(); err != nil {
			return result, err
		}
	}
	result.ScannedCount = len(candidates)
	for _, item := range candidates {
		var rotated, legacy bool
		var err error
		if item.resource == "component_ticket" {
			rotated, legacy, err = s.rotateWeChatComponentTicketCredential(ctx, item.id)
		} else {
			id, parseErr := strconv.Atoi(item.id)
			if parseErr != nil {
				return result, parseErr
			}
			rotated, legacy, err = s.rotateOfficialAccountCredential(ctx, id, tenantID)
		}
		if err != nil {
			return result, err
		}
		if !rotated {
			continue
		}
		result.RotatedCount++
		if item.resource == "component_ticket" {
			result.ComponentTicketRotatedCount++
		} else {
			result.OfficialAccountRotatedCount++
		}
		if legacy {
			result.LegacyCount++
		} else {
			result.ReencryptedCount++
		}
	}
	return result, nil
}

func (s *MySQLStore) rotateWeChatComponentTicketCredential(ctx context.Context, componentAppID string) (bool, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, false, err
	}
	defer rollbackQuietly(tx)
	item, found, err := s.loadWeChatComponentTicketCredential(ctx, tx, componentAppID, true)
	if err != nil || !found {
		return false, false, err
	}
	legacy := strings.TrimSpace(item.Ciphertext) == ""
	if !weChatComponentTicketCredentialNeedsRotation(item, s.weChatOpenCredentialCipher.ConfigStatus().ActiveKeyID) {
		return false, legacy, nil
	}
	credential, err := s.decodeWeChatComponentTicketCredential(item)
	if err != nil {
		return false, legacy, err
	}
	storage, err := s.encodeWeChatComponentTicketCredential(item.ComponentAppID, credential)
	if err != nil {
		return false, legacy, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mochat_go_wechat_component_tickets
		SET component_verify_ticket = ?, component_verify_ticket_ciphertext = ?, credential_key_id = ?, updated_at = NOW()
		WHERE component_appid = ?
	`, storage.Ticket, storage.Ciphertext, storage.KeyID, item.ComponentAppID); err != nil {
		return false, legacy, err
	}
	if err := tx.Commit(); err != nil {
		return false, legacy, err
	}
	return true, legacy, nil
}

func (s *MySQLStore) rotateOfficialAccountCredential(ctx context.Context, id int, tenantID int) (bool, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, false, err
	}
	defer rollbackQuietly(tx)
	item, found, err := s.loadOfficialAccountCredentialByID(ctx, tx, id, true)
	if err != nil || !found {
		return false, false, err
	}
	if tenantID > 0 && item.TenantID != tenantID {
		return false, false, nil
	}
	legacy := strings.TrimSpace(item.Ciphertext) == ""
	if !officialAccountCredentialNeedsRotation(item, s.weChatOpenCredentialCipher.ConfigStatus().ActiveKeyID) {
		return false, legacy, nil
	}
	credential, err := s.decodeOfficialAccountCredential(item)
	if err != nil {
		return false, legacy, err
	}
	storage, err := s.encodeOfficialAccountCredential(item.TenantID, item.AuthorizerAppID, credential)
	if err != nil {
		return false, legacy, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mc_official_account
		SET authorization_code = ?, pre_auth_code = ?, encoding_aes_key = ?, token = ?, secret = ?,
		    wechat_credentials_ciphertext = ?, wechat_credentials_key_id = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, storage.AuthorizationCode, storage.PreAuthCode, storage.ComponentAESKey, storage.ComponentToken, storage.ComponentSecret,
		storage.Ciphertext, storage.KeyID, item.ID); err != nil {
		return false, legacy, err
	}
	if err := tx.Commit(); err != nil {
		return false, legacy, err
	}
	return true, legacy, nil
}

func weChatComponentTicketCredentialConfigured(item weChatComponentTicketCredentialRecord) bool {
	return strings.TrimSpace(item.Ticket) != "" || strings.TrimSpace(item.Ciphertext) != ""
}

func weChatComponentTicketCredentialNeedsRotation(item weChatComponentTicketCredentialRecord, activeKeyID string) bool {
	return strings.TrimSpace(item.Ciphertext) == "" || strings.TrimSpace(item.KeyID) != strings.TrimSpace(activeKeyID) || strings.TrimSpace(item.Ticket) != ""
}

func officialAccountCredentialConfigured(item officialAccountCredentialRecord) bool {
	return strings.TrimSpace(item.Ciphertext) != "" || officialAccountLegacyPlaintextPresent(item)
}

func officialAccountLegacyPlaintextPresent(item officialAccountCredentialRecord) bool {
	return strings.TrimSpace(item.AuthorizationCode) != "" || strings.TrimSpace(item.PreAuthCode) != "" ||
		strings.TrimSpace(item.ComponentAESKey) != "" || strings.TrimSpace(item.ComponentToken) != "" || strings.TrimSpace(item.ComponentSecret) != ""
}

func officialAccountCredentialNeedsRotation(item officialAccountCredentialRecord, activeKeyID string) bool {
	return strings.TrimSpace(item.Ciphertext) == "" || strings.TrimSpace(item.KeyID) != strings.TrimSpace(activeKeyID) || officialAccountLegacyPlaintextPresent(item)
}
