package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
	archiveprovider "jiyi/mochat-go/internal/modules/providers/archive"
	"jiyi/mochat-go/internal/wecomcredentials"

	"github.com/google/uuid"
)

func upsertArchiveComponentTx(ctx context.Context, tx *sql.Tx, cipher *wecomcredentials.Manager, scope archiveprovider.Scope, message archiveprovider.Message) error {
	if message.ContentPolicy != archiveprovider.ContentPolicyComponent {
		if message.Component != nil {
			return errors.New("archive component policy mismatch")
		}
		return nil
	}
	if cipher == nil || !cipher.ConfigStatus().EncryptionConfigured {
		return errors.New("archive component encryption is not configured")
	}
	component := message.Component
	if component == nil || strings.TrimSpace(component.MessageID) != strings.TrimSpace(message.MsgID) || component.PublicKeyVersion == 0 || strings.TrimSpace(component.EncryptedSecretKey) == "" {
		return errors.New("archive component locator is invalid")
	}
	if len(message.Media) != 0 || strings.TrimSpace(message.ContentText) != "" {
		return errors.New("archive component message contains exported content")
	}
	var id string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM mochat_go_archive_component_locators
		WHERE tenant_id=? AND corp_id=? AND msgid=?
		LIMIT 1 FOR UPDATE
	`, scope.TenantID, scope.CorpID, strings.TrimSpace(message.MsgID)).Scan(&id)
	isNew := false
	if errors.Is(err, sql.ErrNoRows) {
		id = uuid.NewString()
		isNew = true
	} else if err != nil {
		return err
	}
	credential := wecomcredentials.ArchiveComponentCredential{
		MessageID: strings.TrimSpace(component.MessageID), PublicKeyVersion: component.PublicKeyVersion,
		EncryptedSecretKey: strings.TrimSpace(component.EncryptedSecretKey),
	}
	ciphertext, keyID, err := cipher.EncryptArchiveComponent(int(scope.TenantID), id, credential)
	if err != nil {
		return err
	}
	if isNew {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO mochat_go_archive_component_locators
			(id,tenant_id,corp_id,msgid,source_identity,content_policy,public_key_version,locator_ciphertext,locator_key_id,status)
			VALUES (?,?,?,?,?,'component',?,?,?,'available')
		`, id, scope.TenantID, scope.CorpID, strings.TrimSpace(message.MsgID), strings.TrimSpace(message.SourceID), component.PublicKeyVersion, ciphertext, keyID)
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE mochat_go_archive_component_locators
		SET source_identity=?,public_key_version=?,locator_ciphertext=?,locator_key_id=?,status='available',last_error_code=''
		WHERE id=? AND tenant_id=? AND corp_id=? AND msgid=?
	`, strings.TrimSpace(message.SourceID), component.PublicKeyVersion, ciphertext, keyID, id, scope.TenantID, scope.CorpID, strings.TrimSpace(message.MsgID))
	return err
}

func (s *MySQLStore) ArchiveComponentByID(ctx context.Context, filter dashboard.ArchiveComponentFilter) (dashboard.ArchiveComponentObject, bool, error) {
	if s == nil || s.db == nil || s.weComCredentialCipher == nil || filter.TenantID <= 0 || filter.CorpID <= 0 || strings.TrimSpace(filter.ID) == "" {
		return dashboard.ArchiveComponentObject{}, false, errors.New("archive component scope invalid")
	}
	conversationTypes := make([]int, 0, len(filter.ConversationScopes))
	for _, scope := range filter.ConversationScopes {
		conversationTypes = append(conversationTypes, scope.ConversationType)
	}
	conversationTypes = uniqueArchiveMediaConversationTypes(conversationTypes)
	conversationScopes, ok := archiveMediaConversationScopes(conversationTypes, filter.ConversationScopes)
	if len(conversationTypes) == 0 || !ok {
		return dashboard.ArchiveComponentObject{}, false, nil
	}
	messageUnion := make([]string, 0, dashboard.WorkMessageArchiveMessageTableCount)
	for tableIndex := 1; tableIndex <= dashboard.WorkMessageArchiveMessageTableCount; tableIndex++ {
		messageUnion = append(messageUnion, fmt.Sprintf("SELECT corp_id,msgid,work_employee_id,to_user_type FROM mc_work_message_%d WHERE deleted_at IS NULL", tableIndex))
	}
	where := []string{"message.to_user_type IN (" + placeholders(len(conversationTypes)) + ")"}
	args := []any{strings.TrimSpace(filter.ID), filter.TenantID, filter.CorpID}
	args = append(args, intsToAny(conversationTypes)...)
	scopeWhere := make([]string, 0, len(conversationScopes))
	for _, scope := range conversationScopes {
		if !scope.RestrictEmployeeIDs {
			scopeWhere = append(scopeWhere, "message.to_user_type=?")
			args = append(args, scope.ConversationType)
			continue
		}
		scopeWhere = append(scopeWhere, "(message.to_user_type=? AND message.work_employee_id IN ("+placeholders(len(scope.AllowedEmployeeIDs))+"))")
		args = append(args, scope.ConversationType)
		args = append(args, intsToAny(scope.AllowedEmployeeIDs)...)
	}
	var item dashboard.ArchiveComponentObject
	var ciphertext, keyID string
	var publicKeyVersion uint32
	err := s.db.QueryRowContext(ctx, `
		SELECT locator.id,locator.tenant_id,locator.corp_id,integration.verified_wx_corpid,
		       locator.msgid,locator.source_identity,locator.public_key_version,locator.locator_ciphertext,locator.locator_key_id
		FROM mochat_go_archive_component_locators locator
		INNER JOIN mc_tenant tenant ON tenant.id=locator.tenant_id AND tenant.status=1 AND tenant.deleted_at IS NULL
		INNER JOIN mc_corp corp ON corp.tenant_id=locator.tenant_id AND corp.id=locator.corp_id AND corp.deleted_at IS NULL
		INNER JOIN mochat_go_tenant_corp_bindings binding ON binding.tenant_id=locator.tenant_id AND binding.corp_id=locator.corp_id
		INNER JOIN mochat_go_wecom_integrations integration ON integration.tenant_id=locator.tenant_id AND integration.corp_id=locator.corp_id
		WHERE `+durableArchiveEligibilityPredicate+`
		  AND binding.wecom_integration_mode='third_party_delegated'
		  AND locator.status='available' AND locator.id=? AND locator.tenant_id=? AND locator.corp_id=?
		  AND EXISTS (
		    SELECT 1 FROM (`+strings.Join(messageUnion, " UNION ALL ")+`) message
		    WHERE message.corp_id=locator.corp_id AND message.msgid=locator.msgid AND `+strings.Join(where, " AND ")+`
		  )
		LIMIT 1
	`, args...).Scan(
		&item.ID, &item.TenantID, &item.CorpID, &item.WXCorpID, &item.MessageID, &item.SourceIdentity,
		&publicKeyVersion, &ciphertext, &keyID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return dashboard.ArchiveComponentObject{}, false, nil
	}
	if err != nil {
		return dashboard.ArchiveComponentObject{}, false, err
	}
	credential, err := s.weComCredentialCipher.DecryptArchiveComponent(item.TenantID, item.ID, keyID, ciphertext)
	if err != nil || credential.MessageID != item.MessageID || credential.PublicKeyVersion != publicKeyVersion {
		if err == nil {
			err = errors.New("archive component locator identity mismatch")
		}
		return dashboard.ArchiveComponentObject{}, false, err
	}
	item.PublicKeyVersion = credential.PublicKeyVersion
	item.EncryptedSecretKey = credential.EncryptedSecretKey
	return item, true, nil
}
