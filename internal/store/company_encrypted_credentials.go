package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"jiyi/mochat-go/internal/wecomcredentials"
)

func loadEncryptedCorpCredentialByID(ctx context.Context, queryer queryRower, corpID int, forUpdate bool) (corpCredentialRecord, bool, error) {
	if corpID <= 0 {
		return corpCredentialRecord{}, false, errors.New("company credential id is invalid")
	}
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE"
	}
	var item corpCredentialRecord
	err := queryer.QueryRowContext(ctx, `
		SELECT id, COALESCE(tenant_id, 0), COALESCE(wx_corpid, ''),
		       COALESCE(wecom_credentials_ciphertext, ''), COALESCE(wecom_credentials_key_id, '')
		FROM mc_corp
		WHERE id = ? AND deleted_at IS NULL`+suffix, corpID).Scan(
		&item.ID, &item.TenantID, &item.WXCorpID, &item.Ciphertext, &item.KeyID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return corpCredentialRecord{}, false, nil
	}
	if err != nil {
		return corpCredentialRecord{}, false, err
	}
	return item, true, nil
}

func (s *MySQLStore) decodeEncryptedCorpCredential(item corpCredentialRecord) (wecomcredentials.CorpCredential, error) {
	if s == nil || s.weComCredentialCipher == nil || strings.TrimSpace(item.Ciphertext) == "" || strings.TrimSpace(item.KeyID) == "" {
		return wecomcredentials.CorpCredential{}, errors.New("company credential is not encrypted")
	}
	return s.weComCredentialCipher.DecryptCorp(item.TenantID, item.WXCorpID, item.KeyID, item.Ciphertext)
}

func companyCredentialForRotation(s *MySQLStore, item corpCredentialRecord) (wecomcredentials.CorpCredential, error) {
	if strings.TrimSpace(item.Ciphertext) == "" && strings.TrimSpace(item.KeyID) == "" {
		return wecomcredentials.CorpCredential{}, nil
	}
	return s.decodeEncryptedCorpCredential(item)
}

func reencryptCompanyCorpCredential(s *MySQLStore, item corpCredentialRecord, wxCorpID string) (corpCredentialStorage, error) {
	if strings.TrimSpace(wxCorpID) == "" {
		return corpCredentialStorage{}, errors.New("company credential CorpID is empty")
	}
	credential, err := s.decodeEncryptedCorpCredential(item)
	if err != nil {
		return corpCredentialStorage{}, err
	}
	storage, err := s.encodeCorpCredential(item.TenantID, wxCorpID, credential)
	if err != nil {
		return corpCredentialStorage{}, err
	}
	if strings.TrimSpace(storage.Ciphertext) == "" || strings.TrimSpace(storage.KeyID) == "" {
		return corpCredentialStorage{}, errors.New("company credential re-encryption is unavailable")
	}
	return storage, nil
}
