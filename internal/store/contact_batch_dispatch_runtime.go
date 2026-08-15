package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"jiyi/mochat-go/internal/dashboard"
	"jiyi/mochat-go/internal/dashboardprincipal"
	"jiyi/mochat-go/internal/wecomcapability"
)

type contactBatchDispatchTarget struct {
	BatchID    int
	EmployeeID int
	ChunkNo    int
}

func (s *MySQLStore) ContactMessageBatchSendByIDForPrincipal(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, batchID int) (dashboard.ContactMessageBatchSendItem, bool, error) {
	if s == nil || s.db == nil || principal.TenantID <= 0 || principal.CorpID <= 0 || principal.UserID <= 0 || batchID <= 0 {
		return dashboard.ContactMessageBatchSendItem{}, false, dashboard.ErrContactBatchBodyScope
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, corp_id, user_id, medium_id, batch_title, user_name, employee_ids, filter_params, filter_params_detail, content,
		       send_way, definite_time, send_time, send_employee_total, send_contact_total, send_total,
		       not_send_total, received_total, not_received_total, receive_limit_total, not_friend_total,
		       send_status, created_at
		FROM mc_contact_message_batch_send
		WHERE tenant_id=? AND corp_id=? AND id=? AND user_id=? AND deleted_at IS NULL
		LIMIT 1`, principal.TenantID, principal.CorpID, batchID, principal.UserID)
	item, err := scanContactMessageBatchSendRow(row)
	if err == sql.ErrNoRows {
		return dashboard.ContactMessageBatchSendItem{}, false, nil
	}
	if err != nil {
		return dashboard.ContactMessageBatchSendItem{}, false, err
	}
	return item, true, nil
}

// ContactBatchDispatchPayload is called by the durable sender after the
// dispatch has been claimed. The payload is reconstructed from the same
// tenant-scoped business rows used at creation; it is never copied into the
// ledger and never logs credentials.
func (s *MySQLStore) ContactBatchDispatchPayload(ctx context.Context, principal dashboardprincipal.DashboardPrincipal, dispatch wecomcapability.Dispatch) (dashboard.ContactMessageBatchSendMessagePayload, error) {
	if s == nil || s.db == nil || principal.TenantID <= 0 || principal.CorpID <= 0 || dispatch.TenantID != principal.TenantID || dispatch.CorpID != principal.CorpID || dispatch.DispatchKind != string(wecomcapability.DispatchKindContactBatch) {
		return dashboard.ContactMessageBatchSendMessagePayload{}, dashboard.ErrContactBatchBodyScope
	}
	if err := s.AuthorizeDispatch(ctx, wecomcapability.DispatchAuthorizationRequest{
		Principal: wecomcapability.DispatchPrincipal{
			UserID: principal.UserID, TenantID: principal.TenantID, CorpID: principal.CorpID,
			IsSuperAdmin: principal.IsSuperAdmin, AuthVersion: principal.AuthVersion,
		},
		Capability: wecomcapability.ContactBatchSend, DispatchID: dispatch.ID,
		ExpectedCredentialVersion: dispatch.CredentialVersion, Stage: "payload",
	}); err != nil {
		return dashboard.ContactMessageBatchSendMessagePayload{}, err
	}
	target, ok := parseContactBatchDispatchTarget(dispatch.TargetID)
	if !ok {
		return dashboard.ContactMessageBatchSendMessagePayload{}, dashboard.ErrContactBatchBodyScope
	}
	var contentJSON, sender string
	if err := s.db.QueryRowContext(ctx, `
		SELECT b.content, e.wx_user_id
		FROM mc_contact_message_batch_send b
		JOIN mc_contact_message_batch_send_employee e ON e.batch_id=b.id AND e.employee_id=?
		WHERE b.tenant_id=? AND b.corp_id=? AND b.id=? AND b.deleted_at IS NULL`, target.EmployeeID, principal.TenantID, principal.CorpID, target.BatchID).Scan(&contentJSON, &sender); err != nil {
		if err == sql.ErrNoRows {
			return dashboard.ContactMessageBatchSendMessagePayload{}, dashboard.ErrContactBatchTargetNotOwned
		}
		return dashboard.ContactMessageBatchSendMessagePayload{}, err
	}
	var content []dashboard.ContactMessageBatchSendContent
	if err := json.Unmarshal([]byte(contentJSON), &content); err != nil || len(content) == 0 || strings.TrimSpace(sender) == "" {
		return dashboard.ContactMessageBatchSendMessagePayload{}, dashboard.ErrContactBatchBodyScope
	}
	offset := target.ChunkNo * contactBatchDispatchChunkSize
	rows, err := s.db.QueryContext(ctx, `
		SELECT external_user_id
		FROM mc_contact_message_batch_send_result
		WHERE batch_id=? AND employee_id=? AND external_user_id<>''
		ORDER BY id ASC LIMIT ?,?`, target.BatchID, target.EmployeeID, offset, contactBatchDispatchChunkSize)
	if err != nil {
		return dashboard.ContactMessageBatchSendMessagePayload{}, err
	}
	defer rows.Close()
	externalIDs := make([]string, 0, contactBatchDispatchChunkSize)
	for rows.Next() {
		var externalID string
		if err := rows.Scan(&externalID); err != nil {
			return dashboard.ContactMessageBatchSendMessagePayload{}, err
		}
		externalIDs = append(externalIDs, strings.TrimSpace(externalID))
	}
	if err := rows.Err(); err != nil {
		return dashboard.ContactMessageBatchSendMessagePayload{}, err
	}
	if len(externalIDs) == 0 {
		return dashboard.ContactMessageBatchSendMessagePayload{}, dashboard.ErrContactBatchTargetNotOwned
	}
	return dashboard.ContactMessageBatchSendMessagePayload{Content: content, ExternalUserID: externalIDs, Sender: strings.TrimSpace(sender)}, nil
}

func (s *MySQLStore) ContactBatchDispatchCredential(ctx context.Context, principal dashboardprincipal.DashboardPrincipal) (dashboard.RoomWelcomeCorpCredential, bool, error) {
	if principal.TenantID <= 0 || principal.CorpID <= 0 {
		return dashboard.RoomWelcomeCorpCredential{}, false, dashboard.ErrContactBatchBodyScope
	}
	credential, found, err := s.RoomWelcomeCorpCredentialByID(ctx, principal.CorpID)
	if err != nil || !found || credential.CorpID != principal.CorpID {
		return dashboard.RoomWelcomeCorpCredential{}, false, err
	}
	return credential, true, nil
}

func parseContactBatchDispatchTarget(value string) (contactBatchDispatchTarget, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 6 || parts[0] != "contact_batch" || parts[1] == "" || parts[2] != "employee" || parts[4] != "chunk" {
		return contactBatchDispatchTarget{}, false
	}
	batchID, batchErr := strconv.Atoi(parts[1])
	employeeID, employeeErr := strconv.Atoi(parts[3])
	chunkNo, chunkErr := strconv.Atoi(parts[5])
	if batchErr != nil || employeeErr != nil || chunkErr != nil || batchID <= 0 || employeeID <= 0 || chunkNo < 0 {
		return contactBatchDispatchTarget{}, false
	}
	return contactBatchDispatchTarget{BatchID: batchID, EmployeeID: employeeID, ChunkNo: chunkNo}, true
}

func contactBatchDispatchTargetString(batchID, employeeID, chunkNo int) string {
	return fmt.Sprintf("contact_batch:%d:employee:%d:chunk:%d", batchID, employeeID, chunkNo)
}
